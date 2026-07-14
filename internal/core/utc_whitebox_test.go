package core

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zoneOffsetREWB matches a trailing numeric zone offset such as "-08:00".
// White-box (package core) sibling of the core_test zoneOffsetRE, named
// distinctly to avoid confusion across the two test packages compiled into the
// same test binary.
var zoneOffsetREWB = regexp.MustCompile(`[+-]\d{2}:\d{2}$`)

// withNonUTCLocalWB forces the process-global time.Local to a fixed non-UTC
// zone so a pre-change time.Now() write serializes a machine-local offset
// instead of a canonical trailing "Z". Restored on cleanup. Tests using it MUST
// be serial (no t.Parallel); package core runs a single t.Parallel test, which
// the Go runtime resumes only after all serial tests complete, so these serial
// overrides never overlap it.
func withNonUTCLocalWB(t *testing.T) {
	t.Helper()
	orig := time.Local
	time.Local = time.FixedZone("TESTNONUTC", -8*3600)
	t.Cleanup(func() { time.Local = orig })
}

// readArtifactContentWB returns the on-disk Markdown for the given artifact ID.
func readArtifactContentWB(t *testing.T, ctx context.Context, ws *Workspace, id string) string {
	t.Helper()
	path, err := FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

// assertFrontmatterUTCWB asserts the top-level frontmatter key's serialized
// value ends with exactly "Z" and carries no numeric zone offset.
func assertFrontmatterUTCWB(t *testing.T, content, key string) {
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
	assert.Falsef(t, zoneOffsetREWB.MatchString(value),
		"%s must not carry a numeric zone offset, got %q", key, value)
}
