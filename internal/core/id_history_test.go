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

	"github.com/softwaresalt/backlogit/internal/config"
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

	t.Run("ledger-write-failure-compensates-file-and-db", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		f1, err := CreateArtifact(ctx, ws, "Baseline Before Ledger Failure", "feature")
		require.NoError(t, err)
		require.Equal(t, "001-F", f1.ID)

		seqPath := filepath.Join(WorkspaceLocksRoot(ws.RootPath), idSequenceLedgerFileName)
		preimage, err := os.ReadFile(seqPath)
		require.NoError(t, err)

		origWrite := writeIDSequenceReservationFn
		t.Cleanup(func() { writeIDSequenceReservationFn = origWrite })
		writeIDSequenceReservationFn = func(_ *Workspace, _ *os.Root, _ string, _ []byte) error {
			return fmt.Errorf("injected ledger failure: %w", blerrors.ErrWriteNotApplied)
		}

		_, err = CreateArtifact(ctx, ws, "Ledger Fail Feature", "feature")
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrWriteNotApplied)

		_, statErr := os.Stat(filepath.Join(WorkspaceStorageRoot(ws.RootPath), "queue", "002-F.md"))
		assert.True(t, os.IsNotExist(statErr), "ledger failure must compensate 002-F.md")
		_, dbErr := bldb.GetItem(ctx, ws.DB, "002-F")
		assert.ErrorIs(t, dbErr, blerrors.ErrNotFound, "ledger failure must compensate 002-F DB row")

		afterFail, err := os.ReadFile(seqPath)
		require.NoError(t, err)
		assert.Equal(t, preimage, afterFail)

		writeIDSequenceReservationFn = origWrite
		f2, err := CreateArtifact(ctx, ws, "Recovered After Ledger Failure", "feature")
		require.NoError(t, err)
		assert.Equal(t, "002-F", f2.ID)
	})

	t.Run("locks-directory-swap-rejected-without-escape", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		_, err := CreateArtifact(ctx, ws, "Baseline Before Swap", "feature")
		require.NoError(t, err)

		locksDir := WorkspaceLocksRoot(ws.RootPath)
		outsideDir := t.TempDir()
		backupDir := locksDir + ".orig"

		// Open a pinned locksRoot handle (with no open child file blocking directory rename on Windows),
		// then swap .locks on disk with a junction/symlink to outsideDir before write and rollback removal.
		storageHandle, locksRoot, locksDirAbs, err := openLocksRootForWrite(ws)
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = locksRoot.Close()
			_ = storageHandle.Close()
		})

		require.NoError(t, os.Rename(locksDir, backupDir))
		if runtime.GOOS == "windows" {
			out, linkErr := exec.Command("cmd", "/c", "mklink", "/J", locksDir, outsideDir).CombinedOutput()
			require.NoError(t, linkErr, "mklink /J failed: %s", out)
		} else {
			require.NoError(t, os.Symlink(outsideDir, locksDir))
		}

		writeErr := writeIDSequenceReservation(ws, locksRoot, locksDirAbs, []byte(`{"schema_version":1,"reserved_ids":["999-F"]}`+"\n"))
		removeErr := removeIDSequenceReservation(locksRoot, locksDirAbs)
		if runtime.GOOS == "windows" {
			require.Error(t, writeErr, "Windows .locks parent swap must fail closed on write")
			require.Error(t, removeErr, "Windows .locks parent swap must fail closed on rollback removal")
		}
		_, outsideErr := os.Stat(filepath.Join(outsideDir, idSequenceLedgerFileName))
		assert.True(t, os.IsNotExist(outsideErr), "parent .locks swap must never write id-sequences.json into outsideDir")
	})
}

