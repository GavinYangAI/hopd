package guiapp

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/GavinYangAI/hopd/internal/gui"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

func TestAdapter_SatisfiesInterface(t *testing.T) {
	var _ gui.DaemonClient = NewDaemonClient("/tmp/x.sock")
}

func TestAdapter_WatchBindsRequest(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan ipc.Request, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var req ipc.Request
		_ = ipc.NewDecoder(conn).Decode(&req)
		got <- req
		_ = ipc.NewEncoder(conn).Encode(ipc.Response{OK: true, Tunnels: []ipc.TunnelStatus{{Name: "a", State: "UP"}}})
	}()

	dc := NewDaemonClient(sock)
	frames := make(chan []ipc.TunnelStatus, 1)
	go func() {
		_ = dc.Watch(func(resp ipc.Response) error {
			frames <- resp.Tunnels
			return errStop
		})
	}()

	if req := <-got; req.Cmd != ipc.CmdWatch {
		t.Fatalf("watch should send CmdWatch, got %q", req.Cmd)
	}
	if f := <-frames; len(f) != 1 || f[0].Name != "a" {
		t.Fatalf("frame = %+v", f)
	}
}

var errStop = stopErr("stop")

type stopErr string

func (e stopErr) Error() string { return string(e) }
