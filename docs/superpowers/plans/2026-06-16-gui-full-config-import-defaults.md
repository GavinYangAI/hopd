# Full GUI Config — Plan 4: Import wizard (~/.ssh/config) + Global defaults editor

> **⚠️ CROSS-PLAN INTEGRATION NOTES (read before executing — this plan runs LAST, after Plans 2 & 3; added after a 3-plan consistency review):**
> 1. **Do NOT define `readUserSSHConfig` or `errMissingSSHConfig`.** Plan 3 already defines both in `internal/guiapp/window.go`. REUSE them (same package `guiapp`). Redefining = duplicate-declaration compile error. Your import dialog calls the existing `readUserSSHConfig()` and handles `errMissingSSHConfig` for the friendly empty state. If for some reason Plan 3 hasn't landed them, add them to window.go yourself — but check first.
> 2. **TOOLBAR — integrate, do NOT overwrite.** By the time this plan runs, `window.go`'s `globalZone` already contains Plan 2's `hosts` button (and `buildToolbar` declares it). Your final line MUST preserve it: `globalZone := container.NewHBox(hosts, reload, settings, importBtn, add)` — read the CURRENT line and ADD `settings`/`importBtn`, never replace the whole `NewHBox(...)` with one that drops `hosts`. Same for the button declarations in `buildToolbar`.
> 3. **TRAY/Handlers/app.go — additive insertions only.** `Handlers` already has `Hosts func()` (Plan 2); ADD `Settings func()` — don't re-paste the original struct. `buildMenu`'s connected `items` already has the `主机…` item; INSERT `设置…` without dropping it. `app.go`'s `handlers()` literal already sets `Hosts:`; ADD `Settings:` alongside it.
> 4. **Helper-name reservations:** Plan 2 owns `menuLabels`/`isReloadWarning` (tray/window test helpers). Reuse, don't redefine. Use the inline `errors.Is(err, gui.ErrReloadAfterSave)` form like the existing `deleteTunnel` if you need it.
> 5. **Adapt to current file state, don't blind-paste** any quoted shared-file baseline.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users (1) seed hopd's `hosts:` section by importing aliases from their `~/.ssh/config` through a GUI wizard, and (2) edit global `defaults.restart` and `defaults.ssh_options` through a GUI settings dialog — so neither `~/.ssh/config` nor `config.yaml`'s defaults block ever needs hand-editing.

**Architecture:** Two new pure, Fyne-free helpers in `internal/gui` carry all the logic and validation: `importmodel.go` maps `sshconf.ImportedHost` values into `config.Host` entries (resolving `ProxyJump` to `jump` only when the referenced alias is also selected), and `defaultsform.go` is a string-typed form over `config.Restart` + `defaults.ssh_options` with live validation. A small backend addition exposes `defaults.ssh_options` (stored in the unexported `Config.defaultOpts`) via `DefaultOptions()`/`SetDefaultOptions()` so the defaults form can round-trip it. Two new Fyne dialogs in `internal/guiapp` (`importdialog.go`, `settingsdialog.go`) drive those helpers and persist through the existing `gui.ConfigStore.Save` flow. The import dialog reads ssh_config through an injectable byte-reader so tests use a fixture, never the real `~/.ssh/config`.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `fyne.io/fyne/v2` (+ `fyne.io/fyne/v2/test` for headless GUI tests), standard `testing`. The backend pieces this plan depends on — `sshconf.ParseSSHConfig`/`ImportedHost`, `config.Host`, `config.AddHost`, `config.Config.Restart`, `gui.ConfigStore`, `paths.ConfigFile` — already exist (landed in Plan 1; the host mutators in `internal/config/marshal.go`).

**Scope note:** This is Plan 4 of the multi-plan feature (spec: `docs/superpowers/specs/2026-06-16-gui-full-config-design.md`). It implements **§7.6 (Import wizard)** and **§7.3 (Settings/defaults editor)**, plus the small backend getter/setter §3/§8 needs to support the defaults editor, and honours **§5** rendering rules (ProxyJump→jump mapping) and **§10** security (read-only `~/.ssh/config`). Plans 2 (hosts manager `(d *dashboard).openHosts()`, `gui.HostForm`, `showHostDialog`) and 3 (tunnel-form rewrite, test-connection, host-key trust) are assumed to exist where referenced — this plan does **not** re-plan them. The import dialog and settings dialog are wired to be reachable independently (toolbar button and/or tray item), so this plan stands alone even if Plan 2's hosts manager is not yet present.

**Conventions:** Run tests with `go test ./...` from the repo root. End every commit message body with the trailer:
`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
Work on the existing `feature/gui-full-config` branch.

---

## File Structure

- `internal/config/config.go` — modify: add `DefaultOptions()` getter and `SetDefaultOptions(m)` setter for the unexported `defaultOpts` (the only field `Marshal` reads for `defaults.ssh_options`).
- `internal/config/config_test.go` — modify: round-trip test for the getter/setter via `Marshal`/`Parse`.
- `internal/gui/importmodel.go` — create: `BuildHostsFromImport(imported, selectedNames) (map[string]config.Host, error)` + `ExistingImportNames(cfg, imported) map[string]bool` diff helper.
- `internal/gui/importmodel_test.go` — create: table tests (mapping, ProxyJump in/out of selection, collisions, dedup helper).
- `internal/gui/defaultsform.go` — create: `DefaultsForm` struct, `ToDefaultsForm(cfg)`, `(f DefaultsForm) Apply(cfg) error`, `CheckDefaults(f) FieldErrors`.
- `internal/gui/defaultsform_test.go` — create: To/Apply round-trip + Check validation tables.
- `internal/guiapp/importdialog.go` — create: Fyne import wizard (injectable ssh_config reader, per-row `widget.Check`, "导入所选" applies `BuildHostsFromImport` + `cfg.AddHost` + `store.Save`).
- `internal/guiapp/importdialog_test.go` — create: headless `test.NewApp()` tests (fixture parse→checkbox rows; selection→AddHost via temp `ConfigStore`).
- `internal/guiapp/settingsdialog.go` — create: Fyne defaults editor (restart min/max + multiline ssh_options, live `CheckDefaults`, save via `DefaultsForm.Apply` + `store.Save`).
- `internal/guiapp/settingsdialog_test.go` — create: headless construct/prefill/validate tests.
- `internal/guiapp/tray.go` — modify: add a `Settings` handler field to `Handlers` and a "设置…" menu item.
- `internal/guiapp/app.go` — modify: wire `Settings` in `u.handlers()`.
- `internal/guiapp/window.go` — modify: add `setStore`-backed `openSettings()`/`openImport()` methods and toolbar buttons ("设置…", "从 ~/.ssh/config 导入…").
- `internal/guiapp/window_test.go` / `tray_test.go` — modify only if existing tests assert on `Handlers` fields or toolbar button counts (see Task 10).

---

## Task 1: Backend — `DefaultOptions()` getter + `SetDefaultOptions()` setter (round-trip)

`defaults.ssh_options` is stored in the unexported `Config.defaultOpts`, read directly by `Marshal` (`internal/config/marshal.go:54-55`). The defaults editor needs to read and replace it, so expose a copy-out getter and a copy-in setter. Pure backend, TDD.

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestDefaultOptionsGetSet(t *testing.T) {
	cfg, err := config.Parse([]byte(`
