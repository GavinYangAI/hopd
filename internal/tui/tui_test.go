package tui

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

func TestRows(t *testing.T) {
	r := rows([]ipc.TunnelStatus{
		{Name: "a", Group: "g", State: "UP", Local: "5432", Remote: "h:5432", Via: "b", UptimeSec: 5, Reconnects: 0},
		{Name: "c", Group: "g", State: "DOWN", Local: "6379", Remote: "h:6379", Reconnects: 3},
	})
	if len(r) != 2 {
		t.Fatalf("rows len = %d, want 2", len(r))
	}
	if r[0][0] != "a" || r[0][2] != "UP" || r[0][6] != "5s" {
		t.Fatalf("row 0 = %v", r[0])
	}
	if r[1][5] != "-" { // empty via renders as -
		t.Fatalf("row 1 via = %q, want -", r[1][5])
	}
	if r[1][7] != "3" {
		t.Fatalf("row 1 reconn = %q, want 3", r[1][7])
	}
}

func TestHeaderLength(t *testing.T) {
	if len(header) != 8 {
		t.Fatalf("header has %d columns, want 8", len(header))
	}
}
