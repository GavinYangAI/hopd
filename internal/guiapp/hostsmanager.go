package guiapp

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// openHosts opens (or focuses) the hosts manager window.
func (d *dashboard) openHosts() {
	if d.store == nil {
		return
	}
	if d.hostsWin == nil {
		d.hostsWin = d.app.NewWindow("hopd · 主机")
		d.hostsWin.SetIcon(logoResource)
		d.hostsWin.Resize(fyne.NewSize(560, 600))
		d.hostsWin.SetCloseIntercept(func() { d.hostsWin.Hide() })
	}
	d.refreshHosts()
	d.hostsWin.Show()
}

// refreshHosts rebuilds the hosts window content from the store.
func (d *dashboard) refreshHosts() {
	if d.hostsWin == nil {
		return
	}
	cfg, err := d.store.Load()
	if err != nil {
		dialog.ShowError(err, d.hostsWin)
		return
	}

	names := make([]string, 0)
	for name := range cfg.Hosts() {
		names = append(names, name)
	}
	sort.Strings(names)

	var cards []fyne.CanvasObject
	if len(names) == 0 {
		cards = append(cards, container.NewPadded(container.NewCenter(
			text("还没有主机，点右上角「新增主机」。", 13, pal.text3, fyne.TextStyle{}))))
	}
	for _, name := range names {
		h, _ := cfg.Host(name)
		f := gui.ToHostForm(name, h)
		cards = append(cards, hostCard(name, f, d.editHost, d.deleteHost))
	}
	body := container.New(layoutPadXY{px: 14, py: 12}, container.New(layoutStackV{gap: 9}, cards...))

	add := widget.NewButtonWithIcon("新增主机", theme.ContentAddIcon(), d.addHost)
	add.Importance = widget.HighImportance
	toolbar := container.New(layoutPadXY{px: 14, py: 9}, container.NewBorder(nil, nil, nil, add,
		text("已保存的主机", 13, pal.text2, bold)))
	tbBg := canvas.NewRectangle(pal.barTop)
	tbSep := canvas.NewRectangle(pal.border)
	tbSep.SetMinSize(fyne.NewSize(0, 1))
	header := container.NewStack(tbBg, container.NewBorder(nil, tbSep, nil, nil, toolbar))

	d.hostsWin.SetContent(container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
}

// hostSummary renders a one-line subtitle for a host card.
func hostSummary(f gui.HostForm) string {
	port := f.Port
	if strings.TrimSpace(port) == "" {
		port = "22"
	}
	parts := []string{f.Host + ":" + port}
	if u := strings.TrimSpace(f.User); u != "" {
		parts = append(parts, u)
	}
	if j := strings.TrimSpace(f.Jump); j != "" {
		parts = append(parts, "↳ "+j)
	}
	return strings.Join(parts, "  ·  ")
}

// hostCard builds one host row with edit/delete actions.
func hostCard(name string, f gui.HostForm, onEdit, onDelete func(string)) fyne.CanvasObject {
	title := text(name, 15, pal.text1, bold)
	sub := text(hostSummary(f), 12, pal.text2, mono)
	info := container.New(layoutStackV{gap: 4}, title, sub)

	edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { onEdit(name) })
	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { onDelete(name) })
	del.Importance = widget.DangerImportance
	actions := container.NewHBox(edit, del)

	bg := roundRect(pal.surface1, 12, 1, pal.border)
	inner := container.NewBorder(nil, nil, nil, actions, info)
	return container.NewStack(bg, container.New(layoutPadXY{px: 14, py: 12}, inner))
}

func (d *dashboard) addHost() {
	showHostDialog(d.hostsWin, d.store, gui.HostForm{}, "", d.refreshHosts)
}

func (d *dashboard) editHost(name string) {
	cfg, err := d.store.Load()
	if err != nil {
		dialog.ShowError(err, d.hostsWin)
		return
	}
	h, ok := cfg.Host(name)
	if !ok {
		return
	}
	showHostDialog(d.hostsWin, d.store, gui.ToHostForm(name, h), name, d.refreshHosts)
}

func (d *dashboard) deleteHost(name string) {
	dialog.ShowConfirm("删除主机", "确定删除主机 "+name+" ？", func(ok bool) {
		if !ok {
			return
		}
		cfg, err := d.store.Load()
		if err != nil {
			dialog.ShowError(err, d.hostsWin)
			return
		}
		if err := cfg.RemoveHost(name); err != nil { // errors if still referenced
			dialog.ShowError(err, d.hostsWin)
			return
		}
		if err := d.store.Save(cfg); err != nil && !isReloadWarning(err) {
			dialog.ShowError(err, d.hostsWin)
			return
		}
		d.refreshHosts()
	}, d.hostsWin)
}
