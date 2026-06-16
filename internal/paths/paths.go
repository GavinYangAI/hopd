// Package paths resolves hopd's config directory, config file, and socket path.
package paths

import (
	"os"
	"path/filepath"
)

// ConfigDir is $XDG_CONFIG_HOME/hopd, or ~/.config/hopd as a fallback.
func ConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "hopd")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".hopd")
	}
	return filepath.Join(home, ".config", "hopd")
}

// ConfigFile is the path to config.yaml.
func ConfigFile() string { return filepath.Join(ConfigDir(), "config.yaml") }

// SocketPath is the path to the daemon control socket.
func SocketPath() string { return filepath.Join(ConfigDir(), "hopd.sock") }

// ControlDir holds ssh ControlMaster sockets.
func ControlDir() string { return filepath.Join(ConfigDir(), "cm") }

// GeneratedDir holds hopd-generated ssh config files (ssh -F targets).
func GeneratedDir() string { return filepath.Join(ConfigDir(), "generated") }
