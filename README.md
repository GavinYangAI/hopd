# hopd

**English** · [中文](README.zh-CN.md)

A resident SSH port-forwarding daemon. Define your internal-service forwards once, and `hopd`
keeps them managed in the background — start/stop on demand, auto-reconnect, a live k9s-style
dashboard — instead of juggling a dozen `ssh -L` terminal windows.

It wraps the system `ssh` binary, so it reuses your existing `~/.ssh/config`, ssh-agent,
`known_hosts`, and 2FA. Mixed-depth jump chains work: a forward can land one hop away or go
through a `ProxyJump` chain.

## Install

```sh
go build -o hopd ./cmd/hopd
# put it on your PATH, e.g.
install hopd /usr/local/bin/hopd
```

Autostart on login (macOS / launchd):

```sh
hopd install     # writes ~/Library/LaunchAgents/com.gavinyangai.hopd.plist and loads it
hopd uninstall   # stop and remove it
```

## Configure

`~/.config/hopd/config.yaml` (honors `$XDG_CONFIG_HOME`):

```yaml
defaults:
  ssh_options:                 # injected as -o Key=Value into every tunnel
    ServerAliveInterval: 15
    ServerAliveCountMax: 3
  restart: { min: 2s, max: 60s }   # reconnect backoff bounds

groups:
  prod:
    - name: prod-db
      local: 5432                # 127.0.0.1:5432 (bare port binds 127.0.0.1)
      remote: 10.0.1.5:5432      # final target:port, reached via the jump
      via: prod-bastion          # a Host alias from ~/.ssh/config
      autostart: true            # bring this tunnel up when the daemon starts
    - name: prod-redis
      local: 6379
      remote: 10.0.1.6:6379
      via: prod-bastion
  staging:
    - name: stg-web
      local: 127.0.0.1:8080
      remote: 127.0.0.1:80
      jump: [user@jump1, user@jump2]   # inline -J chain, no ssh_config needed
      ssh_options: { ConnectTimeout: 5 }   # per-tunnel overrides
```

- **`via`** — an `~/.ssh/config` Host alias. To "connect to a machine behind the jump, then
  forward," point `via` at an alias whose `ProxyJump` is already configured.
- **`jump`** — an inline `ProxyJump` chain. Can be combined with `via`.
- **`remote`** — `host:port` of the final target, relative to the last hop's network.
- **`autostart`** — bring this tunnel up automatically when the daemon starts, so it reconnects
  after a reboot (paired with the launchd agent from `hopd install`). Off by default; mark the
  tunnels you always want connected. Targets needing 2FA settle into `NEEDS_AUTH` until you run
  `hopd auth <name>`.

hopd also injects `ControlMaster`/`ControlPersist` and `ExitOnForwardFailure=yes` by default
(override per-tunnel via `ssh_options`).

## Use

The daemon supervises everything. Tunnels marked `autostart` come up when the daemon starts
(so they reconnect after a reboot); the rest start **down** and you bring up what you need.

```sh
hopd daemon            # run the supervisor in the foreground (launchd uses this)

hopd up [name|group|all]    # start tunnels (default: all)
hopd down [name|group|all]  # stop tunnels
hopd status   (alias: ls)   # status table
hopd reload                 # reload config from disk
hopd logs <name>            # ssh stderr tail for a tunnel
hopd auth <name>            # interactive login (e.g. 2FA), pre-warms the ControlMaster
hopd tui                    # dashboard (same as bare `hopd`)
hopd version
```

### TUI keys

`s` start · `x` stop · `r` restart · `R` reload config · `a` start all · `A` authenticate
(2FA) · `enter` view logs · `q` quit. Rows are colored by state
(`UP` green, `STARTING`/`RETRYING` yellow, `NEEDS_AUTH` orange, `ERROR` red).

### 2FA targets

A tunnel whose target needs an interactive code shows as `NEEDS_AUTH`. Run `hopd auth <name>`
(or press `A` in the TUI) to authenticate once on a real terminal; the persistent
`ControlMaster` then lets the background tunnel reconnect without prompting.

> Limitation: multiplexing covers the final hop. If a *jump host itself* requires 2FA, each
> `ProxyJump` still re-authenticates.

## Menu-bar GUI (macOS)

`hopd-gui` is a menu-bar client for the daemon. The icon shows overall health
(green = all up, red = error/auth needed, grey = mixed or daemon down). Click a
tunnel to toggle it; open the dashboard for a status table and per-tunnel logs.

```sh
go build -o hopd-gui ./cmd/hopd-gui   # or: make gui
./hopd-gui                            # runs in the menu bar
```

Package it as a `.app` (lives only in the menu bar, no Dock icon):

```sh
go install fyne.io/fyne/v2/cmd/fyne@v2.7.4
make gui-package                      # produces hopd-gui.app
```

Add `hopd-gui.app` to **System Settings → General → Login Items** to start it at
login. The GUI is a thin client: it controls the same background daemon as the
CLI/TUI, so quitting the GUI leaves your tunnels running. If the daemon isn't
running, the menu offers **Start daemon** (uses the launchd agent from
`hopd install` if present, otherwise launches `hopd daemon`) and **Install &
autostart** (`安装并开机自启`), which installs the launchd agent so the daemon
starts at login. The GUI finds the `hopd` binary even when launched from Finder
with a minimal PATH — it also checks `/usr/local/bin`, Homebrew, and `~/go/bin`.

A tunnel needing 2FA shows as `⚠ … (auth)`; run `hopd auth <name>` in a terminal
to authenticate it (the GUI does not prompt for codes itself).

## Architecture

One binary, three roles — a background daemon supervising one `ssh -N` child per tunnel, a CLI
client, and a tview TUI — talking JSON-Lines over a Unix socket at `~/.config/hopd/hopd.sock`.
The menu-bar GUI (`hopd-gui`, built with Fyne) is a fourth, optional client driving the same
daemon.

## Contributing

Issues and pull requests are welcome. Please run `gofmt`, `go vet ./...`, and
`go test ./...` before opening a PR (CI runs the same). For security issues, see
[SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © 2026 Gavin Yang (GavinYangAI)
