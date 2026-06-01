package version

import "testing"

func TestString(t *testing.T) {
	old := Version
	defer func() { Version = old }()

	// ldflags value wins.
	Version = "v1.2.3"
	if got := String(); got != "v1.2.3" {
		t.Fatalf("String() = %q, want v1.2.3", got)
	}

	// Empty falls back to build info; in `go test` that has no real module
	// version, so it must degrade to "dev" (never the empty string).
	Version = ""
	if got := String(); got == "" {
		t.Fatal("String() must never be empty")
	}
}
