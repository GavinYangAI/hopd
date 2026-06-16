# Full GUI Config — Plan 3: Tunnel dialog rewrite (via_host) + legacy migration

> **⚠️ CROSS-PLAN INTEGRATION NOTES (read before executing — added after a 3-plan consistency review):**
> 1. **`readUserSSHConfig` is the single source of truth for THIS plan.** Define `func readUserSSHConfig() ([]byte, error)` and a sentinel `var errMissingSSHConfig = errors.New(...)` (returned when `~/.ssh/config` is absent) **once**, in `internal/guiapp/window.go`. Plan 4 will REUSE these — it must NOT redefine them. Keep the body simple (read `~/.ssh/config`; missing file → `errMissingSSHConfig`).
> 2. **`editdialog_test.go` has FIVE `newEditForm(` call sites across four tests** — `TestNewEditForm_CarriesAutostart` contains TWO calls (the second creates `ef2`). When Task 9/10 change `newEditForm`/`showEditDialog` signatures, update ALL FIVE call sites (grep `newEditForm(` and `showEditDialog(` to be sure none are missed).
> 3. **Reuse-list correction:** Plan 2's real signature is `gui.CheckHost(f HostForm, otherNames []string, jumpTargets []string) gui.FieldErrors` (3 args), not the single-arg form mentioned in the scope note. This plan only calls `showHostDialog`/`gui.TestConnection`, so it's a doc note, but keep it in mind.
> 4. **Adapt to current file state, don't blind-paste.** `forwardDiagram` (card.go) and `filtered`/toolbar (window.go) may differ slightly from the quoted baselines — make the equivalent additive change to the REAL current code.
> 5. **Confirm no existing test pins the old blank-form default** (`RouteOf(TunnelForm{}) == ""`) before changing it to `RouteViaHost` (Task 2). Grep `formcheck_test.go`/`form_test.go`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the tunnel editor speak the new `via_host` model — pick a saved host (or create one inline) instead of typing `~/.ssh/config` aliases / inline jump chains — and give every legacy `via`/`jump` tunnel a one-click "迁移为主机" path into that model, with a dismissible hint banner and a "测试连接" button. The forward diagram and search learn to render `via_host`.

**Architecture:** Pure logic first. `gui.TunnelForm` gains a `ViaHost` field and a new `RouteViaHost` route; `CheckRoute`/`RouteOf` learn it and new blank tunnels default to it. A new pure `gui.MigrateLegacyTunnel` rewrites a legacy tunnel into a `via_host` tunnel + synthesized `hosts:` entries (via-alias case reads `~/.ssh/config` through an injected reader and `sshconf.ParseSSHConfig`; inline-jump case synthesizes one host per hop). Then the Fyne layer: `newEditForm`/`showEditDialog` gain the host list so the route card can offer a host picker + "+ 新建主机", legacy tunnels render read-mostly with a migrate button, and a one-time banner (dismissal persisted in `app.Preferences()`) points at migration. `ipc.TunnelStatus` gains `ViaHost`, populated where the daemon builds it, and `card.go`/`window.go` render and search it.

**Tech Stack:** Go, Fyne v2 (`fyne.io/fyne/v2`, `.../test` for headless widget tests), standard `testing`. Pure-logic packages (`internal/gui`, `internal/sshconf`, `internal/config`, `internal/ipc`) have no Fyne deps; `internal/guiapp` is the only Fyne package.

**Scope note:** This is Plan 3 of a multi-plan feature (spec: `docs/superpowers/specs/2026-06-16-gui-full-config-design.md`, §3, §6, §7.2). Plan 1 (backend: `config.Host`, `Tunnel.ViaHost`, `sshconf.Generate`/`ParseSSHConfig`, daemon `-F` rendering) is **already landed** — this plan builds on that locked API. Plan 2 (hosts manager + host dialog + test-connection) defines a SHARED CONTRACT this plan **reuses but does not implement**:

- `gui.HostForm`, `gui.ToHostForm(name string, h config.Host) gui.HostForm`, `(gui.HostForm).Parse() (string, config.Host, error)`, `gui.CheckHost(gui.HostForm) gui.FieldErrors`.
- `gui.TestConnection(ctx, cfg *config.Config, hostName string, run gui.CmdRunner) gui.TestConnResult`, plus types `gui.TestConnResult`, `gui.CmdRunner`, `gui.HostKey`; default runner `execRunner` (in `internal/guiapp`).
- Fyne: `showHostDialog(win fyne.Window, store *gui.ConfigStore, initial gui.HostForm, editingName string, onDone func())` and the hosts-manager entry point `(d *dashboard).openHosts()`.

> **Sequencing requirement:** Task 9 ("+ 新建主机" inline) and Task 11 ("测试连接") call Plan-2 symbols (`showHostDialog`, `gui.TestConnection`, `execRunner`). Execute this plan **after** Plan 2 has landed, or those two tasks will not compile. Tasks 1–8, 10, 12–13 do **not** depend on Plan 2 and can proceed independently.

**Conventions:** Run tests with `go test ./...` from the repo root. End every commit message body with the trailer:
`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
Work on the existing `feature/gui-full-config` branch.

---

## File Structure

- `internal/gui/form.go` — modify: add `TunnelForm.ViaHost`; `Parse` emits `via_host` (and clears legacy via/jump) when set; `ToForm` carries `ViaHost`.
- `internal/gui/form_test.go` — modify: add via_host Parse/ToForm round-trip tests (existing tests stay green).
- `internal/gui/formroute.go` — modify: add `RouteViaHost`; `RouteOf` returns it for `ViaHost != ""`; new-blank default is `RouteViaHost`; `CheckRoute` validates it.
- `internal/gui/formroute_test.go` — create: route-inference + CheckRoute tests for via_host. (No formroute test file exists today.)
- `internal/gui/migrate.go` — create: `MigrateLegacyTunnel` (pure; via-alias + inline-jump cases) and helpers (`uniqueHostName`).
- `internal/gui/migrate_test.go` — create: via-alias case, inline-jump case, error cases.
- `internal/ipc/protocol.go` — modify: add `TunnelStatus.ViaHost` (json `via_host,omitempty`).
- `internal/tunnel/runner.go` — modify: populate `ViaHost` in `Snapshot()`'s `ipc.TunnelStatus{}` literal.
- `internal/tunnel/runner_test.go` — modify/append: assert `Snapshot().ViaHost`.
- `internal/guiapp/editdialog.go` — modify: thread host names into `newEditForm`/`showEditDialog`; add via_host route card + picker; legacy read-mostly view + migrate button; "测试连接" button; banner helper.
- `internal/guiapp/editdialog_test.go` — modify: update all `newEditForm(...)` callers to the new signature; add picker construct/validate + legacy-readonly tests.
- `internal/guiapp/window.go` — modify: pass host names into `showEditDialog` from `addTunnel`/`editTunnel`; show the hint banner; add `ViaHost` to the search filter; wire the migrate/test store paths.
- `internal/guiapp/card.go` — modify: `forwardDiagram` renders `本机 → [主机 name] → 目标` when `ViaHost` is set (legacy `Via` rendering unchanged).
- `internal/guiapp/card_test.go` — create: `forwardDiagram` via_host node assertion. (No card test file exists today.)

---

## Task 1: `TunnelForm.ViaHost` — Parse emits via_host; ToForm carries it

**Files:**
- Modify: `internal/gui/form.go`
- Test: `internal/gui/form_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/gui/form_test.go`:

```go
func TestTunnelForm_Parse_ViaHost(t *testing.T) {
	f := TunnelForm{
		Name: "pg", Group: "prod", LocalPort: "5432",
		DestHost: "10.0.1.5", DestPort: "5432",
		ViaHost: "entryA",
	}
	tn, err := f.Parse()
	if err != nil {
		t.Fatal(err)
	}
	if tn.ViaHost != "entryA" {
		t.Fatalf("ViaHost = %q, want entryA", tn.ViaHost)
	}
	if tn.Remote != "10.0.1.5:5432" {
		t.Fatalf("remote = %q", tn.Remote)
	}
	if tn.Via != "" || len(tn.Jump) != 0 {
		t.Fatalf("via_host set: legacy via/jump must be empty, got via=%q jump=%v", tn.Via, tn.Jump)
	}
}

func TestTunnelForm_Parse_ViaHostWinsOverLegacyFields(t *testing.T) {
	// If a form somehow carries both, via_host wins and legacy is dropped.
	f := TunnelForm{
		Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432",
		ViaHost: "entryA", Via: "stalealias", JumpHost: "ignored", JumpUser: "x",
		RawJump: []string{"a@h1:22"},
	}
	tn, _ := f.Parse()
	if tn.ViaHost != "entryA" || tn.Via != "" || len(tn.Jump) != 0 {
		t.Fatalf("expected via_host-only tunnel, got %+v", tn)
	}
}

