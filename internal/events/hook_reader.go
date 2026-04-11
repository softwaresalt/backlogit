package events

import (
	"context"
	"fmt"
)

// DerivedSignalProvider computes ephemeral signals at poll time.
// The concrete implementation queries the SQLite index; this interface keeps
// the reader independent of database access.
// A nil provider is valid and causes derived signals to be skipped.
type DerivedSignalProvider interface {
	DerivedSignals(ctx context.Context) ([]HookEvent, error)
}

// PollResult separates ackable queued events from ephemeral derived signals.
type PollResult struct {
	// Events contains sequenced, ackable events from the queue with seq strictly
	// greater than the consumer's current checkpoint. These events should be
	// processed and then acked via AckHookEvents.
	Events []HookEvent

	// DerivedSignals contains ephemeral, non-ackable signals computed at poll time.
	// Seq is always 0 for derived signals; they must not be passed to AckHookEvents.
	DerivedSignals []HookEvent
}

// PollHookEvents returns new events since the consumer's last checkpoint plus any
// derived signals from the provider.
// Events are filtered to those with seq strictly greater than the consumer's checkpoint.
// Derived signals always carry Seq=0 and are excluded from the ack stream.
// If provider is nil, DerivedSignals is empty.
func PollHookEvents(
	ctx context.Context,
	w *HookEventWriter,
	cs *CheckpointStore,
	consumerID string,
	provider DerivedSignalProvider,
) (*PollResult, error) {
	checkpoint, err := cs.LoadCheckpoint(consumerID)
	if err != nil {
		return nil, fmt.Errorf("poll: load checkpoint for %q: %w", consumerID, err)
	}

	all, err := w.ReadHookEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("poll: read events: %w", err)
	}

	var newEvents []HookEvent
	for _, ev := range all {
		if ev.Seq > checkpoint {
			newEvents = append(newEvents, ev)
		}
	}

	result := &PollResult{Events: newEvents}

	if provider != nil {
		derived, provErr := provider.DerivedSignals(ctx)
		if provErr != nil {
			return nil, fmt.Errorf("poll: derived signals: %w", provErr)
		}
		for i := range derived {
			derived[i].Seq = 0
		}
		result.DerivedSignals = derived
	}

	return result, nil
}

// AckHookEvents advances the consumer's checkpoint to seq, confirming that all
// events up to and including seq have been processed.
// This is a thin wrapper around CheckpointStore.SaveCheckpoint.
func AckHookEvents(ctx context.Context, cs *CheckpointStore, consumerID string, seq int64) error {
	_ = ctx
	if err := cs.SaveCheckpoint(consumerID, seq); err != nil {
		return fmt.Errorf("ack: save checkpoint for %q: %w", consumerID, err)
	}
	return nil
}
