package guiapp

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// settingsForm edits the global defaults (restart bounds + ssh_options).
type settingsForm struct {
	root fyne.CanvasObject

	min, max   *widget.Entry
	sshOptions *widget.Entry

	captions map[string]*captionLabel
	saveBtn  *widget.Button

	onSave   func()
	onCancel func()
}

// newSettingsForm builds the form prefilled from cfg's defaults.
func newSettingsForm(cfg *config.Config) *settingsForm {
	df := gui.ToDefaultsForm(cfg)
	sf := &settingsForm{
		min:        widget.NewEntry(),
		max:        widget.NewEntry(),
		sshOptions: widget.NewMultiLineEntry(),
		captions:   map[string]*captionLabel{},
	}
	sf.min.SetPlaceHolder("2s")
	sf.max.SetPlaceHolder("60s")
	sf.sshOptions.SetPlaceHolder("ServerAliveInterval=15\nCompression=yes")
	noWheelTrap(sf.min, sf.max)

	sf.min.SetText(df.RestartMin)
	sf.max.SetText(df.RestartMax)
	sf.sshOptions.SetText(df.SSHOptions)

	sf.build()
	sf.refresh()
	return sf
}

func (sf *settingsForm) field(label, help, key string, entry fyne.CanvasObject) fyne.CanvasObject {
	head := text(label, 12.5, pal.text1, fyne.TextStyle{Bold: true})
	cap := newCaption(help)
	sf.captions[key] = cap
	return container.New(layoutStackV{gap: 6}, head, entry, cap.obj)
}

func (sf *settingsForm) build() {
	header := container.New(layoutStackV{gap: 3},
		text("全局设置", 18, pal.text1, bold),
		text("这些默认值会应用到所有隧道", 13, pal.text2, fyne.TextStyle{}),
	)
	body := container.New(layoutStackV{gap: 14},
		container.NewGridWithColumns(2,
			sf.field("重连最短间隔", "断线后第一次重连等待，如 2s", "restartMin", sf.min),
			sf.field("重连最长间隔", "重连退避的上限，如 60s", "restartMax", sf.max),
		),
		sf.field("默认 ssh 选项", "多行 key=value，应用到所有隧道", "sshOptions", sf.sshOptions),
	)

	cancel := widget.NewButton("取消", func() { call(sf.onCancel) })
	sf.saveBtn = widget.NewButtonWithIcon("保存", theme.ConfirmIcon(), func() { call(sf.onSave) })
	sf.saveBtn.Importance = widget.HighImportance
	foot := container.NewBorder(nil, nil, nil, container.NewHBox(cancel, sf.saveBtn))

	sf.root = container.NewBorder(
		container.New(layoutPadXY{px: 20, py: 16}, header), nil, nil, nil,
		container.New(layoutPadXY{px: 20, py: 8}, container.NewVBox(body, foot)),
	)

	for _, e := range []*widget.Entry{sf.min, sf.max, sf.sshOptions} {
		e.OnChanged = func(string) { sf.refresh() }
	}
}

// value reads the entries back into a DefaultsForm.
func (sf *settingsForm) value() gui.DefaultsForm {
	return gui.DefaultsForm{
		RestartMin: sf.min.Text,
		RestartMax: sf.max.Text,
		SSHOptions: sf.sshOptions.Text,
	}
}

func (sf *settingsForm) valid() bool {
	return len(gui.CheckDefaults(sf.value())) == 0
}

// refresh re-runs validation, updates captions, and enables/disables save.
func (sf *settingsForm) refresh() {
	errs := gui.CheckDefaults(sf.value())
	for key, cap := range sf.captions {
		cap.set(errs[key], "")
	}
	if sf.saveBtn != nil {
		if len(errs) == 0 {
			sf.saveBtn.Enable()
		} else {
			sf.saveBtn.Disable()
		}
	}
}

// showSettingsDialog presents the defaults editor modally and saves on confirm.
func showSettingsDialog(win fyne.Window, store *gui.ConfigStore) {
	cfg, err := store.Load()
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	sf := newSettingsForm(cfg)
	dlg := dialog.NewCustomWithoutButtons("全局设置", sf.root, win)
	dlg.Resize(fyne.NewSize(560, 420))
	sf.onCancel = dlg.Hide
	sf.onSave = func() {
		if !sf.valid() {
			sf.refresh()
			return
		}
		c, err := store.Load()
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		if err := sf.value().Apply(c); err != nil {
			dialog.ShowError(err, win)
			return
		}
		if err := store.Save(c); err != nil {
			if errors.Is(err, gui.ErrReloadAfterSave) {
				dlg.Hide()
				dialog.ShowInformation("已保存", "设置已保存。daemon 未运行，将在它启动后生效。", win)
				return
			}
			dialog.ShowError(err, win)
			return
		}
		dlg.Hide()
	}
	dlg.Show()
}
