package gui

import "strings"

// Route is the user-facing "how do we reach the target" choice in the edit
// form. It is not stored in config.Tunnel directly: "relay" maps to Via, and
// "direct" maps to an (optional) inline Jump chain.
const (
	RouteDirect = "direct" // ssh logs into the target itself (optionally via a jump host)
	RouteRelay  = "relay"  // ssh logs into a configured relay (ssh alias / via), which forwards
)

// RouteOf infers the initial route for an existing tunnel form: a tunnel with
// a via alias is a relay; anything else is treated as direct. A brand-new blank
// form (no via, no jump, no name) returns "" so the UI can force an explicit
// choice.
func RouteOf(f TunnelForm) string {
	if strings.TrimSpace(f.Via) != "" {
		return RouteRelay
	}
	if strings.TrimSpace(f.JumpHost) != "" || len(f.RawJump) > 0 {
		return RouteDirect
	}
	return ""
}

// CheckRoute validates the guided form given the currently selected route. It
// returns blocking errors (errs) and non-blocking warnings (warns), each keyed
// by field. It is pure so the dialog can call it live on every keystroke.
//
// Unlike the older Check, the "must reach via jump or alias" rule is expressed
// through the route choice: relay requires an alias; direct allows an empty
// jump host (ssh straight to the target).
func CheckRoute(route string, f TunnelForm) (errs, warns FieldErrors) {
	errs = FieldErrors{}
	warns = FieldErrors{}

	if strings.TrimSpace(f.Name) == "" {
		errs["name"] = "必填：给这条转发起个名字"
	} else if strings.ContainsAny(f.Name, " \t") {
		errs["name"] = "名称里不要有空格"
	}
	if msg := checkPort(f.LocalPort, false); msg != "" {
		errs["localPort"] = msg
	}
	if strings.TrimSpace(f.DestHost) == "" {
		errs["destHost"] = "必填：要访问的内网机器 IP 或域名"
	}
	if msg := checkPort(f.DestPort, false); msg != "" {
		errs["destPort"] = msg
	}

	switch route {
	case RouteRelay:
		if strings.TrimSpace(f.Via) == "" {
			errs["via"] = "必填：~/.ssh/config 里的中继机别名（如 bastion）"
		} else if strings.Contains(f.Via, ",") {
			warns["via"] = "检测到多级中继链，已原样保留——请确认 ssh_config 已配好。"
		}
	case RouteDirect:
		if strings.TrimSpace(f.JumpHost) != "" {
			if strings.Contains(f.JumpHost, "-p") {
				errs["jumpHost"] = "这里只填主机名，不要带 -p 参数"
			} else if strings.Contains(f.JumpHost, ",") {
				warns["jumpHost"] = "检测到多级跳板链，已原样保留——请确认每一跳都能 ssh 登录。"
			}
			if strings.TrimSpace(f.JumpPort) != "" {
				if msg := checkPort(f.JumpPort, true); msg != "" {
					errs["jumpPort"] = msg
				}
			}
		} else if len(f.RawJump) > 0 {
			warns["jumpHost"] = "已保留原有的多级跳板链。"
		}
	default:
		errs["route"] = "先选一种到达方式"
	}

	if strings.Contains(f.SSHOptions, "-p") {
		errs["sshOptions"] = "端口请填在上面的字段，不要用 -p"
	}
	return errs, warns
}
