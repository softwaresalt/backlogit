package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	backlogitcli "github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIClaimPredecessorNotShipped verifies `shipment claim` surfaces the
// shipment_predecessor_not_shipped token and the predecessor ID.
func TestCLIClaimPredecessorNotShipped(t *testing.T) {
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	closed := false
	t.Cleanup(func() {
		if !closed {
			require.NoError(t, ws.Close())
		}
	})

	feat, err := core.CreateArtifact(ctx, ws, "Predecessor feature", "feature")
	require.NoError(t, err)
	predMember, err := core.CreateArtifact(ctx, ws, "Predecessor member", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	depMember, err := core.CreateArtifact(ctx, ws, "Dependent member", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	pred, err := core.CreateShipment(ctx, ws, "Predecessor shipment", []string{predMember.ID})
	require.NoError(t, err)
	dep, err := core.CreateShipment(ctx, ws, "Dependent shipment", []string{depMember.ID})
	require.NoError(t, err)
	require.NoError(t, core.AddDependency(ctx, ws, dep.ID, pred.ID, "blocks"))
	require.NoError(t, ws.Close())
	closed = true

	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd := backlogitcli.NewRootCommand()
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"--cwd", root, "shipment", "claim", dep.ID})
	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shipment_predecessor_not_shipped")
	assert.Contains(t, err.Error(), pred.ID)
}
