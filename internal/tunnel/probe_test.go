package tunnel

import (
	"net"
	"testing"
)

func TestDialProbe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	if !dialProbe(ln.Addr().String()) {
		t.Fatalf("dialProbe should succeed against an open listener")
	}

	ln.Close()
	if dialProbe(ln.Addr().String()) {
		t.Fatalf("dialProbe should fail against a closed listener")
	}
}
