package tunnel

// State is the lifecycle state of a single tunnel.
type State int

const (
	StateDown State = iota // not started, or manually stopped
	StateStarting
	StateUp
	StateRetrying  // ssh exited unexpectedly; backing off before retry
	StateNeedsAuth // ssh is asking for interactive auth (password/2FA)
	StateError     // unrecoverable for now (e.g. local port in use)
)

func (s State) String() string {
	switch s {
	case StateDown:
		return "DOWN"
	case StateStarting:
		return "STARTING"
	case StateUp:
		return "UP"
	case StateRetrying:
		return "RETRYING"
	case StateNeedsAuth:
		return "NEEDS_AUTH"
	case StateError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
