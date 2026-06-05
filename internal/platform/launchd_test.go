package platform

import (
	"strings"
	"testing"
)

func TestPlistContent(t *testing.T) {
	out := PlistContent("/usr/local/bin/hopd", "/tmp/hopd.log")
	for _, want := range []string{
		"com.gavinyangai.hopd",
		"<string>/usr/local/bin/hopd</string>",
		"<string>daemon</string>",
		"RunAtLoad",
		"KeepAlive",
		"/tmp/hopd.log",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("PlistContent missing %q in:\n%s", want, out)
		}
	}
}

func TestPlistContent_EscapesXML(t *testing.T) {
	out := PlistContent("/Users/R&D/bin/hopd", "/Users/R&D/hopd.log")
	if strings.Contains(out, "R&D") {
		t.Fatalf("path with & must be XML-escaped (raw & breaks the plist):\n%s", out)
	}
	if !strings.Contains(out, "R&amp;D") {
		t.Fatalf("expected &amp; escaping in:\n%s", out)
	}
}

func TestPlistPath(t *testing.T) {
	t.Setenv("HOME", "/Users/test")
	if got, want := PlistPath(), "/Users/test/Library/LaunchAgents/com.gavinyangai.hopd.plist"; got != want {
		t.Fatalf("PlistPath() = %q, want %q", got, want)
	}
}
