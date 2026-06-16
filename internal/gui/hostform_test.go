package gui

import (
	"reflect"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

func TestHostFormParse(t *testing.T) {
	f := HostForm{
		Name: "entryA", Host: "198.51.100.7", Port: "65522", User: "userA",
		KeyFile: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: "ServerAliveInterval=15\nCompression=yes",
	}
	name, h, err := f.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if name != "entryA" {
		t.Fatalf("name = %q, want entryA", name)
	}
	want := config.Host{
		Host: "198.51.100.7", Port: 65522, User: "userA", Key: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: map[string]string{"ServerAliveInterval": "15", "Compression": "yes"},
	}
	if !reflect.DeepEqual(h, want) {
		t.Fatalf("parsed host mismatch:\n got  %+v\n want %+v", h, want)
	}
}

func TestHostFormParseDefaultsAndTrim(t *testing.T) {
	f := HostForm{Name: "  b ", Host: "  h2 ", Port: "", User: " ", KeyFile: " ", Jump: "  "}
	name, h, err := f.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if name != "b" {
		t.Fatalf("name not trimmed: %q", name)
	}
	if h.Host != "h2" || h.Port != 22 || h.User != "" || h.Key != "" || h.Jump != "" {
		t.Fatalf("defaults/trim mismatch: %+v", h)
	}
	if h.SSHOptions != nil {
		t.Fatalf("empty ssh_options should be nil map, got %v", h.SSHOptions)
	}
}

func TestHostFormParseBadOption(t *testing.T) {
	f := HostForm{Name: "a", Host: "h", SSHOptions: "ServerAliveInterval 15"}
	if _, _, err := f.Parse(); err == nil {
		t.Fatal("ssh option without '=' should error")
	}
}

func TestToHostForm(t *testing.T) {
	h := config.Host{
		Host: "198.51.100.7", Port: 65522, User: "userA", Key: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: map[string]string{"Zeta": "z", "Alpha": "a"},
	}
	f := ToHostForm("entryA", h)
	want := HostForm{
		Name: "entryA", Host: "198.51.100.7", Port: "65522", User: "userA",
		KeyFile: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: "Alpha=a\nZeta=z", // sorted
	}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("ToHostForm mismatch:\n got  %+v\n want %+v", f, want)
	}
}

func TestToHostFormDefaultPortBlank(t *testing.T) {
	// Port 22 (and 0) render as "" so the field shows the placeholder default,
	// matching how the tunnel form leaves an implicit jump port blank.
	if got := ToHostForm("b", config.Host{Host: "h", Port: 22}).Port; got != "" {
		t.Fatalf("port 22 should render blank, got %q", got)
	}
	if got := ToHostForm("b", config.Host{Host: "h", Port: 0}).Port; got != "" {
		t.Fatalf("port 0 should render blank, got %q", got)
	}
}

func TestHostFormRoundTrip(t *testing.T) {
	orig := config.Host{
		Host: "h", Port: 2222, User: "u", Key: "~/.ssh/k", Jump: "j",
		SSHOptions: map[string]string{"Compression": "yes"},
	}
	_, got, err := ToHostForm("x", orig).Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(got, orig) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", got, orig)
	}
}
