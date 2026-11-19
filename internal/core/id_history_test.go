package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

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
		require.Error(t, writeErr, ".locks parent swap must fail closed on write")
		require.Error(t, removeErr, ".locks parent swap must fail closed on rollback removal")
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

	t.Run("unreserve-selective-ids-preserves-retained-and-pending-journal-ids", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		for _, id := range []string{"001-F", "002-F", "003-F"} {
			pre, preErr := captureIDSequencePreimage(ctx, ws)
			require.NoError(t, preErr)
			require.NoError(t, reserveIDSequenceWithPreimage(ctx, ws, pre, id))
		}

		// Unreserving only 002-F must keep 001-F and 003-F in id-sequences.json.
		require.NoError(t, unreserveIDSequences(ctx, ws, "corr-1", []string{"002-F"}))
		storageHandle, locksRoot, locksDirAbs, err := openLocksRootReadOnly(ws)
		require.NoError(t, err)
		defer func() {
			_ = locksRoot.Close()
			_ = storageHandle.Close()
		}()
		exists, _, ledger, err := readIDSequenceLedger(locksRoot, locksDirAbs)
		require.NoError(t, err)
		require.True(t, exists)
		assert.Contains(t, ledger.ReservedIDs, "001-F")
		assert.NotContains(t, ledger.ReservedIDs, "002-F")
		assert.Contains(t, ledger.ReservedIDs, "003-F")

		// An uncommitted intent reconstitute journal containing 004-F must reserve 004-F in historicalIDsFromDisk.
		const corrID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		journal := shipmentLifecycleJournal{
			SchemaVersion:  "shipment-operation/v1",
			CorrelationID:  corrID,
			Phase:          shipmentOperationPhaseIntent,
			Operation:      shipmentOperationReconstitute,
			RecoveryPolicy: shipmentOperationRecoveryPolicyRollback,
			ShipmentID:     "001-S",
			Target:         shipmentOperationReconstitute,
			Reconstitute: &reconstituteJournalState{
				CreatedIDs: []string{"004-F"},
				Adoptions:  map[string]string{"001.001-T": "004.001-T"},
			},
		}
		_, err = writeShipmentLifecycleJournalForWorkspace(ws, shipmentLifecycleJournalName(corrID), journal)
		require.NoError(t, err)

		hist, err := historicalIDsFromDisk(ws)
		require.NoError(t, err)
		assert.Contains(t, hist, "004-F", "pending intent reconstitute journal must reserve 004-F")
		assert.Contains(t, hist, "004.001-T", "pending intent reconstitute journal must reserve adopted 004.001-T")
	})

	t.Run("scan-local-branch-artifact-ids-detects-other-branch-and-rejects-symlink-git", func(t *testing.T) {
		repo := t.TempDir()
		runGit := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
			cmd.Env = sanitizedGitBranchScanEnv()
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "git %v failed: %s", args, out)
		}
		runGit("init", "-b", "main")
		runGit("config", "user.name", "Test")
		runGit("config", "user.email", "test@example.com")

		storage := filepath.Join(repo, ".backlogit")
		require.NoError(t, os.MkdirAll(filepath.Join(storage, "queue"), 0o755))
		require.NoError(t, config.WriteDefaults(storage))
		require.NoError(t, os.WriteFile(filepath.Join(storage, "queue", "001-F.md"), []byte("---\nid: 001-F\n---\n"), 0o644))
		runGit("add", ".")
		runGit("commit", "-m", "main baseline")

		runGit("checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(filepath.Join(storage, "queue", "007-F.md"), []byte("---\nid: 007-F\n---\n"), 0o644))
		runGit("add", ".")
		runGit("commit", "-m", "branch artifact")
		runGit("checkout", "main")

		ids, err := scanLocalBranchArtifactIDs(&Workspace{RootPath: repo})
		require.NoError(t, err)
		assert.Contains(t, ids, "007-F", "branch scan must detect 007-F on feature-branch")

		// Slugged hierarchical IDs from branch scans must contribute to maxTypedOrdinalAmong.
		assert.Equal(t, 4, maxTypedOrdinalAmong([]string{"001.004-T-implement-slug"}, "001", "", "-T"))

		// Single-branch deleted artifact committed on HEAD must still be detected by scanLocalBranchArtifactIDs.
		singleRepo := t.TempDir()
		runSingleGit := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-C", singleRepo}, args...)...)
			cmd.Env = sanitizedGitBranchScanEnv()
			out, gErr := cmd.CombinedOutput()
			require.NoError(t, gErr, "git %v failed: %s", args, out)
		}
		runSingleGit("init", "-b", "main")
		runSingleGit("config", "user.name", "Test")
		runSingleGit("config", "user.email", "test@example.com")
		singleStorage := filepath.Join(singleRepo, ".backlogit")
		require.NoError(t, os.MkdirAll(filepath.Join(singleStorage, "queue"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(singleStorage, "queue", "009-F.md"), []byte("---\nid: 009-F\n---\n"), 0o644))
		runSingleGit("add", ".")
		runSingleGit("commit", "-m", "commit 009-F")
		require.NoError(t, os.Remove(filepath.Join(singleStorage, "queue", "009-F.md")))
		singleIDs, err := scanLocalBranchArtifactIDs(&Workspace{RootPath: singleRepo})
		require.NoError(t, err)
		assert.Contains(t, singleIDs, "009-F", "single-branch HEAD scan must detect deleted committed artifact 009-F")

		// Symlinked .git must fail closed when symlinks are supported.
		outsideGit := filepath.Join(t.TempDir(), "fake.git")
		require.NoError(t, os.MkdirAll(outsideGit, 0o755))
		symlinkedRepo := t.TempDir()
		gitLink := filepath.Join(symlinkedRepo, ".git")
		if runtime.GOOS == "windows" {
			out, linkErr := exec.Command("cmd", "/c", "mklink", "/J", gitLink, outsideGit).CombinedOutput()
			if linkErr != nil {
				t.Skipf("junction failed: %v (%s)", linkErr, out)
			}
		} else {
			require.NoError(t, os.Symlink(outsideGit, gitLink))
		}
		_, err = scanLocalBranchArtifactIDs(&Workspace{RootPath: symlinkedRepo})
		require.Error(t, err, "symlinked .git must fail closed")
	})

	t.Run("adopt-failure-after-allocator-release-preserves-concurrent-reservations", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		f1, err := CreateArtifact(ctx, ws, "Target Feature", "feature")
		require.NoError(t, err)
		f2, err := CreateArtifact(ctx, ws, "Source Feature", "feature")
		require.NoError(t, err)
		orphan, err := CreateArtifact(ctx, ws, "Orphan Task", "task", WithParent(f2.ID))
		require.NoError(t, err)
		_, err = CreateArtifact(ctx, ws, "Blocker Ref", "task", WithParent(f2.ID), WithDependencies([]string{orphan.ID}))
		require.NoError(t, err)

		// Concurrently create a feature while AdoptItem fails after releasing id_allocator.lock
		// (allowedCrossRefIDs rejects the un-snapshotted cross-reference owner after allocation).
		var wg sync.WaitGroup
		wg.Add(2)
		var adoptErr, createErr error
		var concFeatID string
		go func() {
			defer wg.Done()
			_, adoptErr = AdoptItemWithOptions(ctx, ws, orphan.ID, f1.ID, AdoptItemOptions{
				allowedCrossRefIDs: map[string]struct{}{},
			})
		}()
		go func() {
			defer wg.Done()
			concFeat, cErr := CreateArtifact(ctx, ws, "Concurrent Feature", "feature")
			createErr = cErr
			if concFeat != nil {
				concFeatID = concFeat.ID
			}
		}()
		wg.Wait()

		require.ErrorIs(t, adoptErr, blerrors.ErrShipmentConflict)
		require.NoError(t, createErr)
		require.NotEmpty(t, concFeatID)

		reserved, err := readReservedSequenceIDs(ws)
		require.NoError(t, err)
		assert.Contains(t, reserved, concFeatID, "concurrent CreateArtifact reservation must survive AdoptItem rollback")
		assert.NotContains(t, reserved, "001.001-T", "rolled-back AdoptItem ID must be unreserved")
	})
}

