package guiapp

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

// DashboardActions are the controller hooks the window invokes for the selected
// tunnel. Any may be nil (used by the construction smoke test).
type DashboardActions struct {
	Start       func(name string) error
	Stop        func(name string) error
	Restart     func(name string) error
	Reload      func() error
	Logs        func(name string) ([]string, error)
	StartDaemon func()
}

type dashboard struct {
	app     fyne.App
	actions *DashboardActions
	store   *gui.ConfigStore
	win     fyne.Window

	snap      []ipc.TunnelStatus
	connected bool
	selName   string
	query     string

	// stable widgets / refresh targets
	search    *widget.Entry
	body      *fyne.Container // VBox swapped between cards and empty states
	scroll    *container.Scroll
	summary   *canvas.Text
	toneRow   *fyne.Container
	toolbar   *fyne.Container
	selLabel  *canvas.Text
	startStop *widget.Button
	rowBtns   []*widget.Button // per-selection action buttons to enable/disable
}

// newDashboard builds the (hidden) detail window.
func newDashboard(app fyne.App, actions *DashboardActions) *dashboard {
	d := &dashboard{app: app, actions: actions}
	d.win = app.NewWindow("hopd")
	d.win.SetIcon(logoResource)
	d.win.Resize(fyne.NewSize(780, 600))
	d.win.SetCloseIntercept(func() { d.win.Hide() }) // hide, don't quit

	d.buildContent()
	return d
}

// buildContent (re)constructs the window's header/body/toolbar. Called once at
// construction and again when the theme changes (colours are baked into the
// canvas objects, so a swap needs a rebuild). Selection/search/snapshot state
// is preserved across rebuilds.
func (d *dashboard) buildContent() {
	d.win.SetContent(container.NewBorder(d.buildHeader(), d.buildToolbar(), nil, nil, d.buildBody()))
	if d.query != "" {
		d.search.SetText(d.query)
	}
	d.refresh()
}

// rebuildContent re-themes the window in place.
func (d *dashboard) rebuildContent() { d.buildContent() }

// ---- header --------------------------------------------------------------

func (d *dashboard) buildHeader() fyne.CanvasObject {
	appName := text("hopd", 14, pal.text1, bold)
	d.summary = text("", 11.5, pal.text2, fyne.TextStyle{})
	d.toneRow = container.NewHBox(d.summary)
	titleBlock := container.New(layoutStackV{gap: 3}, appName, d.toneRow)

	logo := canvas.NewImageFromResource(logoResource)
	logo.FillMode = canvas.ImageFillContain
	logoBox := container.NewGridWrap(fyne.NewSize(30, 30), logo)
	left := container.NewHBox(container.NewCenter(logoBox), titleBlock)

	d.search = widget.NewEntry()
	d.search.PlaceHolder = "搜索隧道 / 主机 / 中继"
	d.search.OnChanged = func(s string) {
		d.query = s
		d.refreshBody()
	}
	searchBox := container.NewGridWrap(fyne.NewSize(240, 36), d.search)

	bar := container.NewBorder(nil, nil,
		container.New(layoutPadXY{px: 4, py: 2}, left), searchBox)
	bg := canvas.NewRectangle(pal.barTop)
	sep := canvas.NewRectangle(pal.border)
	sep.SetMinSize(fyne.NewSize(0, 1))
	return container.NewStack(bg,
		container.NewBorder(nil, sep, nil, nil, container.New(layoutPadXY{px: 14, py: 10}, bar)))
}

// tonePill rebuilds the coloured aggregate-health pill beside the summary.
func (d *dashboard) tonePill(o gui.Overall) {
	if d.toneRow == nil {
		return
	}
	t := gui.OverallTone(o)
	dot := canvas.NewCircle(toneColor(t))
	lbl := text(gui.OverallPhrase(o), 12, toneColor(t), bold)
	row := container.New(layout12{gap: 6, padY: 3}, dot, lbl)
	bg := roundRect(toneSoft(t), 999, 0, nil)
	pill := container.NewStack(bg, container.New(layoutPadXY{px: 9, py: 0}, row))
	d.toneRow.Objects = []fyne.CanvasObject{pill, d.summary}
	d.toneRow.Refresh()
}

// ---- body ----------------------------------------------------------------

