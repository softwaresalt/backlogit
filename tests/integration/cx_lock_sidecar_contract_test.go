package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUCXS4_HarnessLockSidecarName pins the `.agent-lock` sidecar suffix. Each
// row is named after its repo-relative file so a failing row is attributable to
// that file. Scripts must name the suffix and drop the retired construction;
// documentation must name the suffix.
func TestUCXS4_HarnessLockSidecarName(t *testing.T) {
	repoRoot := testRepoRoot(t)

	rows := []struct {
		path    string
		retired string
	}{
		{path: "scripts/acquire_lock.ps1", retired: `.$fileName.lock"`},
		{path: "scripts/release_lock.ps1", retired: `.$fileName.lock"`},
		{path: "scripts/acquire_lock.sh", retired: `.${FILENAME}.lock"`},
		{path: "scripts/release_lock.sh", retired: `.${FILENAME}.lock"`},
		{path: ".github/instructions/concurrency.instructions.md"},
		{path: ".github/skills/file-lock/SKILL.md"},
	}
	for _, row := range rows {
		t.Run(row.path, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(row.path)))
			require.NoError(t, err, "read lock contract source %s", row.path)
			text := string(content)

			assert.Contains(t, text, ".agent-lock", "%s must name the .agent-lock sidecar suffix", row.path)
			if row.retired != "" {
				assert.NotContains(t, text, row.retired, "%s must drop the retired sidecar construction", row.path)
			}
		})
	}
}
