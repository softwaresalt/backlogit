// Package version holds the canonical backlogit version string and build metadata.
//
// Release binaries (goreleaser) override these vars at build time via:
//
//	-X github.com/softwaresalt/backlogit/internal/version.Version=<tag>
//	-X github.com/softwaresalt/backlogit/internal/version.Commit=<sha>
//	-X github.com/softwaresalt/backlogit/internal/version.BuildDate=<rfc3339>
//
// Builds produced with `go install github.com/softwaresalt/backlogit/cmd/backlogit@vX.Y.Z`
// do NOT receive those ldflags, so Version stays at its DevVersion default. For
// those builds Resolve recovers the real version from the module build info
// (runtime/debug.ReadBuildInfo), which records the installed module version.
package version

import (
	"runtime/debug"
	"strings"
)

// DevVersion is the compiled-in default used when no release ldflags were applied.
const DevVersion = "dev"

// Version is the backlogit release version. It is overridden by ldflags for
// release binaries and otherwise defaults to DevVersion. Prefer Resolve over
// reading this var directly so `go install ...@vX.Y.Z` builds report correctly.
var Version = DevVersion

// Commit is the short git SHA injected at build time. Defaults to "unknown".
var Commit = "unknown"

// BuildDate is the RFC3339 UTC build timestamp injected at build time. Defaults to "".
var BuildDate = ""

// Resolve returns the effective version string, in priority order:
//
//  1. an ldflags-injected Version (release binaries), when it is not DevVersion;
//  2. the main module version from build info, for `go install ...@vX.Y.Z`
//     builds where no ldflags were applied (leading "v" trimmed);
//  3. the compiled-in DevVersion default (local `go build` / `go test`).
//
// The build-info fallback prevents the version from silently regressing to the
// source default when the release ldflags are absent.
func Resolve() string {
	if Version != "" && Version != DevVersion {
		return Version
	}
	if v := moduleVersion(); v != "" {
		return v
	}
	return Version
}

// moduleVersion returns the usable semantic version from build info, or "" when
// the build carries no real module version (e.g. local `go build`, which records
// "(devel)").
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return ""
	}
	return strings.TrimPrefix(v, "v")
}
