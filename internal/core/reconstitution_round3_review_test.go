package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestRound3ReviewClaimMarkerRollback(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx := context.Background()
	_, err := ClaimShipment(ctx, fx.ws, fx.source.ID)
	require.NoError(t, err)
	before, err := findArtifact(ctx, fx.ws, fx.tasks[0].ID)
	require.NoError(t, err)
	require.Equal(t, fx.source.ID, before.CustomFields[schedulerBaselineClaimKey])
	plan := strings.ReplaceAll(fx.splitPlanYAML(), "    title: Ship ", "    keep_id: true\n    title: Ship ")
	veto := errors.New("source manifest veto")
	called := false
	fx.ws.HookRunner = hooks.NewHookRunner()
	fx.ws.HookRunner.Register(hooks.HookUpdateArtifact, hooks.PhasePre, hooks.HookRegistration{
		Name: "veto-source-after-detach",
		Fn: func(hookCtx context.Context, hc hooks.HookContext) error {
			if hc.ItemID != fx.source.ID {
				return nil
			}
			member, findErr := findArtifact(hookCtx, fx.ws, fx.tasks[0].ID)
			require.NoError(t, findErr)
			require.NotContains(t, member.CustomFields, schedulerBaselineClaimKey, "detach must precede veto")
			called = true
			return veto
		},
	})
	_, err = ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{
		PlanYAML: plan, Force: true, Reason: "review rollback regression",
	})
	require.True(t, called, "source veto must be reached after detach")
	assert.ErrorIs(t, err, veto, "rollback must return the veto without a false outside-op change")
	after, findErr := findArtifact(ctx, fx.ws, fx.tasks[0].ID)
	require.NoError(t, findErr)
	assert.Equal(t, before.Status, after.Status, "rollback must restore member status")
	assert.Equal(t, fx.source.ID, after.CustomFields[schedulerBaselineClaimKey], "rollback must restore scheduler baseline claim")
}

func TestRound3ReviewRelatedOwnerIndependentChanges(t *testing.T) {
	for _, field := range []string{"status", "dependencies"} {
		t.Run(field, func(t *testing.T) {
			fx := newReconstituteFixture(t)
			ctx := context.Background()
			require.NoError(t, AddArtifactLink(ctx, fx.ws, fx.feature.ID, fx.tasks[0].ID, "related_to"))
			st, err := planReconstitution(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
			require.NoError(t, err)
			state, err := captureReconstituteState(ctx, fx.ws, st)
			require.NoError(t, err)
			assert.NotContains(t, state.EventIDs, fx.feature.ID, "RelatedTo is not a direct mutation owner")
			require.Contains(t, state.LockIDs, fx.feature.ID, "reference owner remains locked")
			snapshotted := false
			for _, image := range state.Images {
				snapshotted = snapshotted || reconstituteImageOwner(image) == fx.feature.ID
			}
			require.True(t, snapshotted, "structured reference owner must have a preimage")
			owner, err := findArtifact(ctx, fx.ws, fx.feature.ID)
			require.NoError(t, err)
			switch field {
			case "status":
				owner.Status = models.StatusActive
			case "dependencies":
				owner.Dependencies = append(owner.Dependencies, models.DependencyEdge{ID: fx.downstream.ID, Type: "blocks"})
			}
			require.NoError(t, persistArtifact(ctx, fx.ws, owner, false))
			path, err := FindArtifactPath(ctx, fx.ws, owner.ID)
			require.NoError(t, err)
			before, err := os.ReadFile(path)
			require.NoError(t, err)
			journal := reconstituteRollbackJournal(fx, "0000000000000000000000000000c010")
			journal.Reconstitute = state
			err = rollbackReconstitute(ctx, fx.ws, journal)
			assert.Error(t, err, "rollback must reject an independent RelatedTo owner change")
			after, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, before, after, "rollback must not overwrite the independent "+field)
		})
	}
}