func (d *dashboard) buildBody() fyne.CanvasObject {
	d.body = container.NewVBox()
	d.scroll = container.NewVScroll(d.body)
	return d.scroll
}

// refreshBody rebuilds the grouped card list (or an empty state) from the
// current snapshot + search query, without touching the search field/scroll.
func (d *dashboard) refreshBody() {
	if d.body == nil {
		return
	}
	if !d.connected {
		d.body.Objects = []fyne.CanvasObject{d.emptyState(
			"未连接到 daemon",
			"后台守护进程没有运行，启动后即可看到隧道状态。",
			"启动 daemon", func() { call(d.actions.StartDaemon) }, true)}
		d.body.Refresh()
		return
	}
	if len(d.snap) == 0 {
		d.body.Objects = []fyne.CanvasObject{d.emptyState(
			"还没有隧道",
			"新增第一条隧道，把本地端口连到内网服务。",
			"新增隧道", d.addTunnel, false)}
		d.body.Refresh()
		return
	}

	filtered := d.filtered()
	if len(filtered) == 0 {
		note := text(fmt.Sprintf("没有匹配「%s」的隧道", d.query), 13, pal.text3, fyne.TextStyle{})
		d.body.Objects = []fyne.CanvasObject{container.NewPadded(container.NewCenter(note))}
		d.body.Refresh()
		return
	}

	var objs []fyne.CanvasObject
	for _, g := range groupOrder(filtered) {
		objs = append(objs, groupHeader(g, countInGroup(filtered, g)))
		for _, t := range filtered {
			if t.Group == g {
				objs = append(objs, tunnelCard(t, t.Name == d.selName, d.selectTunnel))
			}
		}
		objs = append(objs, spacer(6))
	}
	d.body.Objects = []fyne.CanvasObject{container.New(layoutPadXY{px: 14, py: 12}, container.New(layoutStackV{gap: 9}, objs...))}
	d.body.Refresh()
}

func (d *dashboard) filtered() []ipc.TunnelStatus {
	q := strings.TrimSpace(d.query)
	if q == "" {
		return d.snap
	}
	var out []ipc.TunnelStatus
	for _, t := range d.snap {
		if strings.Contains(t.Name, q) || strings.Contains(t.Group, q) ||
			strings.Contains(t.Remote, q) || strings.Contains(t.Via, q) {
			out = append(out, t)
		}
	}
	return out
}

func (d *dashboard) selectTunnel(name string) {
	d.selName = name
	d.refreshBody()
	d.refreshToolbar()
}

func (d *dashboard) emptyState(title, sub, btn string, onClick func(), warnIcon bool) fyne.CanvasObject {
	ic := theme.ContentAddIcon()
	if warnIcon {
		ic = theme.MediaPlayIcon()
	}
	t := text(title, 17, pal.text1, bold)
	s := text(sub, 13, pal.text2, fyne.TextStyle{})
	s.Alignment = fyne.TextAlignCenter
	b := widget.NewButtonWithIcon(btn, ic, onClick)
	b.Importance = widget.HighImportance
	col := container.NewVBox(
		container.NewCenter(t),
		container.NewCenter(s),
		spacer(6),
		container.NewCenter(b),
	)
	return container.New(layoutPadXY{px: 40, py: 60}, container.NewCenter(col))
}

// ---- toolbar -------------------------------------------------------------

func (d *dashboard) buildToolbar() fyne.CanvasObject {
	d.selLabel = text("未选择隧道", 12, pal.text3, fyne.TextStyle{})

	d.startStop = widget.NewButtonWithIcon("启动", theme.MediaPlayIcon(), func() { d.run(d.actionStartStop) })
	d.startStop.Importance = widget.HighImportance

	restart := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() { d.run(d.actionRestart) })
	restart.SetText("重启")
	logs := widget.NewButtonWithIcon("日志", theme.DocumentIcon(), d.showLogs)
	edit := widget.NewButtonWithIcon("编辑", theme.DocumentCreateIcon(), d.editTunnel)
	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), d.deleteTunnel)
	del.Importance = widget.DangerImportance

	d.rowBtns = []*widget.Button{d.startStop, restart, logs, edit, del}
	rowZone := container.NewHBox(d.selLabel, widget.NewLabel(" "),
		d.startStop, restart, logs, edit, del)

	reload := widget.NewButtonWithIcon("重载", theme.ViewRefreshIcon(), func() { d.run(d.actionReload) })
	add := widget.NewButtonWithIcon("新增隧道", theme.ContentAddIcon(), d.addTunnel)
	add.Importance = widget.HighImportance
	globalZone := container.NewHBox(reload, add)

	bar := container.NewBorder(nil, nil, rowZone, globalZone)
	bg := canvas.NewRectangle(pal.barBot)
	sep := canvas.NewRectangle(pal.border)
	sep.SetMinSize(fyne.NewSize(0, 1))
	d.toolbar = container.NewStack(bg,
		container.NewBorder(sep, nil, nil, nil, container.New(layoutPadXY{px: 14, py: 9}, bar)))
	return d.toolbar
}

