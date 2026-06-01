package daemon

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

// Supervised ssh must keep ControlPersist=no: under modern OpenSSH a
// ControlPersist=<time> master backgrounds itself (detaches, PPID 1) right
// after binding the forward, the foreground `ssh -N` exits, and the runner —
// which supervises that foreground process — misreads the exit as a failure
// and loops forever in RETRYING. Keeping the master in the foreground lets the
// runner hold and supervise it.
func TestInjectControl_SupervisedUsesControlPersistNo(t *testing.T) {
	cfg, err := config.Parse([]byte(`
groups:
  g1:
    - { name: a, local: 15001, remote: h:80, via: bastion }
`))
	if err != nil {
		t.Fatal(err)
	}
	injectControl(cfg, "/cm")
	tn, ok := cfg.Tunnel("a")
	if !ok {
		t.Fatal("tunnel a missing")
	}
	if got := tn.SSHOptions["ControlPersist"]; got != "no" {
		t.Fatalf("ControlPersist = %q, want no", got)
	}
	if got := tn.SSHOptions["ControlMaster"]; got != "auto" {
		t.Fatalf("ControlMaster = %q, want auto", got)
	}
	if got := tn.SSHOptions["ControlPath"]; got != "/cm/%C" {
		t.Fatalf("ControlPath = %q, want /cm/%%C", got)
	}
	if got := tn.SSHOptions["ExitOnForwardFailure"]; got != "yes" {
		t.Fatalf("ExitOnForwardFailure = %q, want yes", got)
	}
}

// A user who deliberately sets ControlPersist keeps their value.
func TestInjectControl_DoesNotOverrideUserControlPersist(t *testing.T) {
	cfg, err := config.Parse([]byte(`
groups:
  g1:
    - name: a
      local: 15001
      remote: h:80
      via: bastion
      ssh_options:
        ControlPersist: "600"
`))
	if err != nil {
		t.Fatal(err)
	}
	injectControl(cfg, "/cm")
	tn, _ := cfg.Tunnel("a")
	if got := tn.SSHOptions["ControlPersist"]; got != "600" {
		t.Fatalf("user ControlPersist should win, got %q", got)
	}
}
