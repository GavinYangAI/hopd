package config

import (
	"reflect"
	"testing"
)

func TestMarshal_RoundTrip(t *testing.T) {
	src := `
defaults:
  ssh_options:
    ServerAliveInterval: "15"
  restart: { min: 2s, max: 60s }
groups:
  prod:
    - name: prod-db
      local: "5432"
      remote: 10.0.1.5:5432
      via: bastion
  staging:
    - name: stg-web
      local: "8080"
      remote: 127.0.0.1:80
      jump: [user@jump1, user@jump2]
      ssh_options:
        ConnectTimeout: "5"
`
	cfg, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	cfg2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-Parse: %v\n--- marshaled ---\n%s", err, out)
	}
	if !reflect.DeepEqual(cfg.Tunnels(), cfg2.Tunnels()) {
		t.Fatalf("round-trip tunnels differ:\n a=%+v\n b=%+v\n--- yaml ---\n%s",
			cfg.Tunnels(), cfg2.Tunnels(), out)
	}
	if cfg.Restart != cfg2.Restart {
		t.Fatalf("round-trip restart differ: %v vs %v", cfg.Restart, cfg2.Restart)
	}
	if !reflect.DeepEqual(cfg.defaultOpts, cfg2.defaultOpts) {
		t.Fatalf("round-trip defaults differ: %v vs %v", cfg.defaultOpts, cfg2.defaultOpts)
	}
}

func TestMarshal_AutostartRoundTrip(t *testing.T) {
	cfg, _ := Parse([]byte(`
groups:
  g:
    - {name: auto, local: "1", remote: h:1, via: x, autostart: true}
    - {name: manual, local: "2", remote: h:2, via: x}
`))
	out, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v\n%s", err, out)
	}
	if auto, _ := cfg2.Tunnel("auto"); !auto.Autostart {
		t.Fatalf("autostart=true lost in round-trip:\n%s", out)
	}
	if manual, _ := cfg2.Tunnel("manual"); manual.Autostart {
		t.Fatalf("manual tunnel should not gain autostart:\n%s", out)
	}
}

// A manual tunnel (autostart=false) should not emit an autostart line at all,
// keeping the generated YAML clean.
func TestMarshal_OmitsFalseAutostart(t *testing.T) {
	cfg, _ := Parse([]byte("groups:\n  g:\n    - {name: a, local: \"1\", remote: h:1, via: x}\n"))
	out, _ := Marshal(cfg)
	if countSub(string(out), "autostart") != 0 {
		t.Fatalf("autostart should be omitted for false:\n%s", out)
	}
}

func TestMarshal_HasHeaderComment(t *testing.T) {
	cfg, _ := Parse([]byte("groups:\n  g:\n    - {name: a, local: \"1\", remote: h:1, via: x}\n"))
	out, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 || out[0] != '#' {
		t.Fatalf("marshaled YAML should start with a header comment, got:\n%s", out)
	}
}

func TestMarshal_OmitsDefaultsFromTunnels(t *testing.T) {
	cfg, _ := Parse([]byte(`
defaults:
  ssh_options:
    ServerAliveInterval: "15"
groups:
  g:
    - {name: a, local: "1", remote: h:1, via: x}
`))
	out, _ := Marshal(cfg)
	s := string(out)
	if got := countSub(s, "ServerAliveInterval"); got != 1 {
		t.Fatalf("ServerAliveInterval appears %d times, want 1:\n%s", got, s)
	}
}

func TestMarshalHostsRoundTrip(t *testing.T) {
	src := []byte(`
hosts:
  entryA:
    host: 198.51.100.7
    port: 65522
    user: userA
    key: ~/.ssh/idA
    jump: bastionB
  bastionB:
    host: 203.0.113.9
    user: userB
groups:
  prod:
    - name: pg
      local: "5432"
      remote: 10.0.1.5:5432
      via_host: entryA
`)
	cfg, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cfg2, err := Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	a, ok := cfg2.Host("entryA")
	if !ok || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" || a.Jump != "bastionB" {
		t.Fatalf("entryA round-trip mismatch: %+v (ok=%v)", a, ok)
	}
	tun, ok := cfg2.Tunnel("pg")
	if !ok || tun.ViaHost != "entryA" {
		t.Fatalf("pg via_host round-trip mismatch: %+v (ok=%v)", tun, ok)
	}
	b, ok := cfg2.Host("bastionB")
	if !ok || b.Port != 22 {
		t.Fatalf("bastionB round-trip mismatch: %+v (ok=%v)", b, ok)
	}
}

func TestHostMutators(t *testing.T) {
	cfg, err := Parse([]byte(`
hosts:
  a: {host: h1}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: a}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cfg.AddHost("b", Host{Host: "h2", Port: 22}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := cfg.AddHost("a", Host{Host: "dup"}); err == nil {
		t.Fatal("add duplicate should fail")
	}
	if err := cfg.UpdateHost("b", Host{Host: "h2-new", Port: 2222}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if h, _ := cfg.Host("b"); h.Host != "h2-new" || h.Port != 2222 {
		t.Fatalf("update mismatch: %+v", h)
	}
	// Removing a host referenced by a tunnel must fail.
	if err := cfg.RemoveHost("a"); err == nil {
		t.Fatal("remove referenced host should fail")
	}
	// Unreferenced host removes cleanly.
	if err := cfg.RemoveHost("b"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := cfg.Host("b"); ok {
		t.Fatal("b should be gone")
	}
}

func TestAddHostNormalizesPortZero(t *testing.T) {
	cfg, err := Parse([]byte(`
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: x}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// A host added with Port 0 (e.g. from an ssh_config alias with no Port)
	// must be normalized to 22 so Validate() passes.
	if err := cfg.AddHost("x", Host{Host: "h"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if h, _ := cfg.Host("x"); h.Port != 22 {
		t.Fatalf("AddHost Port=0 should normalize to 22, got %d", h.Port)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate after AddHost(Port 0): %v", err)
	}
	// UpdateHost must normalize too.
	if err := cfg.UpdateHost("x", Host{Host: "h2"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if h, _ := cfg.Host("x"); h.Port != 22 {
		t.Fatalf("UpdateHost Port=0 should normalize to 22, got %d", h.Port)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate after UpdateHost(Port 0): %v", err)
	}
}

func countSub(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
