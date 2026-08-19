package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 143.002-T (Unit 2): colocated contract tests for the shipment-scoped
// error-returning appender. The appender's own contract cannot be exercised
// before its signature exists, so it is tested here rather than in the Unit 1
// harness. These tests were written and observed failing before the appender
// body landed.
//
// MUST NOT call t.Parallel(): ShipShipment reads package globals that sibling
// tests in this package override.

func plantLockSidecarDirectory(t *testing.T, logsDir, itemID string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	sidecar := filepath.Join(logsDir, "."+itemID+".jsonl.lock")
	require.NoError(t, os.RemoveAll(sidecar))
	require.NoError(t, os.MkdirAll(sidecar, 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(sidecar) })
	return sidecar
}

// Contract 1: a lock failure raised inside the appender, before any writer call,
// is tagged not-applied. Nothing was written, so compensation is safe, and the
// message names the shipment event log rather than gate evidence.
func TestAppendShipmentEventErr_LockFailureIsNotApplied(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	plantLockSidecarDirectory(t, logsDir, "001-S")

	err := appendShipmentEventErr(ctx, ws, "001-S", "shipment_status_changed", map[string]any{
		"status": string(ShipmentShipped),
	})

	require.Error(t, err)
	assert.True(t, blerrors.IsWriteNotApplied(err),
		"a pre-writer lock failure is proven not-applied, got: %v", err)
	assert.False(t, blerrors.IsWriteIndeterminate(err),
		"a pre-writer lock failure must not be indeterminate, got: %v", err)
	assert.Contains(t, err.Error(), "lock shipment event log",
		"the message must name the shipment event log, not gate evidence")
}

// Contract 2: the appender wraps the writer error with %w and adds no class of
// its own. Any durability sentinel the writer already attached must survive
// unwrapping, and an untagged error must stay untagged so the classifier can
// treat it as unproven.
func TestAppendShipmentEventErr_WriterErrorClassPassthrough(t *testing.T) {
	t.Run("wrapping_preserves_sentinels", func(t *testing.T) {
		cases := []struct {
			name              string
			writerErr         error
			wantNotApplied    bool
			wantIndeterminate bool
		}{
			{
				name:           "tagged_not_applied_survives",
				writerErr:      fmt.Errorf("open item log file: %w: %w", blerrors.ErrWriteNotApplied, errors.New("boom")),
				wantNotApplied: true,
			},
			{
				name:              "tagged_indeterminate_survives",
				writerErr:         fmt.Errorf("append event: %w: %w", blerrors.ErrWriteIndeterminate, errors.New("boom")),
				wantIndeterminate: true,
			},
			{
				name:      "untagged_stays_untagged",
				writerErr: errors.New("plain writer failure"),
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				wrapped := wrapShipmentAppendError("001-S", "shipment_status_changed", testCase.writerErr)
				require.Error(t, wrapped)
				assert.ErrorIs(t, wrapped, testCase.writerErr, "the cause must remain reachable through %%w")
				assert.Equal(t, testCase.wantNotApplied, blerrors.IsWriteNotApplied(wrapped))
				assert.Equal(t, testCase.wantIndeterminate, blerrors.IsWriteIndeterminate(wrapped))
				assert.Contains(t, wrapped.Error(), "append shipment event")
			})
		}
	})

	t.Run("durable_writer_failure_arrives_tagged", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		require.NotNil(t, ws.Config)
		ws.Config.DurableWrites = true

		logsDir := WorkspaceLogsRoot(ws.RootPath)
		require.NoError(t, os.MkdirAll(events.LogPathForItem(logsDir, "001-S"), 0o755))

		err := appendShipmentEventErr(ctx, ws, "001-S", "shipment_status_changed", map[string]any{
			"status": string(ShipmentShipped),
		})
		require.Error(t, err)
		assert.True(t, blerrors.IsWriteNotApplied(err),
			"the durable path tags a pre-write open failure not-applied, got: %v", err)
	})

	t.Run("non_durable_writer_failure_stays_untagged", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		require.NotNil(t, ws.Config)
		ws.Config.DurableWrites = false

		logsDir := WorkspaceLogsRoot(ws.RootPath)
		require.NoError(t, os.MkdirAll(events.LogPathForItem(logsDir, "001-S"), 0o755))

		err := appendShipmentEventErr(ctx, ws, "001-S", "shipment_status_changed", map[string]any{
			"status": string(ShipmentShipped),
		})
		require.Error(t, err)
		assert.False(t, blerrors.IsWriteNotApplied(err),
			"the default non-durable path cannot prove pre-write status, got: %v", err)
		assert.False(t, blerrors.IsWriteIndeterminate(err),
			"the appender must add no class of its own, got: %v", err)
	})
}

