// Package config loads and validates hopd's YAML configuration.
package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Restart controls reconnect backoff bounds.
type Restart struct {
	Min time.Duration `yaml:"min"`
	Max time.Duration `yaml:"max"`
}

// Tunnel is a single resolved forward, after defaults are merged in.
type Tunnel struct {
	Name       string
	Group      string
	Local      string            // port or addr:port
	Remote     string            // host:port
	Via        string            // ssh config Host alias (optional)
	Jump       []string          // inline -J chain (optional)
	ViaHost    string            // name of a Host entry (new model)
	SSHOptions map[string]string // merged defaults + per-tunnel
	Autostart  bool              // bring this tunnel up when the daemon starts
}

// Host is a reusable SSH endpoint: connection params for one hop. Tunnels
// reference an entry host by name via Tunnel.ViaHost; hosts chain via Jump.
type Host struct {
	Host       string            // hostname / IP (ssh HostName)
	Port       int               // ssh Port (defaults to 22 when omitted)
	User       string            // ssh User
	Key        string            // IdentityFile path; "" => ssh-agent / defaults
	Jump       string            // name of another Host entry; "" => none
	SSHOptions map[string]string // extra per-host -o options
}

// Config is the parsed configuration.
type Config struct {
	Restart     Restart
	hosts       map[string]Host // reusable SSH endpoints
	tunnels     []Tunnel        // ordered as parsed
	byName      map[string]int
	defaultOpts map[string]string // defaults.ssh_options, retained for Marshal
}

// rawConfig mirrors the on-disk YAML shape before defaults are merged.
type rawConfig struct {
	Defaults struct {
		SSHOptions map[string]yaml.Node `yaml:"ssh_options"`
		Restart    struct {
			Min string `yaml:"min"`
			Max string `yaml:"max"`
		} `yaml:"restart"`
	} `yaml:"defaults"`
	Hosts  map[string]rawHost     `yaml:"hosts"`
	Groups map[string][]rawTunnel `yaml:"groups"`
}

type rawHost struct {
	Host       string               `yaml:"host"`
	Port       int                  `yaml:"port"`
	User       string               `yaml:"user"`
	Key        string               `yaml:"key"`
	Jump       string               `yaml:"jump"`
	SSHOptions map[string]yaml.Node `yaml:"ssh_options"`
}

type rawTunnel struct {
	Name       string               `yaml:"name"`
	Local      yaml.Node            `yaml:"local"`
	Remote     string               `yaml:"remote"`
	Via        string               `yaml:"via"`
	Jump       []string             `yaml:"jump"`
	ViaHost    string               `yaml:"via_host"`
	SSHOptions map[string]yaml.Node `yaml:"ssh_options"`
	Autostart  bool                 `yaml:"autostart"`
}

// Parse parses YAML bytes into a Config, merging defaults into each tunnel.
// It does not validate semantics — call Validate for that.
func Parse(data []byte) (*Config, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	cfg := &Config{byName: map[string]int{}}

	cfg.Restart.Min = 2 * time.Second
	cfg.Restart.Max = 60 * time.Second
	if s := raw.Defaults.Restart.Min; s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("defaults.restart.min: %w", err)
		}
		cfg.Restart.Min = d
	}
	if s := raw.Defaults.Restart.Max; s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("defaults.restart.max: %w", err)
		}
		cfg.Restart.Max = d
	}

	defOpts := nodeMapToStrings(raw.Defaults.SSHOptions)
	cfg.defaultOpts = defOpts

	cfg.hosts = map[string]Host{}
	for name, rh := range raw.Hosts {
		port := rh.Port
		if port == 0 {
			port = 22
		}
		cfg.hosts[name] = Host{
			Host:       rh.Host,
			Port:       port,
			User:       rh.User,
			Key:        rh.Key,
			Jump:       rh.Jump,
			SSHOptions: nodeMapToStrings(rh.SSHOptions),
		}
	}

	groupNames := make([]string, 0, len(raw.Groups))
	for g := range raw.Groups {
		groupNames = append(groupNames, g)
	}
	sort.Strings(groupNames)

	for _, g := range groupNames {
		for _, rt := range raw.Groups[g] {
			opts := map[string]string{}
			for k, v := range defOpts {
				opts[k] = v
			}
			for k, v := range nodeMapToStrings(rt.SSHOptions) {
				opts[k] = v
			}
			t := Tunnel{
				Name:       rt.Name,
				Group:      g,
				Local:      scalarString(rt.Local),
				Remote:     rt.Remote,
				Via:        rt.Via,
				Jump:       rt.Jump,
				ViaHost:    rt.ViaHost,
				SSHOptions: opts,
				Autostart:  rt.Autostart,
			}
			cfg.byName[t.Name] = len(cfg.tunnels)
			cfg.tunnels = append(cfg.tunnels, t)
		}
	}
	return cfg, nil
}

