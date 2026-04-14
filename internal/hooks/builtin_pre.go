package hooks

import (
	"context"
	"fmt"

	blerrors "github.com/backlogit/backlogit/internal/errors"
)

// DefaultTransitions returns the default status transition map covering
// all 8 artifact statuses plus shipment-specific statuses (shipped, abandoned).
// This map is used by HookUpdateArtifact pre-hooks. Shipment statuses are
// included because the MCP move_item handler routes all item types through
// UpdateArtifact; shipment-specific lifecycle enforcement happens separately
// in MoveShipmentStatus and ShipShipment.
func DefaultTransitions() map[string][]string {
	return map[string][]string{
		"queued":  {"active", "blocked"},
		"active":  {"done", "blocked", "review", "shipped", "abandoned"},
		"blocked": {"active"},
		"review":  {"done", "accepted", "rejected"},
		"done":    {"archived"},
	}
}

// ValidateStatusTransition returns a pre-hook function that validates status
// transitions against the provided transition map. If transitions is nil or
// empty, DefaultTransitions is used.
//
// The hook fires on HookUpdateArtifact at priority 20. It checks whether
// NewValues contains a "status" field different from OldValues["status"],
// and if so, validates the transition is in the allowed map.
//
// Invalid transitions return an error wrapping ErrInvalidStatusTransition.
// If no status change is present, the hook is a no-op.
func ValidateStatusTransition(transitions map[string][]string) HookFunc {
	if len(transitions) == 0 {
		transitions = DefaultTransitions()
	}
	return func(ctx context.Context, hc HookContext) error {
		newStatus, ok := hc.NewValues["status"].(string)
		if !ok || newStatus == "" {
			return nil // no status change
		}
		oldStatus, _ := hc.OldValues["status"].(string)
		if oldStatus == "" || oldStatus == newStatus {
			return nil // no transition
		}

		allowed, exists := transitions[oldStatus]
		if !exists {
			return fmt.Errorf("status %q has no allowed transitions: %w", oldStatus, blerrors.ErrInvalidStatusTransition)
		}
		for _, a := range allowed {
			if a == newStatus {
				return nil
			}
		}
		return fmt.Errorf("transition from %q to %q is not allowed: %w", oldStatus, newStatus, blerrors.ErrInvalidStatusTransition)
	}
}
