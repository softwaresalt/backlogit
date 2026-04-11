package events_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/events"
)

// 027.001-T: HookEvent model and append-only HookEventWriter

func TestHookEventWriter_New_ReturnsNonNil(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	assert.NotNil(t, w)
}

func TestHookEventWriter_AppendHookEvent_AssignsMonotonicSeq(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	seq1, err := w.AppendHookEvent(ctx, events.HookEvent{
		EventType: events.HookEventBlockedStale,
		Payload:   map[string]any{"item_id": "001-T"},
	})
	require.NoError(t, err)

	seq2, err := w.AppendHookEvent(ctx, events.HookEvent{
		EventType: events.HookEventFeatureReviewReady,
		Payload:   map[string]any{"item_id": "002-F"},
	})
	require.NoError(t, err)

	assert.Equal(t, int64(1), seq1, "first event must be seq=1")
	assert.Equal(t, int64(2), seq2, "second event must be seq=2")
	assert.Greater(t, seq2, seq1, "sequence numbers must be monotonically increasing")
}

func TestHookEventWriter_AppendHookEvent_PersistsToJSONL(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	_, err := w.AppendHookEvent(ctx, events.HookEvent{
		EventType: events.HookEventPostMergeClosure,
		Payload:   map[string]any{"feature_id": "010-F"},
	})
	require.NoError(t, err)

	stored, err := w.ReadHookEvents(ctx)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, events.HookEventPostMergeClosure, stored[0].EventType)
}

func TestHookEventWriter_AppendHookEvent_SetsTimestamp(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)

	_, err := w.AppendHookEvent(context.Background(), events.HookEvent{
		EventType: events.HookEventBlockedStale,
	})
	require.NoError(t, err)

	all, err := w.ReadHookEvents(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.False(t, all[0].Timestamp.IsZero(), "timestamp must be set automatically when zero")
}

func TestHookEventWriter_AppendHookEvent_MultipleEvents_PreservesOrder(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	types := []string{
		events.HookEventBlockedStale,
		events.HookEventFeatureReviewReady,
		events.HookEventPostMergeClosure,
	}
	for _, et := range types {
		_, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: et})
		require.NoError(t, err)
	}

	all, err := w.ReadHookEvents(ctx)
	require.NoError(t, err)
	require.Len(t, all, 3)
	for i, et := range types {
		assert.Equal(t, et, all[i].EventType)
	}
}

func TestHookEventWriter_AppendHookEvent_ConcurrentAccess_UniqueSeqs(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	const n = 10
	type result struct {
		seq int64
		err error
	}
	results := make(chan result, n)

	for range n {
		go func() {
			seq, err := w.AppendHookEvent(ctx, events.HookEvent{
				EventType: events.HookEventBlockedStale,
			})
			results <- result{seq, err}
		}()
	}

	seen := make(map[int64]bool)
	for range n {
		r := <-results
		require.NoError(t, r.err)
		assert.False(t, seen[r.seq], "duplicate sequence number %d detected", r.seq)
		seen[r.seq] = true
	}
	assert.Len(t, seen, n, "all goroutines must receive unique sequence numbers")
}

func TestHookEventWriter_ReadHookEvents_EmptyQueue_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)

	result, err := w.ReadHookEvents(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestHookEventWriter_SeqStoredInJSONL(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	seq, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventBlockedStale})
	require.NoError(t, err)

	all, err := w.ReadHookEvents(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, seq, all[0].Seq, "seq stored in JSONL must match returned seq")
}

func TestHookEventV1Constants_NonEmpty(t *testing.T) {
	for _, et := range []string{
		events.HookEventFeatureReviewReady,
		events.HookEventPostMergeClosure,
		events.HookEventBlockedStale,
	} {
		assert.NotEmpty(t, et, "all v1 event type constants must be non-empty strings")
	}
}
