package events_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/events"
)

// 027.003-T: HookEventReader with consumer-scoped polling

// mockDerivedProvider returns a fixed set of derived signals for testing.
type mockDerivedProvider struct {
	signals []events.HookEvent
	err     error
}

func (m *mockDerivedProvider) DerivedSignals(_ context.Context) ([]events.HookEvent, error) {
	return m.signals, m.err
}

func TestPollHookEvents_EmptyQueue_ReturnsEmptyResult(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	cs := events.NewCheckpointStore(dir)

	result, err := events.PollHookEvents(context.Background(), w, cs, "stage", nil)
	require.NoError(t, err)
	assert.Empty(t, result.Events)
	assert.Empty(t, result.DerivedSignals)
}

func TestPollHookEvents_ReturnsEventsAfterCheckpoint(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	cs := events.NewCheckpointStore(dir)
	ctx := context.Background()

	seq1, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventBlockedStale})
	require.NoError(t, err)
	_, err = w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventFeatureReviewReady})
	require.NoError(t, err)

	// Consumer acked seq1; only the second event should appear.
	require.NoError(t, cs.SaveCheckpoint("stage", seq1))

	result, err := events.PollHookEvents(ctx, w, cs, "stage", nil)
	require.NoError(t, err)
	require.Len(t, result.Events, 1)
	assert.Equal(t, events.HookEventFeatureReviewReady, result.Events[0].EventType)
}

func TestPollHookEvents_AllAcked_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	cs := events.NewCheckpointStore(dir)
	ctx := context.Background()

	seq, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventBlockedStale})
	require.NoError(t, err)
	require.NoError(t, cs.SaveCheckpoint("stage", seq))

	result, err := events.PollHookEvents(ctx, w, cs, "stage", nil)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "fully acked queue must return no events")
}

func TestPollHookEvents_NilProvider_EmptyDerivedSignals(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	cs := events.NewCheckpointStore(dir)

	result, err := events.PollHookEvents(context.Background(), w, cs, "stage", nil)
	require.NoError(t, err)
	assert.Empty(t, result.DerivedSignals, "nil provider must produce no derived signals")
}

func TestPollHookEvents_IncludesDerivedSignals(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	cs := events.NewCheckpointStore(dir)
	ctx := context.Background()

	provider := &mockDerivedProvider{signals: []events.HookEvent{
		{EventType: events.HookEventBlockedStale, Payload: map[string]any{"item_id": "001-T"}},
	}}

	result, err := events.PollHookEvents(ctx, w, cs, "stage", provider)
	require.NoError(t, err)
	require.Len(t, result.DerivedSignals, 1)
	assert.Equal(t, events.HookEventBlockedStale, result.DerivedSignals[0].EventType)
}

func TestPollHookEvents_DerivedSignals_SeqAlwaysZero(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	cs := events.NewCheckpointStore(dir)
	ctx := context.Background()

	// Provider returns a signal with a non-zero seq; the reader must reset it.
	provider := &mockDerivedProvider{signals: []events.HookEvent{
		{EventType: events.HookEventBlockedStale, Seq: 99},
	}}

	result, err := events.PollHookEvents(ctx, w, cs, "stage", provider)
	require.NoError(t, err)
	for _, ds := range result.DerivedSignals {
		assert.Equal(t, int64(0), ds.Seq, "all derived signals must carry seq=0")
	}
}

func TestPollHookEvents_DerivedSignalsExcludedFromEvents(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	cs := events.NewCheckpointStore(dir)
	ctx := context.Background()

	provider := &mockDerivedProvider{signals: []events.HookEvent{
		{EventType: events.HookEventBlockedStale},
	}}

	_, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventFeatureReviewReady})
	require.NoError(t, err)

	result, err := events.PollHookEvents(ctx, w, cs, "stage", provider)
	require.NoError(t, err)
	assert.Len(t, result.Events, 1, "queued events in Events field")
	assert.Len(t, result.DerivedSignals, 1, "derived signals in DerivedSignals field")

	for _, e := range result.Events {
		assert.NotEqual(t, int64(0), e.Seq, "ackable events must have non-zero seq")
	}
}

func TestAckHookEvents_AdvancesCheckpoint(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	cs := events.NewCheckpointStore(dir)
	ctx := context.Background()

	seq, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventBlockedStale})
	require.NoError(t, err)

	err = events.AckHookEvents(ctx, cs, "stage", seq)
	require.NoError(t, err)

	// After ack, polling must return empty events.
	result, err := events.PollHookEvents(ctx, w, cs, "stage", nil)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "acked events must not reappear on poll")
}

func TestAckHookEvents_ConsumersIndependent(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	csStage := events.NewCheckpointStore(dir)
	csShip := events.NewCheckpointStore(dir)
	require.NotNil(t, csShip, "ship CheckpointStore must be non-nil")
	ctx := context.Background()

	seq, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventBlockedStale})
	require.NoError(t, err)

	// Stage acks; ship has not yet.
	err = events.AckHookEvents(ctx, csStage, "stage", seq)
	require.NoError(t, err)

	stageResult, err := events.PollHookEvents(ctx, w, csStage, "stage", nil)
	require.NoError(t, err)
	assert.Empty(t, stageResult.Events, "stage already acked; must see no events")

	shipResult, err := events.PollHookEvents(ctx, w, csShip, "ship", nil)
	require.NoError(t, err)
	assert.Len(t, shipResult.Events, 1, "ship has not acked; must see the event")
}
