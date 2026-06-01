package guiapp

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestNewEditForm_PrefillsAndReads(t *testing.T) {
	_ = test.NewApp()

	initial := gui.TunnelForm{
		Name: "win-1", Group: "chuwu", LocalPort: "13389",
		DestHost: "203.0.113.10", DestPort: "3389",
		JumpUser: "root", JumpHost: "198.51.100.20", JumpPort: "65532",
		KeyFile: "~/.ssh/id_rsa",
	}
	ef := newEditForm(initial)

	got := ef.value()
	// RawJump isn't an input widget; compare the visible fields.
	got.RawJump = initial.RawJump
	if !reflect.DeepEqual(got, initial) {
		t.Fatalf("prefill round-trip differs:\n a=%+v\n b=%+v", initial, got)
	}
}

func TestNewEditForm_PreservesRawJump(t *testing.T) {
	_ = test.NewApp()
	initial := gui.TunnelForm{
		Name: "a", LocalPort: "1", DestHost: "h", DestPort: "2",
		RawJump: []string{"a@h1:22", "b@h2:22"},
	}
	ef := newEditForm(initial)
	if got := ef.value(); len(got.RawJump) != 2 || got.RawJump[0] != "a@h1:22" {
		t.Fatalf("RawJump not carried through the form: %v", got.RawJump)
	}
}

func TestNewEditForm_EntriesDoNotTrapScroll(t *testing.T) {
	_ = test.NewApp()
	ef := newEditForm(gui.TunnelForm{})
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
