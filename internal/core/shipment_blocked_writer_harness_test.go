package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

type urWriterTransition struct {
	name string
	from models.ArtifactStatus
	to   models.ArtifactStatus
}

var urWriterTransitions = []urWriterTransition{
	{name: "active_to_blocked", from: models.StatusActive, to: models.StatusBlocked},
	{name: "blocked_to_queued", from: models.StatusBlocked, to: models.StatusQueued},
	{name: "blocked_to_active", from: models.StatusBlocked, to: models.StatusActive},
	{name: "queued_to_active", from: models.StatusQueued, to: models.StatusActive},
}

func TestUR2_GovernedWriterBoundaryRejectsBypassAndCorrelatesWrites(t *testing.T) {
	t.Run("ordinary_mutation_appends_exact_correlated_evidence", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		artifact, err := CreateArtifact(ctx, ws, "writer envelope evidence", "feature")
		require.NoError(t, err)
		baseline := len(readUREvents(t, ws, artifact.ID))
		const updatedTitle = "writer envelope evidence updated"

		_, err = UpdateArtifact(ctx, ws, artifact.ID, map[string]any{"title": updatedTitle})
		require.NoError(t, err)
		requireNewCorrelatedMutationUR(t, ws, artifact.ID, baseline, "update", "title", updatedTitle)
	})

	writers := []struct {
		name  string
		write func(context.Context, *Workspace, *models.Artifact, string) error
	}{
		{
			name: "public_WriteArtifactFile",
			write: func(_ context.Context, _ *Workspace, artifact *models.Artifact, path string) error {
				return WriteArtifactFile(artifact, path)
			},
		},
		{
			name: "private_generic_persist",
			write: func(ctx context.Context, ws *Workspace, artifact *models.Artifact, _ string) error {
				return persistArtifact(ctx, ws, artifact, false)
			},
		},
	}

	t.Run("lower_writer_surfaces_refuse_block_unblock_and_generic_activation", func(t *testing.T) {
		for _, writer := range writers {
			for _, transition := range urWriterTransitions {
				t.Run(writer.name+"_"+transition.name, func(t *testing.T) {
					ws := setupShipmentWorkspace(t)
					fixture := newURBlockedActiveFixture(t, ws)
					beforeArtifact := prepareURWriterTransition(t, ws, fixture.shipment.ID, transition.from)
					beforeAggregate := snapshotURAggregate(t, ws, beforeArtifact.ID)
					path, err := FindArtifactPath(context.Background(), ws, beforeArtifact.ID)
					require.NoError(t, err)

					attempt := cloneArtifact(beforeArtifact)
					attempt.Status = transition.to
					err = writer.write(context.Background(), ws, attempt, path)
					assert.Error(t, err, "lower writer must refuse the ungoverned %s transition", transition.name)
					requireURAggregateUnchanged(t, ws, beforeAggregate)
				})
			}
			t.Run(writer.name+"_preserves_ordinary_member_queued_to_active", func(t *testing.T) {
				ws := setupShipmentWorkspace(t)
				ctx := context.Background()
				member := newUR2OrdinaryShipmentMember(t, ctx, ws, writer.name)
				path, err := FindArtifactPath(ctx, ws, member.ID)
				require.NoError(t, err)
				attempt := cloneArtifact(member)
				attempt.Status = models.StatusActive
				attempt.UpdatedAt = models.NowUTC()

				require.NoError(t, writer.write(ctx, ws, attempt, path),
					"%s must preserve a legitimate ordinary member transition", writer.name)
				require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, member.ID).Status)
			})
		}
	})
}

