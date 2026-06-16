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

func TestMigrateLegacyTunnel_ViaAlias_SlugCollision(t *testing.T) {
	// The via alias and its ProxyJump alias slug to the SAME base name
	// ("host-a"), which used to make the entry host and jump host compute the
	// same key: AddHost(jumpName) succeeded, then AddHost(entryName) failed with
	// "already exists", aborting migration AND leaving the jump host inserted
	// (violating the "on error cfg unchanged" contract).
	const collidingSSH = `
Host host(a
    HostName 198.51.100.7
    Port 65522
    User userEntry
    ProxyJump host)a

Host host)a
    HostName 203.0.113.9
    User userJump
`
	cfg := parseCfg(t, `
groups:
  g:
    - {name: t1, local: "1", remote: x:5432, via: "host(a"}
`)
	read := func() ([]byte, error) { return []byte(collidingSSH), nil }

	newHost, err := MigrateLegacyTunnel(cfg, "t1", read)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Two DISTINCT host names must have been created.
	hosts := cfg.Hosts()
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d: %v", len(hosts), hosts)
	}

	// The tunnel points at the entry host.
	tn, _ := cfg.Tunnel("t1")
	if tn.ViaHost != newHost || tn.Via != "" {
		t.Fatalf("tunnel not rewritten: %+v (newHost=%q)", tn, newHost)
	}

	// The entry host captures the via alias and chains to the jump host.
	entry, ok := cfg.Host(newHost)
	if !ok || entry.Host != "198.51.100.7" || entry.User != "userEntry" {
		t.Fatalf("entry host mismatch: %+v (ok=%v)", entry, ok)
	}
	if entry.Jump == "" || entry.Jump == newHost {
		t.Fatalf("entry.Jump should be a distinct jump host, got %q (entry=%q)", entry.Jump, newHost)
	}
	jump, ok := cfg.Host(entry.Jump)
	if !ok || jump.Host != "203.0.113.9" || jump.User != "userJump" {
		t.Fatalf("jump host mismatch: %+v (ok=%v)", jump, ok)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("migrated config invalid: %v", err)
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

func TestMigrateLegacyTunnel_InlineJump_SingleHop(t *testing.T) {
	cfg := parseCfg(t, `
groups:
  g:
    - {name: rdp, local: "13389", remote: 203.0.113.10:3389, jump: ["root@198.51.100.20:65532"]}
`)
	newHost, err := MigrateLegacyTunnel(cfg, "rdp", func() ([]byte, error) { return nil, nil })
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tn, _ := cfg.Tunnel("rdp")
	if tn.ViaHost != newHost || len(tn.Jump) != 0 {
		t.Fatalf("tunnel not rewritten: %+v (newHost=%q)", tn, newHost)
	}

	// Endpoint host = the tunnel's SSH target (the forward dest host).
	endpoint, ok := cfg.Host(newHost)
	if !ok || endpoint.Host != "203.0.113.10" || endpoint.Port != 22 {
		t.Fatalf("endpoint host mismatch: %+v (ok=%v)", endpoint, ok)
	}
	// One jump hop host capturing user/host/port.
	hop, ok := cfg.Host(endpoint.Jump)
	if !ok || hop.Host != "198.51.100.20" || hop.Port != 65532 || hop.User != "root" {
		t.Fatalf("hop host mismatch: %+v (ok=%v, jump=%q)", hop, ok, endpoint.Jump)
	}
	if hop.Jump != "" {
		t.Fatalf("single hop should have empty jump, got %q", hop.Jump)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("migrated config invalid: %v", err)
	}
}

func TestMigrateLegacyTunnel_InlineJump_MultiHop(t *testing.T) {
	cfg := parseCfg(t, `
groups:
  g:
    - {name: t1, local: "1", remote: 10.0.0.9:5432, jump: ["a@h1:22", "b@h2:2200"]}
`)
	newHost, err := MigrateLegacyTunnel(cfg, "t1", func() ([]byte, error) { return nil, nil })
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	endpoint, _ := cfg.Host(newHost)
	// endpoint -> h1 -> h2 -> ""
	h1, _ := cfg.Host(endpoint.Jump)
	if h1.Host != "h1" || h1.User != "a" || h1.Port != 22 {
		t.Fatalf("first hop mismatch: %+v", h1)
	}
	h2, _ := cfg.Host(h1.Jump)
	if h2.Host != "h2" || h2.User != "b" || h2.Port != 2200 {
		t.Fatalf("second hop mismatch: %+v", h2)
	}
	if h2.Jump != "" {
		t.Fatalf("last hop should have empty jump, got %q", h2.Jump)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("migrated config invalid: %v", err)
	}
}
