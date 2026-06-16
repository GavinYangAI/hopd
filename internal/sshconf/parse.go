package sshconf

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// ImportedHost is one named Host block parsed from a user's ~/.ssh/config,
// limited to the directives hopd's Host model can represent.
type ImportedHost struct {
	Name         string
	HostName     string
	Port         int
	User         string
	IdentityFile string
	ProxyJump    string
}

// ParseSSHConfig parses ssh_config bytes into importable hosts. Wildcard or
// multi-pattern Host lines (e.g. "Host *", "Host a b") are skipped because they
// don't map to a single named hopd host. Unknown directives are ignored.
func ParseSSHConfig(data []byte) ([]ImportedHost, error) {
	var hosts []ImportedHost
	var cur *ImportedHost
	flush := func() {
		if cur != nil && cur.Name != "" {
			hosts = append(hosts, *cur)
		}
		cur = nil
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val := splitDirective(line)
		switch strings.ToLower(key) {
		case "host":
			flush()
			patterns := strings.Fields(val)
			if len(patterns) == 1 && !strings.ContainsAny(patterns[0], "*?!") {
				cur = &ImportedHost{Name: patterns[0]}
			}
		case "hostname":
			if cur != nil {
				cur.HostName = val
			}
		case "port":
			if cur != nil {
				if n, err := strconv.Atoi(val); err == nil {
					cur.Port = n
				}
			}
		case "user":
			if cur != nil {
				cur.User = val
			}
		case "identityfile":
			if cur != nil {
				cur.IdentityFile = val
			}
		case "proxyjump":
			if cur != nil {
				cur.ProxyJump = val
			}
		}
	}
	flush()
	return hosts, sc.Err()
}

// splitDirective splits an ssh_config line into key and value, accepting both
// "Key value" and "Key=value" forms.
func splitDirective(line string) (key, val string) {
	if i := strings.IndexAny(line, " \t="); i >= 0 {
		return line[:i], strings.TrimSpace(strings.TrimLeft(line[i:], " \t="))
	}
	return line, ""
}
