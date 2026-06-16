package guiapp

import (
	"path/filepath"
	"reflect"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestNewHostForm_PrefillsAndReads(t *testing.T) {
	_ = test.NewApp()
	initial := gui.HostForm{
		Name: "entryA", Host: "198.51.100.7", Port: "65522", User: "userA",
		KeyFile: "~/.ssh/idA", Jump: "bastionB",
		SSHOptions: "Compression=yes",
	}
	hf := newHostForm(initial, []string{"bastionB"})
	got := hf.value()
	if !reflect.DeepEqual(got, initial) {
		t.Fatalf("prefill round-trip differs:\n a=%+v\n b=%+v", initial, got)
	}
}

func TestNewHostForm_JumpOptions(t *testing.T) {
	_ = test.NewApp()
	hf := newHostForm(gui.HostForm{Name: "a", Host: "h"}, []string{"b", "c"})
	// The jump select must offer the candidate hosts plus the empty "（不用跳板）"
	// sentinel, and never list the host being edited.
	for _, want := range []string{"b", "c"} {
		found := false
		for _, o := range hf.jump.Options {
			if o == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("jump options %v missing %q", hf.jump.Options, want)
		}
	}
}

func TestHostForm_SaveButtonGating(t *testing.T) {
	_ = test.NewApp()
	hf := newHostForm(gui.HostForm{}, nil) // empty: name+host missing => invalid
	if !hf.saveBtn.Disabled() {
		t.Fatal("save should be disabled while required fields are empty")
	}
	hf.name.SetText("entryA")
	hf.host.SetText("198.51.100.7")
	hf.refresh()
	if hf.saveBtn.Disabled() {
		t.Fatalf("save should enable once name+host are filled")
	}
}

func TestShowHostDialog_Constructs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("t")
	store := gui.NewConfigStore(filepath.Join(t.TempDir(), "config.yaml"), func() error { return nil })
	// Must not panic constructing/showing the dialog.
	showHostDialog(win, store, gui.HostForm{Name: "a", Host: "h"}, "", func() {})
}
