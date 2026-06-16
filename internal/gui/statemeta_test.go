package gui

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

func TestStateInfo(t *testing.T) {
	if m := StateInfo("UP"); m.Label != "已连通" || m.Tone != ToneUp {
		t.Fatalf("UP meta = %+v", m)
	}
	if m := StateInfo("RETRYING"); m.Tone != ToneWarn || !m.Busy {
		t.Fatalf("RETRYING should be a busy warn state: %+v", m)
	}
	if m := StateInfo("NEEDS_AUTH"); m.Tone != ToneWarn || m.Busy {
		t.Fatalf("NEEDS_AUTH should be warn but not busy: %+v", m)
	}
	if m := StateInfo("ERROR"); m.Tone != ToneErr {
		t.Fatalf("ERROR tone = %v", m.Tone)
	}
	if m := StateInfo("???"); m.Tone != ToneDown {
		t.Fatalf("unknown state should fall back to DOWN: %+v", m)
	}
}

func TestSummarize(t *testing.T) {
	got := Summarize([]ipc.TunnelStatus{
		{State: "UP"}, {State: "UP"}, {State: "RETRYING"}, {State: "ERROR"}, {State: "DOWN"},
	})
	want := "2 已连通 · 1 连接中 · 1 出错 · 1 已停止"
	if got != want {
		t.Fatalf("Summarize = %q, want %q", got, want)
	}
	if s := Summarize(nil); s != "暂无隧道" {
		t.Fatalf("empty summarize = %q", s)
	}
}

func TestRouteOf(t *testing.T) {
	if r := RouteOf(TunnelForm{Via: "bastion"}); r != RouteRelay {
		t.Fatalf("via form should be relay, got %q", r)
	}
	if r := RouteOf(TunnelForm{JumpHost: "jump"}); r != RouteDirect {
		t.Fatalf("jump form should be direct, got %q", r)
	}
	if r := RouteOf(TunnelForm{RawJump: []string{"a@h:22"}}); r != RouteDirect {
		t.Fatalf("raw-jump form should be direct, got %q", r)
	}
	if r := RouteOf(TunnelForm{}); r != RouteViaHost {
		t.Fatalf("blank form should default to the host model, got %q", r)
	}
}

func TestCheckRoute(t *testing.T) {
	base := TunnelForm{Name: "win-1", LocalPort: "13389", DestHost: "10.0.0.1", DestPort: "3389"}

	// Relay needs an alias.
	errs, _ := CheckRoute(RouteRelay, base)
	if errs["via"] == "" {
		t.Fatal("relay without via should error")
	}
	errs, _ = CheckRoute(RouteRelay, withVia(base, "bastion"))
	if len(errs) != 0 {
		t.Fatalf("relay with via should be clean, got %v", errs)
	}

	// Direct with no jump host is valid (ssh straight to the target).
	errs, _ = CheckRoute(RouteDirect, base)
	if len(errs) != 0 {
		t.Fatalf("direct with no jump should be clean, got %v", errs)
	}

	// Unselected route is an error.
	if errs, _ = CheckRoute("", base); errs["route"] == "" {
		t.Fatal("unselected route should error")
	}

	// Name with a space.
	bad := base
	bad.Name = "win 1"
	if errs, _ = CheckRoute(RouteDirect, bad); errs["name"] == "" {
		t.Fatal("name with space should error")
	}

	// Multi-hop relay chain warns but does not block.
	_, warns := CheckRoute(RouteRelay, withVia(base, "a,b"))
	if warns["via"] == "" {
		t.Fatal("comma via should warn")
	}
}

func withVia(f TunnelForm, via string) TunnelForm { f.Via = via; return f }
