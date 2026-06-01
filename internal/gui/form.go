package gui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
)

// TunnelForm is the editable, field-per-input form of a tunnel shown in the GUI
// dialog. The split fields are merged into the config model's jump/remote/
// ssh_options on Parse and split back apart on ToForm.
type TunnelForm struct {
	Name      string
	Group     string
	LocalPort string

	DestHost string
	DestPort string

	JumpHost string
	JumpPort string
	JumpUser string
	KeyFile  string

	Via        string
	SSHOptions string // multiline key=value, excluding IdentityFile

	// RawJump carries a multi-hop jump chain that can't be shown in the single
	// jump host/user/port fields. It is preserved across an edit unless the user
	// types a jump host or via (see Parse).
	RawJump []string
}

// Parse converts the form into a config.Tunnel.
func (f TunnelForm) Parse() (config.Tunnel, error) {
	t := config.Tunnel{
		Name:   strings.TrimSpace(f.Name),
		Group:  strings.TrimSpace(f.Group),
		Local:  strings.TrimSpace(f.LocalPort),
		Remote: strings.TrimSpace(f.DestHost) + ":" + strings.TrimSpace(f.DestPort),
		Via:    strings.TrimSpace(f.Via),
	}

	switch {
	case t.Via != "":
		// via wins; no inline jump.
	case strings.TrimSpace(f.JumpHost) != "":
		host := strings.TrimSpace(f.JumpHost)
		port := strings.TrimSpace(f.JumpPort)
		if port == "" {
			port = "22"
		}
		entry := host + ":" + port
		if u := strings.TrimSpace(f.JumpUser); u != "" {
			entry = u + "@" + entry
		}
		t.Jump = []string{entry}
	case len(f.RawJump) > 0:
		t.Jump = append([]string(nil), f.RawJump...)
	}

	opts := map[string]string{}
	for _, line := range strings.Split(f.SSHOptions, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			return config.Tunnel{}, fmt.Errorf("invalid ssh option %q (want key=value)", line)
		}
		opts[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if kf := strings.TrimSpace(f.KeyFile); kf != "" {
		opts["IdentityFile"] = kf
	}
	if len(opts) > 0 {
		t.SSHOptions = opts
	}
	return t, nil
}

// Validate returns the first field error (used by the dialog's submit path so
// existing callers keep working). Field-level checks live in Check.
func (f TunnelForm) Validate() error {
	errs := Check(f)
	for _, key := range checkOrder {
		if msg := errs[key]; msg != "" {
			return fmt.Errorf("%s", msg)
		}
	}
	return nil
}

// ToForm splits a tunnel into editable form fields.
func ToForm(t config.Tunnel) TunnelForm {
	f := TunnelForm{
		Name:      t.Name,
		Group:     t.Group,
		LocalPort: t.Local,
		Via:       t.Via,
	}
	if i := strings.LastIndex(t.Remote, ":"); i >= 0 {
		f.DestHost = t.Remote[:i]
		f.DestPort = t.Remote[i+1:]
	} else {
		f.DestHost = t.Remote
	}

	switch {
	case len(t.Jump) == 1:
		u, hostport := splitJumpUser(t.Jump[0])
		f.JumpUser = u
		if i := strings.LastIndex(hostport, ":"); i >= 0 {
			f.JumpHost = hostport[:i]
			f.JumpPort = hostport[i+1:]
		} else {
			f.JumpHost = hostport
		}
	case len(t.Jump) > 1:
		f.RawJump = append([]string(nil), t.Jump...)
	}

	// Extract IdentityFile into KeyFile; keep the rest as multiline text.
	keys := make([]string, 0, len(t.SSHOptions))
	for k := range t.SSHOptions {
		if k == "IdentityFile" {
			f.KeyFile = t.SSHOptions[k]
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var opts strings.Builder
	for i, k := range keys {
		if i > 0 {
			opts.WriteByte('\n')
		}
		opts.WriteString(k + "=" + t.SSHOptions[k])
	}
	f.SSHOptions = opts.String()
	return f
}

// splitJumpUser splits "user@host:port" into (user, "host:port"). With no '@'
// the user is empty.
func splitJumpUser(s string) (user, hostport string) {
	if i := strings.Index(s, "@"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}
