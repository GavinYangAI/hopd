package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GavinYangAI/hopd/internal/config"
)

func fakeSSH(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ssh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func eventually(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", d)
}

func testCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(`
defaults:
  restart: { min: 1ms, max: 5ms }
groups:
  g1:
    - { name: a, local: 15001, remote: h:80, via: bastion }
  g2:
    - { name: b, local: 15002, remote: h:80, via: bastion }
`))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func stateOf(m *Manager, name string) string {
	for _, s := range m.Status() {
		if s.Name == name {
			return s.State
		}
	}
	return ""
}

func TestManager_StartsAllDown(t *testing.T) {
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	defer m.StopAll()
	if got := len(m.Status()); got != 2 {
		t.Fatalf("Status len = %d, want 2", got)
	}
	if stateOf(m, "a") != "DOWN" {
		t.Fatalf("a should start DOWN")
	}
}

func TestManager_UpDownByGroup(t *testing.T) {
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	defer m.StopAll()
	if err := m.Up("g1"); err != nil {
		t.Fatal(err)
	}
	eventually(t, 2*time.Second, func() bool { return stateOf(m, "a") != "DOWN" })
	if stateOf(m, "b") != "DOWN" {
		t.Fatalf("b should remain DOWN when only g1 is up")
	}
	if err := m.Down("g1"); err != nil {
		t.Fatal(err)
	}
	if stateOf(m, "a") != "DOWN" {
		t.Fatalf("a should be DOWN after Down(g1)")
	}
}

func TestManager_UpAll(t *testing.T) {
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	defer m.StopAll()
	if err := m.Up("all"); err != nil {
		t.Fatal(err)
	}
	eventually(t, 2*time.Second, func() bool {
		return stateOf(m, "a") != "DOWN" && stateOf(m, "b") != "DOWN"
	})
}

func TestManager_StartAutostart(t *testing.T) {
	cfg, err := config.Parse([]byte(`
defaults:
  restart: { min: 1ms, max: 5ms }
groups:
  g1:
    - { name: auto, local: 15010, remote: h:80, via: bastion, autostart: true }
    - { name: manual, local: 15011, remote: h:80, via: bastion }
`))
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(fakeSSH(t, "sleep 30"), cfg)
	defer m.StopAll()
	m.StartAutostart()
	eventually(t, 2*time.Second, func() bool { return stateOf(m, "auto") != "DOWN" })
	if stateOf(m, "manual") != "DOWN" {
		t.Fatalf("manual tunnel should stay DOWN; autostart only starts marked tunnels")
	}
}

func TestManager_ReloadAppliesChangedRestartBounds(t *testing.T) {
	cfg1, err := config.Parse([]byte(`
defaults: { restart: { min: 1s, max: 2s } }
groups: { g: [{ name: a, local: 15050, remote: h:80, via: bastion }] }
`))
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(fakeSSH(t, "sleep 30"), cfg1)
	defer m.StopAll()
	r1 := m.runners["a"]

	cfg2, err := config.Parse([]byte(`
defaults: { restart: { min: 5s, max: 9s } }
groups: { g: [{ name: a, local: 15050, remote: h:80, via: bastion }] }
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Reload(cfg2); err != nil {
		t.Fatal(err)
	}
	if m.runners["a"] == r1 {
		t.Fatal("runner should be rebuilt when restart bounds change, else new backoff never takes effect")
	}
}

func TestManager_ReloadReusesUnchangedTunnel(t *testing.T) {
	cfg := testCfg(t)
	m := NewManager(fakeSSH(t, "sleep 30"), cfg)
	defer m.StopAll()
	r1 := m.runners["a"]
	// Same config (same bounds, same tunnels) must reuse the runner in place.
	if err := m.Reload(testCfg(t)); err != nil {
		t.Fatal(err)
	}
	if m.runners["a"] != r1 {
		t.Fatal("unchanged tunnel with unchanged bounds should keep its runner")
	}
}

func TestManager_UnknownTarget(t *testing.T) {
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	defer m.StopAll()
	if err := m.Up("nope"); err == nil {
		t.Fatalf("Up(nope) should error")
	}
}

func TestManager_LogsUnknown(t *testing.T) {
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	defer m.StopAll()
	if _, err := m.Logs("nope"); err == nil {
		t.Fatalf("Logs(nope) should error")
	}
}

func TestManagerWritesGeneratedConfig(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Parse([]byte(`
hosts:
  entryA:
    host: 198.51.100.7
    port: 65522
    user: userA
groups:
  prod:
    - name: pg
      local: "5432"
      remote: 10.0.1.5:5432
      via_host: entryA
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := NewManagerWithGenDir("/usr/bin/ssh", cfg, dir)
	_ = m
	want := filepath.Join(dir, "pg.sshcfg")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected generated config at %s: %v", want, err)
	}
	if !strings.Contains(string(data), "Host entryA") || !strings.Contains(string(data), "Port 65522") {
		t.Fatalf("generated config missing host block:\n%s", data)
	}
}

func TestReloadRegeneratesOnHostChange(t *testing.T) {
	dir := t.TempDir()
	cfg1, _ := config.Parse([]byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 65522, user: userA}
groups:
  prod:
    - {name: pg, local: "5432", remote: 10.0.1.5:5432, via_host: entryA}
`))
	m := NewManagerWithGenDir("/usr/bin/ssh", cfg1, dir)

	cfg2, _ := config.Parse([]byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 2222, user: userA}
groups:
  prod:
    - {name: pg, local: "5432", remote: 10.0.1.5:5432, via_host: entryA}
`))
	if err := m.Reload(cfg2); err != nil {
		t.Fatalf("reload: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "pg.sshcfg"))
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	if !strings.Contains(string(data), "Port 2222") {
		t.Fatalf("generated config not regenerated after host change:\n%s", data)
	}
}

func TestReloadRemovesGeneratedFileForDroppedTunnel(t *testing.T) {
	dir := t.TempDir()
	cfg1, _ := config.Parse([]byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 65522, user: userA}
groups:
  prod:
    - {name: pg, local: "5432", remote: 10.0.1.5:5432, via_host: entryA}
`))
	m := NewManagerWithGenDir("/usr/bin/ssh", cfg1, dir)
	if _, err := os.Stat(filepath.Join(dir, "pg.sshcfg")); err != nil {
		t.Fatalf("precondition: %v", err)
	}
	cfg2, _ := config.Parse([]byte(`groups: {}`))
	if err := m.Reload(cfg2); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pg.sshcfg")); !os.IsNotExist(err) {
		t.Fatalf("expected generated file removed, stat err=%v", err)
	}
}
