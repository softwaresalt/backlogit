package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

func TestReviewAssociateCommitPreheldCRequiresGlobal(t *testing.T) {
	for _, kind := range []string{"shipment", "member"} {
		t.Run(kind, func(t *testing.T) {
			fx := newReconstituteFixture(t)
			id := fx.source.ID
			if kind == "member" {
				id = fx.tasks[0].ID
			}

			path, err := FindArtifactPath(context.Background(), fx.ws, id)
			require.NoError(t, err)
			before, err := os.ReadFile(path)
			require.NoError(t, err)
			// Hold global independently, as an archive writer would while
			// waiting for C. A C-only call must not even attempt global.
			_, releaseGlobal, err := lockShipmentLifecycleGlobal(context.Background(), fx.ws)
			require.NoError(t, err)
			defer func() { require.NoError(t, releaseGlobal()) }()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
				t.Errorf("C-only caller attempted lifecycle global: %s", phase)
			})
			ctx, releaseC, err := events.LockItemLogCrossProcess(ctx,
				WorkspaceLocksRoot(fx.ws.RootPath), WorkspaceLogsRoot(fx.ws.RootPath), id)
			require.NoError(t, err)
			defer releaseC()
			writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
			err = AssociateCommit(ctx, fx.ws, writer, id, strings.Repeat("b", 40), "", "")
			var orderErr *ShipmentLifecycleLockOrderError
			require.ErrorAs(t, err, &orderErr)
			after, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, before, after)
			var count int
			require.NoError(t, fx.ws.DB.QueryRow("SELECT COUNT(*) FROM commit_links WHERE item_id = ?", id).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestReviewAssociateCommitPreheldGlobalAndC(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx, releaseGlobal, err := lockShipmentLifecycleGlobal(context.Background(), fx.ws)
	require.NoError(t, err)
	defer func() { require.NoError(t, releaseGlobal()) }()
	ctx, releaseC, err := events.LockItemLogCrossProcess(ctx,
		WorkspaceLocksRoot(fx.ws.RootPath), WorkspaceLogsRoot(fx.ws.RootPath), fx.source.ID)
	require.NoError(t, err)
	defer releaseC()
	writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
	require.NoError(t, AssociateCommit(ctx, fx.ws, writer, fx.source.ID, strings.Repeat("c", 40), "", ""))
}

func TestReviewPersistMemberPreheldCDoesNotAcquireGlobal(t *testing.T) {
	// A nonmember classified by AssociateCommit can become a member before
	// persist rechecks membership. The final acquisition boundary must also
	// reject C -> global, rather than relying on the earlier classification.
	fx := newReconstituteFixture(t)
	_, releaseGlobal, err := lockShipmentLifecycleGlobal(context.Background(), fx.ws)
	require.NoError(t, err)
	defer func() { require.NoError(t, releaseGlobal()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
		t.Errorf("persist attempted global while holding C: %s", phase)
	})
	ctx, releaseC, err := events.LockItemLogCrossProcess(ctx,
		WorkspaceLocksRoot(fx.ws.RootPath), WorkspaceLogsRoot(fx.ws.RootPath), fx.tasks[0].ID)
	require.NoError(t, err)
	defer releaseC()
	err = persistArtifact(ctx, fx.ws, cloneArtifact(fx.tasks[0]), false)
	var orderErr *ShipmentLifecycleLockOrderError
	require.ErrorAs(t, err, &orderErr)
}

func TestReviewAssociateCommitReadsRecoveredManifest(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	path, err := FindArtifactPath(ctx, fx.ws, fx.source.ID)
	require.NoError(t, err)
	preimage, err := os.ReadFile(path)
	require.NoError(t, err)
	originalItems := NormalizeShipmentItems(fx.source)

	torn := cloneArtifact(fx.source)
	torn.CustomFields["items"] = removeString(originalItems, fx.tasks[0].ID)
	require.NoError(t, persistArtifact(ctx, fx.ws, torn, false))

	corr := "0000000000000000000000000000d001"
	journal := reconstituteRollbackJournal(fx, corr)
	relativePath, err := filepath.Rel(fx.ws.StorageRoot, path)
	require.NoError(t, err)
	journal.Reconstitute.Images = []reconstituteFileImage{{
		Path: filepath.ToSlash(relativePath), Exists: true, Content: preimage,
	}}
	_, err = writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr), journal)
	require.NoError(t, err)

	writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
	require.NoError(t, AssociateCommit(ctx, fx.ws, writer, fx.source.ID, strings.Repeat("a", 40), "review", "test"))
	recovered, err := findArtifact(ctx, fx.ws, fx.source.ID)
	require.NoError(t, err)
	require.Equal(t, originalItems, NormalizeShipmentItems(recovered))
}

