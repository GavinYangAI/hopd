package sshconf_test

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/sshconf"
)

func TestParseSSHConfig(t *testing.T) {
	data := []byte(`
Host *
    ServerAliveInterval 30

Host entryA
    HostName 198.51.100.7
    Port 65522
    User userA
    IdentityFile ~/.ssh/idA
    ProxyJump bastionB

Host bastionB
    HostName 203.0.113.9
    User userB
`)
	got, err := sshconf.ParseSSHConfig(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d hosts, want 2 (wildcard skipped): %+v", len(got), got)
	}
	byName := map[string]sshconf.ImportedHost{}
	for _, h := range got {
		byName[h.Name] = h
	}
	a := byName["entryA"]
	if a.HostName != "198.51.100.7" || a.Port != 65522 || a.User != "userA" || a.IdentityFile != "~/.ssh/idA" || a.ProxyJump != "bastionB" {
		t.Fatalf("entryA mismatch: %+v", a)
	}
	b := byName["bastionB"]
	if b.HostName != "203.0.113.9" || b.User != "userB" || b.Port != 0 {
		t.Fatalf("bastionB mismatch: %+v", b)
	}
}
