package guiapp

import (
	"errors"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// editForm wraps the Fyne widgets for the guided tunnel editor. It exposes the
// current field values via value() and keeps a single route choice (direct vs
// relay) that drives which fields are shown and how the form is validated.
type editForm struct {
	root fyne.CanvasObject

	name, group, localPort *widget.Entry
	destHost, destPort     *widget.Entry
	jumpHost, jumpPort     *widget.Entry
	jumpUser, keyFile      *widget.Entry
	via, sshOptions        *widget.Entry
	autostart              *widget.Check
	rawJump                []string

	route string // gui.RouteDirect | gui.RouteRelay | ""

	// live-refresh targets
	captions    map[string]*captionLabel
	routeErr    *captionLabel
	previewBox  *fyne.Container
	expandBox   *fyne.Container
	directCard  *routeCard
	relayCard   *routeCard
	footStatus  *canvas.Text
	saveBtn     *widget.Button
	advBox      *fyne.Container
	advExpanded bool

	onSave   func()
	onCancel func()
}

// captionLabel is the help/error/warn line under a field.
type captionLabel struct {
	obj  *canvas.Text
	help string
}

func newCaption(help string) *captionLabel {
	c := &captionLabel{obj: text(help, 11.5, pal.text3, fyne.TextStyle{}), help: help}
	return c
}

func (c *captionLabel) set(errMsg, warnMsg string) {
	switch {
	case errMsg != "":
		c.obj.Text = errMsg
		c.obj.Color = pal.err
	case warnMsg != "":
		c.obj.Text = warnMsg
		c.obj.Color = pal.warn
	default:
		c.obj.Text = c.help
		c.obj.Color = pal.text3
	}
	c.obj.Refresh()
}

// newEditForm builds the guided form prefilled from f.
func newEditForm(f gui.TunnelForm) *editForm {
	ef := &editForm{
		name:       widget.NewEntry(),
		group:      widget.NewEntry(),
		localPort:  widget.NewEntry(),
		destHost:   widget.NewEntry(),
		destPort:   widget.NewEntry(),
		jumpHost:   widget.NewEntry(),
		jumpPort:   widget.NewEntry(),
		jumpUser:   widget.NewEntry(),
		keyFile:    widget.NewEntry(),
		via:        widget.NewEntry(),
		sshOptions: widget.NewMultiLineEntry(),
		autostart:  widget.NewCheck("开机自动连接（守护进程启动时自动建立此隧道）", nil),
		rawJump:    f.RawJump,
		route:      gui.RouteOf(f),
		captions:   map[string]*captionLabel{},
	}
	for _, p := range []struct {
		e  *widget.Entry
		ph string
	}{
		{ef.name, "win-1"}, {ef.group, "chuwu"}, {ef.localPort, "13389"},
		{ef.destHost, "203.0.113.10"}, {ef.destPort, "3389"},
		{ef.jumpHost, "留空 = 直接连目标"}, {ef.jumpPort, "22"}, {ef.jumpUser, "ops"},
		{ef.keyFile, "~/.ssh/id_ed25519"}, {ef.via, "bastion"},
	} {
		p.e.SetPlaceHolder(p.ph)
	}
	ef.sshOptions.SetPlaceHolder("ServerAliveInterval=30\nCompression=yes")

	// Stop single-line entries from swallowing the scroll wheel: by default a
	// widget.Entry wraps its content in an internal Scroll (Wrapping is
	// TextTruncateClip), which eats wheel events when the pointer is over the
	// field, so the form's outer VScroll never scrolls. Turning wrapping off and
	// Scroll to None makes the entry render its content directly, letting wheel
	// events pass through to the form scroll.
	noWheelTrap(ef.name, ef.group, ef.localPort, ef.destHost, ef.destPort,
		ef.jumpHost, ef.jumpPort, ef.jumpUser, ef.keyFile, ef.via)

	ef.name.SetText(f.Name)
	ef.group.SetText(f.Group)
	ef.localPort.SetText(f.LocalPort)
	ef.destHost.SetText(f.DestHost)
	ef.destPort.SetText(f.DestPort)
	ef.jumpHost.SetText(f.JumpHost)
	ef.jumpPort.SetText(f.JumpPort)
	ef.jumpUser.SetText(f.JumpUser)
	ef.keyFile.SetText(f.KeyFile)
	ef.via.SetText(f.Via)
	ef.sshOptions.SetText(f.SSHOptions)
	ef.autostart.SetChecked(f.Autostart)

	ef.build(f)
	ef.refresh()
	return ef
}

func (ef *editForm) build(f gui.TunnelForm) {
	editing := f.Name != ""

	// header
	titleStr := "新增隧道"
	subStr := "把一个本地端口连到内网里的服务"
	if editing {
		titleStr = "编辑隧道"
		subStr = "修改 " + f.Name + " 的转发设置"
	}
	header := container.New(layoutStackV{gap: 3},
		text(titleStr, 18, pal.text1, bold),
		text(subStr, 13, pal.text2, fyne.TextStyle{}),
	)

	// live preview banner
	ef.previewBox = container.NewHBox()
	previewLabel := text("转发预览", 11, pal.text3, bold)
	previewInner := container.NewBorder(nil, nil, container.New(layoutPadXY{px: 0, py: 6}, previewLabel), nil, ef.previewBox)
	previewBg := roundRect(pal.surface1, 11, 1, pal.border)
	preview := container.NewStack(previewBg, container.New(layoutPadXY{px: 14, py: 11}, previewInner))

	// ① basic
	sec1 := container.New(layoutStackV{gap: 12},
		sectionHeader(1, "基本信息", ""),
		container.NewGridWithColumns(3,
			ef.field("名称", true, "这条转发的唯一名字", "name", ef.name),
			ef.field("分组", false, "归类用，随意填", "group", ef.group),
			ef.field("本地端口", true, "在本机用这个端口访问", "localPort", ef.localPort),
		),
		ef.autostart,
	)

	// ② target
	sec2 := container.New(layoutStackV{gap: 12},
		sectionHeader(2, "要访问的服务", "最终目标"),
		container.NewGridWithColumns(2,
			ef.field("目标主机", true, "内网机器的 IP 或域名", "destHost", ef.destHost),
			ef.field("目标端口", true, "服务端口", "destPort", ef.destPort),
		),
	)

	// ③ guided route
	ef.routeErr = newCaption("")
	ef.routeErr.obj.Color = pal.err
	lead := widget.NewLabel("hopd 要先用 SSH 连进去，才能转发端口。下面两种方式很容易搞混——选错了就连不通，按你的情况选一个：")
	lead.Wrapping = fyne.TextWrapWord
	ef.directCard = newRouteCard(
		"目标机器我能 SSH 登录", "经跳板直达",
		"目标主机自己开放了 SSH。hopd 直接登录它（或先穿过一台跳板再登录），然后转发端口。",
		"ssh -J 跳板  user@目标主机",
		func() { ef.setRoute(gui.RouteDirect) })
	ef.relayCard = newRouteCard(
		"目标在一台中继机后面", "经中继转发",
		"目标主机不开 SSH（比如内网里的一个数据库）。hopd 登录你配好的中继机，由它转发到目标。",
		"ssh 中继别名 → 目标主机:端口",
		func() { ef.setRoute(gui.RouteRelay) })
	ef.expandBox = container.NewVBox()
	sec3 := container.New(layoutStackV{gap: 11},
		sectionHeader(3, "怎么到达它？", "关键一步"),
		lead,
		ef.routeErr.obj,
		container.NewGridWithColumns(2, ef.directCard.root, ef.relayCard.root),
		ef.expandBox,
	)

	// ④ advanced (collapsible)
	advToggle := widget.NewButtonWithIcon("高级选项", theme.MenuExpandIcon(), func() { ef.toggleAdvanced() })
	advToggle.Importance = widget.LowImportance
	advToggle.Alignment = widget.ButtonAlignLeading
	ef.advBox = container.NewVBox()
	sec4 := container.NewVBox(advToggle, ef.advBox)

	bodyCol := container.New(layoutStackV{gap: 0},
		section(sec1), sectionDivider(),
		section(sec2), sectionDivider(),
		section(sec3), sectionDivider(),
		section(sec4),
	)
	bodyScroll := container.NewVScroll(container.New(layoutPadXY{px: 20, py: 6}, bodyCol))

	// footer
	ef.footStatus = text("", 12.5, pal.text2, fyne.TextStyle{})
	cancel := widget.NewButton("取消", func() { call(ef.onCancel) })
	ef.saveBtn = widget.NewButtonWithIcon(saveLabel(editing), theme.ConfirmIcon(), func() { call(ef.onSave) })
	ef.saveBtn.Importance = widget.HighImportance
	footBar := container.NewBorder(nil, nil, ef.footStatus, container.NewHBox(cancel, ef.saveBtn))
	footBg := canvas.NewRectangle(pal.barBot)
	footSep := canvas.NewRectangle(pal.border)
	footSep.SetMinSize(fyne.NewSize(0, 1))
	footer := container.NewStack(footBg,
		container.NewBorder(footSep, nil, nil, nil, container.New(layoutPadXY{px: 20, py: 12}, footBar)))

	headerArea := container.New(layoutStackV{gap: 8},
		container.New(layoutPadXY{px: 20, py: 16}, header),
		container.New(layoutPadXY{px: 20, py: 0}, preview),
	)

	ef.rebuildExpand()
	ef.root = container.NewBorder(headerArea, footer, nil, nil, bodyScroll)

	// live validation on every change
	for _, e := range []*widget.Entry{ef.name, ef.group, ef.localPort, ef.destHost, ef.destPort,
		ef.jumpHost, ef.jumpPort, ef.jumpUser, ef.keyFile, ef.via, ef.sshOptions} {
		e.OnChanged = func(string) { ef.refresh() }
	}
}

// noWheelTrap disables a single-line entry's internal scroll so it doesn't
// capture scroll-wheel events meant for the surrounding form scroll.
func noWheelTrap(entries ...*widget.Entry) {
	for _, e := range entries {
		e.Wrapping = fyne.TextWrapOff
		e.Scroll = fyne.ScrollNone
	}
}

func saveLabel(editing bool) string {
	if editing {
		return "保存修改"
	}
	return "创建隧道"
}

// field builds one labelled input with a help/error caption underneath.
func (ef *editForm) field(label string, required bool, help, key string, entry fyne.CanvasObject) fyne.CanvasObject {
	lblRow := text(label, 12.5, pal.text1, fyne.TextStyle{Bold: true})
	var head fyne.CanvasObject = lblRow
	if required {
		head = container.NewHBox(lblRow, text(" *", 12.5, pal.accentH, bold))
	}
	cap := newCaption(help)
	ef.captions[key] = cap
	return container.New(layoutStackV{gap: 6}, head, entry, cap.obj)
}

func (ef *editForm) toggleAdvanced() {
	ef.advExpanded = !ef.advExpanded
	ef.rebuildAdvanced()
}

func (ef *editForm) rebuildAdvanced() {
	if !ef.advExpanded {
		ef.advBox.Objects = nil
		ef.advBox.Refresh()
		return
	}
	ef.advBox.Objects = []fyne.CanvasObject{
		ef.field("其它 ssh 选项", false, "多行 key=value，给高级用户", "sshOptions", ef.sshOptions),
	}
	ef.advBox.Refresh()
}

func (ef *editForm) setRoute(r string) {
	ef.route = r
	ef.directCard.setActive(r == gui.RouteDirect)
	ef.relayCard.setActive(r == gui.RouteRelay)
	ef.rebuildExpand()
	ef.refresh()
}

// rebuildExpand swaps the fields shown under the route cards.
func (ef *editForm) rebuildExpand() {
	ef.directCard.setActive(ef.route == gui.RouteDirect)
	ef.relayCard.setActive(ef.route == gui.RouteRelay)
	switch ef.route {
	case gui.RouteDirect:
		note := infoNote("跳板机（可选）——目标能直接 ssh 就留空；要先过一台跳板才填。")
		keyRow := container.NewBorder(nil, nil, nil,
			widget.NewButtonWithIcon("浏览…", theme.FolderOpenIcon(), ef.pickKeyFile), ef.keyFile)
		grid := container.NewGridWithColumns(2,
			ef.field("跳板主机", false, "中间这台机的 IP / 域名", "jumpHost", ef.jumpHost),
			ef.field("端口", false, "默认 22", "jumpPort", ef.jumpPort),
			ef.field("用户名", false, "登录跳板的用户", "jumpUser", ef.jumpUser),
			ef.field("密钥文件", false, "SSH 私钥路径", "keyFile", keyRow),
		)
		ef.expandBox.Objects = []fyne.CanvasObject{expandPanel(container.NewVBox(note, grid))}
	case gui.RouteRelay:
		note := infoNote("中继机要先在 ~/.ssh/config 里配成一个 Host，这里填它的别名。")
		ef.expandBox.Objects = []fyne.CanvasObject{
			expandPanel(container.NewVBox(note, ef.field("中继机别名 (via)", true, "对应 ~/.ssh/config 里的 Host 名", "via", ef.via))),
		}
	default:
		ef.expandBox.Objects = nil
	}
	ef.expandBox.Refresh()
}

// refresh re-runs validation and updates captions, preview, footer and save.
func (ef *editForm) refresh() {
	val := ef.value()
	errs, warns := gui.CheckRoute(ef.route, val)

	for key, cap := range ef.captions {
		cap.set(errs[key], warns[key])
	}
	if ef.routeErr != nil {
		ef.routeErr.obj.Text = errs["route"]
		if errs["route"] != "" {
			ef.routeErr.obj.Show()
		} else {
			ef.routeErr.obj.Hide()
		}
		ef.routeErr.obj.Refresh()
	}

	ef.rebuildPreview(val)

	ok := len(errs) == 0
	if ef.footStatus != nil {
		if ok {
			ef.footStatus.Text = "✓ 可以保存"
			ef.footStatus.Color = pal.up
		} else {
			ef.footStatus.Text = "还有 " + itoa(len(errs)) + " 处要填/改"
			ef.footStatus.Color = pal.text2
		}
		ef.footStatus.Refresh()
	}
	if ef.saveBtn != nil {
		if ok {
			ef.saveBtn.Enable()
		} else {
			ef.saveBtn.Disable()
		}
	}
}

func (ef *editForm) valid() bool {
	errs, _ := gui.CheckRoute(ef.route, ef.value())
	return len(errs) == 0
}

func (ef *editForm) rebuildPreview(val gui.TunnelForm) {
	nodes := []fyne.CanvasObject{diagNode("本机", ":"+valueOr(val.LocalPort, "—"), false)}
	switch ef.route {
	case gui.RouteRelay:
		if val.Via != "" {
			nodes = append(nodes, arrow(), diagNode("中继", val.Via, true))
		}
	case gui.RouteDirect:
		if val.JumpHost != "" {
			nodes = append(nodes, arrow(), diagNode("跳板", val.JumpHost, true))
		}
	}
	target := valueOr(val.DestHost, "—") + ":" + valueOr(val.DestPort, "—")
	svc := "目标"
	if val.DestPort != "" {
		if name, ok := svcByPort[val.DestPort]; ok {
			svc = name
		}
	}
	nodes = append(nodes, arrow(), diagNode(svc, target, false))
	ef.previewBox.Objects = nodes
	ef.previewBox.Refresh()
}

// pickKeyFile opens a file chooser starting at ~/.ssh and fills the key entry.
func (ef *editForm) pickKeyFile() {
	win := fyne.CurrentApp().Driver().AllWindows()
	if len(win) == 0 {
		return
	}
	dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		ef.keyFile.SetText(r.URI().Path())
	}, win[len(win)-1])
	if home, herr := os.UserHomeDir(); herr == nil {
		if lister, lerr := storage.ListerForURI(storage.NewFileURI(filepath.Join(home, ".ssh"))); lerr == nil {
			dlg.SetLocation(lister)
		}
	}
	dlg.Show()
}

