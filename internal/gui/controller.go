package gui

import (
	"fmt"
	"sync"
	"time"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

// DaemonClient is the subset of the ipc client the controller needs. The real
// client adapts to this in Task 6; tests inject a fake.
type DaemonClient interface {
	Watch(handler func(ipc.Response) error) error
	Do(req ipc.Request) (ipc.Response, error)
}

// Controller manages the daemon connection, caches the latest snapshot, emits
// notifications on alert-worthy transitions, and forwards commands. UI layers
// set OnUpdate / OnNotify; both may be called from a background goroutine, so a
// Fyne UI must marshal them onto the main thread (see guiapp, Task 8).
type Controller struct {
	client DaemonClient

	// OnUpdate fires after every snapshot (and on connect/disconnect) with the
	// current cache and connection state.
	OnUpdate func(snap []ipc.TunnelStatus, connected bool)
	// OnNotify fires once per alert-worthy transition.
	OnNotify func(n Notification)

	mu        sync.Mutex
	snap      []ipc.TunnelStatus
	connected bool

	stop chan struct{}
	once sync.Once
}

// NewController returns a controller bound to the given client.
func NewController(client DaemonClient) *Controller {
	return &Controller{client: client, stop: make(chan struct{})}
}

// Start runs the Watch/reconnect loop in a background goroutine.
func (c *Controller) Start() { go c.loop() }

// Stop ends the loop. Safe to call once.
func (c *Controller) Stop() {
	c.once.Do(func() { close(c.stop) })
}

func (c *Controller) loop() {
	for {
		select {
		case <-c.stop:
			return
		default:
		}
		// Watch blocks until the connection drops or an error occurs.
		_ = c.client.Watch(func(resp ipc.Response) error {
			c.apply(resp.Tunnels)
			select {
			case <-c.stop:
				return fmt.Errorf("stopped")
			default:
				return nil
			}
		})
		c.setConnected(false)
		select {
		case <-c.stop:
			return
		case <-time.After(2 * time.Second): // reconnect backoff
		}
	}
}

func (c *Controller) apply(next []ipc.TunnelStatus) {
	c.mu.Lock()
	prev := c.snap
	wasConnected := c.connected
	c.snap = next
	c.connected = true
	c.mu.Unlock()

	if wasConnected { // only diff between two live frames
		for _, n := range Diff(prev, next) {
			if c.OnNotify != nil {
				c.OnNotify(n)
			}
		}
	}
	if c.OnUpdate != nil {
		c.OnUpdate(next, true)
	}
}

func (c *Controller) setConnected(v bool) {
	c.mu.Lock()
	changed := c.connected != v
	c.connected = v
	snap := c.snap
	c.mu.Unlock()
	if changed && c.OnUpdate != nil {
		c.OnUpdate(snap, v)
	}
}

// Snapshot returns a copy-safe view of the latest cached snapshot.
func (c *Controller) Snapshot() []ipc.TunnelStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ipc.TunnelStatus(nil), c.snap...)
}

// Connected reports whether the last Watch frame arrived without a drop.
func (c *Controller) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *Controller) cmd(req ipc.Request) error {
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// Up starts tunnels matched by target (name | group | "all").
func (c *Controller) Up(target string) error {
	return c.cmd(ipc.Request{Cmd: ipc.CmdUp, Target: target})
}

// Down stops tunnels matched by target.
func (c *Controller) Down(target string) error {
	return c.cmd(ipc.Request{Cmd: ipc.CmdDown, Target: target})
}

// Restart stops then starts a single tunnel.
func (c *Controller) Restart(name string) error {
	if err := c.Down(name); err != nil {
		return err
	}
	return c.Up(name)
}

// Reload asks the daemon to reload its config.
func (c *Controller) Reload() error { return c.cmd(ipc.Request{Cmd: ipc.CmdReload}) }

// Logs returns the captured ssh stderr tail for a tunnel.
func (c *Controller) Logs(name string) ([]string, error) {
	resp, err := c.client.Do(ipc.Request{Cmd: ipc.CmdLogs, Target: name})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Lines, nil
}
