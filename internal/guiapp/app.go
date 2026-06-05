package guiapp

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/GavinYangAI/hopd/internal/gui"
	"github.com/GavinYangAI/hopd/internal/ipc"
	"github.com/GavinYangAI/hopd/internal/paths"
)

// ui holds the assembled application objects.
type ui struct {
	app  fyne.App
	ctrl *gui.Controller
	dash *dashboard

	// last applied frame, kept so a theme switch can re-render the tray.
	lastSnap      []ipc.TunnelStatus
	lastConnected bool
}

const themePrefKey = "theme"

// newUI assembles the controller, dashboard, and update wiring (no tray yet, so
// it is constructible under the headless test driver).
func newUI(a fyne.App, ctrl *gui.Controller) *ui {
	u := &ui{app: a, ctrl: ctrl}
	u.dash = newDashboard(a, &DashboardActions{
		Start:   ctrl.Up,
		Stop:    ctrl.Down,
		Restart: ctrl.Restart,
		Reload:  ctrl.Reload,
		Logs:    ctrl.Logs,
		StartDaemon: func() {
			if err := gui.StartDaemon(); err != nil {
				a.SendNotification(fyne.NewNotification("hopd", err.Error()))
			}
		},
	})
	u.dash.setStore(gui.NewConfigStore(paths.ConfigFile(), ctrl.Reload))
	return u
}

// applyUpdate refreshes tray + window + icon for a snapshot. Safe to call from
// any goroutine: it marshals UI work onto the main thread with fyne.Do.
func (u *ui) applyUpdate(snap []ipc.TunnelStatus, connected bool) {
	model := gui.BuildMenuModel(snap, connected)
	overall := gui.OverallState(snap, connected)
	fyne.Do(func() {
		// lastSnap/lastConnected are also read by setTheme on the main thread.
		// Writing them inside fyne.Do serializes both access sites on the main
		// thread, removing the data race with the controller goroutine.
		u.lastSnap, u.lastConnected = snap, connected
		if desk, ok := u.app.(desktop.App); ok {
			desk.SetSystemTrayMenu(buildMenu(model, u.handlers()))
			desk.SetSystemTrayIcon(iconFor(overall))
		}
		u.dash.updateState(snap, connected)
	})
}

func (u *ui) handlers() Handlers {
	return Handlers{
		Toggle: func(name string) {
			// Toggle based on the cached state of this tunnel.
			for _, t := range u.ctrl.Snapshot() {
				if t.Name == name {
					if t.State == "DOWN" {
						_ = u.ctrl.Up(name)
					} else {
						_ = u.ctrl.Down(name)
					}
					return
				}
			}
		},
		Open:    func() { fyne.Do(u.dash.show) },
		AllUp:   func() { _ = u.ctrl.Up("all") },
		AllDown: func() { _ = u.ctrl.Down("all") },
		Reload:  func() { _ = u.ctrl.Reload() },
		Quit:    func() { u.app.Quit() },
		StartDaemon: func() {
			if err := gui.StartDaemon(); err != nil {
				u.app.SendNotification(fyne.NewNotification("hopd", err.Error()))
			}
		},
		InstallAgent: func() {
			if err := gui.InstallAgent(); err != nil {
				u.app.SendNotification(fyne.NewNotification("hopd", err.Error()))
				return
			}
			u.app.SendNotification(fyne.NewNotification("hopd", "已开启开机自启，daemon 即将启动"))
		},
		Theme:    activeTheme,
		SetTheme: u.setTheme,
	}
}

// setTheme swaps the palette, persists the choice, re-themes Fyne widgets, and
// rebuilds the window + tray so the new colours take effect immediately.
func (u *ui) setTheme(name string) {
	if name == activeTheme {
		return
	}
	setPalette(name)
	u.app.Preferences().SetString(themePrefKey, name)
	fyne.Do(func() {
		u.app.Settings().SetTheme(hopdTheme{})
		u.dash.rebuildContent()
	})
	// Re-render the tray so the checkmark moves to the new theme.
	u.applyUpdate(u.lastSnap, u.lastConnected)
}

// Run is the program entry point: it builds everything, starts the controller,
// installs the tray, and runs the Fyne event loop until Quit.
func Run() {
	a := app.NewWithID("com.gavinyangai.hopd.gui")
	a.SetIcon(logoResource)
	setPalette(a.Preferences().StringWithFallback(themePrefKey, "mist"))
	a.Settings().SetTheme(hopdTheme{})
	ctrl := gui.NewController(NewDaemonClient(paths.SocketPath()))
	u := newUI(a, ctrl)

	ctrl.OnUpdate = u.applyUpdate
	ctrl.OnNotify = func(n gui.Notification) {
		u.app.SendNotification(fyne.NewNotification(n.Title, n.Message))
	}

	// Initial tray render (disconnected until the first frame arrives).
	u.applyUpdate(nil, false)

	// Opt-in: open the dashboard on launch (handy for dev/preview). The normal
	// menu-bar flow keeps the window hidden until "打开 Dashboard…".
	if os.Getenv("HOPD_GUI_OPEN") != "" {
		u.dash.show()
	}

	ctrl.Start()
	defer ctrl.Stop()
	a.Run()
}