func TestU5b_AdoptAndReconstituteSequenceReservationAndRollback(t *testing.T) {
	t.Run("adopt-and-create-serialize-and-rollback-restores-ledger", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		f1, err := CreateArtifact(ctx, ws, "Target Parent", "feature")
		require.NoError(t, err)
		f2, err := CreateArtifact(ctx, ws, "Source Parent", "feature")
		require.NoError(t, err)
		orphan, err := CreateArtifact(ctx, ws, "Adoptable Task", "task", WithParent(f2.ID))
		require.NoError(t, err)

		adopted, err := AdoptItemWithOptions(ctx, ws, orphan.ID, f1.ID, AdoptItemOptions{})
		require.NoError(t, err)
		require.Equal(t, "001.001-T", adopted.NewID)

		// Simulate branch switch removing 001.001-T.md and logs + DB rows.
		adoptedPath, err := FindArtifactPath(ctx, ws, adopted.NewID)
		require.NoError(t, err)
		require.NoError(t, os.Remove(adoptedPath))
		require.NoError(t, os.RemoveAll(WorkspaceLogsRoot(ws.RootPath)))
		require.NoError(t, os.MkdirAll(WorkspaceLogsRoot(ws.RootPath), 0o755))
		_, err = ws.DB.ExecContext(ctx, `DELETE FROM items WHERE id = ?; DELETE FROM item_logs; DELETE FROM item_log_entries;`, adopted.NewID)
		require.NoError(t, err)

		nextChild, err := CreateArtifact(ctx, ws, "Post-Adopt Child", "task", WithParent(f1.ID))
		require.NoError(t, err)
		assert.Equal(t, "001.002-T", nextChild.ID, "adopted ID 001.001-T must stay reserved across branch switches")

		// Concurrent AdoptItem and CreateArtifact under f1 must serialize on id_allocator.lock.
		t2, err := CreateArtifact(ctx, ws, "Concurrent Adopt Source", "task", WithParent(f2.ID))
		require.NoError(t, err)
		var wg sync.WaitGroup
		wg.Add(2)
		var adoptRes *AdoptItemResult
		var adoptErr, createErr error
		var createdTaskID string
		go func() {
			defer wg.Done()
			adoptRes, adoptErr = AdoptItemWithOptions(ctx, ws, t2.ID, f1.ID, AdoptItemOptions{})
		}()
		go func() {
			defer wg.Done()
			c, cErr := CreateArtifact(ctx, ws, "Concurrent Child", "task", WithParent(f1.ID))
			createErr = cErr
			if c != nil {
				createdTaskID = c.ID
			}
		}()
		wg.Wait()
		require.NoError(t, adoptErr)
		require.NoError(t, createErr)
		assert.NotEqual(t, adoptRes.NewID, createdTaskID)
		assert.ElementsMatch(t, []string{"001.003-T", "001.004-T"}, []string{adoptRes.NewID, createdTaskID})

		// Ledger failure during AdoptItem must roll back both adoption and id-sequences.json.
		t3, err := CreateArtifact(ctx, ws, "Fail Adopt Source", "task", WithParent(f2.ID))
		require.NoError(t, err)
		seqPath := filepath.Join(WorkspaceLocksRoot(ws.RootPath), idSequenceLedgerFileName)
		preimage, err := os.ReadFile(seqPath)
		require.NoError(t, err)

		origWrite := writeIDSequenceReservationFn
		writeIDSequenceReservationFn = func(_ *Workspace, _ *os.Root, _ string, _ []byte) error {
			return fmt.Errorf("injected adopt ledger write failure: %w", blerrors.ErrWriteNotApplied)
		}
		_, err = AdoptItemWithOptions(ctx, ws, t3.ID, f1.ID, AdoptItemOptions{})
		writeIDSequenceReservationFn = origWrite
		require.Error(t, err)

		afterFail, err := os.ReadFile(seqPath)
		require.NoError(t, err)
		assert.Equal(t, preimage, afterFail, "failed AdoptItem must restore id-sequences.json to exact preimage")
	})

	t.Run("nested-workspace-reconstitute-cross-branch-reservation", func(t *testing.T) {
		outer := t.TempDir()
		nested := filepath.Join(outer, "nested-ws")
		storage := filepath.Join(nested, ".backlogit")
		require.NoError(t, os.MkdirAll(storage, 0o755))
		require.NoError(t, config.WriteDefaults(storage))
		ctx := context.Background()
		ws, err := NewWorkspace(ctx, nested)
		require.NoError(t, err)
		ws.GateBroker = nil
		t.Cleanup(func() { require.NoError(t, ws.Close()) })

		f1, err := CreateArtifact(ctx, ws, "Orig Feature", "feature")
		require.NoError(t, err)
		t1, err := CreateArtifact(ctx, ws, "Task One", "task", WithParent(f1.ID))
		require.NoError(t, err)
		t2, err := CreateArtifact(ctx, ws, "Task Two", "task", WithParent(f1.ID))
		require.NoError(t, err)
		s1, err := CreateShipment(ctx, ws, "Source Shipment", []string{f1.ID, t1.ID, t2.ID})
		require.NoError(t, err)

		planYAML := fmt.Sprintf(`source_shipments:
  - %s
shipments:
  - id: %s
    feature:
      id: %s
    items:
      - %s
  - label: split-a
    title: Split A Shipment
    blocks_on:
      - %s
    feature:
      create:
        title: Split A Feature
    items:
      - %s
`, s1.ID, s1.ID, f1.ID, t1.ID, s1.ID, t2.ID)
		res, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: planYAML})
		require.NoError(t, err)
		require.Len(t, res.CreatedFeatures, 1)
		createdFeatID := res.CreatedFeatures[0].ID
		adoptedTaskID := res.AdoptedIDMap[t2.ID]
		require.Equal(t, "002-F", createdFeatID)
		require.Equal(t, "002.001-T", adoptedTaskID)

		// Verify id-sequences.json reserved both 002-F and 002.001-T.
		seqBytes, err := os.ReadFile(filepath.Join(WorkspaceLocksRoot(ws.RootPath), idSequenceLedgerFileName))
		require.NoError(t, err)
		assert.Contains(t, string(seqBytes), createdFeatID)
		assert.Contains(t, string(seqBytes), adoptedTaskID)

		// Simulate switching to another branch (removing 002.001-T and logs + DB).
		adoptedPath, findErr := FindArtifactPath(ctx, ws, adoptedTaskID)
		require.NoError(t, findErr)
		require.NoError(t, os.Remove(adoptedPath))
		require.NoError(t, os.RemoveAll(WorkspaceLogsRoot(ws.RootPath)))
		require.NoError(t, os.MkdirAll(WorkspaceLogsRoot(ws.RootPath), 0o755))
		_, err = ws.DB.ExecContext(ctx, `DELETE FROM items WHERE id = ?; DELETE FROM item_logs; DELETE FROM item_log_entries;`, adoptedTaskID)
		require.NoError(t, err)

		nextChild, err := CreateArtifact(ctx, ws, "Other Branch Child", "task", WithParent(createdFeatID))
		require.NoError(t, err)
		assert.Equal(t, "002.002-T", nextChild.ID, "reconstituted adopted ID 002.001-T must remain reserved across branch switch")
	})

	t.Run("ast-checks-u5b-ownership-locking-and-rollback", func(t *testing.T) {
		lifecycleFile, err := parser.ParseFile(token.NewFileSet(), "shipment_lifecycle.go", nil, 0)
		require.NoError(t, err)
		var adoptFn *ast.FuncDecl
		for _, decl := range lifecycleFile.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "adoptItem" {
				adoptFn = fn
				break
			}
		}
		require.NotNil(t, adoptFn)
		var callsAllocLock bool
		ast.Inspect(adoptFn.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "acquireIDAllocatorLock" {
				callsAllocLock = true
			}
			return true
		})
		assert.True(t, callsAllocLock, "adoptItem must acquire acquireIDAllocatorLock")

		execFile, err := parser.ParseFile(token.NewFileSet(), "shipment_reconstitute_exec.go", nil, 0)
		require.NoError(t, err)
		var hasBeforeCreate, hasUnreserve bool
		ast.Inspect(execFile, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				if id.Name == "shipmentOperationBeforeCreateContextKey" {
					hasBeforeCreate = true
				}
				if id.Name == "unreserveIDSequences" {
					hasUnreserve = true
				}
			}
			return true
		})
		assert.True(t, hasBeforeCreate, "shipment_reconstitute_exec.go must wire shipmentOperationBeforeCreateContextKey")
		assert.True(t, hasUnreserve, "shipment_reconstitute_exec.go must call unreserveIDSequences on rollback")
	})

	t.Run("reconstitute-rollback-unreserves-created-and-adopted-ids", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		f1, err := CreateArtifact(ctx, ws, "Orig Feature", "feature")
		require.NoError(t, err)
		t1, err := CreateArtifact(ctx, ws, "Task One", "task", WithParent(f1.ID))
		require.NoError(t, err)
		t2, err := CreateArtifact(ctx, ws, "Task Two", "task", WithParent(f1.ID))
		require.NoError(t, err)
		s1, err := CreateShipment(ctx, ws, "Source Shipment", []string{f1.ID, t1.ID, t2.ID})
		require.NoError(t, err)

		seqPath := filepath.Join(WorkspaceLocksRoot(ws.RootPath), idSequenceLedgerFileName)
		preimage, err := os.ReadFile(seqPath)
		require.NoError(t, err)

		// Fail on the 2nd reservation write inside ReconstituteShipments (after 002-F succeeded and
		// reserved 002-F, when AdoptItem attempts to reserve 002.001-T).
		origWrite := writeIDSequenceReservationFn
		calls := 0
		writeIDSequenceReservationFn = func(innerWS *Workspace, locksRoot *os.Root, locksDirAbs string, raw []byte) error {
			calls++
			if calls == 2 {
				return fmt.Errorf("injected reconstitute mid-flight ledger failure: %w", blerrors.ErrWriteNotApplied)
			}
			return origWrite(innerWS, locksRoot, locksDirAbs, raw)
		}
		t.Cleanup(func() { writeIDSequenceReservationFn = origWrite })

		planYAML := fmt.Sprintf(`source_shipments:
  - %s
shipments:
  - id: %s
    feature:
      id: %s
    items:
      - %s
  - label: split-a
    title: Split A Shipment
    blocks_on:
      - %s
    feature:
      create:
        title: Split A Feature
    items:
      - %s
`, s1.ID, s1.ID, f1.ID, t1.ID, s1.ID, t2.ID)
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: planYAML})
		writeIDSequenceReservationFn = origWrite
		require.Error(t, err)

		afterRollback, err := os.ReadFile(seqPath)
		require.NoError(t, err)
		assert.Equal(t, string(preimage), string(afterRollback),
			"reconstitute rollback must unreserve 002-F and restore exact preimage")
	})
}
