package tunnel

import (
	"net"
	"time"
)

// dialProbe reports whether a TCP connection to addr succeeds quickly.
func dialProbe(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