func TestRound3ReviewEventIDsDirectOwnersOnly(t *testing.T) {
	fx := newReconstituteFixture(t)
	st := &reconstitutePlanState{
		sources:     map[string]*models.Artifact{"source": nil, "synthetic:source": nil},
		moves:       map[string]*reconstituteMemberMove{"move": nil, "source": nil, "synthetic:move": nil},
		edgeAdds:    map[string][]string{"add": {"dependency"}, "synthetic:add": nil},
		edgeRemoves: map[string][]string{"remove": {"dependency"}},
		rewireItems: []reconstituteItemRewire{{item: "rewire", blocksOn: []string{"dependency"}}, {item: "synthetic:rewire"}},
		manifests:   map[string][]string{"source": {"manifest-only"}},
		targets: []*reconstituteTarget{
			{key: "existing", existing: &models.Artifact{}, existingFeature: &models.Artifact{}, featureKey: "feature-only",
				items: []string{"item-only"}, adopt: []string{"adopt", "source", "synthetic:adopt"},
				spec: ReconstituteShipmentSpec{KeepID: true}},
			{key: "new-only"},
		},
	}
	state, err := captureReconstituteState(context.Background(), fx.ws, st)
	require.NoError(t, err)
	assert.Equal(t, []string{"add", "adopt", "existing", "move", "remove", "rewire", "source"}, state.EventIDs,
		"event IDs must be sorted, unique direct owners only")
	for _, id := range []string{"manifest-only", "feature-only", "item-only", "dependency", "new-only", "adopt"} {
		assert.Contains(t, state.LockIDs, id, "lock IDs retain affected IDs and snapshot owners")
	}
}

func TestRound3RecoveryPreservesIndependentSize(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		for _, crossReference := range []bool{false, true} {
			t.Run(fmt.Sprintf("scoped=%v/cross_reference=%v", scoped, crossReference), func(t *testing.T) {
				fx := newReconstituteFixture(t)
				ctx := context.Background()
				owner := fx.tasks[0].ID
				if crossReference {
					other, err := CreateArtifact(ctx, fx.ws, "Reference owner", "task", WithParent(fx.feature.ID))
					require.NoError(t, err)
					owner = other.ID
					require.NoError(t, AddArtifactLink(ctx, fx.ws, owner, fx.tasks[0].ID, "related_to"))
				}
				st, err := planReconstitution(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
				require.NoError(t, err)
				state, err := captureReconstituteState(ctx, fx.ws, st)
				require.NoError(t, err)
				state.ScopedImages = scoped
				size := "M"
				_, err = SetArtifactSizeWithProvenance(ctx, fx.ws, owner, SizeMutation{Size: &size, Actor: ActorContextHuman})
				require.NoError(t, err)
				path, err := FindArtifactPath(ctx, fx.ws, owner)
				require.NoError(t, err)
				before, err := os.ReadFile(path)
				require.NoError(t, err)
				corr := "0000000000000000000000000000c009"
				journal := reconstituteRollbackJournal(fx, corr)
				journal.Reconstitute = state
				journalPath, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr), journal)
				require.NoError(t, err)
				_, unlock, err := lockShipmentLifecycleGlobal(ctx, fx.ws)
				if unlock != nil {
					require.NoError(t, unlock())
				}
				require.Error(t, err)
				require.FileExists(t, journalPath)
				after, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, before, after)
			})
		}
	}
}

func TestRound3NewFeatureRelatedToAdoptedTask(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("rollback=%v", fail), func(t *testing.T) {
			fx := newReconstituteFixture(t)
			ctx := context.Background()
			plan := strings.Replace(fx.splitPlanYAML(), "related_to: "+fx.feature.ID, "related_to: "+fx.tasks[0].ID, 1)
			var featureID string
			if fail {
				fx.ws.HookRunner = hooks.NewHookRunner()
				fx.ws.HookRunner.Register(hooks.HookUpdateArtifact, hooks.PhasePre, hooks.HookRegistration{
					Name: "rollback-new-feature",
					Fn: func(context.Context, hooks.HookContext) error {
						return errors.New("rollback new feature")
					},
				})
			}
			result, err := ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{PlanYAML: plan})
			if fail {
				require.ErrorContains(t, err, "rollback new feature")
				// The covering feature has a deterministic title; no matching
				// artifact may remain after compensation.
				refs, scanErr := scanCanonicalArtifacts(fx.ws)
				require.NoError(t, scanErr)
				for id := range refs {
					artifact, findErr := findArtifact(ctx, fx.ws, id)
					require.NoError(t, findErr)
					require.NotEqual(t, "Split feature one", artifact.Title)
				}
				return
			}
			require.NoError(t, err)
			featureID = result.CreatedFeatures[0].ID
			feature, err := findArtifact(ctx, fx.ws, featureID)
			require.NoError(t, err)
			require.Len(t, feature.Links, 1)
			require.Equal(t, result.AdoptedIDMap[fx.tasks[0].ID], feature.Links[0].TargetID)
		})
	}
}

