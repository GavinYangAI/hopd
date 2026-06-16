package tunnel

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

const maxLogLines = 200

// Runner supervises one ssh -N child process for a single tunnel.
type Runner struct {
	tunnel     config.Tunnel
	sshPath    string
	backoffMin time.Duration
	backoffMax time.Duration
	localAddr  string

	sshConfigPath string // when set, use BuildArgsVia (-F) instead of BuildArgs
	entryHost     string // ssh destination alias for the -F path

	mu         sync.Mutex
	probe      func(addr string) bool
	state      State
	reconnects int
	lastErr    string
	upSince    time.Time
	logs       []string
	fatal      bool // last attempt hit an unrecoverable forward failure

	cancel context.CancelFunc
	done   chan struct{}
}

// NewRunner builds a Runner. The probe defaults to "always healthy"; Task 8
// replaces it with a real TCP dial via SetProbe / the package default.
func NewRunner(t config.Tunnel, sshPath string, min, max time.Duration) *Runner {
	addr, port := normalizeLocal(t.Local)
	return &Runner{
		tunnel:     t,
		sshPath:    sshPath,
		backoffMin: min,
		backoffMax: max,
		localAddr:  addr + ":" + port,
		probe:      defaultProbe,
		state:      StateDown,
	}
}

// SetProbe overrides the health probe (used in tests and by NewRunnerProbed).
func (r *Runner) SetProbe(p func(addr string) bool) {
	r.mu.Lock()
	r.probe = p
	r.mu.Unlock()
}

// defaultProbe checks the local listen port via a real TCP dial.
func defaultProbe(addr string) bool { return dialProbe(addr) }

// Start launches the supervision loop. It is idempotent.
func (r *Runner) Start() {
	r.mu.Lock()
	if r.cancel != nil {
		r.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	r.reconnects = 0
	r.mu.Unlock()
	go r.loop(ctx)
}

// Stop terminates the child process and marks the tunnel DOWN.
func (r *Runner) Stop() {
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.cancel = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	r.set(StateDown, "")
}

func (r *Runner) loop(ctx context.Context) {
	defer func() {
		// Clear cancel so the runner is cleanly restartable even when the loop
		// exits on its own (e.g. a fatal forward error), not just via Stop.
		r.mu.Lock()
		r.cancel = nil
		r.mu.Unlock()
		close(r.done)
	}()
	backoff := r.backoffMin
	first := true
	for {
		if ctx.Err() != nil {
			return
		}
		if !first {
			r.set(StateRetrying, "")
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > r.backoffMax {
				backoff = r.backoffMax
			}
			r.incReconnects()
		}
		first = false

		r.set(StateStarting, "")
		attemptCtx, attemptCancel := context.WithCancel(ctx)
		cmd := exec.CommandContext(attemptCtx, r.sshPath, r.argv()...)
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			r.set(StateError, err.Error())
			attemptCancel()
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}
		stderrDone := make(chan struct{})
		go func() { r.readStderr(stderr); close(stderrDone) }()
		go r.watchUp(attemptCtx)

		err := cmd.Wait()
		attemptCancel()
		<-stderrDone // ensure all stderr is classified before deciding what's next
		if ctx.Err() != nil {
			return
		}
		if r.fatalSet() {
			return // a local-forward failure is not recoverable by retrying
		}
		if err != nil {
			r.setLastErr(err.Error())
		}
	}
}

// watchUp flips STARTING -> UP once the local port accepts a connection.
func (r *Runner) watchUp(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			state, probe := r.state, r.probe
			r.mu.Unlock()
			if state != StateStarting {
				return
			}
			if probe(r.localAddr) {
				r.set(StateUp, "")
				return
			}
		}
	}
}

func (r *Runner) readStderr(rc io.Reader) {
	sc := bufio.NewScanner(rc)
	for sc.Scan() {
		line := sc.Text()
		r.appendLog(line)
		switch {
		case isFatalForward(line):
			r.markFatal(line)
		case isAuthPrompt(line):
			r.set(StateNeedsAuth, line)
		}
	}
}

