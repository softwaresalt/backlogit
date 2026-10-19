package core

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
)

// 200.006-T (CY-B B6): archive events are causally ordered against a concurrent
// C-then-B writer.
//
// ArchiveItem used to take B, mutate, release B, and only then request C for the
// archive event. A C-then-B writer such as AssociateCommit could append its own
// event in that gap, so the log recorded the writer's event between the archive
// mutation and the archive event. The fix is the canonical order (lifecycle
// global, A, C, B): ArchiveItem takes C for its whole archive set first, mutates
// under B, and appends the archive event under that C.
//
// The interleaving is forced with the existing test-only barrier
// artifactMutationLockBarrierHook, which fires right after B is acquired. The
// test holds C for one archive-set item and starts ArchiveItem. An ArchiveItem
// that follows the canonical order must be waiting for that C and never reach B
// while the test holds it. If it does reach B, it is mutating ahead of C and a
// concurrent writer's event can land between its mutation and its event. The
// test then runs AssociateCommit under the held C and checks the item log order.
//
// No production hook is added. This test installs a package-global hook and
// therefore must not call t.Parallel.

// archiveAdvanceWindow bounds how long the test waits to see an ArchiveItem
// reach B while the test holds C. Nothing contends the locks during the window,
// so an ArchiveItem that follows C-before-B simply stays blocked on C; the window
// only bounds the negative observation. A violating ArchiveItem reaches B within
// milliseconds and ends the wait early.
const archiveAdvanceWindow = time.Second

type archiveEventOrderFixture struct {
	ws *Workspace
	// archiveID is the ArchiveItem target.
	archiveID string
	// heldID is the archive-set item whose C the test holds.
	heldID string
	// scope is every artifact ID ArchiveItem may take B for.
	scope   []string
	cascade bool
}

func newStandaloneArchiveEventOrderFixture(t *testing.T) archiveEventOrderFixture {
	t.Helper()
	ws := setupShipmentWorkspace(t)
	item, err := CreateArtifact(context.Background(), ws, "archive event order standalone", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(context.Background(), ws.DB, item))
	return archiveEventOrderFixture{ws: ws, archiveID: item.ID, heldID: item.ID, scope: []string{item.ID}}
}

func newCascadeArchiveEventOrderFixture(t *testing.T) archiveEventOrderFixture {
	t.Helper()
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	parent, err := CreateArtifact(ctx, ws, "archive event order parent", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, parent))
	child, err := CreateArtifact(ctx, ws, "archive event order child", "task", WithParent(parent.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, child))
	return archiveEventOrderFixture{
		ws:        ws,
		archiveID: parent.ID,
		heldID:    child.ID,
		scope:     []string{parent.ID, child.ID},
		cascade:   true,
	}
}

func TestArchiveItem_ArchiveEventCannotBeSplitFromMutationByCThenBWriter(t *testing.T) {
	for name, build := range map[string]func(*testing.T) archiveEventOrderFixture{
		"standalone": newStandaloneArchiveEventOrderFixture,
		"cascade":    newCascadeArchiveEventOrderFixture,
	} {
		t.Run(name, func(t *testing.T) {
			f := build(t)
			ws := f.ws
			ctx := context.Background()
			logsDir := WorkspaceLogsRoot(ws.RootPath)

			cLockedCtx, unlockC, err := events.LockItemLogCrossProcess(ctx, WorkspaceLocksRoot(ws.RootPath), logsDir, f.heldID)
			require.NoError(t, err)
			var unlockOnce sync.Once
			releaseC := func() {
				unlockOnce.Do(func() {
					unlockC()
				})
			}
			t.Cleanup(releaseC)

			inScope := make(map[string]struct{}, len(f.scope))
			for _, id := range f.scope {
				inScope[id] = struct{}{}
			}
			var observing atomic.Bool
			observing.Store(true)
			reachedB := make(chan string, 1)
			previousHook := artifactMutationLockBarrierHook
			artifactMutationLockBarrierHook = func(artifactID string) {
				if _, ok := inScope[artifactID]; !ok || !observing.Load() {
					return
				}
				select {
				case reachedB <- artifactID:
				default:
				}
			}

			archiveDone := make(chan error, 1)
			archiveStarted := false
			// Runs before the hook restore below: it releases C, drains the
			// ArchiveItem goroutine, and only then restores the global hook that
			// goroutine reads, so an early t.Fatal never leaks it.
			t.Cleanup(func() {
				observing.Store(false)
				if archiveStarted {
					releaseC()
					select {
					case <-archiveDone:
					case <-time.After(u172ObservationWait):
						t.Errorf("ArchiveItem goroutine did not exit during cleanup")
					}
				}
				artifactMutationLockBarrierHook = previousHook
			})

			archiveStarted = true
			go func() {
				_, archiveErr := ArchiveItem(ctx, ws.DB, ws, f.archiveID, WithCascade(f.cascade))
				archiveDone <- archiveErr
			}()

			var archiveReachedB string
			select {
			case archiveReachedB = <-reachedB:
			case <-time.After(archiveAdvanceWindow):
			}
			observing.Store(false)

			// The writer holds C (through cLockedCtx) and then takes B, exactly the
			// C-then-B order AssociateCommit follows. It runs while the test still
			// holds C, so any archive mutation that already happened is ordered
			// ahead of this event.
			ew := NewWorkspaceEventWriter(ws, logsDir)
			sha := strings.Repeat("a", 40)
			writerErr := AssociateCommit(cLockedCtx, ws, ew, f.heldID, sha, "concurrent writer", "b6-tester")

			releaseC()
			select {
			case archiveErr := <-archiveDone:
				archiveStarted = false
				require.NoError(t, archiveErr)
			case <-time.After(u172ObservationWait):
				t.Fatal("ArchiveItem did not finish after the item-log lock was released")
			}

			require.Empty(t, archiveReachedB,
				"ArchiveItem acquired B for %q while another writer held C for %q: it mutates before it holds C, so a C-then-B writer can append its event between the archive mutation and the archive event",
				archiveReachedB, f.heldID)
			require.NoError(t, writerErr, "the C-then-B writer must complete before the archive mutation it raced")

			types := readU172ItemLogEventTypes(t, ws, f.heldID)
			writerIndex, archivedIndex := -1, -1
			for i, eventType := range types {
				switch eventType {
				case "commit_tracked":
					writerIndex = i
				case "archived":
					archivedIndex = i
				}
			}
			require.GreaterOrEqual(t, writerIndex, 0, "writer event missing from item log: %v", types)
			require.GreaterOrEqual(t, archivedIndex, 0, "archive event missing from item log: %v", types)
			require.Less(t, writerIndex, archivedIndex,
				"the archive event must follow the writer's event once the archive mutation follows the writer's mutation: %v", types)
		})
	}
}
