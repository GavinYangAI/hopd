package guiapp

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/config"
)

func TestSettingsForm_PrefillsFromConfig(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`
defaults:
  restart: {min: 3s, max: 90s}
  ssh_options: {Compression: "yes"}
groups: {}
`))
	sf := newSettingsForm(cfg)
	if sf.min.Text != "3s" {
		t.Fatalf("min = %q, want 3s", sf.min.Text)
	}
	if sf.max.Text != "1m30s" {
		t.Fatalf("max = %q, want 1m30s", sf.max.Text)
	}
	if sf.sshOptions.Text != "Compression=yes" {
		t.Fatalf("ssh_options = %q", sf.sshOptions.Text)
	}
	if !sf.valid() {
		t.Fatal("freshly prefilled valid config should be valid")
	}
}

func TestSettingsForm_InvalidDisablesSave(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))
	sf := newSettingsForm(cfg)
	sf.min.SetText("nonsense")
	sf.refresh()
	if sf.valid() {
		t.Fatal("a bad duration should make the form invalid")
	}
}

func TestSettingsForm_Value(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))
	sf := newSettingsForm(cfg)
	sf.min.SetText("5s")
	sf.max.SetText("120s")
	sf.sshOptions.SetText("ServerAliveInterval=30")
	v := sf.value()
	if v.RestartMin != "5s" || v.RestartMax != "120s" || v.SSHOptions != "ServerAliveInterval=30" {
		t.Fatalf("value = %+v", v)
	}
}
