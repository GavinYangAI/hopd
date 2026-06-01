package gui

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

func TestOverallState(t *testing.T) {
	up := ipc.TunnelStatus{Name: "a", State: "UP"}
	down := ipc.TunnelStatus{Name: "b", State: "DOWN"}
	errt := ipc.TunnelStatus{Name: "c", State: "ERROR"}
	auth := ipc.TunnelStatus{Name: "d", State: "NEEDS_AUTH"}

	cases := []struct {
		name      string
		snap      []ipc.TunnelStatus
		connected bool
		want      Overall
	}{
		{"disconnected", []ipc.TunnelStatus{up}, false, OverallDisconnected},
		{"error wins", []ipc.TunnelStatus{up, errt}, true, OverallProblem},
		{"error beats auth", []ipc.TunnelStatus{auth, errt}, true, OverallProblem},
		{"auth is busy", []ipc.TunnelStatus{up, auth}, true, OverallBusy},
		{"all up", []ipc.TunnelStatus{up}, true, OverallAllUp},
		{"mixed neutral", []ipc.TunnelStatus{up, down}, true, OverallNeutral},
		{"empty neutral", nil, true, OverallNeutral},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OverallState(tc.snap, tc.connected); got != tc.want {
				t.Fatalf("OverallState = %v, want %v", got, tc.want)
			}
		})
	}
}
