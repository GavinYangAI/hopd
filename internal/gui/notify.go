package gui

import "github.com/GavinYangAI/hopd/internal/ipc"

// Notification is a UI-agnostic description of something worth surfacing.
type Notification struct {
	Tunnel  string
	Title   string
	Message string
}

// Diff compares two snapshots and returns notifications for alert-worthy
// transitions. It only fires when a tunnel existed in prev with a different
// state, so the first frame and freshly-appeared tunnels stay silent.
func Diff(prev, next []ipc.TunnelStatus) []Notification {
	pm := make(map[string]ipc.TunnelStatus, len(prev))
	for _, t := range prev {
		pm[t.Name] = t
	}
	var out []Notification
	for _, t := range next {
		p, ok := pm[t.Name]
		if !ok || p.State == t.State {
			continue
		}
		switch t.State {
		case "ERROR":
			msg := t.Name + " entered ERROR"
			if t.LastError != "" {
				msg += ": " + t.LastError
			}
			out = append(out, Notification{t.Name, "hopd: tunnel error", msg})
		case "NEEDS_AUTH":
			out = append(out, Notification{
				t.Name,
				"hopd: authentication needed",
				t.Name + " needs interactive login — run: hopd auth " + t.Name,
			})
		case "RETRYING":
			if p.State == "UP" {
				out = append(out, Notification{t.Name, "hopd: tunnel dropped", t.Name + " disconnected, reconnecting…"})
			}
		}
	}
	return out
}
