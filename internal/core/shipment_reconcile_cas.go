package core

import (
	"context"
	"errors"
	"fmt"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// guardArchivedStatusUnchangedSince returns a persistArtifactWithGuard guard
// closure that re-reads itemID's on-disk archived_status AFTER the caller's
// artifact-mutation lock (B) is already held, and rejects the persist if it
// has changed since preLockArchivedStatus was captured — the value the
// caller observed via its OWN pre-lock findArtifact read, before any lock
// was acquired.
//
// This closes the stale-snapshot clobber window every snapshot-before-lock
// artifact writer shares: AssociateCommit (commits.go), AddDependency /
// RemoveDependency (dependencies.go), AddArtifactLink (artifacts.go), and
// any future persistArtifact caller that reads the artifact via findArtifact
// BEFORE persistArtifact itself acquires lock B. Without this guard, such a
// writer's in-memory artifact object still carries a stale archived_status
// (e.g. "active") even after a concurrent governed reconciliation has
// already committed archived_status:"shipped" under the SAME lock B and
// released it — the writer's persist would then silently overwrite the
// shipped status back to its own stale value the instant its turn for lock
// B arrives (167.019-T).
//
// It is a REJECT (compare-and-swap failure), not a merge: a persist that
// observes a changed archived_status returns blerrors.ErrShipmentConflict
// (an existing, general "shipment status conflict" sentinel — reused here
// rather than adding a new one for a compatible new scenario) and performs
// NO write. Callers surface this to their own operation's error path exactly
// like any other persist failure; none of AssociateCommit, AddDependency,
// RemoveDependency, or AddArtifactLink has a defined way to "merge" a
// concurrent shipment-lifecycle transition into their own semantics, so
// rejecting and letting the caller observe/retry is the safe default.
//
// A nil-returning findArtifact error other than "not found" is surfaced
// as-is (the guard cannot prove absence of a conflict when the current state
// cannot be read at all). A "not found" result IS treated as a conflict: the
// artifact disappeared between the caller's pre-lock read and this re-read
// under the held lock, and letting the guard pass in that case would let
// persistArtifactWithGuard continue with its stale in-memory artifact object
// and write/upsert it back to disk — silently resurrecting an artifact that
// was legitimately deleted in the interim. A disappearance is itself an
// unobserved change to the item's state and must fail closed exactly like a
// changed archived_status.
func guardArchivedStatusUnchangedSince(ws *Workspace, itemID string, preLockArchivedStatus string) func(context.Context) error {
	return func(ctx context.Context) error {
		current, err := findArtifact(ctx, ws, itemID)
		if err != nil {
			if errors.Is(err, blerrors.ErrNotFound) {
				return fmt.Errorf(
					"%s: artifact no longer exists (was %q) since the pre-lock snapshot: %w",
					itemID, preLockArchivedStatus, blerrors.ErrShipmentConflict)
			}
			return fmt.Errorf("guard %s: re-read artifact under lock: %w", itemID, err)
		}
		if current.ArchivedStatus != preLockArchivedStatus {
			return fmt.Errorf(
				"%s: archived_status changed from %q to %q since the pre-lock snapshot: %w",
				itemID, preLockArchivedStatus, current.ArchivedStatus, blerrors.ErrShipmentConflict)
		}
		return nil
	}
}
