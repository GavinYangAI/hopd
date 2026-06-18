package gui

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

// fakeClient implements DaemonClient with scripted Watch frames.
type fakeClient struct {
	mu     sync.Mutex
	frames chan []ipc.TunnelStatus
	done   []ipc.Request
	failDo bool
	doErr  error // transport error from Do (e.g. socket dial failure)
}

func newFakeClient() *fakeClient { return &fakeClient{frames: make(chan []ipc.TunnelStatus, 8)} }

func (f *fakeClient) Watch(handler func(ipc.Response) error) error {
	for snap := range f.frames {
		if err := handler(ipc.Response{OK: true, Tunnels: snap}); err != nil {
			return err
		}
	}
	return errors.New("closed")
}

func (f *fakeClient) Do(req ipc.Request) (ipc.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.done = append(f.done, req)
	if f.doErr != nil {
		return ipc.Response{}, f.doErr
	}
	if f.failDo {
		return ipc.Response{Error: "boom"}, nil
	}
	return ipc.Response{OK: true}, nil
}

func (f *fakeClient) calls() []ipc.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ipc.Request(nil), f.done...)
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestController_UpdatesCacheAndNotifies(t *testing.T) {
	fc := newFakeClient()
	var (
		mu      sync.Mutex
		updates int
		notes   []Notification
	)
	c := NewController(fc)
	c.OnUpdate = func(snap []ipc.TunnelStatus, connected bool) {
		mu.Lock()
		updates++
		mu.Unlock()
	}
	c.OnNotify = func(n Notification) {
		mu.Lock()
		notes = append(notes, n)
		mu.Unlock()
	}
	c.Start()
	defer c.Stop()

	fc.frames <- []ipc.TunnelStatus{{Name: "a", State: "UP"}}
	fc.frames <- []ipc.TunnelStatus{{Name: "a", State: "ERROR", LastError: "x"}}

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return updates >= 2 && len(notes) == 1
	})
	if c.Snapshot()[0].State != "ERROR" {
		t.Fatalf("cache not updated: %+v", c.Snapshot())
	}
	if c.Connected() != true {
		t.Fatal("should be connected after frames")
	}
}

func TestController_CommandsForward(t *testing.T) {
	fc := newFakeClient()
	c := NewController(fc)
	if err := c.Up("prod"); err != nil {
		t.Fatal(err)
	}
	if err := c.Down("prod-db"); err != nil {
		t.Fatal(err)
	}
	if err := c.Reload(); err != nil {
		t.Fatal(err)
	}
	calls := fc.calls()
	if len(calls) != 3 ||
		calls[0] != (ipc.Request{Cmd: ipc.CmdUp, Target: "prod"}) ||
		calls[1] != (ipc.Request{Cmd: ipc.CmdDown, Target: "prod-db"}) ||
		calls[2] != (ipc.Request{Cmd: ipc.CmdReload}) {
		t.Fatalf("forwarded calls = %+v", calls)
	}
}

func TestController_DoErrorSurfaced(t *testing.T) {
	fc := newFakeClient()
	fc.failDo = true
	c := NewController(fc)
	if err := c.Up("x"); err == nil {
		t.Fatal("expected error from non-OK response")
	}
}

func TestController_ClassifiesDaemonRejection(t *testing.T) {
	fc := newFakeClient()
	fc.failDo = true // daemon reachable, replies OK=false with a reason
	c := NewController(fc)

	err := c.Reload()
	var rej *DaemonRejected
	if !errors.As(err, &rej) {
		t.Fatalf("want *DaemonRejected, got %T: %v", err, err)
	}
	if rej.Reason != "boom" {
		t.Fatalf("Reason = %q, want %q", rej.Reason, "boom")
	}
	if errors.Is(err, ErrDaemonUnreachable) {
		t.Fatal("a rejection must not be classified as unreachable")
	}
}

func TestController_ClassifiesDaemonUnreachable(t *testing.T) {
	fc := newFakeClient()
	fc.doErr = errors.New("dial unix: connect: connection refused")
	c := NewController(fc)

	err := c.Reload()
	if !errors.Is(err, ErrDaemonUnreachable) {
		t.Fatalf("want ErrDaemonUnreachable, got %v", err)
	}
	var rej *DaemonRejected
	if errors.As(err, &rej) {
		t.Fatal("a transport failure must not be classified as a rejection")
	}
}

func TestController_Logs(t *testing.T) {
	fc := newFakeClient()
	c := NewController(fc)
	if _, err := c.Logs("a"); err != nil {
		t.Fatal(err)
	}
	calls := fc.calls()
	if len(calls) != 1 || calls[0] != (ipc.Request{Cmd: ipc.CmdLogs, Target: "a"}) {
		t.Fatalf("logs call = %+v", calls)
	}
}
