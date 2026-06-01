package ipc

import (
	"net"
	"path/filepath"
	"testing"
)

func TestClient_Do(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var req Request
		_ = NewDecoder(conn).Decode(&req)
		_ = NewEncoder(conn).Encode(Response{OK: true, Tunnels: []TunnelStatus{{Name: req.Target}}})
	}()

	c := NewClient(sock)
	resp, err := c.Do(Request{Cmd: CmdStatus, Target: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || len(resp.Tunnels) != 1 || resp.Tunnels[0].Name != "x" {
		t.Fatalf("Do resp = %+v", resp)
	}
}

func TestClient_Watch(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var req Request
		_ = NewDecoder(conn).Decode(&req)
		enc := NewEncoder(conn)
		_ = enc.Encode(Response{OK: true})
		_ = enc.Encode(Response{OK: true})
	}()

	c := NewClient(sock)
	n := 0
	err = c.Watch(Request{Cmd: CmdWatch}, func(Response) error {
		n++
		if n == 2 {
			return errStop
		}
		return nil
	})
	if err != errStop || n != 2 {
		t.Fatalf("Watch n=%d err=%v", n, err)
	}
}

var errStop = errSentinel("stop")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
