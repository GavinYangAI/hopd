package guiapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/config"
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

	viaHostSel  *widget.Select // via_host picker (new model)
	hostNames   []string       // available saved host names
	onNewHost   func(after func(created string))
	viaHostCard *routeCard

	legacy      bool // editing a via/jump tunnel (read-mostly)
	migrateBtn  *widget.Button
	onMigrate   func() error
	closeDialog func()

	// "测试连接" seams (injectable for tests; defaulted in newEditForm).
	testBtn      *widget.Button
	testCfg      func() (*config.Config, error) // provides the config to test against
	testRunner   gui.CmdRunner                  // ssh runner (defaults to gui.ExecRunner)
	testConn     func(ctx context.Context, cfg *config.Config, host string, run gui.CmdRunner) gui.TestConnResult
	onTestResult func(gui.TestConnResult) // result sink (defaults to a dialog)

	route string // gui.RouteViaHost | gui.RouteDirect | gui.RouteRelay | ""

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

// newEditForm builds the guided form prefilled from f. hostNames seeds the
// via_host picker; onNewHost (may be nil) opens the host dialog and calls back
// with the created host name so the picker can refresh and select it.
func newEditForm(f gui.TunnelForm, hostNames []string, onNewHost func(after func(created string)), onMigrate func() error) *editForm {
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
		hostNames:  append([]string(nil), hostNames...),
		onNewHost:  onNewHost,
		onMigrate:  onMigrate,
	}
	ef.legacy = ef.route == gui.RouteRelay || ef.route == gui.RouteDirect
	ef.viaHostSel = widget.NewSelect(ef.hostNames, func(string) { ef.refresh() })
	// Prefill the selection by direct assignment (not SetSelected) so the
	// OnChanged callback doesn't fire ef.refresh() before ef.build(f) has created
	// the preview/caption widgets it touches. ef.build(f)+ef.refresh() below pick
	// the value up. This mirrors the Entry SetText calls that also precede build.
	if f.ViaHost != "" {
		ef.viaHostSel.Selected = f.ViaHost
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

	ef.testRunner = gui.ExecRunner
	ef.testConn = gui.TestConnection
	ef.onTestResult = func(res gui.TestConnResult) {
		win := currentWindow()
		if win == nil {
			return
		}
		if res.OK {
			dialog.ShowInformation("连接成功", "已成功连到所选主机。", win)
		} else {
			dialog.ShowError(fmt.Errorf("连接失败：%s", res.Reason), win)
		}
	}

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
	ef.viaHostCard = newRouteCard(
		"用一台已保存的主机", "推荐",
		"选一台你保存过的 SSH 主机（含端口/用户/密钥/跳板），hopd 登录它再转发端口。无需手改 ~/.ssh/config。",
		"ssh 主机  → 目标主机:端口",
		func() { ef.setRoute(gui.RouteViaHost) })
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
		container.NewGridWithColumns(3, ef.viaHostCard.root, ef.directCard.root, ef.relayCard.root),
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

	if ef.legacy {
		for _, e := range []*widget.Entry{ef.via, ef.jumpHost, ef.jumpPort, ef.jumpUser, ef.keyFile} {
			e.Disable()
		}
		ef.viaHostCard.disable()
		ef.directCard.disable()
		ef.relayCard.disable()
		// The via_host picker must be inert too, or a user could switch a legacy
		// tunnel into via_host mode and Save without going through MigrateLegacyTunnel.
		ef.viaHostSel.Disable()
		ef.migrateBtn = widget.NewButtonWithIcon("迁移为主机", theme.ContentCopyIcon(), func() { ef.doMigrate() })
		ef.migrateBtn.Importance = widget.HighImportance
		banner := legacyMigrateBanner(ef.migrateBtn)
		ef.expandBox.Objects = append([]fyne.CanvasObject{banner}, ef.expandBox.Objects...)
		ef.expandBox.Refresh()
	}

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
	ef.viaHostCard.setActive(r == gui.RouteViaHost)
	ef.directCard.setActive(r == gui.RouteDirect)
	ef.relayCard.setActive(r == gui.RouteRelay)
	ef.rebuildExpand()
	ef.refresh()
}

