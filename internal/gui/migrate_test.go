package gui

import (
	"strings"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

const sshConfigSample = `
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

func parseCfg(t *testing.T, yaml string) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cfg
}

func TestMigrateLegacyTunnel_ViaAlias(t *testing.T) {
	cfg := parseCfg(t, `
groups:
  prod:
    - {name: pg, local: "5432", remote: 10.0.1.5:5432, via: entryA}
`)
	read := func() ([]byte, error) { return []byte(sshConfigSample), nil }

	newHost, err := MigrateLegacyTunnel(cfg, "pg", read)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if newHost != "entryA" {
		t.Fatalf("newHostName = %q, want entryA", newHost)
	}

	// The tunnel is rewritten to via_host and its legacy via cleared.
	tn, _ := cfg.Tunnel("pg")
	if tn.ViaHost != "entryA" || tn.Via != "" {
		t.Fatalf("tunnel not rewritten: %+v", tn)
	}

	// entryA host captured, including ProxyJump -> jump because bastionB is also present.
	a, ok := cfg.Host("entryA")
	if !ok || a.Host != "198.51.100.7" || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" || a.Jump != "bastionB" {
		t.Fatalf("entryA host mismatch: %+v (ok=%v)", a, ok)
	}
	// bastionB pulled in as the jump target.
	b, ok := cfg.Host("bastionB")
	if !ok || b.Host != "203.0.113.9" || b.User != "userB" {
		t.Fatalf("bastionB host mismatch: %+v (ok=%v)", b, ok)
	}

	// The result must validate.
	if err := cfg.Validate(); err != nil {
		t.Fatalf("migrated config invalid: %v", err)
	}
}

func TestMigrateLegacyTunnel_ViaAlias_DropsUnimportedProxyJump(t *testing.T) {
	// ProxyJump points at an alias NOT present in ssh_config => jump left empty.
	const onlyEntry = `
Host entryA
    HostName 198.51.100.7
    Port 65522
    User userA
    ProxyJump ghostbastion
`
	cfg := parseCfg(t, `
groups:
  g:
    - {name: t1, local: "1", remote: x:5432, via: entryA}
`)
	read := func() ([]byte, error) { return []byte(onlyEntry), nil }
	if _, err := MigrateLegacyTunnel(cfg, "t1", read); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	a, _ := cfg.Host("entryA")
	if a.Jump != "" {
		t.Fatalf("jump should be empty when ProxyJump alias not imported, got %q", a.Jump)
	}
	if _, ok := cfg.Host("ghostbastion"); ok {
		t.Fatal("unresolved ProxyJump alias should not create a host")
	}
}

func TestMigrateLegacyTunnel_ViaAlias_NotFound(t *testing.T) {
	cfg := parseCfg(t, `
groups:
  g:
    - {name: t1, local: "1", remote: x:5432, via: missingalias}
`)
	read := func() ([]byte, error) { return []byte(sshConfigSample), nil }
	_, err := MigrateLegacyTunnel(cfg, "t1", read)
	if err == nil || !strings.Contains(err.Error(), "missingalias") {
		t.Fatalf("expected an error naming the missing alias, got %v", err)
	}
}
