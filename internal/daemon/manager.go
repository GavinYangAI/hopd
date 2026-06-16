// Package daemon coordinates tunnel runners and serves the control socket.
package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/ipc"
	"github.com/GavinYangAI/hopd/internal/paths"
	"github.com/GavinYangAI/hopd/internal/sshconf"
	"github.com/GavinYangAI/hopd/internal/tunnel"
)

// Manager owns one runner per tunnel and dispatches up/down/status/reload.
type Manager struct {
	mu      sync.Mutex
	sshPath string
	genDir  string
	cfg     *config.Config
	runners map[string]*tunnel.Runner
	order   []string // config order, for stable Status output
}

// NewManager builds runners (all DOWN) for every tunnel in cfg, writing any
// generated ssh -F configs under paths.GeneratedDir().
func NewManager(sshPath string, cfg *config.Config) *Manager {
	return NewManagerWithGenDir(sshPath, cfg, paths.GeneratedDir())
}

// NewManagerWithGenDir is NewManager with an explicit generated-config dir (for
// tests).
func NewManagerWithGenDir(sshPath string, cfg *config.Config, genDir string) *Manager {
	m := &Manager{sshPath: sshPath, genDir: genDir, cfg: cfg, runners: map[string]*tunnel.Runner{}}
	for _, t := range cfg.Tunnels() {
		m.runners[t.Name] = m.buildRunner(cfg, t)
		m.order = append(m.order, t.Name)
	}
	return m
}

// buildRunner creates a runner for t and, when t uses via_host, writes its
// generated ssh config and attaches it. A generation/write failure is left to
// surface at connect time (the runner falls back to legacy argv, which will
// fail clearly) rather than aborting manager construction.
func (m *Manager) buildRunner(cfg *config.Config, t config.Tunnel) *tunnel.Runner {
	r := tunnel.NewRunner(t, m.sshPath, cfg.Restart.Min, cfg.Restart.Max)
	text, entry, err := sshconf.Generate(cfg, t)
	if err == nil && text != "" {
		path := filepath.Join(m.genDir, t.Name+".sshcfg")
		if writeErr := writeGenerated(path, text); writeErr == nil {
			r.SetSSHConfig(path, entry)
		}
	}
	return r
}

// writeGenerated writes the generated ssh config atomically with 0600 perms,
// creating the parent dir (0700) if needed.
func writeGenerated(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// genConfigChanged reports whether t's freshly generated ssh config differs
// from what is on disk. Legacy tunnels (no via_host) always report false.
func (m *Manager) genConfigChanged(cfg *config.Config, t config.Tunnel) bool {
	text, _, err := sshconf.Generate(cfg, t)
	if err != nil || text == "" {
		return false
	}
	existing, err := os.ReadFile(filepath.Join(m.genDir, t.Name+".sshcfg"))
	if err != nil {
		return true // missing/unreadable => must (re)write
	}
	return string(existing) != text
}

// StartAutostart brings up every tunnel marked autostart in config. The daemon
// calls it once at startup so marked tunnels reconnect after a reboot without
// manual intervention. Tunnels needing interactive auth (2FA/passphrase) settle
// into NEEDS_AUTH until the user runs `hopd auth`.
func (m *Manager) StartAutostart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.cfg.Tunnels() {
		if t.Autostart {
			if r, ok := m.runners[t.Name]; ok {
				r.Start()
			}
		}
	}
}

// Up starts the runners matched by target (name | group | "all"/"").
func (m *Manager) Up(target string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	names, err := m.resolveLocked(target)
	if err != nil {
		return err
	}
	for _, n := range names {
		m.runners[n].Start()
	}
	return nil
}

// Down stops the runners matched by target.
func (m *Manager) Down(target string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	names, err := m.resolveLocked(target)
	if err != nil {
		return err
	}
	for _, n := range names {
		m.runners[n].Stop()
	}
	return nil
}

// Status returns a snapshot of every tunnel in config order.
func (m *Manager) Status() []ipc.TunnelStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ipc.TunnelStatus, 0, len(m.order))
	for _, n := range m.order {
		out = append(out, m.runners[n].Snapshot())
	}
	return out
}

// Logs returns the stderr tail of one tunnel.
func (m *Manager) Logs(name string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runners[name]
	if !ok {
		return nil, fmt.Errorf("no tunnel named %q", name)
	}
	return r.Logs(), nil
}

// Runner returns a runner by name (used by the auth handler in Task 16).
func (m *Manager) Runner(name string) (*tunnel.Runner, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runners[name]
	return r, ok
}

// StopAll stops every runner. Called on daemon shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runners {
		r.Stop()
	}
}

// Reload swaps in a new config: unchanged tunnels keep running, changed ones
// restart if they were active, removed ones stop, new ones are added DOWN.
func (m *Manager) Reload(cfg *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Restart backoff bounds live in cfg.Restart, not in a tunnel's Spec, so a
	// bounds-only edit wouldn't otherwise reach reused runners. When they change,
	// force every runner to rebuild (preserving active state) so the new bounds
	// take effect.
	boundsChanged := m.cfg.Restart != cfg.Restart
	old := m.runners
	next := map[string]*tunnel.Runner{}
	var order []string
	for _, t := range cfg.Tunnels() {
		order = append(order, t.Name)
		if r, ok := old[t.Name]; ok {
			if !boundsChanged && reflect.DeepEqual(r.Spec(), t) && !m.genConfigChanged(cfg, t) {
				next[t.Name] = r
				delete(old, t.Name)
				continue
			}
			wasActive := isActive(r.Snapshot().State)
			r.Stop()
			delete(old, t.Name)
			nr := m.buildRunner(cfg, t)
			if wasActive {
				nr.Start()
			}
			next[t.Name] = nr
			continue
		}
		next[t.Name] = m.buildRunner(cfg, t)
	}
	for name, r := range old { // tunnels removed from config
		r.Stop()
		_ = os.Remove(filepath.Join(m.genDir, name+".sshcfg"))
	}
	m.cfg = cfg
	m.runners = next
	m.order = order
	return nil
}

func (m *Manager) resolveLocked(target string) ([]string, error) {
	if target == "" || target == "all" {
		return append([]string(nil), m.order...), nil
	}
	if _, ok := m.runners[target]; ok {
		return []string{target}, nil
	}
	var names []string
	for _, t := range m.cfg.Tunnels() {
		if t.Group == target {
			names = append(names, t.Name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no tunnel or group named %q", target)
	}
	return names, nil
}

// isActive reports whether a state means the runner is supervising (not DOWN).
func isActive(state string) bool { return state != "DOWN" }