// TestU4_ScanLocalBranchArtifactIDsBatchesTreesAndPreservesMergedRevertedReservations gates 217.004-T (U4).
func TestU4_ScanLocalBranchArtifactIDsBatchesTreesAndPreservesMergedRevertedReservations(t *testing.T) {
	t.Run("hundreds-of-branches-within-budget", func(t *testing.T) {
		repo := t.TempDir()
		runGit := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
			cmd.Env = sanitizedGitBranchScanEnv()
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "git %v failed: %s", args, out)
		}
		runGit("init", "-b", "main")
		runGit("config", "user.name", "Test")
		runGit("config", "user.email", "test@example.com")

		storage := filepath.Join(repo, ".backlogit")
		require.NoError(t, os.MkdirAll(filepath.Join(storage, "queue"), 0o755))
		require.NoError(t, config.WriteDefaults(storage))
		require.NoError(t, os.WriteFile(filepath.Join(storage, "queue", "001-F.md"), []byte("---\nid: 001-F\ntitle: Seed\ntype: feature\nstatus: backlog\n---\n"), 0o644))
		runGit("add", ".")
		runGit("commit", "-m", "main baseline")

		// Use git fast-import to rapidly build 120 distinct commits (and distinct tree OIDs)
		// along main, with 120 merged refs/heads/hist-branch-NNN branches pointing at each commit.
		const mergedCount = 120
		var stream strings.Builder
		for i := 1; i <= mergedCount; i++ {
			mark := i
			content := fmt.Sprintf("history branch %03d\n", i)
			fmt.Fprintf(&stream, "commit refs/heads/main\n")
			fmt.Fprintf(&stream, "mark :%d\n", mark)
			fmt.Fprintf(&stream, "author Test <test@example.com> %d +0000\n", 1700000000+i)
			fmt.Fprintf(&stream, "committer Test <test@example.com> %d +0000\n", 1700000000+i)
			msg := fmt.Sprintf("commit %03d", i)
			fmt.Fprintf(&stream, "data %d\n%s\n", len(msg), msg)
			if i == 1 {
				fmt.Fprintf(&stream, "from refs/heads/main^0\n")
			} else {
				fmt.Fprintf(&stream, "from :%d\n", mark-1)
			}
			fmt.Fprintf(&stream, "M 100644 inline history/file-%03d.txt\n", i)
			fmt.Fprintf(&stream, "data %d\n%s\n", len(content), content)
			fmt.Fprintf(&stream, "reset refs/heads/hist-branch-%03d\nfrom :%d\n\n", i, mark)
		}
		importCmd := exec.Command("git", "-C", repo, "fast-import", "--quiet")
		importCmd.Env = sanitizedGitBranchScanEnv()
		importCmd.Stdin = strings.NewReader(stream.String())
		importOut, err := importCmd.CombinedOutput()
		require.NoError(t, err, "git fast-import failed: %s", importOut)
		runGit("reset", "--hard", "refs/heads/main")

		// Create 2 unmerged branches off different commits containing branch-only .backlogit/queue/098-F.md.
		runGit("checkout", "-b", "unmerged-alpha", "refs/heads/hist-branch-010")
		require.NoError(t, os.WriteFile(filepath.Join(storage, "queue", "098-F.md"), []byte("---\nid: 098-F\ntitle: Branch Only\ntype: feature\nstatus: backlog\n---\n"), 0o644))
		runGit("add", ".")
		runGit("commit", "-m", "unmerged alpha 098-F")

		runGit("checkout", "-b", "unmerged-beta", "refs/heads/hist-branch-050")
		require.NoError(t, os.WriteFile(filepath.Join(storage, "queue", "098-F.md"), []byte("---\nid: 098-F\ntitle: Branch Only Beta\ntype: feature\nstatus: backlog\n---\n"), 0o644))
		runGit("add", ".")
		runGit("commit", "-m", "unmerged beta 098-F")

		runGit("checkout", "main")

		ctx := context.Background()
		ws, err := NewWorkspace(ctx, repo)
		require.NoError(t, err)
		ws.GateBroker = nil
		t.Cleanup(func() { require.NoError(t, ws.Close()) })

		start := time.Now()
		ids, err := scanLocalBranchArtifactIDsContext(ctx, ws)
		require.NoError(t, err)
		assert.Contains(t, ids, "098-F", "branch scan must detect branch-only 098-F")

		created, err := CreateArtifact(ctx, ws, "Next Feature", "feature")
		require.NoError(t, err)
		assert.Equal(t, "099-F", created.ID)
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 2*time.Second, "scanning 120+ branches and allocating ID must complete within 2s (took %v)", elapsed)
	})

	t.Run("merged-then-reverted-branch-preserves-reservation", func(t *testing.T) {
		repo := t.TempDir()
		runGit := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
			cmd.Env = sanitizedGitBranchScanEnv()
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "git %v failed: %s", args, out)
		}
		runGit("init", "-b", "main")
		runGit("config", "user.name", "Test")
		runGit("config", "user.email", "test@example.com")

		storage := filepath.Join(repo, ".backlogit")
		require.NoError(t, os.MkdirAll(filepath.Join(storage, "queue"), 0o755))
		require.NoError(t, config.WriteDefaults(storage))
		require.NoError(t, os.WriteFile(filepath.Join(storage, "queue", "001-F.md"), []byte("---\nid: 001-F\ntitle: Seed\ntype: feature\nstatus: backlog\n---\n"), 0o644))
		runGit("add", ".")
		runGit("commit", "-m", "main baseline")

		runGit("checkout", "-b", "feat-099")
		require.NoError(t, os.WriteFile(filepath.Join(storage, "queue", "099-F.md"), []byte("---\nid: 099-F\n---\n"), 0o644))
		runGit("add", ".")
		runGit("commit", "-m", "add 099-F on feat-099")

		runGit("checkout", "main")
		runGit("merge", "--no-ff", "-m", "merge feat-099", "feat-099")

		// Revert/delete 099-F.md on main while keeping refs/heads/feat-099 at its branch tip.
		require.NoError(t, os.Remove(filepath.Join(storage, "queue", "099-F.md")))
		runGit("add", "-A")
		runGit("commit", "-m", "revert/delete 099-F on main")

		seqPath := filepath.Join(WorkspaceLocksRoot(repo), idSequenceLedgerFileName)
		_, statErr := os.Stat(seqPath)
		require.True(t, os.IsNotExist(statErr), "id-sequences.json must not exist for this scenario")

		ctx := context.Background()
		ws, err := NewWorkspace(ctx, repo)
		require.NoError(t, err)
		ws.GateBroker = nil
		t.Cleanup(func() { require.NoError(t, ws.Close()) })

		ids, err := scanLocalBranchArtifactIDsContext(ctx, ws)
		require.NoError(t, err)
		assert.Contains(t, ids, "099-F", "merged-then-reverted branch feat-099 must preserve 099-F reservation")

		created, err := CreateArtifact(ctx, ws, "After Revert", "feature")
		require.NoError(t, err)
		assert.Equal(t, "100-F", created.ID, "must allocate 100-F and never reuse reverted 099-F")
	})
}