func TestUR2_GenericBulkAndCascadePathsRefuseGovernedEdges(t *testing.T) {
	surfaces := []struct {
		name     string
		bulk     bool
		prepare  func(*testing.T, context.Context, *Workspace, string, models.ArtifactStatus) []string
		act      func(*testing.T, context.Context, *Workspace, []string, models.ArtifactStatus) (*BulkUpdateResult, error)
		positive func(*testing.T, context.Context, *Workspace)
	}{
		{
			name: "UpdateArtifact",
			act: func(_ *testing.T, ctx context.Context, ws *Workspace, ids []string, target models.ArtifactStatus) (*BulkUpdateResult, error) {
				_, err := UpdateArtifact(ctx, ws, ids[0], map[string]any{"status": string(target)})
				return nil, err
			},
			positive: func(t *testing.T, ctx context.Context, ws *Workspace) {
				member := newUR2OrdinaryShipmentMember(t, ctx, ws, "generic update")
				updated, err := UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": string(models.StatusActive)})
				require.NoError(t, err)
				require.Equal(t, models.StatusActive, updated.Status)
			},
		},
		{
			name: "setArtifactStatus",
			act: func(_ *testing.T, ctx context.Context, ws *Workspace, ids []string, target models.ArtifactStatus) (*BulkUpdateResult, error) {
				_, err := setArtifactStatus(ctx, ws, ids[0], target, "ungoverned test")
				return nil, err
			},
			positive: func(t *testing.T, ctx context.Context, ws *Workspace) {
				member := newUR2OrdinaryShipmentMember(t, ctx, ws, "generic status move")
				updated, err := setArtifactStatus(ctx, ws, member.ID, models.StatusActive, "ordinary member activation")
				require.NoError(t, err)
				require.Equal(t, models.StatusActive, updated.Status)
			},
		},
		{
			name: "BulkUpdateStatus",
			bulk: true,
			prepare: func(t *testing.T, _ context.Context, ws *Workspace, id string, _ models.ArtifactStatus) []string {
				shipment := loadURCanonicalArtifact(t, ws, id)
				ids := append([]string(nil), NormalizeShipmentItems(shipment)...)
				return append(ids, id)
			},
			act: func(_ *testing.T, ctx context.Context, ws *Workspace, ids []string, target models.ArtifactStatus) (*BulkUpdateResult, error) {
				return BulkUpdateStatus(ctx, ws.DB, ws, ids, string(target))
			},
			positive: func(t *testing.T, ctx context.Context, ws *Workspace) {
				member := newUR2OrdinaryShipmentMember(t, ctx, ws, "bulk update")
				result, err := BulkUpdateStatus(ctx, ws.DB, ws, []string{member.ID}, string(models.StatusActive))
				require.NoError(t, err)
				require.Equal(t, 1, result.Succeeded)
				require.Empty(t, result.Failed)
				require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, member.ID).Status)
			},
		},
		{
			name: "cascadePersistedParentStatuses",
			prepare: func(t *testing.T, ctx context.Context, ws *Workspace, id string, target models.ArtifactStatus) []string {
				feature, err := CreateArtifact(ctx, ws, "synthetic cascade feature", "feature")
				require.NoError(t, err)
				child, err := CreateArtifact(ctx, ws, "synthetic cascade child", "task", WithParent(feature.ID))
				require.NoError(t, err)
				child.ParentID = id
				child.Status = target
				forceURArtifactFixture(t, ws, child)
				return []string{child.ID}
			},
			act: func(_ *testing.T, ctx context.Context, ws *Workspace, ids []string, _ models.ArtifactStatus) (*BulkUpdateResult, error) {
				return nil, cascadePersistedParentStatuses(ctx, ws, ids[0])
			},
			positive: func(t *testing.T, ctx context.Context, ws *Workspace) {
				feature, err := CreateArtifact(ctx, ws, "ordinary cascade feature", "feature")
				require.NoError(t, err)
				member, err := CreateArtifact(ctx, ws, "ordinary cascade member", "task", WithParent(feature.ID))
				require.NoError(t, err)
				_, err = CreateShipment(ctx, ws, "ordinary cascade membership", []string{member.ID})
				require.NoError(t, err)
				member = cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
				member.Status = models.StatusActive
				member.UpdatedAt = models.NowUTC()
				forceURArtifactFixture(t, ws, member)

				require.NoError(t, cascadePersistedParentStatuses(ctx, ws, member.ID))
				require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, feature.ID).Status)
			},
		},
	}

	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			for _, transition := range urWriterTransitions {
				t.Run("shipment_"+transition.name+"_is_refused", func(t *testing.T) {
					ws := setupShipmentWorkspace(t)
					fixture := newURBlockedActiveFixture(t, ws)
					shipment := prepareURWriterTransition(t, ws, fixture.shipment.ID, transition.from)
					operationIDs := []string{shipment.ID}
					if surface.prepare != nil {
						operationIDs = surface.prepare(t, context.Background(), ws, shipment.ID, transition.to)
					}
					before := snapshotURAggregate(t, ws, shipment.ID)
					allowsMemberActivation := surface.bulk &&
						transition.from == models.StatusQueued &&
						transition.to == models.StatusActive

					result, err := surface.act(t, context.Background(), ws, operationIDs, transition.to)
					if !surface.bulk {
						assert.Error(t, err, "generic surface must refuse %s", transition.name)
					} else {
						assert.NoError(t, err,
							"bulk item refusal must be reported through BulkUpdateResult.Failed")
						if assert.NotNil(t, result, "bulk item refusal must return item-level results") {
							assert.NoError(t, result.Err,
								"bulk item refusal must not be reported as a workspace-level failure")
							if allowsMemberActivation {
								memberIDs := NormalizeShipmentItems(shipment)
								assert.Equal(t, len(memberIDs), result.Succeeded,
									"queued-to-active bulk update must report every legitimate member activation as successful")
								assert.ElementsMatch(t, []string{shipment.ID}, result.Failed,
									"queued-to-active bulk update must refuse the shipment item only")
								for _, memberID := range memberIDs {
									assert.NotContains(t, result.Failed, memberID,
										"legitimate member %s must not be reported failed", memberID)
								}
							} else {
								assert.Zero(t, result.Succeeded,
									"refusing one governed blocked transition must make the entire bulk batch all-or-nothing")
								assert.ElementsMatch(t, operationIDs, result.Failed,
									"every bulk batch item must be reported failed when a governed blocked transition is refused")
							}
						}
					}
					if !allowsMemberActivation {
						requireURAggregateUnchanged(t, ws, before)
						return
					}

					after := snapshotURAggregate(t, ws, shipment.ID)
					require.Equal(t, before.ArtifactIDs, after.ArtifactIDs,
						"queued-to-active bulk refusal must preserve aggregate membership")
					require.Equal(t, models.StatusQueued, after.Artifacts[shipment.ID].Status,
						"queued-to-active bulk update must leave the refused shipment queued")
					assertURArtifactEqual(t, before.Artifacts[shipment.ID], after.Artifacts[shipment.ID])

					expectedDatabaseItems := make(map[string]map[string]any, len(before.DatabaseItems))
					for id, item := range before.DatabaseItems {
						expectedDatabaseItems[id] = item
					}
					expectedCanonicalFiles := make(map[string]string, len(before.CanonicalFiles))
					for path, content := range before.CanonicalFiles {
						expectedCanonicalFiles[path] = content
					}
					expectedLocations := make(map[string][]string, len(before.ArtifactLocations))
					for id, locations := range before.ArtifactLocations {
						expectedLocations[id] = locations
					}

					for _, memberID := range NormalizeShipmentItems(shipment) {
						beforeMember := before.Artifacts[memberID]
						afterMember := after.Artifacts[memberID]
						require.NotNil(t, beforeMember)
						require.NotNil(t, afterMember)
						require.Equal(t, models.StatusActive, afterMember.Status,
							"legitimate member %s must be activated", memberID)
						assert.True(t, afterMember.UpdatedAt.After(beforeMember.UpdatedAt),
							"legitimate member %s must be restamped after activation", memberID)

						expectedMember := cloneArtifact(beforeMember)
						expectedMember.Status = models.StatusActive
						expectedMember.UpdatedAt = afterMember.UpdatedAt
						assertURArtifactEqual(t, expectedMember, afterMember)
						expectedDatabaseItems[memberID] = artifactCodecViewUR(t, afterMember)

						for _, path := range before.ArtifactLocations[memberID] {
							delete(expectedCanonicalFiles, path)
						}
						for _, path := range after.ArtifactLocations[memberID] {
							expectedCanonicalFiles[path] = after.CanonicalFiles[path]
						}
						expectedLocations[memberID] = after.ArtifactLocations[memberID]
					}

					require.Equal(t, expectedDatabaseItems, after.DatabaseItems,
						"queued-to-active bulk refusal may update only legitimate member item projections")
					require.Equal(t, expectedCanonicalFiles, after.CanonicalFiles,
						"queued-to-active bulk refusal may update only legitimate member canonical files")
					require.Equal(t, expectedLocations, after.ArtifactLocations,
						"queued-to-active bulk refusal may relocate only legitimate member canonical files")
					for table, rows := range before.DatabaseProjection {
						if table == "items" {
							continue
						}
						require.Equal(t, rows, after.DatabaseProjection[table],
							"queued-to-active bulk refusal must preserve the %s projection", table)
					}
					require.Equal(t, before.EventLogs, after.EventLogs,
						"queued-to-active bulk refusal must not alter event logs")
					require.Equal(t, before.OperationJournals, after.OperationJournals,
						"queued-to-active bulk refusal must not alter operation journals")
				})
			}
			t.Run("ordinary_non_shipment_transition_succeeds", func(t *testing.T) {
				ws := setupShipmentWorkspace(t)
				surface.positive(t, context.Background(), ws)
			})
		})
	}

	t.Run("MoveShipmentStatus", func(t *testing.T) {
		t.Run("non_shipment_id_is_rejected", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			member := newUR2OrdinaryShipmentMember(t, ctx, ws, "shipment-only type enforcement")
			before := snapshotURWorkspace(t, ws)

			err := MoveShipmentStatus(ctx, ws, member.ID, ShipmentAbandoned)
			require.Error(t, err, "shipment-only surface must fail closed for a non-shipment ID")
			requireURAggregateUnchanged(t, ws, before)
		})

		blockedEdges := []urWriterTransition{
			{name: "active_to_blocked", from: models.StatusActive, to: models.StatusBlocked},
			{name: "blocked_to_queued", from: models.StatusBlocked, to: models.StatusQueued},
			{name: "blocked_to_active", from: models.StatusBlocked, to: models.StatusActive},
		}
		for _, transition := range blockedEdges {
			t.Run(transition.name+"_requires_governed_envelope", func(t *testing.T) {
				ws := setupShipmentWorkspace(t)
				fixture := newURBlockedActiveFixture(t, ws)
				shipment := prepareURWriterTransition(t, ws, fixture.shipment.ID, transition.from)
				before := snapshotURAggregate(t, ws, shipment.ID)

				err := MoveShipmentStatus(
					context.Background(),
					ws,
					shipment.ID,
					ShipmentStatus(transition.to),
				)
				require.ErrorIs(t, err, blerrors.ErrShipmentBlockedRequiresEnvelope,
					"MoveShipmentStatus must reject the governed %s bypass with the blocked-envelope sentinel",
					transition.name)
				requireURAggregateUnchanged(t, ws, before)
			})
		}

		t.Run("queued_to_active_remains_claim_only", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			shipment, err := CreateShipment(ctx, ws, "restricted direct activation", nil)
			require.NoError(t, err)
			before := snapshotURAggregate(t, ws, shipment.ID)

			err = MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentActive)
			require.Error(t, err,
				"MoveShipmentStatus must not provide an activation path outside ClaimShipment or confirmed unblock")
			requireURAggregateUnchanged(t, ws, before)
		})

		t.Run("active_to_abandoned_remains_allowed", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			fixture := newURBlockedActiveFixture(t, ws)

			require.NoError(t, MoveShipmentStatus(ctx, ws, fixture.shipment.ID, ShipmentAbandoned),
				"the otherwise-allowed active-to-abandoned transition must remain available")
			require.Equal(t, models.StatusAbandoned,
				loadURCanonicalArtifact(t, ws, fixture.shipment.ID).Status)
		})
	})
}