// Contract 3: success ordering. active -> shipped -> archived persists, the
// shipped event is present in the item JSONL, it is ordered before the archival
// records, and the append now flows through appendShipmentEventErr.
func TestAppendShipmentEventErr_SuccessOrderingOnShipPath(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	require.NotNil(t, ws.Config)
	ws.Config.DurableWrites = false

	feature, err := CreateArtifact(ctx, ws, "Ordering covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := CreateArtifact(ctx, ws, "Ordering task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	shipment, err := CreateShipment(ctx, ws, "Ordering shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	routedThroughAppender := false
	ws.shipmentEventAppend = func(seamCtx context.Context, seamWS *Workspace, itemID, eventType string, delta map[string]any) error {
		if itemID == shipment.ID && eventType == "shipment_status_changed" && delta["status"] == string(ShipmentShipped) {
			routedThroughAppender = true
			current, loadErr := loadArtifact(seamCtx, seamWS, shipment.ID)
			require.NoError(t, loadErr)
			assert.Equal(t, models.StatusShipped, current.Status,
				"the shipment status must be persisted before the shipped event append runs")
		}
		return appendShipmentEventErr(seamCtx, seamWS, itemID, eventType, delta)
	}

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, routedThroughAppender, "the shipped event must flow through the shipment appender dispatcher")

	archivedShipment, err := findArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusArchived, archivedShipment.Status)
	assert.Equal(t, string(ShipmentShipped), archivedShipment.ArchivedStatus)

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	itemEvents, err := events.ReadAllEvents(ctx, logsDir, shipment.ID)
	require.NoError(t, err)

	shippedIndex := -1
	archivedIndex := -1
	for i, event := range itemEvents {
		switch event.EventType {
		case "shipment_status_changed":
			if status, ok := event.Delta["status"].(string); ok && status == string(ShipmentShipped) && shippedIndex < 0 {
				shippedIndex = i
			}
		case "archived":
			if archivedIndex < 0 {
				archivedIndex = i
			}
		}
	}
	require.GreaterOrEqual(t, shippedIndex, 0, "the shipped event must be present in the item JSONL")
	require.GreaterOrEqual(t, archivedIndex, 0, "the archival record must be present in the item JSONL")
	assert.Less(t, shippedIndex, archivedIndex, "the shipped event must be ordered before the archival record")
}

// Review follow-up (143.002-T): an item ID that would resolve its log outside the
// workspace storage root is refused before any lock or write, and is tagged
// not-applied so the governed classifier may safely compensate.
func TestAppendShipmentEventErr_RefusesUncontainedItemID(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	err := appendShipmentEventErr(ctx, ws, filepath.Join("..", "..", "escape"), "shipment_status_changed", map[string]any{
		"status": string(ShipmentShipped),
	})

	require.Error(t, err)
	assert.True(t, blerrors.IsWriteNotApplied(err),
		"a refused, never-attempted append is proven not-applied, got: %v", err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
	assert.Contains(t, err.Error(), "outside the workspace storage root")
}
