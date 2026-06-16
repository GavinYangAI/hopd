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

func TestCheckRoute_ViaHost_RequiresChosenHost(t *testing.T) {
	f := TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432"} // no ViaHost
	errs, _ := CheckRoute(RouteViaHost, f)
	if errs["viaHost"] == "" {
		t.Fatalf("expected a viaHost error when no host chosen, got %v", errs)
	}
}

func TestCheckRoute_ViaHost_OK(t *testing.T) {
	f := TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432", ViaHost: "entryA"}
	errs, _ := CheckRoute(RouteViaHost, f)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for a complete via_host form, got %v", errs)
	}
}

func TestCheckRoute_ViaHost_StillChecksCommonFields(t *testing.T) {
	// A chosen host doesn't excuse missing name/localPort/destHost/destPort.
	f := TunnelForm{ViaHost: "entryA"}
	errs, _ := CheckRoute(RouteViaHost, f)
	for _, key := range []string{"name", "localPort", "destHost", "destPort"} {
		if errs[key] == "" {
			t.Fatalf("expected error on %q for an otherwise-empty via_host form, got %v", key, errs)
		}
	}
}
