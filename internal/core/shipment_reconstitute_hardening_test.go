package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

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
