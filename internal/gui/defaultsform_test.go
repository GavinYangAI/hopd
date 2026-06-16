package gui

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

func TestToDefaultsForm(t *testing.T) {
	cfg, err := config.Parse([]byte(`
defaults:
  restart: {min: 2s, max: 60s}
  ssh_options: {ServerAliveInterval: "15", Compression: "yes"}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via: alias}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := ToDefaultsForm(cfg)
	if f.RestartMin != "2s" || f.RestartMax != "1m0s" {
		t.Fatalf("durations wrong: min=%q max=%q", f.RestartMin, f.RestartMax)
	}
	// ssh_options rendered as sorted multiline key=value.
	if f.SSHOptions != "Compression=yes\nServerAliveInterval=15" {
		t.Fatalf("ssh_options = %q", f.SSHOptions)
	}
}
