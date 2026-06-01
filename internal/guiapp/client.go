// Package guiapp holds hopd-gui's Fyne adapters: the daemon client adapter,
// tray menu, icon, detail window, and app assembly. The pure presentation
// logic lives in internal/gui (no Fyne dependency).
package guiapp

import (
	"github.com/GavinYangAI/hopd/internal/ipc"
)

// daemonClient adapts *ipc.Client to gui.DaemonClient by binding the watch
// request, so the controller can call Watch(handler) without knowing the
// protocol detail.
type daemonClient struct{ c *ipc.Client }

// NewDaemonClient returns a gui.DaemonClient for the daemon socket at sock.
func NewDaemonClient(sock string) *daemonClient {
	return &daemonClient{c: ipc.NewClient(sock)}
}

// Watch subscribes to streaming status updates.
func (d *daemonClient) Watch(handler func(ipc.Response) error) error {
	return d.c.Watch(ipc.Request{Cmd: ipc.CmdWatch}, handler)
}

// Do sends a one-shot request.
func (d *daemonClient) Do(req ipc.Request) (ipc.Response, error) { return d.c.Do(req) }