defaults:
  ssh_options: {ServerAliveInterval: "15"}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via: alias}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Getter returns a copy of the parsed defaults.
	got := cfg.DefaultOptions()
	if got["ServerAliveInterval"] != "15" {
		t.Fatalf("DefaultOptions = %v, want ServerAliveInterval=15", got)
	}
	// Mutating the returned map must not affect the config.
	got["ServerAliveInterval"] = "999"
	if again := cfg.DefaultOptions(); again["ServerAliveInterval"] != "15" {
		t.Fatalf("DefaultOptions returned a live reference, got mutated %v", again)
	}

	// Setter replaces the whole map and round-trips through Marshal/Parse.
	cfg.SetDefaultOptions(map[string]string{"Compression": "yes", "ConnectTimeout": "5"})
	out, err := config.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cfg2, err := config.Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	rt := cfg2.DefaultOptions()
	if rt["Compression"] != "yes" || rt["ConnectTimeout"] != "5" {
		t.Fatalf("round-trip defaults = %v, want Compression=yes ConnectTimeout=5", rt)
	}
	if _, gone := rt["ServerAliveInterval"]; gone {
		t.Fatalf("SetDefaultOptions should replace, not merge; got stale key in %v", rt)
	}
	// Setting empty/nil clears the section.
	cfg2.SetDefaultOptions(nil)
	if len(cfg2.DefaultOptions()) != 0 {
		t.Fatalf("SetDefaultOptions(nil) should clear, got %v", cfg2.DefaultOptions())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestDefaultOptionsGetSet -v`
Expected: FAIL (compile error — `cfg.DefaultOptions` / `cfg.SetDefaultOptions` undefined).

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add after the `Hosts()` method (around line 190):

```go
// DefaultOptions returns a copy of defaults.ssh_options. Mutating the returned
// map does not affect the config — use SetDefaultOptions to change it.
func (c *Config) DefaultOptions() map[string]string {
	out := make(map[string]string, len(c.defaultOpts))
	for k, v := range c.defaultOpts {
		out[k] = v
	}
	return out
}

// SetDefaultOptions replaces defaults.ssh_options with a copy of m (nil/empty
// clears the section). It stores into the same unexported field Marshal reads,
// so the change round-trips through Marshal/Parse.
func (c *Config) SetDefaultOptions(m map[string]string) {
	if len(m) == 0 {
		c.defaultOpts = map[string]string{}
		return
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	c.defaultOpts = cp
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestDefaultOptionsGetSet -v`
Then the whole package: `go test ./internal/config/`
Expected: PASS (existing tests still green).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): DefaultOptions getter/setter for defaults.ssh_options

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: `BuildHostsFromImport` — map selected ImportedHosts to config.Hosts

Pure logic in `internal/gui`. Maps each selected `sshconf.ImportedHost` to a `config.Host` (`HostName`→`Host`, `Port` default 22, `User`, `IdentityFile`→`Key`). `ProxyJump` becomes `Host.Jump` **only** when the referenced alias is also in `selectedNames`; otherwise the jump is dropped (the function does not error on a dropped jump — the dialog surfaces a warning separately). The function is pure over its inputs; collision handling against existing hosts happens at the call site (Task 7).

**Files:**
- Create: `internal/gui/importmodel.go`
- Test: `internal/gui/importmodel_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/gui/importmodel_test.go`:

```go
package gui_test

import (
	"reflect"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/gui"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)

func sampleImported() []sshconf.ImportedHost {
	return []sshconf.ImportedHost{
		{Name: "entryA", HostName: "198.51.100.7", Port: 65522, User: "userA", IdentityFile: "~/.ssh/idA", ProxyJump: "bastionB"},
		{Name: "bastionB", HostName: "203.0.113.9", User: "userB"},
		{Name: "lonely", HostName: "192.0.2.1", ProxyJump: "missing"},
	}
}

func TestBuildHostsFromImport_MapsFieldsAndDefaults(t *testing.T) {
	got, err := gui.BuildHostsFromImport(sampleImported(), []string{"bastionB"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := map[string]config.Host{
		"bastionB": {Host: "203.0.113.9", Port: 22, User: "userB"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestBuildHostsFromImport_ProxyJumpKeptWhenAlsoSelected(t *testing.T) {
	got, err := gui.BuildHostsFromImport(sampleImported(), []string{"entryA", "bastionB"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	a := got["entryA"]
	if a.Host != "198.51.100.7" || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" {
		t.Fatalf("entryA fields wrong: %+v", a)
	}
	if a.Jump != "bastionB" {
		t.Fatalf("entryA.Jump = %q, want bastionB (referenced alias is selected)", a.Jump)
	}
}

func TestBuildHostsFromImport_ProxyJumpDroppedWhenNotSelected(t *testing.T) {
	// entryA selected but bastionB is NOT -> jump dropped, no error.
	got, err := gui.BuildHostsFromImport(sampleImported(), []string{"entryA"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got["entryA"].Jump != "" {
		t.Fatalf("entryA.Jump = %q, want empty (referenced alias not selected)", got["entryA"].Jump)
	}
}

func TestBuildHostsFromImport_ProxyJumpDroppedWhenAliasUnknown(t *testing.T) {
	// "lonely" references "missing" which isn't in the import set at all.
	got, err := gui.BuildHostsFromImport(sampleImported(), []string{"lonely"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got["lonely"].Jump != "" {
		t.Fatalf("lonely.Jump = %q, want empty (unknown alias)", got["lonely"].Jump)
	}
}

func TestBuildHostsFromImport_SelectedNameNotInImportIsIgnored(t *testing.T) {
	got, err := gui.BuildHostsFromImport(sampleImported(), []string{"entryA", "ghost"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, ok := got["ghost"]; ok {
		t.Fatalf("ghost should not appear; got %+v", got)
	}
	if len(got) != 1 {
		t.Fatalf("want exactly entryA, got %+v", got)
	}
}

func TestExistingImportNames(t *testing.T) {
	cfg, err := config.Parse([]byte(`
hosts:
  bastionB: {host: 203.0.113.9, user: userB}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: bastionB}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	dup := gui.ExistingImportNames(cfg, sampleImported())
	if !dup["bastionB"] {
		t.Fatalf("bastionB should be flagged as existing")
	}
	if dup["entryA"] {
		t.Fatalf("entryA should not be flagged (not in config)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run 'BuildHostsFromImport|ExistingImportNames' -v`
Expected: FAIL (compile error — `gui.BuildHostsFromImport` / `gui.ExistingImportNames` undefined).

- [ ] **Step 3: Implement**

Create `internal/gui/importmodel.go`:

```go
package gui

import (
	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)

// BuildHostsFromImport maps the selected ssh_config aliases into config.Host
// entries. Selection is by name: only ImportedHosts whose Name is in
// selectedNames are mapped; names in selectedNames with no matching ImportedHost
// are ignored. A host's ProxyJump becomes Host.Jump only when the referenced
// alias is itself selected; otherwise the jump is dropped (the dialog warns
// about dropped jumps separately). The function is pure over its inputs — it
// does not touch any config and does not check collisions with existing hosts.
func BuildHostsFromImport(imported []sshconf.ImportedHost, selectedNames []string) (map[string]config.Host, error) {
	selected := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		selected[n] = true
	}

	out := map[string]config.Host{}
	for _, ih := range imported {
		if !selected[ih.Name] {
			continue
		}
		port := ih.Port
		if port == 0 {
			port = 22
		}
		h := config.Host{
			Host: ih.HostName,
			Port: port,
			User: ih.User,
			Key:  ih.IdentityFile,
		}
		// Keep the jump only if the target alias is also being imported.
		if ih.ProxyJump != "" && selected[ih.ProxyJump] {
			h.Jump = ih.ProxyJump
		}
		out[ih.Name] = h
	}
	return out, nil
}

// ExistingImportNames reports which imported aliases already exist as hosts in
// cfg, so the wizard can pre-mark/skip duplicates.
func ExistingImportNames(cfg *config.Config, imported []sshconf.ImportedHost) map[string]bool {
	dup := map[string]bool{}
	for _, ih := range imported {
		if _, ok := cfg.Host(ih.Name); ok {
			dup[ih.Name] = true
		}
	}
	return dup
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run 'BuildHostsFromImport|ExistingImportNames' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/importmodel.go internal/gui/importmodel_test.go
git commit -m "feat(gui): BuildHostsFromImport + ExistingImportNames import model

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: `DroppedJumps` helper — surface jumps lost because the target wasn't selected

A small pure helper the import dialog uses to build a friendly warning. Kept separate from `BuildHostsFromImport` (which is intentionally silent about drops) so both stay easy to test.

**Files:**
- Modify: `internal/gui/importmodel.go`
- Test: `internal/gui/importmodel_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/gui/importmodel_test.go`:

```go
func TestDroppedJumps(t *testing.T) {
	// entryA -> bastionB (not selected); lonely -> missing (not in import).
	dropped := gui.DroppedJumps(sampleImported(), []string{"entryA", "lonely"})
	want := map[string]string{
		"entryA": "bastionB",
		"lonely": "missing",
	}
	if !reflect.DeepEqual(dropped, want) {
		t.Fatalf("got %+v\nwant %+v", dropped, want)
	}

	// When the jump target is also selected, nothing is dropped.
	none := gui.DroppedJumps(sampleImported(), []string{"entryA", "bastionB"})
	if len(none) != 0 {
		t.Fatalf("want no dropped jumps, got %+v", none)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run TestDroppedJumps -v`
Expected: FAIL (`gui.DroppedJumps` undefined).

- [ ] **Step 3: Implement**

Add to `internal/gui/importmodel.go`:

```go
// DroppedJumps reports, for each selected ImportedHost that had a ProxyJump,
// the jump target that will be dropped because it is not itself selected. The
// returned map is alias -> dropped jump target. Useful for a wizard warning.
func DroppedJumps(imported []sshconf.ImportedHost, selectedNames []string) map[string]string {
	selected := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		selected[n] = true
	}
	dropped := map[string]string{}
	for _, ih := range imported {
		if !selected[ih.Name] || ih.ProxyJump == "" {
			continue
		}
		if !selected[ih.ProxyJump] {
			dropped[ih.Name] = ih.ProxyJump
		}
	}
	return dropped
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run TestDroppedJumps -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/importmodel.go internal/gui/importmodel_test.go
git commit -m "feat(gui): DroppedJumps helper for import-wizard warnings

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `DefaultsForm` + `ToDefaultsForm` — read defaults into an editable form

Pure logic in `internal/gui`. `DefaultsForm` is a string-typed view of `config.Restart` (durations as strings) plus `defaults.ssh_options` as multiline `key=value` text. This task adds the type and the `ToDefaultsForm` reader; `Apply` and `CheckDefaults` follow in Tasks 5 and 6.

**Files:**
- Create: `internal/gui/defaultsform.go`
- Test: `internal/gui/defaultsform_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/gui/defaultsform_test.go`:

```go
package gui_test

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestToDefaultsForm(t *testing.T) {
	cfg, err := config.Parse([]byte(`
defaults:
  restart: {min: 2s, max: 60s}
  ssh_options: {ServerAliveInterval: "15", Compression: "yes"}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via: alias}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := gui.ToDefaultsForm(cfg)
	if f.RestartMin != "2s" || f.RestartMax != "1m0s" {
		t.Fatalf("durations wrong: min=%q max=%q", f.RestartMin, f.RestartMax)
	}
	// ssh_options rendered as sorted multiline key=value.
	if f.SSHOptions != "Compression=yes\nServerAliveInterval=15" {
		t.Fatalf("ssh_options = %q", f.SSHOptions)
	}
}
```

> Note on `1m0s`: `time.Duration.String()` renders 60s as `1m0s`. `time.ParseDuration` accepts that form on the way back (verified by Task 5's round-trip test).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run TestToDefaultsForm -v`
Expected: FAIL (`gui.ToDefaultsForm` / `gui.DefaultsForm` undefined).

- [ ] **Step 3: Implement**

Create `internal/gui/defaultsform.go`:

```go
package gui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GavinYangAI/hopd/internal/config"
)

// DefaultsForm is the editable, string-typed view of the global defaults block
// (defaults.restart + defaults.ssh_options) shown in the settings dialog.
type DefaultsForm struct {
	RestartMin string // duration string, e.g. "2s"
	RestartMax string // duration string, e.g. "1m0s"
	SSHOptions string // multiline key=value
}

// ToDefaultsForm reads a config's defaults into an editable form.
func ToDefaultsForm(cfg *config.Config) DefaultsForm {
	f := DefaultsForm{
		RestartMin: cfg.Restart.Min.String(),
		RestartMax: cfg.Restart.Max.String(),
	}
	opts := cfg.DefaultOptions()
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k + "=" + opts[k])
	}
	f.SSHOptions = b.String()
	return f
}

// parseOptionLines parses multiline "key=value" text into a map, rejecting the
// "-p" port option (which belongs on a host, not in global ssh_options) and
// lines missing an '='.
func parseOptionLines(text string) (map[string]string, error) {
	opts := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("无效的 ssh 选项 %q（要写成 key=value）", line)
		}
		key := strings.TrimSpace(k)
		if key == "-p" || strings.HasPrefix(key, "-p") {
			return nil, fmt.Errorf("不要在这里写 -p；端口请在主机里设置")
		}
		opts[key] = strings.TrimSpace(v)
	}
	return opts, nil
}

var _ = time.Second // keep time imported for Apply (Task 5)
```

> The `var _ = time.Second` line is a temporary keep-alive so the package compiles before Task 5 adds `Apply` (which uses `time.ParseDuration`). Task 5 removes it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run TestToDefaultsForm -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/defaultsform.go internal/gui/defaultsform_test.go
git commit -m "feat(gui): DefaultsForm + ToDefaultsForm reader

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `DefaultsForm.Apply` — write the form back into a config

Parses the duration strings and the multiline ssh_options, then sets `cfg.Restart` and `cfg.SetDefaultOptions(...)`. Friendly Chinese errors; rejects `-p` (via the shared `parseOptionLines`).

**Files:**
- Modify: `internal/gui/defaultsform.go`
- Test: `internal/gui/defaultsform_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/gui/defaultsform_test.go`:

```go
func TestDefaultsForm_ApplyRoundTrip(t *testing.T) {
	cfg, err := config.Parse([]byte(`groups: {}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := gui.DefaultsForm{
		RestartMin: "3s",
		RestartMax: "1m0s",
		SSHOptions: "ServerAliveInterval=30\nCompression=yes",
	}
	if err := f.Apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if cfg.Restart.Min != 3*time.Second {
		t.Fatalf("restart.min = %v, want 3s", cfg.Restart.Min)
	}
	if cfg.Restart.Max != time.Minute {
		t.Fatalf("restart.max = %v, want 1m", cfg.Restart.Max)
	}
	opts := cfg.DefaultOptions()
	if opts["ServerAliveInterval"] != "30" || opts["Compression"] != "yes" {
		t.Fatalf("ssh_options = %v", opts)
	}
	// The applied config must validate and round-trip via Marshal/Parse.
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate after apply: %v", err)
	}
}

func TestDefaultsForm_ApplyRejectsBadDuration(t *testing.T) {
	cfg, _ := config.Parse([]byte(`groups: {}`))
	f := gui.DefaultsForm{RestartMin: "soon", RestartMax: "1m0s"}
	if err := f.Apply(cfg); err == nil {
		t.Fatal("Apply should reject an unparseable duration")
	}
}

func TestDefaultsForm_ApplyRejectsDashP(t *testing.T) {
	cfg, _ := config.Parse([]byte(`groups: {}`))
	f := gui.DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: "-p=2222"}
	if err := f.Apply(cfg); err == nil {
		t.Fatal("Apply should reject -p in ssh_options")
	}
}

func TestDefaultsForm_ApplyClearsOptionsWhenEmpty(t *testing.T) {
	cfg, _ := config.Parse([]byte(`
defaults:
  ssh_options: {ServerAliveInterval: "15"}
groups: {}
`))
	f := gui.DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: ""}
	if err := f.Apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(cfg.DefaultOptions()) != 0 {
		t.Fatalf("empty ssh_options text should clear defaults, got %v", cfg.DefaultOptions())
	}
}
```

Ensure `internal/gui/defaultsform_test.go` imports `"time"` (add it to the import block).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run 'DefaultsForm_Apply' -v`
Expected: FAIL (`Apply` undefined).

- [ ] **Step 3: Implement**

In `internal/gui/defaultsform.go`, remove the temporary keep-alive line:

```go
var _ = time.Second // keep time imported for Apply (Task 5)
```

and add the `Apply` method:

```go
// Apply parses the form and writes it into cfg: restart bounds and
// defaults.ssh_options. It returns a friendly error on bad input and does not
// mutate cfg on failure (durations are parsed before anything is written).
func (f DefaultsForm) Apply(cfg *config.Config) error {
	min, err := time.ParseDuration(strings.TrimSpace(f.RestartMin))
	if err != nil {
		return fmt.Errorf("重连最短间隔不是有效时长（如 2s、500ms）：%v", err)
	}
	max, err := time.ParseDuration(strings.TrimSpace(f.RestartMax))
	if err != nil {
		return fmt.Errorf("重连最长间隔不是有效时长（如 60s、2m）：%v", err)
	}
	opts, err := parseOptionLines(f.SSHOptions)
	if err != nil {
		return err
	}
	cfg.Restart.Min = min
	cfg.Restart.Max = max
	cfg.SetDefaultOptions(opts)
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run 'DefaultsForm' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/defaultsform.go internal/gui/defaultsform_test.go
git commit -m "feat(gui): DefaultsForm.Apply writes restart + ssh_options into config

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: `CheckDefaults` — live field validation for the settings dialog

Returns per-field messages keyed `restartMin`, `restartMax`, `sshOptions` for live validation on every keystroke (mirrors `gui.Check` in `formcheck.go`). Rules: both durations parse and are > 0; max ≥ min; ssh_options lines are valid `key=value` and contain no `-p`.

**Files:**
- Modify: `internal/gui/defaultsform.go`
- Test: `internal/gui/defaultsform_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/gui/defaultsform_test.go`:

```go
func TestCheckDefaults(t *testing.T) {
	cases := []struct {
		name     string
		form     gui.DefaultsForm
		wantKeys []string // field keys expected to carry an error ("" entries impossible)
	}{
		{
			name: "all valid",
			form: gui.DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: "Compression=yes"},
		},
		{
			name:     "empty min",
			form:     gui.DefaultsForm{RestartMin: "", RestartMax: "60s"},
			wantKeys: []string{"restartMin"},
		},
		{
			name:     "bad max duration",
			form:     gui.DefaultsForm{RestartMin: "2s", RestartMax: "later"},
			wantKeys: []string{"restartMax"},
		},
		{
			name:     "max less than min",
			form:     gui.DefaultsForm{RestartMin: "60s", RestartMax: "2s"},
			wantKeys: []string{"restartMax"},
		},
		{
			name:     "zero min",
			form:     gui.DefaultsForm{RestartMin: "0s", RestartMax: "60s"},
			wantKeys: []string{"restartMin"},
		},
		{
			name:     "bad ssh option line",
			form:     gui.DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: "noequals"},
			wantKeys: []string{"sshOptions"},
		},
		{
			name:     "dash p in ssh options",
			form:     gui.DefaultsForm{RestartMin: "2s", RestartMax: "60s", SSHOptions: "-p=2222"},
			wantKeys: []string{"sshOptions"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := gui.CheckDefaults(tc.form)
			if len(tc.wantKeys) == 0 {
				if len(errs) != 0 {
					t.Fatalf("want no errors, got %v", errs)
				}
				return
			}
			for _, k := range tc.wantKeys {
				if errs[k] == "" {
					t.Fatalf("want error on %q, got %v", k, errs)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run TestCheckDefaults -v`
Expected: FAIL (`gui.CheckDefaults` undefined).

- [ ] **Step 3: Implement**

Add to `internal/gui/defaultsform.go`:

```go
// CheckDefaults validates the defaults form field-by-field and returns per-field
// messages (keys: restartMin, restartMax, sshOptions). It is pure so the dialog
// can call it live on every keystroke. An absent key means that field is valid.
func CheckDefaults(f DefaultsForm) FieldErrors {
	errs := FieldErrors{}

	min, minErr := time.ParseDuration(strings.TrimSpace(f.RestartMin))
	switch {
	case strings.TrimSpace(f.RestartMin) == "":
		errs["restartMin"] = "填重连最短间隔（如 2s）"
	case minErr != nil:
		errs["restartMin"] = "不是有效时长（如 2s、500ms）"
	case min <= 0:
		errs["restartMin"] = "要大于 0"
	}

	max, maxErr := time.ParseDuration(strings.TrimSpace(f.RestartMax))
	switch {
	case strings.TrimSpace(f.RestartMax) == "":
		errs["restartMax"] = "填重连最长间隔（如 60s）"
	case maxErr != nil:
		errs["restartMax"] = "不是有效时长（如 60s、2m）"
	case minErr == nil && max < min:
		errs["restartMax"] = "最长间隔要 ≥ 最短间隔"
	}

	if _, err := parseOptionLines(f.SSHOptions); err != nil {
		errs["sshOptions"] = err.Error()
	}
	return errs
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run TestCheckDefaults -v`
Then the whole package: `go test ./internal/gui/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/defaultsform.go internal/gui/defaultsform_test.go
git commit -m "feat(gui): CheckDefaults live validation for the settings form

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Fyne import dialog — parse a fixture ssh_config and render checkbox rows

`internal/guiapp/importdialog.go`. An `importForm` struct holds the parsed `ImportedHost` rows and a `widget.Check` per row. The ssh_config bytes come from an **injectable reader** (`readSSHConfig func() ([]byte, error)`) so tests pass a fixture and production reads `~/.ssh/config` (read-only, §10). Rows whose name already exists as a host are pre-disabled (skip duplicates). This task builds construction + row rendering + the "select" read-back; the apply/save path is Task 8.

**Files:**
- Create: `internal/guiapp/importdialog.go`
- Test: `internal/guiapp/importdialog_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/guiapp/importdialog_test.go`:

```go
package guiapp

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/config"
)

const fixtureSSHConfig = `
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
`

func TestImportForm_RendersRowsFromFixture(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))

	f, err := newImportForm(cfg, func() ([]byte, error) { return []byte(fixtureSSHConfig), nil })
	if err != nil {
		t.Fatalf("newImportForm: %v", err)
	}
	// Wildcard "Host *" is skipped by the parser; two named rows remain.
	if len(f.rows) != 2 {
		t.Fatalf("got %d rows, want 2 (wildcard skipped)", len(f.rows))
	}
	if _, ok := f.rows["entryA"]; !ok {
		t.Fatalf("entryA row missing: %v", f.rows)
	}
}

func TestImportForm_PreDisablesExistingHosts(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`
hosts:
  bastionB: {host: 203.0.113.9, user: userB}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: bastionB}
`))
	f, err := newImportForm(cfg, func() ([]byte, error) { return []byte(fixtureSSHConfig), nil })
	if err != nil {
		t.Fatalf("newImportForm: %v", err)
	}
	if !f.rows["bastionB"].check.Disabled() {
		t.Fatal("bastionB already exists; its checkbox should be disabled")
	}
	if f.rows["entryA"].check.Disabled() {
		t.Fatal("entryA does not exist; its checkbox should be enabled")
	}
}

func TestImportForm_SelectedNames(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))
	f, _ := newImportForm(cfg, func() ([]byte, error) { return []byte(fixtureSSHConfig), nil })

	f.rows["entryA"].check.SetChecked(true)
	got := f.selectedNames()
	if len(got) != 1 || got[0] != "entryA" {
		t.Fatalf("selectedNames = %v, want [entryA]", got)
	}
}

func TestImportForm_EmptyWhenNoFile(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))
	// Missing file: reader returns os.ErrNotExist-style; the form should
	// construct with zero rows, not error.
	f, err := newImportForm(cfg, func() ([]byte, error) { return nil, errMissingSSHConfig })
	if err != nil {
		t.Fatalf("missing file should be a friendly empty state, got err: %v", err)
	}
	if len(f.rows) != 0 {
		t.Fatalf("want 0 rows for missing file, got %d", len(f.rows))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestImportForm -v`
Expected: FAIL (compile error — `newImportForm`, `importForm`, `errMissingSSHConfig`, etc. undefined).

- [ ] **Step 3: Implement**

Create `internal/guiapp/importdialog.go`:

```go
package guiapp

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)

// errMissingSSHConfig is returned by the default reader when ~/.ssh/config does
// not exist, so the dialog can show a friendly empty state instead of an error.
var errMissingSSHConfig = errors.New("no ~/.ssh/config")

// importRow is one discovered ssh_config alias and its selection checkbox.
type importRow struct {
	host  sshconf.ImportedHost
	check *widget.Check
}

// importForm holds the parsed import rows. The ssh_config bytes are supplied by
// an injectable reader so tests use a fixture and never touch the real file.
type importForm struct {
	root fyne.CanvasObject
	rows map[string]*importRow
	cfg  *config.Config
}

// readUserSSHConfig is the production reader: it reads ~/.ssh/config (read-only,
// per SECURITY.md). A missing file maps to errMissingSSHConfig.
func readUserSSHConfig() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if os.IsNotExist(err) {
		return nil, errMissingSSHConfig
	}
	return data, err
}

// newImportForm parses the ssh_config bytes from read and builds a checkbox row
// per importable alias. Aliases that already exist as hosts in cfg are
// pre-disabled (skip duplicates). A missing file yields an empty form, not an
// error.
func newImportForm(cfg *config.Config, read func() ([]byte, error)) (*importForm, error) {
	f := &importForm{rows: map[string]*importRow{}, cfg: cfg}

	data, err := read()
	if errors.Is(err, errMissingSSHConfig) {
		f.build(nil)
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	imported, err := sshconf.ParseSSHConfig(data)
	if err != nil {
		return nil, err
	}
	existing := gui.ExistingImportNames(cfg, imported)

	for _, ih := range imported {
		chk := widget.NewCheck("", nil)
		if existing[ih.Name] {
			chk.SetChecked(false)
			chk.Disable()
		}
		f.rows[ih.Name] = &importRow{host: ih, check: chk}
	}
	f.build(imported)
	return f, nil
}

// orderedNames returns the row names in a stable (sorted) order for rendering.
func (f *importForm) orderedNames() []string {
	names := make([]string, 0, len(f.rows))
	for n := range f.rows {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// build lays out the rows. imported is the parse order (may be nil for the empty
// state); rendering uses the sorted names so the layout is deterministic.
func (f *importForm) build(imported []sshconf.ImportedHost) {
	if len(f.rows) == 0 {
		f.root = container.NewVBox(
			widget.NewLabel("没有在 ~/.ssh/config 里找到可导入的主机。"),
		)
		return
	}
	var objs []fyne.CanvasObject
	objs = append(objs, widget.NewLabel("勾选要导入为 hopd 主机的条目（灰色表示已存在）："))
	for _, name := range f.orderedNames() {
		r := f.rows[name]
		desc := name + "  —  " + describeImported(r.host)
		objs = append(objs, container.NewHBox(r.check, widget.NewLabel(desc)))
	}
	f.root = container.NewVScroll(container.NewVBox(objs...))
}

// describeImported renders a one-line summary of an imported host for the row.
func describeImported(h sshconf.ImportedHost) string {
	s := h.HostName
	if h.Port != 0 {
		s += ":" + itoa(h.Port)
	}
	if h.User != "" {
		s = h.User + "@" + s
	}
	if h.IdentityFile != "" {
		s += "  key=" + h.IdentityFile
	}
	if h.ProxyJump != "" {
		s += "  jump=" + h.ProxyJump
	}
	return s
}

// selectedNames returns the names of the checked (enabled) rows.
func (f *importForm) selectedNames() []string {
	var out []string
	for _, name := range f.orderedNames() {
		if f.rows[name].check.Checked {
			out = append(out, name)
		}
	}
	return out
}
```

Add the `gui` import to the import block (the file references `gui.ExistingImportNames`):

```go
	"github.com/GavinYangAI/hopd/internal/gui"
```

> `itoa` already exists in package `guiapp` (used in `editdialog.go`/`window.go`). Reuse it; do not redefine.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -run TestImportForm -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/importdialog.go internal/guiapp/importdialog_test.go
git commit -m "feat(guiapp): import wizard form (fixture-injectable ssh_config reader)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Import dialog apply path — selection → AddHost → Save

Adds the "导入所选" apply logic: build hosts via `gui.BuildHostsFromImport`, add each to a freshly-loaded config, and persist with `store.Save`. Tested against a real temp-file-backed `gui.ConfigStore` (no daemon; reload returns nil).

**Files:**
- Modify: `internal/guiapp/importdialog.go`
- Test: `internal/guiapp/importdialog_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/guiapp/importdialog_test.go` (imports `"path/filepath"`, `"github.com/GavinYangAI/hopd/internal/gui"`):

```go
func TestImportForm_ApplyAddsHostsAndSaves(t *testing.T) {
	_ = test.NewApp()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	store := gui.NewConfigStore(path, nil) // nil reload => Save won't fail on daemon

	cfg, _ := config.Parse([]byte(`groups: {}`))
	f, _ := newImportForm(cfg, func() ([]byte, error) { return []byte(fixtureSSHConfig), nil })

	// Select both so the entryA->bastionB jump is preserved.
	f.rows["entryA"].check.SetChecked(true)
	f.rows["bastionB"].check.SetChecked(true)

	if err := f.apply(store); err != nil {
		t.Fatalf("apply: %v", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	a, ok := reloaded.Host("entryA")
	if !ok || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" || a.Jump != "bastionB" {
		t.Fatalf("entryA not imported correctly: %+v (ok=%v)", a, ok)
	}
	b, ok := reloaded.Host("bastionB")
	if !ok || b.Host != "203.0.113.9" || b.Port != 22 {
		t.Fatalf("bastionB not imported correctly: %+v (ok=%v)", b, ok)
	}
}

func TestImportForm_ApplySkipsAlreadyExisting(t *testing.T) {
	_ = test.NewApp()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// Pre-seed a config that already has bastionB.
	seed := gui.NewConfigStore(path, nil)
	base, _ := config.Parse([]byte(`
hosts:
  bastionB: {host: 203.0.113.9, user: userB}
groups: {}
`))
	if err := seed.Save(base); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	cfg, _ := seed.Load()
	f, _ := newImportForm(cfg, func() ([]byte, error) { return []byte(fixtureSSHConfig), nil })
	// bastionB's checkbox is disabled (pre-existing); only entryA is selectable.
	f.rows["entryA"].check.SetChecked(true)

	if err := f.apply(seed); err != nil {
		t.Fatalf("apply: %v", err)
	}
	reloaded, _ := seed.Load()
	if _, ok := reloaded.Host("entryA"); !ok {
		t.Fatal("entryA should have been imported")
	}
	// entryA's jump to bastionB is dropped (bastionB wasn't in this selection).
	if reloaded.Host2Jump(t, "entryA") != "" {
		t.Fatalf("entryA.Jump should be empty (bastionB not in selection)")
	}
}
```

> The test above references a `Host2Jump` test helper that does not exist; replace that last assertion with a direct field read instead. Use this exact final block for `TestImportForm_ApplySkipsAlreadyExisting`'s tail:

```go
	reloaded, _ := seed.Load()
	a, ok := reloaded.Host("entryA")
	if !ok {
		t.Fatal("entryA should have been imported")
	}
	if a.Jump != "" {
		t.Fatalf("entryA.Jump = %q, want empty (bastionB not in this selection)", a.Jump)
	}
```

(Use the corrected block; do not add a `Host2Jump` helper.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run 'TestImportForm_Apply' -v`
Expected: FAIL (`f.apply` undefined).

- [ ] **Step 3: Implement**

Add to `internal/guiapp/importdialog.go`:

```go
// apply builds config.Hosts from the checked rows, adds any that don't already
// exist, and saves through the store. Existing hosts are skipped (their rows are
// disabled, but apply also guards in case state changed). It returns the store's
// error (including gui.ErrReloadAfterSave, which the dialog treats as soft).
func (f *importForm) apply(store *gui.ConfigStore) error {
	selected := f.selectedNames()
	if len(selected) == 0 {
		return nil
	}
	hosts, err := gui.BuildHostsFromImport(importedSlice(f), selected)
	if err != nil {
		return err
	}
	cfg, err := store.Load()
	if err != nil {
		return err
	}
	for name, h := range hosts {
		if _, exists := cfg.Host(name); exists {
			continue // skip duplicates defensively
		}
		if err := cfg.AddHost(name, h); err != nil {
			return err
		}
	}
	return store.Save(cfg)
}

// importedSlice rebuilds the []sshconf.ImportedHost from the rows so apply can
// call the pure model with the same data the dialog rendered.
func importedSlice(f *importForm) []sshconf.ImportedHost {
	out := make([]sshconf.ImportedHost, 0, len(f.rows))
	for _, name := range f.orderedNames() {
		out = append(out, f.rows[name].host)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -run TestImportForm -v`
Expected: PASS.

- [ ] **Step 5: Add the dialog presenter + commit**

Add the modal presenter to `internal/guiapp/importdialog.go` (no separate test — it is exercised through Task 10's wiring and the form tests above):

```go
// showImportDialog presents the import wizard modally. On "导入所选" it applies
// the selection through the store and closes; a soft reload failure is shown as
// an information dialog, consistent with the tunnel dialog.
func showImportDialog(win fyne.Window, cfg *config.Config, store *gui.ConfigStore) {
	f, err := newImportForm(cfg, readUserSSHConfig)
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	dlg := dialog.NewCustomConfirm("从 ~/.ssh/config 导入", "导入所选", "取消", f.root, func(ok bool) {
		if !ok {
			return
		}
		if err := f.apply(store); err != nil {
			if errors.Is(err, gui.ErrReloadAfterSave) {
				dialog.ShowInformation("已导入", "主机已保存。daemon 未运行，将在它启动后生效。", win)
				return
			}
			dialog.ShowError(err, win)
			return
		}
	}, win)
	dlg.Resize(fyne.NewSize(560, 460))
	dlg.Show()
}
```

Add `"fyne.io/fyne/v2/dialog"` to the import block.

```bash
git add internal/guiapp/importdialog.go internal/guiapp/importdialog_test.go
git commit -m "feat(guiapp): import wizard apply path (AddHost + Save) and presenter

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Fyne settings dialog — defaults editor with live validation

`internal/guiapp/settingsdialog.go`. A `settingsForm` with three entries (restart min, restart max, multiline ssh_options), prefilled from `gui.ToDefaultsForm`, live-validated with `gui.CheckDefaults`, saved via `gui.DefaultsForm.Apply` + `store.Save`. Headless tests assert prefill and validity (derivable state), not pixels.

**Files:**
- Create: `internal/guiapp/settingsdialog.go`
- Test: `internal/guiapp/settingsdialog_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/guiapp/settingsdialog_test.go`:

```go
package guiapp

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/config"
)

func TestSettingsForm_PrefillsFromConfig(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`
defaults:
  restart: {min: 3s, max: 90s}
  ssh_options: {Compression: "yes"}
groups: {}
`))
	sf := newSettingsForm(cfg)
	if sf.min.Text != "3s" {
		t.Fatalf("min = %q, want 3s", sf.min.Text)
	}
	if sf.max.Text != "1m30s" {
		t.Fatalf("max = %q, want 1m30s", sf.max.Text)
	}
	if sf.sshOptions.Text != "Compression=yes" {
		t.Fatalf("ssh_options = %q", sf.sshOptions.Text)
	}
	if !sf.valid() {
		t.Fatal("freshly prefilled valid config should be valid")
	}
}

func TestSettingsForm_InvalidDisablesSave(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))
	sf := newSettingsForm(cfg)
	sf.min.SetText("nonsense")
	sf.refresh()
	if sf.valid() {
		t.Fatal("a bad duration should make the form invalid")
	}
}

func TestSettingsForm_Value(t *testing.T) {
	_ = test.NewApp()
	cfg, _ := config.Parse([]byte(`groups: {}`))
	sf := newSettingsForm(cfg)
	sf.min.SetText("5s")
	sf.max.SetText("120s")
	sf.sshOptions.SetText("ServerAliveInterval=30")
	v := sf.value()
	if v.RestartMin != "5s" || v.RestartMax != "120s" || v.SSHOptions != "ServerAliveInterval=30" {
		t.Fatalf("value = %+v", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestSettingsForm -v`
Expected: FAIL (`newSettingsForm` etc. undefined).

- [ ] **Step 3: Implement**

Create `internal/guiapp/settingsdialog.go`:

```go
package guiapp

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// settingsForm edits the global defaults (restart bounds + ssh_options).
type settingsForm struct {
	root fyne.CanvasObject

	min, max   *widget.Entry
	sshOptions *widget.Entry

	captions map[string]*captionLabel
	saveBtn  *widget.Button

	onSave   func()
	onCancel func()
}

// newSettingsForm builds the form prefilled from cfg's defaults.
func newSettingsForm(cfg *config.Config) *settingsForm {
	df := gui.ToDefaultsForm(cfg)
	sf := &settingsForm{
		min:        widget.NewEntry(),
		max:        widget.NewEntry(),
		sshOptions: widget.NewMultiLineEntry(),
		captions:   map[string]*captionLabel{},
	}
	sf.min.SetPlaceHolder("2s")
	sf.max.SetPlaceHolder("60s")
	sf.sshOptions.SetPlaceHolder("ServerAliveInterval=15\nCompression=yes")
	noWheelTrap(sf.min, sf.max)

	sf.min.SetText(df.RestartMin)
	sf.max.SetText(df.RestartMax)
	sf.sshOptions.SetText(df.SSHOptions)

	sf.build()
	sf.refresh()
	return sf
}

func (sf *settingsForm) field(label, help, key string, entry fyne.CanvasObject) fyne.CanvasObject {
	head := text(label, 12.5, pal.text1, fyne.TextStyle{Bold: true})
	cap := newCaption(help)
	sf.captions[key] = cap
	return container.New(layoutStackV{gap: 6}, head, entry, cap.obj)
}

func (sf *settingsForm) build() {
	header := container.New(layoutStackV{gap: 3},
		text("全局设置", 18, pal.text1, bold),
		text("这些默认值会应用到所有隧道", 13, pal.text2, fyne.TextStyle{}),
	)
	body := container.New(layoutStackV{gap: 14},
		container.NewGridWithColumns(2,
			sf.field("重连最短间隔", "断线后第一次重连等待，如 2s", "restartMin", sf.min),
			sf.field("重连最长间隔", "重连退避的上限，如 60s", "restartMax", sf.max),
		),
		sf.field("默认 ssh 选项", "多行 key=value，应用到所有隧道", "sshOptions", sf.sshOptions),
	)

	cancel := widget.NewButton("取消", func() { call(sf.onCancel) })
	sf.saveBtn = widget.NewButtonWithIcon("保存", theme.ConfirmIcon(), func() { call(sf.onSave) })
	sf.saveBtn.Importance = widget.HighImportance
	foot := container.NewBorder(nil, nil, nil, container.NewHBox(cancel, sf.saveBtn))

	sf.root = container.NewBorder(
		container.New(layoutPadXY{px: 20, py: 16}, header), nil, nil, nil,
		container.New(layoutPadXY{px: 20, py: 8}, container.NewVBox(body, foot)),
	)

	for _, e := range []*widget.Entry{sf.min, sf.max, sf.sshOptions} {
		e.OnChanged = func(string) { sf.refresh() }
	}
}

// value reads the entries back into a DefaultsForm.
func (sf *settingsForm) value() gui.DefaultsForm {
	return gui.DefaultsForm{
		RestartMin: sf.min.Text,
		RestartMax: sf.max.Text,
		SSHOptions: sf.sshOptions.Text,
	}
}

func (sf *settingsForm) valid() bool {
	return len(gui.CheckDefaults(sf.value())) == 0
}

// refresh re-runs validation, updates captions, and enables/disables save.
func (sf *settingsForm) refresh() {
	errs := gui.CheckDefaults(sf.value())
	for key, cap := range sf.captions {
		cap.set(errs[key], "")
	}
	if sf.saveBtn != nil {
		if len(errs) == 0 {
			sf.saveBtn.Enable()
		} else {
			sf.saveBtn.Disable()
		}
	}
}

// showSettingsDialog presents the defaults editor modally and saves on confirm.
func showSettingsDialog(win fyne.Window, store *gui.ConfigStore) {
	cfg, err := store.Load()
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	sf := newSettingsForm(cfg)
	dlg := dialog.NewCustomWithoutButtons("全局设置", sf.root, win)
	dlg.Resize(fyne.NewSize(560, 420))
	sf.onCancel = dlg.Hide
	sf.onSave = func() {
		if !sf.valid() {
			sf.refresh()
			return
		}
		c, err := store.Load()
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		if err := sf.value().Apply(c); err != nil {
			dialog.ShowError(err, win)
			return
		}
		if err := store.Save(c); err != nil {
			if errors.Is(err, gui.ErrReloadAfterSave) {
				dlg.Hide()
				dialog.ShowInformation("已保存", "设置已保存。daemon 未运行，将在它启动后生效。", win)
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

Add `"fyne.io/fyne/v2/theme"` to the import block (used by `theme.ConfirmIcon()`).

> `text`, `bold`, `pal`, `newCaption`, `captionLabel`, `noWheelTrap`, `layoutStackV`, `layoutPadXY`, `call` are all existing helpers in package `guiapp` (see `editdialog.go`). Reuse them; do not redefine.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -run TestSettingsForm -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/settingsdialog.go internal/guiapp/settingsdialog_test.go
git commit -m "feat(guiapp): settings dialog for global defaults (restart + ssh_options)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Entry-point wiring — toolbar buttons + tray "设置…" item

Make both dialogs reachable. Add `openSettings()`/`openImport()` methods to the dashboard (using the existing `d.store`/`d.win`), wire two toolbar buttons in `window.go`, add a `Settings` field to `Handlers`, a "设置…" tray item, and wire `Settings` in `app.go`.

**Files:**
- Modify: `internal/guiapp/window.go`
- Modify: `internal/guiapp/tray.go`
- Modify: `internal/guiapp/app.go`
- Test: `internal/guiapp/window_test.go` (add a small wiring test)

- [ ] **Step 1: Write the failing test**

Add to `internal/guiapp/window_test.go` (if the file does not exist, create it with `package guiapp` and imports `"testing"`, `"path/filepath"`, `"fyne.io/fyne/v2/test"`, `"github.com/GavinYangAI/hopd/internal/gui"`):

```go
func TestDashboard_OpenSettingsAndImportNoPanic(t *testing.T) {
	_ = test.NewApp()
	dir := t.TempDir()
	d := newDashboard(test.NewApp(), &DashboardActions{})
	d.setStore(gui.NewConfigStore(filepath.Join(dir, "config.yaml"), nil))

	// These open modal dialogs; under the headless driver they must construct
	// without panicking (no store == nil guard tripping, no missing window).
	d.openSettings()
	d.openImport()
}
```

> This is a smoke test: it proves the wiring constructs. The dialog *behavior* is covered by Tasks 7–9. `newImportForm` is called with the production `readUserSSHConfig` here, which tolerates a missing `~/.ssh/config` (errMissingSSHConfig → empty form), so the test is deterministic across machines.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestDashboard_OpenSettingsAndImport -v`
Expected: FAIL (`d.openSettings` / `d.openImport` undefined).

- [ ] **Step 3: Implement**

In `internal/guiapp/window.go`, add the two methods near `addTunnel` (after `setStore`):

```go
// openSettings shows the global-defaults editor.
func (d *dashboard) openSettings() {
	if d.store == nil {
		return
	}
	showSettingsDialog(d.win, d.store)
}

// openImport shows the ~/.ssh/config import wizard.
func (d *dashboard) openImport() {
	if d.store == nil {
		return
	}
	cfg, err := d.store.Load()
	if err != nil {
		dialog.ShowError(err, d.win)
		return
	}
	showImportDialog(d.win, cfg, d.store)
}
```

In `internal/guiapp/window.go`, add the two buttons to the `globalZone` in `buildToolbar` (replace the existing two lines that build `reload`/`add`/`globalZone`):

```go
	reload := widget.NewButtonWithIcon("重载", theme.ViewRefreshIcon(), func() { d.run(d.actionReload) })
	settings := widget.NewButtonWithIcon("设置…", theme.SettingsIcon(), d.openSettings)
	importBtn := widget.NewButtonWithIcon("从 ~/.ssh/config 导入…", theme.FolderOpenIcon(), d.openImport)
	add := widget.NewButtonWithIcon("新增隧道", theme.ContentAddIcon(), d.addTunnel)
	add.Importance = widget.HighImportance
	globalZone := container.NewHBox(reload, settings, importBtn, add)
```

In `internal/guiapp/tray.go`, add a `Settings` field to the `Handlers` struct (after `InstallAgent`):

```go
	Settings     func()            // open the global-defaults settings dialog
```

In `buildMenu` (the connected branch), add a "设置…" item to the trailing `items` block (insert after the "重载配置" line):

```go
		fyne.NewMenuItem("设置…", func() { call(h.Settings) }),
```

In `internal/guiapp/app.go`, wire `Settings` in `u.handlers()` (add to the `Handlers{...}` literal, e.g. after `Reload`):

```go
		Settings: func() { fyne.Do(u.dash.openSettings) },
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -run TestDashboard_OpenSettingsAndImport -v`
Then the whole package: `go test ./internal/guiapp/`
Expected: PASS. If any existing test in `tray_test.go`/`window_test.go` asserts an exact toolbar button count or an exact `Handlers` field set, update that assertion to include the new buttons / `Settings` field (the new field is additive and defaults to nil, so menu construction with a nil `Settings` is safe via `call`).

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/window.go internal/guiapp/tray.go internal/guiapp/app.go internal/guiapp/window_test.go
git commit -m "feat(guiapp): wire settings + import wizard into toolbar and tray

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: Full-suite verification

**Files:** none (verification only)

- [ ] **Step 1: Run the whole test suite**

Run: `go test ./...`
Expected: PASS across all packages. (Two pre-existing environment-flaky tests on this Mac — socket-path length and a timing-sensitive test — may fail for env reasons unrelated to this change; confirm any failure matches those known cases before treating it as a regression.)

- [ ] **Step 2: Vet and build**

Run: `go vet ./... && go build ./...`
Expected: no errors.

- [ ] **Step 3: Manual GUI smoke (optional, documents the milestone)**

With a `~/.ssh/config` containing at least one `Host` alias, run the GUI (`HOPD_GUI_OPEN=1` to auto-open the dashboard). Click "从 ~/.ssh/config 导入…", confirm the discovered aliases render with checkboxes (pre-existing ones greyed out), import one, and verify it appears in `~/.config/hopd/config.yaml` under `hosts:`. Then click "设置…", change `重连最短间隔`, save, and confirm `defaults.restart.min` updated in `config.yaml`. Confirm `~/.ssh/config` is byte-for-byte unchanged (read-only, §10).

- [ ] **Step 4: Commit (if any incidental fixes were needed)**

```bash
git add -A
git commit -m "test: full-suite verification for import wizard + defaults editor

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review (author checklist — already applied)

**Spec coverage (Plan 4 scope):**
- §7.6 Import wizard — parse `~/.ssh/config`, list aliases with HostName/Port/User/IdentityFile/ProxyJump + checkboxes → Task 7 (rows + describeImported); import selected as `hosts:` with ProxyJump→jump only when the target is also imported → Tasks 2 (`BuildHostsFromImport`) + 8 (apply). Read-only file, friendly empty state on missing file → Task 7 (`readUserSSHConfig`/`errMissingSSHConfig`).
- §7.3 Settings window — edit `defaults.restart` (min/max) and `defaults.ssh_options`; save via `ConfigStore.Save` (atomic + `.bak` + reload) → Tasks 4–6 (form/apply/check) + 9 (dialog).
- §3/§8 backend getter/setter for `defaults.ssh_options` (the unexported `defaultOpts` that `Marshal` reads) → Task 1.
- §5 rendering rule (ProxyJump mapped to `jump` only where the referenced alias is also imported) → Task 2 + Task 3 (`DroppedJumps` for the user-facing warning).
- §10 security — `~/.ssh/config` only read, never written (injectable reader; production `readUserSSHConfig` is `os.ReadFile` only); no secret material stored (only key *paths* via `IdentityFile`→`Key`) → Tasks 7, 8.
- Reachability from hosts manager and/or tray → Task 10 (toolbar buttons + tray "设置…" item). The import wizard is reachable from the dashboard toolbar; Plan 2's hosts manager may additionally surface it, but this plan does not depend on Plan 2.

**Deferred to / owned by other plans (intentionally out of scope here):** hosts manager UI, `gui.HostForm`, `showHostDialog`, `(d *dashboard).openHosts()` (Plan 2); tunnel-form rewrite, test-connection runner, host-key trust dialog, "迁移为主机" migration action (Plan 3). This plan references Plan 2's hosts manager only as an *optional additional* entry point and does not require it.

**Type consistency:** `config.DefaultOptions()`/`SetDefaultOptions(map[string]string)`; `gui.BuildHostsFromImport(imported []sshconf.ImportedHost, selectedNames []string) (map[string]config.Host, error)`; `gui.ExistingImportNames(cfg, imported) map[string]bool`; `gui.DroppedJumps(imported, selectedNames) map[string]string`; `gui.DefaultsForm{RestartMin,RestartMax,SSHOptions string}`; `gui.ToDefaultsForm(cfg) DefaultsForm`; `(DefaultsForm).Apply(cfg) error`; `gui.CheckDefaults(f) FieldErrors` (keys `restartMin`/`restartMax`/`sshOptions`); `guiapp.newImportForm(cfg, read) (*importForm, error)`, `(*importForm).selectedNames()`, `(*importForm).apply(store)`, `showImportDialog`; `guiapp.newSettingsForm(cfg) *settingsForm`, `(*settingsForm).value()/valid()/refresh()`, `showSettingsDialog`; `Handlers.Settings func()`; `(d *dashboard).openSettings()/openImport()` — names are used identically across all tasks. `FieldErrors` and `Check`-style keying match `internal/gui/formcheck.go`. `itoa`, `text`, `pal`, `newCaption`, `noWheelTrap`, `layoutStackV`, `layoutPadXY`, `call` are reused from existing package `guiapp` code (not redefined).

**Placeholder scan:** none — every code step contains complete code. Task 8 Step 1 explicitly flags and corrects its own illustrative `Host2Jump` line before implementation (use the corrected block). Task 4 adds a temporary `var _ = time.Second` keep-alive that Task 5 removes; this is called out in both tasks.
