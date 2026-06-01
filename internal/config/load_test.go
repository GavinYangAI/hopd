package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(p, []byte(`
groups:
  g:
    - { name: a, local: 5432, remote: h:5432, via: bastion }
`), 0o644)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Tunnels()) != 1 {
		t.Fatalf("want 1 tunnel")
	}
}

func TestLoad_InvalidFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(p, []byte(`
groups:
  g:
    - { name: a, local: 5432, remote: h:5432 }
`), 0o644) // missing via/jump -> Validate should fail
	if _, err := Load(p); err == nil {
		t.Fatalf("Load should fail validation")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatalf("Load missing file should error")
	}
}
