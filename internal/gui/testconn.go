package gui

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

// HostKey is one host-key fingerprint surfaced by a test connection, so the
// trust dialog can show what was added to ~/.ssh/known_hosts.
type HostKey struct {
	Host        string // hostname / IP as ssh names it
	Algo        string // ED25519 / RSA / ECDSA …
	Fingerprint string // SHA256:… ("" if ssh didn't print one)
}

// parseHostKeys extracts host-key info from ssh stderr produced under
// StrictHostKeyChecking=accept-new. It pairs each
//
//	Warning: Permanently added '<host>' (<ALGO>) to the list of known hosts.
//
// line with a following
//
//	<ALGO> key fingerprint is SHA256:…
//
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
