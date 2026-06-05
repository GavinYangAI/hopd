package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/GavinYangAI/hopd/internal/paths"
	"github.com/GavinYangAI/hopd/internal/platform"
)

// daemonStartArgs picks how to start the daemon. With a LaunchAgent installed,
// it kickstarts the launchd job; otherwise it runs `hopd daemon` from the
// resolved binary path. hopdPath == "" means hopd was not found on PATH.
func daemonStartArgs(hasLaunchAgent bool, uid, hopdPath string) (cmd string, args []string, err error) {
	if hasLaunchAgent {
		return "launchctl", []string{"kickstart", "-k", "gui/" + uid + "/" + platform.Label}, nil
	}
	if hopdPath == "" {
		return "", nil, fmt.Errorf("hopd not found: install the launchd agent (hopd install) or put hopd on your PATH")
	}
	return hopdPath, []string{"daemon"}, nil
}

// locateHopdWith finds the hopd binary: PATH first, then well-known install
// locations. A GUI app launched from Finder/the menu bar inherits a minimal
// PATH that usually excludes /usr/local/bin, so LookPath alone often misses a
// perfectly installed hopd. lookPath and isExec are injected for testing.
func locateHopdWith(lookPath func(string) (string, error), isExec func(string) bool, candidates []string) string {
	if p, err := lookPath("hopd"); err == nil {
		return p
	}
	for _, c := range candidates {
		if isExec(c) {
			return c
		}
	}
	return ""
}

// hopdCandidates lists well-known hopd install locations in priority order,
// including the GUI app's own directory (handy for a side-by-side dev build).
func hopdCandidates(selfDir, home string) []string {
	return []string{
		filepath.Join(selfDir, "hopd"),
		"/usr/local/bin/hopd",
		"/opt/homebrew/bin/hopd",
		filepath.Join(home, "bin", "hopd"),
		filepath.Join(home, "go", "bin", "hopd"),
	}
}

// isExecutable reports whether path is a regular, executable file.
func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}

// locateHopd finds the hopd binary on PATH or in well-known locations.
func locateHopd() string {
	selfDir := ""
	if exe, err := os.Executable(); err == nil {
		selfDir = filepath.Dir(exe)
	}
	home, _ := os.UserHomeDir()
	return locateHopdWith(exec.LookPath, isExecutable, hopdCandidates(selfDir, home))
}

// InstallAgent installs and loads the launchd autostart agent so the daemon
// starts at login. It locates hopd itself (the GUI's minimal PATH usually can't),
// returning a clear, actionable error when hopd cannot be found.
func InstallAgent() error {
	hopdPath := locateHopd()
	if hopdPath == "" {
		return fmt.Errorf("找不到 hopd 命令：请先安装 hopd（例如把二进制放到 /usr/local/bin，或 brew 安装），再开启开机自启")
	}
	logPath := filepath.Join(paths.ConfigDir(), "hopd.log")
	if err := os.MkdirAll(paths.ConfigDir(), 0o700); err != nil {
		return err
	}
	return platform.Install(hopdPath, logPath)
}

// StartDaemon launches the daemon in the background using the chosen strategy.
// The spawned process is detached into its own process group so it survives the
// GUI exiting.
func StartDaemon() error {
	hasAgent := false
	if _, statErr := os.Stat(platform.PlistPath()); statErr == nil {
		hasAgent = true
	}
	hopdPath := locateHopd()
	cmd, args, err := daemonStartArgs(hasAgent, strconv.Itoa(os.Getuid()), hopdPath)
	if err != nil {
		return err
	}
	c := exec.Command(cmd, args...)
	c.SysProcAttr = detachAttr()
	return c.Start()
}
