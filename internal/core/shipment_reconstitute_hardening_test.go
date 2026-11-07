package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

func TestU1f_ExecuteReconstitutionPreliminaryPlanReadsUnderCanonicalLocks(t *testing.T) {
	ctx := context.Background()
	// Allocate through the parent test to keep Windows fixture paths short.
	parent := t
	fixture := func() *reconstituteFixture {
		fx := newReconstituteFixture(parent)
		for _, item := range []string{fx.source.ID, fx.feature.ID,
			fx.tasks[0].ID, fx.tasks[1].ID, fx.tasks[2].ID, fx.tasks[3].ID, fx.tasks[4].ID, fx.tasks[5].ID} {
			path, err := FindArtifactPath(ctx, fx.ws, item)
			require.NoError(parent, err)
			artifact, _, err := parseFile(path)
			require.NoError(parent, err)
			require.Equal(parent, item, artifact.ID)
			if item == fx.source.ID {
				require.Equal(parent, "shipment", artifact.ArtifactType)
			} else if item == fx.feature.ID {
				require.Equal(parent, "feature", artifact.ArtifactType)
			} else {
				require.Equal(parent, "task", artifact.ArtifactType)
				require.Equal(parent, fx.feature.ID, artifact.ParentID)
			}
		}
		return fx
	}
	type outcome struct {
		res *ReconstituteShipmentResult
		err error
	}

	t.Run("preliminary_plan_reads_wait_for_stash_global_and_membership_locks", func(t *testing.T) {
		for _, preCanceled := range []bool{true, false} {
			cancelFX := fixture()
			before := workspaceFileSnapshot(t, cancelFX.ws)
			unlockStash, lockErr := tryLockActiveStash(cancelFX.ws)
			require.NoError(t, lockErr)
			callCtx, cancelCall := context.WithCancel(ctx)
			var attempts atomic.Int32
			callCtx = context.WithValue(callCtx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
				if stage == "attempt" {
					attempts.Add(1)
				}
			})
			ack := make(chan struct{})
			done := make(chan outcome, 1)
			var releaseOnce sync.Once
			releaseStash := func() {
				releaseOnce.Do(func() { assert.NoError(t, unlockStash()) })
			}
			joined := false
			t.Cleanup(func() {
				cancelCall()
				// The blocking baseline must be unblocked BEFORE every join.
				releaseStash()
				if !joined {
					<-done
				}
				t.Logf("U1f F1 cleanup completed preCanceled=%t", preCanceled)
			})
			if preCanceled {
				cancelCall()
			}
			plan := cancelFX.splitPlanYAML()
			go func() {
				close(ack)
				res, callErr := ReconstituteShipments(callCtx, cancelFX.ws, ReconstituteOptions{PlanYAML: plan})
				done <- outcome{res, callErr}
			}()
			<-ack
			if !preCanceled {
				cancelCall()
			}
			// Ack proves launch, NOT exact entry into stashMu.Lock. This
			// bounded select observes only return while the holder is live.
			observation := time.NewTimer(2 * time.Second)
			select {
			case got := <-done:
				joined = true
				assert.ErrorIs(t, got.err, context.Canceled, "U1f F1 cancellation error preCanceled=%t", preCanceled)
				assert.Nil(t, got.res, "U1f F1 canceled result preCanceled=%t", preCanceled)
			case <-observation.C:
				t.Errorf("U1f F1 return-before-stash-release assertion preCanceled=%t: canceled call remained blocked", preCanceled)
			}
			observation.Stop()
			releaseStash()
			if !joined {
				got := <-done
				joined = true
				assert.ErrorIs(t, got.err, context.Canceled, "U1f F1 error after cleanup release preCanceled=%t", preCanceled)
			}
			assert.Zero(t, attempts.Load(), "U1f F6 pre-canceled acquisition must not attempt global preCanceled=%t", preCanceled)
			assert.Equal(t, before, workspaceFileSnapshot(t, cancelFX.ws),
				"U1f F6 canceled call final persisted artifacts/events (not proof of no transient writes)")
		}
		fx := fixture()
		task5Path, err := FindArtifactPath(ctx, fx.ws, fx.tasks[5].ID)
		require.NoError(t, err)
		task5Bytes, err := os.ReadFile(task5Path)
		require.NoError(t, err)
		unlockMembership, err := lockShipmentMembership(ctx, fx.ws, fx.source.ID)
		require.NoError(t, err)
		reconCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var globalAcquired atomic.Bool
		membershipContended := make(chan struct{}, 1)
		reconCtx = context.WithValue(reconCtx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
			if stage == "acquired" {
				globalAcquired.Store(true)
			}
			if stage == "contended" {
				select {
				case membershipContended <- struct{}{}:
				default:
				}
			}
		})
		reconDone := make(chan outcome, 1)
		started, joined := false, false
		t.Cleanup(func() {
			cancel()
			assert.NoError(t, unlockMembership())
			if started && !joined {
				<-reconDone
			}
			if _, statErr := os.Stat(task5Path); os.IsNotExist(statErr) {
				assert.NoError(t, os.WriteFile(task5Path, task5Bytes, 0o644))
			} else {
				assert.NoError(t, statErr)
			}
			t.Log("U1f subtest1 cleanup completed")
		})
		require.NoError(t, os.Remove(task5Path))
		started = true
		go func() {
			res, reconErr := ReconstituteShipments(reconCtx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
			reconDone <- outcome{res, reconErr}
		}()
		select {
		case <-membershipContended:
			require.True(t, globalAcquired.Load())
		case got := <-reconDone:
			joined = true
			require.ErrorIs(t, got.err, blerrors.ErrValidation)
			require.False(t, globalAcquired.Load())
			t.Fatalf("U1f preliminary-read ordering assertion: returned before canonical locks: %v", got.err)
		case <-reconCtx.Done():
			t.Fatal("reconstitution did not reach membership contention")
		}
		if unlock, stashErr := tryLockActiveStash(fx.ws); stashErr == nil {
			require.NoError(t, unlock())
			t.Fatal("active stash lock must be held before preliminary plan reads")
		} else {
			require.ErrorIs(t, stashErr, os.ErrExist)
		}
		_, err = os.Lstat(StashFilePath(fx.ws.RootPath) + ".lock")
		assert.NoError(t, err)
		require.NoError(t, os.WriteFile(task5Path, task5Bytes, 0o644))
		require.NoError(t, unlockMembership())
		got := <-reconDone
		joined = true
		require.NoError(t, got.err)
		require.NotNil(t, got.res)
	})

	t.Run("concurrent_associate_commit_durable_replace_serializes_before_preliminary_plan_reads", func(t *testing.T) {
		digestFX := fixture()
		before := workspaceFileSnapshot(t, digestFX.ws)
		planPath := filepath.Join(digestFX.ws.RootPath, "u1f-plan.yaml")
		originalYAML := digestFX.splitPlanYAML()
		changedYAML := originalYAML + "\n# changed at lifecycle-global acquisition\n"
		require.NoError(t, os.WriteFile(planPath, []byte(originalYAML), 0o644))
		_, originalDigest, err := loadAndValidateReconstitutePlan(digestFX.ws, ReconstituteOptions{PlanYAML: originalYAML})
		require.NoError(t, err)
		_, changedDigest, err := loadAndValidateReconstitutePlan(digestFX.ws, ReconstituteOptions{PlanYAML: changedYAML})
		require.NoError(t, err)
		assert.NotEqual(t, originalDigest, changedDigest, "U1f F7 YAML byte digest discriminator")
		digestCtx, cancelDigest := context.WithTimeout(ctx, 10*time.Second)
		defer cancelDigest()
		changed := false
		var rewriteErr error
		digestCtx = context.WithValue(digestCtx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
			if stage == "acquired" {
				changed = true
				rewriteErr = os.WriteFile(planPath, []byte(changedYAML), 0o644)
			}
		})
		digestResult, digestErr := ReconstituteShipments(digestCtx, digestFX.ws, ReconstituteOptions{PlanPath: planPath})
		require.NoError(t, rewriteErr)
		assert.True(t, changed, "U1f F7 existing global hook changed plan YAML")
		assert.ErrorIs(t, digestErr, blerrors.ErrValidation, "U1f F7 digest change rejects")
		assert.Nil(t, digestResult, "U1f F7 digest change has no result")
		assert.Equal(t, before, workspaceFileSnapshot(t, digestFX.ws), "U1f F7 digest rejection preserves artifacts/events")
		planBytes, err := os.ReadFile(planPath)
		require.NoError(t, err)
		assert.Equal(t, changedYAML, string(planBytes), "U1f F7 plan writer's bytes remain")
		pending, err := loadShipmentOperationJournals(digestFX.ws)
		require.NoError(t, err)
		assert.Empty(t, pending, "U1f F7 digest rejection leaves no pending journal")

		fx := fixture()
		fx.ws.Config.DurableWrites = true
		ew := events.NewEventWriter(WorkspaceLogsRoot(fx.ws.RootPath))
		commitAtReplaceBarrier := make(chan struct{})
		releaseCommitReplace := make(chan struct{})
		var interceptOnce, releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(releaseCommitReplace) }) }
		prevHook := artifactMutationLockBarrierHook
		artifactMutationLockBarrierHook = func(id string) {
			if id == fx.tasks[5].ID {
				interceptOnce.Do(func() {
					close(commitAtReplaceBarrier)
					<-releaseCommitReplace
				})
			}
		}
		commitCtx, cancelCommit := context.WithTimeout(ctx, 10*time.Second)
		reconCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		commitDone := make(chan error, 1)
		reconDone := make(chan outcome, 1)
		commitJoined, reconStarted, reconJoined := false, false, false
		t.Cleanup(func() {
			// Release before cancellation, joins, and restoring the package hook.
			release()
			cancel()
			cancelCommit()
			if !commitJoined {
				<-commitDone
			}
			if reconStarted && !reconJoined {
				<-reconDone
			}
			artifactMutationLockBarrierHook = prevHook
			t.Log("U1f subtest2 cleanup completed")
		})
		go func() {
			commitDone <- AssociateCommit(commitCtx, fx.ws, ew, fx.tasks[5].ID, "feedface", "concurrent", "tester")
		}()
		select {
		case <-commitAtReplaceBarrier:
		case <-commitCtx.Done():
			t.Fatal("AssociateCommit did not reach replacement barrier")
		}
		var globalAttempted atomic.Bool
		reconContendedOnGlobal := make(chan struct{}, 1)
		reconCtx = context.WithValue(reconCtx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
			if stage == "attempt" {
				globalAttempted.Store(true)
			}
			if stage == "contended" {
				release()
				select {
				case reconContendedOnGlobal <- struct{}{}:
				default:
				}
			}
		})
		reconStarted = true
		go func() {
			defer release()
			res, reconErr := ReconstituteShipments(reconCtx, fx.ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
			reconDone <- outcome{res, reconErr}
		}()
		commitErr := <-commitDone
		commitJoined = true
		got := <-reconDone
		reconJoined = true
		artifactMutationLockBarrierHook = prevHook
		assert.True(t, globalAttempted.Load())
		select {
		case <-reconContendedOnGlobal:
		default:
			t.Error("reconstitution must contend on AssociateCommit's lifecycle-global lock")
		}
		require.NoError(t, commitErr)
		require.NoError(t, got.err)
		require.NotNil(t, got.res)
		path, err := FindArtifactPath(ctx, fx.ws, fx.tasks[5].ID)
		require.NoError(t, err)
		artifact, _, err := parseFile(path)
		require.NoError(t, err)
		assert.Equal(t, "feedface", artifact.Commit)
		var count int
		require.NoError(t, fx.ws.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM commit_links WHERE item_id = ? AND commit_sha = ?`,
			fx.tasks[5].ID, "feedface").Scan(&count))
		assert.Equal(t, 1, count)
		history, err := events.ReadAllEvents(ctx, WorkspaceLogsRoot(fx.ws.RootPath), fx.tasks[5].ID)
		require.NoError(t, err)
		tracked := false
		for _, event := range history {
			tracked = tracked || event.EventType == "commit_tracked"
		}
		assert.True(t, tracked)
	})

	t.Run("pure_input_and_plan_validation_fails_fast_before_locks", func(t *testing.T) {
		fx := fixture()
		_, releaseGlobal, err := lockShipmentLifecycleGlobal(ctx, fx.ws)
		require.NoError(t, err)
		defer func() { assert.NoError(t, releaseGlobal()) }()
		var globalAttempted atomic.Bool
		failFastCtx := context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
			if stage == "attempt" {
				globalAttempted.Store(true)
			}
		})
		plan := fx.splitPlanYAML()
		tests := []ReconstituteOptions{
			{PlanYAML: plan, Force: true},
			{PlanYAML: plan + "\nbogus: 1\n"},
			{PlanYAML: strings.Replace(plan, "source_shipments: ["+fx.source.ID+"]", `source_shipments: [""]`, 1)},
			{PlanYAML: strings.Replace(plan, "source_shipments: ["+fx.source.ID+"]", `source_shipments: ["../escape"]`, 1)},
			{PlanYAML: strings.Replace(plan, "source_shipments: ["+fx.source.ID+"]", `source_shipments: ["shipment-lifecycle-global"]`, 1)},
			{PlanYAML: strings.Replace(plan, "source_shipments: ["+fx.source.ID+"]",
				fmt.Sprintf("source_shipments: [%q, %q]", " "+fx.source.ID+" ", strings.ToLower(fx.source.ID)), 1)},
		}
		for i, opts := range tests {
			globalAttempted.Store(false)
			_, err := ReconstituteShipments(failFastCtx, fx.ws, opts)
			assert.ErrorIs(t, err, blerrors.ErrValidation, "input case %d", i)
			assert.False(t, globalAttempted.Load(), "input case %d must fail before lifecycle-global", i)
			_, statErr := os.Lstat(StashFilePath(fx.ws.RootPath) + ".lock")
			assert.True(t, os.IsNotExist(statErr), "input case %d must not create stash sidecar: %v", i, statErr)
		}
		require.NoError(t, releaseGlobal())
		for _, cancelStage := range []string{"attempt", "acquired"} {
			cancelFX := fixture()
			taskPath, pathErr := FindArtifactPath(ctx, cancelFX.ws, cancelFX.tasks[5].ID)
			require.NoError(t, pathErr)
			taskBytes, readErr := os.ReadFile(taskPath)
			require.NoError(t, readErr)
			// A missing artifact discriminates cancellation from proceeding
			// into preliminary validation. It does not instrument each scan.
			require.NoError(t, os.Remove(taskPath))
			t.Cleanup(func() { assert.NoError(t, os.WriteFile(taskPath, taskBytes, 0o644)) })
			before := workspaceFileSnapshot(t, cancelFX.ws)
			cancelCtx, cancelCall := context.WithCancel(ctx)
			var stages []string
			cancelCtx = context.WithValue(cancelCtx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
				stages = append(stages, stage)
				if stage == cancelStage {
					cancelCall()
				}
			})
			result, callErr := ReconstituteShipments(cancelCtx, cancelFX.ws, ReconstituteOptions{PlanYAML: cancelFX.splitPlanYAML()})
			cancelCall()
			assert.Contains(t, stages, cancelStage, "U1f F6 cancellation barrier reached")
			assert.ErrorIs(t, callErr, context.Canceled, "U1f F6 cancellation before acquisitions/scans stage=%s", cancelStage)
			assert.Nil(t, result, "U1f F6 canceled result stage=%s", cancelStage)
			if cancelStage == "attempt" {
				assert.NotContains(t, stages, "acquired", "U1f F6 canceled global attempt must not acquire")
			}
			assert.Equal(t, before, workspaceFileSnapshot(t, cancelFX.ws),
				"U1f F6 final persisted state stage=%s (not proof of no transient post-cancel writes)", cancelStage)
			pending, pendingErr := loadShipmentOperationJournals(cancelFX.ws)
			require.NoError(t, pendingErr)
			assert.Empty(t, pending, "U1f F6 no pending journal stage=%s", cancelStage)
		}

		sortedFX := fixture()
		downPath, err := FindArtifactPath(ctx, sortedFX.ws, sortedFX.downstream.ID)
		require.NoError(t, err)
		downstream, _, err := parseFile(downPath)
		require.NoError(t, err)
		require.Equal(t, sortedFX.downstream.ID, downstream.ID)
		require.Equal(t, "shipment", downstream.ArtifactType)
		downItems := NormalizeShipmentItems(downstream)
		require.Len(t, downItems, 1)
		downTaskPath, err := FindArtifactPath(ctx, sortedFX.ws, downItems[0])
		require.NoError(t, err)
		downTask, _, err := parseFile(downTaskPath)
		require.NoError(t, err)
		require.Equal(t, downItems[0], downTask.ID)
		require.Equal(t, "task", downTask.ArtifactType)
		downFeaturePath, err := FindArtifactPath(ctx, sortedFX.ws, downTask.ParentID)
		require.NoError(t, err)
		downFeature, _, err := parseFile(downFeaturePath)
		require.NoError(t, err)
		require.Equal(t, downTask.ParentID, downFeature.ID)
		require.Equal(t, "feature", downFeature.ArtifactType)
		require.Less(t, sortedFX.source.ID, downstream.ID, "U1f F7 fixture IDs ordered")
		unsortedPlan := strings.Replace(sortedFX.splitPlanYAML(),
			"source_shipments: ["+sortedFX.source.ID+"]",
			fmt.Sprintf("source_shipments: [%s, %s]", downstream.ID, sortedFX.source.ID), 1)
		unsortedPlan += fmt.Sprintf("  - id: %s\n    feature: {id: %s}\n    items: [%s]\n",
			downstream.ID, downFeature.ID, downTask.ID)
		before := workspaceFileSnapshot(t, sortedFX.ws)
		unlockLow, err := lockShipmentMembership(ctx, sortedFX.ws, sortedFX.source.ID)
		require.NoError(t, err)
		sortedCtx, cancelSorted := context.WithTimeout(ctx, 10*time.Second)
		contended := make(chan struct{}, 1)
		var acquired atomic.Bool
		sortedCtx = context.WithValue(sortedCtx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
			if stage == "acquired" {
				acquired.Store(true)
			}
			if stage == "contended" {
				select {
				case contended <- struct{}{}:
				default:
				}
			}
		})
		done := make(chan outcome, 1)
		joined := false
		t.Cleanup(func() {
			cancelSorted()
			assert.NoError(t, unlockLow())
			if !joined {
				<-done
			}
			t.Log("U1f F7 sorted membership cleanup completed")
		})
		go func() {
			result, callErr := ReconstituteShipments(sortedCtx, sortedFX.ws, ReconstituteOptions{PlanYAML: unsortedPlan})
			done <- outcome{result, callErr}
		}()
		select {
		case <-contended:
			assert.True(t, acquired.Load(), "U1f F7 global precedes source membership")
		case got := <-done:
			joined = true
			t.Fatalf("U1f F7 returned before held membership: %v", got.err)
		case <-sortedCtx.Done():
			t.Fatal("U1f F7 did not reach held membership")
		}
		// With the lower ID held, an unsorted acquisition would already
		// own the higher ID when it contends. Probe using the existing lock.
		highKey := filepath.Join(WorkspaceStorageRoot(sortedFX.ws.RootPath), shipmentMembershipLocksDirName, downstream.ID)
		probeUnlock, probeErr := lockTaskFile(highKey)
		assert.NoError(t, probeErr, "U1f F7 unsorted sources must acquire memberships sorted")
		if probeErr == nil {
			assert.NoError(t, probeUnlock())
		}
		cancelSorted()
		observation := time.NewTimer(2 * time.Second)
		select {
		case got := <-done:
			joined = true
			assert.ErrorIs(t, got.err, context.Canceled, "U1f F6 membership cancellation before scans")
			assert.Nil(t, got.res, "U1f F6 canceled membership result")
		case <-observation.C:
			t.Error("U1f F6 cancellation must return before membership holder release")
		}
		observation.Stop()
		require.NoError(t, unlockLow())
		if !joined {
			<-done
			joined = true
		}
		assert.Equal(t, before, workspaceFileSnapshot(t, sortedFX.ws), "U1f F6 canceled membership final artifacts/events")
		pending, err := loadShipmentOperationJournals(sortedFX.ws)
		require.NoError(t, err)
		assert.Empty(t, pending, "U1f F6 canceled membership leaves no pending journal")
	})
}

func reconstituteRollbackUnderStash(t *testing.T, fx *reconstituteFixture, journal shipmentLifecycleJournal) error {
	t.Helper()
	unlock, err := lockActiveStash(fx.ws)
	require.NoError(t, err)
	defer func() { _ = unlock() }()
	return rollbackReconstitute(withStashLockHeld(context.Background()), fx.ws, journal)
}

func TestReconstituteJournalRejectsUnsafeRecoveryState(t *testing.T) {
	tests := []struct {
		name  string
		state reconstituteJournalState
	}{
		{"created id traversal", reconstituteJournalState{CreatedIDs: []string{"../../../victim"}}},
		{"created id separator", reconstituteJournalState{CreatedIDs: []string{`a\b`}}},
		{"adoption old id", reconstituteJournalState{Adoptions: map[string]string{"../x": "001-F"}}},
		{"adoption new id", reconstituteJournalState{Adoptions: map[string]string{"001-F": "a/b"}}},
		{"image traversal", reconstituteJournalState{Images: []reconstituteFileImage{{Path: "../outside.md"}}}},
		{"image absolute", reconstituteJournalState{Images: []reconstituteFileImage{{Path: "/etc/x.md"}}}},
		{"image wrong kind", reconstituteJournalState{Images: []reconstituteFileImage{{Path: "config.yaml"}}}},
		{"image in ops", reconstituteJournalState{Images: []reconstituteFileImage{{Path: "ops/x.md"}}}},
		{"known log traversal", reconstituteJournalState{KnownLogs: []string{"logs/../x.jsonl"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, validateReconstituteJournalState(&tc.state), blerrors.ErrValidation)
		})
	}
	valid := reconstituteJournalState{
		CreatedIDs: []string{"001-F"}, Adoptions: map[string]string{"001.001-T": "002.001-T"},
		Images:    []reconstituteFileImage{{Path: "queue/001-F.md"}, {Path: "archive/stash.jsonl"}},
		KnownLogs: []string{"logs/001-F.jsonl"},
	}
	require.NoError(t, validateReconstituteJournalState(&valid))
}

func TestReconstituteRollbackRefusesTraversalCreatedID(t *testing.T) {
	fx := newReconstituteFixture(t)
	victim := filepath.Join(filepath.Dir(fx.ws.StorageRoot), "victim.md")
	require.NoError(t, os.WriteFile(victim, []byte("keep"), 0o644))

	journal := reconstituteRollbackJournal(fx, "0000000000000000000000000000c001", "../victim")
	err := reconstituteRollbackUnderStash(t, fx, journal)
	require.ErrorIs(t, err, blerrors.ErrValidation)
	assert.FileExists(t, victim)
}

func TestReconstituteRollbackRefusesRedirectedImagePath(t *testing.T) {
	fx := newReconstituteFixture(t)
	outside := t.TempDir()
	link := filepath.Join(fx.ws.StorageRoot, "queue-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	journal := reconstituteRollbackJournal(fx, "0000000000000000000000000000c002")
	journal.Reconstitute.Images = []reconstituteFileImage{{Path: "queue-link/escape.md", Exists: true, Content: []byte("x")}}
	require.Error(t, reconstituteRollbackUnderStash(t, fx, journal))
	assert.NoFileExists(t, filepath.Join(outside, "escape.md"))
}

func appendRawEvent(t *testing.T, ws *Workspace, itemID, eventType string, delta map[string]any) {
	t.Helper()
	writer := events.NewEventWriter(WorkspaceLogsRoot(ws.RootPath))
	require.NoError(t, writer.AppendEvent(context.Background(), events.Event{
		Actor: "tester", ItemID: itemID, EventType: eventType, Delta: delta,
	}))
}

func TestReconstituteRollbackMergesLogsFromJournaledAdoption(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	oldID, newID := fx.tasks[0].ID, "009.001-T"
	logsRoot := WorkspaceLogsRoot(fx.ws.RootPath)

	// The adopted item's history lives under the new ID (carrying no tagged
	// event, so only the journaled pair can reveal it); an independent comment
	// then recreated the old-ID log.
	appendRawEvent(t, fx.ws, newID, "comment", map[string]any{"text": "history before adoption"})
	time.Sleep(5 * time.Millisecond)
	appendRawEvent(t, fx.ws, oldID, "comment", map[string]any{"text": "independent comment after rename"})

	journal := reconstituteRollbackJournal(fx, "0000000000000000000000000000c003")
	journal.Reconstitute.Adoptions = map[string]string{oldID: newID}
	journal.Reconstitute.EventIDs = []string{fx.source.ID, oldID}
	require.NoError(t, reconstituteRollbackUnderStash(t, fx, journal))

	assert.NoFileExists(t, events.LogPathForItem(logsRoot, newID), "adopted log is removed only after merge")
	merged, err := events.ReadAllEvents(ctx, logsRoot, oldID)
	require.NoError(t, err)
	var texts []string
	for _, event := range merged {
		texts = append(texts, fmt.Sprint(event.Delta["text"]))
	}
	assert.Contains(t, texts, "history before adoption")
	assert.Contains(t, texts, "independent comment after rename")
}

func TestReconstituteRollbackKeepsAdoptedLogWhenRestoreFails(t *testing.T) {
	fx := newReconstituteFixture(t)
	oldID, newID := fx.tasks[1].ID, "009.002-T"
	logsRoot := WorkspaceLogsRoot(fx.ws.RootPath)
	appendRawEvent(t, fx.ws, newID, "comment", map[string]any{"text": "must survive"})
	// A directory at the old-ID log path makes the merge read fail.
	require.NoError(t, os.MkdirAll(events.LogPathForItem(logsRoot, oldID), 0o755))

	journal := reconstituteRollbackJournal(fx, "0000000000000000000000000000c004")
	journal.Reconstitute.Adoptions = map[string]string{oldID: newID}
	require.Error(t, reconstituteRollbackUnderStash(t, fx, journal))
	assert.FileExists(t, events.LogPathForItem(logsRoot, newID), "history must not be deleted after a failed restore")
}

func TestReconstituteRollbackPreservesDBOnlyLinks(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	a, b := fx.tasks[0].ID, fx.tasks[1].ID
	require.NoError(t, bldb.AddLink(ctx, fx.ws.DB, a, b, "related_to"))

	journal := reconstituteRollbackJournal(fx, "0000000000000000000000000000c005")
	require.NoError(t, reconstituteRollbackUnderStash(t, fx, journal))

	links, err := bldb.GetLinks(ctx, fx.ws.DB, a)
	require.NoError(t, err)
	found := false
	for _, link := range links {
		found = found || (link.TargetID == b && link.LinkType == "related_to")
	}
	assert.True(t, found, "a database-only link must survive the rollback rehydrate")
}

func rewirePlanYAML(fx *reconstituteFixture) string {
	t := fx.tasks
	return fmt.Sprintf(`source_shipments: [%s]
shipments:
  - id: %s
    feature: {id: %s}
    items: [%s, %s, %s, %s, %s, %s]
rewire:
  - item: %s
    blocks_on: [%s]
`, fx.source.ID, fx.source.ID, fx.feature.ID, t[0].ID, t[1].ID, t[2].ID, t[3].ID, t[4].ID, t[5].ID, t[0].ID, t[1].ID)
}

func TestReconstituteOrderingHonorsPlannedRewires(t *testing.T) {
	ctx := context.Background()
	for _, dryRun := range []bool{true, false} {
		t.Run(fmt.Sprintf("dry_run=%v", dryRun), func(t *testing.T) {
			fx := newReconstituteFixture(t)
			res, err := ReconstituteShipments(ctx, fx.ws, ReconstituteOptions{PlanYAML: rewirePlanYAML(fx), DryRun: dryRun})
			require.NoError(t, err)
			order := res.AfterManifests[fx.source.ID]
			require.NotEmpty(t, order)
			index := func(id string) int {
				for i, v := range order {
					if v == id {
						return i
					}
				}
				return -1
			}
			require.NotEqual(t, -1, index(fx.tasks[0].ID))
			require.NotEqual(t, -1, index(fx.tasks[1].ID))
			assert.Less(t, index(fx.tasks[1].ID), index(fx.tasks[0].ID),
				"a rewired blocker must precede its dependent: %v", order)
		})
	}
}

func TestReconstituteDryRunOnReadOnlyIndexWorkspaceIsZeroWrite(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	require.NoError(t, fx.ws.Close())
	fx.ws.DB = nil

	ro, err := NewReadOnlyIndexWorkspace(ctx, fx.ws.RootPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ro.Close() })
	require.NotNil(t, ro.DB)
	before := workspaceFileSnapshot(t, ro)

	res, err := ReconstituteShipments(ctx, ro, ReconstituteOptions{PlanYAML: fx.splitPlanYAML(), DryRun: true})
	require.NoError(t, err)
	assert.NotEmpty(t, res.CreatedFeatures[0].ID)
	assert.Equal(t, before, workspaceFileSnapshot(t, ro))

	_, err = ro.DB.ExecContext(ctx, `DELETE FROM items`)
	require.Error(t, err, "the read-only index must reject writes")
}

func TestAssociateCommitAcquiresLifecycleGlobalBeforeItemLog(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	item := fx.tasks[0].ID
	logsRoot, locksRoot := WorkspaceLogsRoot(fx.ws.RootPath), WorkspaceLocksRoot(fx.ws.RootPath)

	_, releaseGlobal, err := lockShipmentLifecycleGlobal(ctx, fx.ws)
	require.NoError(t, err)
	released := false
	t.Cleanup(func() {
		if !released {
			_ = releaseGlobal()
		}
	})

	attempted := make(chan struct{}, 1)
	hookCtx := context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
		if stage == "attempt" {
			select {
			case attempted <- struct{}{}:
			default:
			}
		}
	})
	done := make(chan error, 1)
	writer := events.NewEventWriter(logsRoot)
	go func() { done <- AssociateCommit(hookCtx, fx.ws, writer, item, "abc1234", "msg", "author") }()
	select {
	case <-attempted:
	case <-time.After(10 * time.Second):
		t.Fatal("AssociateCommit never attempted the lifecycle-global lock")
	}

	// While AssociateCommit waits for lifecycle-global it must not hold the
	// item-log lock, otherwise it inverts against reconstitution/adoption.
	acquired := make(chan func(), 1)
	go func() {
		_, unlock, lockErr := events.LockItemLogCrossProcess(ctx, locksRoot, logsRoot, item)
		if lockErr == nil {
			acquired <- unlock
		}
	}()
	select {
	case unlock := <-acquired:
		unlock()
	case <-time.After(5 * time.Second):
		t.Fatal("item-log lock is held while waiting for lifecycle-global (lock-order inversion)")
	}

	released = true
	require.NoError(t, releaseGlobal())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("AssociateCommit did not finish after lifecycle-global was released")
	}
}

func TestAdoptItemMarksStashLockHeldBeforeRename(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	other := createIndexedFeature(t, fx.ws, "Adoption target")
	var stashHeld, called bool
	_, err := AdoptItemWithOptions(ctx, fx.ws, fx.tasks[0].ID, other.ID, AdoptItemOptions{
		BeforeRename: func(hookCtx context.Context, oldID, newID string) error {
			called = true
			stashHeld = stashLockHeld(hookCtx)
			assert.NotEqual(t, oldID, newID)
			return nil
		},
	})
	require.NoError(t, err)
	require.True(t, called, "adoption must allocate a new ID in the default layout")
	assert.True(t, stashHeld, "standalone adoption must mark the context once it holds the stash lock")
}
