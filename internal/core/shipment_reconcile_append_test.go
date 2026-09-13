package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// 167.007-T behavior harness for appendShipmentReconcileEvent (167.003-T
// panic-body declaration): the always-fsync durable event append primitive
// that runs UNDER the caller's already-held item-log lock (it never
// acquires the lock itself).

func TestAppendShipmentReconcileEvent_AppendsDurablyAndIndexes(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	itemID := "167.007-T-item"
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), itemID)

	require.NoError(t, appendShipmentReconcileEvent(ctx, ws, itemID, raw))

	got, err := os.ReadFile(events.LogPathForItem(logsDir, itemID))
	require.NoError(t, err)
	assert.Equal(t, string(raw), string(got))

	// Validate-before-index: the indexed row must exist too.
	evs, err := events.ReadAllEvents(ctx, logsDir, itemID)
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, EventShipmentReconciledShipped, evs[0].EventType)
}

func TestAppendShipmentReconcileEvent_AppendsAfterExistingEvents(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	itemID := "167.007-T-append-second"
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Seed one pre-existing, unrelated but well-formed event line.
	firstEvent := events.Event{ItemID: itemID, EventType: "comment", Delta: map[string]any{"comment": "hi"}}
	seedWriter := events.NewEventWriter(logsDir)
	require.NoError(t, seedWriter.AppendEvent(ctx, firstEvent))

	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), itemID)
	require.NoError(t, appendShipmentReconcileEvent(ctx, ws, itemID, raw))

	evs, err := events.ReadAllEvents(ctx, logsDir, itemID)
	require.NoError(t, err)
	require.Len(t, evs, 2)
	assert.Equal(t, "comment", evs[0].EventType)
	assert.Equal(t, EventShipmentReconciledShipped, evs[1].EventType)
}

func TestAppendShipmentReconcileEvent_RejectsPartialTrailingLine(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	itemID := "167.007-T-partial-tail"
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Simulate a prior crash mid-write: a trailing line with NO newline.
	logPath := events.LogPathForItem(logsDir, itemID)
	require.NoError(t, os.WriteFile(logPath, []byte(`{"item_id":"167.007-T-partial-tail","event_type":"comment"`), 0o644))

	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), itemID)
	err := appendShipmentReconcileEvent(ctx, ws, itemID, raw)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err), "a partial trailing line must be indeterminate, never blindly concatenated onto, got: %v", err)

	// The file must be untouched (no concatenation).
	got, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	assert.Equal(t, `{"item_id":"167.007-T-partial-tail","event_type":"comment"`, string(got))
}

func TestAppendShipmentReconcileEvent_AlwaysFsyncsRegardlessOfDurableWritesConfig(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	require.NotNil(t, ws.Config)
	ws.Config.DurableWrites = false // workspace durable_writes OFF

	itemID := "167.007-T-always-fsync"
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), itemID)
	// This primitive must succeed and durably append regardless of the
	// workspace's durable_writes flag: it is an unconditional-fsync
	// primitive, decoupled from that config (the append precedes/represents
	// the governed transaction's own audit record and cannot be optional).
	require.NoError(t, appendShipmentReconcileEvent(ctx, ws, itemID, raw))

	got, err := os.ReadFile(events.LogPathForItem(logsDir, itemID))
	require.NoError(t, err)
	assert.Equal(t, string(raw), string(got))
}

func TestAppendShipmentReconcileEvent_ValidatesInputs(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "167.007-T")

	assert.Error(t, appendShipmentReconcileEvent(ctx, nil, "167.007-T", raw), "a nil workspace must be refused")
	assert.Error(t, appendShipmentReconcileEvent(ctx, ws, "", raw), "an empty item id must be refused")
	assert.Error(t, appendShipmentReconcileEvent(ctx, ws, "167.007-T", nil), "empty event bytes must be refused")
}

