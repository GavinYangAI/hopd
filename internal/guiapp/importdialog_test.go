package guiapp

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/config"
)

const fixtureSSHConfig = `
Host *
    ServerAliveInterval 30

Host entryA
    HostName 198.51.100.7
    Port 65522
    User userA
    IdentityFile ~/.ssh/idA
    ProxyJump bastionB

Host bastionB
    HostName 203.0.113.9
    User userB
`

func TestImportForm_RendersRowsFromFixture(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))

	f, err := newImportForm(cfg, func() ([]byte, error) { return []byte(fixtureSSHConfig), nil })
	if err != nil {
		t.Fatalf("newImportForm: %v", err)
	}
	// Wildcard "Host *" is skipped by the parser; two named rows remain.
	if len(f.rows) != 2 {
		t.Fatalf("got %d rows, want 2 (wildcard skipped)", len(f.rows))
	}
	if _, ok := f.rows["entryA"]; !ok {
		t.Fatalf("entryA row missing: %v", f.rows)
	}
}

func TestImportForm_PreDisablesExistingHosts(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`
hosts:
  bastionB: {host: 203.0.113.9, user: userB}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: bastionB}
`))
	f, err := newImportForm(cfg, func() ([]byte, error) { return []byte(fixtureSSHConfig), nil })
	if err != nil {
		t.Fatalf("newImportForm: %v", err)
	}
	if !f.rows["bastionB"].check.Disabled() {
		t.Fatal("bastionB already exists; its checkbox should be disabled")
	}
	if f.rows["entryA"].check.Disabled() {
		t.Fatal("entryA does not exist; its checkbox should be enabled")
	}
}

func TestImportForm_SelectedNames(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))
	f, _ := newImportForm(cfg, func() ([]byte, error) { return []byte(fixtureSSHConfig), nil })

	f.rows["entryA"].check.SetChecked(true)
	got := f.selectedNames()
	if len(got) != 1 || got[0] != "entryA" {
		t.Fatalf("selectedNames = %v, want [entryA]", got)
	}
}

func TestImportForm_EmptyWhenNoFile(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))
	// Missing file: reader returns os.ErrNotExist-style; the form should
	// construct with zero rows, not error.
	f, err := newImportForm(cfg, func() ([]byte, error) { return nil, errMissingSSHConfig })
	if err != nil {
		t.Fatalf("missing file should be a friendly empty state, got err: %v", err)
	}
	if len(f.rows) != 0 {
		t.Fatalf("want 0 rows for missing file, got %d", len(f.rows))
	}
}
