package gui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
)

// HostForm is the editable, field-per-input form of a reusable SSH host shown in
// the GUI host dialog. It mirrors TunnelForm: split string fields are merged into
// the config.Host model on Parse and split back apart on ToHostForm.
type HostForm struct {
	Name       string
	Host       string
	Port       string
	User       string
	KeyFile    string // -> Host.Key (IdentityFile path)
	Jump       string // name of another host entry; "" => none
	SSHOptions string // multiline key=value
}

// Parse converts the form into (name, config.Host). An empty port defaults to
// 22. SSHOptions is parsed line-by-line as key=value; a malformed line errors.
func (f HostForm) Parse() (string, config.Host, error) {
	name := strings.TrimSpace(f.Name)
	h := config.Host{
		Host: strings.TrimSpace(f.Host),
		Port: 22,
		User: strings.TrimSpace(f.User),
		Key:  strings.TrimSpace(f.KeyFile),
		Jump: strings.TrimSpace(f.Jump),
	}
	if p := strings.TrimSpace(f.Port); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return "", config.Host{}, fmt.Errorf("invalid port %q", p)
		}
		h.Port = n
	}
	opts := map[string]string{}
	for _, line := range strings.Split(f.SSHOptions, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			return "", config.Host{}, fmt.Errorf("invalid ssh option %q (want key=value)", line)
		}
		opts[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if len(opts) > 0 {
		h.SSHOptions = opts
	}
	return name, h, nil
}

// ToHostForm splits a config.Host into editable form fields. Port 22 (and the
// zero value) render as "" so the field shows its default placeholder, matching
// how the tunnel form leaves an implicit jump port blank. SSHOptions render as
// sorted multiline key=value.
func ToHostForm(name string, h config.Host) HostForm {
	f := HostForm{
		Name:    name,
		Host:    h.Host,
		User:    h.User,
		KeyFile: h.Key,
		Jump:    h.Jump,
	}
	if h.Port != 0 && h.Port != 22 {
		f.Port = strconv.Itoa(h.Port)
	}
	keys := make([]string, 0, len(h.SSHOptions))
	for k := range h.SSHOptions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k + "=" + h.SSHOptions[k])
	}
	f.SSHOptions = b.String()
	return f
}
