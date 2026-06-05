# hopd

[English](README.md) · **中文**

一个常驻的 SSH 端口转发守护进程。把「本地端口 → 经 SSH 跳板/中继 → 内网服务」的转发**配置一次**,
`hopd` 就在后台帮你托管——按需启停、断线自动重连、还有一个 k9s 风格的实时面板——再也不用同时开一堆
`ssh -L` 终端窗口。

它直接调用系统的 `ssh`,所以会复用你现有的 `~/.ssh/config`、ssh-agent、`known_hosts` 和 2FA。
支持任意深度的跳板链:一条转发既可以只跨一跳,也可以走完整的 `ProxyJump` 链。

## 安装

```sh
go build -o hopd ./cmd/hopd
# 放到 PATH 里,例如
install hopd /usr/local/bin/hopd
```

开机自启(macOS / launchd):

```sh
hopd install     # 写入 ~/Library/LaunchAgents/com.gavinyangai.hopd.plist 并加载
hopd uninstall   # 停止并移除
```

## 配置

`~/.config/hopd/config.yaml`(遵循 `$XDG_CONFIG_HOME`):

```yaml
defaults:
  ssh_options:                 # 作为 -o Key=Value 注入每条隧道
    ServerAliveInterval: 15
    ServerAliveCountMax: 3
  restart: { min: 2s, max: 60s }   # 重连退避的上下界

groups:
  prod:
    - name: prod-db
      local: 5432                # 127.0.0.1:5432(只写端口则绑定 127.0.0.1)
      remote: 10.0.1.5:5432      # 最终目标 host:port,经跳板到达
      via: prod-bastion          # ~/.ssh/config 里的一个 Host 别名
      autostart: true            # 守护进程启动时自动连接这条隧道
    - name: prod-redis
      local: 6379
      remote: 10.0.1.6:6379
      via: prod-bastion
  staging:
    - name: stg-web
      local: 127.0.0.1:8080
      remote: 127.0.0.1:80
      jump: [user@jump1, user@jump2]   # 内联 -J 跳板链,不需要写 ssh_config
      ssh_options: { ConnectTimeout: 5 }   # 单条隧道的覆盖项
```

- **`via`** —— `~/.ssh/config` 里的 Host 别名。想「连到跳板后面的某台机器再转发」,就让 `via` 指向一个
  已经配好 `ProxyJump` 的别名。
- **`jump`** —— 内联的 `ProxyJump` 跳板链,可与 `via` 组合使用。
- **`remote`** —— 最终目标的 `host:port`,相对于最后一跳所在的网络。
- **`autostart`** —— 守护进程启动时自动把这条隧道拉起来,这样**重启电脑后会自动重连**(配合
  `hopd install` 装的 launchd agent)。默认关闭;把你总要用的隧道标上即可。需要 2FA 的目标会停在
  `NEEDS_AUTH`,跑一次 `hopd auth <name>` 即可。

hopd 默认还会注入 `ControlMaster`/`ControlPersist` 与 `ExitOnForwardFailure=yes`
(可在 `ssh_options` 里按隧道覆盖)。

## 使用

守护进程统管一切。标了 `autostart` 的隧道会随守护进程启动自动连接(所以重启后会自动重连);
其余隧道默认是**停止**状态,你按需把要用的拉起来。

```sh
hopd daemon            # 前台运行supervisor(launchd 用的就是它)

hopd up [name|group|all]    # 启动隧道(默认 all)
hopd down [name|group|all]  # 停止隧道
hopd status   (别名: ls)    # 状态表
hopd reload                 # 从磁盘重载配置
hopd logs <name>            # 某条隧道的 ssh stderr 尾部
hopd auth <name>            # 交互登录(如 2FA),预热 ControlMaster
hopd tui                    # 面板(等同于直接敲 `hopd`)
hopd version
```

### TUI 快捷键

`s` 启动 · `x` 停止 · `r` 重启 · `R` 重载配置 · `a` 全部启动 · `A` 认证(2FA) ·
`enter` 查看日志 · `q` 退出。行按状态着色
(`UP` 绿、`STARTING`/`RETRYING` 黄、`NEEDS_AUTH` 橙、`ERROR` 红)。

### 需要 2FA 的目标

目标需要交互式验证码的隧道会显示为 `NEEDS_AUTH`。在真实终端上跑一次 `hopd auth <name>`
(或在 TUI 里按 `A`)完成认证;之后常驻的 `ControlMaster` 会让后台隧道无需再次输入即可重连。

> 限制:多路复用只覆盖最后一跳。如果**跳板机本身**要 2FA,每个 `ProxyJump` 仍会各自重新认证。

## 菜单栏 GUI(macOS)

`hopd-gui` 是守护进程的菜单栏客户端。图标颜色表示整体健康度
(绿=全部正常、红=出错/待认证、灰=混合或 daemon 未运行)。点某条隧道即可切换启停;
打开 Dashboard 可看状态卡片和每条隧道的日志。

```sh
go build -o hopd-gui ./cmd/hopd-gui   # 或:make gui
./hopd-gui                            # 在菜单栏运行
```

打包成 `.app`(只常驻菜单栏,不占 Dock):

```sh
go install fyne.io/fyne/v2/cmd/fyne@v2.7.4
make gui-package                      # 生成 hopd-gui.app
```

把 `hopd-gui.app` 加到 **系统设置 → 通用 → 登录项** 即可开机自启。GUI 是个轻客户端:
它控制的是与 CLI/TUI 同一个后台守护进程,所以退出 GUI 不会停掉你的隧道。若 daemon 没在运行,
菜单里会出现 **启动 daemon**(优先用 `hopd install` 装的 launchd agent,否则拉起 `hopd daemon`)
和 **安装并开机自启**(装好 launchd agent,让 daemon 开机自动启动)。从 Finder 启动时 GUI 继承的
PATH 很精简,所以它除了 PATH 还会自动到 `/usr/local/bin`、Homebrew、`~/go/bin` 等位置找 `hopd`。

需要 2FA 的隧道显示为 `⚠ …(auth)`;在终端里跑 `hopd auth <name>` 完成认证
(GUI 本身不弹验证码输入框)。

## 架构

一个二进制、三种角色——后台守护进程(每条隧道一个 `ssh -N` 子进程并做监管)、CLI 客户端、tview TUI——
通过 `~/.config/hopd/hopd.sock` 上的 JSON-Lines 协议通信。菜单栏 GUI(`hopd-gui`,基于 Fyne)
是第四个、可选的客户端,驱动的是同一个守护进程。

## 贡献

欢迎提 Issue 和 PR。提交 PR 前请先跑 `gofmt`、`go vet ./...`、`go test ./...`(CI 跑的也是这些)。
安全问题请见 [SECURITY.md](SECURITY.md)。

## 许可证

[MIT](LICENSE) © 2026 Gavin Yang (GavinYangAI)