func TestAppendShipmentReconcileEvent_RejectsMalformedEventBytes(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	itemID := "167.007-T-malformed"
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Malformed: not even valid JSON.
	err := appendShipmentReconcileEvent(ctx, ws, itemID, []byte("not json\n"))
	require.Error(t, err)

	// Nothing must have been written.
	_, statErr := os.Stat(events.LogPathForItem(logsDir, itemID))
	assert.True(t, os.IsNotExist(statErr), "a malformed event must never be appended")
}

func TestAppendShipmentReconcileEvent_IndexFailureDoesNotUnreconcile(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	itemID := "167.007-T-index-fail"
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Close the DB to force IndexEvent to fail while the durable append
	// itself has already succeeded and been validated.
	require.NoError(t, ws.DB.Close())

	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), itemID)
	err := appendShipmentReconcileEvent(ctx, ws, itemID, raw)
	require.NoError(t, err, "an index failure after a validated durable append must NOT un-reconcile: the JSONL is the source of truth")

	got, readErr := os.ReadFile(events.LogPathForItem(logsDir, itemID))
	require.NoError(t, readErr)
	assert.Equal(t, string(raw), string(got))
}

// TestAppendShipmentReconcileEventImplWithSeams_ValidatesPrimitivesOwnReadBack
// proves the 167.007-T TOCTOU fix directly: the portable orchestration must
// validate/index EXACTLY the readBack bytes the (seam-injected)
// handle-relative append primitive itself durably re-read from the SAME
// handle it wrote through, never bytes derived from a fresh pathname
// re-open of logPath. The stub primitive here writes SWAPPED bytes onto
// logPath's actual on-disk file (simulating an ancestor/leaf swap that
// happened between the primitive's own append and any later pathname-based
// re-open) while returning the ORIGINAL event bytes as its readBack. If the
// orchestration re-opened logPath by pathname instead of trusting readBack,
// it would see the swapped bytes and either fail validation or index the
// wrong content; with the fix, it must succeed and index exactly the
// original event.
func TestAppendShipmentReconcileEventImplWithSeams_ValidatesPrimitivesOwnReadBack(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	itemID := "167.007-T-readback-not-pathname"
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), itemID)
	const swappedContent = "swapped-out-bytes-not-the-durable-event\n"

	seams := shipmentReconcileAppendSeams{
		appendHandleRelative: func(dir, fileName string, eventBytes []byte) (shipmentReconcileAppendResult, error) {
			// Simulate the on-disk file at this path being something OTHER
			// than what the (real) handle-relative primitive durably wrote,
			// as would happen if an attacker swapped the leaf/ancestor
			// between the primitive's own append and a subsequent pathname
			// re-open.
			if err := os.WriteFile(filepath.Join(dir, fileName), []byte(swappedContent), 0o644); err != nil {
				return shipmentReconcileAppendResult{}, err
			}
			return shipmentReconcileAppendResult{
				preAppendSize: 0,
				bytesWritten:  len(eventBytes),
				readBack:      append([]byte(nil), eventBytes...),
			}, nil
		},
	}

	err := appendShipmentReconcileEventImplWithSeams(ctx, ws, itemID, raw, seams)
	require.NoError(t, err, "orchestration must validate/index the primitive's own readBack, not re-open logPath by pathname")

	logPath := events.LogPathForItem(logsDir, itemID)
	onDisk, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	assert.Equal(t, swappedContent, string(onDisk), "precondition: the on-disk bytes must actually differ from the durable event")

	// The indexed event must reflect the readBack (the original valid
	// event), never the swapped on-disk content: a pathname re-read of the
	// swapped bytes is not valid JSON for this event shape and would have
	// failed to index (or would have indexed garbage) had readBack not been
	// used.
	var eventType, deltaJSON string
	queryErr := ws.DB.QueryRowContext(ctx,
		`SELECT event_type, delta_json FROM item_log_entries WHERE item_id = ?`, itemID,
	).Scan(&eventType, &deltaJSON)
	require.NoError(t, queryErr, "the durably-validated readBack event must have been indexed under itemID")
	assert.Equal(t, EventShipmentReconciledShipped, eventType)
	assert.Contains(t, deltaJSON, "governed repair", "the indexed delta must reflect the original (readBack) event content, not the swapped on-disk bytes")
}
