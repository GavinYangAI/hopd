package daemon

import (
	"net"
	"os"
	"sync"
	"time"

	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

// ReloadFunc re-reads config from disk; injected so the server need not know
// file paths. It returns a validated Config or an error.
type ReloadFunc func() (*config.Config, error)

// Server serves the control protocol over a Unix socket.
type Server struct {
	sock    string
	mgr     *Manager
	reload  ReloadFunc
	ln      net.Listener
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	closing bool
}

// NewServer creates a server bound to socket path sock.
func NewServer(sock string, mgr *Manager, reload ReloadFunc) *Server {
	return &Server{sock: sock, mgr: mgr, reload: reload, conns: map[net.Conn]struct{}{}}
}

// Serve listens and handles connections until Close is called.
func (s *Server) Serve() error {
	_ = os.Remove(s.sock) // clear any stale socket
	ln, err := net.Listen("unix", s.sock)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closing := s.closing
			s.mu.Unlock()
			if closing {
				return nil
			}
			return err
		}
		s.track(conn)
		go s.handle(conn)
	}
}

// Close stops the listener and closes the socket file.
func (s *Server) Close() {
	s.mu.Lock()
	s.closing = true
	ln := s.ln
	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	_ = os.Remove(s.sock)
}

func (s *Server) track(c net.Conn) {
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) untrack(c net.Conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	defer s.untrack(conn)
	dec := ipc.NewDecoder(conn)
	enc := ipc.NewEncoder(conn)
	for {
		var req ipc.Request
		if err := dec.Decode(&req); err != nil {
			return // client disconnected
		}
		if req.Cmd == ipc.CmdWatch {
			s.streamStatus(conn, enc)
			return
		}
		_ = enc.Encode(s.dispatch(req))
	}
}

func (s *Server) dispatch(req ipc.Request) ipc.Response {
	switch req.Cmd {
	case ipc.CmdStatus, ipc.CmdList:
		return ipc.Response{OK: true, Tunnels: s.mgr.Status()}
	case ipc.CmdUp:
		if err := s.mgr.Up(req.Target); err != nil {
			return ipc.Response{Error: err.Error()}
		}
		return ipc.Response{OK: true, Tunnels: s.mgr.Status()}
	case ipc.CmdDown:
		if err := s.mgr.Down(req.Target); err != nil {
			return ipc.Response{Error: err.Error()}
		}
		return ipc.Response{OK: true, Tunnels: s.mgr.Status()}
	case ipc.CmdReload:
		cfg, err := s.reload()
		if err != nil {
			return ipc.Response{Error: err.Error()}
		}
		if err := s.mgr.Reload(cfg); err != nil {
			return ipc.Response{Error: err.Error()}
		}
		return ipc.Response{OK: true, Tunnels: s.mgr.Status()}
	case ipc.CmdLogs:
		lines, err := s.mgr.Logs(req.Target)
		if err != nil {
			return ipc.Response{Error: err.Error()}
		}
		return ipc.Response{OK: true, Lines: lines}
	default:
		return ipc.Response{Error: "unknown command: " + req.Cmd}
	}
}

// streamStatus pushes a status snapshot every second until the client leaves.
func (s *Server) streamStatus(conn net.Conn, enc *ipc.Encoder) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	if err := enc.Encode(ipc.Response{OK: true, Tunnels: s.mgr.Status()}); err != nil {
		return
	}
	for range ticker.C {
		if err := enc.Encode(ipc.Response{OK: true, Tunnels: s.mgr.Status()}); err != nil {
			return
		}
	}
}