func TestUR2_OnlyClaimAndConfirmedUnblockMayActivateShipment(t *testing.T) {
	t.Run("create_as_active_is_refused", func(t *testing.T) {
		tests := []struct {
			name string
			act  func(context.Context, *Workspace) (string, error)
		}{
			{
				name: "CreateArtifact_with_active_status",
				act: func(ctx context.Context, ws *Workspace) (string, error) {
					artifact, err := CreateArtifact(ctx, ws, "direct active shipment", "shipment", WithStatus(string(ShipmentActive)))
					if artifact == nil {
						return "", err
					}
					return artifact.ID, err
				},
			},
			{
				name: "CreateShipment_with_active_status",
				act: func(ctx context.Context, ws *Workspace) (string, error) {
					artifact, err := CreateShipment(ctx, ws, "option active shipment", nil, WithStatus(string(ShipmentActive)))
					if artifact == nil {
						return "", err
					}
					return artifact.ID, err
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				ws := setupShipmentWorkspace(t)
				before := snapshotURWorkspace(t, ws)
				id, err := test.act(context.Background(), ws)
				require.Error(t, err, "create-as-active must refuse outside ClaimShipment")
				requireURAggregateUnchanged(t, ws, before)
				if id != "" {
					after := snapshotURWorkspace(t, ws)
					assert.NotContains(t, after.ArtifactLocations, id,
						"error-returning create must not leave a canonical artifact at any governed location")
					assert.NotContains(t, after.DatabaseItems, id,
						"error-returning create must not leave a SQLite projection row")
					assert.Equal(t, before.EventLogs, after.EventLogs,
						"error-returning create must not append an event for the refused ID")
					assert.Equal(t, before.OperationJournals, after.OperationJournals,
						"error-returning create must not create an operation journal for the refused ID")
				}
			})
		}
	})

	t.Run("governed_activation_surfaces_succeed", func(t *testing.T) {
		t.Run("CreateArtifact_task_as_active", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			feature, err := CreateArtifact(ctx, ws, "active-create ordinary feature", "feature")
			require.NoError(t, err)
			member, err := CreateArtifact(
				ctx,
				ws,
				"active-create ordinary member",
				"task",
				WithParent(feature.ID),
				WithStatus(string(models.StatusActive)),
			)
			require.NoError(t, err,
				"create-as-active guard must remain scoped to shipment artifacts")
			require.Equal(t, models.StatusActive, member.Status)
			require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, member.ID).Status)
		})

		t.Run("generic_task_activation", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			member := newUR2OrdinaryShipmentMember(t, ctx, ws, "generic activation")
			updated, err := UpdateArtifact(ctx, ws, member.ID, map[string]any{
				"status": string(models.StatusActive),
			})
			require.NoError(t, err,
				"generic activation guard must remain scoped to shipment artifacts")
			require.Equal(t, models.StatusActive, updated.Status)
		})

		t.Run("ClaimShipment", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			shipment, err := CreateShipment(ctx, ws, "claim activation", nil)
			require.NoError(t, err)
			claimed, err := ClaimShipment(ctx, ws, shipment.ID)
			require.NoError(t, err)
			assert.Equal(t, models.StatusActive, claimed.Status)
		})

		t.Run("confirmed_unblock_to_active", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			fixture := newURBlockedActiveFixture(t, ws)
			_, err := BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{Reason: "temporary"})
			require.NoError(t, err)
			unblocked, err := UnblockShipment(ctx, ws, fixture.shipment.ID, UnblockOptions{
				Target:  ShipmentActive,
				Confirm: true,
			})
			require.NoError(t, err)
			assert.Equal(t, models.StatusActive, unblocked.Status)
		})
	})
}

