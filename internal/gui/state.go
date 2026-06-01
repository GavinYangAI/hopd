// Package gui holds hopd-gui's pure presentation logic and the daemon
// controller. It deliberately does not import Fyne, so its tests run headless.
package gui

import "github.com/GavinYangAI/hopd/internal/ipc"

// Overall is the aggregate health of all tunnels, used to pick the tray icon.
type Overall int

const (
	OverallDisconnected Overall = iota // not connected to the daemon
	OverallProblem                     // at least one ERROR (red)
	OverallAllUp                       // at least one tunnel, all UP (green)
	OverallNeutral                     // connected, mixed or no active tunnels (grey)
	OverallBusy                        // at least one STARTING/RETRYING/NEEDS_AUTH (amber)
)

// OverallState reduces a snapshot to a single aggregate state. Precedence
// follows the brief: any ERROR → problem (red); else any transient/auth state →
// busy (amber); else all-UP → green; otherwise neutral grey.
func OverallState(snap []ipc.TunnelStatus, connected bool) Overall {
	if !connected {
		return OverallDisconnected
	}
	busy := false
	for _, t := range snap {
		switch StateInfo(t.State).Tone {
		case ToneErr:
			return OverallProblem
		case ToneWarn:
			busy = true
		}
	}
	if busy {
		return OverallBusy
	}
	if len(snap) > 0 {
		allUp := true
		for _, t := range snap {
			if t.State != "UP" {
				allUp = false
				break
			}
		}
		if allUp {
			return OverallAllUp
		}
	}
	return OverallNeutral
}
