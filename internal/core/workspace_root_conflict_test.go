package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

func TestCheckWorkspaceRootConflict_FindsBothCandidates(t *testing.T) {
	root := t.TempDir()
	for _, candidate := range WorkspaceRootCandidates() {
		path := writeResolverCandidate(t, root, candidate)
		require.NoError(t, config.WriteDefaults(path))
	}

	findings, err := CheckWorkspaceRootConflict(root)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, FindingWorkspaceRootConflict, findings[0].Type)
	assert.Contains(t, findings[0].Description, ".backlog")
	assert.Contains(t, findings[0].Description, ".backlogit")
}
