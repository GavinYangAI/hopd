package ipc

import (
	"bytes"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(Request{Cmd: CmdUp, Target: "prod"}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dec := NewDecoder(&buf)
	var got Request
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Cmd != CmdUp || got.Target != "prod" {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestNewlineDelimited(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	_ = enc.Encode(Request{Cmd: CmdStatus})
	_ = enc.Encode(Request{Cmd: CmdReload})
	if n := bytes.Count(buf.Bytes(), []byte("\n")); n != 2 {
		t.Fatalf("newline count = %d, want 2", n)
	}
	dec := NewDecoder(&buf)
	var a, b Request
	if err := dec.Decode(&a); err != nil || a.Cmd != CmdStatus {
		t.Fatalf("first decode: %v %+v", err, a)
	}
	if err := dec.Decode(&b); err != nil || b.Cmd != CmdReload {
		t.Fatalf("second decode: %v %+v", err, b)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	resp := Response{
		OK: true,
		Tunnels: []TunnelStatus{
			{Name: "a", Group: "g", State: "UP", Local: "127.0.0.1:5432", Remote: "h:5432", Reconnects: 2},
		},
	}
	if err := enc.Encode(resp); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dec := NewDecoder(&buf)
	var got Response
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !got.OK || len(got.Tunnels) != 1 || got.Tunnels[0].Reconnects != 2 {
		t.Fatalf("response round trip = %+v", got)
	}
}
