package events

import "context"

// TailEvents reads the most recent events for a specific item from events.jsonl.
//
// Worker: Implement JSONL tail reading with item_id filtering.
func TailEvents(ctx context.Context, path string, itemID string, limit int) ([]Event, error) {
	panic("not implemented: Worker: Implement event tail reader")
}

// ReadAllEvents reads all events from events.jsonl.
//
// Worker: Implement full JSONL file reading.
func ReadAllEvents(ctx context.Context, path string) ([]Event, error) {
	panic("not implemented: Worker: Implement full event reader")
}
