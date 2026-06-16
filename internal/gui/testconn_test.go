package gui

import (
	"reflect"
	"testing"
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
