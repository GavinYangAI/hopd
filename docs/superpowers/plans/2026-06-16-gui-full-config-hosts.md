# Full GUI Config — Plan 2: Hosts manager + Test-connection + Host-key trust

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the GUI a first-class **hosts manager** (add/edit/delete reusable SSH endpoints), a **测试连接** runner that shells out to ssh through a hopd-generated `-F` config, and a **host-key trust** dialog so the no-TTY GUI never hangs on an unknown key — all built on the Plan 1 backend (`config.Host`, `sshconf.Generate`).

**Architecture:** Pure logic lives in `internal/gui` (no Fyne deps): a `HostForm` model (Parse/ToHostForm), field validation `CheckHost`, and a fully injectable test-connection runner (`testconn.go`) that calls `sshconf.Generate`, writes a temp `0600` `-F` file in a `0700` temp dir, runs `ssh ... true` via an injected `CmdRunner`, and maps exit/stderr into a friendly Chinese reason plus parsed host-key fingerprints. The Fyne layer (`internal/guiapp`) adds a host dialog mirroring `editForm`, a hosts list manager reusing `card.go` chrome, a test/host-key result dialog, and wires "主机…" into the toolbar and tray.

**Tech Stack:** Go, Fyne v2 (`fyne.io/fyne/v2`, `fyne.io/fyne/v2/test` for headless tests), standard `testing` / `os/exec`. Pure-logic packages carry the heavy validation coverage; Fyne tests assert only on constructible/derivable state.

**Scope note:** This is Plan 2 of a multi-plan feature (spec: `docs/superpowers/specs/2026-06-16-gui-full-config-design.md`, §7.1, §7.4, §7.5, §10). It builds on the **locked** Plan 1 backend (`internal/config` host model + mutators, `internal/sshconf.Generate`, `gui.ConfigStore`, `paths.GeneratedDir`). The simplified tunnel-dialog rewrite (§7.2), settings window (§7.3), and import wizard (§7.6) are **Plans 3 & 4** — they reuse the names defined here (`gui.HostForm`, `gui.CheckHost`, `gui.TestConnection`, `showHostDialog`, `(*dashboard).openHosts`).

**Conventions:** Run tests with `go test ./...` from the repo root. Module path is `github.com/GavinYangAI/hopd`. End every commit message body with the trailer:
`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
Work on the existing `feature/gui-full-config` branch.

---

## File Structure

Pure logic (package `gui`, no Fyne imports — mirrors `form.go`/`formcheck.go`):

- `internal/gui/hostform.go` — create: `HostForm` struct, `HostForm.Parse()`, `ToHostForm()`.
- `internal/gui/hostform_test.go` — create: round-trip + parse-error table tests.
- `internal/gui/hostformcheck.go` — create: `CheckHost(f, otherNames, jumpTargets)` live field validation.
- `internal/gui/hostformcheck_test.go` — create: table-driven per-field rule tests.
- `internal/gui/testconn.go` — create: `CmdRunner`, `TestConnResult`, `HostKey`, `TestConnection`, `execRunner`, host-key parse + `RemoveKnownHostEntry` (injectable).
- `internal/gui/testconn_test.go` — create: fake-runner tests for success/auth/unreachable/port/host-key + host-key parsing.

Fyne layer (package `guiapp` — mirrors `editdialog.go`/`window.go`):

- `internal/guiapp/hostdialog.go` — create: `showHostDialog`, `hostForm` widget struct (captions, live `CheckHost`, key-file picker, jump `widget.Select`, 测试连接 button).
- `internal/guiapp/hostdialog_test.go` — create: `test.NewApp()` prefill round-trip + save-button gating.
- `internal/guiapp/hostsmanager.go` — create: `(*dashboard).openHosts()` hosts list window (cards), Add/Edit/Delete wired to `ConfigStore`.
- `internal/guiapp/hostsmanager_test.go` — create: `test.NewApp()` constructs the manager from a stub store.
- `internal/guiapp/testconndialog.go` — create: result dialog + unknown-host-key 信任并保存/取消 flow.
- `internal/guiapp/testconndialog_test.go` — create: `test.NewApp()` constructs both dialog variants.
- `internal/guiapp/window.go` — modify: add a "主机…" button to `globalZone`; add `openHosts` host-manager entry point uses `d.store`.
- `internal/guiapp/tray.go` — modify: add `Hosts func()` to `Handlers`; add a "主机…" item to the connected menu.
- `internal/guiapp/app.go` — modify: wire `Hosts: func() { fyne.Do(u.dash.openHosts) }` into `handlers()`.
- `internal/guiapp/tray_test.go` — modify/add: assert the "主机…" item is present in the connected menu.

---

## Cross-plan contract (defined here, reused by Plans 3 & 4 — names are load-bearing)

```go
// internal/gui/hostform.go
type HostForm struct{ Name, Host, Port, User, KeyFile, Jump, SSHOptions string }
func (f HostForm) Parse() (name string, h config.Host, err error)
func ToHostForm(name string, h config.Host) HostForm

// internal/gui/hostformcheck.go
func CheckHost(f HostForm, otherNames []string, jumpTargets []string) FieldErrors // keys: name,host,port,jump,sshOptions

// internal/gui/testconn.go
type CmdRunner func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)
type HostKey struct{ Host, Algo, Fingerprint string }
type TestConnResult struct{ OK bool; Reason string; Fingerprints []HostKey }
func TestConnection(ctx context.Context, cfg *config.Config, hostName string, run CmdRunner) TestConnResult
func execRunner(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)

// internal/guiapp
func showHostDialog(win fyne.Window, store *gui.ConfigStore, initial gui.HostForm, editingName string, onDone func())
func (d *dashboard) openHosts()
// Handlers gains: Hosts func()
```

---

## Task 1: `HostForm` Parse / ToHostForm round-trip

**Files:**
- Create: `internal/gui/hostform.go`
- Test: `internal/gui/hostform_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/gui/hostform_test.go`:

```go
package gui

import (
	"reflect"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

func TestHostFormParse(t *testing.T) {
	f := HostForm{
		Name: "entryA", Host: "198.51.100.7", Port: "65522", User: "userA",
		KeyFile: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: "ServerAliveInterval=15\nCompression=yes",
	}
	name, h, err := f.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if name != "entryA" {
		t.Fatalf("name = %q, want entryA", name)
	}
	want := config.Host{
		Host: "198.51.100.7", Port: 65522, User: "userA", Key: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: map[string]string{"ServerAliveInterval": "15", "Compression": "yes"},
	}
	if !reflect.DeepEqual(h, want) {
		t.Fatalf("parsed host mismatch:\n got  %+v\n want %+v", h, want)
	}
}

func TestHostFormParseDefaultsAndTrim(t *testing.T) {
	f := HostForm{Name: "  b ", Host: "  h2 ", Port: "", User: " ", KeyFile: " ", Jump: "  "}
	name, h, err := f.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if name != "b" {
		t.Fatalf("name not trimmed: %q", name)
	}
	if h.Host != "h2" || h.Port != 22 || h.User != "" || h.Key != "" || h.Jump != "" {
		t.Fatalf("defaults/trim mismatch: %+v", h)
	}
	if h.SSHOptions != nil {
		t.Fatalf("empty ssh_options should be nil map, got %v", h.SSHOptions)
	}
}

func TestHostFormParseBadOption(t *testing.T) {
	f := HostForm{Name: "a", Host: "h", SSHOptions: "ServerAliveInterval 15"}
	if _, _, err := f.Parse(); err == nil {
		t.Fatal("ssh option without '=' should error")
	}
}

func TestToHostForm(t *testing.T) {
	h := config.Host{
		Host: "198.51.100.7", Port: 65522, User: "userA", Key: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: map[string]string{"Zeta": "z", "Alpha": "a"},
	}
	f := ToHostForm("entryA", h)
	want := HostForm{
		Name: "entryA", Host: "198.51.100.7", Port: "65522", User: "userA",
		KeyFile: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: "Alpha=a\nZeta=z", // sorted
	}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("ToHostForm mismatch:\n got  %+v\n want %+v", f, want)
	}
}

