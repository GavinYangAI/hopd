package guiapp

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestShowTestConnDialog_Success(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("t")
	// No host keys, OK => must not panic.
	showTestConnDialog(win, "entryA", gui.TestConnResult{OK: true})
}

func TestShowTestConnDialog_NewHostKey(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("t")
	showTestConnDialog(win, "entryA", gui.TestConnResult{
		OK: true,
		Fingerprints: []gui.HostKey{
			{Host: "198.51.100.7", Algo: "ED25519", Fingerprint: "SHA256:abc"},
		},
	})
}

func TestShowTestConnDialog_Failure(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("t")
	showTestConnDialog(win, "entryA", gui.TestConnResult{OK: false, Reason: "认证失败"})
}

func TestFingerprintLines(t *testing.T) {
	got := fingerprintLines([]gui.HostKey{
		{Host: "h1", Algo: "ED25519", Fingerprint: "SHA256:abc"},
		{Host: "h2", Algo: "RSA", Fingerprint: ""},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d: %v", len(got), got)
	}
}
