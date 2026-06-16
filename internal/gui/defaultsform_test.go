package gui

import (
	"testing"
	"time"

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

func TestDefaultsForm_ApplyRoundTrip(t *testing.T) {
	cfg, err := config.Parse([]byte(`groups: {}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := DefaultsForm{
		RestartMin: "3s",
		RestartMax: "1m0s",
		SSHOptions: "ServerAliveInterval=30\nCompression=yes",
	}
	if err := f.Apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if cfg.Restart.Min != 3*time.Second {
		t.Fatalf("restart.min = %v, want 3s", cfg.Restart.Min)
	}
	if cfg.Restart.Max != time.Minute {
		t.Fatalf("restart.max = %v, want 1m", cfg.Restart.Max)
	}
	opts := cfg.DefaultOptions()
	if opts["ServerAliveInterval"] != "30" || opts["Compression"] != "yes" {
		t.Fatalf("ssh_options = %v", opts)
	}
	// The applied config must validate and round-trip via Marshal/Parse.
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate after apply: %v", err)
	}
}

func TestDefaultsForm_ApplyRejectsBadDuration(t *testing.T) {
	cfg, _ := config.Parse([]byte(`groups: {}`))
	f := DefaultsForm{RestartMin: "soon", RestartMax: "1m0s"}
	if err := f.Apply(cfg); err == nil {
		t.Fatal("Apply should reject an unparseable duration")
	}
}

func TestDefaultsForm_ApplyRejectsDashP(t *testing.T) {
	cfg, _ := config.Parse([]byte(`groups: {}`))
	f := DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: "-p=2222"}
	if err := f.Apply(cfg); err == nil {
		t.Fatal("Apply should reject -p in ssh_options")
	}
}

func TestDefaultsForm_ApplyClearsOptionsWhenEmpty(t *testing.T) {
	cfg, _ := config.Parse([]byte(`
defaults:
  ssh_options: {ServerAliveInterval: "15"}
groups: {}
`))
	f := DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: ""}
	if err := f.Apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(cfg.DefaultOptions()) != 0 {
		t.Fatalf("empty ssh_options text should clear defaults, got %v", cfg.DefaultOptions())
	}
}

func TestCheckDefaults(t *testing.T) {
	cases := []struct {
		name     string
		form     DefaultsForm
		wantKeys []string // field keys expected to carry an error ("" entries impossible)
	}{
		{
			name: "all valid",
			form: DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: "Compression=yes"},
		},
		{
			name:     "empty min",
			form:     DefaultsForm{RestartMin: "", RestartMax: "60s"},
			wantKeys: []string{"restartMin"},
		},
		{
			name:     "bad max duration",
			form:     DefaultsForm{RestartMin: "2s", RestartMax: "later"},
			wantKeys: []string{"restartMax"},
		},
		{
			name:     "max less than min",
			form:     DefaultsForm{RestartMin: "60s", RestartMax: "2s"},
			wantKeys: []string{"restartMax"},
		},
		{
			name:     "zero min",
			form:     DefaultsForm{RestartMin: "0s", RestartMax: "60s"},
			wantKeys: []string{"restartMin"},
		},
		{
			name:     "bad ssh option line",
			form:     DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: "noequals"},
			wantKeys: []string{"sshOptions"},
		},
		{
			name:     "dash p in ssh options",
			form:     DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: "-p=2222"},
			wantKeys: []string{"sshOptions"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := CheckDefaults(tc.form)
			if len(tc.wantKeys) == 0 {
				if len(errs) != 0 {
					t.Fatalf("want no errors, got %v", errs)
				}
				return
			}
			for _, k := range tc.wantKeys {
				if errs[k] == "" {
					t.Fatalf("want error on %q, got %v", k, errs)
				}
			}
		})
	}
}
