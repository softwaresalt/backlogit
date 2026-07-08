package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestVersion_NotEmpty asserts that Version has a non-empty default value.
func TestVersion_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, Version, "Version must not be empty")
}

// TestResolve_NotEmpty asserts that Resolve always returns a non-empty string.
func TestResolve_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, Resolve(), "Resolve must never return an empty string")
}

// TestResolve_PrefersInjectedVersion asserts that an ldflags-injected Version
// (anything other than DevVersion) is returned verbatim, ahead of build info.
func TestResolve_PrefersInjectedVersion(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "9.9.9"
	assert.Equal(t, "9.9.9", Resolve(),
		"Resolve must return the ldflags-injected Version when it is not DevVersion")
}

// TestResolve_FallsBackWhenDev asserts that with the DevVersion default, Resolve
// returns either a real module version (from build info) or DevVersion itself,
// never a stale hardcoded release number.
func TestResolve_FallsBackWhenDev(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = DevVersion
	got := Resolve()
	assert.NotEmpty(t, got, "Resolve must not be empty in dev context")
	// In `go test`, build info carries no real module version, so Resolve
	// returns DevVersion. Under `go install ...@vX.Y.Z` it would return the
	// module version instead. Either way it must not be the old stale default.
	assert.NotEqual(t, "1.2.0", got, "Resolve must not report the old stale source default")
}

// TestCommit_DefaultIsUnknown asserts that Commit defaults to "unknown" when
// not injected by ldflags (go run / go test context).
func TestCommit_DefaultIsUnknown(t *testing.T) {
	assert.Equal(t, "unknown", Commit,
		"Commit should default to 'unknown' when not injected by -ldflags")
}

// TestBuildDate_DefaultIsEmpty asserts that BuildDate defaults to "" when
// not injected by ldflags.
func TestBuildDate_DefaultIsEmpty(t *testing.T) {
	assert.Equal(t, "", BuildDate,
		"BuildDate should default to empty string when not injected by -ldflags")
}
