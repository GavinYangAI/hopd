package tunnel

import "testing"

func TestStateString(t *testing.T) {
	cases := map[State]string{
		StateDown:      "DOWN",
		StateStarting:  "STARTING",
		StateUp:        "UP",
		StateRetrying:  "RETRYING",
		StateNeedsAuth: "NEEDS_AUTH",
		StateError:     "ERROR",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Fatalf("State(%d).String() = %q, want %q", s, got, want)
		}
	}
}