// isFatalForward detects an unrecoverable local-forward failure (e.g. the local
// port is already in use). Retrying cannot fix it, so the tunnel goes to ERROR.
// Requires ssh to exit on the failure — ExitOnForwardFailure=yes is injected by
// the daemon defaults so this signal reliably arrives.
func isFatalForward(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "address already in use") ||
		strings.Contains(l, "could not request local forwarding") ||
		strings.Contains(l, "cannot listen to port") ||
		strings.Contains(l, "error: bind:")
}

// markFatal records an unrecoverable error and moves the tunnel to ERROR.
func (r *Runner) markFatal(msg string) {
	r.mu.Lock()
	r.fatal = true
	r.lastErr = msg
	r.state = StateError
	r.upSince = time.Time{}
	r.mu.Unlock()
}

// fatalSet reports (and clears) whether an unrecoverable error was seen during
// the last attempt. Clearing lets a later Start() retry from a clean slate.
func (r *Runner) fatalSet() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	f := r.fatal
	r.fatal = false
	return f
}

// isAuthPrompt detects ssh interactive auth requests in stderr. Task 15 acts
// on the NEEDS_AUTH state; here we only classify.
func isAuthPrompt(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "password:") ||
		strings.Contains(l, "verification code") ||
		strings.Contains(l, "passcode") ||
		strings.Contains(l, "permission denied")
}

func (r *Runner) set(s State, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = s
	if s == StateUp {
		if r.upSince.IsZero() {
			r.upSince = time.Now()
		}
	} else {
		r.upSince = time.Time{}
	}
	if errMsg != "" {
		r.lastErr = errMsg
	}
}

func (r *Runner) setLastErr(msg string) {
	r.mu.Lock()
	r.lastErr = msg
	r.mu.Unlock()
}

func (r *Runner) incReconnects() {
	r.mu.Lock()
	r.reconnects++
	r.mu.Unlock()
}

func (r *Runner) appendLog(line string) {
	r.mu.Lock()
	r.logs = append(r.logs, line)
	if len(r.logs) > maxLogLines {
		r.logs = r.logs[len(r.logs)-maxLogLines:]
	}
	r.mu.Unlock()
}

// Logs returns a copy of the captured stderr tail.
func (r *Runner) Logs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.logs))
	copy(out, r.logs)
	return out
}

// Spec returns the tunnel configuration this runner was built from.
func (r *Runner) Spec() config.Tunnel { return r.tunnel }

// SetSSHConfig attaches a hopd-generated ssh config so the runner launches ssh
// with -F <path> and connects to entry. Called by the daemon for via_host
// tunnels before Start.
func (r *Runner) SetSSHConfig(path, entry string) {
	r.mu.Lock()
	r.sshConfigPath = path
	r.entryHost = entry
	r.mu.Unlock()
}

// argv builds the ssh argument vector for the current attempt, choosing the
// generated-config path when one is attached.
func (r *Runner) argv() []string {
	r.mu.Lock()
	path, entry := r.sshConfigPath, r.entryHost
	r.mu.Unlock()
	if path != "" {
		return BuildArgsVia(r.tunnel, path, entry)
	}
	return BuildArgs(r.tunnel)
}

// Snapshot returns a point-in-time status for the IPC layer.
func (r *Runner) Snapshot() ipc.TunnelStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	var uptime int64
	if !r.upSince.IsZero() {
		uptime = int64(time.Since(r.upSince).Seconds())
	}
	return ipc.TunnelStatus{
		Name:       r.tunnel.Name,
		Group:      r.tunnel.Group,
		State:      r.state.String(),
		Local:      r.tunnel.Local,
		Remote:     r.tunnel.Remote,
		Via:        r.tunnel.Via,
		ViaHost:    r.tunnel.ViaHost,
		UptimeSec:  uptime,
		Reconnects: r.reconnects,
		LastError:  r.lastErr,
	}
}
