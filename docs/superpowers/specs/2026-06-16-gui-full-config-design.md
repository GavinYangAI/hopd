# Design: Full GUI Configuration for hopd

**Date:** 2026-06-16
**Status:** Approved (design), pending implementation plan
**Goal:** Make hopd fully configurable through the menu-bar GUI so a user never hand-edits
`~/.config/hopd/config.yaml` *or* `~/.ssh/config`. Every SSH connection detail (host, port,
user, key) for every hop becomes a first-class GUI field.

---

## 1. Background & Problem

hopd today already has a guided "新增/编辑隧道" form that writes `config.yaml` itself (atomic
write, `.bak` backup, daemon reload via IPC). So hand-editing **config.yaml is mostly solved**.

The remaining gap is **`~/.ssh/config`**:

- The "Relay (via)" route only takes an *alias string* and instructs the user to define that
  `Host` in `~/.ssh/config` by hand.
- Only *jump* hops expose port/user/key fields. The **final SSH endpoint** cannot be given its
  own port/user/key in the GUI.

Concrete failing scenario (the one that motivated this): connect to PostgreSQL on host A's
internal IP `:5432`, where A's SSH endpoint is `A_public:65522`, reachable only through bastion
B. Expressing "ssh to A:65522 via B, then forward to A_internal:5432" requires a hand-written
`~/.ssh/config` alias today, because the SSH endpoint (A:65522) differs from the forward target
(A_internal:5432) and the endpoint's port/user can't be entered in the GUI.

## 2. Resolved Decisions

| # | Decision | Choice |
|---|----------|--------|
| 1 | Where connection info lives / how it reaches ssh | **Self-contained / inline** — stored in hopd's own `config.yaml`, never read/write user's `~/.ssh/config` |
| 2 | Data model | **Reusable `hosts:` entities** referenced by tunnels via `via_host` |
| 3 | SSH rendering mechanism | hopd generates an **ephemeral `ssh -F` config** it owns; not fragile `-p/-J/-o` argv |
| 4 | `host.jump` shape | **Single reference**, chains formed by linking host→host (no parallel jumps) |
| 5 | Legacy migration | Legacy `via`/`jump` tunnels keep running unchanged; **opt-in** "迁移为主机" button + dismissible one-time hint banner |
| 6 | v1 scope | Test-connection, host-key trust UI, import `~/.ssh/config`, global defaults editor — all in v1 |

### Non-goals (v1)

- No storing of secret key *material* — only key file *paths* (preserves SECURITY.md posture).
- No parallel/fan-out jump topologies.
- No editing of the user's real `~/.ssh/config` (we only *read* it for the one-time import).
- No batch/group bulk operations beyond what exists.

## 3. Config Schema

New top-level `hosts:` section. Hosts link via `jump`. Tunnels reference an entry host via
`via_host`.

```yaml
defaults:
  restart: {min: 2s, max: 60s}
  ssh_options: {ServerAliveInterval: "15"}

hosts:
  bastionB:
    host: B_public_ip
    port: 22
    user: userB
    key: ~/.ssh/idB        # optional; empty => ssh-agent / ssh defaults
    jump: ""               # optional; name of another host entry
  entryA:
    host: A_public_ip
    port: 65522
    user: userA
    key: ~/.ssh/idA
    jump: bastionB         # chain: entryA -> bastionB

groups:
  prod:
    - name: pg
      via_host: entryA     # NEW: reference into hosts:
      remote: 10.0.1.5:5432
      local: 5432
      autostart: true
    - name: redis
      via_host: entryA     # reuse the same entry, no re-entry of B/A
      remote: 10.0.1.5:6379
      local: 6379
```

### Go types (`internal/config`)

```go
type Host struct {
    Host       string            // hostname / IP (HostName)
    Port       int               // default 22
    User       string
    Key        string            // IdentityFile path; "" => agent/defaults
    Jump       string            // name of another Host entry; "" => none
    SSHOptions map[string]string // per-host extra -o options
}

type Config struct {
    // ... existing ...
    Hosts map[string]Host // NEW
}

type Tunnel struct {
    // ... existing: Name, Group, Local, Remote, Via, Jump, SSHOptions, Autostart ...
    ViaHost string // NEW: name of a Host entry
}
```

