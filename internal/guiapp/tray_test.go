package guiapp

import (
	"testing"

	"fyne.io/fyne/v2"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// findItem searches a menu and its submenus for an item by label.
func findItem(t *testing.T, menu *fyne.Menu, label string) *fyne.MenuItem {
	t.Helper()
	var walk func(items []*fyne.MenuItem) *fyne.MenuItem
	walk = func(items []*fyne.MenuItem) *fyne.MenuItem {
		for _, it := range items {
			if it.Label == label {
				return it
			}
			if it.ChildMenu != nil {
				if found := walk(it.ChildMenu.Items); found != nil {
					return found
				}
			}
		}
		return nil
	}
	if it := walk(menu.Items); it != nil {
		return it
	}
	t.Fatalf("menu item %q not found", label)
	return nil
}

func TestBuildMenu_Disconnected(t *testing.T) {
	m := gui.MenuModel{Connected: false, Summary: "daemon not running"}
	var started bool
	menu := buildMenu(m, Handlers{StartDaemon: func() { started = true }})

	if menu.Items[0].Label != "daemon not running" {
		t.Fatalf("first item = %q", menu.Items[0].Label)
	}
	var foundStart bool
	for _, it := range menu.Items {
		if it.Label == "启动 daemon" {
			foundStart = true
			it.Action()
		}
	}
	if !foundStart || !started {
		t.Fatalf("Start daemon item missing or not wired (found=%v started=%v)", foundStart, started)
	}
}

func TestBuildMenu_DisconnectedHasInstallAgent(t *testing.T) {
	m := gui.MenuModel{Connected: false, Summary: "daemon not running"}
	var installed bool
	menu := buildMenu(m, Handlers{InstallAgent: func() { installed = true }})
	findItem(t, menu, "安装并开机自启").Action()
	if !installed {
		t.Fatal("install-agent menu item not wired")
	}
}

func TestBuildMenu_ConnectedTogglesAndActions(t *testing.T) {
	m := gui.MenuModel{
		Connected: true,
		Summary:   "hopd · 1 up / 2",
		Groups: []gui.MenuGroup{{
			Name: "prod",
			Items: []gui.MenuTunnelItem{
				{Name: "prod-db", Label: "prod-db", State: "UP", Checked: true},
				{Name: "prod-redis", Label: "prod-redis", State: "DOWN", Checked: false},
			},
		}},
	}
	var toggled string
	var opened, allUp, allDown, reload, quit bool
	menu := buildMenu(m, Handlers{
		Toggle:  func(name string) { toggled = name },
		Open:    func() { opened = true },
		AllUp:   func() { allUp = true },
		AllDown: func() { allDown = true },
		Reload:  func() { reload = true },
		Quit:    func() { quit = true },
	})

	db := findItem(t, menu, "prod-db")
	if !db.Checked {
		t.Fatal("prod-db should be checked")
	}
	db.Action()
	if toggled != "prod-db" {
		t.Fatalf("toggle wired to %q", toggled)
	}

	findItem(t, menu, "打开 Dashboard…").Action()
	findItem(t, menu, "全部启动").Action()
	findItem(t, menu, "全部停止").Action()
	findItem(t, menu, "重载配置").Action()
	findItem(t, menu, "退出 hopd").Action()
	if !opened || !allUp || !allDown || !reload || !quit {
		t.Fatalf("fixed actions not all wired: open=%v allup=%v alldown=%v reload=%v quit=%v",
			opened, allUp, allDown, reload, quit)
	}
}

func TestThemeMenu(t *testing.T) {
	var picked string
	m := gui.MenuModel{Connected: true, Summary: "x"}
	menu := buildMenu(m, Handlers{Theme: "slate", SetTheme: func(n string) { picked = n }})

	// The active theme is checked.
	if it := findItem(t, menu, "石板蓝灰"); !it.Checked {
		t.Fatal("active theme (slate) should be checked")
	}
	if it := findItem(t, menu, "雾灰浅色"); it.Checked {
		t.Fatal("non-active theme should be unchecked")
	}
	// Picking another theme fires SetTheme with its name.
	findItem(t, menu, "暖炭灰").Action()
	if picked != "graphite" {
		t.Fatalf("SetTheme wired to %q, want graphite", picked)
	}
}
