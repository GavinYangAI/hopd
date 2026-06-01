package gui

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

func names(ns []Notification) map[string]bool {
	m := map[string]bool{}
	for _, n := range ns {
		m[n.Tunnel] = true
	}
	return m
}

func TestDiff_FirstFrameSilent(t *testing.T) {
	next := []ipc.TunnelStatus{{Name: "a", State: "ERROR"}}
	if got := Diff(nil, next); len(got) != 0 {
		t.Fatalf("first frame (no prev) should be silent, got %v", got)
	}
}

func TestDiff_Transitions(t *testing.T) {
	prev := []ipc.TunnelStatus{
		{Name: "a", State: "UP"},
		{Name: "b", State: "UP"},
		{Name: "c", State: "STARTING"},
		{Name: "d", State: "UP"},
	}
	next := []ipc.TunnelStatus{
		{Name: "a", State: "ERROR", LastError: "bind"}, // UP -> ERROR : notify
		{Name: "b", State: "RETRYING"},                 // UP -> RETRYING : notify (dropped)
		{Name: "c", State: "NEEDS_AUTH"},               // STARTING -> NEEDS_AUTH : notify
		{Name: "d", State: "UP"},                       // unchanged : silent
	}
	got := names(Diff(prev, next))
	for _, want := range []string{"a", "b", "c"} {
		if !got[want] {
			t.Fatalf("expected notification for %q, got %v", want, got)
		}
	}
	if got["d"] {
		t.Fatalf("unchanged tunnel d should not notify")
	}
}

func TestDiff_RetryingOnlyFromUp(t *testing.T) {
	prev := []ipc.TunnelStatus{{Name: "a", State: "STARTING"}}
	next := []ipc.TunnelStatus{{Name: "a", State: "RETRYING"}}
	if got := Diff(prev, next); len(got) != 0 {
		t.Fatalf("STARTING->RETRYING should be silent, got %v", got)
	}
}

func TestDiff_NewTunnelSilent(t *testing.T) {
	next := []ipc.TunnelStatus{{Name: "new", State: "ERROR"}}
	prev := []ipc.TunnelStatus{{Name: "other", State: "UP"}}
	if got := Diff(prev, next); len(got) != 0 {
		t.Fatalf("a newly-appeared tunnel should not notify, got %v", got)
	}
}
