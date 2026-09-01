package tunnel

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GavinYangAI/hopd/internal/config"
)

// writeFakeSSH writes an executable shell script and returns its path.
func writeFakeSSH(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ssh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func eventually(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", d)
}

func testTunnel() config.Tunnel {
	return config.Tunnel{Name: "t", Group: "g", Local: "15999", Remote: "h:80", Via: "bastion"}
}

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

func TestRunner_RetriesOnQuickExit(t *testing.T) {
	ssh := writeFakeSSH(t, "exit 1")
	r := NewRunner(testTunnel(), ssh, time.Millisecond, 5*time.Millisecond)
	r.SetProbe(func(string) bool { return false })
	r.Start()
	defer r.Stop()
	// Reaching two reconnects starts three child processes; loaded macOS runners
	// can spend most of the timeout in process startup despite millisecond backoff.
	eventually(t, 5*time.Second, func() bool { return r.Snapshot().Reconnects >= 2 })
}

func TestRunner_MarksUpWhenProbeSucceeds(t *testing.T) {
	ssh := writeFakeSSH(t, "sleep 30")
	r := NewRunner(testTunnel(), ssh, time.Millisecond, 5*time.Millisecond)
	r.SetProbe(func(string) bool { return true })
	r.Start()
	defer r.Stop()
	eventually(t, 2*time.Second, func() bool { return r.Snapshot().State == "UP" })
	if up := r.Snapshot().UptimeSec; up < 0 {
		t.Fatalf("uptime should be >= 0, got %d", up)
	}
}

func TestRunner_StopGoesDown(t *testing.T) {
	ssh := writeFakeSSH(t, "sleep 30")
	r := NewRunner(testTunnel(), ssh, time.Millisecond, 5*time.Millisecond)
	r.SetProbe(func(string) bool { return true })
	r.Start()
	eventually(t, 2*time.Second, func() bool { return r.Snapshot().State == "UP" })
	r.Stop()
	if got := r.Snapshot().State; got != "DOWN" {
		t.Fatalf("after Stop, state = %q, want DOWN", got)
	}
}

func TestRunner_FatalForwardErrorStopsRetrying(t *testing.T) {
	// ssh that reports a local-bind failure on stderr and exits non-zero.
	ssh := writeFakeSSH(t, "echo 'bind: Address already in use' 1>&2; exit 1")
	r := NewRunner(testTunnel(), ssh, time.Millisecond, 5*time.Millisecond)
	r.SetProbe(func(string) bool { return false })
	r.Start()
	defer r.Stop()

	eventually(t, 2*time.Second, func() bool { return r.Snapshot().State == "ERROR" })

	// It must stop retrying once ERROR is reached (no growing reconnect count).
	n1 := r.Snapshot().Reconnects
	time.Sleep(60 * time.Millisecond)
	if n2 := r.Snapshot().Reconnects; n2 > n1 {
		t.Fatalf("fatal error should halt retries; reconnects grew %d -> %d", n1, n2)
	}
}

func TestSnapshotCarriesViaHost(t *testing.T) {
	tun := config.Tunnel{Name: "pg", Local: "5432", Remote: "10.0.1.5:5432", ViaHost: "entryA"}
	r := NewRunner(tun, "/usr/bin/ssh", time.Second, time.Minute)
	if got := r.Snapshot().ViaHost; got != "entryA" {
		t.Fatalf("Snapshot().ViaHost = %q, want entryA", got)
	}
}
