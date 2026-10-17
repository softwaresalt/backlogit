package evidencedag

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU163_008_ReadWorkspaceFileOnlyAllowsGithubHiddenComponent(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{".git/config", ".github/policies/r.yaml", "_x/f.json", ".backlogit/f.json", "a/.git/config"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, p), []byte("x"), 0o600))
	}
	got, err := ReadWorkspaceFile(root, ".github/policies/r.yaml", 16)
	require.NoError(t, err)
	assert.Equal(t, "x", string(got))
	for _, rel := range []string{".git/config", ".GIT/config", "_x/f.json", ".backlogit/f.json", "a/.git/config", ".github/.git/config"} {
		_, err := ReadWorkspaceFile(root, rel, 16)
		assert.Error(t, err, rel)
	}
}

func TestU163_008_ShipmentIDsStillValidTaskIDs(t *testing.T) {
	assert.True(t, ValidTaskID("011-S"))
	assert.True(t, ValidTaskID("001.002-T"))
}
