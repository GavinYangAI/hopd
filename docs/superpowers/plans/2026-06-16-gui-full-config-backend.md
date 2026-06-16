# Full GUI Config — Plan 1: Backend Foundation (hosts model + ssh -F rendering)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reusable `hosts:` config model and `via_host` tunnel field, and make hopd render those into an ephemeral `ssh -F` config it owns — so a tunnel can ssh to an endpoint with its own port/user/key through a jump chain, with zero dependency on `~/.ssh/config`.

**Architecture:** A new `hosts:` map in `config` holds per-hop connection params; tunnels reference an entry host via `via_host`. A new pure `internal/sshconf` package turns a tunnel's host chain into ssh config text + the entry alias. `internal/tunnel` gains a `-F` argv path; the daemon writes the generated file per tunnel before launching ssh and rebuilds runners when a referenced host changes. Legacy `via`/`jump` tunnels keep the existing argv path unchanged.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, standard `testing`. This plan is backend-only — no Fyne/GUI changes.

**Scope note:** This is Plan 1 of a multi-plan feature (spec: `docs/superpowers/specs/2026-06-16-gui-full-config-design.md`). Plans 2–4 (GUI hosts manager, tunnel-form rewrite, test-connection/host-key trust, import wizard, defaults editor) are authored after this backend API lands. The import-config parser (`sshconf.ParseSSHConfig`) is included here because it is pure backend, even though only the GUI consumes it.

**Conventions:** Run tests with `go test ./...` from the repo root. End every commit message body with the trailer:
`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
Work on the existing `feature/gui-full-config` branch.

---

## File Structure

- `internal/config/config.go` — modify: add `Host` type, `Config.hosts`, `Tunnel.ViaHost`, raw shapes, parse, accessors, validation.
- `internal/config/marshal.go` — modify: marshal `hosts:` and `via_host`; add `AddHost`/`UpdateHost`/`RemoveHost`.
- `internal/config/config_test.go`, `internal/config/marshal_test.go` — modify/add tests.
- `internal/sshconf/sshconf.go` — create: `Generate` (chain → ssh config text + entry).
- `internal/sshconf/parse.go` — create: `ParseSSHConfig` (read `~/.ssh/config` for import).
- `internal/sshconf/sshconf_test.go`, `internal/sshconf/parse_test.go` — create.
- `internal/tunnel/argv.go` — modify: extract option args helper, add `BuildArgsVia`.
- `internal/tunnel/argv_test.go` — modify/add tests.
- `internal/tunnel/runner.go` — modify: `sshConfigPath`/`entryHost` fields, `SetSSHConfig`, `argv()` selector.
- `internal/tunnel/runner_test.go` — add `argv()` selection test.
- `internal/paths/paths.go` — modify: add `GeneratedDir`.
- `internal/daemon/manager.go` — modify: `buildRunner` (generate+write file), wire into `NewManager`/`Reload`, host-change detection, cleanup.
- `internal/daemon/manager_test.go` — add tests.

---

## Task 1: Add the `Host` type and parse the `hosts:` section

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestParseHosts(t *testing.T) {
	data := []byte(`
hosts:
  bastionB:
    host: 203.0.113.9
    user: userB
  entryA:
    host: 198.51.100.7
    port: 65522
    user: userA
    key: ~/.ssh/idA
    jump: bastionB
groups:
  prod:
    - name: pg
      local: "5432"
      remote: 10.0.1.5:5432
      via_host: entryA
`)
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, ok := cfg.Host("entryA")
	if !ok {
		t.Fatal("entryA not found")
	}
	if a.Host != "198.51.100.7" || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" || a.Jump != "bastionB" {
		t.Fatalf("entryA mismatch: %+v", a)
	}
	b, ok := cfg.Host("bastionB")
	if !ok {
		t.Fatal("bastionB not found")
	}
	if b.Port != 22 { // default applied
		t.Fatalf("bastionB default port = %d, want 22", b.Port)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestParseHosts -v`
Expected: FAIL (compile error — `cfg.Host` undefined, no `via_host` field).

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add the `Host` type after the `Tunnel` type:

```go
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
```

Add a `hosts` field to `Config`:

```go
type Config struct {
	Restart     Restart
	hosts       map[string]Host // reusable SSH endpoints
	tunnels     []Tunnel // ordered as parsed
	byName      map[string]int
	defaultOpts map[string]string // defaults.ssh_options, retained for Marshal
}
```

Add `ViaHost` to `Tunnel`:

```go
type Tunnel struct {
	Name       string
	Group      string
	Local      string            // port or addr:port
	Remote     string            // host:port
	Via        string            // ssh config Host alias (legacy)
	Jump       []string          // inline -J chain (legacy)
	ViaHost    string            // name of a Host entry (new model)
	SSHOptions map[string]string // merged defaults + per-tunnel
	Autostart  bool              // bring this tunnel up when the daemon starts
}
```

Add raw shapes. Add a `Hosts` field to `rawConfig` and a `rawHost` type, plus `ViaHost` to `rawTunnel`:

