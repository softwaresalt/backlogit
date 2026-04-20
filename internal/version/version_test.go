package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestVersion_NotEmpty asserts that Version has a non-empty default value.
func TestVersion_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, Version, "Version must not be empty")
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
