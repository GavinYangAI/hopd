package gui

import "testing"

func TestRouteOf_ViaHost(t *testing.T) {
	if got := RouteOf(TunnelForm{ViaHost: "entryA"}); got != RouteViaHost {
		t.Fatalf("via_host form route = %q, want %q", got, RouteViaHost)
	}
}

func TestRouteOf_LegacyStillWorks(t *testing.T) {
	if got := RouteOf(TunnelForm{Via: "bastion"}); got != RouteRelay {
		t.Fatalf("via form route = %q, want %q", got, RouteRelay)
	}
	if got := RouteOf(TunnelForm{JumpHost: "j"}); got != RouteDirect {
		t.Fatalf("jump form route = %q, want %q", got, RouteDirect)
	}
	if got := RouteOf(TunnelForm{RawJump: []string{"a@h:22"}}); got != RouteDirect {
		t.Fatalf("raw jump form route = %q, want %q", got, RouteDirect)
	}
}

func TestRouteOf_NewBlankDefaultsToViaHost(t *testing.T) {
	if got := RouteOf(TunnelForm{}); got != RouteViaHost {
		t.Fatalf("blank form route = %q, want %q (new tunnels default to host model)", got, RouteViaHost)
	}
}