func TestRound3ReconstituteSnapshotOwners(t *testing.T) {
	for _, structured := range []bool{false, true} {
		t.Run(fmt.Sprintf("structured=%v", structured), func(t *testing.T) {
			fx := newReconstituteFixture(t)
			ctx := context.Background()
			other, err := CreateArtifact(ctx, fx.ws, "Independent sizing", "task",
				WithParent(fx.feature.ID),
				WithDescription("Prose mentions "+fx.tasks[0].ID))
			require.NoError(t, err)
			if structured {
				require.NoError(t, AddArtifactLink(ctx, fx.ws, other.ID, fx.tasks[0].ID, "related_to"))
			}
			st, err := planReconstitution(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
			require.NoError(t, err)
			state, err := captureReconstituteState(ctx, fx.ws, st)
			require.NoError(t, err)
			found := false
			for _, image := range state.Images {
				found = found || filepath.Base(image.Path) == other.ID+".md"
			}
			assert.Equal(t, structured, found, "only structured propagation owners belong in preimages")
			if structured {
				assert.Contains(t, state.LockIDs, other.ID, "every preimage owner must be locked")
			}

			size := "M"
			done := make(chan error, 1)
			called := false
			// The hook runs after capture and before mutation; a standalone
			// size writer does not take lifecycle-global, only the owner's B.
			fx.ws.HookRunner = hooks.NewHookRunner()
			fx.ws.HookRunner.Register(hooks.HookAdoptItem, hooks.PhasePre, hooks.HookRegistration{
				Name: "round3-size",
				Fn: func(context.Context, hooks.HookContext) error {
					called = true
					go func() {
						writerCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
						defer cancel()
						_, writeErr := SetArtifactSizeWithProvenance(writerCtx, fx.ws, other.ID,
							SizeMutation{Size: &size, Actor: ActorContextHuman})
						done <- writeErr
					}()
					writeErr := <-done
					if structured {
						assert.Error(t, writeErr, "structured owner writer must serialize across rollback")
					} else {
						assert.NoError(t, writeErr, "prose-only writer must remain independent")
					}
					return errors.New("round3 rollback veto")
				},
			})
			_, err = ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
			require.ErrorContains(t, err, "round3 rollback veto")
			require.True(t, called, "must reach the sizing writer hook")
			if structured {
				_, err = SetArtifactSizeWithProvenance(ctx, fx.ws, other.ID,
					SizeMutation{Size: &size, Actor: ActorContextHuman})
				require.NoError(t, err, "writer proceeds after rollback releases ownership")
			}
			current, err := findArtifact(ctx, fx.ws, other.ID)
			require.NoError(t, err)
			assert.Equal(t, size, current.CustomFields["size"], "rollback must preserve the independent writer")
		})
	}
}

func TestRound3ReconstituteRejectsNewSnapshotOwner(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx := context.Background()
	other, err := CreateArtifact(ctx, fx.ws, "Late reference", "task", WithParent(fx.feature.ID))
	require.NoError(t, err)
	called := false
	fx.ws.HookRunner = hooks.NewHookRunner()
	fx.ws.HookRunner.Register(hooks.HookAdoptItem, hooks.PhasePre, hooks.HookRegistration{
		Name: "round3-late-reference",
		Fn: func(context.Context, hooks.HookContext) error {
			called = true
			return AddArtifactLink(ctx, fx.ws, other.ID, fx.tasks[0].ID, "related_to")
		},
	})
	_, err = ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
	require.True(t, called, "the reference must be added after capture")
	require.ErrorContains(t, err, "was not snapshotted")
	current, err := findArtifact(ctx, fx.ws, other.ID)
	require.NoError(t, err)
	require.Len(t, current.Links, 1)
	assert.Equal(t, fx.tasks[0].ID, current.Links[0].TargetID)
}

func TestRound3ReconstituteRecoveryLocksSnapshotOwner(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx := context.Background()
	other, err := CreateArtifact(ctx, fx.ws, "Recovery reference", "task", WithParent(fx.feature.ID))
	require.NoError(t, err)
	require.NoError(t, AddArtifactLink(ctx, fx.ws, other.ID, fx.tasks[0].ID, "related_to"))
	st, err := planReconstitution(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
	require.NoError(t, err)
	state, err := captureReconstituteState(ctx, fx.ws, st)
	require.NoError(t, err)
	require.Contains(t, state.LockIDs, other.ID)
	state.ScopedImages = false
	// Historical journals did not record cross-reference owners in LockIDs.
	legacyLocks := make([]string, 0, len(state.LockIDs))
	for _, id := range state.LockIDs {
		if id != other.ID {
			legacyLocks = append(legacyLocks, id)
		}
	}
	state.LockIDs = legacyLocks
	corr := "0000000000000000000000000000c003"
	journal := reconstituteRollbackJournal(fx, corr)
	journal.Reconstitute = state
	size := "M"
	_, err = SetArtifactSizeWithProvenance(ctx, fx.ws, other.ID, SizeMutation{Size: &size, Actor: ActorContextHuman})
	require.NoError(t, err)
	ownerPath, err := FindArtifactPath(ctx, fx.ws, other.ID)
	require.NoError(t, err)
	before, err := os.ReadFile(ownerPath)
	require.NoError(t, err)
	unlock, err := lockArtifactMutation(ctx, fx.ws, other.ID)
	require.NoError(t, err)
	locked := true
	t.Cleanup(func() {
		if locked {
			_ = unlock()
		}
	})
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr), journal)
	require.NoError(t, err)
	recoveryCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, globalUnlock, err := lockShipmentLifecycleGlobal(recoveryCtx, fx.ws)
	cancel()
	if globalUnlock != nil {
		require.NoError(t, globalUnlock())
	}
	require.Error(t, err, "recovery must not restore an owner while its mutation lock is held")
	after, err := os.ReadFile(ownerPath)
	require.NoError(t, err)
	require.Equal(t, before, after, "recovery must not overwrite the locked owner before failing")
	require.NoError(t, unlock())
	locked = false
	_, err = os.Stat(journalPath)
	require.NoError(t, err, "failed recovery must retain the journal")

	_, globalUnlock, err = lockShipmentLifecycleGlobal(ctx, fx.ws)
	if globalUnlock != nil {
		require.NoError(t, globalUnlock())
	}
	require.Error(t, err, "legacy recovery must refuse a changed image after acquiring its owner lock")
	require.FileExists(t, journalPath)
	after, err = os.ReadFile(ownerPath)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestLegacyReconstitutionRecoveryRefusesChangedProseOnlyOwner(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx := context.Background()
	other, err := CreateArtifact(ctx, fx.ws, "Unrelated artifact", "task",
		WithParent(fx.feature.ID), WithDescription("Mentions "+fx.tasks[0].ID))
	require.NoError(t, err)
	st, err := planReconstitution(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
	require.NoError(t, err)
	state, err := captureReconstituteState(ctx, fx.ws, st)
	require.NoError(t, err)
	state.ScopedImages = false
	path, err := FindArtifactPath(ctx, fx.ws, other.ID)
	require.NoError(t, err)
	preimage, err := os.ReadFile(path)
	require.NoError(t, err)
	rel, err := filepath.Rel(fx.ws.StorageRoot, path)
	require.NoError(t, err)
	state.Images = append(state.Images, reconstituteFileImage{
		Path: filepath.ToSlash(rel), Exists: true, Content: preimage,
	})
	size := "M"
	_, err = SetArtifactSizeWithProvenance(ctx, fx.ws, other.ID, SizeMutation{Size: &size, Actor: ActorContextHuman})
	require.NoError(t, err)
	current, err := os.ReadFile(path)
	require.NoError(t, err)
	corr := "0000000000000000000000000000c006"
	journal := reconstituteRollbackJournal(fx, corr)
	journal.Reconstitute = state
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr), journal)
	require.NoError(t, err)
	_, unlock, err := lockShipmentLifecycleGlobal(ctx, fx.ws)
	if unlock != nil {
		require.NoError(t, unlock())
	}
	require.Error(t, err)
	require.FileExists(t, journalPath)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, current, after)
}