// value reads the current field values back into a TunnelForm.
func (ef *editForm) value() gui.TunnelForm {
	return gui.TunnelForm{
		Name:       ef.name.Text,
		Group:      ef.group.Text,
		LocalPort:  ef.localPort.Text,
		DestHost:   ef.destHost.Text,
		DestPort:   ef.destPort.Text,
		JumpHost:   ef.jumpHost.Text,
		JumpPort:   ef.jumpPort.Text,
		JumpUser:   ef.jumpUser.Text,
		KeyFile:    ef.keyFile.Text,
		Via:        ef.via.Text,
		SSHOptions: ef.sshOptions.Text,
		Autostart:  ef.autostart.Checked,
		RawJump:    ef.rawJump,
	}
}

// showEditDialog presents the guided form modally. onSubmit receives the edited
// form when the user saves; returning an error keeps the dialog open.
func showEditDialog(win fyne.Window, title string, initial gui.TunnelForm, onSubmit func(gui.TunnelForm) error) {
	ef := newEditForm(initial)
	dlg := dialog.NewCustomWithoutButtons(title, ef.root, win)
	dlg.Resize(fyne.NewSize(640, 600))
	ef.onCancel = dlg.Hide
	ef.onSave = func() {
		if !ef.valid() {
			ef.refresh()
			return
		}
		if err := onSubmit(ef.value()); err != nil {
			if errors.Is(err, gui.ErrReloadAfterSave) {
				// Config saved; the daemon just isn't running. Close and inform
				// gently instead of showing a red error over a successful save.
				dlg.Hide()
				dialog.ShowInformation("已保存", "配置已保存。daemon 未运行，将在它启动后生效。", win)
				return
			}
			dialog.ShowError(err, win)
			return
		}
		dlg.Hide()
	}
	dlg.Show()
}

