package guiapp

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestOpenHosts_Constructs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
hosts:
  entryA: {host: 198.51.100.7, port: 65522, user: userA, jump: bastionB}
  bastionB: {host: 203.0.113.9, user: userB}
groups: {}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	d := newDashboard(app, &DashboardActions{})
	d.setStore(gui.NewConfigStore(path, func() error { return nil }))
	// Must not panic building the hosts window from a real store.
	d.openHosts()
	if d.hostsWin == nil {
		t.Fatal("hosts window not created")
	}
}

func TestHostCardSummary(t *testing.T) {
	_ = test.NewApp()
	// hostSummary renders host:port · user · ↳jump for the card subtitle.
	got := hostSummary(gui.HostForm{Host: "198.51.100.7", Port: "65522", User: "userA", Jump: "bastionB"})
	for _, want := range []string{"198.51.100.7:65522", "userA", "bastionB"} {
		if !contains(got, want) {
			t.Fatalf("summary %q missing %q", got, want)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