func TestReconstitutionSequentialAdoptionsPreserveForwardReference(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx := context.Background()
	require.NoError(t, AddDependency(ctx, fx.ws, fx.tasks[0].ID, fx.tasks[1].ID, "blocks"))
	checkedLock := false
	fx.ws.HookRunner = hooks.NewHookRunner()
	fx.ws.HookRunner.Register(hooks.HookAdoptItem, hooks.PhasePre, hooks.HookRegistration{
		Name: "check-renamed-owner-lock",
		Fn: func(hookCtx context.Context, hc hooks.HookContext) error {
			if hc.ItemID != fx.tasks[1].ID {
				return nil
			}
			refs, err := findCrossArtifactReferences(hookCtx, fx.ws, fx.tasks[1].ID, "\x00")
			require.NoError(t, err)
			for _, ref := range refs {
				if ref.artifact.ID == fx.tasks[0].ID || ref.artifact.ArtifactType != "task" {
					continue
				}
				unlock, lockErr := lockArtifactMutation(context.Background(), fx.ws, ref.artifact.ID)
				if unlock != nil {
					require.NoError(t, unlock())
				}
				require.ErrorIs(t, lockErr, ErrTaskBusy)
				checkedLock = true
			}
			return nil
		},
	})
	result, err := ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
	require.NoError(t, err)
	require.True(t, checkedLock)
	first, ok := result.AdoptedIDMap[fx.tasks[0].ID]
	require.True(t, ok)
	second, ok := result.AdoptedIDMap[fx.tasks[1].ID]
	require.True(t, ok)
	owner, err := findArtifact(ctx, fx.ws, first)
	require.NoError(t, err)
	require.Len(t, owner.Dependencies, 1)
	require.Equal(t, second, owner.Dependencies[0].ID)
}