// ---- guided-route card ---------------------------------------------------

type routeCard struct {
	root   *fyne.Container
	bg     *canvas.Rectangle
	radio  *canvas.Circle
	active bool
}

func newRouteCard(title, badge, desc, example string, onTap func()) *routeCard {
	rc := &routeCard{}
	rc.radio = canvas.NewCircle(transparent)
	rc.radio.StrokeColor = pal.text3
	rc.radio.StrokeWidth = 1.6
	radioWrap := container.NewGridWrap(fyne.NewSize(16, 16), rc.radio)

	titleRow := container.NewHBox(
		text(title, 13.5, pal.text1, bold),
		badgeChip(badge),
	)
	descLbl := widget.NewLabel(desc)
	descLbl.Wrapping = fyne.TextWrapWord
	egBg := roundRect(pal.surface2, 6, 0, nil)
	eg := container.NewStack(egBg, container.New(layoutPadXY{px: 8, py: 5}, text(example, 11, pal.text3, mono)))
	col := container.NewVBox(titleRow, descLbl, eg)

	rc.bg = roundRect(pal.surface1, 12, 1, pal.border)
	inner := container.NewBorder(nil, nil, container.New(layoutPadXY{px: 0, py: 2}, radioWrap), nil, col)
	rc.root = container.NewStack(rc.bg, container.New(layoutPadXY{px: 14, py: 14},
		newTappable(inner, onTap)))
	return rc
}

