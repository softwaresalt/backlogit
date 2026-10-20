package core

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// setDisposeHook installs disposeAfterGovernedWriteHook for one test and
// restores it with t.Cleanup. Tests using it must not call t.Parallel().
func setDisposeHook(t *testing.T, hook func() error) {
	t.Helper()
	previous := disposeAfterGovernedWriteHook
	disposeAfterGovernedWriteHook = hook
	t.Cleanup(func() { disposeAfterGovernedWriteHook = previous })
}

// forceShipmentState rewrites the shipment preimage into an arbitrary state
// (file and index) so a refusal can be proven against states that no governed
// path produces from a fresh fixture.
func forceShipmentState(t *testing.T, f *disposeFixture, id string, status models.ArtifactStatus, archivedStatus string) {
	t.Helper()
	forced := cloneArtifact(loadURCanonicalArtifact(t, f.ws, id))
	forced.Status = status
	forced.ArchivedStatus = archivedStatus
	forced.UpdatedAt = models.NowUTC()
	forceURArtifactFixture(t, f.ws, forced)
}

// TestDisposeQueuedShipmentState covers the state refusals, live dependents,
// and failure handling of the governed disposition (B-U11 / 192.004-T).
// Tests that set the hook seam do not use t.Parallel().
func TestDisposeQueuedShipmentState(t *testing.T) {
	ctx := context.Background()

	t.Run("HookSeamIsNilInProduction", func(t *testing.T) {
		require.Nil(t, disposeAfterGovernedWriteHook)
	})

	t.Run("StateRefusals", func(t *testing.T) {
		shipmentStates := []struct {
			name           string
			status         models.ArtifactStatus
			archivedStatus string
		}{
			{"active", models.StatusActive, ""},
			{"blocked", models.StatusBlocked, ""},
			{"shipped", models.StatusShipped, ""},
			{"abandoned", models.StatusAbandoned, ""},
			{"archived", models.StatusArchived, "abandoned"},
		}
		for _, tc := range shipmentStates {
			t.Run(tc.name, func(t *testing.T) {
				f := newDisposeFixture(t)
				authorizationRef := f.deliberation(t, "done", f.shipmentID)
				forceShipmentState(t, f, f.shipmentID, tc.status, tc.archivedStatus)
				before := f.fileBytes(t, f.shipmentID)

				got, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, f.opts(authorizationRef))

				require.ErrorIs(t, err, blerrors.ErrShipmentDispositionRefused)
				require.Nil(t, got)
				f.requireUnchanged(t, f.shipmentID, before)
				f.requireNoJournals(t)
			})
		}

		t.Run("not_a_shipment", func(t *testing.T) {
			f := newDisposeFixture(t)
			authorizationRef := f.deliberation(t, "done", f.featureID)
			before := f.fileBytes(t, f.featureID)

			got, err := DisposeQueuedShipment(ctx, f.ws, f.featureID, f.opts(authorizationRef))

			require.ErrorIs(t, err, blerrors.ErrShipmentDispositionRefused)
			require.Nil(t, got)
			f.requireUnchanged(t, f.featureID, before)
		})
	})

	t.Run("LiveDependents", func(t *testing.T) {
		t.Run("non_terminal_dependent_refuses_and_names_dependents", func(t *testing.T) {
			f := newDisposeFixture(t)
			authorizationRef := f.deliberation(t, "done", f.shipmentID)
			require.NoError(t, AddShipmentBlock(ctx, f.ws, f.otherID, f.shipmentID))
			targetBefore := f.fileBytes(t, f.shipmentID)
			dependentBefore := f.fileBytes(t, f.otherID)

			got, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, f.opts(authorizationRef))

			require.ErrorIs(t, err, blerrors.ErrShipmentDispositionRefused)
			require.Nil(t, got)
			require.Contains(t, err.Error(), f.otherID, "the refusal names the live dependents")
			f.requireUnchanged(t, f.shipmentID, targetBefore)
			f.requireUnchanged(t, f.otherID, dependentBefore)
			f.requireNoJournals(t)
		})

		t.Run("terminal_dependent_is_not_live_and_edges_are_not_carried", func(t *testing.T) {
			f := newDisposeFixture(t)
			authorizationRef := f.deliberation(t, "done", f.shipmentID)
			require.NoError(t, AddShipmentBlock(ctx, f.ws, f.otherID, f.shipmentID))
			forceShipmentState(t, f, f.otherID, models.StatusAbandoned, "")
			dependentBefore := f.fileBytes(t, f.otherID)
			opts := f.opts(authorizationRef)
			opts.SupersededBy = f.otherID

			_, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, opts)
			require.NoError(t, err)

			disposed := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
			require.Equal(t, models.StatusAbandoned, disposed.Status)
			require.Equal(t, f.otherID, disposed.CustomFields["superseded_by"])
			require.Empty(t, disposed.Dependencies, "edges are never carried to the successor reference")
			f.requireUnchanged(t, f.otherID, dependentBefore)
		})
	})

	t.Run("FailureHandling", func(t *testing.T) {
		requireNoDispositionKeys := func(t *testing.T, artifact *models.Artifact) {
			t.Helper()
			for _, key := range dispositionProvenanceKeys {
				require.NotContains(t, artifact.CustomFields, key)
			}
		}

		rollbackCases := []struct {
			name string
			err  error
		}{
			{"plain_error", errors.New("injected post-write failure")},
			{"classified_not_applied", fmt.Errorf("post-write: %w", blerrors.ErrWriteNotApplied)},
		}
		for _, tc := range rollbackCases {
			t.Run("not_applied_rolls_back_"+tc.name, func(t *testing.T) {
				f := newDisposeFixture(t)
				authorizationRef := f.deliberation(t, "done", f.shipmentID)
				require.NoError(t, AddArtifactLink(ctx, f.ws, f.shipmentID, f.featureID, "related_to"))
				linksBefore := loadURCanonicalArtifact(t, f.ws, f.shipmentID).Links
				require.NotEmpty(t, linksBefore)
				shipmentBefore := f.fileBytes(t, f.shipmentID)
				memberBefore := map[string][]byte{}
				for _, id := range f.memberIDs {
					memberBefore[id] = f.fileBytes(t, id)
				}
				calls := 0
				setDisposeHook(t, func() error {
					calls++
					return tc.err
				})

				got, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, f.opts(authorizationRef))

				require.Error(t, err)
				require.ErrorIs(t, err, tc.err)
				require.False(t, blerrors.IsWriteIndeterminate(err))
				require.Nil(t, got)
				require.Equal(t, 1, calls, "the hook runs once and the write is not retried")
				reloaded := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
				require.Equal(t, models.StatusQueued, reloaded.Status)
				requireNoDispositionKeys(t, reloaded)
				require.Equal(t, linksBefore, reloaded.Links, "links stay intact")
				f.requireUnchanged(t, f.shipmentID, shipmentBefore)
				for id, want := range memberBefore {
					f.requireUnchanged(t, id, want)
				}
				require.Equal(t, 0, f.abandonedStatusEvents(t, f.shipmentID), "the dispose status event is removed")
				f.requireNoJournals(t)
			})
		}

		t.Run("rollback_failure_keeps_journal_and_returns_partial_error", func(t *testing.T) {
			f := newDisposeFixture(t)
			authorizationRef := f.deliberation(t, "done", f.shipmentID)
			setDisposeHook(t, func() error { return errors.New("injected post-write failure") })
			restoreErr := errors.New("injected restore failure")
			previousRestore := restoreShipmentSnapshotFn
			restoreShipmentSnapshotFn = func(fileSnapshot) error { return restoreErr }
			t.Cleanup(func() { restoreShipmentSnapshotFn = previousRestore })

			got, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, f.opts(authorizationRef))

			require.Nil(t, got)
			var partial *blerrors.MutationPartialError
			require.ErrorAs(t, err, &partial)
			require.ErrorIs(t, err, restoreErr)
			require.ErrorContains(t, err, "injected post-write failure", "both errors are joined")
			records, loadErr := loadShipmentOperationJournals(f.ws)
			require.NoError(t, loadErr)
			require.Len(t, records, 1, "the intent journal stays as recovery evidence")
		})

		t.Run("post_commit_indeterminate_is_not_rolled_back", func(t *testing.T) {
			// G-D: an ErrWriteIndeterminate after the governed write landed means
			// the disposition may be committed. There is no rollback and no retry.
			f := newDisposeFixture(t)
			authorizationRef := f.deliberation(t, "done", f.shipmentID)
			calls := 0
			setDisposeHook(t, func() error {
				calls++
				return fmt.Errorf("post-write fsync: %w", blerrors.ErrWriteIndeterminate)
			})

			_, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, f.opts(authorizationRef))

			require.Error(t, err)
			require.True(t, errors.Is(err, blerrors.ErrWriteIndeterminate))
			require.Equal(t, 1, calls, "no retry over a possibly committed write")

			requireKeptDisposition := func(t *testing.T) {
				t.Helper()
				got := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
				require.Equal(t, models.StatusAbandoned, got.Status, "no compensating rollback")
				require.Equal(t, "superseded by a newer plan", got.CustomFields["disposition_reason"])
				require.Equal(t, "operator", got.CustomFields["disposition_by"])
				require.Equal(t, authorizationRef, got.CustomFields["disposition_authorization_ref"])
				require.Equal(t, f.otherID, got.CustomFields["superseded_by"])
				require.Equal(t, 1, f.abandonedStatusEvents(t, f.shipmentID), "the status event is kept")
			}
			requireKeptDisposition(t)

			// Either the journal was finalized, or it is left as recovery evidence
			// that the next recovery finalizes without restoring queued.
			records, loadErr := loadShipmentOperationJournals(f.ws)
			require.NoError(t, loadErr)
			if len(records) > 0 {
				require.NoError(t, recoverPendingShipmentOperations(ctx, f.ws))
				requireKeptDisposition(t)
			}
			setDisposeHook(t, nil)
			claimed, claimErr := ClaimShipment(ctx, f.ws, f.otherID)
			require.NoError(t, claimErr, "later lifecycle operations proceed after the indeterminate result")
			require.Equal(t, models.StatusActive, claimed.Status)
			requireKeptDisposition(t)
		})
	})
}
