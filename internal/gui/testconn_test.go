package gui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

func TestParseHostKeys(t *testing.T) {
	stderr := `Warning: Permanently added '198.51.100.7' (ED25519) to the list of known hosts.
ED25519 key fingerprint is SHA256:abc123def456.
userA@198.51.100.7: Permission denied (publickey).`
	got := parseHostKeys([]byte(stderr))
	want := []HostKey{
		{Host: "198.51.100.7", Algo: "ED25519", Fingerprint: "SHA256:abc123def456"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseHostKeys mismatch:\n got  %+v\n want %+v", got, want)
	}
}

func TestParseHostKeys_None(t *testing.T) {
	if got := parseHostKeys([]byte("no key info here\n")); len(got) != 0 {
		t.Fatalf("expected no host keys, got %+v", got)
	}
}

func TestParseHostKeys_AddedWithoutFingerprintLine(t *testing.T) {
	// Some ssh builds only emit the "Permanently added" line.
	stderr := `Warning: Permanently added 'bastionB' (RSA) to the list of known hosts.`
	got := parseHostKeys([]byte(stderr))
	if len(got) != 1 || got[0].Host != "bastionB" || got[0].Algo != "RSA" {
		t.Fatalf("expected one host key for bastionB/RSA, got %+v", got)
	}
}

func testConnCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(`
hosts:
  bastionB: {host: 203.0.113.9, user: userB}
  entryA: {host: 198.51.100.7, port: 65522, user: userA, key: ~/.ssh/idA, jump: bastionB}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: entryA}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cfg
}

func TestTestConnection_Success(t *testing.T) {
	cfg := testConnCfg(t)
	var gotArgs []string
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name != "ssh" {
			t.Fatalf("ran %q, want ssh", name)
		}
		gotArgs = args
		return nil, nil, nil // clean exit
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if !res.OK {
		t.Fatalf("expected OK, got %+v", res)
	}
	// argv must point ssh at a -F config, use BatchMode + accept-new, run "true".
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"-F", "BatchMode=yes", "ConnectTimeout=8", "StrictHostKeyChecking=accept-new", "entryA", "true"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %v missing %q", gotArgs, want)
		}
	}
}

func TestTestConnection_AuthFailure(t *testing.T) {
	cfg := testConnCfg(t)
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("userA@198.51.100.7: Permission denied (publickey)."), errors.New("exit status 255")
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if res.OK {
		t.Fatal("auth failure should not be OK")
	}
	if !strings.Contains(res.Reason, "认证") {
		t.Fatalf("reason should mention auth, got %q", res.Reason)
	}
}

func TestTestConnection_Unreachable(t *testing.T) {
	cfg := testConnCfg(t)
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("ssh: connect to host 198.51.100.7 port 65522: Connection timed out"), errors.New("exit status 255")
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if res.OK || !strings.Contains(res.Reason, "连不上") {
		t.Fatalf("expected unreachable reason, got %+v", res)
	}
}

func TestTestConnection_WrongPort(t *testing.T) {
	cfg := testConnCfg(t)
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("ssh: connect to host 198.51.100.7 port 65522: Connection refused"), errors.New("exit status 255")
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if res.OK || !strings.Contains(res.Reason, "端口") {
		t.Fatalf("expected port reason, got %+v", res)
	}
}

func TestTestConnection_NewHostKey(t *testing.T) {
	cfg := testConnCfg(t)
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("Warning: Permanently added '198.51.100.7' (ED25519) to the list of known hosts.\nED25519 key fingerprint is SHA256:abc."), nil
	}
	res := TestConnection(context.Background(), cfg, "entryA", run)
	if !res.OK {
		t.Fatalf("clean exit should be OK, got %+v", res)
	}
	if len(res.Fingerprints) != 1 || res.Fingerprints[0].Fingerprint != "SHA256:abc" {
		t.Fatalf("expected one parsed host key, got %+v", res.Fingerprints)
	}
}

func TestTestConnection_UnknownHost(t *testing.T) {
	cfg := testConnCfg(t)
	res := TestConnection(context.Background(), cfg, "nope", func(context.Context, string, ...string) ([]byte, []byte, error) {
		t.Fatal("runner should not be called for an unknown host")
		return nil, nil, nil
	})
	if res.OK {
		t.Fatal("unknown host should not be OK")
	}
}
