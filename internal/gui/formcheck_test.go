package gui

import "testing"

func TestCheck_OK(t *testing.T) {
	f := TunnelForm{
		Name: "a", LocalPort: "13389", DestHost: "h", DestPort: "3389",
		JumpUser: "root", JumpHost: "j", JumpPort: "65532",
	}
	if errs := Check(f); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestCheck_OKWithVia(t *testing.T) {
	f := TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", Via: "bastion"}
	if errs := Check(f); len(errs) != 0 {
		t.Fatalf("via-only should be valid, got %v", errs)
	}
}

func TestCheck_NameRequired(t *testing.T) {
	f := TunnelForm{LocalPort: "1", DestHost: "h", DestPort: "2", JumpHost: "j", JumpUser: "u"}
	if Check(f)["name"] == "" {
		t.Fatal("empty name should report a name error")
	}
}

func TestCheck_PortRanges(t *testing.T) {
	base := func() TunnelForm {
		return TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", JumpHost: "j", JumpUser: "u", JumpPort: "22"}
	}
	bad := base()
	bad.LocalPort = "70000"
	if Check(bad)["localPort"] == "" {
		t.Fatal("out-of-range local port should error")
	}
	bad = base()
	bad.DestPort = "0"
	if Check(bad)["destPort"] == "" {
		t.Fatal("zero dest port should error")
	}
	bad = base()
	bad.DestPort = ""
	if Check(bad)["destPort"] == "" {
		t.Fatal("empty dest port should error")
	}
}

func TestCheck_JumpPortNoDashP(t *testing.T) {
	f := TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", JumpHost: "j", JumpUser: "u", JumpPort: "-p65532"}
	msg := Check(f)["jumpPort"]
	if msg == "" {
		t.Fatal("jump port with -p should error")
	}
}

func TestCheck_JumpPortEmptyOK(t *testing.T) {
	// Empty jump port is fine (defaults to 22 on Parse).
	f := TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", JumpHost: "j", JumpUser: "u"}
	if Check(f)["jumpPort"] != "" {
		t.Fatal("empty jump port should be allowed")
	}
}

func TestCheck_NeedJumpOrVia(t *testing.T) {
	f := TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2"}
	if Check(f)["jump"] == "" {
		t.Fatal("no jump host and no via should report a jump error")
	}
}

func TestCheck_JumpHostNeedsUser(t *testing.T) {
	f := TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", JumpHost: "j"}
	if Check(f)["jumpUser"] == "" {
		t.Fatal("jump host without user should report a jumpUser error")
	}
}

func TestCheck_ViaIgnoresStrayJumpFields(t *testing.T) {
	// When Via is set, inline jump fields are ignored by Parse (via wins),
	// so stray jump-host text with no jump user must not block save.
	f := TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", Via: "bastion", JumpHost: "stray"}
	errs := Check(f)
	if errs["jumpUser"] != "" {
		t.Fatalf("via set: stray jump host must not report jumpUser, got %q", errs["jumpUser"])
	}
	if errs["jump"] != "" {
		t.Fatalf("via set: must not report jump error, got %q", errs["jump"])
	}
	if len(errs) != 0 {
		t.Fatalf("via set with stray jump host should be valid, got %v", errs)
	}
}

func TestCheck_ViaIgnoresStrayJumpPort(t *testing.T) {
	// When Via is set, inline jump port is ignored by Parse, so stray text
	// in the jump-port field must not report a jumpPort error.
	f := TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", Via: "bastion", JumpPort: "-p999"}
	if msg := Check(f)["jumpPort"]; msg != "" {
		t.Fatalf("via set: stray jump port must not report jumpPort, got %q", msg)
	}
}

func TestCheck_DestHostRequired(t *testing.T) {
	f := TunnelForm{Name: "a", LocalPort: "1", DestPort: "2", JumpHost: "j", JumpUser: "u"}
	if Check(f)["destHost"] == "" {
		t.Fatal("empty dest host should error")
	}
}
