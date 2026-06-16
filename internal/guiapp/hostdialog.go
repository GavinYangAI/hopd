package guiapp

import (
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

// noJump is the sentinel shown in the jump Select for "no jump host".
const noJump = "（不用跳板）"

// hostForm wraps the Fyne widgets for the host editor. It mirrors editForm but
// is simpler: there is no route choice.
type hostForm struct {
	root fyne.CanvasObject

	name, host, port, user *widget.Entry
	keyFile, sshOptions    *widget.Entry
	jump                   *widget.Select

	editingName string   // "" => adding; non-empty => editing (uniqueness/jump exclude this)
	allNames    []string // every existing host name (for jump targets / uniqueness)

	captions   map[string]*captionLabel
	footStatus *canvas.Text
	saveBtn    *widget.Button
	testBtn    *widget.Button

	onSave   func()
	onCancel func()
	onTest   func()
}

// newHostForm builds the host editor prefilled from f. jumpCandidates are the
// names a jump may reference (existing hosts, excluding the one being edited).
func newHostForm(f gui.HostForm, jumpCandidates []string) *hostForm {
	hf := &hostForm{
		name:        widget.NewEntry(),
		host:        widget.NewEntry(),
		port:        widget.NewEntry(),
		user:        widget.NewEntry(),
		keyFile:     widget.NewEntry(),
		sshOptions:  widget.NewMultiLineEntry(),
		editingName: f.Name,
		allNames:    jumpCandidates,
		captions:    map[string]*captionLabel{},
	}
	hf.jump = widget.NewSelect(append([]string{noJump}, jumpCandidates...), nil)

	for _, p := range []struct {
		e  *widget.Entry
		ph string
	}{
		{hf.name, "entryA"}, {hf.host, "198.51.100.7"}, {hf.port, "22"},
		{hf.user, "ops"}, {hf.keyFile, "~/.ssh/id_ed25519"},
	} {
		p.e.SetPlaceHolder(p.ph)
	}
	hf.sshOptions.SetPlaceHolder("ServerAliveInterval=30\nCompression=yes")
	noWheelTrap(hf.name, hf.host, hf.port, hf.user, hf.keyFile)

	hf.name.SetText(f.Name)
	hf.host.SetText(f.Host)
	hf.port.SetText(f.Port)
	hf.user.SetText(f.User)
	hf.keyFile.SetText(f.KeyFile)
	hf.sshOptions.SetText(f.SSHOptions)
	if f.Jump != "" {
		hf.jump.SetSelected(f.Jump)
	} else {
		hf.jump.SetSelected(noJump)
	}

	hf.build(f)
	hf.refresh()
	return hf
}

func (hf *hostForm) build(f gui.HostForm) {
	editing := f.Name != ""
	titleStr, subStr := "新增主机", "保存一台可复用的 SSH 跳板/入口"
	if editing {
		titleStr, subStr = "编辑主机", "修改 "+f.Name+" 的连接参数"
	}
	header := container.New(layoutStackV{gap: 3},
		text(titleStr, 18, pal.text1, bold),
		text(subStr, 13, pal.text2, fyne.TextStyle{}),
	)

	keyRow := container.NewBorder(nil, nil, nil,
		widget.NewButtonWithIcon("浏览…", theme.FolderOpenIcon(), hf.pickKeyFile), hf.keyFile)

	sec1 := container.New(layoutStackV{gap: 12},
		sectionHeader(1, "基本信息", ""),
		container.NewGridWithColumns(2,
			hf.field("名称", true, "这台主机的唯一名字", "name", hf.name),
			hf.field("主机地址", true, "IP 或域名", "host", hf.host),
		),
		container.NewGridWithColumns(2,
			hf.field("端口", false, "默认 22", "port", hf.port),
			hf.field("用户名", false, "登录用户", "user", hf.user),
		),
		hf.field("密钥文件", false, "SSH 私钥路径，留空用 ssh-agent", "keyFile", keyRow),
	)

	sec2 := container.New(layoutStackV{gap: 12},
		sectionHeader(2, "跳板（可选）", ""),
		hf.field("经由主机", false, "先连这台已配好的主机，再连本机", "jump", hf.jump),
	)

	sec3 := container.New(layoutStackV{gap: 12},
		sectionHeader(3, "高级选项", ""),
		hf.field("其它 ssh 选项", false, "多行 key=value，给高级用户", "sshOptions", hf.sshOptions),
	)

	bodyCol := container.New(layoutStackV{gap: 0},
		section(sec1), sectionDivider(),
		section(sec2), sectionDivider(),
		section(sec3),
	)
	bodyScroll := container.NewVScroll(container.New(layoutPadXY{px: 20, py: 6}, bodyCol))

	hf.footStatus = text("", 12.5, pal.text2, fyne.TextStyle{})
	hf.testBtn = widget.NewButtonWithIcon("测试连接", theme.MediaPlayIcon(), func() { call(hf.onTest) })
	cancel := widget.NewButton("取消", func() { call(hf.onCancel) })
	hf.saveBtn = widget.NewButtonWithIcon(saveLabel(editing), theme.ConfirmIcon(), func() { call(hf.onSave) })
	hf.saveBtn.Importance = widget.HighImportance
	footBar := container.NewBorder(nil, nil, hf.footStatus, container.NewHBox(hf.testBtn, cancel, hf.saveBtn))
	footBg := canvas.NewRectangle(pal.barBot)
	footSep := canvas.NewRectangle(pal.border)
	footSep.SetMinSize(fyne.NewSize(0, 1))
	footer := container.NewStack(footBg,
		container.NewBorder(footSep, nil, nil, nil, container.New(layoutPadXY{px: 20, py: 12}, footBar)))

	headerArea := container.New(layoutPadXY{px: 20, py: 16}, header)
	hf.root = container.NewBorder(headerArea, footer, nil, nil, bodyScroll)

	for _, e := range []*widget.Entry{hf.name, hf.host, hf.port, hf.user, hf.keyFile, hf.sshOptions} {
		e.OnChanged = func(string) { hf.refresh() }
	}
	hf.jump.OnChanged = func(string) { hf.refresh() }
}

func (hf *hostForm) field(label string, required bool, help, key string, w fyne.CanvasObject) fyne.CanvasObject {
	lblRow := text(label, 12.5, pal.text1, fyne.TextStyle{Bold: true})
	var head fyne.CanvasObject = lblRow
	if required {
		head = container.NewHBox(lblRow, text(" *", 12.5, pal.accentH, bold))
	}
	cap := newCaption(help)
	hf.captions[key] = cap
	return container.New(layoutStackV{gap: 6}, head, w, cap.obj)
}

// pickKeyFile opens a file chooser starting at ~/.ssh and fills the key entry.
func (hf *hostForm) pickKeyFile() {
	wins := fyne.CurrentApp().Driver().AllWindows()
	if len(wins) == 0 {
		return
	}
	dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		hf.keyFile.SetText(r.URI().Path())
	}, wins[len(wins)-1])
	if home, herr := os.UserHomeDir(); herr == nil {
		if lister, lerr := storage.ListerForURI(storage.NewFileURI(filepath.Join(home, ".ssh"))); lerr == nil {
			dlg.SetLocation(lister)
		}
	}
	dlg.Show()
}