Legacy `Tunnel.Via` (ssh_config alias) and `Tunnel.Jump` (inline chain) are retained for
back-compat.

## 4. Validation Rules (extend existing validate)

- A tunnel must have exactly one of: `via_host` (new) **or** `via`/`jump` (legacy). Error if
  both `via_host` and legacy fields are set on the same tunnel.
- `via_host` must reference an existing key in `hosts:`.
- `host.jump`, if non-empty, must reference an existing host.
- Host jump chains must be **acyclic** (detect cycles; report the offending cycle).
- `host.port` in 1–65535 (default 22 when omitted).
- Host names unique (map keys inherently unique) and non-empty.
- Existing rules unchanged: unique tunnel names, unique local binds, remote `host:port`, port
  ranges, restart bounds.

## 5. SSH Rendering: `ssh -F` Generated Config

Instead of flattening hops into `-p/-J/-o` argv (which cannot express a distinct IdentityFile
per jump hop), hopd **generates an ssh config file it fully owns** and points ssh at it with
`-F`.

### Generated artifact

- Location: `~/.config/hopd/generated/` (new dir; mode 0700). One file per tunnel, e.g.
  `pg.sshcfg` (mode 0600).
- Regenerated from `config.yaml` on every daemon start / reload — a **pure derived artifact,
  never hand-edited**.
- Example for the motivating scenario:

```sshconfig
# Generated by hopd — do not edit. Source: ~/.config/hopd/config.yaml
Host entryA
    HostName A_public_ip
    Port 65522
    User userA
    IdentityFile ~/.ssh/idA
    IdentitiesOnly yes
    ProxyJump bastionB
Host bastionB
    HostName B_public_ip
    Port 22
    User userB
    IdentityFile ~/.ssh/idB
    IdentitiesOnly yes
```

ssh invocation:

```
ssh -F ~/.config/hopd/generated/pg.sshcfg -N -T \
    -L 127.0.0.1:5432:10.0.1.5:5432 entryA
```

### Rendering rules

- `IdentitiesOnly yes` is emitted only when a `key` is set (forces exactly that key);
  omitted otherwise so ssh-agent / defaults still work.
- `known_hosts` is left at the default `~/.ssh/known_hosts` (shared, persistent trust). We do
  **not** override `UserKnownHostsFile`.
- hopd's default injected options (`ControlMaster`/`ControlPersist`, `ExitOnForwardFailure=yes`)
  and `defaults.ssh_options` are written into the entry Host block (or all blocks where
  appropriate). Per-host and per-tunnel `ssh_options` override defaults.
- ssh-agent continues to work (env `SSH_AUTH_SOCK` is inherited).

### Dual rendering path (back-compat)

