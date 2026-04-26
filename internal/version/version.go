// Package version holds the canonical backlogit version string and build metadata.
// All vars are overridden at build time via:
//
//	-X github.com/softwaresalt/backlogit/internal/version.Version=<tag>
//	-X github.com/softwaresalt/backlogit/internal/version.Commit=<sha>
//	-X github.com/softwaresalt/backlogit/internal/version.BuildDate=<rfc3339>
package version

// Version is the current backlogit release version.
var Version = "1.2.0"

// Commit is the short git SHA injected at build time. Defaults to "unknown".
var Commit = "unknown"

// BuildDate is the RFC3339 UTC build timestamp injected at build time. Defaults to "".
var BuildDate = ""