// rebuildExpand swaps the fields shown under the route cards.
func (ef *editForm) rebuildExpand() {
	ef.viaHostCard.setActive(ef.route == gui.RouteViaHost)
	ef.directCard.setActive(ef.route == gui.RouteDirect)
	ef.relayCard.setActive(ef.route == gui.RouteRelay)
	switch ef.route {
	case gui.RouteViaHost:
		note := infoNote("选一台已保存的主机；没有就点「+ 新建主机」。")
		newBtn := widget.NewButtonWithIcon("+ 新建主机", theme.ContentAddIcon(), ef.addNewHost)
		picker := container.NewBorder(nil, nil, nil, newBtn, ef.viaHostSel)
		ef.testBtn = widget.NewButtonWithIcon("测试连接", theme.ConfirmIcon(), ef.runTest)
		ef.expandBox.Objects = []fyne.CanvasObject{
			expandPanel(container.NewVBox(
				note,
				ef.field("主机", true, "选一台已保存的 SSH 主机", "viaHost", picker),
				container.NewHBox(ef.testBtn),
			)),
		}
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
	case gui.RouteViaHost:
		if val.ViaHost != "" {
			nodes = append(nodes, arrow(), diagNode("主机", val.ViaHost, true))
		}
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

// addNewHost opens the host dialog (via onNewHost) and, on creation, adds the
// new host to the picker and selects it.
func (ef *editForm) addNewHost() {
	if ef.onNewHost == nil {
		return
	}
	ef.onNewHost(func(created string) {
		if created == "" {
			return
		}
		ef.hostNames = append(ef.hostNames, created)
		ef.viaHostSel.Options = ef.hostNames
		ef.viaHostSel.SetSelected(created)
		ef.viaHostSel.Refresh()
		ef.refresh()
	})
}

// doMigrate runs the injected migration (which saves + reopens). On success it
// closes this dialog; the window adapter reopens on the migrated tunnel.
func (ef *editForm) doMigrate() {
	if ef.onMigrate == nil {
		return
	}
	if err := ef.onMigrate(); err != nil {
		if win := currentWindow(); win != nil {
			dialog.ShowError(err, win)
		}
		return
	}
	if ef.closeDialog != nil {
		ef.closeDialog()
	}
}

// runTest tests the connection to the currently chosen via_host. It builds the
// config to test against (testCfg), runs gui.TestConnection with the injected
// runner, and reports via onTestResult.
func (ef *editForm) runTest() {
	host := ef.viaHostSel.Selected
	if host == "" || ef.testCfg == nil {
		return
	}
	cfg, err := ef.testCfg()
	if err != nil {
		ef.onTestResult(gui.TestConnResult{Reason: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res := ef.testConn(ctx, cfg, host, ef.testRunner)
	ef.onTestResult(res)
}

// legacyMigrateBanner is the in-dialog prompt shown over a legacy tunnel's
// (disabled) fields, with the migrate action.
func legacyMigrateBanner(btn *widget.Button) fyne.CanvasObject {
	msg := widget.NewLabel("这是一条旧式隧道（via/jump）。仍可运行，但建议迁移成「已保存的主机」，以便填写端口/用户/密钥并复用。")
	msg.Wrapping = fyne.TextWrapWord
	bg := roundRect(pal.warnSoft, 10, 1, pal.warnEdge)
	body := container.NewBorder(nil, nil, nil, btn, msg)
	return container.NewStack(bg, container.New(layoutPadXY{px: 12, py: 10}, body))
}

// currentWindow returns the last open Fyne window (for showing dialogs from the
// form, which doesn't hold a window reference).
func currentWindow() fyne.Window {
	wins := fyne.CurrentApp().Driver().AllWindows()
	if len(wins) == 0 {
		return nil
	}
	return wins[len(wins)-1]
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
		ViaHost:    ef.viaHostSel.Selected,
		SSHOptions: ef.sshOptions.Text,
		Autostart:  ef.autostart.Checked,
		RawJump:    ef.rawJump,
	}
}

// showEditDialog presents the guided form modally. hostNames seeds the via_host
// picker; onNewHost (may be nil) opens the host dialog; onMigrate (may be nil)
// runs legacy migration; loadCfg (may be nil) provides the config the "测试连接"
// button tests against. onSubmit receives the edited form when the user saves;
// returning an error keeps the dialog open.
func showEditDialog(win fyne.Window, title string, initial gui.TunnelForm, hostNames []string,
	onNewHost func(after func(created string)), onMigrate func() error,
	loadCfg func() (*config.Config, error), onSubmit func(gui.TunnelForm) error) {
	ef := newEditForm(initial, hostNames, onNewHost, onMigrate)
	if loadCfg != nil {
		ef.testCfg = loadCfg
	}
	dlg := dialog.NewCustomWithoutButtons(title, ef.root, win)
	dlg.Resize(fyne.NewSize(720, 620))
	ef.onCancel = dlg.Hide
	ef.closeDialog = dlg.Hide
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
	tap    *tappable
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
	rc.tap = newTappable(inner, onTap)
	rc.root = container.NewStack(rc.bg, container.New(layoutPadXY{px: 14, py: 14}, rc.tap))
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

// disable greys a card AND makes it inert so it can't be tapped (used in legacy
// read-mostly mode). Greying alone is not enough: tappable.Tapped still fires.
func (rc *routeCard) disable() {
	if rc.tap != nil {
		rc.tap.disabled = true
	}
	rc.bg.FillColor = pal.surface2
	rc.bg.StrokeColor = pal.border
	rc.bg.StrokeWidth = 1
	rc.radio.FillColor = transparent
	rc.radio.StrokeColor = pal.text3
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