- Tunnel has `via_host` set → render via the generated `-F` config (new path).
- Tunnel uses legacy `via`/`jump` (no `via_host`) → **existing argv path is untouched** (still
  resolves the user's real `~/.ssh/config`). Avoids `-F` shadowing legacy aliases.

## 6. Migration & Back-Compat

- Existing configs load and run unchanged (legacy path). No forced rewrite.
- A legacy tunnel shows an **opt-in** "迁移为主机" action:
  - For `via: <alias>`: reuse the ssh_config import parser to pull that alias's
    HostName/Port/User/IdentityFile/ProxyJump into a new `hosts:` entry, then rewrite the tunnel
    to `via_host`.
  - For inline `jump`: extract the chain + endpoint into host entries and rewrite to `via_host`.
- A dismissible one-time hint banner points users with legacy tunnels at the migrate action.
- `config.Marshal` writes the `hosts:` section and preserves legacy fields on tunnels not yet
  migrated. Existing "only write ssh_options differing from defaults" behavior preserved.

## 7. GUI Surfaces (Fyne)

### 7.1 Hosts manager (new)

- New "主机" section in the dashboard window: cards listing saved hosts (name, host:port, user,
  jump chain), with Add / Edit / Delete.
- Host edit dialog fields: name, host, port, user, key (file picker), jump (dropdown of other
  hosts), advanced `ssh_options` (multiline key=value). Live validation mirroring the existing
  tunnel-form pattern (`internal/gui/form.go`, `formcheck.go`).
- "测试连接" button (see 7.4).

### 7.2 Tunnel dialog (simplified rewrite)

- Replace Direct/Relay radio + inline jump fields with:
  - `via_host` picker: dropdown of saved hosts, plus "+ 新建主机" to create one inline.
  - Forward target `remote` (host:port), local port, autostart, advanced ssh_options.
- Editing a **legacy** tunnel: show its legacy fields read-mostly with the "迁移为主机" action.
- "测试连接" button validates the full chain (via the chosen host) end to end.

### 7.3 Settings window (new)

- Edit `defaults.restart` (min/max duration strings) and global `defaults.ssh_options`.
- Save through the existing `ConfigStore.Save` flow (atomic + `.bak` + reload).

### 7.4 Test-connection flow

- Runs in a shared package usable by both GUI and daemon. The GUI shells out directly (short
  task, no new IPC needed): generate a temp `-F` config for the host/chain, then
  `ssh -F <tmp> -o BatchMode=... -o ConnectTimeout=... <host> true`.
- Reports success / failure with the captured ssh stderr reason (wrong port, auth failure,
  unreachable jump, etc.).
- On an unknown host key, hand off to the host-key trust dialog (7.5) instead of hanging.

### 7.5 Host-key trust UI

- GUI has no TTY, so `StrictHostKeyChecking=ask` would hang. v1 approach: run test with
  `StrictHostKeyChecking=accept-new` and, before/after, surface the recorded fingerprint(s) in a
  dialog ("信任并保存" / "取消"). Accepting persists to `~/.ssh/known_hosts`; cancel removes the
  just-added entry. (Jump-hop key display behind further jumps is best-effort; refined in plan.)

### 7.6 Import wizard

- Parse `~/.ssh/config`, list discovered `Host` aliases (HostName/Port/User/IdentityFile/
  ProxyJump) with checkboxes. Import selected aliases as `hosts:` entries (ProxyJump mapped to
  `jump` references where the referenced alias is also imported).
- One-time helper; the user's `~/.ssh/config` is only **read**, never modified.

## 8. Package-by-Package Changes

- `internal/config`: add `Host`, `Config.Hosts`, `Tunnel.ViaHost`; validation (ref integrity +
  cycle detection); marshal/unmarshal; `AddHost`/`UpdateHost`/`RemoveHost`.
- `internal/sshconf` (new): generate `-F` config text from hosts+tunnel; resolve jump chains;
  parse `~/.ssh/config` for the import wizard. Pure, no Fyne deps; unit-testable.
- `internal/tunnel/argv.go`: `via_host` branch builds `ssh -F <file> ... <hostname>`; legacy
  path unchanged.
- `internal/daemon/manager.go`: write the generated `-F` file before launching each tunnel;
  clean up on stop/reload.
- `internal/gui`: host form model + validation, test-host runner, import parser glue, defaults
  form model. (No Fyne deps — mirrors existing split.)
- `internal/guiapp`: hosts manager UI, rewritten tunnel dialog, settings window, import dialog,
  test/host-key dialogs.

No new IPC command required for v1 (test runs GUI-side via the shared `sshconf` package). The
generation logic lives in `sshconf` and is shared by daemon (live tunnels) and GUI (test).

## 9. Testing Strategy

- `config`: schema round-trip (marshal/unmarshal), host-ref validation, cycle detection, legacy
  + new coexistence, migration transforms.
- `sshconf`: golden tests for generated config text; chain resolution; `-F` argv assembly;
  ssh_config import parser (including ProxyJump → jump mapping).
- `tunnel`: existing argv tests for the legacy path stay green; new tests for the `via_host`
  `-F` argv.
- `gui`: host form validation; test-host runner against a mock ssh (injectable runner); import
  parser.

## 10. Security Considerations

- hopd still stores **no credentials** — only key file *paths* and host metadata, consistent
  with SECURITY.md.
- Generated `-F` configs live under `~/.config/hopd/generated/` with `0700` dir / `0600` files.
- The user's `~/.ssh/config` is never written; only read during the explicit import action.
- `known_hosts` trust remains in the standard `~/.ssh/known_hosts`; the trust UI makes additions
  explicit and user-confirmed.

## 11. Deferred (post-v1)

- Parallel/fan-out jump topologies.
- Bulk group operations.
- Round-tripping comments/formatting in the user's `~/.ssh/config` (we never write it).
- Optional `ssh -F` `Include ~/.ssh/config` bridging for hybrid setups.