// refreshToolbar updates the selection label, the start/stop button, and the
// enabled state of the per-selection buttons.
func (d *dashboard) refreshToolbar() {
	if d.toolbar == nil {
		return
	}
	sel, ok := d.selected()
	if !ok {
		d.selLabel.Text = "未选择隧道"
		d.selLabel.Color = pal.text3
		for _, b := range d.rowBtns {
			b.Disable()
		}
	} else {
		d.selLabel.Text = "选中 " + sel.Name
		d.selLabel.Color = pal.text2
		for _, b := range d.rowBtns {
			b.Enable()
		}
		running := sel.State != "DOWN" && sel.State != "ERROR"
		if running {
			d.startStop.SetText("停止")
			d.startStop.SetIcon(theme.MediaStopIcon())
			d.startStop.Importance = widget.MediumImportance
		} else {
			d.startStop.SetText("启动")
			d.startStop.SetIcon(theme.MediaPlayIcon())
			d.startStop.Importance = widget.HighImportance
		}
		d.startStop.Refresh()
	}
	d.selLabel.Refresh()

	// Hide the toolbar entirely on the empty / disconnected states.
	if d.connected && len(d.snap) > 0 {
		d.toolbar.Show()
	} else {
		d.toolbar.Hide()
	}
}

func (d *dashboard) selected() (ipc.TunnelStatus, bool) {
	for _, t := range d.snap {
		if t.Name == d.selName {
			return t, true
		}
	}
	return ipc.TunnelStatus{}, false
}

// ---- actions -------------------------------------------------------------

func (d *dashboard) actionStartStop() error {
	sel, ok := d.selected()
	if !ok {
		return nil
	}
	if sel.State != "DOWN" && sel.State != "ERROR" {
		return d.actionStop()
	}
	return d.actionStart()
}

func (d *dashboard) actionStart() error {
	if d.actions == nil || d.actions.Start == nil || d.selName == "" {
		return nil
	}
	return d.actions.Start(d.selName)
}
func (d *dashboard) actionStop() error {
	if d.actions == nil || d.actions.Stop == nil || d.selName == "" {
		return nil
	}
	return d.actions.Stop(d.selName)
}
func (d *dashboard) actionRestart() error {
	if d.actions == nil || d.actions.Restart == nil || d.selName == "" {
		return nil
	}
	return d.actions.Restart(d.selName)
}
func (d *dashboard) actionReload() error {
	if d.actions == nil || d.actions.Reload == nil {
		return nil
	}
	return d.actions.Reload()
}

// setStore wires the config store used by Add/Edit/Delete.
func (d *dashboard) setStore(s *gui.ConfigStore) { d.store = s }

func (d *dashboard) addTunnel() {
	if d.store == nil {
		return
	}
	showEditDialog(d.win, "新增隧道", gui.TunnelForm{Autostart: true}, func(f gui.TunnelForm) error {
		tn, err := f.Parse()
		if err != nil {
			return err
		}
		cfg, err := d.store.Load()
		if err != nil {
			return err
		}
		if err := cfg.AddTunnel(tn); err != nil {
			return err
		}
		return d.store.Save(cfg)
	})
}