func TestReconstitutionRecoveryLocksRenamedOwner(t *testing.T) {
	for _, journaled := range []bool{true, false} {
		t.Run(fmt.Sprintf("journaled=%v", journaled), func(t *testing.T) {
			fx := newReconstituteFixture(t)
			ctx := context.Background()
			newID := "009.001-T"
			journal := reconstituteRollbackJournal(fx, "0000000000000000000000000000c007")
			if journaled {
				journal.Reconstitute.Adoptions = map[string]string{fx.tasks[0].ID: newID}
			} else {
				appendRawEvent(t, fx.ws, newID, "adopted", map[string]any{
					"old_id": fx.tasks[0].ID, "new_id": newID,
					shipmentOperationDeltaKey: journal.CorrelationID,
				})
			}
			journalPath, err := writeShipmentLifecycleJournalForWorkspace(
				fx.ws, shipmentLifecycleJournalName(journal.CorrelationID), journal)
			require.NoError(t, err)
			unlock, err := lockArtifactMutation(ctx, fx.ws, newID)
			require.NoError(t, err)
			defer func() { require.NoError(t, unlock()) }()
			recoveryCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer cancel()
			_, globalUnlock, err := lockShipmentLifecycleGlobal(recoveryCtx, fx.ws)
			if globalUnlock != nil {
				require.NoError(t, globalUnlock())
			}
			require.Error(t, err, "recovery must not mutate a renamed owner while its lock is held")
			require.FileExists(t, journalPath)
		})
	}
}

