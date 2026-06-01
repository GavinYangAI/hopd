// Package version holds the build version of hopd.
package version

import "runtime/debug"

// Version is the release version, injected at build time via
//
//	-ldflags "-X github.com/GavinYangAI/hopd/internal/version.Version=v1.2.3"
//
// (the Makefile and GoReleaser do this from the git tag). When it is empty —
// e.g. a plain `go install github.com/GavinYangAI/hopd/cmd/hopd@v1.2.3` or a
// bare `go build` — String() falls back to the module version recorded in the
// binary's build info.
var Version = ""

// String returns the best-known version string: the ldflags value if set,
// otherwise the module version from the build info, otherwise "dev".
func String() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
