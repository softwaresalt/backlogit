package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestU3_ShipmentReconstituteDryRun_PendingJournalSurvivesCLI(t *testing.T) {
	fx := newU3Fixture(t)
	path := u3WritePendingJournal(t, fx.ws, "reorder_items", fx.shipA, "reorder", "")
	before := u3SnapshotStorage(t, fx.ws.StorageRoot)

	plan := filepath.Join(fx.root, "plan.yaml")
	require.NoError(t, os.WriteFile(plan, []byte("source_shipments: ["+fx.shipA+"]\nshipments: []\n"), 0o644))

	_, err := u3CLI(t, fx.root, "shipment", "reconstitute", "--plan", plan, "--dry-run")
	require.Error(t, err)
	require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
	assert.Contains(t, err.Error(), "unfinished shipment recovery journal")

	require.FileExists(t, path, "dry-run must not recover or delete the journal")
	assert.Equal(t, before, u3SnapshotStorage(t, fx.ws.StorageRoot),
		"dry-run must leave artifacts and journals untouched")
}