func TestToHostFormDefaultPortBlank(t *testing.T) {
	// Port 22 (and 0) render as "" so the field shows the placeholder default,
	// matching how the tunnel form leaves an implicit jump port blank.
	if got := ToHostForm("b", config.Host{Host: "h", Port: 22}).Port; got != "" {
		t.Fatalf("port 22 should render blank, got %q", got)
	}
	if got := ToHostForm("b", config.Host{Host: "h", Port: 0}).Port; got != "" {
		t.Fatalf("port 0 should render blank, got %q", got)
	}
}

func TestHostFormRoundTrip(t *testing.T) {
	orig := config.Host{
		Host: "h", Port: 2222, User: "u", Key: "~/.ssh/k", Jump: "j",
		SSHOptions: map[string]string{"Compression": "yes"},
	}
	_, got, err := ToHostForm("x", orig).Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(got, orig) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", got, orig)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run 'TestHostForm|TestToHostForm' -v`
Expected: FAIL (compile error — `HostForm`, `ToHostForm` undefined).

- [ ] **Step 3: Implement**

Create `internal/gui/hostform.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run 'TestHostForm|TestToHostForm' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/hostform.go internal/gui/hostform_test.go
git commit -m "feat(gui): HostForm Parse/ToHostForm host model round-trip

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: `CheckHost` — live per-field host validation

**Files:**
- Create: `internal/gui/hostformcheck.go`
- Test: `internal/gui/hostformcheck_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/gui/hostformcheck_test.go`:

```go
package gui

import "testing"

func TestCheckHost_OK(t *testing.T) {
	f := HostForm{Name: "entryA", Host: "198.51.100.7", Port: "65522", User: "u", Jump: "bastionB"}
	errs := CheckHost(f, []string{"other"}, []string{"bastionB", "other"})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestCheckHost_NameRules(t *testing.T) {
	base := HostForm{Host: "h"}
	if CheckHost(base, nil, nil)["name"] == "" {
		t.Fatal("empty name should error")
	}
	f := base
	f.Name = "has space"
	if CheckHost(f, nil, nil)["name"] == "" {
		t.Fatal("name with space should error")
	}
	f = base
	f.Name = "dup"
	if CheckHost(f, []string{"dup"}, nil)["name"] == "" {
		t.Fatal("duplicate name should error")
	}
}

func TestCheckHost_HostRequired(t *testing.T) {
	if CheckHost(HostForm{Name: "a"}, nil, nil)["host"] == "" {
		t.Fatal("empty host should error")
	}
}

func TestCheckHost_Port(t *testing.T) {
	ok := HostForm{Name: "a", Host: "h"}
	if CheckHost(ok, nil, nil)["port"] != "" {
		t.Fatal("empty port should be allowed (defaults to 22)")
	}
	bad := ok
	bad.Port = "70000"
	if CheckHost(bad, nil, nil)["port"] == "" {
		t.Fatal("out-of-range port should error")
	}
	bad = ok
	bad.Port = "-p22"
	if CheckHost(bad, nil, nil)["port"] == "" {
		t.Fatal("port with -p should error")
	}
}

func TestCheckHost_Jump(t *testing.T) {
	base := HostForm{Name: "a", Host: "h"}
	ok := base
	ok.Jump = ""
	if CheckHost(ok, nil, nil)["jump"] != "" {
		t.Fatal("empty jump should be allowed")
	}
	unknown := base
	unknown.Jump = "ghost"
	if CheckHost(unknown, nil, []string{"realhost"})["jump"] == "" {
		t.Fatal("jump to a non-existent host should error")
	}
	self := base
	self.Jump = "a"
	if CheckHost(self, nil, []string{"a"})["jump"] == "" {
		t.Fatal("self-jump should error")
	}
}

func TestCheckHost_SSHOptions(t *testing.T) {
	base := HostForm{Name: "a", Host: "h"}
	bad := base
	bad.SSHOptions = "ServerAliveInterval 15" // no '='
	if CheckHost(bad, nil, nil)["sshOptions"] == "" {
		t.Fatal("option line without '=' should error")
	}
	dashP := base
	dashP.SSHOptions = "Foo=-p2222"
	if CheckHost(dashP, nil, nil)["sshOptions"] == "" {
		t.Fatal("option containing -p should error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run TestCheckHost -v`
Expected: FAIL (compile error — `CheckHost` undefined).

- [ ] **Step 3: Implement**

Create `internal/gui/hostformcheck.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run TestCheckHost -v`
Then the package: `go test ./internal/gui/ -v`
Expected: PASS (existing gui tests still green; `hostCheckOrder` is referenced by the dialog in Task 5, declared now to keep the file self-contained).

> If `go vet` flags `hostCheckOrder` as unused before Task 5 lands, that is expected and resolved by Task 5; the test run above is `go test`, which does not fail on an unused package-level var.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/hostformcheck.go internal/gui/hostformcheck_test.go
git commit -m "feat(gui): CheckHost live per-field host validation

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Host-key parsing helpers (pure)

**Files:**
- Create: `internal/gui/testconn.go` (host-key helpers only this task)
- Test: `internal/gui/testconn_test.go`

This task lands the pure host-key parsing used by both the test runner (Task 4) and the trust dialog (Task 8). The `ssh ... -o StrictHostKeyChecking=accept-new` path writes new keys to `~/.ssh/known_hosts` and reports the fingerprint on stderr in the form `Warning: Permanently added '<host>' (<ALGO>) to the list of known hosts.` plus a separate `<ALGO> key fingerprint is SHA256:...` line.

- [ ] **Step 1: Write the failing test**

Create `internal/gui/testconn_test.go`:

```go
package gui

import (
	"reflect"
	"testing"
)

func TestParseHostKeys(t *testing.T) {
	stderr := `Warning: Permanently added '198.51.100.7' (ED25519) to the list of known hosts.
ED25519 key fingerprint is SHA256:abc123def456.
userA@198.51.100.7: Permission denied (publickey).`
	got := parseHostKeys([]byte(stderr))
	want := []HostKey{
		{Host: "198.51.100.7", Algo: "ED25519", Fingerprint: "SHA256:abc123def456"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseHostKeys mismatch:\n got  %+v\n want %+v", got, want)
	}
}

func TestParseHostKeys_None(t *testing.T) {
	if got := parseHostKeys([]byte("no key info here\n")); len(got) != 0 {
		t.Fatalf("expected no host keys, got %+v", got)
	}
}

func TestParseHostKeys_AddedWithoutFingerprintLine(t *testing.T) {
	// Some ssh builds only emit the "Permanently added" line.
	stderr := `Warning: Permanently added 'bastionB' (RSA) to the list of known hosts.`
	got := parseHostKeys([]byte(stderr))
	if len(got) != 1 || got[0].Host != "bastionB" || got[0].Algo != "RSA" {
		t.Fatalf("expected one host key for bastionB/RSA, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run TestParseHostKeys -v`
Expected: FAIL (compile error — `parseHostKeys`, `HostKey` undefined).

- [ ] **Step 3: Implement**

Create `internal/gui/testconn.go` with the types + parser (the runner is added in Task 4):

```go
package gui

import (
	"bufio"
	"bytes"
	"strings"
)

// HostKey is one host-key fingerprint surfaced by a test connection, so the
// trust dialog can show what was added to ~/.ssh/known_hosts.
type HostKey struct {
	Host        string // hostname / IP as ssh names it
	Algo        string // ED25519 / RSA / ECDSA …
	Fingerprint string // SHA256:… ("" if ssh didn't print one)
}

// parseHostKeys extracts host-key info from ssh stderr produced under
// StrictHostKeyChecking=accept-new. It pairs each
//   Warning: Permanently added '<host>' (<ALGO>) to the list of known hosts.
// line with a following
//   <ALGO> key fingerprint is SHA256:…
// line when present.
func parseHostKeys(stderr []byte) []HostKey {
	var keys []HostKey
	fps := map[string]string{} // ALGO -> fingerprint
	sc := bufio.NewScanner(bytes.NewReader(stderr))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "key fingerprint is "); i >= 0 {
			algo := strings.ToUpper(strings.TrimSpace(line[:i]))
			fp := strings.TrimRight(strings.TrimSpace(line[i+len("key fingerprint is "):]), ".")
			fps[algo] = fp
		}
	}
	sc = bufio.NewScanner(bytes.NewReader(stderr))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "Warning: Permanently added") {
			continue
		}
		host := between(line, "'", "'")
		algo := strings.ToUpper(between(line, "(", ")"))
		keys = append(keys, HostKey{Host: host, Algo: algo, Fingerprint: fps[algo]})
	}
	return keys
}

// between returns the substring between the first open and the next close after
// it; "" if not found.
func between(s, open, close string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return ""
	}
	return rest[:j]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -run TestParseHostKeys -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/testconn.go internal/gui/testconn_test.go
git commit -m "feat(gui): parse ssh host-key fingerprints from stderr

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `TestConnection` runner (injectable, temp `-F` config, friendly reasons)

**Files:**
- Modify: `internal/gui/testconn.go` (add the runner + result type + reason mapping + `execRunner` + `RemoveKnownHostEntry`)
- Test: `internal/gui/testconn_test.go`

`TestConnection` builds a synthetic single-hop tunnel referencing `hostName` (so `sshconf.Generate` renders the host's full chain), writes the text to a `0600` `-F` file inside a `0700` `os.MkdirTemp` dir (cleaned up), then runs ssh through the injected `CmdRunner`. Exit status + stderr map to a friendly Chinese reason. On a clean exit it reports OK with any newly-added host keys.

- [ ] **Step 1: Write the failing test**

Add to `internal/gui/testconn_test.go`:

```go
import (
	"context"
	"errors"

	"github.com/GavinYangAI/hopd/internal/config"
)

func testConnCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(`
hosts:
  bastionB: {host: 203.0.113.9, user: userB}
  entryA: {host: 198.51.100.7, port: 65522, user: userA, key: ~/.ssh/idA, jump: bastionB}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: entryA}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cfg
}

func TestTestConnection_Success(t *testing.T) {
	cfg := testConnCfg(t)
	var gotArgs []string
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name != "ssh" {
			t.Fatalf("ran %q, want ssh", name)
		}
		gotArgs = args
		return nil, nil, nil // clean exit
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if !res.OK {
		t.Fatalf("expected OK, got %+v", res)
	}
	// argv must point ssh at a -F config, use BatchMode + accept-new, run "true".
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"-F", "BatchMode=yes", "ConnectTimeout=8", "StrictHostKeyChecking=accept-new", "entryA", "true"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %v missing %q", gotArgs, want)
		}
	}
}

func TestTestConnection_AuthFailure(t *testing.T) {
	cfg := testConnCfg(t)
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("userA@198.51.100.7: Permission denied (publickey)."), errors.New("exit status 255")
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if res.OK {
		t.Fatal("auth failure should not be OK")
	}
	if !strings.Contains(res.Reason, "认证") {
		t.Fatalf("reason should mention auth, got %q", res.Reason)
	}
}

func TestTestConnection_Unreachable(t *testing.T) {
	cfg := testConnCfg(t)
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("ssh: connect to host 198.51.100.7 port 65522: Connection timed out"), errors.New("exit status 255")
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if res.OK || !strings.Contains(res.Reason, "连不上") {
		t.Fatalf("expected unreachable reason, got %+v", res)
	}
}

func TestTestConnection_WrongPort(t *testing.T) {
	cfg := testConnCfg(t)
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("ssh: connect to host 198.51.100.7 port 65522: Connection refused"), errors.New("exit status 255")
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if res.OK || !strings.Contains(res.Reason, "端口") {
		t.Fatalf("expected port reason, got %+v", res)
	}
}

func TestTestConnection_NewHostKey(t *testing.T) {
	cfg := testConnCfg(t)
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("Warning: Permanently added '198.51.100.7' (ED25519) to the list of known hosts.\nED25519 key fingerprint is SHA256:abc."), nil
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if !res.OK {
		t.Fatalf("clean exit should be OK, got %+v", res)
	}
	if len(res.Fingerprints) != 1 || res.Fingerprints[0].Fingerprint != "SHA256:abc" {
		t.Fatalf("expected one parsed host key, got %+v", res.Fingerprints)
	}
}

func TestTestConnection_UnknownHost(t *testing.T) {
	cfg := testConnCfg(t)
	res := TestConnection(context.Background(), cfg, "nope", func(context.Context, string, ...string) ([]byte, []byte, error) {
		t.Fatal("runner should not be called for an unknown host")
		return nil, nil, nil
	})
	if res.OK {
		t.Fatal("unknown host should not be OK")
	}
}
```

Ensure `testconn_test.go` has `"strings"` in its import block (add it alongside the new imports).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gui/ -run TestTestConnection -v`
Expected: FAIL (compile error — `TestConnection`, `CmdRunner`, `TestConnResult` undefined).

- [ ] **Step 3: Implement**

Add to `internal/gui/testconn.go`. First extend the import block:

```go
import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)
```

Then append:

```go
// CmdRunner runs a command and returns its stdout, stderr, and error. Injected
// so TestConnection is unit-testable without spawning real ssh.
type CmdRunner func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)

// TestConnResult is the outcome of a test connection.
type TestConnResult struct {
	OK           bool
	Reason       string    // friendly Chinese reason ("" when OK with no caveat)
	Fingerprints []HostKey // host keys ssh added/saw during this attempt
}

// TestConnection verifies that hopd can ssh to hostName's chain. It builds a
// synthetic via_host tunnel so sshconf.Generate renders the full chain, writes
// the config to a 0600 file inside a 0700 temp dir (removed before returning),
// then runs `ssh -F <tmp> -o BatchMode=yes -o ConnectTimeout=8 -o
// StrictHostKeyChecking=accept-new <entry> true`. A clean exit is OK; otherwise
// stderr maps to a friendly reason.
func TestConnection(ctx context.Context, cfg *config.Config, hostName string, run CmdRunner) TestConnResult {
	if _, ok := cfg.Host(hostName); !ok {
		return TestConnResult{OK: false, Reason: "找不到主机 " + hostName}
	}
	synthetic := config.Tunnel{
		Name: "__hopd_test__", Local: "0", Remote: "127.0.0.1:1", ViaHost: hostName,
	}
	text, entry, err := sshconf.Generate(cfg, synthetic)
	if err != nil {
		return TestConnResult{OK: false, Reason: "生成 ssh 配置失败：" + err.Error()}
	}

	dir, err := os.MkdirTemp("", "hopd-test-*")
	if err != nil {
		return TestConnResult{OK: false, Reason: "创建临时目录失败：" + err.Error()}
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o700); err != nil {
		return TestConnResult{OK: false, Reason: "设置临时目录权限失败：" + err.Error()}
	}
	cfgPath := filepath.Join(dir, "test.sshcfg")
	if err := os.WriteFile(cfgPath, []byte(text), 0o600); err != nil {
		return TestConnResult{OK: false, Reason: "写入临时配置失败：" + err.Error()}
	}

	_, stderr, runErr := run(ctx, "ssh",
		"-F", cfgPath,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=8",
		"-o", "StrictHostKeyChecking=accept-new",
		entry, "true",
	)
	keys := parseHostKeys(stderr)
	if runErr == nil {
		return TestConnResult{OK: true, Fingerprints: keys}
	}
	return TestConnResult{OK: false, Reason: reasonFromStderr(string(stderr)), Fingerprints: keys}
}

// reasonFromStderr maps common ssh failures to a short Chinese explanation.
func reasonFromStderr(stderr string) string {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "permission denied"):
		return "认证失败：检查用户名、密钥或目标是否允许该用户登录"
	case strings.Contains(s, "connection refused"):
		return "端口不通：对方拒绝连接，确认 SSH 端口是否正确"
	case strings.Contains(s, "connection timed out"), strings.Contains(s, "no route to host"),
		strings.Contains(s, "could not resolve hostname"), strings.Contains(s, "operation timed out"):
		return "连不上：主机不可达或域名解析失败，检查地址、网络或跳板"
	case strings.Contains(s, "host key verification failed"), strings.Contains(s, "remote host identification has changed"):
		return "主机密钥校验失败：known_hosts 里的记录与对方不一致"
	case strings.TrimSpace(stderr) == "":
		return "连接失败"
	default:
		return "连接失败：" + firstStderrLine(stderr)
	}
}

func firstStderrLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return s
}

// execRunner is the default real CmdRunner used by the GUI: it runs the command
// with exec.CommandContext and returns stdout/stderr separately.
func execRunner(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err = cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

// RemoveKnownHostEntry removes hostName's entries from ~/.ssh/known_hosts via
// `ssh-keygen -R <host>`. Used by the trust dialog's 取消 path to undo a key
// that accept-new just added. The runner is injected for testing.
func RemoveKnownHostEntry(ctx context.Context, hostName string, run CmdRunner) error {
	_, stderr, err := run(ctx, "ssh-keygen", "-R", hostName)
	if err != nil {
		return fmt.Errorf("ssh-keygen -R %s: %v: %s", hostName, err, strings.TrimSpace(string(stderr)))
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gui/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gui/testconn.go internal/gui/testconn_test.go
git commit -m "feat(gui): TestConnection runner with temp -F config and friendly reasons

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Host dialog (construct + prefill round-trip)

**Files:**
- Create: `internal/guiapp/hostdialog.go`
- Test: `internal/guiapp/hostdialog_test.go`

This task builds the `hostForm` widget struct (mirroring `editForm`) and `newHostForm`, prefilled from a `gui.HostForm`, with captions and a value() reader. Live validation, the 测试连接 button, and `showHostDialog` come in Task 6. The jump field is a `widget.Select` populated from existing host names.

- [ ] **Step 1: Write the failing test**

Create `internal/guiapp/hostdialog_test.go`:

```go
package guiapp

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestNewHostForm_PrefillsAndReads(t *testing.T) {
	_ = test.NewApp()
	initial := gui.HostForm{
		Name: "entryA", Host: "198.51.100.7", Port: "65522", User: "userA",
		KeyFile: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: "Compression=yes",
	}
	hf := newHostForm(initial, []string{"bastionB"})
	got := hf.value()
	if !reflect.DeepEqual(got, initial) {
		t.Fatalf("prefill round-trip differs:\n a=%+v\n b=%+v", initial, got)
	}
}

func TestNewHostForm_JumpOptions(t *testing.T) {
	_ = test.NewApp()
	hf := newHostForm(gui.HostForm{Name: "a", Host: "h"}, []string{"b", "c"})
	// The jump select must offer the candidate hosts plus the empty "（不用跳板）"
	// sentinel, and never list the host being edited.
	for _, want := range []string{"b", "c"} {
		found := false
		for _, o := range hf.jump.Options {
			if o == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("jump options %v missing %q", hf.jump.Options, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestNewHostForm -v`
Expected: FAIL (compile error — `newHostForm` undefined).

- [ ] **Step 3: Implement**

Create `internal/guiapp/hostdialog.go`:

```go
package guiapp

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// noJump is the sentinel shown in the jump Select for "no jump host".
const noJump = "（不用跳板）"

// hostForm wraps the Fyne widgets for the host editor. It mirrors editForm but
// is simpler: there is no route choice.
type hostForm struct {
	root fyne.CanvasObject

	name, host, port, user *widget.Entry
	keyFile, sshOptions    *widget.Entry
	jump                   *widget.Select

	editingName string   // "" => adding; non-empty => editing (uniqueness/jump exclude this)
	allNames    []string // every existing host name (for jump targets / uniqueness)

	captions   map[string]*captionLabel
	footStatus *canvas.Text
	saveBtn    *widget.Button
	testBtn    *widget.Button

	onSave   func()
	onCancel func()
	onTest   func()
}

// newHostForm builds the host editor prefilled from f. jumpCandidates are the
// names a jump may reference (existing hosts, excluding the one being edited).
func newHostForm(f gui.HostForm, jumpCandidates []string) *hostForm {
	hf := &hostForm{
		name:        widget.NewEntry(),
		host:        widget.NewEntry(),
		port:        widget.NewEntry(),
		user:        widget.NewEntry(),
		keyFile:     widget.NewEntry(),
		sshOptions:  widget.NewMultiLineEntry(),
		editingName: f.Name,
		allNames:    jumpCandidates,
		captions:    map[string]*captionLabel{},
	}
	hf.jump = widget.NewSelect(append([]string{noJump}, jumpCandidates...), nil)

	for _, p := range []struct {
		e  *widget.Entry
		ph string
	}{
		{hf.name, "entryA"}, {hf.host, "198.51.100.7"}, {hf.port, "22"},
		{hf.user, "ops"}, {hf.keyFile, "~/.ssh/id_ed25519"},
	} {
		p.e.SetPlaceHolder(p.ph)
	}
	hf.sshOptions.SetPlaceHolder("ServerAliveInterval=30\nCompression=yes")
	noWheelTrap(hf.name, hf.host, hf.port, hf.user, hf.keyFile)

	hf.name.SetText(f.Name)
	hf.host.SetText(f.Host)
	hf.port.SetText(f.Port)
	hf.user.SetText(f.User)
	hf.keyFile.SetText(f.KeyFile)
	hf.sshOptions.SetText(f.SSHOptions)
	if f.Jump != "" {
		hf.jump.SetSelected(f.Jump)
	} else {
		hf.jump.SetSelected(noJump)
	}

	hf.build(f)
	hf.refresh()
	return hf
}

func (hf *hostForm) build(f gui.HostForm) {
	editing := f.Name != ""
	titleStr, subStr := "新增主机", "保存一台可复用的 SSH 跳板/入口"
	if editing {
		titleStr, subStr = "编辑主机", "修改 "+f.Name+" 的连接参数"
	}
	header := container.New(layoutStackV{gap: 3},
		text(titleStr, 18, pal.text1, bold),
		text(subStr, 13, pal.text2, fyne.TextStyle{}),
	)

	keyRow := container.NewBorder(nil, nil, nil,
		widget.NewButtonWithIcon("浏览…", theme.FolderOpenIcon(), hf.pickKeyFile), hf.keyFile)

	sec1 := container.New(layoutStackV{gap: 12},
		sectionHeader(1, "基本信息", ""),
		container.NewGridWithColumns(2,
			hf.field("名称", true, "这台主机的唯一名字", "name", hf.name),
			hf.field("主机地址", true, "IP 或域名", "host", hf.host),
		),
		container.NewGridWithColumns(2,
			hf.field("端口", false, "默认 22", "port", hf.port),
			hf.field("用户名", false, "登录用户", "user", hf.user),
		),
		hf.field("密钥文件", false, "SSH 私钥路径，留空用 ssh-agent", "keyFile", keyRow),
	)

	sec2 := container.New(layoutStackV{gap: 12},
		sectionHeader(2, "跳板（可选）", ""),
		hf.field("经由主机", false, "先连这台已配好的主机，再连本机", "jump", hf.jump),
	)

	sec3 := container.New(layoutStackV{gap: 12},
		sectionHeader(3, "高级选项", ""),
		hf.field("其它 ssh 选项", false, "多行 key=value，给高级用户", "sshOptions", hf.sshOptions),
	)

	bodyCol := container.New(layoutStackV{gap: 0},
		section(sec1), sectionDivider(),
		section(sec2), sectionDivider(),
		section(sec3),
	)
	bodyScroll := container.NewVScroll(container.New(layoutPadXY{px: 20, py: 6}, bodyCol))

	hf.footStatus = text("", 12.5, pal.text2, fyne.TextStyle{})
	hf.testBtn = widget.NewButtonWithIcon("测试连接", theme.MediaPlayIcon(), func() { call(hf.onTest) })
	cancel := widget.NewButton("取消", func() { call(hf.onCancel) })
	hf.saveBtn = widget.NewButtonWithIcon(saveLabel(editing), theme.ConfirmIcon(), func() { call(hf.onSave) })
	hf.saveBtn.Importance = widget.HighImportance
	footBar := container.NewBorder(nil, nil, hf.footStatus, container.NewHBox(hf.testBtn, cancel, hf.saveBtn))
	footBg := canvas.NewRectangle(pal.barBot)
	footSep := canvas.NewRectangle(pal.border)
	footSep.SetMinSize(fyne.NewSize(0, 1))
	footer := container.NewStack(footBg,
		container.NewBorder(footSep, nil, nil, nil, container.New(layoutPadXY{px: 20, py: 12}, footBar)))

	headerArea := container.New(layoutPadXY{px: 20, py: 16}, header)
	hf.root = container.NewBorder(headerArea, footer, nil, nil, bodyScroll)

	for _, e := range []*widget.Entry{hf.name, hf.host, hf.port, hf.user, hf.keyFile, hf.sshOptions} {
		e.OnChanged = func(string) { hf.refresh() }
	}
	hf.jump.OnChanged = func(string) { hf.refresh() }
}

func (hf *hostForm) field(label string, required bool, help, key string, w fyne.CanvasObject) fyne.CanvasObject {
	lblRow := text(label, 12.5, pal.text1, fyne.TextStyle{Bold: true})
	var head fyne.CanvasObject = lblRow
	if required {
		head = container.NewHBox(lblRow, text(" *", 12.5, pal.accentH, bold))
	}
	cap := newCaption(help)
	hf.captions[key] = cap
	return container.New(layoutStackV{gap: 6}, head, w, cap.obj)
}

// pickKeyFile opens a file chooser starting at ~/.ssh and fills the key entry.
func (hf *hostForm) pickKeyFile() {
	wins := fyne.CurrentApp().Driver().AllWindows()
	if len(wins) == 0 {
		return
	}
	dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		hf.keyFile.SetText(r.URI().Path())
	}, wins[len(wins)-1])
	if home, herr := os.UserHomeDir(); herr == nil {
		if lister, lerr := storage.ListerForURI(storage.NewFileURI(filepath.Join(home, ".ssh"))); lerr == nil {
			dlg.SetLocation(lister)
		}
	}
	dlg.Show()
}

// value reads the current field values back into a gui.HostForm.
func (hf *hostForm) value() gui.HostForm {
	jump := hf.jump.Selected
	if jump == noJump {
		jump = ""
	}
	return gui.HostForm{
		Name:       hf.name.Text,
		Host:       hf.host.Text,
		Port:       hf.port.Text,
		User:       hf.user.Text,
		KeyFile:    hf.keyFile.Text,
		Jump:       jump,
		SSHOptions: hf.sshOptions.Text,
	}
}

// otherNames returns existing host names excluding the one being edited (for
// uniqueness). jumpTargets is the same set (a host may jump to any other host).
func (hf *hostForm) otherNames() []string {
	out := make([]string, 0, len(hf.allNames))
	for _, n := range hf.allNames {
		if n != hf.editingName {
			out = append(out, n)
		}
	}
	return out
}

// refresh re-runs validation and updates captions, footer, and the save button.
func (hf *hostForm) refresh() {
	errs := gui.CheckHost(hf.value(), hf.otherNames(), hf.otherNames())
	for key, cap := range hf.captions {
		cap.set(errs[key], "")
	}
	ok := len(errs) == 0
	if hf.footStatus != nil {
		if ok {
			hf.footStatus.Text = "✓ 可以保存"
			hf.footStatus.Color = pal.up
		} else {
			hf.footStatus.Text = "还有 " + itoa(len(errs)) + " 处要填/改"
			hf.footStatus.Color = pal.text2
		}
		hf.footStatus.Refresh()
	}
	if hf.saveBtn != nil {
		if ok {
			hf.saveBtn.Enable()
		} else {
			hf.saveBtn.Disable()
		}
	}
}

func (hf *hostForm) valid() bool {
	return len(gui.CheckHost(hf.value(), hf.otherNames(), hf.otherNames())) == 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -run TestNewHostForm -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/hostdialog.go internal/guiapp/hostdialog_test.go
git commit -m "feat(guiapp): host editor form (prefill, jump select, key picker)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: `showHostDialog` — save gating + ConfigStore wiring + test button

**Files:**
- Modify: `internal/guiapp/hostdialog.go`
- Test: `internal/guiapp/hostdialog_test.go`

`showHostDialog` presents the host form modally; on save it loads the config, `AddHost`/`UpdateHost`s the parsed host, saves through `ConfigStore`, and handles `ErrReloadAfterSave` exactly like `deleteTunnel` (soft info, not a red error). The 测试连接 button parses the current host, applies it to a fresh copy of the config in memory, and runs `gui.TestConnection` with `execRunner`, then shows the test/host-key dialog (Task 8).

- [ ] **Step 1: Write the failing test**

Add to `internal/guiapp/hostdialog_test.go`:

```go
func TestHostForm_SaveButtonGating(t *testing.T) {
	_ = test.NewApp()
	hf := newHostForm(gui.HostForm{}, nil) // empty: name+host missing => invalid
	if !hf.saveBtn.Disabled() {
		t.Fatal("save should be disabled while required fields are empty")
	}
	hf.name.SetText("entryA")
	hf.host.SetText("198.51.100.7")
	hf.refresh()
	if hf.saveBtn.Disabled() {
		t.Fatalf("save should enable once name+host are filled")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestHostForm_SaveButtonGating -v`
Expected: FAIL initially only if `Disabled()` gating is wrong — but since `refresh()` already gates from Task 5, this test should pass against Task 5 code. To make this task TDD-meaningful, the *failing* artifact is `showHostDialog` itself; add this compile-driving test too:

```go
func TestShowHostDialog_Constructs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("t")
	store := gui.NewConfigStore(filepath.Join(t.TempDir(), "config.yaml"), func() error { return nil })
	// Must not panic constructing/showing the dialog.
	showHostDialog(win, store, gui.HostForm{Name: "a", Host: "h"}, "", func() {})
}
```

Add `"path/filepath"` to the test file's imports.

Run: `go test ./internal/guiapp/ -run 'TestShowHostDialog|TestHostForm_SaveButtonGating' -v`
Expected: FAIL (compile error — `showHostDialog` undefined).

- [ ] **Step 3: Implement**

Append to `internal/guiapp/hostdialog.go`. Extend the import block with:

```go
	"context"
	"errors"
	"time"

	"github.com/GavinYangAI/hopd/internal/config"
```

Then add:

```go
// showHostDialog presents the host editor modally. On save it adds/updates the
// host through the store; editingName == "" means adding. onDone runs after a
// successful save (e.g. to refresh a list).
func showHostDialog(win fyne.Window, store *gui.ConfigStore, initial gui.HostForm, editingName string, onDone func()) {
	candidates := jumpCandidates(store, editingName)
	hf := newHostForm(initial, candidates)
	hf.editingName = editingName

	dlg := dialog.NewCustomWithoutButtons(hostDialogTitle(editingName), hf.root, win)
	dlg.Resize(fyne.NewSize(620, 560))
	hf.onCancel = dlg.Hide
	hf.onSave = func() {
		if !hf.valid() {
			hf.refresh()
			return
		}
		if err := saveHost(store, editingName, hf.value()); err != nil {
			if errors.Is(err, gui.ErrReloadAfterSave) {
				dlg.Hide()
				dialog.ShowInformation("已保存", "主机已保存。daemon 未运行，将在它启动后生效。", win)
				call(onDone)
				return
			}
			dialog.ShowError(err, win)
			return
		}
		dlg.Hide()
		call(onDone)
	}
	hf.onTest = func() { runHostTest(win, store, hf.value()) }
	dlg.Show()
}

func hostDialogTitle(editingName string) string {
	if editingName == "" {
		return "新增主机"
	}
	return "编辑主机"
}

// jumpCandidates returns existing host names (excluding editingName) usable as
// jump targets. A load failure yields an empty list (the dialog still opens).
func jumpCandidates(store *gui.ConfigStore, editingName string) []string {
	cfg, err := store.Load()
	if err != nil {
		return nil
	}
	var names []string
	for name := range cfg.Hosts() {
		if name != editingName {
			names = append(names, name)
		}
	}
	return names
}

// saveHost loads, mutates, and saves the config for one host add/update.
func saveHost(store *gui.ConfigStore, editingName string, f gui.HostForm) error {
	name, h, err := f.Parse()
	if err != nil {
		return err
	}
	cfg, err := store.Load()
	if err != nil {
		return err
	}
	if editingName == "" {
		if err := cfg.AddHost(name, h); err != nil {
			return err
		}
	} else if name == editingName {
		if err := cfg.UpdateHost(name, h); err != nil {
			return err
		}
	} else {
		// Rename: add the new name, then remove the old (UpdateHost can't rename).
		if err := cfg.AddHost(name, h); err != nil {
			return err
		}
		if err := cfg.RemoveHost(editingName); err != nil {
			return err
		}
	}
	return store.Save(cfg)
}

// runHostTest applies the in-progress host to a fresh config copy in memory and
// runs a test connection, then shows the result/host-key dialog.
func runHostTest(win fyne.Window, store *gui.ConfigStore, f gui.HostForm) {
	name, h, err := f.Parse()
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	cfg, err := store.Load()
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	// Apply the edited host into the copy so Generate sees the current values,
	// without persisting anything.
	if _, ok := cfg.Host(name); ok {
		_ = cfg.UpdateHost(name, h)
	} else {
		_ = cfg.AddHost(name, h)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	go func() {
		defer cancel()
		res := gui.TestConnection(ctx, cfg, name, execRunner)
		fyne.Do(func() { showTestConnDialog(win, name, res) })
	}()
}

// ensure config import is used even if a future edit removes the only reference.
var _ = config.Host{}
```

> The trailing `var _ = config.Host{}` keeps the `config` import referenced; remove it if any later edit adds a direct `config.` use in this file.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -v`
Expected: PASS. (`showTestConnDialog` is defined in Task 8; if implementing strictly task-by-task, add a temporary stub `func showTestConnDialog(fyne.Window, string, gui.TestConnResult) {}` to make this task compile, then replace it in Task 8. Note this in the commit.)

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/hostdialog.go internal/guiapp/hostdialog_test.go
git commit -m "feat(guiapp): showHostDialog save/rename gating + test-connection trigger

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Hosts manager window (list cards + Add/Edit/Delete)

**Files:**
- Create: `internal/guiapp/hostsmanager.go`
- Test: `internal/guiapp/hostsmanager_test.go`

`(*dashboard).openHosts()` opens a dedicated window listing saved hosts as cards (name, host:port, user, jump chain) using `card.go` chrome (`roundRect`, `layoutStackV`, `layoutPadXY`, `newTappable`, `text`). Add / Edit / Delete wire to `showHostDialog` and `ConfigStore`.

- [ ] **Step 1: Write the failing test**

Create `internal/guiapp/hostsmanager_test.go`:

```go
package guiapp

import (
	"path/filepath"
	"os"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestOpenHosts_Constructs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 65522, user: userA, jump: bastionB}
  bastionB: {host: 203.0.113.9, user: userB}
groups: {}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	d := newDashboard(app, &DashboardActions{})
	d.setStore(gui.NewConfigStore(path, func() error { return nil }))
	// Must not panic building the hosts window from a real store.
	d.openHosts()
	if d.hostsWin == nil {
		t.Fatal("hosts window not created")
	}
}

func TestHostCardSummary(t *testing.T) {
	_ = test.NewApp()
	// hostSummary renders host:port · user · ↳jump for the card subtitle.
	got := hostSummary(gui.HostForm{Host: "198.51.100.7", Port: "65522", User: "userA", Jump: "bastionB"})
	for _, want := range []string{"198.51.100.7:65522", "userA", "bastionB"} {
		if !contains(got, want) {
			t.Fatalf("summary %q missing %q", got, want)
		}
	}
}

func contains(s, sub string) bool { return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run 'TestOpenHosts|TestHostCardSummary' -v`
Expected: FAIL (compile error — `openHosts`, `hostsWin`, `hostSummary` undefined).

- [ ] **Step 3: Implement**

First add the `hostsWin` field to the dashboard struct in `internal/guiapp/window.go`. Locate the struct field block:

```go
	startStop *widget.Button
	rowBtns   []*widget.Button // per-selection action buttons to enable/disable
```

and add after it:

```go
	hostsWin  fyne.Window // lazily-created hosts manager window
```

Create `internal/guiapp/hostsmanager.go`:

```go
package guiapp

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// openHosts opens (or focuses) the hosts manager window.
func (d *dashboard) openHosts() {
	if d.store == nil {
		return
	}
	if d.hostsWin == nil {
		d.hostsWin = d.app.NewWindow("hopd · 主机")
		d.hostsWin.SetIcon(logoResource)
		d.hostsWin.Resize(fyne.NewSize(560, 600))
		d.hostsWin.SetCloseIntercept(func() { d.hostsWin.Hide() })
	}
	d.refreshHosts()
	d.hostsWin.Show()
}

// refreshHosts rebuilds the hosts window content from the store.
func (d *dashboard) refreshHosts() {
	if d.hostsWin == nil {
		return
	}
	cfg, err := d.store.Load()
	if err != nil {
		dialog.ShowError(err, d.hostsWin)
		return
	}

	names := make([]string, 0)
	for name := range cfg.Hosts() {
		names = append(names, name)
	}
	sort.Strings(names)

	var cards []fyne.CanvasObject
	if len(names) == 0 {
		cards = append(cards, container.NewPadded(container.NewCenter(
			text("还没有主机，点右上角「新增主机」。", 13, pal.text3, fyne.TextStyle{}))))
	}
	for _, name := range names {
		h, _ := cfg.Host(name)
		f := gui.ToHostForm(name, h)
		cards = append(cards, hostCard(name, f, d.editHost, d.deleteHost))
	}
	body := container.New(layoutPadXY{px: 14, py: 12}, container.New(layoutStackV{gap: 9}, cards...))

	add := widget.NewButtonWithIcon("新增主机", theme.ContentAddIcon(), d.addHost)
	add.Importance = widget.HighImportance
	toolbar := container.New(layoutPadXY{px: 14, py: 9}, container.NewBorder(nil, nil, nil, add,
		text("已保存的主机", 13, pal.text2, bold)))
	tbBg := canvas.NewRectangle(pal.barTop)
	tbSep := canvas.NewRectangle(pal.border)
	tbSep.SetMinSize(fyne.NewSize(0, 1))
	header := container.NewStack(tbBg, container.NewBorder(nil, tbSep, nil, nil, toolbar))

	d.hostsWin.SetContent(container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
}

// hostSummary renders a one-line subtitle for a host card.
func hostSummary(f gui.HostForm) string {
	port := f.Port
	if strings.TrimSpace(port) == "" {
		port = "22"
	}
	parts := []string{f.Host + ":" + port}
	if u := strings.TrimSpace(f.User); u != "" {
		parts = append(parts, u)
	}
	if j := strings.TrimSpace(f.Jump); j != "" {
		parts = append(parts, "↳ "+j)
	}
	return strings.Join(parts, "  ·  ")
}

// hostCard builds one host row with edit/delete actions.
func hostCard(name string, f gui.HostForm, onEdit, onDelete func(string)) fyne.CanvasObject {
	title := text(name, 15, pal.text1, bold)
	sub := text(hostSummary(f), 12, pal.text2, mono)
	info := container.New(layoutStackV{gap: 4}, title, sub)

	edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { onEdit(name) })
	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { onDelete(name) })
	del.Importance = widget.DangerImportance
	actions := container.NewHBox(edit, del)

	bg := roundRect(pal.surface1, 12, 1, pal.border)
	inner := container.NewBorder(nil, nil, nil, actions, info)
	return container.NewStack(bg, container.New(layoutPadXY{px: 14, py: 12}, inner))
}

func (d *dashboard) addHost() {
	showHostDialog(d.hostsWin, d.store, gui.HostForm{}, "", d.refreshHosts)
}

func (d *dashboard) editHost(name string) {
	cfg, err := d.store.Load()
	if err != nil {
		dialog.ShowError(err, d.hostsWin)
		return
	}
	h, ok := cfg.Host(name)
	if !ok {
		return
	}
	showHostDialog(d.hostsWin, d.store, gui.ToHostForm(name, h), name, d.refreshHosts)
}

func (d *dashboard) deleteHost(name string) {
	dialog.ShowConfirm("删除主机", "确定删除主机 "+name+" ？", func(ok bool) {
		if !ok {
			return
		}
		cfg, err := d.store.Load()
		if err != nil {
			dialog.ShowError(err, d.hostsWin)
			return
		}
		if err := cfg.RemoveHost(name); err != nil { // errors if still referenced
			dialog.ShowError(err, d.hostsWin)
			return
		}
		if err := d.store.Save(cfg); err != nil && !isReloadWarning(err) {
			dialog.ShowError(err, d.hostsWin)
			return
		}
		d.refreshHosts()
	}, d.hostsWin)
}
```

Add the small shared helper `isReloadWarning` to `internal/guiapp/window.go` (it formalizes the `errors.Is(err, gui.ErrReloadAfterSave)` check already inlined in `deleteTunnel`). Place it near `deleteTunnel`:

```go
// isReloadWarning reports whether err is the soft "saved but daemon reload
// failed" warning (config is on disk; not a real failure).
func isReloadWarning(err error) bool { return errors.Is(err, gui.ErrReloadAfterSave) }
```

(`errors` and `gui` are already imported in window.go.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/hostsmanager.go internal/guiapp/hostsmanager_test.go internal/guiapp/window.go
git commit -m "feat(guiapp): hosts manager window with add/edit/delete cards

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Test-connection result + host-key trust dialog

**Files:**
- Create: `internal/guiapp/testconndialog.go`
- Test: `internal/guiapp/testconndialog_test.go`

`showTestConnDialog` renders the `gui.TestConnResult`. On success with **no** new host keys, a plain "连接成功" info dialog. When ssh added host key(s) (accept-new already wrote `~/.ssh/known_hosts`), show the fingerprint(s) with **信任并保存** (keep) / **取消** (run `gui.RemoveKnownHostEntry` to undo). On failure, show the reason; if a host key was nonetheless recorded, offer the same trust/undo choice.

> If Task 6 used a temporary stub `showTestConnDialog`, delete it now and replace with the real implementation below.

- [ ] **Step 1: Write the failing test**

Create `internal/guiapp/testconndialog_test.go`:

```go
package guiapp

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestShowTestConnDialog_Success(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("t")
	// No host keys, OK => must not panic.
	showTestConnDialog(win, "entryA", gui.TestConnResult{OK: true})
}

func TestShowTestConnDialog_NewHostKey(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("t")
	showTestConnDialog(win, "entryA", gui.TestConnResult{
		OK: true,
		Fingerprints: []gui.HostKey{
			{Host: "198.51.100.7", Algo: "ED25519", Fingerprint: "SHA256:abc"},
		},
	})
}

func TestShowTestConnDialog_Failure(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("t")
	showTestConnDialog(win, "entryA", gui.TestConnResult{OK: false, Reason: "认证失败"})
}

func TestFingerprintLines(t *testing.T) {
	got := fingerprintLines([]gui.HostKey{
		{Host: "h1", Algo: "ED25519", Fingerprint: "SHA256:abc"},
		{Host: "h2", Algo: "RSA", Fingerprint: ""},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d: %v", len(got), got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run 'TestShowTestConnDialog|TestFingerprintLines' -v`
Expected: FAIL (compile error — `showTestConnDialog`, `fingerprintLines` undefined, unless a stub from Task 6 exists, in which case the signature/behaviour tests fail).

- [ ] **Step 3: Implement**

Create `internal/guiapp/testconndialog.go`:

```go
package guiapp

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// showTestConnDialog renders a TestConnResult. When ssh recorded new host key(s)
// (accept-new already wrote ~/.ssh/known_hosts), it offers 信任并保存 / 取消; 取消
// removes the just-added entry via ssh-keygen -R.
func showTestConnDialog(win fyne.Window, hostName string, res gui.TestConnResult) {
	if len(res.Fingerprints) > 0 {
		showHostKeyTrust(win, hostName, res)
		return
	}
	if res.OK {
		dialog.ShowInformation("连接成功", "已成功连接到 "+hostName+"。", win)
		return
	}
	dialog.ShowError(errReason(res.Reason), win)
}

func showHostKeyTrust(win fyne.Window, hostName string, res gui.TestConnResult) {
	head := "首次连接 " + hostName + "，对方主机密钥如下："
	if !res.OK {
		head = "连接未成功（" + res.Reason + "），但已记录到主机密钥："
	}
	lines := fingerprintLines(res.Fingerprints)
	body := container.NewVBox(widget.NewLabel(head))
	for _, l := range lines {
		lbl := widget.NewLabel(l)
		lbl.TextStyle = mono
		body.Add(lbl)
	}
	body.Add(widget.NewLabel("信任并保存：保留到 ~/.ssh/known_hosts。\n取消：撤销刚才添加的记录。"))

	dlg := dialog.NewCustomConfirm("主机密钥", "信任并保存", "取消", body, func(trust bool) {
		if trust {
			return // accept-new already persisted the key
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		go func() {
			defer cancel()
			_ = gui.RemoveKnownHostEntry(ctx, hostName, execRunner)
		}()
	}, win)
	dlg.Show()
}

// fingerprintLines formats each host key as "<host> <ALGO> <fingerprint>".
func fingerprintLines(keys []gui.HostKey) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		fp := k.Fingerprint
		if fp == "" {
			fp = "(未显示指纹)"
		}
		out = append(out, k.Host+"  "+k.Algo+"  "+fp)
	}
	return out
}

// errReason wraps a reason string as an error for dialog.ShowError.
func errReason(s string) error {
	if s == "" {
		s = "连接失败"
	}
	return reasonError(s)
}

type reasonError string

func (e reasonError) Error() string { return string(e) }
```

If a stub `showTestConnDialog` was added in Task 6, remove it from `hostdialog.go` now.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/testconndialog.go internal/guiapp/testconndialog_test.go internal/guiapp/hostdialog.go
git commit -m "feat(guiapp): test-connection result + host-key trust dialog

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Wire "主机…" into the toolbar, tray menu, and app handlers

**Files:**
- Modify: `internal/guiapp/window.go` (toolbar `globalZone`)
- Modify: `internal/guiapp/tray.go` (`Handlers.Hosts` + connected menu item)
- Modify: `internal/guiapp/app.go` (`handlers()` wiring)
- Test: `internal/guiapp/tray_test.go` (assert the menu item is present)

- [ ] **Step 1: Write the failing test**

If `internal/guiapp/tray_test.go` doesn't exist, create it; otherwise append. The test asserts the connected menu contains "主机…":

```go
package guiapp

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/gui"
)

func menuLabels(m *fyne.Menu) []string {
	var out []string
	for _, it := range m.Items {
		out = append(out, it.Label)
	}
	return out
}

func TestBuildMenu_HasHostsItemWhenConnected(t *testing.T) {
	model := gui.MenuModel{Connected: true, Summary: "ok"}
	m := buildMenu(model, Handlers{})
	found := false
	for _, lbl := range menuLabels(m) {
		if lbl == "主机…" {
			found = true
		}
	}
	if !found {
		t.Fatalf("connected menu missing 主机… item: %v", menuLabels(m))
	}
}

func TestBuildMenu_NoHostsItemWhenDisconnected(t *testing.T) {
	m := buildMenu(gui.MenuModel{Connected: false}, Handlers{})
	for _, lbl := range menuLabels(m) {
		if lbl == "主机…" {
			t.Fatal("disconnected menu should not show 主机…")
		}
	}
}
```

Add `"fyne.io/fyne/v2"` to the test imports (used by `menuLabels`). If `tray_test.go` already declares a `menuLabels` helper or imports `fyne`, reuse them instead of redeclaring.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guiapp/ -run TestBuildMenu_HasHostsItem -v`
Expected: FAIL (the connected menu has no "主机…" item yet).

- [ ] **Step 3: Implement**

In `internal/guiapp/tray.go`, add a `Hosts` field to `Handlers` (after `Open`):

```go
	Open         func()            // open the dashboard window
	Hosts        func()            // open the hosts manager window
```

In `buildMenu`'s connected branch, add the item right after "打开 Dashboard…":

```go
	items = append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("打开 Dashboard…", func() { call(h.Open) }),
		fyne.NewMenuItem("主机…", func() { call(h.Hosts) }),
		fyne.NewMenuItem("全部启动", func() { call(h.AllUp) }),
		fyne.NewMenuItem("全部停止", func() { call(h.AllDown) }),
		fyne.NewMenuItem("重载配置", func() { call(h.Reload) }),
		themeMenu(h),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("退出 hopd", func() { call(h.Quit) }),
	)
```

In `internal/guiapp/app.go`, wire the handler inside `handlers()` after `Open`:

```go
		Open:    func() { fyne.Do(u.dash.show) },
		Hosts:   func() { fyne.Do(u.dash.openHosts) },
```

In `internal/guiapp/window.go`, add a "主机…" button to the toolbar's `globalZone` in `buildToolbar`. Replace:

```go
	reload := widget.NewButtonWithIcon("重载", theme.ViewRefreshIcon(), func() { d.run(d.actionReload) })
	add := widget.NewButtonWithIcon("新增隧道", theme.ContentAddIcon(), d.addTunnel)
	add.Importance = widget.HighImportance
	globalZone := container.NewHBox(reload, add)
```

with:

```go
	hosts := widget.NewButtonWithIcon("主机…", theme.ComputerIcon(), d.openHosts)
	reload := widget.NewButtonWithIcon("重载", theme.ViewRefreshIcon(), func() { d.run(d.actionReload) })
	add := widget.NewButtonWithIcon("新增隧道", theme.ContentAddIcon(), d.addTunnel)
	add.Importance = widget.HighImportance
	globalZone := container.NewHBox(hosts, reload, add)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/guiapp/ -v`
Expected: PASS (existing `app_test.go` `TestNewUI_Wires` still green — the new handler is non-nil and `openHosts` is a no-op when `d.store`/`d.hostsWin` aren't shown).

- [ ] **Step 5: Commit**

```bash
git add internal/guiapp/window.go internal/guiapp/tray.go internal/guiapp/app.go internal/guiapp/tray_test.go
git commit -m "feat(guiapp): wire 主机… into toolbar, tray menu, and app handlers

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Full-suite verification

**Files:** none (verification only)

- [ ] **Step 1: Run the whole test suite**

Run: `go test ./...`
Expected: PASS across all packages. (Two pre-existing environment-flaky tests on this Mac — a socket-path-length test and a timing-sensitive test — may fail for env reasons unrelated to this change; confirm any failure matches those known cases before treating it as a regression.)

- [ ] **Step 2: Vet and build**

Run: `go vet ./... && go build ./...`
Expected: no errors. (If `go vet` flags the `var _ = config.Host{}` guard or `hostCheckOrder` as redundant once real references exist, remove the guard / confirm the reference.)

- [ ] **Step 3: Manual smoke (optional, documents the milestone)**

Launch the GUI with `HOPD_GUI_OPEN=1`, click "主机…", add a host pointing at a reachable bastion, click "测试连接", confirm the success or host-key dialog appears, then save and verify the host appears in the list and round-trips on re-open. Confirm `~/.ssh/config` is never written and the temp `-F` file under `os.TempDir()` is gone after the test.

- [ ] **Step 4: Commit (if any incidental fixes were needed)**

```bash
git add -A
git commit -m "test: full-suite verification for hosts manager + test-connection

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review (author checklist — already applied)

**Spec coverage (Plan 2 scope):**
- §7.1 Hosts manager (cards: name, host:port, user, jump chain; Add/Edit/Delete) → Tasks 5, 6, 7.
- §7.1 Host edit dialog fields (name, host, port, user, key file picker, jump dropdown, advanced ssh_options) → Task 5.
- §7.1 Live validation mirroring the tunnel-form pattern → Tasks 2 (`CheckHost` pure), 5 (live wiring).
- §7.4 Test-connection flow (shared package; generate temp `-F`; `ssh -F <tmp> -o BatchMode -o ConnectTimeout <host> true`; friendly stderr reason) → Tasks 4 (runner), 6 (trigger), 8 (result dialog).
- §7.5 Host-key trust UI (accept-new; show fingerprints; 信任并保存 / 取消 with undo) → Tasks 3 (parse), 4 (`RemoveKnownHostEntry`), 8 (dialog).
- §10 Security: temp `-F` config `0600` in a `0700` temp dir, removed via `defer os.RemoveAll` → Task 4; never write `~/.ssh/config` (only read for import — out of scope here) → no writes anywhere in this plan; `known_hosts` changes only via the explicit trust dialog (accept-new is shown + undoable) → Tasks 4, 8.
- Entry-point wiring ("主机…" in toolbar + tray + `Handlers.Hosts`) → Task 9.

**Cross-plan contract honoured:** `gui.HostForm`/`Parse`/`ToHostForm` (Task 1), `gui.CheckHost(f, otherNames, jumpTargets)` keyed name/host/port/jump/sshOptions (Task 2), `gui.CmdRunner`/`TestConnResult`/`HostKey`/`TestConnection`/`execRunner`/`RemoveKnownHostEntry` (Tasks 3, 4), `showHostDialog(win, store, initial, editingName, onDone)` (Task 6), `(*dashboard).openHosts()` (Task 7), `Handlers.Hosts` (Task 9), `showTestConnDialog` (Task 8). Names match the prompt exactly so Plans 3 & 4 can reference them.

**Deferred to later plans (intentionally out of scope):** simplified tunnel-dialog rewrite (§7.2), settings/defaults window (§7.3), import wizard UI (§7.6), legacy "迁移为主机" action (§6). These reuse `sshconf` (Plan 1) and the host form/validation/test runner defined here.

**Type consistency:** `gui.HostForm` fields (Name/Host/Port/User/KeyFile/Jump/SSHOptions) used identically in `hostform.go`, `hostformcheck.go`, `hostdialog.go`, `hostsmanager.go`. `config.Host` fields (Host/Port/User/Key/Jump/SSHOptions) from the locked Plan 1 API. `gui.TestConnResult.{OK,Reason,Fingerprints}` and `gui.HostKey.{Host,Algo,Fingerprint}` consumed identically in `testconndialog.go`. `ConfigStore.{Load,Save}` + `ErrReloadAfterSave` handled exactly like `window.go`'s `deleteTunnel`. Fyne chrome helpers (`text`, `roundRect`, `layoutStackV`, `layoutPadXY`, `newTappable`, `newCaption`, `captionLabel.set`, `section`, `sectionHeader`, `sectionDivider`, `saveLabel`, `noWheelTrap`, `itoa`, `pal`, `bold`, `mono`) reused from `card.go`/`editdialog.go` with matching signatures.

**Placeholder scan:** none — every code step contains complete code. The only conditional artifact is the optional temporary `showTestConnDialog` stub noted in Task 6 (deleted in Task 8), called out explicitly in both tasks.
