package daemon

import (
	"os"
	"path/filepath"
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
