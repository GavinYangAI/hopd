// Package ipc defines the JSON-Lines control protocol between the hopd
// daemon and its CLI/TUI clients over a Unix domain socket.
package ipc

// Command names sent by clients.
const (
	CmdUp     = "up"
	CmdDown   = "down"
	CmdStatus = "status"
	CmdList   = "list"
	CmdReload = "reload"
	CmdLogs   = "logs"
	CmdWatch  = "watch" // subscribe to streaming status updates
)

// Request is a single client command. Target is a tunnel name, group name,
// or the literal "all" where applicable.
type Request struct {
	Cmd    string `json:"cmd"`
	Target string `json:"target,omitempty"`
}

// TunnelStatus is a point-in-time snapshot of one tunnel.
type TunnelStatus struct {
	Name       string `json:"name"`
	Group      string `json:"group"`
	State      string `json:"state"`
	Local      string `json:"local"`
	Remote     string `json:"remote"`
	Via        string `json:"via,omitempty"`
	ViaHost    string `json:"via_host,omitempty"`
	UptimeSec  int64  `json:"uptime_sec"`
	Reconnects int    `json:"reconnects"`
	LastError  string `json:"last_error,omitempty"`
}

// Response is the daemon's reply. For CmdWatch the daemon emits a stream of
// Response values, each carrying the full Tunnels snapshot.
type Response struct {
	OK      bool           `json:"ok"`
	Error   string         `json:"error,omitempty"`
	Tunnels []TunnelStatus `json:"tunnels,omitempty"`
	Lines   []string       `json:"lines,omitempty"` // for CmdLogs
}
