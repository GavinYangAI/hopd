package gui

import (
	"strconv"
	"strings"
)

// FieldErrors maps a field key to a human (Chinese) error message.
// An absent or empty value means that field is currently valid.
type FieldErrors map[string]string

// checkOrder is the field priority used by TunnelForm.Validate to pick a single
// representative error.
var checkOrder = []string{"name", "destHost", "destPort", "localPort", "jump", "jumpUser", "jumpPort"}

// Check validates a form field-by-field and returns per-field messages. It is a
// pure function so the dialog can call it live on every keystroke.
func Check(f TunnelForm) FieldErrors {
	errs := FieldErrors{}

	if strings.TrimSpace(f.Name) == "" {
		errs["name"] = "名称不能为空"
	}
	if strings.TrimSpace(f.DestHost) == "" {
		errs["destHost"] = "填要连的目标主机"
	}
	if msg := checkPort(f.DestPort, false); msg != "" {
		errs["destPort"] = msg
	}
	if msg := checkPort(f.LocalPort, false); msg != "" {
		errs["localPort"] = msg
	}

	hasJump := strings.TrimSpace(f.JumpHost) != ""
	hasVia := strings.TrimSpace(f.Via) != ""
	hasRaw := len(f.RawJump) > 0
	if !hasJump && !hasVia && !hasRaw {
		errs["jump"] = "填跳板机，或在高级里填 ssh 别名"
	}
	if !hasVia && hasJump && strings.TrimSpace(f.JumpUser) == "" {
		errs["jumpUser"] = "填登录跳板的用户名"
	}
	// Jump port: empty is OK (defaults to 22); otherwise must be a plain port.
	// When Via is set the inline jump fields are ignored (see Parse), so skip them.
	if !hasVia && strings.TrimSpace(f.JumpPort) != "" {
		if msg := checkPort(f.JumpPort, true); msg != "" {
			errs["jumpPort"] = msg
		}
	}
	return errs
}

// checkPort validates a port string. jumpHint gives the friendlier
// "don't write -p" message for the jump-port field.
func checkPort(s string, jumpHint bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "填端口号"
	}
	if jumpHint && (strings.Contains(s, "-p") || strings.ContainsAny(s, " \t")) {
		return "只填数字，不要写 -p"
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 65535 {
		return "端口要在 1–65535"
	}
	return ""
}
