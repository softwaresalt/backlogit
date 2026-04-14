// Package hooks provides a lifecycle hook system for backlogit operations.
// Hooks run before (pre) or after (post) key artifact and shipment operations,
// allowing callers to inject validation, side effects, or telemetry without
// coupling core logic to those concerns.
package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

// HookPoint identifies a lifecycle operation that can fire hooks.
type HookPoint string

const (
	// HookCreateArtifact fires when an artifact is created.
	HookCreateArtifact HookPoint = "create_artifact"
	// HookUpdateArtifact fires when an artifact is updated.
	HookUpdateArtifact HookPoint = "update_artifact"
	// HookArchiveItem fires when an artifact is archived.
	HookArchiveItem HookPoint = "archive_item"
	// HookShipShipment fires when a shipment is shipped.
	HookShipShipment HookPoint = "ship_shipment"
	// HookAdoptItem fires when an item is adopted under a new parent.
	HookAdoptItem HookPoint = "adopt_item"
	// HookMoveShipmentStatus fires when a shipment status changes.
	HookMoveShipmentStatus HookPoint = "move_shipment_status"
)

// HookPhase indicates whether a hook runs before or after the operation.
type HookPhase string

const (
	// PhasePre indicates the hook runs before the operation.
	PhasePre HookPhase = "pre"
	// PhasePost indicates the hook runs after the operation.
	PhasePost HookPhase = "post"
)

// HookContext carries contextual data about the lifecycle operation.
type HookContext struct {
	// ItemID is the artifact or shipment identifier involved in the operation.
	ItemID string
	// ArtifactType is the type of the artifact (e.g. "task", "feature").
	ArtifactType string
	// OldValues holds the field values before the operation.
	OldValues map[string]any
	// NewValues holds the field values after the operation.
	NewValues map[string]any
	// Actor identifies the agent or user triggering the operation.
	Actor string
	// Workspace is the root path used for safe path resolution.
	Workspace string
	// TopLevel is false when called from within another hooked operation.
	TopLevel bool
}

// HookFunc is the callback signature for hook implementations.
type HookFunc func(ctx context.Context, hc HookContext) error

// HookRegistration associates a named hook function with a priority.
type HookRegistration struct {
	// Name is a human-readable identifier for the hook, used in log messages.
	Name string
	// Priority controls execution order; lower values fire first. Default 100.
	Priority int
	// Fn is the hook callback.
	Fn HookFunc
}

// hookKey indexes registrations by their point and phase combination.
type hookKey struct {
	Point HookPoint
	Phase HookPhase
}

// HookRunner manages hook registrations and dispatches lifecycle hooks.
type HookRunner struct {
	mu    sync.RWMutex
	hooks map[hookKey][]HookRegistration
}

// NewHookRunner creates an empty hook runner.
func NewHookRunner() *HookRunner {
	return &HookRunner{
		hooks: make(map[hookKey][]HookRegistration),
	}
}

// Register adds a hook registration for a specific point and phase.
// Registrations are appended; order within the same priority is unspecified.
func (r *HookRunner) Register(point HookPoint, phase HookPhase, reg HookRegistration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := hookKey{Point: point, Phase: phase}
	r.hooks[key] = append(r.hooks[key], reg)
}

// FirePre snapshots pre-hook registrations under read lock, releases the lock,
// then fires hooks in ascending priority order. The first error stops execution
// and is returned to the caller, preventing the guarded operation from proceeding.
func (r *HookRunner) FirePre(ctx context.Context, point HookPoint, hc HookContext) error {
	snapshot := r.snapshot(point, PhasePre)
	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].Priority < snapshot[j].Priority
	})
	for _, reg := range snapshot {
		if err := reg.Fn(ctx, hc); err != nil {
			return fmt.Errorf("pre-hook %q: %w", reg.Name, err)
		}
	}
	return nil
}

// FirePost snapshots post-hook registrations under read lock, releases the lock,
// then fires hooks in ascending priority order. Errors are logged via slog.Warn
// and never returned; post-hooks must not block the operation's completion.
func (r *HookRunner) FirePost(ctx context.Context, point HookPoint, hc HookContext) {
	snapshot := r.snapshot(point, PhasePost)
	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].Priority < snapshot[j].Priority
	})
	for _, reg := range snapshot {
		if err := reg.Fn(ctx, hc); err != nil {
			slog.Warn("post-hook error (swallowed)", "hook", reg.Name, "point", string(point), "error", err)
		}
	}
}

// snapshot copies the registration slice under read lock then releases the lock
// before returning, so callers can iterate the copy without holding the lock.
func (r *HookRunner) snapshot(point HookPoint, phase HookPhase) []HookRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := hookKey{Point: point, Phase: phase}
	src := r.hooks[key]
	if len(src) == 0 {
		return nil
	}
	cp := make([]HookRegistration, len(src))
	copy(cp, src)
	return cp
}
