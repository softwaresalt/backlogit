package hooks

import (
	"context"
	"log/slog"
	"time"
)

// allHookPoints lists every HookPoint that carries mutation semantics.
// RegisterEmitHookEvent and RegisterLogIndexStale iterate over this slice.
var allHookPoints = []HookPoint{
	HookCreateArtifact,
	HookUpdateArtifact,
	HookArchiveItem,
	HookShipShipment,
	HookAdoptItem,
	HookMoveShipmentStatus,
}

// HookEventPayload is a compact, versioned event payload optimized for
// agent context windows. It captures the essential mutation data without
// duplicating full OldValues/NewValues maps.
type HookEventPayload struct {
	SchemaVersion int          `json:"schema_version"`
	EventType     string       `json:"event_type"`
	ItemID        string       `json:"item_id"`
	ArtifactType  string       `json:"artifact_type"`
	Actor         string       `json:"actor"`
	Timestamp     time.Time    `json:"timestamp"`
	ChangedFields []string     `json:"changed_fields,omitempty"`
	StatusDelta   *StatusDelta `json:"status_delta,omitempty"`
	TitleDelta    *StringDelta `json:"title_delta,omitempty"`
}

// StatusDelta records a status transition.
type StatusDelta struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// StringDelta records a string field change.
type StringDelta struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// HookEventAppender is the interface for appending hook events.
// It decouples the hooks package from the events package to avoid import
// cycles. The concrete HookEventWriter in internal/events satisfies this
// interface via an adapter registered from core during NewWorkspace.
type HookEventAppender interface {
	AppendEvent(ctx context.Context, event HookEventPayload) error
}

// RegisterEmitHookEvent registers an emit_hook_event post-hook on every
// mutation HookPoint. The hook fires at priority 50 and skips non-top-level
// calls to prevent duplicate events from nested operations
// (e.g. ShipShipment → ArchiveItem).
func RegisterEmitHookEvent(runner *HookRunner, appender HookEventAppender) {
	for _, point := range allHookPoints {
		p := point
		runner.Register(p, PhasePost, HookRegistration{
			Name:     "emit_hook_event",
			Priority: 50,
			Fn:       emitHookEventFn(appender, p),
		})
	}
}

// emitHookEventFn builds the HookFunc for a specific point with the appender
// and event type baked into the closure at registration time.
func emitHookEventFn(appender HookEventAppender, point HookPoint) HookFunc {
	return func(ctx context.Context, hc HookContext) error {
		if !hc.TopLevel {
			return nil
		}

		payload := HookEventPayload{
			SchemaVersion: 1,
			EventType:     string(point),
			ItemID:        hc.ItemID,
			ArtifactType:  hc.ArtifactType,
			Actor:         hc.Actor,
			Timestamp:     time.Now().UTC(),
		}

		// Build ChangedFields from the keys present in NewValues.
		for k := range hc.NewValues {
			payload.ChangedFields = append(payload.ChangedFields, k)
		}

		// Extract status delta when the status field changed.
		if newStatus, ok := hc.NewValues["status"].(string); ok {
			oldStatus, _ := hc.OldValues["status"].(string)
			if oldStatus != newStatus {
				payload.StatusDelta = &StatusDelta{From: oldStatus, To: newStatus}
			}
		}

		// Extract title delta when the title field changed.
		if newTitle, ok := hc.NewValues["title"].(string); ok {
			oldTitle, _ := hc.OldValues["title"].(string)
			if oldTitle != newTitle {
				payload.TitleDelta = &StringDelta{From: oldTitle, To: newTitle}
			}
		}

		return appender.AppendEvent(ctx, payload)
	}
}

// LogIndexStale returns a post-hook HookFunc that emits a slog.Info message
// signaling that the SQLite index may be stale after a Markdown mutation.
// This is informational; actual index synchronisation happens within the
// lifecycle methods themselves.
func LogIndexStale() HookFunc {
	return func(ctx context.Context, hc HookContext) error {
		slog.InfoContext(ctx, "index may be stale after mutation",
			"item_id", hc.ItemID,
			"artifact_type", hc.ArtifactType,
		)
		return nil
	}
}

// RegisterLogIndexStale registers the log_index_stale post-hook on every
// mutation HookPoint. The hook fires at priority 90 and is informational only.
func RegisterLogIndexStale(runner *HookRunner) {
	for _, point := range allHookPoints {
		runner.Register(point, PhasePost, HookRegistration{
			Name:     "log_index_stale",
			Priority: 90,
			Fn:       LogIndexStale(),
		})
	}
}
