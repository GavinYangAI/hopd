// Package tunnel builds ssh argument vectors and supervises ssh subprocesses.
package tunnel

import (
	"sort"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
)

// BuildArgs converts a legacy tunnel spec (via alias / inline jump) into the
// argv for `ssh` (excluding argv[0]). Options are emitted in sorted key order
// for deterministic, testable output.
func BuildArgs(t config.Tunnel) []string {
	args := []string{"-N", "-T"}
	args = append(args, optionArgs(t)...)
	if len(t.Jump) > 0 {
		args = append(args, "-J", strings.Join(t.Jump, ","))
	}
	args = append(args, "-L", localForward(t.Local, t.Remote))
	args = append(args, targetHost(t))
	return args
}

// BuildArgsVia converts a via_host tunnel into ssh argv that points ssh at the
// hopd-generated config (sshConfigPath) and connects to the entry host alias.
func BuildArgsVia(t config.Tunnel, sshConfigPath, entry string) []string {
	args := []string{"-F", sshConfigPath, "-N", "-T"}
	args = append(args, optionArgs(t)...)
	args = append(args, "-L", localForward(t.Local, t.Remote))
	args = append(args, entry)
	return args
}

// optionArgs renders t.SSHOptions as sorted -o key=value pairs.
func optionArgs(t config.Tunnel) []string {
	keys := make([]string, 0, len(t.SSHOptions))
	for k := range t.SSHOptions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		args = append(args, "-o", k+"="+t.SSHOptions[k])
	}
	return args
}

// localForward renders -L value as bindaddr:bindport:remotehost:remoteport.
// A bare local port binds 127.0.0.1.
func localForward(local, remote string) string {
	addr, port := normalizeLocal(local)
	return addr + ":" + port + ":" + remote
}

func normalizeLocal(local string) (addr, port string) {
	if i := strings.LastIndex(local, ":"); i >= 0 {
		addr, port = local[:i], local[i+1:]
		if addr == "" {
			addr = "127.0.0.1"
		}
		return addr, port
	}
	return "127.0.0.1", local
}

// targetHost is the ssh destination: the via alias when set, otherwise the
// remote host (so an inline jump chain lands on the final target's network).
func targetHost(t config.Tunnel) string {
	if t.Via != "" {
		return t.Via
	}
	if i := strings.LastIndex(t.Remote, ":"); i >= 0 {
		return t.Remote[:i]
	}
	return t.Remote
}
