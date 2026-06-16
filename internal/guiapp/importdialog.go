package guiapp

import (
	"errors"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
