package config

import (
	"testing"
	"time"
)

func TestParseHosts(t *testing.T) {
	data := []byte(`
hosts:
  bastionB:
    host: 203.0.113.9
    user: userB
  entryA:
    host: 198.51.100.7
    port: 65522
    user: userA
    key: ~/.ssh/idA
    jump: bastionB
groups:
  prod:
    - name: pg
      local: "5432"
      remote: 10.0.1.5:5432
      via_host: entryA
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, ok := cfg.Host("entryA")
	if !ok {
		t.Fatal("entryA not found")
	}
	if a.Host != "198.51.100.7" || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" || a.Jump != "bastionB" {
		t.Fatalf("entryA mismatch: %+v", a)
	}
	b, ok := cfg.Host("bastionB")
	if !ok {
		t.Fatal("bastionB not found")
	}
	if b.Port != 22 { // default applied
		t.Fatalf("bastionB default port = %d, want 22", b.Port)
	}
}

func TestParseYAML_Autostart(t *testing.T) {
	src := `
groups:
  g:
    - {name: auto, local: "1", remote: h:1, via: x, autostart: true}
    - {name: manual, local: "2", remote: h:2, via: x}
`
	cfg, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	auto, _ := cfg.Tunnel("auto")
	if !auto.Autostart {
		t.Fatal("tunnel with autostart: true should have Autostart=true")
	}
	manual, _ := cfg.Tunnel("manual")
	if manual.Autostart {
		t.Fatal("tunnel without autostart should default to Autostart=false")
	}
}

func TestParseYAML_FullConfig(t *testing.T) {
	src := `
defaults:
  ssh_options:
    ServerAliveInterval: 15
    ExitOnForwardFailure: yes
  restart: { min: 2s, max: 60s }
groups:
  prod:
    - name: prod-db
      local: 5432
      remote: 10.0.1.5:5432
      via: prod-bastion
  staging:
    - name: stg-web
      local: "127.0.0.1:8080"
      remote: 127.0.0.1:80
      jump: [user@jump1, user@jump2]
      ssh_options: { ConnectTimeout: 5 }
`
	cfg, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := len(cfg.Tunnels()); got != 2 {
		t.Fatalf("Tunnels() len = %d, want 2", got)
	}
	db, ok := cfg.Tunnel("prod-db")
	if !ok {
		t.Fatal("tunnel prod-db not found")
	}
	if db.Group != "prod" || db.Local != "5432" || db.Remote != "10.0.1.5:5432" || db.Via != "prod-bastion" {
		t.Fatalf("prod-db parsed wrong: %+v", db)
	}
	if db.SSHOptions["ServerAliveInterval"] != "15" {
		t.Fatalf("prod-db should inherit default ServerAliveInterval=15, got %q", db.SSHOptions["ServerAliveInterval"])
	}
	web, _ := cfg.Tunnel("stg-web")
	if web.Local != "127.0.0.1:8080" {
		t.Fatalf("stg-web Local = %q, want 127.0.0.1:8080", web.Local)
	}
	if len(web.Jump) != 2 || web.Jump[0] != "user@jump1" {
		t.Fatalf("stg-web Jump = %v", web.Jump)
	}
	if web.SSHOptions["ConnectTimeout"] != "5" {
		t.Fatalf("stg-web per-tunnel ConnectTimeout override missing: %v", web.SSHOptions)
	}
	if web.SSHOptions["ServerAliveInterval"] != "15" {
		t.Fatalf("stg-web should still inherit default ServerAliveInterval")
	}
	if cfg.Restart.Min != 2*time.Second || cfg.Restart.Max != 60*time.Second {
		t.Fatalf("restart = %+v, want 2s..60s", cfg.Restart)
	}
}
