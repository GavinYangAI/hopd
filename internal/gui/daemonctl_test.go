package gui

import (
	"errors"
	"reflect"
	"testing"
)

func TestLocateHopd_PrefersPATH(t *testing.T) {
	got := locateHopdWith(
		func(string) (string, error) { return "/from/path/hopd", nil },
		func(string) bool { return true },
		[]string{"/usr/local/bin/hopd"},
	)
	if got != "/from/path/hopd" {
		t.Fatalf("got %q, want the PATH result", got)
	}
}

func TestLocateHopd_FallsBackToCandidate(t *testing.T) {
	got := locateHopdWith(
		func(string) (string, error) { return "", errors.New("not on PATH") },
		func(p string) bool { return p == "/opt/homebrew/bin/hopd" },
		[]string{"/usr/local/bin/hopd", "/opt/homebrew/bin/hopd"},
	)
	if got != "/opt/homebrew/bin/hopd" {
		t.Fatalf("got %q, want the existing candidate", got)
	}
}

func TestLocateHopd_NoneFound(t *testing.T) {
	got := locateHopdWith(
		func(string) (string, error) { return "", errors.New("nope") },
		func(string) bool { return false },
		[]string{"/usr/local/bin/hopd"},
	)
	if got != "" {
		t.Fatalf("got %q, want empty when nothing is found", got)
	}
}

func TestDaemonStartArgs_LaunchAgentPresent(t *testing.T) {
	cmd, args, err := daemonStartArgs(true, "501", "/usr/local/bin/hopd")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "launchctl" {
		t.Fatalf("cmd = %q, want launchctl", cmd)
	}
	want := []string{"kickstart", "-k", "gui/501/com.gavinyangai.hopd"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestDaemonStartArgs_FallbackToBinary(t *testing.T) {
	cmd, args, err := daemonStartArgs(false, "501", "/usr/local/bin/hopd")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "/usr/local/bin/hopd" || !reflect.DeepEqual(args, []string{"daemon"}) {
		t.Fatalf("cmd=%q args=%v, want /usr/local/bin/hopd [daemon]", cmd, args)
	}
}

func TestDaemonStartArgs_NoOption(t *testing.T) {
	if _, _, err := daemonStartArgs(false, "501", ""); err == nil {
		t.Fatal("expected error when no launch agent and no hopd binary on PATH")
	}
}
