package core

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// disposeFixture is a temp-dir workspace holding a queued shipment (the
// disposition target) with two members under one covering feature, and a
// second queued shipment that a disposition may name as its successor.
// Every disposition test calls DisposeQueuedShipment in process on this
// fixture only (G-B): no test invokes the CLI or MCP surface.
type disposeFixture struct {
	ws         *Workspace
	shipmentID string
	otherID    string
	featureID  string
	memberIDs  []string
}

func newDisposeFixture(t *testing.T) *disposeFixture {
	t.Helper()
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	feature, err := CreateArtifact(ctx, ws, "Dispose feature", "feature")
	require.NoError(t, err)
	memberA, err := CreateArtifact(ctx, ws, "Dispose member A", "task", WithParent(feature.ID))
	require.NoError(t, err)
	memberB, err := CreateArtifact(ctx, ws, "Dispose member B", "task", WithParent(feature.ID))
	require.NoError(t, err)
	otherMember, err := CreateArtifact(ctx, ws, "Dispose other member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Dispose target shipment", []string{memberA.ID, memberB.ID})
	require.NoError(t, err)
	other, err := CreateShipment(ctx, ws, "Dispose successor shipment", []string{otherMember.ID})
	require.NoError(t, err)
	require.Equal(t, models.StatusQueued, shipment.Status)
	return &disposeFixture{
		ws:         ws,
		shipmentID: shipment.ID,
		otherID:    other.ID,
		featureID:  feature.ID,
		memberIDs:  []string{memberA.ID, memberB.ID},
	}
}

// deliberation creates a deliberation with the given status whose
// authorizes_disposition field names target (omitted when target is empty).
func (f *disposeFixture) deliberation(t *testing.T, status, target string) string {
	t.Helper()
	fields := map[string]any{}
	if target != "" {
		fields["authorizes_disposition"] = target
	}
	artifact, err := CreateArtifact(context.Background(), f.ws, "Disposition authorization", "deliberation",
		WithStatus(status), WithFields(fields))
	require.NoError(t, err)
	return artifact.ID
}

// archivedDoneDeliberation creates a done deliberation and archives it, so the
// Markdown carries status archived with archived_status done.
func (f *disposeFixture) archivedDoneDeliberation(t *testing.T, target string) string {
	t.Helper()
	id := f.deliberation(t, "done", target)
	_, err := ArchiveItem(context.Background(), f.ws.DB, f.ws, id)
	require.NoError(t, err)
	archived := loadURCanonicalArtifact(t, f.ws, id)
	require.Equal(t, models.StatusArchived, archived.Status)
	require.Equal(t, "done", archived.ArchivedStatus)
	return id
}

func (f *disposeFixture) opts(authorizationRef string) ShipmentDispositionOptions {
	return ShipmentDispositionOptions{
		Reason:           "superseded by a newer plan",
		SupersededBy:     f.otherID,
		By:               "operator",
		AuthorizationRef: authorizationRef,
		Confirm:          true,
	}
}