func TestLegacyReconstitutionRecoveryPreservesNotes(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("changed=%v", changed), func(t *testing.T) {
			fx := newReconstituteFixture(t)
			ctx := context.Background()
			st, err := planReconstitution(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
			require.NoError(t, err)
			state, err := captureReconstituteState(ctx, fx.ws, st)
			require.NoError(t, err)
			note := filepath.Join(fx.ws.StorageRoot, "queue", "legacy-note.md")
			require.NoError(t, os.WriteFile(note, []byte("legacy note\n"), 0o600))
			state.Images = append(state.Images, reconstituteFileImage{
				Path: "queue/legacy-note.md", Exists: true, Content: []byte("legacy note\n"),
			})
			if changed {
				require.NoError(t, os.WriteFile(note, []byte("new note\n"), 0o600))
			}
			corr := "0000000000000000000000000000c004"
			journal := reconstituteRollbackJournal(fx, corr)
			journal.Reconstitute = state
			journalPath, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr), journal)
			require.NoError(t, err)
			_, unlock, err := lockShipmentLifecycleGlobal(ctx, fx.ws)
			if unlock != nil {
				require.NoError(t, unlock())
			}
			if changed {
				require.Error(t, err)
				require.FileExists(t, journalPath)
			} else {
				require.NoError(t, err)
				require.NoFileExists(t, journalPath)
			}
			noteData, err := os.ReadFile(note)
			require.NoError(t, err)
			if changed {
				require.Equal(t, "new note\n", string(noteData))
			} else {
				require.Equal(t, "legacy note\n", string(noteData))
			}
		})
	}
}

func TestReconstitutionRecoveryRejectsDifferentDestinationOwner(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx := context.Background()
	st, err := planReconstitution(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
	require.NoError(t, err)
	state, err := captureReconstituteState(ctx, fx.ws, st)
	require.NoError(t, err)
	ownerPath, err := FindArtifactPath(ctx, fx.ws, fx.tasks[0].ID)
	require.NoError(t, err)
	otherPath, err := FindArtifactPath(ctx, fx.ws, fx.tasks[1].ID)
	require.NoError(t, err)
	other, err := os.ReadFile(otherPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(ownerPath, other, 0o600))
	corr := "0000000000000000000000000000c005"
	journal := reconstituteRollbackJournal(fx, corr)
	journal.Reconstitute = state
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr), journal)
	require.NoError(t, err)
	_, unlock, err := lockShipmentLifecycleGlobal(ctx, fx.ws)
	if unlock != nil {
		require.NoError(t, unlock())
	}
	require.ErrorContains(t, err, "different owner")
	require.FileExists(t, journalPath)
	got, err := os.ReadFile(ownerPath)
	require.NoError(t, err)
	require.Equal(t, other, got)
}

func TestReconstitutionDoesNotWaitForArtifactWhileHoldingMemberLog(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id := fx.tasks[0].ID
	writerLocked := make(chan struct{})
	releaseWriter := make(chan struct{})
	logLocked := make(chan struct{})
	artifactMutationLockBarrierHook = func(lockedID string) {
		if lockedID == id {
			select {
			case <-writerLocked:
			default:
				close(writerLocked)
				<-releaseWriter
			}
		}
	}
	t.Cleanup(func() { artifactMutationLockBarrierHook = nil })
	restore := events.SetItemLogLockBarrierHookForTest(func(lockedID string) {
		if lockedID == id {
			select {
			case <-logLocked:
			default:
				close(logLocked)
			}
		}
	})
	t.Cleanup(restore)
	writerDone := make(chan error, 1)
	go func() {
		size := "M"
		_, err := SetArtifactSizeWithProvenance(ctx, fx.ws, id, SizeMutation{Size: &size, Actor: ActorContextHuman})
		writerDone <- err
	}()
	<-writerLocked
	reconstituteDone := make(chan error, 1)
	go func() {
		_, err := ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
		reconstituteDone <- err
	}()
	select {
	case <-logLocked:
	case <-ctx.Done():
		close(releaseWriter)
		t.Fatal("reconstitution did not acquire member log")
	}
	close(releaseWriter)
	select {
	case err := <-reconstituteDone:
		require.Error(t, err, "contended artifact must fail fast")
	case <-time.After(time.Second):
		t.Error("reconstitution waited for B while holding C")
	}
	select {
	case err := <-writerDone:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("size writer did not complete")
	}
}

