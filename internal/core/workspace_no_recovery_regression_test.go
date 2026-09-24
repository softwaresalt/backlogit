package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestNewWorkspaceWithoutRecoveryForTest_IsolatesRecoveryState(t *testing.T) {
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	opsRoot := filepath.Join(storageRoot, "ops")
	require.NoError(t, os.MkdirAll(opsRoot, 0o755))
	journalPath := filepath.Join(
		opsRoot,
		"shipment-operation-0123456789abcdef0123456789abcdef.json",
	)
	const malformedJournal = `{}`
	require.NoError(t, os.WriteFile(journalPath, []byte(malformedJournal), 0o644))

	recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelRecovery()
	recoveryWorkspace, err := core.NewWorkspace(recoveryCtx, root)
	require.ErrorIs(t, err, blerrors.ErrValidation)
	require.Nil(t, recoveryWorkspace)

	isolatedCtx, cancelIsolated := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelIsolated()
	isolatedWorkspace, err := core.NewWorkspaceWithoutRecoveryForTest(isolatedCtx, root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, isolatedWorkspace.Close()) })

	gotJournal, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	require.Equal(t, malformedJournal, string(gotJournal))
}
