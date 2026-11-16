package core

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/hooks"
)

func TestU5_CrossBranchAndMultiSessionArtifactIDSequenceReservation(t *testing.T) {
	t.Run("cross-branch-sequence-reservation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		f1, err := CreateArtifact(ctx, ws, "Branch A Feature", "feature")
		require.NoError(t, err)
		require.Equal(t, "001-F", f1.ID)

		t1, err := CreateArtifact(ctx, ws, "Branch A Task", "task", WithParent(f1.ID))
		require.NoError(t, err)
		require.Equal(t, "001.001-T", t1.ID)

		// Simulate switching to branch-b where branch-a's queue files do not exist
		// and SQLite index is rehydrated from the branch-b working tree.
		f1Path, err := FindArtifactPath(ctx, ws, f1.ID)
		require.NoError(t, err)
		t1Path, err := FindArtifactPath(ctx, ws, t1.ID)
		require.NoError(t, err)
		f1Bytes, err := os.ReadFile(f1Path)
		require.NoError(t, err)
		require.NoError(t, os.Remove(t1Path))
		require.NoError(t, os.Remove(f1Path))
		// Remove branch-a logs too so only .locks/id-sequences.json carries the reservation.
		require.NoError(t, os.RemoveAll(WorkspaceLogsRoot(ws.RootPath)))
		require.NoError(t, os.MkdirAll(WorkspaceLogsRoot(ws.RootPath), 0o755))

		_, err = ws.DB.ExecContext(ctx, `DELETE FROM items; DELETE FROM item_logs; DELETE FROM item_log_entries;`)
		require.NoError(t, err)

		f2, err := CreateArtifact(ctx, ws, "Branch B Feature", "feature")
		require.NoError(t, err)
		assert.Equal(t, "002-F", f2.ID, "branch-b must reserve 001-F and allocate 002-F")

		t2, err := CreateArtifact(ctx, ws, "Branch B Task", "task", WithParent(f2.ID))
		require.NoError(t, err)
		assert.Equal(t, "002.001-T", t2.ID)

		// Restore 001-F.md and verify child under 001-F advances to 001.002-T (never reusing 001.001-T).
		require.NoError(t, os.WriteFile(f1Path, f1Bytes, 0o644))
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, f1))
		t1b, err := CreateArtifact(ctx, ws, "Branch A Second Task", "task", WithParent(f1.ID))
		require.NoError(t, err)
		assert.Equal(t, "001.002-T", t1b.ID)
	})

	t.Run("concurrent-multisession-creates-no-collision", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		parent, err := CreateArtifact(ctx, ws, "Concurrent Parent", "feature")
		require.NoError(t, err)

		const n = 10
		var wg sync.WaitGroup
		errs := make(chan error, n*2)
		featureIDs := make(chan string, n)
		taskIDs := make(chan string, n)

		for i := range n {
			wg.Add(2)
			go func(idx int) {
				defer wg.Done()
				f, createErr := CreateArtifact(ctx, ws, fmt.Sprintf("Concurrent Feature %d", idx), "feature")
				if createErr != nil {
					errs <- createErr
					return
				}
				featureIDs <- f.ID
			}(i)
			go func(idx int) {
				defer wg.Done()
				task, createErr := CreateArtifact(ctx, ws, fmt.Sprintf("Concurrent Task %d", idx), "task", WithParent(parent.ID))
				if createErr != nil {
					errs <- createErr
					return
				}
				taskIDs <- task.ID
			}(i)
		}
		wg.Wait()
		close(errs)
		close(featureIDs)
		close(taskIDs)

		for err := range errs {
			require.NoError(t, err)
		}
		seenFeatures := map[string]bool{}
		for id := range featureIDs {
			seenFeatures[id] = true
		}
		seenTasks := map[string]bool{}
		for id := range taskIDs {
			seenTasks[id] = true
		}
		assert.Len(t, seenFeatures, n)
		assert.Len(t, seenTasks, n)
	})

	t.Run("compensated-rollback-on-hook-veto-and-db-failure", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		f1, err := CreateArtifact(ctx, ws, "Baseline Feature", "feature")
		require.NoError(t, err)
		require.Equal(t, "001-F", f1.ID)

		seqPath := filepath.Join(WorkspaceLocksRoot(ws.RootPath), "id-sequences.json")
		preimage, err := os.ReadFile(seqPath)
		require.NoError(t, err, "CreateArtifact must persist id-sequences.json")

		// 1. Pre-create hook veto leaves sequences untouched.
		origHooks := ws.HookRunner
		vetoRunner := hooks.NewHookRunner()
		vetoRunner.Register(hooks.HookCreateArtifact, hooks.PhasePre, hooks.HookRegistration{
			Name: "veto-create",
			Fn: func(context.Context, hooks.HookContext) error {
				return errors.New("vetoed by pre-create hook")
			},
		})
		ws.HookRunner = vetoRunner
		_, err = CreateArtifact(ctx, ws, "Vetoed Feature", "feature")
		require.Error(t, err)
		afterVeto, err := os.ReadFile(seqPath)
		require.NoError(t, err)
		assert.Equal(t, preimage, afterVeto, "pre-create hook veto must leave id-sequences.json untouched")
		_, vetoStatErr := os.Stat(filepath.Join(WorkspaceStorageRoot(ws.RootPath), "queue", "002-F.md"))
		assert.True(t, os.IsNotExist(vetoStatErr), "pre-create hook veto must not create 002-F.md")
		ws.HookRunner = origHooks

		// 2. DB failure rolls back file-write AND id-sequence-reservation without burning 002-F.
		dbPath := filepath.Join(WorkspaceStorageRoot(ws.RootPath), "backlogit.db")
		require.NoError(t, ws.DB.Close())

		_, err = CreateArtifact(ctx, ws, "Failed DB Feature", "feature")
		require.Error(t, err)

		afterDBFail, err := os.ReadFile(seqPath)
		require.NoError(t, err)
		assert.Equal(t, preimage, afterDBFail, "DB failure must roll back id-sequences.json to exact preimage")
		_, statErr := os.Stat(filepath.Join(WorkspaceStorageRoot(ws.RootPath), "queue", "002-F.md"))
		assert.True(t, os.IsNotExist(statErr), "DB failure must roll back 002-F.md")

		reopened, err := bldb.Open(dbPath)
		require.NoError(t, err)
		ws.DB = reopened

		f2, err := CreateArtifact(ctx, ws, "Recovered Next Feature", "feature")
		require.NoError(t, err)
		assert.Equal(t, "002-F", f2.ID, "rolled-back 002-F must not be burned")
	})

	t.Run("ast-checks-id-sequence-reservation-step-and-seam", func(t *testing.T) {
		artifactsFile, err := parser.ParseFile(token.NewFileSet(), "artifacts.go", nil, 0)
		require.NoError(t, err)

		var createFn *ast.FuncDecl
		for _, decl := range artifactsFile.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "CreateArtifact" {
				createFn = fn
				break
			}
		}
		require.NotNil(t, createFn)

		var foundStep bool
		ast.Inspect(createFn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if ok && lit.Kind == token.STRING && lit.Value == `"id-sequence-reservation"` {
				foundStep = true
			}
			return true
		})
		assert.True(t, foundStep, "CreateArtifact must include compensated MutationStep \"id-sequence-reservation\"")

		idHistFile, err := parser.ParseFile(token.NewFileSet(), "id_history.go", nil, 0)
		require.NoError(t, err)
		var hasLockWait, hasSeam bool
		for _, decl := range idHistFile.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if name.Name == "idAllocatorLockBoundedWait" {
						hasLockWait = true
					}
					if name.Name == "writeIDSequenceReservationFn" {
						hasSeam = true
					}
				}
			}
		}
		assert.True(t, hasLockWait, "id_history.go must declare idAllocatorLockBoundedWait")
		assert.True(t, hasSeam, "id_history.go must declare writeIDSequenceReservationFn seam")
	})

	t.Run("rejects-symlinked-id-sequences-and-allocator-lock", func(t *testing.T) {
		for _, leaf := range []string{"id-sequences.json", "id_allocator.lock"} {
			t.Run(leaf, func(t *testing.T) {
				ws := setupShipmentWorkspace(t)
				ctx := context.Background()
				locksDir := WorkspaceLocksRoot(ws.RootPath)
				require.NoError(t, os.MkdirAll(locksDir, 0o755))

				outside := filepath.Join(t.TempDir(), "outside.txt")
				require.NoError(t, os.WriteFile(outside, []byte("untouched"), 0o600))
				target := filepath.Join(locksDir, leaf)

				if runtime.GOOS == "windows" {
					out, linkErr := exec.Command("cmd", "/c", "mklink", target, outside).CombinedOutput()
					if linkErr != nil {
						t.Skipf("file symlink requires privilege on Windows: %v (%s)", linkErr, out)
					}
				} else {
					require.NoError(t, os.Symlink(outside, target))
				}

				_, err := CreateArtifact(ctx, ws, "Blocked By Symlink", "feature")
				require.Error(t, err)
				assert.True(t, errors.Is(err, blerrors.ErrValidation) || errors.Is(err, blerrors.ErrWriteNotApplied),
					"unexpected error: %v", err)
				data, readErr := os.ReadFile(outside)
				require.NoError(t, readErr)
				assert.Equal(t, "untouched", string(data))
			})
		}
	})
}
