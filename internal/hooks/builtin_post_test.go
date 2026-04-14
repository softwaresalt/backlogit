package hooks_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/backlogit/backlogit/internal/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAppender captures appended events and optionally returns a fixed error.
type mockAppender struct {
	events []hooks.HookEventPayload
	err    error
}

func (m *mockAppender) AppendEvent(_ context.Context, event hooks.HookEventPayload) error {
	m.events = append(m.events, event)
	return m.err
}

// TestEmitHookEvent_EmitsPayload verifies that a top-level post-hook fires and
// produces a payload with the correct schema version, event type, and item ID.
func TestEmitHookEvent_EmitsPayload(t *testing.T) {
	runner := hooks.NewHookRunner()
	appender := &mockAppender{}
	hooks.RegisterEmitHookEvent(runner, appender)

	hc := hooks.HookContext{
		ItemID:       "001-T",
		ArtifactType: "task",
		Actor:        "test-agent",
		TopLevel:     true,
		OldValues:    map[string]any{},
		NewValues:    map[string]any{"title": "My Task"},
	}

	runner.FirePost(context.Background(), hooks.HookCreateArtifact, hc)

	require.Len(t, appender.events, 1)
	ev := appender.events[0]
	assert.Equal(t, 1, ev.SchemaVersion)
	assert.Equal(t, "create_artifact", ev.EventType)
	assert.Equal(t, "001-T", ev.ItemID)
	assert.Equal(t, "task", ev.ArtifactType)
	assert.Equal(t, "test-agent", ev.Actor)
	assert.False(t, ev.Timestamp.IsZero())
}

// TestEmitHookEvent_SkipsNonTopLevel verifies that the hook is a no-op when
// TopLevel is false, preventing duplicate events from nested operations.
func TestEmitHookEvent_SkipsNonTopLevel(t *testing.T) {
	runner := hooks.NewHookRunner()
	appender := &mockAppender{}
	hooks.RegisterEmitHookEvent(runner, appender)

	hc := hooks.HookContext{
		ItemID:    "001-T",
		TopLevel:  false,
		OldValues: map[string]any{},
		NewValues: map[string]any{"title": "My Task"},
	}

	runner.FirePost(context.Background(), hooks.HookCreateArtifact, hc)

	assert.Empty(t, appender.events)
}

// TestEmitHookEvent_StatusDelta verifies that a status transition populates
// StatusDelta with the correct From/To values.
func TestEmitHookEvent_StatusDelta(t *testing.T) {
	runner := hooks.NewHookRunner()
	appender := &mockAppender{}
	hooks.RegisterEmitHookEvent(runner, appender)

	hc := hooks.HookContext{
		ItemID:   "001-T",
		TopLevel: true,
		OldValues: map[string]any{
			"status": "queued",
		},
		NewValues: map[string]any{
			"status": "active",
		},
	}

	runner.FirePost(context.Background(), hooks.HookUpdateArtifact, hc)

	require.Len(t, appender.events, 1)
	ev := appender.events[0]
	require.NotNil(t, ev.StatusDelta)
	assert.Equal(t, "queued", ev.StatusDelta.From)
	assert.Equal(t, "active", ev.StatusDelta.To)
}

// TestEmitHookEvent_TitleDelta verifies that a title change populates
// TitleDelta with the correct From/To values.
func TestEmitHookEvent_TitleDelta(t *testing.T) {
	runner := hooks.NewHookRunner()
	appender := &mockAppender{}
	hooks.RegisterEmitHookEvent(runner, appender)

	hc := hooks.HookContext{
		ItemID:   "001-T",
		TopLevel: true,
		OldValues: map[string]any{
			"title": "Old Title",
		},
		NewValues: map[string]any{
			"title": "New Title",
		},
	}

	runner.FirePost(context.Background(), hooks.HookUpdateArtifact, hc)

	require.Len(t, appender.events, 1)
	ev := appender.events[0]
	require.NotNil(t, ev.TitleDelta)
	assert.Equal(t, "Old Title", ev.TitleDelta.From)
	assert.Equal(t, "New Title", ev.TitleDelta.To)
}

// TestEmitHookEvent_ChangedFields verifies that ChangedFields lists every key
// present in NewValues. Map iteration is non-deterministic, so we sort before
// comparing.
func TestEmitHookEvent_ChangedFields(t *testing.T) {
	runner := hooks.NewHookRunner()
	appender := &mockAppender{}
	hooks.RegisterEmitHookEvent(runner, appender)

	hc := hooks.HookContext{
		ItemID:    "001-T",
		TopLevel:  true,
		OldValues: map[string]any{},
		NewValues: map[string]any{
			"title":    "My Task",
			"priority": "high",
		},
	}

	runner.FirePost(context.Background(), hooks.HookUpdateArtifact, hc)

	require.Len(t, appender.events, 1)
	got := make([]string, len(appender.events[0].ChangedFields))
	copy(got, appender.events[0].ChangedFields)
	sort.Strings(got)
	assert.Equal(t, []string{"priority", "title"}, got)
}

// TestEmitHookEvent_AppenderError verifies that an error returned by the
// appender propagates back through the HookFunc. FirePost swallows it via
// slog.Warn in production, so the runner itself does not return an error.
// We confirm the appender was called exactly once (the fn ran to completion)
// and that FirePost did not panic.
func TestEmitHookEvent_AppenderError(t *testing.T) {
	runner := hooks.NewHookRunner()
	appender := &mockAppender{err: errors.New("storage unavailable")}
	hooks.RegisterEmitHookEvent(runner, appender)

	hc := hooks.HookContext{
		ItemID:    "001-T",
		TopLevel:  true,
		OldValues: map[string]any{},
		NewValues: map[string]any{"title": "x"},
	}

	// FirePost swallows the error; confirm no panic and the appender was reached.
	runner.FirePost(context.Background(), hooks.HookCreateArtifact, hc)

	assert.Len(t, appender.events, 1, "appender should have been called once")
}

// TestLogIndexStale verifies that the log_index_stale hook fires without error.
func TestLogIndexStale(t *testing.T) {
	runner := hooks.NewHookRunner()
	hooks.RegisterLogIndexStale(runner)

	hc := hooks.HookContext{
		ItemID:       "001-T",
		ArtifactType: "task",
		TopLevel:     true,
		OldValues:    map[string]any{},
		NewValues:    map[string]any{},
	}

	// LogIndexStale is informational only; reaching this line without panic is success.
	runner.FirePost(context.Background(), hooks.HookCreateArtifact, hc)
}
