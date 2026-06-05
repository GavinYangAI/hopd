// Package platform handles OS-level integration (macOS launchd autostart).
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// plistEscaper escapes the XML metacharacters that can appear in a filesystem
// path (& < >), so a home/install dir like /Users/R&D/... can't produce a
// malformed plist (and launchctl load silently fail) or inject extra keys.
var plistEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// Label is the launchd job label.
const Label = "com.gavinyangai.hopd"

// PlistContent renders the launchd plist for the hopd daemon.
func PlistContent(execPath, logPath string) string {
	execPath = plistEscaper.Replace(execPath)
	logPath = plistEscaper.Replace(logPath)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, Label, execPath, logPath, logPath)
}

// PlistPath is the per-user LaunchAgents path for the plist.
func PlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist")
}

// Install writes the plist and loads it with launchctl.
func Install(execPath, logPath string) error {
	p := PlistPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(PlistContent(execPath, logPath)), 0o644); err != nil {
		return err
	}
	// Reload if already present, then load.
	_ = exec.Command("launchctl", "unload", p).Run()
	if out, err := exec.Command("launchctl", "load", "-w", p).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %v: %s", err, out)
	}
	return nil
}

// Uninstall unloads the job and removes the plist.
func Uninstall() error {
	p := PlistPath()
	_ = exec.Command("launchctl", "unload", p).Run()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
