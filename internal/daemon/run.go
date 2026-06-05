package daemon

import (
	"context"
	"os"
	"path/filepath"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/tunnel"
)

// Run loads config, starts the control server, brings up any autostart tunnels,
// and supervises until ctx is cancelled. Non-autostart tunnels start DOWN;
// clients bring them up on demand.
func Run(ctx context.Context, configFile, sockPath, controlDir, sshPath string) error {
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(controlDir, 0o700); err != nil {
		return err
	}

	load := func() (*config.Config, error) {
		c, err := config.Load(configFile)
		if err != nil {
			return nil, err
		}
		injectControl(c, controlDir)
		return c, nil
	}

	cfg, err := load()
	if err != nil {
		return err
	}
	mgr := NewManager(sshPath, cfg)
	mgr.StartAutostart()
	srv := NewServer(sockPath, mgr, load)

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve() }()

	select {
	case <-ctx.Done():
		mgr.StopAll()
		srv.Close()
		return nil
	case err := <-errc:
		mgr.StopAll()
		return err
	}
}

// injectControl adds hopd's default ssh options to each tunnel that hasn't set
// them: ControlMaster multiplexing (so one auth covers reconnects) plus
// ExitOnForwardFailure=yes (so a local-bind failure makes ssh exit, letting the
// runner surface it as ERROR instead of looping). Tunnel.SSHOptions is a shared
// map, so writing through the loop value mutates the config in place.
//
// ControlPersist is forced to "no" (overriding ControlOptions' auth default):
// under modern OpenSSH a ControlPersist=<time> master detaches into the
// background right after binding the forward, so the foreground `ssh -N` the
// runner supervises exits immediately and the tunnel loops forever in
// RETRYING. "no" keeps the master in the foreground for the runner to hold; an
// already-running master pre-warmed by `hopd auth` (ControlPersist=300) is
// still reused as a client. A user who sets ControlPersist themselves wins.
func injectControl(cfg *config.Config, controlDir string) {
	defaults := tunnel.ControlOptions(controlDir, "no")
	defaults["ExitOnForwardFailure"] = "yes"
	for _, t := range cfg.Tunnels() {
		for k, v := range defaults {
			if _, ok := t.SSHOptions[k]; !ok {
				t.SSHOptions[k] = v
			}
		}
	}
}
