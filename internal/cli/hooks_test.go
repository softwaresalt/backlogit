package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/events"
)

// U2 (079.002-T): `backlogit hooks poll|ack` must mirror the MCP
// poll_hook_events / ack_hook_events tools over the shared events path
// (events.PollHookEvents / events.AckHookEvents). Poll collection fields are
// arrays never null (Rule 3); ack advances the durable consumer checkpoint.

// seedHookEvent appends an event to the workspace hook queue via the same
// events path the MCP surface uses, returning the assigned sequence number.
func seedHookEvent(t *testing.T, root, eventType string, payload map[string]any) int64 {
	t.Helper()
	w := events.NewHookEventWriter(filepath.Join(root, ".backlogit"))
	seq, err := w.AppendHookEvent(context.Background(), events.HookEvent{
		EventType: eventType,
		Payload:   payload,
	})
	require.NoError(t, err)
	return seq
}

// Scenario 1: poll on an empty queue → events is [] not null and derived_signals
// is [] not null, matching handlePollHookEvents.
func TestHooksPoll_EmptyQueue_ArraysNotNull(t *testing.T) {
	root := setupCLIWorkspace(t)
	var payload struct {
		Events         []map[string]any `json:"events"`
		DerivedSignals []map[string]any `json:"derived_signals"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "hooks", "poll", "--consumer-id", "ship")), &payload))
	require.NotNil(t, payload.Events, "events must decode to [] (non-nil), not null")
	require.NotNil(t, payload.DerivedSignals, "derived_signals must decode to [] (non-nil), not null")
	assert.Empty(t, payload.Events)
	assert.Empty(t, payload.DerivedSignals)
}

// Scenario 2: poll returns appended events in monotonic seq order.
func TestHooksPoll_ReturnsEventsInSeqOrder(t *testing.T) {
	root := setupCLIWorkspace(t)
	seq1 := seedHookEvent(t, root, events.HookEventFeatureReviewReady, map[string]any{"feature_id": "001-F"})
	seq2 := seedHookEvent(t, root, events.HookEventPostMergeClosure, map[string]any{"shipment_id": "001-S"})

	var payload struct {
		Events []struct {
			Seq       int64  `json:"seq"`
			EventType string `json:"event_type"`
		} `json:"events"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "hooks", "poll", "--consumer-id", "ship")), &payload))
	require.Len(t, payload.Events, 2)
	assert.Equal(t, seq1, payload.Events[0].Seq)
	assert.Equal(t, seq2, payload.Events[1].Seq)
	assert.Less(t, payload.Events[0].Seq, payload.Events[1].Seq, "events must be in seq order")
}

// Scenario 3: ack --seq N emits {acked_seq:N}, advances the durable checkpoint,
// and a subsequent poll for the same consumer excludes acked events.
func TestHooksAck_AdvancesCheckpoint(t *testing.T) {
	root := setupCLIWorkspace(t)
	_ = seedHookEvent(t, root, events.HookEventFeatureReviewReady, map[string]any{"feature_id": "001-F"})

	var ackResp struct {
		AckedSeq int64 `json:"acked_seq"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "hooks", "ack", "--consumer-id", "ship", "--seq", "1")), &ackResp))
	assert.Equal(t, int64(1), ackResp.AckedSeq)

	var payload struct {
		Events []map[string]any `json:"events"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "hooks", "poll", "--consumer-id", "ship")), &payload))
	assert.Empty(t, payload.Events, "acked events must be excluded from the next poll")
}

// U2: the hooks group and its subcommands are registered and discoverable.
func TestHooks_RegisteredUnderRoot(t *testing.T) {
	root := cli.NewRootCommand()
	var hooksCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "hooks" {
			hooksCmd = sub
			break
		}
	}
	require.NotNil(t, hooksCmd, "root must register the hooks command group")

	names := make([]string, 0)
	for _, sub := range hooksCmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "poll")
	assert.Contains(t, names, "ack")
}
