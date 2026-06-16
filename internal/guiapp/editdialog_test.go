package guiapp

import (
	"context"
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestNewEditForm_ViaHostPicker_ListsHostsAndValidates(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(
		gui.TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "10.0.1.5", DestPort: "5432", ViaHost: "entryA"},
		[]string{"entryA", "bastionB"},
		nil, nil,
	)
	if ef.viaHostSel == nil {
		t.Fatal("expected a via_host Select widget")
	}
	if len(ef.viaHostSel.Options) != 2 || ef.viaHostSel.Options[0] != "entryA" {
		t.Fatalf("picker options = %v, want [entryA bastionB]", ef.viaHostSel.Options)
	}
	if ef.viaHostSel.Selected != "entryA" {
		t.Fatalf("picker should preselect the form's ViaHost, got %q", ef.viaHostSel.Selected)
	}
	if !ef.valid() {
		t.Fatal("a complete via_host form should be valid")
	}
}

func TestNewEditForm_NewBlank_DefaultsToViaHostRoute(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(gui.TunnelForm{}, []string{"entryA"}, nil, nil)
	if ef.route != gui.RouteViaHost {
		t.Fatalf("new blank form route = %q, want %q", ef.route, gui.RouteViaHost)
	}
}

func TestNewEditForm_LegacyTunnelIsReadMostly(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(
		gui.TunnelForm{Name: "old", LocalPort: "1", DestHost: "h", DestPort: "2", Via: "bastion"},
		[]string{"entryA"}, nil, nil,
	)
	if !ef.legacy {
		t.Fatal("a via/jump tunnel should be detected as legacy")
	}
	if !ef.via.Disabled() {
		t.Fatal("legacy via entry should be disabled in read-mostly mode")
	}
	if ef.migrateBtn == nil {
		t.Fatal("expected a 迁移为主机 button for a legacy tunnel")
	}

	// The route cards must be inert: tapping the via_host card must NOT switch a
	// legacy tunnel's route, otherwise the user could bypass MigrateLegacyTunnel.
	before := ef.route
	ef.viaHostCard.tap.Tapped(nil)
	if ef.route != before {
		t.Fatalf("tapping a disabled route card changed route from %q to %q", before, ef.route)
	}
	// The via_host picker must also be disabled in read-mostly mode.
	if !ef.viaHostSel.Disabled() {
		t.Fatal("legacy via_host picker should be disabled in read-mostly mode")
	}
}

func TestNewEditForm_ViaHostTunnelIsNotLegacy(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(
		gui.TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432", ViaHost: "entryA"},
		[]string{"entryA"}, nil, nil,
	)
	if ef.legacy {
		t.Fatal("a via_host tunnel must not be flagged legacy")
	}
}

func TestEditForm_TestConnection_UsesChosenHost(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(
		gui.TunnelForm{Name: "pg", LocalPort: "5432", DestHost: "h", DestPort: "5432", ViaHost: "entryA"},
		[]string{"entryA"}, nil, nil,
	)
	ef.testCfg = func() (*config.Config, error) {
		return config.Parse([]byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 65522, user: userA}
groups:
  g:
    - {name: pg, local: "5432", remote: h:5432, via_host: entryA}
`))
	}
	var gotHost string
	ef.testRunner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return []byte(""), []byte(""), nil // success
	}
	ef.onTestResult = func(res gui.TestConnResult) { /* capture */ }
	ef.testConn = func(ctx context.Context, cfg *config.Config, host string, run gui.CmdRunner) gui.TestConnResult {
		gotHost = host
		return gui.TestConnResult{OK: true}
	}

	ef.runTest()
	if gotHost != "entryA" {
		t.Fatalf("test connection used host %q, want entryA", gotHost)
	}
}

func TestNewEditForm_PrefillsAndReads(t *testing.T) {
	_ = test.NewApp()

	initial := gui.TunnelForm{
		Name: "win-1", Group: "chuwu", LocalPort: "13389",
		DestHost: "203.0.113.10", DestPort: "3389",
		JumpUser: "root", JumpHost: "198.51.100.20", JumpPort: "65532",
		KeyFile: "~/.ssh/id_rsa",
	}
	ef := newEditForm(initial, nil, nil, nil)

	got := ef.value()
	// RawJump isn't an input widget; compare the visible fields.
	got.RawJump = initial.RawJump
	if !reflect.DeepEqual(got, initial) {
		t.Fatalf("prefill round-trip differs:\n a=%+v\n b=%+v", initial, got)
	}
}

func TestNewEditForm_CarriesAutostart(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(gui.TunnelForm{
		Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2", Autostart: true,
	}, nil, nil, nil)
	if !ef.value().Autostart {
		t.Fatal("autostart checkbox should round-trip through the form")
	}
	ef2 := newEditForm(gui.TunnelForm{Name: "b", LocalPort: "2", DestHost: "h", DestPort: "3"}, nil, nil, nil)
	if ef2.value().Autostart {
		t.Fatal("autostart should default to false when the tunnel is not marked")
	}
}

func TestNewEditForm_PreservesRawJump(t *testing.T) {
	_ = test.NewApp()
	initial := gui.TunnelForm{
		Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2",
		RawJump: []string{"a@h1:22", "b@h2:22"},
	}
	ef := newEditForm(initial, nil, nil, nil)
	if got := ef.value(); len(got.RawJump) != 2 || got.RawJump[0] != "a@h1:22" {
		t.Fatalf("RawJump not carried through the form: %v", got.RawJump)
	}
}

func TestNewEditForm_EntriesDoNotTrapScroll(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(gui.TunnelForm{}, nil, nil, nil)
	// Single-line entries must have wrapping off and scrolling none, otherwise
	// widget.Entry's internal Scroll swallows wheel events and the form can't
	// scroll while the pointer is over a field.
	singles := []*widget.Entry{ef.name, ef.group, ef.localPort, ef.destHost, ef.destPort,
		ef.jumpHost, ef.jumpPort, ef.jumpUser, ef.keyFile, ef.via}
	for i, e := range singles {
		if e.Wrapping != fyne.TextWrapOff {
			t.Fatalf("entry %d: wrapping = %v, want TextWrapOff", i, e.Wrapping)
		}
		if e.Scroll != fyne.ScrollNone {
			t.Fatalf("entry %d: scroll = %v, want ScrollNone", i, e.Scroll)
		}
	}
}
