package gui

import (
	"sort"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

// MenuTunnelItem is one toggleable tunnel row in the tray menu.
type MenuTunnelItem struct {
	Name    string
	Label   string
	State   string
	Checked bool // true when the tunnel is not DOWN (i.e. switched on)
}

// MenuGroup is a named set of tunnel items.
type MenuGroup struct {
	Name  string
	Items []MenuTunnelItem
}

// MenuModel is a UI-agnostic description of the tray menu contents.
type MenuModel struct {
	Connected bool
	Summary   string
	Groups    []MenuGroup
}

// BuildMenuModel converts a snapshot into a tray menu model. Groups are sorted
// by name; items keep snapshot (config) order within a group.
func BuildMenuModel(snap []ipc.TunnelStatus, connected bool) MenuModel {
	if !connected {
		return MenuModel{Connected: false, Summary: "daemon 未运行"}
	}
	m := MenuModel{Connected: true, Summary: Summarize(snap)}

	byGroup := map[string][]MenuTunnelItem{}
	var order []string
	for _, t := range snap {
		if _, ok := byGroup[t.Group]; !ok {
			order = append(order, t.Group)
		}
		byGroup[t.Group] = append(byGroup[t.Group], MenuTunnelItem{
			Name:    t.Name,
			Label:   itemLabel(t),
			State:   t.State,
			Checked: t.State != "DOWN",
		})
	}
	sort.Strings(order)
	for _, g := range order {
		m.Groups = append(m.Groups, MenuGroup{Name: g, Items: byGroup[g]})
	}
	return m
}

func itemLabel(t ipc.TunnelStatus) string {
	meta := StateInfo(t.State)
	// macOS tray menu items are plain text (no colour), so we carry the state
	// in words and flag the two attention states with a leading glyph.
	switch t.State {
	case "ERROR":
		return "✗ " + t.Name + " · " + meta.Label
	case "NEEDS_AUTH":
		return "⚠ " + t.Name + " · " + meta.Label
	default:
		return t.Name + " · " + meta.Label
	}
}
