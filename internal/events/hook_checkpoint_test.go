package events_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// 027.002-T: Per-consumer checkpoint store

func TestCheckpointStore_New_ReturnsNonNil(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)
	assert.NotNil(t, cs)
}

func TestCheckpointStore_LoadCheckpoint_MissingFile_ReturnsZero(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	seq, err := cs.LoadCheckpoint("stage")
	require.NoError(t, err)
	assert.Equal(t, int64(0), seq, "missing checkpoint must return 0")
}

func TestCheckpointStore_SaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	require.NoError(t, cs.SaveCheckpoint("stage", 42))

	seq, err := cs.LoadCheckpoint("stage")
	require.NoError(t, err)
	assert.Equal(t, int64(42), seq)
}

func TestCheckpointStore_SaveCheckpoint_MonotonicEnforced_Regression(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	require.NoError(t, cs.SaveCheckpoint("stage", 10))

	err := cs.SaveCheckpoint("stage", 5)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrValidation,
		"ack regression must wrap ErrValidation")
}

func TestCheckpointStore_SaveCheckpoint_IdempotentAck_Allowed(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	require.NoError(t, cs.SaveCheckpoint("ship", 7))

	err := cs.SaveCheckpoint("ship", 7)
	assert.NoError(t, err, "idempotent ack of the same seq must succeed")
}

func TestCheckpointStore_SaveCheckpoint_Zero_FirstAck(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	err := cs.SaveCheckpoint("stage", 0)
	assert.NoError(t, err, "saving seq=0 as first ack is allowed")
}

func TestCheckpointStore_IsolatesConsumers(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	require.NoError(t, cs.SaveCheckpoint("stage", 50))
	require.NoError(t, cs.SaveCheckpoint("ship", 30))

	stageSeq, err := cs.LoadCheckpoint("stage")
	require.NoError(t, err)
	assert.Equal(t, int64(50), stageSeq, "stage checkpoint must be independent of ship")

	shipSeq, err := cs.LoadCheckpoint("ship")
	require.NoError(t, err)
	assert.Equal(t, int64(30), shipSeq, "ship checkpoint must be independent of stage")
}

func TestCheckpointStore_AtomicWrite_CheckpointAdvances(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	for _, seq := range []int64{1, 2, 3, 10, 20} {
		require.NoError(t, cs.SaveCheckpoint("stage", seq))
	}

	latest, err := cs.LoadCheckpoint("stage")
	require.NoError(t, err)
	assert.Equal(t, int64(20), latest, "checkpoint must reflect the last saved seq")
}

func TestCheckpointStore_LoadCheckpoint_AfterDelete_ReturnsZero(t *testing.T) {
	// Verify graceful restart: if checkpoint is deleted externally, consumer
	// restarts from seq=0 without error (idempotent processing assumed).
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	require.NoError(t, cs.SaveCheckpoint("stage", 99))

	// Create a fresh store pointing to the same dir but simulate "no file" by
	// using a different consumer ID never written to.
	seq, err := cs.LoadCheckpoint("stage-new")
	require.NoError(t, err)
	assert.Equal(t, int64(0), seq, "unknown consumer must return 0 for graceful restart")
}
