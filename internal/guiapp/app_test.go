package guiapp

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/gui"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

// stubClient is a no-op DaemonClient for headless wiring tests.
type stubClient struct{}

func (stubClient) Watch(handler func(ipc.Response) error) error { select {} }
func (stubClient) Do(req ipc.Request) (ipc.Response, error)     { return ipc.Response{OK: true}, nil }

func TestNewUI_Wires(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	ctrl := gui.NewController(stubClient{})
	u := newUI(app, ctrl)
	if u.dash == nil || u.ctrl == nil {
		t.Fatal("ui not wired")
	}
	// applyUpdate must not panic with an empty snapshot and disconnected state.
	u.applyUpdate(nil, false)
	u.applyUpdate([]ipc.TunnelStatus{{Name: "a", Group: "g", State: "UP"}}, true)
}
