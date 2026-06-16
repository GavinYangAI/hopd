package gui

import (
	"reflect"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)

func sampleImported() []sshconf.ImportedHost {
	return []sshconf.ImportedHost{
		{Name: "entryA", HostName: "198.51.100.7", Port: 65522, User: "userA", IdentityFile: "~/.ssh/idA", ProxyJump: "bastionB"},
		{Name: "bastionB", HostName: "203.0.113.9", User: "userB"},
		{Name: "lonely", HostName: "192.0.2.1", ProxyJump: "missing"},
	}
}

func TestBuildHostsFromImport_MapsFieldsAndDefaults(t *testing.T) {
	got, err := BuildHostsFromImport(sampleImported(), []string{"bastionB"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := map[string]config.Host{
		"bastionB": {Host: "203.0.113.9", Port: 22, User: "userB"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestBuildHostsFromImport_ProxyJumpKeptWhenAlsoSelected(t *testing.T) {
	got, err := BuildHostsFromImport(sampleImported(), []string{"entryA", "bastionB"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	a := got["entryA"]
	if a.Host != "198.51.100.7" || a.Port != 65522 || a.User != "userA" || a.Key != "~/.ssh/idA" {
		t.Fatalf("entryA fields wrong: %+v", a)
	}
	if a.Jump != "bastionB" {
		t.Fatalf("entryA.Jump = %q, want bastionB (referenced alias is selected)", a.Jump)
	}
}

func TestBuildHostsFromImport_ProxyJumpDroppedWhenNotSelected(t *testing.T) {
	// entryA selected but bastionB is NOT -> jump dropped, no error.
	got, err := BuildHostsFromImport(sampleImported(), []string{"entryA"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got["entryA"].Jump != "" {
		t.Fatalf("entryA.Jump = %q, want empty (referenced alias not selected)", got["entryA"].Jump)
	}
}

func TestBuildHostsFromImport_ProxyJumpDroppedWhenAliasUnknown(t *testing.T) {
	// "lonely" references "missing" which isn't in the import set at all.
	got, err := BuildHostsFromImport(sampleImported(), []string{"lonely"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got["lonely"].Jump != "" {
		t.Fatalf("lonely.Jump = %q, want empty (unknown alias)", got["lonely"].Jump)
	}
}

func TestBuildHostsFromImport_SelectedNameNotInImportIsIgnored(t *testing.T) {
	got, err := BuildHostsFromImport(sampleImported(), []string{"entryA", "ghost"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, ok := got["ghost"]; ok {
		t.Fatalf("ghost should not appear; got %+v", got)
	}
	if len(got) != 1 {
		t.Fatalf("want exactly entryA, got %+v", got)
	}
}

func TestExistingImportNames(t *testing.T) {
	cfg, err := config.Parse([]byte(`
hosts:
  bastionB: {host: 203.0.113.9, user: userB}
groups:
  g:
    - {name: t1, local: "5432", remote: x:5432, via_host: bastionB}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	dup := ExistingImportNames(cfg, sampleImported())
	if !dup["bastionB"] {
		t.Fatalf("bastionB should be flagged as existing")
	}
	if dup["entryA"] {
		t.Fatalf("entryA should not be flagged (not in config)")
	}
}