// Tunnels returns all tunnels in parse order.
func (c *Config) Tunnels() []Tunnel { return c.tunnels }

// Tunnel looks up a tunnel by name.
func (c *Config) Tunnel(name string) (Tunnel, bool) {
	i, ok := c.byName[name]
	if !ok {
		return Tunnel{}, false
	}
	return c.tunnels[i], true
}

// Host looks up a reusable host by name.
func (c *Config) Host(name string) (Host, bool) {
	h, ok := c.hosts[name]
	return h, ok
}

// Hosts returns a copy of the host map.
func (c *Config) Hosts() map[string]Host {
	out := make(map[string]Host, len(c.hosts))
	for k, v := range c.hosts {
		out[k] = v
	}
	return out
}

// nodeMapToStrings renders YAML scalar values (int/bool/string) as strings,
// so ssh -o options keep their literal form (e.g. yes -> "yes", 15 -> "15").
func nodeMapToStrings(m map[string]yaml.Node) map[string]string {
	out := map[string]string{}
	for k, n := range m {
		out[k] = scalarString(n)
	}
	return out
}

func scalarString(n yaml.Node) string {
	return n.Value
}

// Validate checks semantic correctness across all tunnels.
func (c *Config) Validate() error {
	if c.Restart.Min <= 0 {
		return fmt.Errorf("restart.min must be > 0, got %s", c.Restart.Min)
	}
	if c.Restart.Max < c.Restart.Min {
		return fmt.Errorf("restart.max (%s) must be >= restart.min (%s)", c.Restart.Max, c.Restart.Min)
	}

	seenName := map[string]bool{}
	seenLocal := map[string]bool{}
	for _, t := range c.tunnels {
		if t.Name == "" {
			return fmt.Errorf("group %q: tunnel with empty name", t.Group)
		}
		if seenName[t.Name] {
			return fmt.Errorf("duplicate tunnel name %q", t.Name)
		}
		seenName[t.Name] = true

		if t.Via == "" && len(t.Jump) == 0 {
			return fmt.Errorf("tunnel %q: must set via or jump", t.Name)
		}
		if !strings.Contains(t.Remote, ":") {
			return fmt.Errorf("tunnel %q: remote must be host:port, got %q", t.Name, t.Remote)
		}
		laddr, lport, err := splitLocal(t.Local)
		if err != nil {
			return fmt.Errorf("tunnel %q: local: %w", t.Name, err)
		}
		key := laddr + ":" + lport
		if seenLocal[key] {
			return fmt.Errorf("duplicate local listen address %q (tunnel %q)", key, t.Name)
		}
		seenLocal[key] = true
	}
	return nil
}

// splitLocal normalizes Local into (addr, port). A bare port implies 127.0.0.1.
func splitLocal(local string) (addr, port string, err error) {
	if local == "" {
		return "", "", fmt.Errorf("local is required")
	}
	if i := strings.LastIndex(local, ":"); i >= 0 {
		addr, port = local[:i], local[i+1:]
	} else {
		addr, port = "127.0.0.1", local
	}
	if addr == "" {
		addr = "127.0.0.1"
	}
	if n, e := strconv.Atoi(port); e != nil || n <= 0 || n > 65535 {
		return "", "", fmt.Errorf("invalid port %q", port)
	}
	return addr, port, nil
}
