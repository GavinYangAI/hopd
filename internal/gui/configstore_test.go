package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

func TestConfigStore_SaveSurvivesReloadFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("groups:\n  g:\n    - {name: a, local: \"1\", remote: h:1, via: x}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewConfigStore(path, func() error { return fmt.Errorf("daemon offline") })
	cfg, _ := s.Load()
	if err := cfg.AddTunnel(cfgTunnel("b", "2")); err != nil {
		t.Fatal(err)
	}
	err := s.Save(cfg)
	if !errors.Is(err, ErrReloadAfterSave) {
		t.Fatalf("reload failure should surface as ErrReloadAfterSave, got %v", err)
	}
	// The config must be persisted despite the reload failure.
	reloaded, lerr := s.Load()
	if lerr != nil {
		t.Fatal(lerr)
	}
	if len(reloaded.Tunnels()) != 2 {
		t.Fatalf("config not persisted despite reload failure, got %d tunnels", len(reloaded.Tunnels()))
	}
}

func cfgTunnel(name, port string) config.Tunnel {
	return config.Tunnel{Name: name, Group: "g", Local: port, Remote: "h:9", Via: "x"}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestConfigStore_LoadMissingReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s := NewConfigStore(path, func() error { return nil })
	cfg, err := s.Load()
	if err != nil {
		t.Fatalf("Load missing should not error, got %v", err)
	}
	if len(cfg.Tunnels()) != 0 {
		t.Fatalf("empty config expected, got %d tunnels", len(cfg.Tunnels()))
	}
}

func TestConfigStore_SaveWritesBackupAtomicAndReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("groups:\n  g:\n    - {name: a, local: \"1\", remote: h:1, via: x}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reloads := 0
	s := NewConfigStore(path, func() error { reloads++; return nil })

	cfg, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddTunnel(cfgTunnel("b", "2")); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if !contains(string(bak), "name: a") {
		t.Fatalf("backup missing original content:\n%s", bak)
	}
	reloaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Tunnels()) != 2 {
		t.Fatalf("want 2 tunnels after save, got %d", len(reloaded.Tunnels()))
	}
	if reloads != 1 {
		t.Fatalf("reload hook called %d times, want 1", reloads)
	}
}

func TestConfigStore_SaveInvalidDoesNotReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	reloads := 0
	s := NewConfigStore(path, func() error { reloads++; return nil })
	cfg, _ := s.Load()
	cfg.AddTunnel(cfgTunnel("bad-novia", "3"))
	bad, _ := cfg.Tunnel("bad-novia")
	bad.Via = ""
	bad.Jump = nil
	cfg.UpdateTunnel("bad-novia", bad)
	if err := s.Save(cfg); err == nil {
		t.Fatal("saving invalid config should error")
	}
	if reloads != 0 {
		t.Fatal("invalid save must not trigger reload")
	}
}
