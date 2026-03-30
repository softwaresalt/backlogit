package events

import (
	"context"
	"sync"
	"time"
)

// TelemetryEntry represents an agent telemetry log entry.
type TelemetryEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}

// TelemetryWriter provides goroutine-safe writes to telemetry.jsonl.
type TelemetryWriter struct {
	path string
	mu   sync.Mutex
}

// NewTelemetryWriter creates a telemetry writer.
func NewTelemetryWriter(path string) *TelemetryWriter {
	return &TelemetryWriter{path: path}
}

// LogTelemetry appends a telemetry entry to telemetry.jsonl.
//
// Worker: Implement JSONL telemetry append.
func (w *TelemetryWriter) LogTelemetry(ctx context.Context, entry TelemetryEntry) error {
	panic("not implemented: Worker: Implement telemetry stream append")
}
