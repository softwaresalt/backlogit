package models_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// withNonUTCLocal forces the process-global time.Local to a fixed non-UTC zone
// so a pre-change time.Now() write serializes a machine-local offset instead of
// a canonical trailing "Z". The override is restored on cleanup. Tests that call
// it MUST be serial (no t.Parallel) because time.Local is process-global; the
// internal/models package runs no t.Parallel tests, so a serial override is safe.
func withNonUTCLocal(t *testing.T) {
	t.Helper()
	orig := time.Local
	time.Local = time.FixedZone("TESTNONUTC", -8*3600)
	t.Cleanup(func() { time.Local = orig })
}

// TestNowUTC_ReturnsCanonicalUTC proves NowUTC normalizes wall-clock time to UTC
// even when the process-local zone is non-UTC, so every writer that consumes it
// serializes created_at/updated_at with a canonical trailing "Z".
func TestNowUTC_ReturnsCanonicalUTC(t *testing.T) {
	withNonUTCLocal(t)

	got := models.NowUTC()

	assert.Equal(t, "UTC", got.Location().String(), "NowUTC must return a UTC-located time")
	formatted := got.Format(time.RFC3339Nano)
	assert.True(t, strings.HasSuffix(formatted, "Z"),
		"NowUTC serialization must end with exactly Z, got %q", formatted)
	assert.NotRegexp(t, `[+-]\d{2}:\d{2}$`, formatted,
		"NowUTC serialization must not carry a numeric zone offset, got %q", formatted)
}

// TestArtifactFromFrontmatter_DefaultTimestampsUTC proves the defensive
// created_at/updated_at defaults (applied when parsed frontmatter omits the
// timestamps) are normalized to UTC rather than the machine-local offset.
func TestArtifactFromFrontmatter_DefaultTimestampsUTC(t *testing.T) {
	withNonUTCLocal(t)

	// Frontmatter deliberately omits created_at/updated_at so the defensive
	// defaults in ArtifactFromFrontmatter apply.
	fm := map[string]any{
		"id":            "T001",
		"title":         "Test",
		"status":        "queued",
		"artifact_type": "task",
	}
	artifact, err := models.ArtifactFromFrontmatter(fm, "body")
	require.NoError(t, err)

	assert.Equal(t, "UTC", artifact.CreatedAt.Location().String(), "default created_at must be UTC")
	assert.Equal(t, "UTC", artifact.UpdatedAt.Location().String(), "default updated_at must be UTC")
	assert.True(t, strings.HasSuffix(artifact.CreatedAt.Format(time.RFC3339Nano), "Z"),
		"default created_at must serialize with a trailing Z")
	assert.True(t, strings.HasSuffix(artifact.UpdatedAt.Format(time.RFC3339Nano), "Z"),
		"default updated_at must serialize with a trailing Z")
}
