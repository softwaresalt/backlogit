package templates_test

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

// templatesZoneOffsetRE matches a trailing numeric zone offset such as "-08:00".
var templatesZoneOffsetRE = regexp.MustCompile(`[+-]\d{2}:\d{2}$`)

// withNonUTCLocalTemplates forces the process-global time.Local to a fixed
// non-UTC zone so a pre-change time.Now() write serializes a machine-local
// offset instead of a canonical trailing "Z". Restored on cleanup. Package
// templates_test runs no t.Parallel tests, so this serial override is safe.
func withNonUTCLocalTemplates(t *testing.T) {
	t.Helper()
	orig := time.Local
	time.Local = time.FixedZone("TESTNONUTC", -8*3600)
	t.Cleanup(func() { time.Local = orig })
}

// assertFrontmatterUTCTemplates asserts the top-level frontmatter key's
// serialized value ends with exactly "Z" and carries no numeric zone offset.
func assertFrontmatterUTCTemplates(t *testing.T, content, key string) {
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
	assert.Falsef(t, templatesZoneOffsetRE.MatchString(value),
		"%s must not carry a numeric zone offset, got %q", key, value)
}

// TestServiceUpdate_EmitsUTCUpdatedAt proves the template service Update path
// serializes updated_at in canonical UTC even under a non-UTC local zone
// (site: templates/service.go Update).
func TestServiceUpdate_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocalTemplates(t)
	ws, svc := setupServiceSyncWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Templates UTC feature", "feature")
	require.NoError(t, err)
	artifact, err := svc.Create(ctx, ws, "Templates UTC task", "task", nil, core.WithParent(feat.ID))
	require.NoError(t, err)

	_, err = svc.Update(ctx, ws, artifact.ID, map[string]string{"description": "UTC body"})
	require.NoError(t, err)

	filePath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assertFrontmatterUTCTemplates(t, string(raw), "updated_at")
}
