// Package daemon coordinates tunnel runners and serves the control socket.
package daemon

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/ipc"
	"github.com/GavinYangAI/hopd/internal/tunnel"
)

// Manager owns one runner per tunnel and dispatches up/down/status/reload.
type Manager struct {
	mu      sync.Mutex
	sshPath string
	cfg     *config.Config
	runners map[string]*tunnel.Runner
	order   []string // config order, for stable Status output
}

// NewManager builds runners (all DOWN) for every tunnel in cfg.
func NewManager(sshPath string, cfg *config.Config) *Manager {
	m := &Manager{sshPath: sshPath, cfg: cfg, runners: map[string]*tunnel.Runner{}}
	for _, t := range cfg.Tunnels() {
		m.runners[t.Name] = tunnel.NewRunner(t, sshPath, cfg.Restart.Min, cfg.Restart.Max)
		m.order = append(m.order, t.Name)
	}
	return m
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
	old := m.runners
	next := map[string]*tunnel.Runner{}
	var order []string
	for _, t := range cfg.Tunnels() {
		order = append(order, t.Name)
		if r, ok := old[t.Name]; ok {
			if reflect.DeepEqual(r.Spec(), t) {
				next[t.Name] = r
				delete(old, t.Name)
				continue
			}
			wasActive := isActive(r.Snapshot().State)
			r.Stop()
			delete(old, t.Name)
			nr := tunnel.NewRunner(t, m.sshPath, cfg.Restart.Min, cfg.Restart.Max)
			if wasActive {
				nr.Start()
			}
			next[t.Name] = nr
			continue
		}
		next[t.Name] = tunnel.NewRunner(t, m.sshPath, cfg.Restart.Min, cfg.Restart.Max)
	}
	for _, r := range old { // tunnels removed from config
		r.Stop()
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