func TestU4_GitCatFileAndTreeEntrySafety(t *testing.T) {
	t.Run("readGitCatFileBatch-rejects-oversized-and-negative", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			header string
		}{
			{name: "negative-size", header: "0123456789abcdef0123456789abcdef01234567 tree -1\n"},
			{name: "oversized-payload", header: fmt.Sprintf("0123456789abcdef0123456789abcdef01234567 tree %d\n", (64<<20)+1)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				seenTrees := map[string]struct{}{}
				parsedTrees := map[string]struct{}{}
				seenIDs := map[string]struct{}{}
				var collected []string
				_, err := readGitCatFileBatch(bufio.NewReader(strings.NewReader(tc.header)), 1, seenTrees, parsedTrees, seenIDs, &collected)
				require.ErrorIs(t, err, blerrors.ErrValidation)
			})
		}
	})

	t.Run("parseGitTreePayload-collects-regular-md-and-ignores-symlinks-gitlinks", func(t *testing.T) {
		buildEntry := func(mode, name, oidHex string) []byte {
			rawOID, err := hex.DecodeString(oidHex)
			require.NoError(t, err)
			var buf bytes.Buffer
			buf.WriteString(mode)
			buf.WriteByte(' ')
			buf.WriteString(name)
			buf.WriteByte(0)
			buf.Write(rawOID)
			return buf.Bytes()
		}

		const treeOID = "0123456789abcdef0123456789abcdef01234567"
		var payload bytes.Buffer
		payload.Write(buildEntry("100644", "001-F.md", "1111111111111111111111111111111111111111"))
		payload.Write(buildEntry("100755", "002-F.md", "2222222222222222222222222222222222222222"))
		payload.Write(buildEntry("120000", "003-F.md", "3333333333333333333333333333333333333333"))
		payload.Write(buildEntry("160000", "004-F.md", "4444444444444444444444444444444444444444"))
		payload.Write(buildEntry("100644", "notes.txt", "5555555555555555555555555555555555555555"))
		payload.Write(buildEntry("40000", "queue", "6666666666666666666666666666666666666666"))

		seenTrees := map[string]struct{}{}
		var nextSpecs []string
		seenIDs := map[string]struct{}{}
		var collected []string
		err := parseGitTreePayload(treeOID, payload.Bytes(), seenTrees, &nextSpecs, seenIDs, &collected)
		require.NoError(t, err)
		assert.Equal(t, []string{"001-F", "002-F"}, collected)
		assert.Equal(t, []string{"6666666666666666666666666666666666666666"}, nextSpecs)
	})
}
