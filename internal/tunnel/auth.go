package tunnel

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
)

// ControlOptions returns ssh ControlMaster options that multiplex connections
// through one authenticated master socket under controlDir. persist sets
// ControlPersist: a duration like "300" keeps the master alive in the
// background after the last client (used by `hopd auth` to pre-warm a 2FA
// session); "no" keeps the master in the foreground (used by the daemon, whose
// runner supervises the foreground ssh process — a backgrounded master would be
// misread as an exited tunnel).
func ControlOptions(controlDir, persist string) map[string]string {
	return map[string]string{
		"ControlMaster":  "auto",
		"ControlPath":    filepath.Join(controlDir, "%C"),
		"ControlPersist": persist,
	}
}

// AuthArgs builds an interactive foreground ssh argv that authenticates the
// tunnel's target and leaves a persistent ControlMaster behind. It runs the
// remote command "true" so the session exits immediately after auth.
func AuthArgs(t config.Tunnel, controlDir string) []string {
	opts := map[string]string{}
	for k, v := range t.SSHOptions {
		opts[k] = v
	}
	for k, v := range ControlOptions(controlDir, "300") {
		opts[k] = v
	}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(keys)*2+4)
	for _, k := range keys {
		args = append(args, "-o", k+"="+opts[k])
	}
	if len(t.Jump) > 0 {
		args = append(args, "-J", strings.Join(t.Jump, ","))
	}
	args = append(args, targetHost(t), "true")
	return args
}
