package core_test

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// zoneOffsetRE matches a trailing numeric zone offset such as "-08:00".
var zoneOffsetRE = regexp.MustCompile(`[+-]\d{2}:\d{2}$`)

// withNonUTCLocal forces the process-global time.Local to a fixed non-UTC zone
// so a pre-change time.Now() write serializes a machine-local offset instead of
// a canonical trailing "Z". The override is restored on cleanup. Tests using it
// MUST be serial (no t.Parallel) because time.Local is process-global; package
// core_test runs a single t.Parallel test, which the Go runtime resumes only
// after all serial tests complete, so these serial overrides never overlap it.
func withNonUTCLocal(t *testing.T) {
	t.Helper()
	orig := time.Local
	time.Local = time.FixedZone("TESTNONUTC", -8*3600)
	t.Cleanup(func() { time.Local = orig })
}

// readArtifactContent returns the on-disk Markdown for the given artifact ID.
func readArtifactContent(t *testing.T, ctx context.Context, ws *core.Workspace, id string) string {
	t.Helper()
	path, err := core.FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

// assertFrontmatterUTC asserts the top-level frontmatter key's serialized value
// ends with exactly "Z" and carries no numeric zone offset.
func assertFrontmatterUTC(t *testing.T, content, key string) {
	t.Helper()
	var value string
	found := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, key+":") {
			value = strings.TrimSpace(strings.TrimPrefix(line, key+":"))
			found = true
			break
		}
	}
	require.Truef(t, found, "frontmatter key %q not found in:\n%s", key, content)
	assert.Truef(t, strings.HasSuffix(value, "Z"),
		"%s must end with exactly Z, got %q", key, value)
	assert.Falsef(t, zoneOffsetRE.MatchString(value),
		"%s must not carry a numeric zone offset, got %q", key, value)
}

// TestCreateArtifact_EmitsUTCTimestamps proves CreateArtifact stamps both
// created_at and updated_at in canonical UTC even under a non-UTC local zone.
func TestCreateArtifact_EmitsUTCTimestamps(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "UTC create feature", "feature")
	require.NoError(t, err)

	content := readArtifactContent(t, ctx, ws, feat.ID)
	assertFrontmatterUTC(t, content, "created_at")
	assertFrontmatterUTC(t, content, "updated_at")
}

// TestUpdateArtifact_EmitsUTCUpdatedAt proves the ungated field-update path
// restamps updated_at in canonical UTC.
func TestUpdateArtifact_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "UTC update feature", "feature")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, feat.ID, map[string]any{"title": "Retitled"})
	require.NoError(t, err)

	content := readArtifactContent(t, ctx, ws, feat.ID)
	assertFrontmatterUTC(t, content, "updated_at")
}

// TestArtifactLinks_EmitUTCUpdatedAt proves both the add-link and remove-link
// restamp paths write updated_at in canonical UTC.
func TestArtifactLinks_EmitUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Link source", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Link target", "feature")
	require.NoError(t, err)

	require.NoError(t, core.AddArtifactLink(ctx, ws, source.ID, target.ID, "related_to"))
	assertFrontmatterUTC(t, readArtifactContent(t, ctx, ws, source.ID), "updated_at")

	require.NoError(t, core.RemoveArtifactLink(ctx, ws, source.ID, target.ID, "related_to"))
	assertFrontmatterUTC(t, readArtifactContent(t, ctx, ws, source.ID), "updated_at")
}
