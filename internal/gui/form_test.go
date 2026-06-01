package gui

import (
	"reflect"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

func TestTunnelForm_Parse_JumpFields(t *testing.T) {
	f := TunnelForm{
		Name: "win-1", Group: "chuwu", LocalPort: "13389",
		DestHost: "203.0.113.10", DestPort: "3389",
		JumpUser: "root", JumpHost: "198.51.100.20", JumpPort: "65532",
		KeyFile: "~/.ssh/id_rsa",
	}
	tn, err := f.Parse()
	if err != nil {
		t.Fatal(err)
	}
	if tn.Name != "win-1" || tn.Group != "chuwu" || tn.Local != "13389" {
		t.Fatalf("scalars wrong: %+v", tn)
	}
	if tn.Remote != "203.0.113.10:3389" {
		t.Fatalf("remote = %q", tn.Remote)
	}
	if !reflect.DeepEqual(tn.Jump, []string{"root@198.51.100.20:65532"}) {
		t.Fatalf("jump = %v", tn.Jump)
	}
	if tn.SSHOptions["IdentityFile"] != "~/.ssh/id_rsa" {
		t.Fatalf("IdentityFile = %v", tn.SSHOptions)
	}
}

func TestTunnelForm_Parse_JumpPortDefaults22(t *testing.T) {
	f := TunnelForm{
		Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2",
		JumpUser: "me", JumpHost: "jump",
	}
	tn, err := f.Parse()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tn.Jump, []string{"me@jump:22"}) {
		t.Fatalf("jump = %v, want me@jump:22", tn.Jump)
	}
}

func TestTunnelForm_Parse_NoUserOmitsAt(t *testing.T) {
	f := TunnelForm{
		Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2",
		JumpHost: "jump", JumpPort: "2222",
	}
	tn, _ := f.Parse()
	if !reflect.DeepEqual(tn.Jump, []string{"jump:2222"}) {
		t.Fatalf("jump = %v, want jump:2222 (no @)", tn.Jump)
	}
}

func TestTunnelForm_Parse_ViaWins(t *testing.T) {
	f := TunnelForm{
		Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2",
		Via: "bastion", JumpHost: "ignored", JumpUser: "x",
	}
	tn, _ := f.Parse()
	if tn.Via != "bastion" {
		t.Fatalf("via = %q", tn.Via)
	}
	if len(tn.Jump) != 0 {
		t.Fatalf("via set, jump should be empty, got %v", tn.Jump)
	}
}

func TestTunnelForm_Parse_RawJumpFallback(t *testing.T) {
	// No via, no jump host typed, but RawJump carried a multi-hop chain.
	f := TunnelForm{
		Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2",
		RawJump: []string{"a@h1:22", "b@h2:22"},
	}
	tn, _ := f.Parse()
	if !reflect.DeepEqual(tn.Jump, []string{"a@h1:22", "b@h2:22"}) {
		t.Fatalf("jump = %v, want raw chain preserved", tn.Jump)
	}
}

func TestTunnelForm_Parse_KeyFileOverridesOption(t *testing.T) {
	f := TunnelForm{
		Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", JumpHost: "j",
		KeyFile:    "/keys/main",
		SSHOptions: "IdentityFile=/keys/stale\nConnectTimeout=5",
	}
	tn, err := f.Parse()
	if err != nil {
		t.Fatal(err)
	}
	if tn.SSHOptions["IdentityFile"] != "/keys/main" {
		t.Fatalf("KeyFile should override the multiline IdentityFile, got %q", tn.SSHOptions["IdentityFile"])
	}
	if tn.SSHOptions["ConnectTimeout"] != "5" {
		t.Fatalf("other options dropped: %v", tn.SSHOptions)
	}
}

func TestTunnelForm_Parse_BadOptionLine(t *testing.T) {
	f := TunnelForm{Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", JumpHost: "j", SSHOptions: "noequals"}
	if _, err := f.Parse(); err == nil {
		t.Fatal("ssh option line without '=' should error")
	}
}

func TestToForm_SingleJumpRoundTrip(t *testing.T) {
	tn := config.Tunnel{
		Name: "win-1", Group: "chuwu", Local: "13389", Remote: "203.0.113.10:3389",
		Jump:       []string{"root@198.51.100.20:65532"},
		SSHOptions: map[string]string{"IdentityFile": "~/.ssh/id_rsa", "ConnectTimeout": "5"},
	}
	f := ToForm(tn)
	if f.JumpUser != "root" || f.JumpHost != "198.51.100.20" || f.JumpPort != "65532" {
		t.Fatalf("jump split wrong: user=%q host=%q port=%q", f.JumpUser, f.JumpHost, f.JumpPort)
	}
	if f.DestHost != "203.0.113.10" || f.DestPort != "3389" {
		t.Fatalf("dest split wrong: host=%q port=%q", f.DestHost, f.DestPort)
	}
	if f.LocalPort != "13389" {
		t.Fatalf("local = %q", f.LocalPort)
	}
	if f.KeyFile != "~/.ssh/id_rsa" {
		t.Fatalf("keyfile = %q", f.KeyFile)
	}
	if f.SSHOptions != "ConnectTimeout=5" {
		t.Fatalf("remaining options = %q (IdentityFile should be extracted)", f.SSHOptions)
	}
	back, err := f.Parse()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, tn) {
		t.Fatalf("round trip differs:\n a=%+v\n b=%+v", tn, back)
	}
}

func TestToForm_MultiJumpUsesRaw(t *testing.T) {
	tn := config.Tunnel{
		Name: "a", Group: "g", Local: "1", Remote: "h:2",
		Jump: []string{"a@h1:22", "b@h2:22"},
	}
	f := ToForm(tn)
	if f.JumpHost != "" || f.JumpUser != "" || f.JumpPort != "" {
		t.Fatalf("multi-hop should leave jump fields empty: %+v", f)
	}
	if !reflect.DeepEqual(f.RawJump, []string{"a@h1:22", "b@h2:22"}) {
		t.Fatalf("RawJump not preserved: %v", f.RawJump)
	}
}

func TestToForm_ViaTunnel(t *testing.T) {
	tn := config.Tunnel{Name: "a", Group: "g", Local: "1", Remote: "h:2", Via: "bastion"}
	f := ToForm(tn)
	if f.Via != "bastion" || f.JumpHost != "" {
		t.Fatalf("via tunnel form wrong: %+v", f)
	}
	back, _ := f.Parse()
	if !reflect.DeepEqual(back, tn) {
		t.Fatalf("via round trip differs:\n a=%+v\n b=%+v", tn, back)
	}
}
