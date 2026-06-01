package paths

import (
	"path/filepath"
	"testing"
)

func TestPaths_HonorXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if got, want := ConfigFile(), filepath.Join(dir, "hopd", "config.yaml"); got != want {
		t.Fatalf("ConfigFile() = %q, want %q", got, want)
	}
	if got, want := SocketPath(), filepath.Join(dir, "hopd", "hopd.sock"); got != want {
		t.Fatalf("SocketPath() = %q, want %q", got, want)
	}
	if got, want := ControlDir(), filepath.Join(dir, "hopd", "cm"); got != want {
		t.Fatalf("ControlDir() = %q, want %q", got, want)
	}
}
