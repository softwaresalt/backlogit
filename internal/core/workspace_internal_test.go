package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/backlogit/backlogit/internal/errors"
)

func TestResolveWorkspaceRoot_PropagatesReadDirErrors(t *testing.T) {
	// Arrange
	root := t.TempDir()
	filePath := filepath.Join(root, "not-a-directory")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	// Act
	_, err := resolveWorkspaceRoot(filePath)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read workspace root")
}

func TestLoadArtifact_PreservesFindArtifactErrors(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	brokenRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(brokenRoot, ".backlogit"), []byte("not a workspace dir"), 0o644))

	brokenWorkspace := &Workspace{
		RootPath: brokenRoot,
		DB:       ws.DB,
	}

	// Act
	_, err := loadArtifact(ctx, brokenWorkspace, "T999")

	// Assert
	require.Error(t, err)
	assert.False(t, errors.Is(err, blerrors.ErrNotFound))
	assert.Contains(t, err.Error(), "read workspace storage root")
}
