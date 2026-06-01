package cli

import (
	"strings"
	"testing"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

func TestFormatStatus(t *testing.T) {
	out := FormatStatus([]ipc.TunnelStatus{
		{Name: "a", Group: "g", State: "UP", Local: "5432", Remote: "h:5432", Via: "b", UptimeSec: 65, Reconnects: 1},
	})
	for _, want := range []string{"NAME", "STATE", "UPTIME", "a", "UP", "5432", "h:5432", "1m5s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("FormatStatus missing %q in:\n%s", want, out)
		}
	}
}

func TestFormatStatus_Empty(t *testing.T) {
	out := FormatStatus(nil)
	if !strings.Contains(out, "NAME") {
		t.Fatalf("empty status should still show header, got:\n%s", out)
	}
}