func (rc *routeCard) setActive(on bool) {
	rc.active = on
	if on {
		rc.bg.FillColor = pal.accentBg
		rc.bg.StrokeColor = pal.accent
		rc.bg.StrokeWidth = 2
		rc.radio.FillColor = pal.accent
		rc.radio.StrokeColor = pal.accent
	} else {
		rc.bg.FillColor = pal.surface1
		rc.bg.StrokeColor = pal.border
		rc.bg.StrokeWidth = 1
		rc.radio.FillColor = transparent
		rc.radio.StrokeColor = pal.text3
	}
	rc.bg.Refresh()
	rc.radio.Refresh()
}

// ---- section chrome ------------------------------------------------------

func sectionHeader(n int, title, hint string) fyne.CanvasObject {
	num := text(itoa(n), 12, pal.accentH, bold)
	numBg := roundRect(pal.accentSoft, 999, 0, nil)
	numBadge := container.NewGridWrap(fyne.NewSize(21, 21), container.NewStack(numBg, container.NewCenter(num)))
	items := []fyne.CanvasObject{numBadge, text(title, 14, pal.text1, bold)}
	if hint != "" {
		items = append(items, badgeChipMuted(hint))
	}
	return container.NewHBox(items...)
}

func section(content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layoutPadXY{px: 0, py: 14}, content)
}

