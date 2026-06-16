package gui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)

// MigrateLegacyTunnel rewrites a legacy via/jump tunnel into the new via_host
// model, synthesizing config Host entries for the SSH endpoint(s) it needs, and
// returns the name of the entry host the tunnel now points at.
//
//   - via: <alias>  — parse ssh_config (via readSSHConfig) for that alias, create
//     a host capturing HostName/Port/User/IdentityFile, map ProxyJump -> jump
//     when the referenced alias is ALSO present, then set ViaHost and clear Via.
//   - inline Jump   — synthesize one host per hop plus the endpoint; see Task 5.
//
// readSSHConfig is injected so callers (and tests) control where ssh_config text
// comes from; the real GUI passes a reader for ~/.ssh/config. cfg is mutated in
// place; on error cfg is left unchanged.
func MigrateLegacyTunnel(cfg *config.Config, tunnelName string, readSSHConfig func() ([]byte, error)) (newHostName string, err error) {
	t, ok := cfg.Tunnel(tunnelName)
	if !ok {
		return "", fmt.Errorf("tunnel %q not found", tunnelName)
	}
	if t.ViaHost != "" {
		return "", fmt.Errorf("tunnel %q is already on the host model", tunnelName)
	}
	switch {
	case strings.TrimSpace(t.Via) != "":
		return migrateVia(cfg, t, readSSHConfig)
	case len(t.Jump) > 0:
		return migrateJump(cfg, t)
	default:
		return "", fmt.Errorf("tunnel %q has no via or jump to migrate", tunnelName)
	}
}

// migrateVia handles the `via: <alias>` case.
func migrateVia(cfg *config.Config, t config.Tunnel, readSSHConfig func() ([]byte, error)) (string, error) {
	data, err := readSSHConfig()
	if err != nil {
		return "", fmt.Errorf("read ssh config: %w", err)
	}
	imported, err := sshconf.ParseSSHConfig(data)
	if err != nil {
		return "", fmt.Errorf("parse ssh config: %w", err)
	}
	byName := map[string]sshconf.ImportedHost{}
	for _, h := range imported {
		byName[h.Name] = h
	}
	alias := strings.TrimSpace(t.Via)
	ih, ok := byName[alias]
	if !ok {
		return "", fmt.Errorf("ssh config has no Host %q to migrate", alias)
	}

	// Reserve a unique name for the entry host first, so a jump dependency that
	// resolves to the same slug can't collide with it.
	entryName := uniqueHostName(cfg, alias)

	// Pull in the ProxyJump target only when it is also importable.
	jump := ""
	if pj := strings.TrimSpace(ih.ProxyJump); pj != "" {
		if jih, ok := byName[pj]; ok {
			jumpName := uniqueHostName(cfg, pj)
			if addErr := cfg.AddHost(jumpName, importedToHost(jih, "")); addErr != nil {
				return "", addErr
			}
			jump = jumpName
		}
	}

	if addErr := cfg.AddHost(entryName, importedToHost(ih, jump)); addErr != nil {
		return "", addErr
	}

	t.Via = ""
	t.Jump = nil
	t.ViaHost = entryName
	if err := cfg.UpdateTunnel(t.Name, t); err != nil {
		return "", err
	}
	return entryName, nil
}

// importedToHost converts a parsed ssh_config Host into a config.Host, applying
// the resolved jump name (which may be "").
func importedToHost(ih sshconf.ImportedHost, jump string) config.Host {
	port := ih.Port
	if port == 0 {
		port = 22
	}
	return config.Host{
		Host: ih.HostName,
		Port: port,
		User: ih.User,
		Key:  ih.IdentityFile,
		Jump: jump,
	}
}

// uniqueHostName turns a desired name into a config-safe, currently-unused host
// key: it slugs the input and, on collision with an existing host, suffixes
// -2, -3, … until free.
func uniqueHostName(cfg *config.Config, desired string) string {
	base := slug(desired)
	if base == "" {
		base = "host"
	}
	if _, taken := cfg.Host(base); !taken {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if _, taken := cfg.Host(cand); !taken {
			return cand
		}
	}
}

// slug keeps letters, digits, '-', '_' and '.', replacing every other run of
// characters with a single '-', so an alias or host:port becomes a host key.
func slug(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// migrateJump handles an inline `jump:` chain. It builds one host per hop plus
// an endpoint host for the tunnel's own SSH target (the forward dest host, which
// legacy inline-jump ssh-es into directly), linking endpoint -> hop1 -> ... ->
// hopN. The tunnel's ViaHost becomes the endpoint host.
func migrateJump(cfg *config.Config, t config.Tunnel) (string, error) {
	destHost, _ := splitHostPort(t.Remote)
	if destHost == "" {
		return "", fmt.Errorf("tunnel %q: cannot determine SSH endpoint from remote %q", t.Name, t.Remote)
	}

	// Name and reserve every host up front so later AddHosts can't collide.
	endpointName := uniqueHostName(cfg, slug(destHost)+"-entry")
	endpoint := config.Host{Host: destHost, Port: 22}
	if err := cfg.AddHost(endpointName, endpoint); err != nil {
		return "", err
	}

	// Build hop hosts in order; link each to the next via Jump.
	hopNames := make([]string, len(t.Jump))
	hops := make([]config.Host, len(t.Jump))
	for i, raw := range t.Jump {
		user, hostport := splitJumpUser(raw)
		host, portStr := splitHostPort(hostport)
		port := 22
		if portStr != "" {
			if n, err := strconv.Atoi(portStr); err == nil {
				port = n
			}
		}
		hopNames[i] = uniqueHostName(cfg, host)
		hops[i] = config.Host{Host: host, Port: port, User: user}
		// Reserve the name immediately so the next iteration's uniqueHostName
		// sees it as taken.
		if err := cfg.AddHost(hopNames[i], hops[i]); err != nil {
			return "", err
		}
	}
	// Link the chain: endpoint -> hop0 -> hop1 -> ... (now that names are stable).
	for i := range hopNames {
		next := ""
		if i+1 < len(hopNames) {
			next = hopNames[i+1]
		}
		h := hops[i]
		h.Jump = next
		if err := cfg.UpdateHost(hopNames[i], h); err != nil {
			return "", err
		}
	}
	if len(hopNames) > 0 {
		endpoint.Jump = hopNames[0]
		if err := cfg.UpdateHost(endpointName, endpoint); err != nil {
			return "", err
		}
	}

	t.Via = ""
	t.Jump = nil
	t.ViaHost = endpointName
	if err := cfg.UpdateTunnel(t.Name, t); err != nil {
		return "", err
	}
	return endpointName, nil
}

// splitHostPort splits "host:port" into (host, port); with no ':' the port is
// empty. The last ':' is used so IPv6-free host:port forms work.
func splitHostPort(s string) (host, port string) {
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}
