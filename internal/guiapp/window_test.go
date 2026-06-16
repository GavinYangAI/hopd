package guiapp

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/gui"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

func TestDashboard_Constructs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	d := newDashboard(app, &DashboardActions{}) // empty actions: construction smoke test
	d.update([]ipc.TunnelStatus{{Name: "a", Group: "g", State: "UP", Local: "5432", Remote: "h:5432"}})
	if d.win == nil || d.body == nil {
		t.Fatal("dashboard not constructed")
	}
	if len(d.body.Objects) == 0 {
		t.Fatal("body should render cards for a non-empty snapshot")
	}
	d.selectTunnel("a")
	if _, ok := d.selected(); !ok {
		t.Fatal("selectTunnel should mark the tunnel selected")
	}
}

func TestFmtDuration(t *testing.T) {
	if got := fmtDuration(65); got != "1m05s" {
		t.Fatalf("fmtDuration(65) = %q", got)
	}
	if got := fmtDuration(7980); got != "2h13m" {
		t.Fatalf("fmtDuration(7980) = %q", got)
	}
}

func TestHasLegacyTunnels(t *testing.T) {
	if hasLegacyTunnels(nil) {
		t.Fatal("nil snapshot has no legacy tunnels")
	}
	snap := []ipc.TunnelStatus{
		{Name: "a", ViaHost: "entryA"},
		{Name: "b", Via: "bastion"},
	}
	if !hasLegacyTunnels(snap) {
		t.Fatal("a tunnel with Via should count as legacy")
	}
	only := []ipc.TunnelStatus{{Name: "a", ViaHost: "entryA"}}
	if hasLegacyTunnels(only) {
		t.Fatal("via_host-only snapshot is not legacy")
	}
}

func TestFiltered_MatchesViaHost(t *testing.T) {
	d := &dashboard{
		snap: []ipc.TunnelStatus{
			{Name: "pg", Group: "prod", Remote: "10.0.1.5:5432", ViaHost: "entryA"},
			{Name: "rd", Group: "prod", Remote: "h:6379", ViaHost: "bastionB"},
		},
		query: "entryA",
	}
	got := d.filtered()
	if len(got) != 1 || got[0].Name != "pg" {
		t.Fatalf("filter by via_host: got %+v, want only pg", got)
	}
}

func TestDashboard_WithStoreConstructs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	store := gui.NewConfigStore(filepath.Join(dir, "config.yaml"), func() error { return nil })
	d := newDashboard(app, &DashboardActions{})
	d.setStore(store)
	if d.store == nil {
		t.Fatal("store not set")
	}
}
