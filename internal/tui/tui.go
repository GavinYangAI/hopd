// Package tui renders the live hopd dashboard with tview.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/GavinYangAI/hopd/internal/ipc"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var header = []string{"NAME", "GROUP", "STATE", "LOCAL", "REMOTE", "VIA", "UPTIME", "RECONN"}

// rows converts statuses into table cell rows (data only, no header).
func rows(tunnels []ipc.TunnelStatus) [][]string {
	out := make([][]string, 0, len(tunnels))
	for _, t := range tunnels {
		uptime := "-"
		if t.UptimeSec > 0 {
			uptime = (time.Duration(t.UptimeSec) * time.Second).String()
		}
		via := t.Via
		if via == "" {
			via = "-"
		}
		out = append(out, []string{
			t.Name, t.Group, t.State, t.Local, t.Remote, via, uptime, fmt.Sprintf("%d", t.Reconnects),
		})
	}
	return out
}

func stateColor(state string) tcell.Color {
	switch state {
	case "UP":
		return tcell.ColorGreen
	case "STARTING", "RETRYING":
		return tcell.ColorYellow
	case "NEEDS_AUTH":
		return tcell.ColorOrange
	case "ERROR":
		return tcell.ColorRed
	default:
		return tcell.ColorGray
	}
}

// Run launches the dashboard, streaming status from the daemon.
func Run(c *ipc.Client) error {
	app := tview.NewApplication()
	table := tview.NewTable().SetFixed(1, 0).SetSelectable(true, false)

	render := func(tunnels []ipc.TunnelStatus) {
		table.Clear()
		for col, h := range header {
			table.SetCell(0, col, tview.NewTableCell(h).
				SetSelectable(false).SetAttributes(tcell.AttrBold))
		}
		for i, row := range rows(tunnels) {
			color := stateColor(row[2])
			for col, val := range row {
				table.SetCell(i+1, col, tview.NewTableCell(val).SetTextColor(color))
			}
		}
	}

	selectedName := func() string {
		r, _ := table.GetSelection()
		if r <= 0 {
			return ""
		}
		return table.GetCell(r, 0).Text
	}

	pages := tview.NewPages()
	logs := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	logs.SetBorder(true).SetTitle(" logs (esc to close) ")

	send := func(req ipc.Request) { go func() { _, _ = c.Do(req) }() }

	// authenticate suspends the TUI and runs `hopd auth <name>` so the user can
	// complete interactive 2FA on a real terminal (e.g. for a NEEDS_AUTH row).
	authenticate := func(name string) {
		if name == "" {
			return
		}
		exe, err := os.Executable()
		if err != nil {
			return
		}
		app.Suspend(func() {
			cmd := exec.Command(exe, "auth", name)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			_ = cmd.Run()
		})
	}

	showLogs := func(name string) {
		resp, err := c.Do(ipc.Request{Cmd: ipc.CmdLogs, Target: name})
		logs.Clear()
		switch {
		case err != nil:
			fmt.Fprintf(logs, "error: %v\n", err)
		case !resp.OK:
			fmt.Fprintf(logs, "error: %s\n", resp.Error)
		case len(resp.Lines) == 0:
			fmt.Fprintln(logs, "(no output captured)")
		default:
			for _, line := range resp.Lines {
				fmt.Fprintln(logs, line)
			}
		}
		logs.SetTitle(fmt.Sprintf(" logs: %s (esc to close) ", name))
		pages.ShowPage("logs")
	}

	logs.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEsc {
			pages.HidePage("logs")
		}
		return ev
	})

	table.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEnter {
			if n := selectedName(); n != "" {
				showLogs(n)
			}
			return nil
		}
		switch ev.Rune() {
		case 'q':
			app.Stop()
		case 's':
			send(ipc.Request{Cmd: ipc.CmdUp, Target: selectedName()})
		case 'x':
			send(ipc.Request{Cmd: ipc.CmdDown, Target: selectedName()})
		case 'r':
			n := selectedName()
			send(ipc.Request{Cmd: ipc.CmdDown, Target: n})
			send(ipc.Request{Cmd: ipc.CmdUp, Target: n})
		case 'R':
			send(ipc.Request{Cmd: ipc.CmdReload})
		case 'a':
			send(ipc.Request{Cmd: ipc.CmdUp, Target: "all"})
		case 'A':
			authenticate(selectedName())
		}
		return ev
	})

	go func() {
		_ = c.Watch(ipc.Request{Cmd: ipc.CmdWatch}, func(resp ipc.Response) error {
			app.QueueUpdateDraw(func() { render(resp.Tunnels) })
			return nil
		})
	}()

	pages.AddPage("table", table, true, true)
	pages.AddPage("logs", logs, true, false)
	app.SetRoot(pages, true).EnableMouse(true)
	return app.Run()
}