func sectionDivider() fyne.CanvasObject {
	line := canvas.NewRectangle(pal.border)
	line.SetMinSize(fyne.NewSize(0, 1))
	return line
}

func expandPanel(content fyne.CanvasObject) fyne.CanvasObject {
	bg := roundRect(pal.surface1, 11, 1, pal.borderStrong)
	return container.New(layoutPadXY{px: 0, py: 4}, container.NewStack(bg, container.New(layoutPadXY{px: 14, py: 14}, content)))
}

func infoNote(s string) fyne.CanvasObject {
	lbl := widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{})
	lbl.Wrapping = fyne.TextWrapWord
	ic := widget.NewIcon(theme.InfoIcon())
	return container.NewBorder(nil, nil, ic, nil, lbl)
}

func badgeChip(s string) fyne.CanvasObject {
	bg := roundRect(pal.accentSoft, 5, 0, nil)
	return container.NewStack(bg, container.New(layoutPadXY{px: 6, py: 2}, text(s, 10.5, pal.accentH, bold)))
}

func badgeChipMuted(s string) fyne.CanvasObject {
	bg := roundRect(pal.surface2, 5, 0, nil)
	return container.NewStack(bg, container.New(layoutPadXY{px: 7, py: 2}, text(s, 11, pal.text3, bold)))
}
