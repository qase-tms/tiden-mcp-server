// Package version exposes the build version.
package version

import "runtime/debug"

// Version is set at build time via:
//
//	-ldflags "-X github.com/qase-tms/tiden-mcp-server/internal/version.Version=v1.2.3"
var Version = ""

// Get returns the stamped version, falling back to module build info
// (covers `go install ...@vX.Y.Z`), then "dev".
func Get() string {
	if Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