func (f *disposeFixture) fileBytes(t *testing.T, id string) []byte {
	t.Helper()
	path, err := FindArtifactPath(context.Background(), f.ws, id)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func (f *disposeFixture) requireUnchanged(t *testing.T, id string, before []byte) {
	t.Helper()
	require.Equal(t, string(before), string(f.fileBytes(t, id)), "artifact %s file must be unchanged", id)
}

func (f *disposeFixture) abandonedStatusEvents(t *testing.T, id string) int {
	t.Helper()
	count := 0
	for _, event := range readUREvents(t, f.ws, id) {
		if event.EventType == "shipment_status_changed" && event.Delta["status"] == "abandoned" {
			count++
		}
	}
	return count
}

func (f *disposeFixture) requireNoJournals(t *testing.T) {
	t.Helper()
	records, err := loadShipmentOperationJournals(f.ws)
	require.NoError(t, err)
	require.Empty(t, records, "no lifecycle journal may remain")
}

var dispositionProvenanceKeys = []string{
	"disposition_reason", "disposition_by", "disposition_authorization_ref", "superseded_by",
}

// TestDisposeQueuedShipment covers success and the input and authorization
// refusals of the governed queued-shipment disposition (B-U10 / 192.003-T).
func TestDisposeQueuedShipment(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		variants := []struct {
			name   string
			create func(*testing.T, *disposeFixture) string
		}{
			{"done_deliberation", func(t *testing.T, f *disposeFixture) string {
				return f.deliberation(t, "done", f.shipmentID)
			}},
			{"archived_done_deliberation", func(t *testing.T, f *disposeFixture) string {
				return f.archivedDoneDeliberation(t, f.shipmentID)
			}},
		}
		for _, variant := range variants {
			t.Run(variant.name, func(t *testing.T) {
				f := newDisposeFixture(t)
				authorizationRef := variant.create(t, f)
				untouched := map[string][]byte{f.featureID: f.fileBytes(t, f.featureID)}
				for _, id := range f.memberIDs {
					untouched[id] = f.fileBytes(t, id)
				}

				got, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, f.opts(authorizationRef))
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, models.StatusAbandoned, got.Status)

				reloaded := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
				require.Equal(t, models.StatusAbandoned, reloaded.Status)
				require.Equal(t, "superseded by a newer plan", reloaded.CustomFields["disposition_reason"])
				require.Equal(t, "operator", reloaded.CustomFields["disposition_by"])
				require.Equal(t, authorizationRef, reloaded.CustomFields["disposition_authorization_ref"])
				require.Equal(t, f.otherID, reloaded.CustomFields["superseded_by"])
				require.Equal(t, 1, f.abandonedStatusEvents(t, f.shipmentID), "exactly one status event")

				for id, want := range untouched {
					f.requireUnchanged(t, id, want)
				}
				f.requireNoJournals(t)

				// The members are assignable to another shipment.
				reassigned, err := CreateShipment(ctx, f.ws, "Replacement shipment", f.memberIDs)
				require.NoError(t, err)
				require.Equal(t, models.StatusQueued, reassigned.Status)
			})
		}

		t.Run("superseded_by_omitted", func(t *testing.T) {
			f := newDisposeFixture(t)
			opts := f.opts(f.deliberation(t, "done", f.shipmentID))
			opts.SupersededBy = ""
			_, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, opts)
			require.NoError(t, err)
			reloaded := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
			require.Equal(t, models.StatusAbandoned, reloaded.Status)
			require.NotContains(t, reloaded.CustomFields, "superseded_by")
		})
	})

	t.Run("InputRefusals", func(t *testing.T) {
		f := newDisposeFixture(t)
		authorizationRef := f.deliberation(t, "done", f.shipmentID)
		cases := []struct {
			name    string
			mutate  func(*ShipmentDispositionOptions)
			wantErr error
		}{
			{"confirm_false", func(o *ShipmentDispositionOptions) { o.Confirm = false }, blerrors.ErrConfirmationRequired},
			{"empty_reason", func(o *ShipmentDispositionOptions) { o.Reason = "" }, blerrors.ErrValidation},
			{"blank_reason", func(o *ShipmentDispositionOptions) { o.Reason = "  \t" }, blerrors.ErrValidation},
			{"empty_by", func(o *ShipmentDispositionOptions) { o.By = "" }, blerrors.ErrValidation},
			{"empty_authorization_ref", func(o *ShipmentDispositionOptions) { o.AuthorizationRef = "" }, blerrors.ErrValidation},
			{"superseded_by_missing", func(o *ShipmentDispositionOptions) { o.SupersededBy = "999-S" }, blerrors.ErrValidation},
			{"superseded_by_not_a_shipment", func(o *ShipmentDispositionOptions) { o.SupersededBy = f.featureID }, blerrors.ErrValidation},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				before := f.fileBytes(t, f.shipmentID)
				opts := f.opts(authorizationRef)
				tc.mutate(&opts)

				got, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, opts)

				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, got)
				f.requireUnchanged(t, f.shipmentID, before)
				f.requireNoJournals(t)
			})
		}
	})

	t.Run("AuthorizationRefusals", func(t *testing.T) {
		cases := []struct {
			name string
			ref  func(*testing.T, *disposeFixture) string
		}{
			{"unresolvable_ref", func(*testing.T, *disposeFixture) string { return "999-DL" }},
			{"malformed_ref", func(*testing.T, *disposeFixture) string { return "not-a-deliberation" }},
			{"non_deliberation_artifact", func(t *testing.T, f *disposeFixture) string {
				id := f.deliberation(t, "done", f.shipmentID)
				retyped := cloneArtifact(loadURCanonicalArtifact(t, f.ws, id))
				retyped.ArtifactType = "task"
				forceURArtifactFixture(t, f.ws, retyped)
				return id
			}},
			{"missing_authorizes_disposition", func(t *testing.T, f *disposeFixture) string {
				return f.deliberation(t, "done", "")
			}},
			{"wrong_authorizes_disposition", func(t *testing.T, f *disposeFixture) string {
				return f.deliberation(t, "done", f.otherID)
			}},
			{"queued_deliberation_naming_target", func(t *testing.T, f *disposeFixture) string {
				return f.deliberation(t, "queued", f.shipmentID)
			}},
			{"archived_but_not_done_deliberation", func(t *testing.T, f *disposeFixture) string {
				id := f.deliberation(t, "queued", f.shipmentID)
				_, err := ArchiveItem(ctx, f.ws.DB, f.ws, id)
				require.NoError(t, err)
				require.NotEqual(t, "done", loadURCanonicalArtifact(t, f.ws, id).ArchivedStatus)
				return id
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				f := newDisposeFixture(t)
				ref := tc.ref(t, f)
				before := f.fileBytes(t, f.shipmentID)

				got, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, f.opts(ref))

				require.ErrorIs(t, err, blerrors.ErrValidation)
				require.Nil(t, got)
				f.requireUnchanged(t, f.shipmentID, before)
				require.Equal(t, models.StatusQueued, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Status)
				f.requireNoJournals(t)
			})
		}
	})
}