func TestReviewReconstitutionRecoveryWithAbandonedStashSidecar(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	corr := "0000000000000000000000000000d002"
	_, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr),
		reconstituteRollbackJournal(fx, corr))
	require.NoError(t, err)
	lockPath := StashFilePath(fx.ws.RootPath) + ".lock"
	require.NoError(t, os.WriteFile(lockPath, nil, 0o644))
	old := time.Now().Add(-2 * staleLockTTL)
	require.NoError(t, os.Chtimes(lockPath, old, old))

	reopened, err := NewWorkspace(ctx, fx.ws.RootPath)
	require.NoError(t, err, "recovery should handle a sidecar left by a crashed reconstitution")
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
}

func reviewStorageTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := make(map[string]string)
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			tree[rel] = "<directory>"
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[rel] = string(content)
		return nil
	}))
	return tree
}

func TestReviewColdReadOnlyPreviewDoesNotChangeStorageTree(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	require.NoError(t, fx.ws.Close())
	dbPath := filepath.Join(fx.ws.StorageRoot, "backlogit.db")
	require.NoFileExists(t, dbPath+"-wal")
	require.NoFileExists(t, dbPath+"-shm")
	before := reviewStorageTree(t, fx.ws.StorageRoot)

	ro, err := NewReadOnlyIndexWorkspace(ctx, fx.ws.RootPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ro.Close() })
	_, err = ReconstituteShipments(ctx, ro, ReconstituteOptions{PlanYAML: fx.splitPlanYAML(), DryRun: true})
	require.NoError(t, err)
	after := reviewStorageTree(t, fx.ws.StorageRoot)
	require.True(t, len(before) == len(after), "storage entry count changed: before=%d after=%d", len(before), len(after))
	for path, content := range before {
		require.Contains(t, after, path)
		require.True(t, content == after[path], "storage entry changed: %s", path)
	}
}

