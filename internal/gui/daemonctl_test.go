package gui

import (
	"reflect"
	"testing"
)

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
