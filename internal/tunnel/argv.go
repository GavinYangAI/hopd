// Package tunnel builds ssh argument vectors and supervises ssh subprocesses.
package tunnel

import (
	"sort"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
)

// BuildArgs converts a tunnel spec into the argv for `ssh` (excluding argv[0]).
// Options are emitted in sorted key order for deterministic, testable output.
func BuildArgs(t config.Tunnel) []string {
	args := []string{"-N", "-T"}

	keys := make([]string, 0, len(t.SSHOptions))
	for k := range t.SSHOptions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-o", k+"="+t.SSHOptions[k])
	}

	if len(t.Jump) > 0 {
		args = append(args, "-J", strings.Join(t.Jump, ","))
	}

	args = append(args, "-L", localForward(t.Local, t.Remote))
	args = append(args, targetHost(t))
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