```go
// in rawConfig struct literal, add:
//   Hosts  map[string]rawHost      `yaml:"hosts"`
// after the Groups field.

type rawHost struct {
	Host       string               `yaml:"host"`
	Port       int                  `yaml:"port"`
	User       string               `yaml:"user"`
	Key        string               `yaml:"key"`
	Jump       string               `yaml:"jump"`
	SSHOptions map[string]yaml.Node `yaml:"ssh_options"`
}

// in rawTunnel struct, add:
//   ViaHost    string               `yaml:"via_host"`
```

In `Parse`, after `cfg.defaultOpts = defOpts` and before the group loop, build hosts:

```go
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
```

In the tunnel build inside the group loop, set `ViaHost: rt.ViaHost,` in the `Tunnel{...}` literal.

Add accessors after the `Tunnel(name)` method:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestParseHosts -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): parse reusable hosts and via_host tunnel field

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Validate the hosts model (refs, port range, cycles, via_host XOR legacy)

**Files:**
- Modify: `internal/config/config.go` (the `Validate` method)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestValidateHostRefs(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string // substring expected in the error; "" => expect success
	}{
		{
			name: "ok",
			yaml: `
hosts:
  a: {host: h1}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: a}
`,
			want: "",
		},
		{
			name: "unknown via_host",
			yaml: `
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: nope}
`,
			want: "unknown host",
		},
		{
			name: "both via_host and legacy",
			yaml: `
hosts:
  a: {host: h1}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: a, via: legacyalias}
`,
			want: "not both",
		},
		{
			name: "host jump cycle",
			yaml: `
hosts:
  a: {host: h1, jump: b}
  b: {host: h2, jump: a}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: a}
`,
			want: "cycle",
		},
		{
			name: "host jump unknown",
			yaml: `
hosts:
  a: {host: h1, jump: ghost}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: a}
`,
			want: "unknown host",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.Parse([]byte(tc.yaml))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			err = cfg.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("want ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}
```

Ensure `internal/config/config_test.go` imports `"strings"` (add it if missing).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestValidateHostRefs -v`
Expected: FAIL (the new rules don't exist; e.g. "both via_host and legacy" currently passes).

- [ ] **Step 3: Implement**

In `internal/config/config.go`, replace the per-tunnel route check inside `Validate`'s tunnel loop. Replace this block:

```go
		if t.Via == "" && len(t.Jump) == 0 {
			return fmt.Errorf("tunnel %q: must set via or jump", t.Name)
		}
```

with:

```go
		hasLegacy := t.Via != "" || len(t.Jump) > 0
		if t.ViaHost != "" {
			if hasLegacy {
				return fmt.Errorf("tunnel %q: set either via_host or via/jump, not both", t.Name)
			}
			if _, ok := c.hosts[t.ViaHost]; !ok {
				return fmt.Errorf("tunnel %q: via_host references unknown host %q", t.Name, t.ViaHost)
			}
		} else if !hasLegacy {
			return fmt.Errorf("tunnel %q: must set via_host, via, or jump", t.Name)
		}
```

Then, at the end of `Validate` (just before `return nil`), add host validation:

```go
	for name, h := range c.hosts {
		if h.Port < 1 || h.Port > 65535 {
			return fmt.Errorf("host %q: invalid port %d", name, h.Port)
		}
		if h.Jump != "" {
			if _, ok := c.hosts[h.Jump]; !ok {
				return fmt.Errorf("host %q: jump references unknown host %q", name, h.Jump)
			}
		}
	}
	for name := range c.hosts {
		seen := map[string]bool{}
		for cur := name; cur != ""; cur = c.hosts[cur].Jump {
			if seen[cur] {
				return fmt.Errorf("host %q: jump chain has a cycle", name)
			}
			seen[cur] = true
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestValidateHostRefs -v`
Then the whole package: `go test ./internal/config/ -v`
Expected: PASS (existing tests still green).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): validate host refs, port range, jump cycles, via_host XOR legacy

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Marshal `hosts:` and `via_host` with round-trip fidelity

**Files:**
- Modify: `internal/config/marshal.go`
- Test: `internal/config/marshal_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/marshal_test.go` (create the file if it doesn't exist, with `package config_test` and imports `"testing"`, `"github.com/GavinYangAI/hopd/internal/config"`):

```go
func TestMarshalHostsRoundTrip(t *testing.T) {
	src := []byte(`
hosts:
  entryA:
    host: 198.51.100.7
    port: 65522
    user: userA
    key: ~/.ssh/idA
    jump: bastionB
  bastionB:
    host: 203.0.113.9
    user: userB
groups:
  prod:
    - name: pg
      local: "5432"
      remote: 10.0.1.5:5432
      via_host: entryA
`)
	cfg, err := config.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := config.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cfg2, err := config.Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	a, ok := cfg2.Host("entryA")
	if !ok || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" || a.Jump != "bastionB" {
		t.Fatalf("entryA round-trip mismatch: %+v (ok=%v)", a, ok)
	}
	tun, ok := cfg2.Tunnel("pg")
	if !ok || tun.ViaHost != "entryA" {
		t.Fatalf("pg via_host round-trip mismatch: %+v (ok=%v)", tun, ok)
	}
	b, ok := cfg2.Host("bastionB")
	if !ok || b.Port != 22 {
		t.Fatalf("bastionB round-trip mismatch: %+v (ok=%v)", b, ok)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestMarshalHostsRoundTrip -v`
Expected: FAIL (marshal drops hosts and via_host).

- [ ] **Step 3: Implement**

In `internal/config/marshal.go`, add a `marshalHost` type and extend the shapes:

```go
type marshalHost struct {
	Host       string            `yaml:"host"`
	Port       int               `yaml:"port,omitempty"`
	User       string            `yaml:"user,omitempty"`
	Key        string            `yaml:"key,omitempty"`
	Jump       string            `yaml:"jump,omitempty"`
	SSHOptions map[string]string `yaml:"ssh_options,omitempty"`
}
```

Add `Hosts` to `marshalShape`:

```go
type marshalShape struct {
	Defaults marshalDefaults            `yaml:"defaults,omitempty"`
	Hosts    map[string]marshalHost     `yaml:"hosts,omitempty"`
	Groups   map[string][]marshalTunnel `yaml:"groups"`
}
```

Add `ViaHost` to `marshalTunnel`:

```go
	ViaHost    string            `yaml:"via_host,omitempty"`
```

In `Marshal`, after setting `shape.Defaults.Restart`, populate hosts:

```go
	if len(c.hosts) > 0 {
		shape.Hosts = make(map[string]marshalHost, len(c.hosts))
		for name, h := range c.hosts {
			shape.Hosts[name] = marshalHost{
				Host:       h.Host,
				Port:       h.Port,
				User:       h.User,
				Key:        h.Key,
				Jump:       h.Jump,
				SSHOptions: h.SSHOptions,
			}
		}
	}
```

In the tunnel loop, set `ViaHost: t.ViaHost,` in the `marshalTunnel{...}` literal.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/marshal.go internal/config/marshal_test.go
git commit -m "feat(config): marshal hosts and via_host (round-trip)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `AddHost` / `UpdateHost` / `RemoveHost`

**Files:**
- Modify: `internal/config/marshal.go` (alongside the existing tunnel mutators)
- Test: `internal/config/marshal_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/marshal_test.go`:

```go
func TestHostMutators(t *testing.T) {
	cfg, err := config.Parse([]byte(`
hosts:
  a: {host: h1}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: a}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cfg.AddHost("b", config.Host{Host: "h2", Port: 22}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := cfg.AddHost("a", config.Host{Host: "dup"}); err == nil {
		t.Fatal("add duplicate should fail")
	}
	if err := cfg.UpdateHost("b", config.Host{Host: "h2-new", Port: 2222}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if h, _ := cfg.Host("b"); h.Host != "h2-new" || h.Port != 2222 {
		t.Fatalf("update mismatch: %+v", h)
	}
	// Removing a host referenced by a tunnel must fail.
	if err := cfg.RemoveHost("a"); err == nil {
		t.Fatal("remove referenced host should fail")
	}
	// Unreferenced host removes cleanly.
	if err := cfg.RemoveHost("b"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := cfg.Host("b"); ok {
		t.Fatal("b should be gone")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestHostMutators -v`
Expected: FAIL (mutators undefined).

- [ ] **Step 3: Implement**

In `internal/config/marshal.go`, add after `RemoveTunnel`:

```go
// AddHost inserts a new reusable host. It errors if the name already exists.
func (c *Config) AddHost(name string, h Host) error {
	if c.hosts == nil {
		c.hosts = map[string]Host{}
	}
	if _, ok := c.hosts[name]; ok {
		return fmt.Errorf("host %q already exists", name)
	}
	c.hosts[name] = h
	return nil
}

// UpdateHost replaces an existing host's params.
func (c *Config) UpdateHost(name string, h Host) error {
	if _, ok := c.hosts[name]; !ok {
		return fmt.Errorf("host %q not found", name)
	}
	c.hosts[name] = h
	return nil
}

// RemoveHost deletes a host. It errors if any tunnel's ViaHost or any other
// host's Jump still references it, so the config never dangles.
func (c *Config) RemoveHost(name string) error {
	if _, ok := c.hosts[name]; !ok {
		return fmt.Errorf("host %q not found", name)
	}
	for _, t := range c.tunnels {
		if t.ViaHost == name {
			return fmt.Errorf("host %q is referenced by tunnel %q", name, t.Name)
		}
	}
	for hn, h := range c.hosts {
		if h.Jump == name {
			return fmt.Errorf("host %q is referenced by host %q (jump)", name, hn)
		}
	}
	delete(c.hosts, name)
	return nil
}
```

> Note: renaming a host is intentionally not supported by `UpdateHost` (keeps refs stable). The GUI renames by add-new + repoint + remove-old.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/marshal.go internal/config/marshal_test.go
git commit -m "feat(config): AddHost/UpdateHost/RemoveHost with ref-safety

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `sshconf.Generate` — render a tunnel's host chain to ssh config text

**Files:**
- Create: `internal/sshconf/sshconf.go`
- Test: `internal/sshconf/sshconf_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/sshconf/sshconf_test.go`:

```go
package sshconf_test

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)

func TestGenerateChain(t *testing.T) {
	cfg, err := config.Parse([]byte(`
hosts:
  bastionB:
    host: 203.0.113.9
    user: userB
  entryA:
    host: 198.51.100.7
    port: 65522
    user: userA
    key: ~/.ssh/idA
    jump: bastionB
groups:
  prod:
    - name: pg
      local: "5432"
      remote: 10.0.1.5:5432
      via_host: entryA
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tun, _ := cfg.Tunnel("pg")
	text, entry, err := sshconf.Generate(cfg, tun)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if entry != "entryA" {
		t.Fatalf("entry = %q, want entryA", entry)
	}
	want := `# Generated by hopd — do not edit. Source: config.yaml
Host entryA
    HostName 198.51.100.7
    Port 65522
    User userA
    IdentityFile ~/.ssh/idA
    IdentitiesOnly yes
    ProxyJump bastionB
Host bastionB
    HostName 203.0.113.9
    Port 22
    User userB
`
	if text != want {
		t.Fatalf("generated mismatch:\n--- got ---\n%s\n--- want ---\n%s", text, want)
	}
}

func TestGenerateLegacyReturnsEmpty(t *testing.T) {
	cfg, _ := config.Parse([]byte(`
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via: somealias}
`))
	tun, _ := cfg.Tunnel("t1")
	text, entry, err := sshconf.Generate(cfg, tun)
	if err != nil || text != "" || entry != "" {
		t.Fatalf("legacy tunnel should yield empty: text=%q entry=%q err=%v", text, entry, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sshconf/ -v`
Expected: FAIL (package/file does not exist).

- [ ] **Step 3: Implement**

Create `internal/sshconf/sshconf.go`:

```go
// Package sshconf renders hopd's reusable host model into an ssh config file
// that hopd owns and passes to ssh via -F, plus parses the user's ~/.ssh/config
// for the GUI import wizard. It never writes the user's ~/.ssh/config.
package sshconf

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
)

const header = "# Generated by hopd — do not edit. Source: config.yaml\n"

// Generate renders the ssh -F config for a via_host tunnel's host chain and
// returns the config text plus the entry Host alias to use as the ssh
// destination. For a tunnel without ViaHost (legacy via/jump), it returns
// ("", "", nil), signalling the caller to use the legacy argv path.
func Generate(cfg *config.Config, t config.Tunnel) (text, entry string, err error) {
	if t.ViaHost == "" {
		return "", "", nil
	}
	var b strings.Builder
	b.WriteString(header)
	seen := map[string]bool{}
	for name := t.ViaHost; name != ""; {
		if seen[name] {
			return "", "", fmt.Errorf("host %q: jump chain has a cycle", name)
		}
		seen[name] = true
		h, ok := cfg.Host(name)
		if !ok {
			return "", "", fmt.Errorf("via_host references unknown host %q", name)
		}
		writeHostBlock(&b, name, h)
		name = h.Jump
	}
	return b.String(), t.ViaHost, nil
}

func writeHostBlock(b *strings.Builder, name string, h config.Host) {
	fmt.Fprintf(b, "Host %s\n", name)
	fmt.Fprintf(b, "    HostName %s\n", h.Host)
	if h.Port != 0 {
		fmt.Fprintf(b, "    Port %s\n", strconv.Itoa(h.Port))
	}
	if h.User != "" {
		fmt.Fprintf(b, "    User %s\n", h.User)
	}
	if h.Key != "" {
		fmt.Fprintf(b, "    IdentityFile %s\n", h.Key)
		b.WriteString("    IdentitiesOnly yes\n")
	}
	if h.Jump != "" {
		fmt.Fprintf(b, "    ProxyJump %s\n", h.Jump)
	}
	keys := make([]string, 0, len(h.SSHOptions))
	for k := range h.SSHOptions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, "    %s %s\n", k, h.SSHOptions[k])
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sshconf/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sshconf/sshconf.go internal/sshconf/sshconf_test.go
git commit -m "feat(sshconf): render host chain to ssh -F config text

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: `BuildArgsVia` — ssh argv that uses the generated `-F` config

**Files:**
- Modify: `internal/tunnel/argv.go`
- Test: `internal/tunnel/argv_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/tunnel/argv_test.go`:

```go
func TestBuildArgsVia(t *testing.T) {
	tun := config.Tunnel{
		Name:    "pg",
		Local:   "5432",
		Remote:  "10.0.1.5:5432",
		ViaHost: "entryA",
		SSHOptions: map[string]string{
			"ExitOnForwardFailure": "yes",
		},
	}
	got := tunnel.BuildArgsVia(tun, "/tmp/hopd/pg.sshcfg", "entryA")
	want := []string{
		"-F", "/tmp/hopd/pg.sshcfg",
		"-N", "-T",
		"-o", "ExitOnForwardFailure=yes",
		"-L", "127.0.0.1:5432:10.0.1.5:5432",
		"entryA",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}
```

Ensure `internal/tunnel/argv_test.go` imports `"reflect"` and `"github.com/GavinYangAI/hopd/internal/config"` (the existing tests already use them; confirm).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tunnel/ -run TestBuildArgsVia -v`
Expected: FAIL (`BuildArgsVia` undefined).

- [ ] **Step 3: Implement**

In `internal/tunnel/argv.go`, extract the shared option emission and add `BuildArgsVia`. Replace the body of `BuildArgs` and add the helper + new function:

```go
// BuildArgs converts a legacy tunnel spec (via alias / inline jump) into the
// argv for `ssh` (excluding argv[0]). Options are emitted in sorted key order
// for deterministic, testable output.
func BuildArgs(t config.Tunnel) []string {
	args := []string{"-N", "-T"}
	args = append(args, optionArgs(t)...)
	if len(t.Jump) > 0 {
		args = append(args, "-J", strings.Join(t.Jump, ","))
	}
	args = append(args, "-L", localForward(t.Local, t.Remote))
	args = append(args, targetHost(t))
	return args
}

// BuildArgsVia converts a via_host tunnel into ssh argv that points ssh at the
// hopd-generated config (sshConfigPath) and connects to the entry host alias.
func BuildArgsVia(t config.Tunnel, sshConfigPath, entry string) []string {
	args := []string{"-F", sshConfigPath, "-N", "-T"}
	args = append(args, optionArgs(t)...)
	args = append(args, "-L", localForward(t.Local, t.Remote))
	args = append(args, entry)
	return args
}

// optionArgs renders t.SSHOptions as sorted -o key=value pairs.
func optionArgs(t config.Tunnel) []string {
	keys := make([]string, 0, len(t.SSHOptions))
	for k := range t.SSHOptions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		args = append(args, "-o", k+"="+t.SSHOptions[k])
	}
	return args
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tunnel/ -v`
Expected: PASS (existing `BuildArgs` tests unchanged — `optionArgs` produces identical output).

- [ ] **Step 5: Commit**

```bash
git add internal/tunnel/argv.go internal/tunnel/argv_test.go
git commit -m "feat(tunnel): BuildArgsVia for ssh -F generated-config path

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Runner selects the `-F` path when a generated config is set

**Files:**
- Modify: `internal/tunnel/runner.go`
- Test: `internal/tunnel/runner_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/tunnel/runner_test.go` (create with `package tunnel` if it doesn't exist — note this is an internal/white-box test so it can call the unexported `argv()`):

```go
func TestRunnerArgvSelection(t *testing.T) {
	tun := config.Tunnel{Name: "pg", Local: "5432", Remote: "10.0.1.5:5432", ViaHost: "entryA"}

	r := NewRunner(tun, "/usr/bin/ssh", time.Second, time.Minute)
	// Legacy path until a generated config is attached.
	if got := r.argv(); got[0] == "-F" {
		t.Fatalf("expected legacy argv without -F, got %v", got)
	}

	r.SetSSHConfig("/tmp/hopd/pg.sshcfg", "entryA")
	got := r.argv()
	if len(got) < 2 || got[0] != "-F" || got[1] != "/tmp/hopd/pg.sshcfg" {
		t.Fatalf("expected -F argv, got %v", got)
	}
	if got[len(got)-1] != "entryA" {
		t.Fatalf("expected entry host as ssh target, got %v", got)
	}
}
```

If `runner_test.go` already exists with a different package clause, add this test to whichever existing white-box test file uses `package tunnel`. (The existing `argv_test.go` is `package tunnel_test`; this selection test must be white-box, so keep it in a `package tunnel` file such as `runner_test.go`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tunnel/ -run TestRunnerArgvSelection -v`
Expected: FAIL (`SetSSHConfig` and `argv` undefined).

- [ ] **Step 3: Implement**

In `internal/tunnel/runner.go`, add two fields to the `Runner` struct (after `localAddr`):

```go
	sshConfigPath string // when set, use BuildArgsVia (-F) instead of BuildArgs
	entryHost     string // ssh destination alias for the -F path
```

Add the setter and the argv selector (place near `Spec`):

```go
// SetSSHConfig attaches a hopd-generated ssh config so the runner launches ssh
// with -F <path> and connects to entry. Called by the daemon for via_host
// tunnels before Start.
func (r *Runner) SetSSHConfig(path, entry string) {
	r.mu.Lock()
	r.sshConfigPath = path
	r.entryHost = entry
	r.mu.Unlock()
}

// argv builds the ssh argument vector for the current attempt, choosing the
// generated-config path when one is attached.
func (r *Runner) argv() []string {
	r.mu.Lock()
	path, entry := r.sshConfigPath, r.entryHost
	r.mu.Unlock()
	if path != "" {
		return BuildArgsVia(r.tunnel, path, entry)
	}
	return BuildArgs(r.tunnel)
}
```

In `loop`, replace the exec line:

```go
		cmd := exec.CommandContext(attemptCtx, r.sshPath, BuildArgs(r.tunnel)...)
```

with:

```go
		cmd := exec.CommandContext(attemptCtx, r.sshPath, r.argv()...)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tunnel/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tunnel/runner.go internal/tunnel/runner_test.go
git commit -m "feat(tunnel): runner picks -F generated-config argv when attached

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `paths.GeneratedDir`

**Files:**
- Modify: `internal/paths/paths.go`
- Test: `internal/paths/paths_test.go`

- [ ] **Step 1: Write the failing test**

Create/append `internal/paths/paths_test.go`:

```go
package paths_test

import (
	"path/filepath"
	"testing"

	"github.com/GavinYangAI/hopd/internal/paths"
)

func TestGeneratedDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	want := filepath.Join("/tmp/xdg", "hopd", "generated")
	if got := paths.GeneratedDir(); got != want {
		t.Fatalf("GeneratedDir = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/paths/ -v`
Expected: FAIL (`GeneratedDir` undefined).

- [ ] **Step 3: Implement**

In `internal/paths/paths.go`, add after `ControlDir`:

```go
// GeneratedDir holds hopd-generated ssh config files (ssh -F targets).
func GeneratedDir() string { return filepath.Join(ConfigDir(), "generated") }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/paths/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/paths/paths.go internal/paths/paths_test.go
git commit -m "feat(paths): add GeneratedDir for ssh -F config files

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Daemon writes the generated config and attaches it to runners

**Files:**
- Modify: `internal/daemon/manager.go`
- Test: `internal/daemon/manager_test.go`

This task introduces a `buildRunner` helper that, for `via_host` tunnels, generates the ssh config (via `sshconf`), writes it under a configurable directory, and calls `SetSSHConfig`. To keep the manager unit-testable without touching the real home dir, the generated-config directory becomes a field on `Manager` defaulting to `paths.GeneratedDir()`.

- [ ] **Step 1: Write the failing test**

Add to `internal/daemon/manager_test.go` (create with `package daemon` — white-box, to read the unexported `genDir` and call `buildRunner` indirectly via `NewManager`):

```go
func TestManagerWritesGeneratedConfig(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Parse([]byte(`
hosts:
  entryA:
    host: 198.51.100.7
    port: 65522
    user: userA
groups:
  prod:
    - name: pg
      local: "5432"
      remote: 10.0.1.5:5432
      via_host: entryA
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := NewManagerWithGenDir("/usr/bin/ssh", cfg, dir)
	_ = m
	want := filepath.Join(dir, "pg.sshcfg")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected generated config at %s: %v", want, err)
	}
	if !strings.Contains(string(data), "Host entryA") || !strings.Contains(string(data), "Port 65522") {
		t.Fatalf("generated config missing host block:\n%s", data)
	}
}
```

Imports needed: `"os"`, `"path/filepath"`, `"strings"`, `"testing"`, `"github.com/GavinYangAI/hopd/internal/config"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/ -run TestManagerWritesGeneratedConfig -v`
Expected: FAIL (`NewManagerWithGenDir` undefined).

- [ ] **Step 3: Implement**

In `internal/daemon/manager.go`, add imports `"os"`, `"path/filepath"`, `"github.com/GavinYangAI/hopd/internal/paths"`, `"github.com/GavinYangAI/hopd/internal/sshconf"`.

Add a `genDir` field to `Manager`:

```go
type Manager struct {
	mu      sync.Mutex
	sshPath string
	genDir  string
	cfg     *config.Config
	runners map[string]*tunnel.Runner
	order   []string
}
```

Change `NewManager` to delegate, and add the test seam + the helper:

```go
// NewManager builds runners (all DOWN) for every tunnel in cfg, writing any
// generated ssh -F configs under paths.GeneratedDir().
func NewManager(sshPath string, cfg *config.Config) *Manager {
	return NewManagerWithGenDir(sshPath, cfg, paths.GeneratedDir())
}

// NewManagerWithGenDir is NewManager with an explicit generated-config dir (for
// tests).
func NewManagerWithGenDir(sshPath string, cfg *config.Config, genDir string) *Manager {
	m := &Manager{sshPath: sshPath, genDir: genDir, cfg: cfg, runners: map[string]*tunnel.Runner{}}
	for _, t := range cfg.Tunnels() {
		m.runners[t.Name] = m.buildRunner(cfg, t)
		m.order = append(m.order, t.Name)
	}
	return m
}

// buildRunner creates a runner for t and, when t uses via_host, writes its
// generated ssh config and attaches it. A generation/write failure is left to
// surface at connect time (the runner falls back to legacy argv, which will
// fail clearly) rather than aborting manager construction.
func (m *Manager) buildRunner(cfg *config.Config, t config.Tunnel) *tunnel.Runner {
	r := tunnel.NewRunner(t, m.sshPath, cfg.Restart.Min, cfg.Restart.Max)
	text, entry, err := sshconf.Generate(cfg, t)
	if err == nil && text != "" {
		path := filepath.Join(m.genDir, t.Name+".sshcfg")
		if writeErr := writeGenerated(path, text); writeErr == nil {
			r.SetSSHConfig(path, entry)
		}
	}
	return r
}

// writeGenerated writes the generated ssh config atomically with 0600 perms,
// creating the parent dir (0700) if needed.
func writeGenerated(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/daemon/ -v`
Expected: PASS (existing daemon tests still green; `NewManager` behavior is unchanged for legacy configs because `Generate` returns empty).

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/manager.go internal/daemon/manager_test.go
git commit -m "feat(daemon): write generated ssh -F config per via_host tunnel

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Reload rebuilds runners when a referenced host changes; cleans up removed files

**Files:**
- Modify: `internal/daemon/manager.go` (`Reload`)
- Test: `internal/daemon/manager_test.go`

The problem: `Reload` reuses a runner when `reflect.DeepEqual(r.Spec(), t)` is true. A `via_host` tunnel's `Spec()` is unchanged even when the *host it points at* changes (e.g. port 65522 → 2222), so the stale generated config would keep being used. Fix: for `via_host` tunnels, also compare the freshly generated config text against what's on disk, and rebuild (rewriting the file) when it differs. Also remove generated files for tunnels dropped from config.

- [ ] **Step 1: Write the failing test**

Add to `internal/daemon/manager_test.go`:

```go
func TestReloadRegeneratesOnHostChange(t *testing.T) {
	dir := t.TempDir()
	cfg1, _ := config.Parse([]byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 65522, user: userA}
groups:
  prod:
    - {name: pg, local: "5432", remote: 10.0.1.5:5432, via_host: entryA}
`))
	m := NewManagerWithGenDir("/usr/bin/ssh", cfg1, dir)

	cfg2, _ := config.Parse([]byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 2222, user: userA}
groups:
  prod:
    - {name: pg, local: "5432", remote: 10.0.1.5:5432, via_host: entryA}
`))
	if err := m.Reload(cfg2); err != nil {
		t.Fatalf("reload: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "pg.sshcfg"))
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	if !strings.Contains(string(data), "Port 2222") {
		t.Fatalf("generated config not regenerated after host change:\n%s", data)
	}
}

func TestReloadRemovesGeneratedFileForDroppedTunnel(t *testing.T) {
	dir := t.TempDir()
	cfg1, _ := config.Parse([]byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 65522, user: userA}
groups:
  prod:
    - {name: pg, local: "5432", remote: 10.0.1.5:5432, via_host: entryA}
`))
	m := NewManagerWithGenDir("/usr/bin/ssh", cfg1, dir)
	if _, err := os.Stat(filepath.Join(dir, "pg.sshcfg")); err != nil {
		t.Fatalf("precondition: %v", err)
	}
	cfg2, _ := config.Parse([]byte(`groups: {}`))
	if err := m.Reload(cfg2); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pg.sshcfg")); !os.IsNotExist(err) {
		t.Fatalf("expected generated file removed, stat err=%v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/ -run TestReload -v`
Expected: FAIL — `TestReloadRegeneratesOnHostChange` fails (reused runner keeps stale file); the removal test fails (no cleanup).

- [ ] **Step 3: Implement**

In `internal/daemon/manager.go`, add a helper that decides whether a `via_host` tunnel's generated config changed, and rewrite/rebuild accordingly. Replace the reuse branch inside `Reload`'s loop. The current loop body for an existing runner is:

```go
		if r, ok := old[t.Name]; ok {
			if !boundsChanged && reflect.DeepEqual(r.Spec(), t) {
				next[t.Name] = r
				delete(old, t.Name)
				continue
			}
			wasActive := isActive(r.Snapshot().State)
			r.Stop()
			delete(old, t.Name)
			nr := tunnel.NewRunner(t, m.sshPath, cfg.Restart.Min, cfg.Restart.Max)
			if wasActive {
				nr.Start()
			}
			next[t.Name] = nr
			continue
		}
		next[t.Name] = tunnel.NewRunner(t, m.sshPath, cfg.Restart.Min, cfg.Restart.Max)
```

Replace it with (note both `NewRunner` calls become `m.buildRunner(cfg, t)`, and reuse now also requires the generated config to be unchanged):

```go
		if r, ok := old[t.Name]; ok {
			if !boundsChanged && reflect.DeepEqual(r.Spec(), t) && !m.genConfigChanged(cfg, t) {
				next[t.Name] = r
				delete(old, t.Name)
				continue
			}
			wasActive := isActive(r.Snapshot().State)
			r.Stop()
			delete(old, t.Name)
			nr := m.buildRunner(cfg, t)
			if wasActive {
				nr.Start()
			}
			next[t.Name] = nr
			continue
		}
		next[t.Name] = m.buildRunner(cfg, t)
```

After the loop, where removed tunnels are stopped, also delete their generated files:

```go
	for name, r := range old { // tunnels removed from config
		r.Stop()
		_ = os.Remove(filepath.Join(m.genDir, name+".sshcfg"))
	}
```

(Change the existing `for _, r := range old {` to `for name, r := range old {` so the name is available.)

Add the helper:

```go
// genConfigChanged reports whether t's freshly generated ssh config differs
// from what is on disk. Legacy tunnels (no via_host) always report false.
func (m *Manager) genConfigChanged(cfg *config.Config, t config.Tunnel) bool {
	text, _, err := sshconf.Generate(cfg, t)
	if err != nil || text == "" {
		return false
	}
	existing, err := os.ReadFile(filepath.Join(m.genDir, t.Name+".sshcfg"))
	if err != nil {
		return true // missing/unreadable => must (re)write
	}
	return string(existing) != text
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/daemon/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/manager.go internal/daemon/manager_test.go
git commit -m "feat(daemon): regenerate ssh config on host change; clean up dropped tunnels

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: `sshconf.ParseSSHConfig` — read ~/.ssh/config for the import wizard

**Files:**
- Create: `internal/sshconf/parse.go`
- Test: `internal/sshconf/parse_test.go`

This is pure backend for the GUI import wizard (Plan 4). It parses Host blocks into importable host params. It handles the directives hopd maps to its `Host` model: `HostName`, `Port`, `User`, `IdentityFile`, `ProxyJump`. Wildcard `Host *` patterns and `Host` lines with multiple patterns are skipped (not importable as a single named host).

- [ ] **Step 1: Write the failing test**

Create `internal/sshconf/parse_test.go`:

```go
package sshconf_test

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/sshconf"
)

func TestParseSSHConfig(t *testing.T) {
	data := []byte(`
Host *
    ServerAliveInterval 30

Host entryA
    HostName 198.51.100.7
    Port 65522
    User userA
    IdentityFile ~/.ssh/idA
    ProxyJump bastionB

Host bastionB
    HostName 203.0.113.9
    User userB
`)
	got, err := sshconf.ParseSSHConfig(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d hosts, want 2 (wildcard skipped): %+v", len(got), got)
	}
	byName := map[string]sshconf.ImportedHost{}
	for _, h := range got {
		byName[h.Name] = h
	}
	a := byName["entryA"]
	if a.HostName != "198.51.100.7" || a.Port != 65522 || a.User != "userA" || a.IdentityFile != "~/.ssh/idA" || a.ProxyJump != "bastionB" {
		t.Fatalf("entryA mismatch: %+v", a)
	}
	b := byName["bastionB"]
	if b.HostName != "203.0.113.9" || b.User != "userB" || b.Port != 0 {
		t.Fatalf("bastionB mismatch: %+v", b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sshconf/ -run TestParseSSHConfig -v`
Expected: FAIL (`ParseSSHConfig`/`ImportedHost` undefined).

- [ ] **Step 3: Implement**

Create `internal/sshconf/parse.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sshconf/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sshconf/parse.go internal/sshconf/parse_test.go
git commit -m "feat(sshconf): parse ~/.ssh/config into importable hosts

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: Full-suite verification

**Files:** none (verification only)

- [ ] **Step 1: Run the whole test suite**

Run: `go test ./...`
Expected: PASS across all packages. (Two pre-existing environment-flaky tests on this Mac — socket-path length and a timing-sensitive test — may fail for env reasons unrelated to this change; confirm any failure matches those known cases before treating it as a regression.)

- [ ] **Step 2: Vet and build**

Run: `go vet ./... && go build ./...`
Expected: no errors.

- [ ] **Step 3: Manual end-to-end smoke (optional, documents the milestone)**

This proves the backend works before any GUI exists. With a reachable bastion + endpoint, hand-write `~/.config/hopd/config.yaml`:

```yaml
hosts:
  entryA: {host: <A_public>, port: 65522, user: <userA>, key: ~/.ssh/idA, jump: bastionB}
  bastionB: {host: <B_public>, port: 22, user: <userB>}
groups:
  prod:
    - {name: pg, local: "5432", remote: <A_internal>:5432, via_host: entryA, autostart: true}
```

Run the daemon, `hopd up pg`, and confirm `~/.config/hopd/generated/pg.sshcfg` was written and `psql -h 127.0.0.1 -p 5432` reaches A — with no `~/.ssh/config` entry present.

- [ ] **Step 4: Commit (if any incidental fixes were needed)**

```bash
git add -A
git commit -m "test: full-suite verification for hosts/via_host backend

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review (author checklist — already applied)

**Spec coverage (Plan 1 scope):**
- §3 schema `hosts:`/`via_host` → Tasks 1, 3.
- §4 validation (refs, cycles, port, via_host XOR legacy) → Task 2.
- §5 `ssh -F` generation + dual path + IdentitiesOnly + known_hosts default → Tasks 5, 6, 7, 9.
- §5 per-tunnel `-o` options stay on the command line (incl. injected ControlMaster/ExitOnForwardFailure) → preserved by Task 6 `optionArgs`; generated file holds only host blocks.
- §6 back-compat (legacy path untouched) → Tasks 2, 5 (empty generate), 6 (BuildArgs unchanged), 7 (falls back).
- §6 reload correctness on host change → Task 10.
- §7.6 / §8 import parser (`ParseSSHConfig`) → Task 11.
- §10 security (0700 dir / 0600 file, atomic write, no ~/.ssh/config writes) → Tasks 8, 9.

**Deferred to later plans (intentionally out of scope here):** all Fyne GUI surfaces (hosts manager, tunnel-form rewrite, settings window), test-connection runner + host-key trust dialog, import wizard UI, legacy "迁移为主机" action, defaults editor. The migration *transform* and test-connection *runner* will reuse `sshconf.Generate`/`ParseSSHConfig` defined here.

**Type consistency:** `config.Host`, `Config.Host(name)`, `Config.Hosts()`, `Tunnel.ViaHost`, `sshconf.Generate(cfg, t) (text, entry, err)`, `tunnel.BuildArgsVia(t, path, entry)`, `Runner.SetSSHConfig(path, entry)`, `Runner.argv()`, `paths.GeneratedDir()`, `Manager.buildRunner`/`genConfigChanged`/`NewManagerWithGenDir`, `sshconf.ImportedHost` — names are used identically across all tasks.

**Placeholder scan:** none — every code step contains complete code. (Task 9 Step 1 explicitly corrects its own illustrative YAML before implementation.)