func TestU1_CommitWritersShipmentMemberLockOrderGlobalBeforeItemLog(t *testing.T) {
	t.Run("a_preheld_c_without_global_fails_closed", func(t *testing.T) {
		writers := []struct {
			name string
			call func(context.Context, *reconstituteFixture, string, string) error
		}{
			{
				name: "AssociateCommit",
				call: func(ctx context.Context, fx *reconstituteFixture, memberID, sha string) error {
					writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
					return AssociateCommit(ctx, fx.ws, writer, memberID, sha, "msg", "author")
				},
			},
			{
				name: "LinkCommit",
				call: func(ctx context.Context, fx *reconstituteFixture, memberID, sha string) error {
					return LinkCommit(ctx, fx.ws.DB, fx.ws, memberID, sha, "msg", "author")
				},
			},
		}

		for _, tc := range writers {
			t.Run(tc.name, func(t *testing.T) {
				fx := newReconstituteFixture(t)
				memberID := fx.tasks[0].ID

				path, err := FindArtifactPath(context.Background(), fx.ws, memberID)
				require.NoError(t, err)
				before, err := os.ReadFile(path)
				require.NoError(t, err)

				// Hold lifecycle-global independently, as an archive or lifecycle writer
				// would while waiting on C. A C-only commit writer must fail closed
				// without attempting lifecycle-global or mutating commit_links.
				_, releaseGlobal, err := lockShipmentLifecycleGlobal(context.Background(), fx.ws)
				require.NoError(t, err)
				defer func() { require.NoError(t, releaseGlobal()) }()

				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
					t.Errorf("%s C-only caller attempted lifecycle global: %s", tc.name, phase)
				})
				ctx, releaseC, err := events.LockItemLogCrossProcess(ctx,
					WorkspaceLocksRoot(fx.ws.RootPath), WorkspaceLogsRoot(fx.ws.RootPath), memberID)
				require.NoError(t, err)
				defer releaseC()

				err = tc.call(ctx, fx, memberID, strings.Repeat("b", 40))
				var orderErr *ShipmentLifecycleLockOrderError
				require.ErrorAs(t, err, &orderErr)

				after, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, before, after)

				var count int
				require.NoError(t, fx.ws.DB.QueryRow(`SELECT COUNT(*) FROM commit_links WHERE item_id = ?`, memberID).Scan(&count))
				require.Zero(t, count, "%s must not mutate commit_links when failing closed on C-only lock order", tc.name)
			})
		}
	})

	t.Run("b_waits_on_global_before_acquiring_item_log", func(t *testing.T) {
		writers := []struct {
			name string
			sha  string
			call func(context.Context, *reconstituteFixture, string, string) error
		}{
			{
				name: "AssociateCommit",
				sha:  strings.Repeat("1", 40),
				call: func(ctx context.Context, fx *reconstituteFixture, memberID, sha string) error {
					writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
					return AssociateCommit(ctx, fx.ws, writer, memberID, sha, "msg", "author")
				},
			},
			{
				name: "LinkCommit",
				sha:  strings.Repeat("2", 40),
				call: func(ctx context.Context, fx *reconstituteFixture, memberID, sha string) error {
					return LinkCommit(ctx, fx.ws.DB, fx.ws, memberID, sha, "msg", "author")
				},
			},
		}

		for _, tc := range writers {
			t.Run(tc.name, func(t *testing.T) {
				fx := newReconstituteFixture(t)
				memberID := fx.tasks[0].ID

				globalCtx, releaseGlobal, err := lockShipmentLifecycleGlobal(context.Background(), fx.ws)
				require.NoError(t, err)
				globalReleased := false
				releaseGlobalOnce := func() {
					if !globalReleased {
						globalReleased = true
						require.NoError(t, releaseGlobal())
					}
				}

				var cAttempts atomic.Int32
				restoreItemLogHook := events.SetItemLogLockBarrierHookForTest(func(itemID string) {
					if itemID == memberID {
						cAttempts.Add(1)
					}
				})
				t.Cleanup(restoreItemLogHook)

				globalAttempted := make(chan struct{})
				globalAcquired := make(chan struct{})
				var attemptOnce, acquireOnce sync.Once
				writerCtx := context.WithValue(context.Background(), shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
					switch phase {
					case "attempt":
						attemptOnce.Do(func() { close(globalAttempted) })
					case "acquired":
						acquireOnce.Do(func() { close(globalAcquired) })
					}
				})

				done := make(chan error, 1)
				writerReturned := false
				go func() {
					done <- tc.call(writerCtx, fx, memberID, tc.sha)
				}()
				// Registered after restoreItemLogHook so LIFO cleanup drains the
				// goroutine before restoring the package-global hook.
				t.Cleanup(func() {
					releaseGlobalOnce()
					if writerReturned {
						return
					}
					select {
					case <-done:
					case <-time.After(u172ObservationWait):
						t.Errorf("%s goroutine did not exit during cleanup", tc.name)
					}
				})

				select {
				case <-globalAttempted:
				case earlyErr := <-done:
					writerReturned = true
					t.Fatalf("%s bypassed lifecycle-global while a competing goroutine held it (err=%v, cAttempts=%d)",
						tc.name, earlyErr, cAttempts.Load())
				case <-time.After(u172ObservationWait):
					t.Fatalf("%s never attempted lifecycle-global", tc.name)
				}

				select {
				case <-globalAcquired:
					t.Fatalf("%s acquired lifecycle-global while competitor held it", tc.name)
				case earlyErr := <-done:
					writerReturned = true
					t.Fatalf("%s finished while lifecycle-global was held (err=%v)", tc.name, earlyErr)
				case <-time.After(100 * time.Millisecond):
				}

				require.Zero(t, cAttempts.Load(), "%s must not attempt item-log lock C before acquiring lifecycle-global", tc.name)

				// The lifecycle-global holder must be able to acquire C while still
				// holding lifecycle-global, proving no C-then-global inversion.
				// Note: the holder's own LockItemLogCrossProcess call triggers the
				// package-wide barrier hook once.
				_, releaseC, err := events.LockItemLogCrossProcess(globalCtx,
					WorkspaceLocksRoot(fx.ws.RootPath), WorkspaceLogsRoot(fx.ws.RootPath), memberID)
				require.NoError(t, err, "lifecycle-global holder must acquire C freely while commit writer waits on lifecycle-global")
				releaseC()

				afterHolderC := cAttempts.Load()
				require.Equal(t, int32(1), afterHolderC, "%s attempted C while lifecycle-global remained held", tc.name)
				releaseGlobalOnce()

				select {
				case writerErr := <-done:
					writerReturned = true
					require.NoError(t, writerErr)
				case <-time.After(u172ObservationWait):
					t.Fatalf("%s did not finish after lifecycle-global was released", tc.name)
				}

				select {
				case <-globalAcquired:
				default:
					t.Fatalf("%s never recorded lifecycle-global acquisition after release", tc.name)
				}
				require.Greater(t, cAttempts.Load(), afterHolderC, "%s never acquired item-log lock C after lifecycle-global", tc.name)

				links, err := GetCommitLinks(context.Background(), fx.ws.DB, memberID)
				require.NoError(t, err)
				require.Len(t, links, 1)
				assert.Equal(t, tc.sha, links[0].CommitSHA)
			})
		}
	})

	t.Run("c_associate_commit_compensation_under_global_then_c", func(t *testing.T) {
		t.Run("commit-links-upsert", func(t *testing.T) {
			ctx := context.Background()
			fx := newReconstituteFixture(t)
			memberID := fx.tasks[0].ID

			path, err := FindArtifactPath(ctx, fx.ws, memberID)
			require.NoError(t, err)
			before, err := os.ReadFile(path)
			require.NoError(t, err)

			_, err = fx.ws.DB.ExecContext(ctx, `DROP TABLE commit_links`)
			require.NoError(t, err)

			var mu sync.Mutex
			var trace []string
			record := func(step string) {
				mu.Lock()
				trace = append(trace, step)
				mu.Unlock()
			}

			restoreItemLogHook := events.SetItemLogLockBarrierHookForTest(func(itemID string) {
				if itemID == memberID {
					record("c:acquire")
				}
			})
			t.Cleanup(restoreItemLogHook)

			previousPersistHook := persistArtifactPreLockHook
			persistArtifactPreLockHook = func(id string) {
				if id == memberID {
					record("persist")
				}
			}
			t.Cleanup(func() { persistArtifactPreLockHook = previousPersistHook })

			observedCtx := context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
				record("global:" + phase)
			})

			writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
			err = AssociateCommit(observedCtx, fx.ws, writer, memberID, strings.Repeat("d", 40), "msg", "author")
			require.Error(t, err)

			var partialErr *blerrors.MutationPartialError
			require.ErrorAs(t, err, &partialErr)
			assert.Equal(t, []string{"frontmatter-scalar"}, partialErr.Completed)
			assert.Equal(t, "commit-links-upsert", partialErr.FailedStep)
			assert.Equal(t, "compensated", partialErr.CompensationState)
			assert.Equal(t, "not-applied", partialErr.Class)

			after, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, before, after)

			assert.Equal(t, []string{
				"global:attempt",
				"global:acquired",
				"c:acquire",
				"persist",
				"persist",
			}, trace)
		})

		t.Run("jsonl-append", func(t *testing.T) {
			ctx := context.Background()
			fx := newReconstituteFixture(t)
			memberID := fx.tasks[0].ID
			priorSHA := strings.Repeat("e", 40)

			goodWriter := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
			require.NoError(t, AssociateCommit(ctx, fx.ws, goodWriter, memberID, priorSHA, "prior msg", "prior@example.com"))

			path, err := FindArtifactPath(ctx, fx.ws, memberID)
			require.NoError(t, err)
			before, err := os.ReadFile(path)
			require.NoError(t, err)

			logsPath := filepath.Join(t.TempDir(), "not-a-dir")
			require.NoError(t, os.WriteFile(logsPath, []byte("blocking file"), 0o644))
			badWriter := events.NewEventWriter(logsPath)

			var mu sync.Mutex
			var trace []string
			record := func(step string) {
				mu.Lock()
				trace = append(trace, step)
				mu.Unlock()
			}

			restoreItemLogHook := events.SetItemLogLockBarrierHookForTest(func(itemID string) {
				if itemID == memberID {
					record("c:acquire")
				}
			})
			t.Cleanup(restoreItemLogHook)

			previousPersistHook := persistArtifactPreLockHook
			persistArtifactPreLockHook = func(id string) {
				if id == memberID {
					record("persist")
				}
			}
			t.Cleanup(func() { persistArtifactPreLockHook = previousPersistHook })

			observedCtx := context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
				record("global:" + phase)
			})

			err = AssociateCommit(observedCtx, fx.ws, badWriter, memberID, strings.Repeat("f", 40), "new msg", "new@example.com")
			require.Error(t, err)

			var partialErr *blerrors.MutationPartialError
			require.ErrorAs(t, err, &partialErr)
			assert.Equal(t, []string{"frontmatter-scalar", "commit-links-upsert"}, partialErr.Completed)
			assert.Equal(t, "jsonl-append", partialErr.FailedStep)
			assert.Equal(t, "compensated", partialErr.CompensationState)
			assert.Equal(t, "not-applied", partialErr.Class)

			after, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, before, after)

			links, err := GetCommitLinks(ctx, fx.ws.DB, memberID)
			require.NoError(t, err)
			require.Len(t, links, 1)
			assert.Equal(t, priorSHA, links[0].CommitSHA)
			assert.Equal(t, "prior msg", links[0].Message)
			assert.Equal(t, "prior@example.com", links[0].Author)

			// The second c:acquire is badWriter.AppendEvent acquiring its own
			// logsPath lock before failing; the compensation persist runs
			// afterward under the single global -> C hold without re-acquiring global.
			assert.Equal(t, []string{
				"global:attempt",
				"global:acquired",
				"c:acquire",
				"persist",
				"c:acquire",
				"persist",
			}, trace)
		})
	})
}
