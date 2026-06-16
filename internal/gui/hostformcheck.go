package gui

import "strings"

// hostCheckOrder is the field priority for picking a single representative host
// error (parallels checkOrder in formcheck.go).
var hostCheckOrder = []string{"name", "host", "port", "jump", "sshOptions"}

// CheckHost validates a HostForm field-by-field and returns per-field Chinese
// messages keyed "name","host","port","jump","sshOptions". It is pure so the
// dialog can call it live on every keystroke.
//
// otherNames are the names of the OTHER existing hosts (excluding the one being
// edited) used for uniqueness. jumpTargets are the names a jump may reference
// (typically all host names except the one being edited).
func CheckHost(f HostForm, otherNames, jumpTargets []string) FieldErrors {
	errs := FieldErrors{}

	name := strings.TrimSpace(f.Name)
	switch {
	case name == "":
		errs["name"] = "必填：给这台主机起个名字"
	case strings.ContainsAny(name, " \t"):
		errs["name"] = "名称里不要有空格"
	default:
		for _, o := range otherNames {
			if o == name {
				errs["name"] = "已存在同名主机"
				break
			}
		}
	}

	if strings.TrimSpace(f.Host) == "" {
		errs["host"] = "必填：主机的 IP 或域名"
	}

	// Port: empty is OK (defaults to 22); otherwise must be a plain 1–65535 port.
	if strings.TrimSpace(f.Port) != "" {
		if msg := checkPort(f.Port, true); msg != "" {
			errs["port"] = msg
		}
	}

	if jump := strings.TrimSpace(f.Jump); jump != "" {
		switch {
		case jump == name:
			errs["jump"] = "不能跳板到自己"
		default:
			found := false
			for _, t := range jumpTargets {
				if t == jump {
					found = true
					break
				}
			}
			if !found {
				errs["jump"] = "跳板要指向一台已存在的主机"
			}
		}
	}

	for _, line := range strings.Split(f.SSHOptions, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "=") {
			errs["sshOptions"] = "每行写成 key=value"
			break
		}
		if strings.Contains(line, "-p") {
			errs["sshOptions"] = "端口请填在上面的字段，不要用 -p"
			break
		}
	}

	return errs
}
