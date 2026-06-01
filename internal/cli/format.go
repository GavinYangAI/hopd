// Package cli holds presentation helpers shared by hopd's CLI commands.
package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

// FormatStatus renders tunnel statuses as an aligned table.
func FormatStatus(tunnels []ipc.TunnelStatus) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tGROUP\tSTATE\tLOCAL\tREMOTE\tVIA\tUPTIME\tRECONN")
	for _, t := range tunnels {
		uptime := "-"
		if t.UptimeSec > 0 {
			uptime = (time.Duration(t.UptimeSec) * time.Second).String()
		}
		via := t.Via
		if via == "" {
			via = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
			t.Name, t.Group, t.State, t.Local, t.Remote, via, uptime, t.Reconnects)
	}
	w.Flush()
	return b.String()
}
