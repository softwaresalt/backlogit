package cli

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const u3DryRunJournalName = "shipment-operation-0123456789abcdef0123456789abcdef.json"

// u3WritePendingJournal writes a valid pending (intent) lifecycle journal and
// returns its path.
func u3WritePendingJournal(t *testing.T, ws *core.Workspace, operation, shipmentID, target string, related string) string {
	t.Helper()
	ctx := context.Background()
	shipment, err := core.GetShipment(ctx, ws, shipmentID)
	require.NoError(t, err)
	members := make([]any, 0)
	for _, memberID := range core.NormalizeShipmentItems(shipment) {
		member, memberErr := bldb.GetItem(ctx, ws.DB, memberID)
		require.NoError(t, memberErr)
		members = append(members, member)
	}
	preimage := map[string]any{"shipment": shipment, "members": members}
	if related != "" {
		other, relErr := core.GetShipment(ctx, ws, related)
		require.NoError(t, relErr)
		preimage["related"] = []any{other}
	}
	journal := map[string]any{
		"schema_version":  "shipment-operation/v1",
		"correlation_id":  "0123456789abcdef0123456789abcdef",
		"phase":           "intent",
		"operation":       operation,
		"recovery_policy": "rollback",
		"shipment_id":     shipmentID,
		"target":          target,
		"preimage":        preimage,
	}
	data, err := json.Marshal(journal)
	require.NoError(t, err)
	opsDir := filepath.Join(ws.StorageRoot, "ops")
	require.NoError(t, os.MkdirAll(opsDir, 0o755))
	path := filepath.Join(opsDir, u3DryRunJournalName)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

// u3SnapshotStorage returns the bytes of every markdown artifact and ops journal.
func u3SnapshotStorage(t *testing.T, storageRoot string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	require.NoError(t, filepath.WalkDir(storageRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(storageRoot, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if rel == "logs" || rel == ".locks" {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") || strings.HasPrefix(rel, "ops"+string(filepath.Separator)) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			snap[rel] = string(data)
		}
		return nil
	}))
	return snap
}

func TestU3_ShipmentManifestDryRun_PendingJournalSurvivesCLI(t *testing.T) {
	tests := []struct {
		name    string
		journal func(fx *u3Fixture) (op, shipment, target, related string)
		args    func(fx *u3Fixture) []string
	}{
		{
			name: "remove",
			journal: func(fx *u3Fixture) (string, string, string, string) {
				return "remove_item", fx.shipA, fx.taskA1, ""
			},
			args: func(fx *u3Fixture) []string {
				return []string{"shipment", "remove", fx.shipA, fx.taskA1, "--dry-run"}
			},
		},
		{
			name: "move",
			journal: func(fx *u3Fixture) (string, string, string, string) {
				return "move_item", fx.shipA, fx.shipB, fx.shipB
			},
			args: func(fx *u3Fixture) []string {
				return []string{"shipment", "move", fx.taskA1, "--from", fx.shipA, "--to", fx.shipB, "--dry-run"}
			},
		},
		{
			name: "reorder",
			journal: func(fx *u3Fixture) (string, string, string, string) {
				return "reorder_items", fx.shipA, "reorder", ""
			},
			args: func(fx *u3Fixture) []string {
				return []string{"shipment", "reorder", fx.shipA, "--auto", "--dry-run"}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fx := newU3Fixture(t)
			op, shipment, target, related := tc.journal(fx)
			path := u3WritePendingJournal(t, fx.ws, op, shipment, target, related)
			before := u3SnapshotStorage(t, fx.ws.StorageRoot)

			_, err := u3CLI(t, fx.root, tc.args(fx)...)
			require.Error(t, err)
			require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
			assert.Contains(t, err.Error(), "unfinished shipment recovery journal")

			require.FileExists(t, path, "dry-run must not recover or delete the journal")
			assert.Equal(t, before, u3SnapshotStorage(t, fx.ws.StorageRoot),
				"dry-run must leave artifacts and journals untouched")
		})
	}
}
