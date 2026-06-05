package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

func TestServer_SocketIsOwnerOnly(t *testing.T) {
	// Short /tmp path (macOS t.TempDir() can exceed the 104-char sun_path limit).
	sock := fmt.Sprintf("/tmp/hopd-sock-%d.sock", os.Getpid())
	defer os.Remove(sock)
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	srv := NewServer(sock, m, func() (*config.Config, error) { return testCfg(t), nil })
	go srv.Serve()
	defer srv.Close()
	eventually(t, time.Second, func() bool { _, err := os.Stat(sock); return err == nil })
	fi, err := os.Stat(sock)
	if err != nil {
		t.Fatal(err)
	}
	if mode := fi.Mode().Perm(); mode != 0o600 {
		t.Fatalf("control socket mode = %04o, want 0600 (owner-only)", mode)
	}
}

func dialServer(t *testing.T, sock string) net.Conn {
	t.Helper()
	var conn net.Conn
	var err error
	eventually(t, time.Second, func() bool {
		conn, err = net.Dial("unix", sock)
		return err == nil
	})
	return conn
}

func TestServer_StatusRoundTrip(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "h.sock")
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	srv := NewServer(sock, m, func() (*config.Config, error) { return testCfg(t), nil })
	go srv.Serve()
	defer srv.Close()

	conn := dialServer(t, sock)
	defer conn.Close()
	enc := ipc.NewEncoder(conn)
	dec := ipc.NewDecoder(conn)

	if err := enc.Encode(ipc.Request{Cmd: ipc.CmdStatus}); err != nil {
		t.Fatal(err)
	}
	var resp ipc.Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK || len(resp.Tunnels) != 2 {
		t.Fatalf("status resp = %+v", resp)
	}
}

func TestServer_UpThenStatus(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "h.sock")
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	srv := NewServer(sock, m, func() (*config.Config, error) { return testCfg(t), nil })
	go srv.Serve()
	defer srv.Close()

	conn := dialServer(t, sock)
	defer conn.Close()
	enc := ipc.NewEncoder(conn)
	dec := ipc.NewDecoder(conn)

	_ = enc.Encode(ipc.Request{Cmd: ipc.CmdUp, Target: "all"})
	var up ipc.Response
	if err := dec.Decode(&up); err != nil || !up.OK {
		t.Fatalf("up resp = %+v err=%v", up, err)
	}
	eventually(t, 2*time.Second, func() bool {
		_ = enc.Encode(ipc.Request{Cmd: ipc.CmdStatus})
		var s ipc.Response
		if dec.Decode(&s) != nil {
			return false
		}
		for _, tn := range s.Tunnels {
			if tn.State == "DOWN" {
				return false
			}
		}
		return len(s.Tunnels) == 2
	})
}

func TestServer_UnknownTargetReturnsError(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "h.sock")
	m := NewManager(fakeSSH(t, "sleep 30"), testCfg(t))
	srv := NewServer(sock, m, func() (*config.Config, error) { return testCfg(t), nil })
	go srv.Serve()
	defer srv.Close()

	conn := dialServer(t, sock)
	defer conn.Close()
	enc := ipc.NewEncoder(conn)
	dec := ipc.NewDecoder(conn)

	_ = enc.Encode(ipc.Request{Cmd: ipc.CmdUp, Target: "nope"})
	var resp ipc.Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK || resp.Error == "" {
		t.Fatalf("expected error response, got %+v", resp)
	}
}