func TestToForm_ViaHostRoundTrip(t *testing.T) {
	tn := config.Tunnel{
		Name: "pg", Group: "prod", Local: "5432", Remote: "10.0.1.5:5432",
		ViaHost: "entryA",
	}
	f := ToForm(tn)
	if f.ViaHost != "entryA" || f.DestHost != "10.0.1.5" || f.DestPort != "5432" {
		t.Fatalf("ToForm via_host wrong: %+v", f)
	}
	back, err := f.Parse()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, tn) {
		t.Fatalf("via_host round trip differs:\n a=%+v\n b=%+v", tn, back)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run 'TunnelForm_Parse_ViaHost|ToForm_ViaHost' -v`
Expected: FAIL (compile error — `TunnelForm` has no `ViaHost` field; `Parse` doesn't set `Tunnel.ViaHost`).

- [ ] **Step 3: Implement**

In `internal/gui/form.go`, add the `ViaHost` field to `TunnelForm`. Place it right after the `Via` field:

```go
	Via        string
	ViaHost    string // name of a config Host entry (new model); wins over Via/Jump
	SSHOptions string // multiline key=value, excluding IdentityFile
```

In `Parse`, set `ViaHost` on the tunnel literal and make it suppress legacy routing. Change the `config.Tunnel{...}` literal to include `ViaHost`:

```go
	t := config.Tunnel{
		Name:      strings.TrimSpace(f.Name),
		Group:     strings.TrimSpace(f.Group),
		Local:     strings.TrimSpace(f.LocalPort),
		Remote:    strings.TrimSpace(f.DestHost) + ":" + strings.TrimSpace(f.DestPort),
		Via:       strings.TrimSpace(f.Via),
		ViaHost:   strings.TrimSpace(f.ViaHost),
		Autostart: f.Autostart,
	}
```

Then change the routing switch so `ViaHost` wins and clears legacy fields. Replace the existing `switch { ... }` block:

```go
	switch {
	case t.ViaHost != "":
		// New model: via_host wins. Drop any legacy via/jump entirely.
		t.Via = ""
		t.Jump = nil
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
```

In `ToForm`, carry `ViaHost` into the form. Add it to the initial `TunnelForm{...}` literal:

```go
	f := TunnelForm{
		Name:      t.Name,
		Group:     t.Group,
		LocalPort: t.Local,
		Via:       t.Via,
		ViaHost:   t.ViaHost,
		Autostart: t.Autostart,
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -v`
Expected: PASS (new tests green; all existing `form_test.go` tests — `ViaWins`, `JumpFields`, round-trips — still green because a legacy tunnel has `ViaHost == ""`).

- [ ] **Step 5: Commit**

```bash
git add internal/gui/form.go internal/gui/form_test.go
git commit -m "feat(gui): TunnelForm.ViaHost — Parse emits via_host and drops legacy fields

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: `RouteViaHost` — route inference + new-blank default

**Files:**
- Modify: `internal/gui/formroute.go`
- Test: `internal/gui/formroute_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/gui/formroute_test.go`:

```go
package gui

import "testing"

func TestRouteOf_ViaHost(t *testing.T) {
	if got := RouteOf(TunnelForm{ViaHost: "entryA"}); got != RouteViaHost {
		t.Fatalf("via_host form route = %q, want %q", got, RouteViaHost)
	}
}

func TestRouteOf_LegacyStillWorks(t *testing.T) {
	if got := RouteOf(TunnelForm{Via: "bastion"}); got != RouteRelay {
		t.Fatalf("via form route = %q, want %q", got, RouteRelay)
	}
	if got := RouteOf(TunnelForm{JumpHost: "j"}); got != RouteDirect {
		t.Fatalf("jump form route = %q, want %q", got, RouteDirect)
	}
	if got := RouteOf(TunnelForm{RawJump: []string{"a@h:22"}}); got != RouteDirect {
		t.Fatalf("raw jump form route = %q, want %q", got, RouteDirect)
	}
}

func TestRouteOf_NewBlankDefaultsToViaHost(t *testing.T) {
	if got := RouteOf(TunnelForm{}); got != RouteViaHost {
		t.Fatalf("blank form route = %q, want %q (new tunnels default to host model)", got, RouteViaHost)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run RouteOf -v`
Expected: FAIL (compile error — `RouteViaHost` undefined; blank currently returns `""`).

- [ ] **Step 3: Implement**

In `internal/gui/formroute.go`, add the constant to the existing `const (...)` block:

```go
const (
	RouteDirect  = "direct" // ssh logs into the target itself (optionally via a jump host)
	RouteRelay   = "relay"  // ssh logs into a configured relay (ssh alias / via), which forwards
	RouteViaHost = "host"   // ssh logs into a saved Host entry (new model), which forwards
)
```

Rewrite `RouteOf` so `ViaHost` wins, legacy paths are preserved, and a brand-new blank form defaults to the host model:

```go
// RouteOf infers the initial route for an existing tunnel form. A tunnel with a
// via_host is the new host model; a via alias is a legacy relay; an inline jump
// is legacy direct. A brand-new blank form (no via_host, no via, no jump, no
// name) defaults to the host model so new tunnels use saved hosts.
func RouteOf(f TunnelForm) string {
	if strings.TrimSpace(f.ViaHost) != "" {
		return RouteViaHost
	}
	if strings.TrimSpace(f.Via) != "" {
		return RouteRelay
	}
	if strings.TrimSpace(f.JumpHost) != "" || len(f.RawJump) > 0 {
		return RouteDirect
	}
	return RouteViaHost
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run RouteOf -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/formroute.go internal/gui/formroute_test.go
git commit -m "feat(gui): RouteViaHost route + new-tunnel default to host model

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: `CheckRoute` validates the via_host route

**Files:**
- Modify: `internal/gui/formroute.go`
- Test: `internal/gui/formroute_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/gui/formroute_test.go`:

```go
func TestCheckRoute_ViaHost_RequiresChosenHost(t *testing.T) {
	f := TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432"} // no ViaHost
	errs, _ := CheckRoute(RouteViaHost, f)
	if errs["viaHost"] == "" {
		t.Fatalf("expected a viaHost error when no host chosen, got %v", errs)
	}
}

func TestCheckRoute_ViaHost_OK(t *testing.T) {
	f := TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432", ViaHost: "entryA"}
	errs, _ := CheckRoute(RouteViaHost, f)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for a complete via_host form, got %v", errs)
	}
}

func TestCheckRoute_ViaHost_StillChecksCommonFields(t *testing.T) {
	// A chosen host doesn't excuse missing name/localPort/destHost/destPort.
	f := TunnelForm{ViaHost: "entryA"}
	errs, _ := CheckRoute(RouteViaHost, f)
	for _, key := range []string{"name", "localPort", "destHost", "destPort"} {
		if errs[key] == "" {
			t.Fatalf("expected error on %q for an otherwise-empty via_host form, got %v", key, errs)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run CheckRoute_ViaHost -v`
Expected: FAIL — the current `default:` branch sets `errs["route"] = "先选一种到达方式"` for an unknown route, so `errs["viaHost"]` is empty and the OK case wrongly reports a route error.

- [ ] **Step 3: Implement**

In `internal/gui/formroute.go`, add a `case RouteViaHost:` to the `switch route { ... }` in `CheckRoute`, before the `default:`:

```go
	case RouteViaHost:
		if strings.TrimSpace(f.ViaHost) == "" {
			errs["viaHost"] = "选择一台主机，或新建一台"
		}
```

(The shared name/localPort/destHost/destPort checks above the switch already run for every route, so they cover the via_host route too. The trailing `ssh_options` `-p` check after the switch is also unchanged.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -v`
Expected: PASS (legacy `CheckRoute` tests in `formcheck_test.go`, if any touch relay/direct, stay green — those routes are untouched).

- [ ] **Step 5: Commit**

```bash
git add internal/gui/formroute.go internal/gui/formroute_test.go
git commit -m "feat(gui): CheckRoute validates the via_host route

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `MigrateLegacyTunnel` — via-alias case (read ~/.ssh/config)

**Files:**
- Create: `internal/gui/migrate.go`
- Test: `internal/gui/migrate_test.go` (create)

This task introduces the pure migration entry point and implements the `via: <alias>` branch. `readSSHConfig` is injected so tests never touch the real `~/.ssh/config`. The naming scheme for synthesized hosts is **a slug of the source alias/host, made unique by suffixing `-2`, `-3`, … on collision** — implemented once in `uniqueHostName` and reused by Task 5.

- [ ] **Step 1: Write the failing test**

Create `internal/gui/migrate_test.go`:

```go
package gui

import (
	"strings"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

const sshConfigSample = `
Host entryA
    HostName 198.51.100.7
    Port 65522
    User userA
    IdentityFile ~/.ssh/idA
    ProxyJump bastionB

Host bastionB
    HostName 203.0.113.9
    User userB
`

func parseCfg(t *testing.T, yaml string) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cfg
}

func TestMigrateLegacyTunnel_ViaAlias(t *testing.T) {
	cfg := parseCfg(t, `
groups:
  prod:
    - {name: pg, local: "5432", remote: 10.0.1.5:5432, via: entryA}
`)
	read := func() ([]byte, error) { return []byte(sshConfigSample), nil }

	newHost, err := MigrateLegacyTunnel(cfg, "pg", read)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if newHost != "entryA" {
		t.Fatalf("newHostName = %q, want entryA", newHost)
	}

	// The tunnel is rewritten to via_host and its legacy via cleared.
	tn, _ := cfg.Tunnel("pg")
	if tn.ViaHost != "entryA" || tn.Via != "" {
		t.Fatalf("tunnel not rewritten: %+v", tn)
	}

	// entryA host captured, including ProxyJump -> jump because bastionB is also present.
	a, ok := cfg.Host("entryA")
	if !ok || a.Host != "198.51.100.7" || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" || a.Jump != "bastionB" {
		t.Fatalf("entryA host mismatch: %+v (ok=%v)", a, ok)
	}
	// bastionB pulled in as the jump target.
	b, ok := cfg.Host("bastionB")
	if !ok || b.Host != "203.0.113.9" || b.User != "userB" {
		t.Fatalf("bastionB host mismatch: %+v (ok=%v)", b, ok)
	}

	// The result must validate.
	if err := cfg.Validate(); err != nil {
		t.Fatalf("migrated config invalid: %v", err)
	}
}

func TestMigrateLegacyTunnel_ViaAlias_DropsUnimportedProxyJump(t *testing.T) {
	// ProxyJump points at an alias NOT present in ssh_config => jump left empty.
	const onlyEntry = `
Host entryA
    HostName 198.51.100.7
    Port 65522
    User userA
    ProxyJump ghostbastion
`
	cfg := parseCfg(t, `
groups:
  g:
    - {name: t1, local: "1", remote: x:5432, via: entryA}
`)
	read := func() ([]byte, error) { return []byte(onlyEntry), nil }
	if _, err := MigrateLegacyTunnel(cfg, "t1", read); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	a, _ := cfg.Host("entryA")
	if a.Jump != "" {
		t.Fatalf("jump should be empty when ProxyJump alias not imported, got %q", a.Jump)
	}
	if _, ok := cfg.Host("ghostbastion"); ok {
		t.Fatal("unresolved ProxyJump alias should not create a host")
	}
}

func TestMigrateLegacyTunnel_ViaAlias_NotFound(t *testing.T) {
	cfg := parseCfg(t, `
groups:
  g:
    - {name: t1, local: "1", remote: x:5432, via: missingalias}
`)
	read := func() ([]byte, error) { return []byte(sshConfigSample), nil }
	_, err := MigrateLegacyTunnel(cfg, "t1", read)
	if err == nil || !strings.Contains(err.Error(), "missingalias") {
		t.Fatalf("expected an error naming the missing alias, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run MigrateLegacyTunnel_ViaAlias -v`
Expected: FAIL (compile error — `MigrateLegacyTunnel` undefined).

- [ ] **Step 3: Implement**

Create `internal/gui/migrate.go`:

```go
package gui

import (
	"fmt"
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
```

> Note on `migrateJump`: it is referenced here but implemented in Task 5. To keep this task compiling and its tests passing in isolation, add a temporary stub now and replace it in Task 5:
>
> ```go
> // migrateJump is implemented in Task 5.
> func migrateJump(cfg *config.Config, t config.Tunnel) (string, error) {
> 	return "", fmt.Errorf("inline-jump migration not yet implemented")
> }
> ```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run MigrateLegacyTunnel_ViaAlias -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/migrate.go internal/gui/migrate_test.go
git commit -m "feat(gui): MigrateLegacyTunnel via-alias case (read ~/.ssh/config)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `MigrateLegacyTunnel` — inline-jump case

**Files:**
- Modify: `internal/gui/migrate.go` (replace the `migrateJump` stub)
- Test: `internal/gui/migrate_test.go`

**Naming scheme (documented):** For an inline `Jump` chain `[hopN, …, hop1]` reaching endpoint `t.Remote`, hopd synthesizes one host per hop plus an **endpoint host** for the tunnel's own SSH target. ssh's `-J a,b` means "through a, then b". The endpoint host's `jump` points at the first hop; each hop's `jump` points at the next; the last hop has empty `jump`. Each host is named by `uniqueHostName(slug(...))`: hop hosts from their `host` part, the endpoint host from `slug(destHost) + "-entry"` (so it never collides with the forward target's own slug). The tunnel's `ViaHost` becomes the endpoint host.

> Important: a legacy inline-jump tunnel ssh-es into the **forward target itself** (`BuildArgs` uses `targetHost(t)` as the ssh destination and `-J` for the chain). So the endpoint host's `Host` is the tunnel's `DestHost`, with default port 22 and no user (legacy inline jump never carried an endpoint user/port — that gap is exactly what the host model fixes; the user fills those in afterward).

- [ ] **Step 1: Write the failing test**

Add to `internal/gui/migrate_test.go`:

```go
func TestMigrateLegacyTunnel_InlineJump_SingleHop(t *testing.T) {
	cfg := parseCfg(t, `
groups:
  g:
    - {name: rdp, local: "13389", remote: 203.0.113.10:3389, jump: ["root@198.51.100.20:65532"]}
`)
	newHost, err := MigrateLegacyTunnel(cfg, "rdp", func() ([]byte, error) { return nil, nil })
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tn, _ := cfg.Tunnel("rdp")
	if tn.ViaHost != newHost || len(tn.Jump) != 0 {
		t.Fatalf("tunnel not rewritten: %+v (newHost=%q)", tn, newHost)
	}

	// Endpoint host = the tunnel's SSH target (the forward dest host).
	endpoint, ok := cfg.Host(newHost)
	if !ok || endpoint.Host != "203.0.113.10" || endpoint.Port != 22 {
		t.Fatalf("endpoint host mismatch: %+v (ok=%v)", endpoint, ok)
	}
	// One jump hop host capturing user/host/port.
	hop, ok := cfg.Host(endpoint.Jump)
	if !ok || hop.Host != "198.51.100.20" || hop.Port != 65532 || hop.User != "root" {
		t.Fatalf("hop host mismatch: %+v (ok=%v, jump=%q)", hop, ok, endpoint.Jump)
	}
	if hop.Jump != "" {
		t.Fatalf("single hop should have empty jump, got %q", hop.Jump)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("migrated config invalid: %v", err)
	}
}

func TestMigrateLegacyTunnel_InlineJump_MultiHop(t *testing.T) {
	cfg := parseCfg(t, `
groups:
  g:
    - {name: t1, local: "1", remote: 10.0.0.9:5432, jump: ["a@h1:22", "b@h2:2200"]}
`)
	newHost, err := MigrateLegacyTunnel(cfg, "t1", func() ([]byte, error) { return nil, nil })
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	endpoint, _ := cfg.Host(newHost)
	// endpoint -> h1 -> h2 -> ""
	h1, _ := cfg.Host(endpoint.Jump)
	if h1.Host != "h1" || h1.User != "a" || h1.Port != 22 {
		t.Fatalf("first hop mismatch: %+v", h1)
	}
	h2, _ := cfg.Host(h1.Jump)
	if h2.Host != "h2" || h2.User != "b" || h2.Port != 2200 {
		t.Fatalf("second hop mismatch: %+v", h2)
	}
	if h2.Jump != "" {
		t.Fatalf("last hop should have empty jump, got %q", h2.Jump)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("migrated config invalid: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run MigrateLegacyTunnel_InlineJump -v`
Expected: FAIL (the stub returns "not yet implemented").

- [ ] **Step 3: Implement**

In `internal/gui/migrate.go`, add `strconv` to the imports:

```go
import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)
```

Replace the `migrateJump` stub with the real implementation:

```go
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
```

> `splitJumpUser` already exists in `internal/gui/form.go` (same package) — reuse it; do not redefine.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -v`
Expected: PASS (all migrate + form + route tests green).

- [ ] **Step 5: Commit**

```bash
git add internal/gui/migrate.go internal/gui/migrate_test.go
git commit -m "feat(gui): MigrateLegacyTunnel inline-jump case (chain + endpoint hosts)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: `ipc.TunnelStatus.ViaHost` field

**Files:**
- Modify: `internal/ipc/protocol.go`
- Test: none yet (covered by Task 7's runner test; the field add is a one-liner with no behavior).

- [ ] **Step 1: Implement**

In `internal/ipc/protocol.go`, add `ViaHost` to `TunnelStatus` right after `Via`:

```go
type TunnelStatus struct {
	Name       string `json:"name"`
	Group      string `json:"group"`
	State      string `json:"state"`
	Local      string `json:"local"`
	Remote     string `json:"remote"`
	Via        string `json:"via,omitempty"`
	ViaHost    string `json:"via_host,omitempty"`
	UptimeSec  int64  `json:"uptime_sec"`
	Reconnects int    `json:"reconnects"`
	LastError  string `json:"last_error,omitempty"`
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/ipc/ && go test ./internal/ipc/...`
Expected: builds; existing ipc tests (if any) pass.

- [ ] **Step 3: Commit**

```bash
git add internal/ipc/protocol.go
git commit -m "feat(ipc): add ViaHost to TunnelStatus

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Daemon populates `TunnelStatus.ViaHost`

**Files:**
- Modify: `internal/tunnel/runner.go` (the `Snapshot()` method's `ipc.TunnelStatus{}` literal)
- Test: `internal/tunnel/runner_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/tunnel/runner_test.go` (white-box `package tunnel`):

```go
func TestSnapshotCarriesViaHost(t *testing.T) {
	tun := config.Tunnel{Name: "pg", Local: "5432", Remote: "10.0.1.5:5432", ViaHost: "entryA"}
	r := NewRunner(tun, "/usr/bin/ssh", time.Second, time.Minute)
	if got := r.Snapshot().ViaHost; got != "entryA" {
		t.Fatalf("Snapshot().ViaHost = %q, want entryA", got)
	}
}
```

If `runner_test.go` does not yet exist as `package tunnel`, add this test to whichever existing white-box file in the package uses `package tunnel` (e.g. the file added in Plan 1's Task 7). Ensure imports include `"time"` and `"github.com/GavinYangAI/hopd/internal/config"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tunnel/ -run TestSnapshotCarriesViaHost -v`
Expected: FAIL (`Snapshot().ViaHost` is the zero value because the literal doesn't set it).

- [ ] **Step 3: Implement**

In `internal/tunnel/runner.go`, in `Snapshot()`, add `ViaHost` to the returned `ipc.TunnelStatus{}` literal right after `Via`:

```go
	return ipc.TunnelStatus{
		Name:       r.tunnel.Name,
		Group:      r.tunnel.Group,
		State:      r.state.String(),
		Local:      r.tunnel.Local,
		Remote:     r.tunnel.Remote,
		Via:        r.tunnel.Via,
		ViaHost:    r.tunnel.ViaHost,
		UptimeSec:  uptime,
		Reconnects: r.reconnects,
		LastError:  r.lastErr,
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tunnel/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tunnel/runner.go internal/tunnel/runner_test.go
git commit -m "feat(tunnel): populate TunnelStatus.ViaHost in Snapshot

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `forwardDiagram` renders the via_host node

**Files:**
- Modify: `internal/guiapp/card.go`
- Test: `internal/guiapp/card_test.go` (create)

The diagram should show `本机 :port → [主机 entryA] → svc host:port` when `ViaHost` is set, while legacy `Via` continues to render the `中继` node unchanged. Since the diagram is built from canvas objects, the test walks the returned container for a `diagNode` whose caption is "主机" and value is the host name.

- [ ] **Step 1: Write the failing test**

Create `internal/guiapp/card_test.go`:

```go
package guiapp

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

// diagramTexts flattens all canvas.Text strings in a forwardDiagram for asserting.
func diagramTexts(obj fyne.CanvasObject) []string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *canvas.Text:
			out = append(out, v.Text)
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *container.Scroll:
			walk(v.Content)
		}
	}
	walk(obj)
	return out
}

func containsText(texts []string, want string) bool {
	for _, s := range texts {
		if s == want {
			return true
		}
	}
	return false
}

func TestForwardDiagram_ViaHostNode(t *testing.T) {
	_ = test.NewApp()
	obj := forwardDiagram(ipc.TunnelStatus{
		Name: "pg", Local: "5432", Remote: "10.0.1.5:5432", ViaHost: "entryA",
	})
	texts := diagramTexts(obj)
	if !containsText(texts, "主机") {
		t.Fatalf("expected a 主机 caption node, got texts %v", texts)
	}
	if !containsText(texts, "entryA") {
		t.Fatalf("expected the host name entryA in the diagram, got %v", texts)
	}
}

func TestForwardDiagram_LegacyViaUnchanged(t *testing.T) {
	_ = test.NewApp()
	obj := forwardDiagram(ipc.TunnelStatus{
		Name: "t1", Local: "1", Remote: "h:2", Via: "bastion",
	})
	texts := diagramTexts(obj)
	if !containsText(texts, "中继") || !containsText(texts, "bastion") {
		t.Fatalf("legacy via diagram changed: %v", texts)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestForwardDiagram -v`
Expected: FAIL — `TestForwardDiagram_ViaHostNode` fails (no 主机 node); the legacy test passes already.

- [ ] **Step 3: Implement**

In `internal/guiapp/card.go`, update `forwardDiagram` to add the via_host node. Replace the `if strings.TrimSpace(t.Via) != "" { ... }` block:

```go
func forwardDiagram(t ipc.TunnelStatus) fyne.CanvasObject {
	nodes := []fyne.CanvasObject{
		diagNode("本机", ":"+localPart(t.Local), false),
	}
	switch {
	case strings.TrimSpace(t.ViaHost) != "":
		nodes = append(nodes, arrow(), diagNode("主机", t.ViaHost, true))
	case strings.TrimSpace(t.Via) != "":
		nodes = append(nodes, arrow(), diagNode("中继", t.Via, true))
	}
	nodes = append(nodes, arrow(), diagNode(svcFor(t.Remote), valueOr(t.Remote, "—"), false))
	return container.NewHBox(nodes...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -run TestForwardDiagram -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/card.go internal/guiapp/card_test.go
git commit -m "feat(guiapp): forwardDiagram renders the via_host node

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Thread host names into the edit form + via_host picker (with "+ 新建主机")

**Files:**
- Modify: `internal/guiapp/editdialog.go`
- Modify: `internal/guiapp/window.go` (callers `addTunnel`/`editTunnel`)
- Test: `internal/guiapp/editdialog_test.go` (update existing callers + add new tests)

> **DEPENDS ON PLAN 2:** the "+ 新建主机" button calls `showHostDialog(...)`. Do not start this task until Plan 2 has landed `showHostDialog`.
>
> **BACK-COMPAT — existing tests MUST be updated in this same task:** changing `newEditForm`/`showEditDialog` signatures breaks the four existing `newEditForm(...)` calls in `editdialog_test.go` (`TestNewEditForm_PrefillsAndReads`, `_CarriesAutostart`, `_PreservesRawJump`, `_EntriesDoNotTrapScroll`) and the two `showEditDialog(...)` calls in `window.go`. All are updated below in Steps 3a–3c. Run the whole `guiapp` package after, not just the new tests.

The new signatures take the list of saved host names (for the picker) and a callback used by "+ 新建主机":

- `newEditForm(f gui.TunnelForm, hostNames []string, onNewHost func(after func(created string))) *editForm`
- `showEditDialog(win fyne.Window, title string, initial gui.TunnelForm, hostNames []string, onNewHost func(after func(created string)), onSubmit func(gui.TunnelForm) error)`

`onNewHost(after)` opens the host dialog; when a host is created it calls `after(createdName)` so the form can refresh its picker options and select the new host. Callers wire `onNewHost` to `showHostDialog`. Tests pass `nil` for `onNewHost` (the button is simply inert under test).

- [ ] **Step 1: Write the failing test**

Add to `internal/guiapp/editdialog_test.go`:

```go
func TestNewEditForm_ViaHostPicker_ListsHostsAndValidates(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(
		gui.TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "10.0.1.5", DestPort: "5432", ViaHost: "entryA"},
		[]string{"entryA", "bastionB"},
		nil,
	)
	// The picker widget must exist and offer the supplied host names.
	if ef.viaHostSel == nil {
		t.Fatal("expected a via_host Select widget")
	}
	if len(ef.viaHostSel.Options) != 2 || ef.viaHostSel.Options[0] != "entryA" {
		t.Fatalf("picker options = %v, want [entryA bastionB]", ef.viaHostSel.Options)
	}
	if ef.viaHostSel.Selected != "entryA" {
		t.Fatalf("picker should preselect the form's ViaHost, got %q", ef.viaHostSel.Selected)
	}
	// A complete via_host form is valid (save enabled).
	if !ef.valid() {
		t.Fatal("a complete via_host form should be valid")
	}
}

func TestNewEditForm_NewBlank_DefaultsToViaHostRoute(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(gui.TunnelForm{}, []string{"entryA"}, nil)
	if ef.route != gui.RouteViaHost {
		t.Fatalf("new blank form route = %q, want %q", ef.route, gui.RouteViaHost)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run 'ViaHostPicker|NewBlank_DefaultsToViaHost' -v`
Expected: FAIL (compile error — `newEditForm` takes one arg; `ef.viaHostSel` undefined).

- [ ] **Step 3a: Update the editForm struct + newEditForm signature**

In `internal/guiapp/editdialog.go`, add fields to the `editForm` struct (after `via, sshOptions ...`):

```go
	via, sshOptions *widget.Entry
	autostart       *widget.Check
	rawJump         []string

	viaHostSel *widget.Select // via_host picker (new model)
	hostNames  []string       // available saved host names
	onNewHost  func(after func(created string))
	viaHostCard *routeCard
```

Change `newEditForm` to accept the host list + callback and build the picker:

```go
// newEditForm builds the guided form prefilled from f. hostNames seeds the
// via_host picker; onNewHost (may be nil) opens the host dialog and calls back
// with the created host name so the picker can refresh and select it.
func newEditForm(f gui.TunnelForm, hostNames []string, onNewHost func(after func(created string))) *editForm {
	ef := &editForm{
		name:       widget.NewEntry(),
		group:      widget.NewEntry(),
		localPort:  widget.NewEntry(),
		destHost:   widget.NewEntry(),
		destPort:   widget.NewEntry(),
		jumpHost:   widget.NewEntry(),
		jumpPort:   widget.NewEntry(),
		jumpUser:   widget.NewEntry(),
		keyFile:    widget.NewEntry(),
		via:        widget.NewEntry(),
		sshOptions: widget.NewMultiLineEntry(),
		autostart:  widget.NewCheck("开机自动连接（守护进程启动时自动建立此隧道）", nil),
		rawJump:    f.RawJump,
		route:      gui.RouteOf(f),
		captions:   map[string]*captionLabel{},
		hostNames:  append([]string(nil), hostNames...),
		onNewHost:  onNewHost,
	}
	ef.viaHostSel = widget.NewSelect(ef.hostNames, func(string) { ef.refresh() })
	if f.ViaHost != "" {
		ef.viaHostSel.SetSelected(f.ViaHost)
	}
	// ... existing placeholder loop, noWheelTrap, SetText calls, autostart unchanged ...
```

Keep the rest of `newEditForm` (placeholders, `noWheelTrap`, the `SetText` block, `ef.build(f)`, `ef.refresh()`, `return ef`) exactly as it is.

- [ ] **Step 3b: Build the via_host route card + expand panel; read the picker in value()**

In `editForm.build`, add a third route card and include it in section ③'s grid. Replace the route-card construction + `sec3` grid:

```go
	ef.viaHostCard = newRouteCard(
		"用一台已保存的主机", "推荐",
		"选一台你保存过的 SSH 主机（含端口/用户/密钥/跳板），hopd 登录它再转发端口。无需手改 ~/.ssh/config。",
		"ssh 主机  → 目标主机:端口",
		func() { ef.setRoute(gui.RouteViaHost) })
	ef.directCard = newRouteCard(
		"目标机器我能 SSH 登录", "经跳板直达",
		"目标主机自己开放了 SSH。hopd 直接登录它（或先穿过一台跳板再登录），然后转发端口。",
		"ssh -J 跳板  user@目标主机",
		func() { ef.setRoute(gui.RouteDirect) })
	ef.relayCard = newRouteCard(
		"目标在一台中继机后面", "经中继转发",
		"目标主机不开 SSH（比如内网里的一个数据库）。hopd 登录你配好的中继机，由它转发到目标。",
		"ssh 中继别名 → 目标主机:端口",
		func() { ef.setRoute(gui.RouteRelay) })
	ef.expandBox = container.NewVBox()
	sec3 := container.New(layoutStackV{gap: 11},
		sectionHeader(3, "怎么到达它？", "关键一步"),
		lead,
		ef.routeErr.obj,
		container.NewGridWithColumns(3, ef.viaHostCard.root, ef.directCard.root, ef.relayCard.root),
		ef.expandBox,
	)
```

In `value()`, read the picker into `ViaHost`. Add to the returned `gui.TunnelForm{...}`:

```go
		Via:        ef.via.Text,
		ViaHost:    ef.viaHostSel.Selected,
		SSHOptions: ef.sshOptions.Text,
```

- [ ] **Step 3c: Render the via_host fields in rebuildExpand + setRoute + preview**

In `setRoute`, also toggle the new card's active state:

```go
func (ef *editForm) setRoute(r string) {
	ef.route = r
	ef.viaHostCard.setActive(r == gui.RouteViaHost)
	ef.directCard.setActive(r == gui.RouteDirect)
	ef.relayCard.setActive(r == gui.RouteRelay)
	ef.rebuildExpand()
	ef.refresh()
}
```

In `rebuildExpand`, set the active states for all three cards and add the `RouteViaHost` case. Replace the head of `rebuildExpand` and add the case:

```go
func (ef *editForm) rebuildExpand() {
	ef.viaHostCard.setActive(ef.route == gui.RouteViaHost)
	ef.directCard.setActive(ef.route == gui.RouteDirect)
	ef.relayCard.setActive(ef.route == gui.RouteRelay)
	switch ef.route {
	case gui.RouteViaHost:
		note := infoNote("选一台已保存的主机；没有就点「+ 新建主机」。")
		newBtn := widget.NewButtonWithIcon("+ 新建主机", theme.ContentAddIcon(), ef.addNewHost)
		picker := container.NewBorder(nil, nil, nil, newBtn, ef.viaHostSel)
		ef.expandBox.Objects = []fyne.CanvasObject{
			expandPanel(container.NewVBox(note, ef.fieldWrap("主机", true, "选一台已保存的 SSH 主机", "viaHost", picker))),
		}
	case gui.RouteDirect:
		// ... existing direct case unchanged ...
	case gui.RouteRelay:
		// ... existing relay case unchanged ...
	default:
		ef.expandBox.Objects = nil
	}
	ef.expandBox.Refresh()
}
```

> `fieldWrap` is `field` renamed-or-aliased only if needed; the existing `field(label, required, help, key, entry)` already returns a labelled input with a caption — **use the existing `field` method**, not a new one. Replace `ef.fieldWrap(...)` above with `ef.field("主机", true, "选一台已保存的 SSH 主机", "viaHost", picker)`.

Add the `addNewHost` helper (placed near `pickKeyFile`):

```go
// addNewHost opens the host dialog (via onNewHost) and, on creation, adds the
// new host to the picker and selects it.
func (ef *editForm) addNewHost() {
	if ef.onNewHost == nil {
		return
	}
	ef.onNewHost(func(created string) {
		if created == "" {
			return
		}
		ef.hostNames = append(ef.hostNames, created)
		ef.viaHostSel.Options = ef.hostNames
		ef.viaHostSel.SetSelected(created)
		ef.viaHostSel.Refresh()
		ef.refresh()
	})
}
```

In `rebuildPreview`, add the via_host node. Add a case to the `switch ef.route { ... }`:

```go
	switch ef.route {
	case gui.RouteViaHost:
		if val.ViaHost != "" {
			nodes = append(nodes, arrow(), diagNode("主机", val.ViaHost, true))
		}
	case gui.RouteRelay:
		if val.Via != "" {
			nodes = append(nodes, arrow(), diagNode("中继", val.Via, true))
		}
	case gui.RouteDirect:
		if val.JumpHost != "" {
			nodes = append(nodes, arrow(), diagNode("跳板", val.JumpHost, true))
		}
	}
```

- [ ] **Step 3d: Update `showEditDialog` signature**

Replace `showEditDialog` to thread the new args into `newEditForm`:

```go
// showEditDialog presents the guided form modally. hostNames seeds the via_host
// picker; onNewHost (may be nil) opens the host dialog. onSubmit receives the
// edited form when the user saves; returning an error keeps the dialog open.
func showEditDialog(win fyne.Window, title string, initial gui.TunnelForm, hostNames []string, onNewHost func(after func(created string)), onSubmit func(gui.TunnelForm) error) {
	ef := newEditForm(initial, hostNames, onNewHost)
	dlg := dialog.NewCustomWithoutButtons(title, ef.root, win)
	dlg.Resize(fyne.NewSize(720, 620))
	ef.onCancel = dlg.Hide
	ef.onSave = func() {
		if !ef.valid() {
			ef.refresh()
			return
		}
		if err := onSubmit(ef.value()); err != nil {
			if errors.Is(err, gui.ErrReloadAfterSave) {
				dlg.Hide()
				dialog.ShowInformation("已保存", "配置已保存。daemon 未运行，将在它启动后生效。", win)
				return
			}
			dialog.ShowError(err, win)
			return
		}
		dlg.Hide()
	}
	dlg.Show()
}
```

- [ ] **Step 3e: Update window.go callers**

In `internal/guiapp/window.go`, add a helper to read host names from the store and an `onNewHost` adapter, then update `addTunnel`/`editTunnel`.

Add helpers near `addTunnel`:

```go
// hostNames returns the saved host names for the via_host picker (empty on load
// error so the dialog still opens).
func (d *dashboard) hostNames() []string {
	cfg, err := d.store.Load()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Hosts()))
	for name := range cfg.Hosts() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// onNewHost opens the host dialog and reports the created host name back to the
// form via after(). It reuses the Plan-2 host dialog.
func (d *dashboard) onNewHost(after func(created string)) {
	showHostDialog(d.win, d.store, gui.HostForm{}, "", func() {
		// showHostDialog persists the host; surface the most-recently-added name
		// by diffing is overkill — instead, re-list and let the form keep its
		// current selection if the user named it. We pass the picker the full
		// refreshed list; selection of the new host happens when the user picks
		// it. To preselect, the host dialog could return the name; until then we
		// refresh options only.
		after("")
		_ = after
	})
}
```

> Refinement note: if Plan-2's `showHostDialog` `onDone` exposes the created name, change `onNewHost` to call `after(createdName)` so the new host is auto-selected. The contract above (`onDone func()`) does not carry the name, so the safe behavior is to refresh options via a re-list; the picker keeps its current value. To still refresh options after creation, replace the body with a re-list:
>
> ```go
> func (d *dashboard) onNewHost(after func(created string)) {
> 	showHostDialog(d.win, d.store, gui.HostForm{}, "", func() { after("") })
> }
> ```
>
> Use this simpler form. (Auto-select is a nice-to-have deferred to when the host dialog returns the name.)

Update `addTunnel`:

```go
func (d *dashboard) addTunnel() {
	if d.store == nil {
		return
	}
	showEditDialog(d.win, "新增隧道", gui.TunnelForm{Autostart: true}, d.hostNames(), d.onNewHost, func(f gui.TunnelForm) error {
		tn, err := f.Parse()
		if err != nil {
			return err
		}
		cfg, err := d.store.Load()
		if err != nil {
			return err
		}
		if err := cfg.AddTunnel(tn); err != nil {
			return err
		}
		return d.store.Save(cfg)
	})
}
```

Update `editTunnel`'s `showEditDialog` call (keep the surrounding load/lookup logic) to pass `d.hostNames(), d.onNewHost`:

```go
	showEditDialog(d.win, "编辑隧道", gui.ToForm(cur), d.hostNames(), d.onNewHost, func(f gui.TunnelForm) error {
		tn, err := f.Parse()
		if err != nil {
			return err
		}
		c, err := d.store.Load()
		if err != nil {
			return err
		}
		if err := c.UpdateTunnel(oldName, tn); err != nil {
			return err
		}
		return d.store.Save(c)
	})
```

- [ ] **Step 3f: Update the four existing editdialog_test.go callers**

In `internal/guiapp/editdialog_test.go`, change every `newEditForm(<form>)` to `newEditForm(<form>, nil, nil)`:

- `TestNewEditForm_PrefillsAndReads`: `ef := newEditForm(initial, nil, nil)`
- `TestNewEditForm_CarriesAutostart`: both `newEditForm(...)` calls → add `, nil, nil`
- `TestNewEditForm_PreservesRawJump`: `ef := newEditForm(initial, nil, nil)`
- `TestNewEditForm_EntriesDoNotTrapScroll`: `ef := newEditForm(gui.TunnelForm{}, nil, nil)`

(No assertion changes are needed; `value().ViaHost` is `""` for these legacy/blank forms, and `RawJump` handling is unchanged. The `PrefillsAndReads` test already overwrites `got.RawJump` before comparing, and `ViaHost` is `""` on both sides.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/guiapp/ -v`
Expected: PASS (new picker tests + the four updated existing tests + the card tests + all other guiapp tests).

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/editdialog.go internal/guiapp/window.go internal/guiapp/editdialog_test.go
git commit -m "feat(guiapp): via_host route card + host picker; thread host list into edit form

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Legacy tunnel read-mostly view + "迁移为主机" button

**Files:**
- Modify: `internal/guiapp/editdialog.go`
- Modify: `internal/guiapp/window.go` (migrate store path)
- Test: `internal/guiapp/editdialog_test.go`

When editing a **legacy** tunnel (route inferred as relay or direct, i.e. `ViaHost == ""` and via/jump set), the route cards are disabled and the legacy fields are shown read-only (via `Entry.Disable()`), with a prominent "迁移为主机" button. Clicking it runs `MigrateLegacyTunnel`, saves, and reopens the dialog in via_host mode. The dialog gets an `onMigrate func() error` callback (wired in window.go to the store + reopen). Tests pass `nil` and assert the legacy entries are disabled.

- [ ] **Step 1: Write the failing test**

Add to `internal/guiapp/editdialog_test.go`:

```go
func TestNewEditForm_LegacyTunnelIsReadMostly(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(
		gui.TunnelForm{Name: "old", LocalPort: "1", DestHost: "h", DestPort: "2", Via: "bastion"},
		[]string{"entryA"}, nil,
	)
	if !ef.legacy {
		t.Fatal("a via/jump tunnel should be detected as legacy")
	}
	// The legacy via entry is shown disabled (read-only).
	if !ef.via.Disabled() {
		t.Fatal("legacy via entry should be disabled in read-mostly mode")
	}
	// The migrate button exists.
	if ef.migrateBtn == nil {
		t.Fatal("expected a 迁移为主机 button for a legacy tunnel")
	}
}

func TestNewEditForm_ViaHostTunnelIsNotLegacy(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(
		gui.TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432", ViaHost: "entryA"},
		[]string{"entryA"}, nil,
	)
	if ef.legacy {
		t.Fatal("a via_host tunnel must not be flagged legacy")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run 'LegacyTunnelIsReadMostly|ViaHostTunnelIsNotLegacy' -v`
Expected: FAIL (compile error — `ef.legacy`, `ef.migrateBtn`, `onMigrate` undefined).

- [ ] **Step 3a: Add the legacy fields + detection**

In `internal/guiapp/editdialog.go`, add to the `editForm` struct:

```go
	legacy     bool                  // editing a via/jump tunnel (read-mostly)
	migrateBtn *widget.Button
	onMigrate  func() error
```

In `newEditForm`, set `legacy` from the route just after computing it (`ef.route = gui.RouteOf(f)`):

```go
	ef.legacy = ef.route == gui.RouteRelay || ef.route == gui.RouteDirect
```

And widen the `newEditForm` signature to take `onMigrate` (it's only meaningful for legacy edits; tests pass nil):

```go
func newEditForm(f gui.TunnelForm, hostNames []string, onNewHost func(after func(created string)), onMigrate func() error) *editForm {
```

Set it in the struct literal: `onMigrate: onMigrate,` (add to the literal alongside `onNewHost`).

- [ ] **Step 3b: Read-mostly rendering + migrate button**

In `build`, after `ef.rebuildExpand()`, if legacy, disable the legacy inputs and route cards. Add right before `ef.root = container.NewBorder(...)`:

```go
	if ef.legacy {
		for _, e := range []*widget.Entry{ef.via, ef.jumpHost, ef.jumpPort, ef.jumpUser, ef.keyFile} {
			e.Disable()
		}
		ef.viaHostCard.disable()
		ef.directCard.disable()
		ef.relayCard.disable()
		ef.migrateBtn = widget.NewButtonWithIcon("迁移为主机", theme.ContentCopyIcon(), func() { ef.doMigrate() })
		ef.migrateBtn.Importance = widget.HighImportance
		banner := legacyMigrateBanner(ef.migrateBtn)
		ef.expandBox.Objects = append([]fyne.CanvasObject{banner}, ef.expandBox.Objects...)
		ef.expandBox.Refresh()
	}
```

Add a `disable()` method to `routeCard` (near `setActive`):

```go
// disable greys a card so it can't be tapped (used in legacy read-mostly mode).
func (rc *routeCard) disable() {
	rc.bg.FillColor = pal.surface2
	rc.bg.StrokeColor = pal.border
	rc.bg.StrokeWidth = 1
	rc.radio.FillColor = transparent
	rc.radio.StrokeColor = pal.text3
	rc.bg.Refresh()
	rc.radio.Refresh()
}
```

Add the migrate trigger + banner helper:

```go
// doMigrate runs the injected migration (which saves + reopens). It surfaces
// errors via the standard dialog path.
func (ef *editForm) doMigrate() {
	if ef.onMigrate == nil {
		return
	}
	if err := ef.onMigrate(); err != nil {
		if win := currentWindow(); win != nil {
			dialog.ShowError(err, win)
		}
	}
}

// legacyMigrateBanner is the in-dialog prompt shown over a legacy tunnel's
// (disabled) fields, with the migrate action.
func legacyMigrateBanner(btn *widget.Button) fyne.CanvasObject {
	msg := widget.NewLabel("这是一条旧式隧道（via/jump）。仍可运行，但建议迁移成「已保存的主机」，以便填写端口/用户/密钥并复用。")
	msg.Wrapping = fyne.TextWrapWord
	bg := roundRect(pal.warnSoft, 10, 1, pal.warnEdge)
	body := container.NewBorder(nil, nil, nil, btn, msg)
	return container.NewStack(bg, container.New(layoutPadXY{px: 12, py: 10}, body))
}

// currentWindow returns the last open Fyne window (for showing dialogs from the
// form, which doesn't hold a window reference).
func currentWindow() fyne.Window {
	wins := fyne.CurrentApp().Driver().AllWindows()
	if len(wins) == 0 {
		return nil
	}
	return wins[len(wins)-1]
}
```

- [ ] **Step 3c: Thread `onMigrate` through showEditDialog + window.go**

Update `showEditDialog` to accept and pass `onMigrate`:

```go
func showEditDialog(win fyne.Window, title string, initial gui.TunnelForm, hostNames []string, onNewHost func(after func(created string)), onMigrate func() error, onSubmit func(gui.TunnelForm) error) {
	ef := newEditForm(initial, hostNames, onNewHost, onMigrate)
	dlg := dialog.NewCustomWithoutButtons(title, ef.root, win)
	// ... unchanged body, but the migrate path needs to close this dialog and reopen.
	ef.onCancel = dlg.Hide
	// keep a way to close from doMigrate's reopen:
	ef.closeDialog = dlg.Hide
	// ... rest unchanged ...
}
```

Add `closeDialog func()` to the struct and call it from the window-side migrate adapter (below). In `doMigrate`, after a successful migrate the window adapter reopens the dialog, so `doMigrate` should close first:

```go
func (ef *editForm) doMigrate() {
	if ef.onMigrate == nil {
		return
	}
	if err := ef.onMigrate(); err != nil {
		if win := currentWindow(); win != nil {
			dialog.ShowError(err, win)
		}
		return
	}
	if ef.closeDialog != nil {
		ef.closeDialog()
	}
}
```

In `internal/guiapp/window.go`, update both `showEditDialog` callers. `addTunnel` is never legacy, so pass `nil` for `onMigrate`:

```go
	showEditDialog(d.win, "新增隧道", gui.TunnelForm{Autostart: true}, d.hostNames(), d.onNewHost, nil, func(f gui.TunnelForm) error {
		// ... unchanged ...
	})
```

`editTunnel` wires `onMigrate` to migrate + reopen:

```go
	oldName := d.selName
	migrate := func() error {
		cfg, err := d.store.Load()
		if err != nil {
			return err
		}
		if _, err := gui.MigrateLegacyTunnel(cfg, oldName, readUserSSHConfig); err != nil {
			return err
		}
		if err := d.store.Save(cfg); err != nil && !errors.Is(err, gui.ErrReloadAfterSave) {
			return err
		}
		// Reopen on the now-migrated tunnel (now a via_host tunnel).
		fresh, _ := d.store.Load()
		cur2, ok := fresh.Tunnel(oldName)
		if !ok {
			return fmt.Errorf("tunnel %q vanished after migration", oldName)
		}
		d.openEditDialog(fresh, oldName, cur2)
		return nil
	}
	showEditDialog(d.win, "编辑隧道", gui.ToForm(cur), d.hostNames(), d.onNewHost, migrate, func(f gui.TunnelForm) error {
		// ... unchanged submit body ...
	})
```

To avoid duplicating the editTunnel dialog-open block, extract the dialog open into a small method and call it from both `editTunnel` and the migrate reopen:

```go
// openEditDialog shows the editor for an existing tunnel cur (named name) loaded
// from cfg, wiring host list, new-host, migrate and save.
func (d *dashboard) openEditDialog(cfg *config.Config, name string, cur config.Tunnel) {
	migrate := func() error { /* same body as above, closing over name */ ... }
	showEditDialog(d.win, "编辑隧道", gui.ToForm(cur), d.hostNames(), d.onNewHost, migrate, func(f gui.TunnelForm) error {
		tn, err := f.Parse()
		if err != nil {
			return err
		}
		c, err := d.store.Load()
		if err != nil {
			return err
		}
		if err := c.UpdateTunnel(name, tn); err != nil {
			return err
		}
		return d.store.Save(c)
	})
}
```

Then `editTunnel` becomes: load cfg, look up `cur`, call `d.openEditDialog(cfg, d.selName, cur)`. Add `config` and `fmt` imports if not present (`fmt` already imported; add `"github.com/GavinYangAI/hopd/internal/config"`).

Add the ssh_config reader used by migration (reads the real `~/.ssh/config`, the only place it's read):

```go
// readUserSSHConfig reads ~/.ssh/config for the migration import (read-only).
func readUserSSHConfig() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(home, ".ssh", "config"))
}
```

Add `"os"` and `"path/filepath"` to `window.go` imports.

- [ ] **Step 3d: Update the existing newEditForm callers in tests for the new 4th arg**

Every `newEditForm(form, nil, nil)` from Task 9 becomes `newEditForm(form, nil, nil, nil)`. Update all calls in `editdialog_test.go` (the four pre-existing tests + the two added in Task 9 + the two added in this task already use the 4-arg form). Grep to be sure:

Run: `grep -n "newEditForm(" internal/guiapp/editdialog_test.go`
Ensure every call has four arguments.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/guiapp/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/editdialog.go internal/guiapp/window.go internal/guiapp/editdialog_test.go
git commit -m "feat(guiapp): legacy tunnel read-mostly view + 迁移为主机 action

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: "测试连接" button (reuse gui.TestConnection on the chosen host)

**Files:**
- Modify: `internal/guiapp/editdialog.go`
- Test: `internal/guiapp/editdialog_test.go`

> **DEPENDS ON PLAN 2:** uses `gui.TestConnection`, `gui.CmdRunner`, `gui.TestConnResult`, and the default `execRunner`. Do not start until Plan 2 lands these.

The button is shown only in the via_host route (a saved host is needed to test). It builds a `*config.Config` containing the chosen host's chain, then calls `gui.TestConnection(ctx, cfg, hostName, execRunner)` and shows the result. To keep it testable, the runner is injectable on the form (`testRunner gui.CmdRunner`, defaulting to `execRunner`); the test sets a fake runner and asserts the result dialog text via a captured callback.

- [ ] **Step 1: Write the failing test**

Add to `internal/guiapp/editdialog_test.go`:

```go
func TestEditForm_TestConnection_UsesChosenHost(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(
		gui.TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432", ViaHost: "entryA"},
		[]string{"entryA"}, nil, nil,
	)
	// Inject a fake config provider + runner so no real ssh runs.
	ef.testCfg = func() (*config.Config, error) {
		return config.Parse([]byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 65522, user: userA}
groups:
  g:
    - {name: pg, local: "5432", remote: h:5432, via_host: entryA}
`))
	}
	var gotHost string
	ef.testRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(""), nil // success
	}
	ef.onTestResult = func(res gui.TestConnResult) { /* capture */ }
	// Capture which host TestConnection was asked about by wrapping testConn.
	ef.testConn = func(ctx context.Context, cfg *config.Config, host string, run gui.CmdRunner) gui.TestConnResult {
		gotHost = host
		return gui.TestConnResult{OK: true}
	}

	ef.runTest()
	if gotHost != "entryA" {
		t.Fatalf("test connection used host %q, want entryA", gotHost)
	}
}
```

> If Plan-2's `gui.TestConnResult` field name for success differs from `OK`, adjust the literal accordingly when implementing — keep the assertion on `gotHost`, which is contract-stable.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestEditForm_TestConnection -v`
Expected: FAIL (compile error — `ef.testCfg`, `ef.testRunner`, `ef.testConn`, `ef.onTestResult`, `ef.runTest` undefined).

- [ ] **Step 3: Implement**

In `internal/guiapp/editdialog.go`, add the seams + fields to `editForm`:

```go
	testBtn      *widget.Button
	testCfg      func() (*config.Config, error)                                                       // provides the config to test against
	testRunner   gui.CmdRunner                                                                         // ssh runner (defaults to execRunner)
	testConn     func(ctx context.Context, cfg *config.Config, host string, run gui.CmdRunner) gui.TestConnResult // defaults to gui.TestConnection
	onTestResult func(gui.TestConnResult)                                                              // result sink (defaults to a dialog)
```

Default the seams in `newEditForm` (after the struct literal, before `ef.build(f)`):

```go
	ef.testRunner = execRunner
	ef.testConn = gui.TestConnection
	ef.onTestResult = func(res gui.TestConnResult) {
		win := currentWindow()
		if win == nil {
			return
		}
		if res.OK {
			dialog.ShowInformation("连接成功", "已成功连到所选主机。", win)
		} else {
			dialog.ShowError(fmt.Errorf("连接失败：%s", res.Reason), win)
		}
	}
```

> Use whatever the Plan-2 `gui.TestConnResult` exposes for the failure reason; if it's `res.Stderr` or `res.Err` rather than `res.Reason`, adjust the message expression. Keep `res.OK` if that's the success field; otherwise use Plan-2's name.

Add `"context"` and `"github.com/GavinYangAI/hopd/internal/config"` to the imports.

Add the test button to the via_host expand panel. In `rebuildExpand`'s `case gui.RouteViaHost:`, append the button to the panel content:

```go
	case gui.RouteViaHost:
		note := infoNote("选一台已保存的主机；没有就点「+ 新建主机」。")
		newBtn := widget.NewButtonWithIcon("+ 新建主机", theme.ContentAddIcon(), ef.addNewHost)
		picker := container.NewBorder(nil, nil, nil, newBtn, ef.viaHostSel)
		ef.testBtn = widget.NewButtonWithIcon("测试连接", theme.ConfirmIcon(), ef.runTest)
		ef.expandBox.Objects = []fyne.CanvasObject{
			expandPanel(container.NewVBox(
				note,
				ef.field("主机", true, "选一台已保存的 SSH 主机", "viaHost", picker),
				container.NewHBox(ef.testBtn),
			)),
		}
```

Add `runTest`:

```go
// runTest tests the connection to the currently chosen via_host. It builds the
// config to test against (testCfg), runs gui.TestConnection with the injected
// runner, and reports via onTestResult.
func (ef *editForm) runTest() {
	host := ef.viaHostSel.Selected
	if host == "" {
		return
	}
	if ef.testCfg == nil {
		return
	}
	cfg, err := ef.testCfg()
	if err != nil {
		ef.onTestResult(gui.TestConnResult{Reason: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res := ef.testConn(ctx, cfg, host, ef.testRunner)
	ef.onTestResult(res)
}
```

> Add `"time"` to the imports. If `gui.TestConnResult` has no `Reason` field, set whatever the failure field is for the parse-error case, or drop that line and report via `dialog.ShowError` directly.

Wire `testCfg` from the window so the live button tests against the real saved config. In `window.go`'s `openEditDialog`/`addTunnel`, the form needs a config provider; the simplest seam is to default `testCfg` inside `newEditForm` to load from disk is not possible (no store ref). Instead, set it in `showEditDialog` via a parameter is heavy; **default `testCfg` to nil in `newEditForm` and have window.go set it after construction is also not possible** (form is built inside showEditDialog). Resolve by passing a `loadCfg func() (*config.Config, error)` into `showEditDialog` and assigning `ef.testCfg = loadCfg` there:

Add the param to `showEditDialog`:

```go
func showEditDialog(win fyne.Window, title string, initial gui.TunnelForm, hostNames []string,
	onNewHost func(after func(created string)), onMigrate func() error,
	loadCfg func() (*config.Config, error), onSubmit func(gui.TunnelForm) error) {
	ef := newEditForm(initial, hostNames, onNewHost, onMigrate)
	if loadCfg != nil {
		ef.testCfg = loadCfg
	}
	// ... rest unchanged ...
```

In `window.go`, pass `d.store.Load` as `loadCfg` in both `addTunnel` and `openEditDialog`:

```go
	showEditDialog(d.win, "新增隧道", gui.TunnelForm{Autostart: true}, d.hostNames(), d.onNewHost, nil, d.store.Load, func(f gui.TunnelForm) error { ... })
```
```go
	showEditDialog(d.win, "编辑隧道", gui.ToForm(cur), d.hostNames(), d.onNewHost, migrate, d.store.Load, func(f gui.TunnelForm) error { ... })
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/guiapp/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/editdialog.go internal/guiapp/window.go internal/guiapp/editdialog_test.go
git commit -m "feat(guiapp): 测试连接 button reuses gui.TestConnection on the chosen host

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: One-time legacy-migration hint banner (dismissal persisted in Preferences)

**Files:**
- Modify: `internal/guiapp/window.go`
- Test: `internal/guiapp/window_test.go` (append)

When any legacy tunnel exists in the snapshot and the user hasn't dismissed the hint, the dashboard body shows a dismissible banner above the cards pointing at migration ("有旧式隧道，可在编辑里「迁移为主机」"). Dismissal is stored under a Preferences key so it never reappears. A snapshot tunnel is "legacy" when it has `Via != ""` (the snapshot doesn't carry inline `jump`, but legacy `via` is the user-facing case; inline-jump tunnels still get the per-dialog banner from Task 10).

- [ ] **Step 1: Write the failing test**

Append to `internal/guiapp/window_test.go`:

```go
func TestHasLegacyTunnels(t *testing.T) {
	if hasLegacyTunnels(nil) {
		t.Fatal("nil snapshot has no legacy tunnels")
	}
	snap := []ipc.TunnelStatus{
		{Name: "a", ViaHost: "entryA"},
		{Name: "b", Via: "bastion"},
	}
	if !hasLegacyTunnels(snap) {
		t.Fatal("a tunnel with Via should count as legacy")
	}
	only := []ipc.TunnelStatus{{Name: "a", ViaHost: "entryA"}}
	if hasLegacyTunnels(only) {
		t.Fatal("via_host-only snapshot is not legacy")
	}
}
```

If `window_test.go` lacks the `ipc` import, add `"github.com/GavinYangAI/hopd/internal/ipc"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestHasLegacyTunnels -v`
Expected: FAIL (`hasLegacyTunnels` undefined).

- [ ] **Step 3: Implement**

In `internal/guiapp/window.go`, add the predicate and the banner constant:

```go
const legacyHintDismissedKey = "legacyMigrateHintDismissed"

// hasLegacyTunnels reports whether any tunnel in the snapshot still uses the
// legacy via alias (the migrate-hint trigger).
func hasLegacyTunnels(snap []ipc.TunnelStatus) bool {
	for _, t := range snap {
		if strings.TrimSpace(t.Via) != "" {
			return true
		}
	}
	return false
}
```

In `refreshBody`, after computing `filtered` and before building the group cards, prepend the banner when applicable. Insert just before the `var objs []fyne.CanvasObject` loop:

```go
	var objs []fyne.CanvasObject
	if hasLegacyTunnels(d.snap) && !d.app.Preferences().Bool(legacyHintDismissedKey) {
		objs = append(objs, d.legacyHintBanner())
	}
	for _, g := range groupOrder(filtered) {
		// ... existing loop unchanged ...
	}
```

Add the banner builder:

```go
// legacyHintBanner is the dismissible one-time prompt shown above the card list
// when legacy via tunnels exist. Dismissal is persisted in Preferences.
func (d *dashboard) legacyHintBanner() fyne.CanvasObject {
	msg := text("有旧式隧道（via）。在「编辑」里可一键「迁移为主机」，以便填端口/用户/密钥并复用。", 12.5, pal.text1, fyne.TextStyle{})
	dismiss := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		d.app.Preferences().SetBool(legacyHintDismissedKey, true)
		d.refreshBody()
	})
	dismiss.Importance = widget.LowImportance
	bg := roundRect(pal.warnSoft, 10, 1, pal.warnEdge)
	body := container.NewBorder(nil, nil, nil, dismiss, msg)
	return container.New(layoutPadXY{px: 14, py: 8},
		container.NewStack(bg, container.New(layoutPadXY{px: 12, py: 9}, body)))
}
```

(`d.app` is the stored `fyne.App` on the `dashboard` struct — already present.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/guiapp/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/window.go internal/guiapp/window_test.go
git commit -m "feat(guiapp): dismissible one-time legacy-migration hint banner

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 13: Search filter includes ViaHost + full-suite verification

**Files:**
- Modify: `internal/guiapp/window.go` (the `filtered` method)
- Test: `internal/guiapp/window_test.go` (append) + whole suite

- [ ] **Step 1: Write the failing test**

Append to `internal/guiapp/window_test.go`:

```go
func TestFiltered_MatchesViaHost(t *testing.T) {
	d := &dashboard{
		snap: []ipc.TunnelStatus{
			{Name: "pg", Group: "prod", Remote: "10.0.1.5:5432", ViaHost: "entryA"},
			{Name: "rd", Group: "prod", Remote: "h:6379", ViaHost: "bastionB"},
		},
		query: "entryA",
	}
	got := d.filtered()
	if len(got) != 1 || got[0].Name != "pg" {
		t.Fatalf("filter by via_host: got %+v, want only pg", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestFiltered_MatchesViaHost -v`
Expected: FAIL (the filter doesn't check `ViaHost`).

- [ ] **Step 3: Implement**

In `internal/guiapp/window.go`, update `filtered` to also match `ViaHost`:

```go
		if strings.Contains(t.Name, q) || strings.Contains(t.Group, q) ||
			strings.Contains(t.Remote, q) || strings.Contains(t.Via, q) ||
			strings.Contains(t.ViaHost, q) {
			out = append(out, t)
		}
```

- [ ] **Step 4: Run the whole suite + vet + build**

Run: `go test ./...`
Expected: PASS across all packages. (Per project memory, two tests on this Mac may flake for environment reasons — socket-path length and a timing-sensitive test — unrelated to this change; confirm any failure matches those known cases before treating it as a regression.)

Run: `go vet ./... && go build ./...`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/window.go internal/guiapp/window_test.go
git commit -m "feat(guiapp): search filter matches via_host; full-suite verification

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review (author checklist — already applied)

**Spec coverage (Plan 3 scope = §3 data model surface, §6 migration, §7.2 tunnel dialog):**

- §7.2 "via_host picker: dropdown of saved hosts, plus + 新建主机" → Task 9 (picker + `addNewHost` + `onNewHost`).
- §7.2 "Forward target remote (host:port), local port, autostart, advanced ssh_options" → existing fields retained; `value()`/`Parse` carry them (Tasks 1, 9). Autostart/sshOptions widgets unchanged.
- §7.2 "Editing a legacy tunnel: show its legacy fields read-mostly with the 迁移为主机 action" → Task 10.
- §7.2 "测试连接 button validates the full chain (via the chosen host)" → Task 11 (reuses Plan-2 `gui.TestConnection`).
- §6 "via: <alias> → reuse ssh_config import parser → new hosts entry → rewrite to via_host" → Tasks 4 (uses `sshconf.ParseSSHConfig`).
- §6 "inline jump → extract chain + endpoint into host entries → rewrite to via_host" → Task 5 (documented naming scheme + collision handling via `uniqueHostName`).
- §6 "dismissible one-time hint banner points users with legacy tunnels at the migrate action" → Task 12 (Preferences-persisted).
- §6 "ProxyJump mapped to jump references where the referenced alias is also imported" → Task 4 (`migrateVia` only sets `jump` when the ProxyJump alias is present; drops it otherwise, tested).
- §3 "tunnels reference an entry host via via_host" — form/route/parse model → Tasks 1, 2, 3.
- Display: `ipc.TunnelStatus.ViaHost` + daemon population + diagram + search → Tasks 6, 7, 8, 13. Legacy `Via` rendering unchanged (Task 8 keeps the `中继` branch; Task 13 keeps the `Via` filter term).

**Back-compat / existing tests stay green (called out explicitly):**
- `internal/gui/form_test.go`: legacy `ViaWins`, `JumpFields`, round-trips untouched — a legacy tunnel has `ViaHost == ""`, so the new Parse branch is inert (Task 1 Step 4 re-runs the whole package).
- `internal/gui/formcheck_test.go`: `Check`/`CheckRoute` legacy routes unchanged; only a new `case RouteViaHost` added (Task 3).
- `internal/guiapp/editdialog_test.go`: every `newEditForm(...)` caller is updated in the same task that changes the signature — Task 9 (→ 3 args) then Task 10 (→ 4 args). Task 10 Step 3d includes a grep to confirm arity. Task 11 changes only `showEditDialog` (not `newEditForm`) arity and updates window.go callers.
- `internal/guiapp/window_test.go`: new tests appended; `filtered`/`hasLegacyTunnels` are additive.

**Type consistency (names used identically across tasks):**
`TunnelForm.ViaHost`, `gui.RouteViaHost`, `errs["viaHost"]`, `gui.MigrateLegacyTunnel(cfg, name, readSSHConfig)`, `uniqueHostName`/`slug`/`splitHostPort` (gui), reuse of existing `splitJumpUser`, `ipc.TunnelStatus.ViaHost` (json `via_host`), `editForm.viaHostSel/.viaHostCard/.legacy/.migrateBtn/.onMigrate/.closeDialog/.testBtn/.testCfg/.testRunner/.testConn/.onTestResult/.runTest`, `newEditForm(f, hostNames, onNewHost, onMigrate)`, `showEditDialog(win, title, initial, hostNames, onNewHost, onMigrate, loadCfg, onSubmit)`, `(d *dashboard).hostNames()/.onNewHost/.openEditDialog/.legacyHintBanner`, `hasLegacyTunnels`, `legacyHintDismissedKey`, `routeCard.disable()`, `currentWindow()`, `readUserSSHConfig`.

**Plan-2 reuse boundary (NOT implemented here):** `gui.HostForm`, `gui.ToHostForm`, `gui.CheckHost`, `gui.TestConnection`, `gui.TestConnResult`, `gui.CmdRunner`, `gui.HostKey`, `execRunner`, `showHostDialog`, `(d *dashboard).openHosts()` — referenced only by Tasks 9, 10, 11, which are gated behind the "DEPENDS ON PLAN 2" notes. If Plan-2 result field names differ (`TestConnResult.OK`/`.Reason`), Tasks 11 inline notes say where to adjust.

**Placeholder scan:** every code step shows complete code. Two seams call out a contract uncertainty (Plan-2 `TestConnResult` field names) with explicit "adjust here" guidance rather than leaving them blank. Task 4 ships a documented temporary `migrateJump` stub that Task 5 replaces (so each task compiles in isolation).