func (d *dashboard) editTunnel() {
	if d.store == nil || d.selName == "" {
		return
	}
	cfg, err := d.store.Load()
	if err != nil {
		dialog.ShowError(err, d.win)
		return
	}
	cur, ok := cfg.Tunnel(d.selName)
	if !ok {
		dialog.ShowError(fmt.Errorf("tunnel %q not found", d.selName), d.win)
		return
	}
	oldName := d.selName
	showEditDialog(d.win, "编辑隧道", gui.ToForm(cur), func(f gui.TunnelForm) error {
		tn, err := f.Parse()
		if err != nil {
			return err
		}
		c, err := d.store.Load()
		if err != nil {
			return err
		}
		if err := c.UpdateTunnel(oldName, tn); err != nil {
			return err
		}
		return d.store.Save(c)
	})
}

func (d *dashboard) deleteTunnel() {
	if d.store == nil || d.selName == "" {
		return
	}
	name := d.selName
	dialog.ShowConfirm("删除隧道", "确定删除 "+name+" ？", func(ok bool) {
		if !ok {
			return
		}
		cfg, err := d.store.Load()
		if err != nil {
			dialog.ShowError(err, d.win)
			return
		}
		if err := cfg.RemoveTunnel(name); err != nil {
			dialog.ShowError(err, d.win)
			return
		}
		if err := d.store.Save(cfg); err != nil {
			if errors.Is(err, gui.ErrReloadAfterSave) {
				dialog.ShowInformation("已删除", "配置已保存。daemon 未运行，将在它启动后生效。", d.win)
			} else {
				dialog.ShowError(err, d.win)
			}
		}
	}, d.win)
}

func (d *dashboard) run(fn func() error) {
	if err := fn(); err != nil {
		dialog.ShowError(err, d.win)
	}
}

func (d *dashboard) showLogs() {
	if d.actions == nil || d.actions.Logs == nil || d.selName == "" {
		return
	}
	lines, err := d.actions.Logs(d.selName)
	if err != nil {
		dialog.ShowError(err, d.win)
		return
	}
	t := "(no output captured)"
	if len(lines) > 0 {
		t = strings.Join(lines, "\n")
	}
	entry := widget.NewMultiLineEntry()
	entry.TextStyle = mono
	entry.SetText(t)
	entry.Disable()
	dlg := dialog.NewCustom("日志 · "+d.selName, "关闭", container.NewScroll(entry), d.win)
	dlg.Resize(fyne.NewSize(660, 420))
	dlg.Show()
}

// ---- snapshot wiring -----------------------------------------------------

// update refreshes the window with a new snapshot. Must be called on the main
// (Fyne) goroutine.
func (d *dashboard) update(snap []ipc.TunnelStatus) {
	d.updateState(snap, true)
}

// updateState records a snapshot + connection flag and refreshes the view.
func (d *dashboard) updateState(snap []ipc.TunnelStatus, connected bool) {
	d.snap = snap
	d.connected = connected
	d.refresh()
}

func (d *dashboard) refresh() {
	d.refreshBody()
	d.refreshToolbar()
	if d.summary != nil {
		if d.connected {
			d.summary.Text = gui.Summarize(d.snap)
		} else {
			d.summary.Text = "daemon 未运行"
		}
		d.summary.Refresh()
		d.tonePill(gui.OverallState(d.snap, d.connected))
	}
}

func (d *dashboard) show() { d.win.Show() }

// ---- small helpers -------------------------------------------------------

func groupHeader(name string, n int) fyne.CanvasObject {
	g := strings.ToUpper(name)
	if g == "" {
		g = "（未分组）"
	}
	lbl := text(g, 11.5, pal.text3, bold)
	cnt := text(itoa(n), 11, pal.text3, fyne.TextStyle{})
	cntBg := roundRect(pal.surface2, 999, 0, nil)
	cntPill := container.NewStack(cntBg, container.New(layoutPadXY{px: 7, py: 1}, cnt))
	return container.New(layoutPadXY{px: 4, py: 2}, container.NewHBox(lbl, cntPill))
}

func groupOrder(snap []ipc.TunnelStatus) []string {
	seen := map[string]bool{}
	var order []string
	for _, t := range snap {
		if !seen[t.Group] {
			seen[t.Group] = true
			order = append(order, t.Group)
		}
	}
	sort.Strings(order)
	return order
}

func countInGroup(snap []ipc.TunnelStatus, g string) int {
	n := 0
	for _, t := range snap {
		if t.Group == g {
			n++
		}
	}
	return n
}

func spacer(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}