// value reads the current field values back into a gui.HostForm.
func (hf *hostForm) value() gui.HostForm {
	jump := hf.jump.Selected
	if jump == noJump {
		jump = ""
	}
	return gui.HostForm{
		Name:       hf.name.Text,
		Host:       hf.host.Text,
		Port:       hf.port.Text,
		User:       hf.user.Text,
		KeyFile:    hf.keyFile.Text,
		Jump:       jump,
		SSHOptions: hf.sshOptions.Text,
	}
}

// otherNames returns existing host names excluding the one being edited (for
// uniqueness). jumpTargets is the same set (a host may jump to any other host).
func (hf *hostForm) otherNames() []string {
	out := make([]string, 0, len(hf.allNames))
	for _, n := range hf.allNames {
		if n != hf.editingName {
			out = append(out, n)
		}
	}
	return out
}

// refresh re-runs validation and updates captions, footer, and the save button.
func (hf *hostForm) refresh() {
	errs := gui.CheckHost(hf.value(), hf.otherNames(), hf.otherNames())
	for key, cap := range hf.captions {
		cap.set(errs[key], "")
	}
	ok := len(errs) == 0
	if hf.footStatus != nil {
		if ok {
			hf.footStatus.Text = "✓ 可以保存"
			hf.footStatus.Color = pal.up
		} else {
			hf.footStatus.Text = "还有 " + itoa(len(errs)) + " 处要填/改"
			hf.footStatus.Color = pal.text2
		}
		hf.footStatus.Refresh()
	}
	if hf.saveBtn != nil {
		if ok {
			hf.saveBtn.Enable()
		} else {
			hf.saveBtn.Disable()
		}
	}
}

func (hf *hostForm) valid() bool {
	return len(gui.CheckHost(hf.value(), hf.otherNames(), hf.otherNames())) == 0
}
