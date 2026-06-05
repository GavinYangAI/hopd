package config

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Config {
	t.Helper()
	cfg, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return cfg
}

func TestValidate_OK(t *testing.T) {
	cfg := mustParse(t, `
groups:
  g:
    - { name: a, local: 5432, remote: h:5432, via: bastion }
    - { name: b, local: "127.0.0.1:8080", remote: h:80, jump: [j1] }
`)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidate_RestartBounds(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"min zero", `
defaults: { restart: { min: 0s, max: 60s } }
groups: { g: [{ name: a, local: "1", remote: h:1, via: x }] }
`, "restart.min"},
		{"min negative", `
defaults: { restart: { min: -1s, max: 60s } }
groups: { g: [{ name: a, local: "1", remote: h:1, via: x }] }
`, "restart.min"},
		{"max below min", `
defaults: { restart: { min: 10s, max: 5s } }
groups: { g: [{ name: a, local: "1", remote: h:1, via: x }] }
`, "restart.max"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustParse(t, tc.src)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestValidate_Errors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"duplicate name", `
groups:
  g:
    - { name: a, local: 1, remote: h:1, via: x }
  h:
    - { name: a, local: 2, remote: h:2, via: x }
`, "duplicate tunnel name"},
		{"missing target", `
groups:
  g:
    - { name: a, local: 1, remote: h:1 }
`, "must set via or jump"},
		{"bad remote", `
groups:
  g:
    - { name: a, local: 1, remote: nohost, via: x }
`, "remote must be host:port"},
		{"bad local port", `
groups:
  g:
    - { name: a, local: notaport, remote: h:1, via: x }
`, "local"},
		{"duplicate local", `
groups:
  g:
    - { name: a, local: 5000, remote: h:1, via: x }
    - { name: b, local: 5000, remote: h:2, via: x }
`, "duplicate local listen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustParse(t, tc.src)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.want)
			}
		})
	}
}