func newUR2OrdinaryShipmentMember(
	t *testing.T,
	ctx context.Context,
	ws *Workspace,
	name string,
) *models.Artifact {
	t.Helper()

	feature, err := CreateArtifact(ctx, ws, name+" feature", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, name+" member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	_, err = CreateShipment(ctx, ws, name+" membership", []string{member.ID})
	require.NoError(t, err)
	require.Equal(t, models.StatusQueued, member.Status)
	return member
}

func prepareURWriterTransition(
	t *testing.T,
	ws *Workspace,
	shipmentID string,
	status models.ArtifactStatus,
) *models.Artifact {
	t.Helper()

	shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipmentID))
	shipment.Status = status
	shipment.UpdatedAt = time.Date(2026, time.September, 21, 0, 0, 0, 0, time.UTC)
	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	if status == models.StatusBlocked {
		shipment.CustomFields["blocked_reason"] = "writer boundary fixture"
		shipment.CustomFields["blocked_at"] = "2026-09-21T00:00:00Z"
		shipment.CustomFields["blocked_by"] = "writer-harness"
	} else {
		delete(shipment.CustomFields, "blocked_reason")
		delete(shipment.CustomFields, "blocked_at")
		delete(shipment.CustomFields, "blocked_by")
	}
	forceURArtifactFixture(t, ws, shipment)
	return cloneArtifact(loadURCanonicalArtifact(t, ws, shipmentID))
}
