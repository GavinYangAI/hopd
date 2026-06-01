package gui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

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

// StartDaemon launches the daemon in the background using the chosen strategy.
// The spawned process is detached into its own process group so it survives the
// GUI exiting.
func StartDaemon() error {
	hasAgent := false
	if _, statErr := os.Stat(platform.PlistPath()); statErr == nil {
		hasAgent = true
	}
	hopdPath, _ := exec.LookPath("hopd")
	cmd, args, err := daemonStartArgs(hasAgent, strconv.Itoa(os.Getuid()), hopdPath)
	if err != nil {
		return err
	}
	c := exec.Command(cmd, args...)
	c.SysProcAttr = detachAttr()
	return c.Start()
}
