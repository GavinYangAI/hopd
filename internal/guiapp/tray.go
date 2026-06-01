package guiapp

import (
	"fyne.io/fyne/v2"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// Handlers are the callbacks the tray menu invokes. Any may be nil.
type Handlers struct {
	Toggle      func(name string) // toggle one tunnel up/down
	Open        func()            // open the dashboard window
	AllUp       func()
	AllDown     func()
	Reload      func()
	Quit        func()
	StartDaemon func()            // shown only when disconnected
	Theme       string            // name of the active theme
	SetTheme    func(name string) // switch the active theme
}

// themeMenu builds the "主题" submenu with the active theme checked.
func themeMenu(h Handlers) *fyne.MenuItem {
	var subs []*fyne.MenuItem
	for _, t := range themeOrder {
		name := t.Name
		mi := fyne.NewMenuItem(t.Label, func() {
			if h.SetTheme != nil {
				h.SetTheme(name)
			}
		})
		mi.Checked = name == h.Theme
		subs = append(subs, mi)
	}
	item := fyne.NewMenuItem("主题", nil)
	item.ChildMenu = fyne.NewMenu("主题", subs...)
	return item
}

func call(f func()) {
	if f != nil {
		f()
	}
}

// buildMenu turns a menu model into a Fyne menu tree.
func buildMenu(m gui.MenuModel, h Handlers) *fyne.Menu {
	summary := fyne.NewMenuItem(m.Summary, nil)
	summary.Disabled = true
	items := []*fyne.MenuItem{summary, fyne.NewMenuItemSeparator()}

	if !m.Connected {
		start := fyne.NewMenuItem("启动 daemon", func() { call(h.StartDaemon) })
		quit := fyne.NewMenuItem("退出", func() { call(h.Quit) })
		items = append(items, start, themeMenu(h), fyne.NewMenuItemSeparator(), quit)
		return fyne.NewMenu("hopd", items...)
	}

	for _, g := range m.Groups {
		var sub []*fyne.MenuItem
		for _, it := range g.Items {
			name := it.Name
			mi := fyne.NewMenuItem(it.Label, func() {
				if h.Toggle != nil {
					h.Toggle(name)
				}
			})
			mi.Checked = it.Checked
			sub = append(sub, mi)
		}
		groupItem := fyne.NewMenuItem(g.Name, nil)
		groupItem.ChildMenu = fyne.NewMenu(g.Name, sub...)
		items = append(items, groupItem)
	}

	items = append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("打开 Dashboard…", func() { call(h.Open) }),
		fyne.NewMenuItem("全部启动", func() { call(h.AllUp) }),
		fyne.NewMenuItem("全部停止", func() { call(h.AllDown) }),
		fyne.NewMenuItem("重载配置", func() { call(h.Reload) }),
		themeMenu(h),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("退出 hopd", func() { call(h.Quit) }),
	)
	return fyne.NewMenu("hopd", items...)
}