func TestRound3ReadOnlyWorkspaceRejectsSourceLinks(t *testing.T) {
	for _, wal := range []bool{false, true} {
		t.Run(fmt.Sprintf("wal=%v", wal), func(t *testing.T) {
			fx := newReconstituteFixture(t)
			require.NoError(t, fx.ws.Close())
			path := filepath.Join(fx.ws.StorageRoot, "backlogit.db")
			outside := filepath.Join(t.TempDir(), "outside.db")
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(outside, raw, 0o600))
			if wal {
				path += "-wal"
			} else {
				require.NoError(t, os.Remove(path))
			}
			require.NoError(t, os.Symlink(outside, path))
			ro, err := NewReadOnlyIndexWorkspace(context.Background(), fx.ws.RootPath)
			if ro != nil {
				require.NoError(t, ro.Close())
			}
			require.Error(t, err)
		})
	}
}

func TestRound3ForceReconstituteHoldsUnassignedMemberLog(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx := context.Background()
	_, err := ClaimShipment(ctx, fx.ws, fx.source.ID)
	require.NoError(t, err)
	// Leaving every item UNASSIGNED exercises the omitted-source-member case.
	plan := fmt.Sprintf("source_shipments: [%s]\nshipments:\n  - id: %s\n    feature: {id: %s}\n    items: [%s]\nunassigned_items: [%s, %s, %s, %s, %s]\n",
		fx.source.ID, fx.source.ID, fx.feature.ID, fx.tasks[5].ID, fx.tasks[0].ID, fx.tasks[1].ID, fx.tasks[2].ID,
		fx.tasks[3].ID, fx.tasks[4].ID)
	_, err = ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{PlanYAML: plan, Force: true, Reason: "fixture", DryRun: true})
	require.NoError(t, err, "fixture must pass planning before the coordinated race")
	done := make(chan error, 1)
	started := make(chan struct{})
	called := false
	var memberLogAcquired atomic.Bool
	restore := events.SetItemLogLockBarrierHookForTest(func(id string) {
		if id == fx.tasks[0].ID {
			memberLogAcquired.Store(true)
		}
	})
	t.Cleanup(restore)
	fx.ws.HookRunner = hooks.NewHookRunner()
	fx.ws.HookRunner.Register(hooks.HookUpdateArtifact, hooks.PhasePre, hooks.HookRegistration{
		Name: "round3-start",
		Fn: func(context.Context, hooks.HookContext) error {
			called = true
			assert.True(t, memberLogAcquired.Load(), "unassigned member log must be acquired before mutation")
			go func() {
				writerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				close(started)
				done <- AppendComment(writerCtx, fx.ws, events.NewEventWriter(WorkspaceLogsRoot(fx.ws.RootPath)),
					fx.tasks[0].ID, "tester", "WORK_STARTED: coordinated fixture only", "")
			}()
			<-started
			select {
			case err := <-done:
				t.Errorf("WORK_STARTED completed before rollback released the lock: %v", err)
			case <-time.After(150 * time.Millisecond):
			}
			return errors.New("round3 rollback veto")
		},
	})
	_, err = ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{PlanYAML: plan, Force: true, Reason: "fixture"})
	require.ErrorContains(t, err, "round3 rollback veto")
	require.True(t, called, "must reach the mutation hook, not fail early")
	select {
	case err := <-done:
		require.NoError(t, err, "writer must proceed after rollback")
	case <-time.After(30 * time.Second):
		t.Fatal("writer remained blocked after rollback")
	}
	current, err := findArtifact(ctx, fx.ws, fx.tasks[0].ID)
	require.NoError(t, err)
	assert.Equal(t, fx.tasks[0].ID, current.ID)
}
