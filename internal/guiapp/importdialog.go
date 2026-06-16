package guiapp

import (
	"errors"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/gui"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)

// importRow is one discovered ssh_config alias and its selection checkbox.
type importRow struct {
	host  sshconf.ImportedHost
	check *widget.Check
}

// importForm holds the parsed import rows. The ssh_config bytes are supplied by
// an injectable reader so tests use a fixture and never touch the real file.
// The production reader is readUserSSHConfig (defined in window.go), which reads
// ~/.ssh/config read-only and maps a missing file to errMissingSSHConfig.
type importForm struct {
	root fyne.CanvasObject
	rows map[string]*importRow
	cfg  *config.Config
}

// newImportForm parses the ssh_config bytes from read and builds a checkbox row
// per importable alias. Aliases that already exist as hosts in cfg are
// pre-disabled (skip duplicates). A missing file yields an empty form, not an
// error.
func newImportForm(cfg *config.Config, read func() ([]byte, error)) (*importForm, error) {
	f := &importForm{rows: map[string]*importRow{}, cfg: cfg}

	data, err := read()
	if errors.Is(err, errMissingSSHConfig) {
		f.build(nil)
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	imported, err := sshconf.ParseSSHConfig(data)
	if err != nil {
		return nil, err
	}
	existing := gui.ExistingImportNames(cfg, imported)

	for _, ih := range imported {
		chk := widget.NewCheck("", nil)
		if existing[ih.Name] {
			chk.SetChecked(false)
			chk.Disable()
		}
		f.rows[ih.Name] = &importRow{host: ih, check: chk}
	}
	f.build(imported)
	return f, nil
}

// orderedNames returns the row names in a stable (sorted) order for rendering.
func (f *importForm) orderedNames() []string {
	names := make([]string, 0, len(f.rows))
	for n := range f.rows {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// build lays out the rows. imported is the parse order (may be nil for the empty
// state); rendering uses the sorted names so the layout is deterministic.
func (f *importForm) build(imported []sshconf.ImportedHost) {
	if len(f.rows) == 0 {
		f.root = container.NewVBox(
			widget.NewLabel("没有在 ~/.ssh/config 里找到可导入的主机。"),
		)
		return
	}
	var objs []fyne.CanvasObject
	objs = append(objs, widget.NewLabel("勾选要导入为 hopd 主机的条目（灰色表示已存在）："))
	for _, name := range f.orderedNames() {
		r := f.rows[name]
		desc := name + "  —  " + describeImported(r.host)
		objs = append(objs, container.NewHBox(r.check, widget.NewLabel(desc)))
	}
	f.root = container.NewVScroll(container.NewVBox(objs...))
}

// describeImported renders a one-line summary of an imported host for the row.
func describeImported(h sshconf.ImportedHost) string {
	s := h.HostName
	if h.Port != 0 {
		s += ":" + itoa(h.Port)
	}
	if h.User != "" {
		s = h.User + "@" + s
	}
	if h.IdentityFile != "" {
		s += "  key=" + h.IdentityFile
	}
	if h.ProxyJump != "" {
		s += "  jump=" + h.ProxyJump
	}
	return s
}

// selectedNames returns the names of the checked (enabled) rows.
func (f *importForm) selectedNames() []string {
	var out []string
	for _, name := range f.orderedNames() {
		if f.rows[name].check.Checked {
			out = append(out, name)
		}
	}
	return out
}

// apply builds config.Hosts from the checked rows, adds any that don't already
// exist, and saves through the store. Existing hosts are skipped (their rows are
// disabled, but apply also guards in case state changed). It returns the number
// of hosts actually imported and the store's error (including
// gui.ErrReloadAfterSave, which the dialog treats as soft — the count is still
// returned in that case so callers can report it).
func (f *importForm) apply(store *gui.ConfigStore) (int, error) {
	selected := f.selectedNames()
	if len(selected) == 0 {
		return 0, nil
	}
	hosts, err := gui.BuildHostsFromImport(importedSlice(f), selected)
	if err != nil {
		return 0, err
	}
	cfg, err := store.Load()
	if err != nil {
		return 0, err
	}
	added := 0
	for name, h := range hosts {
		if _, exists := cfg.Host(name); exists {
			continue // skip duplicates defensively
		}
		if err := cfg.AddHost(name, h); err != nil {
			return 0, err
		}
		added++
	}
	if added == 0 {
		return 0, nil
	}
	return added, store.Save(cfg)
}

// importedSlice rebuilds the []sshconf.ImportedHost from the rows so apply can
// call the pure model with the same data the dialog rendered.
func importedSlice(f *importForm) []sshconf.ImportedHost {
	out := make([]sshconf.ImportedHost, 0, len(f.rows))
	for _, name := range f.orderedNames() {
		out = append(out, f.rows[name].host)
	}
	return out
}

// showImportDialog presents the import wizard modally. On "导入所选" it applies
// the selection through the store and closes; on success it confirms how many
// hosts were imported and calls onDone (if set) so an open hosts window can
// refresh. A soft reload failure is shown as an information dialog, consistent
// with the tunnel dialog.
func showImportDialog(win fyne.Window, cfg *config.Config, store *gui.ConfigStore, onDone func()) {
	f, err := newImportForm(cfg, readUserSSHConfig)
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	dlg := dialog.NewCustomConfirm("从 ~/.ssh/config 导入", "导入所选", "取消", f.root, func(ok bool) {
		if !ok {
			return
		}
		n, err := f.apply(store)
		if err != nil {
			if errors.Is(err, gui.ErrReloadAfterSave) {
				// Saved, but the daemon couldn't be told to reload — still a success.
				call(onDone)
				dialog.ShowInformation("已导入", importedMsg(n)+" daemon 未运行，将在它启动后生效。", win)
				return
			}
			dialog.ShowError(err, win)
			return
		}
		call(onDone)
		dialog.ShowInformation("已导入", importedMsg(n), win)
	}, win)
	dlg.Resize(fyne.NewSize(560, 460))
	dlg.Show()
}

// importedMsg renders the success confirmation for n imported hosts.
func importedMsg(n int) string {
	if n == 0 {
		return "没有选择要导入的主机。"
	}
	return "已导入 " + itoa(n) + " 台主机。"
}
