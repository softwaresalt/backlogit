package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

type reconstituteFixture struct {
	ws         *Workspace
	feature    *models.Artifact
	tasks      []*models.Artifact
	source     *models.Artifact
	downstream *models.Artifact
}

func newReconstituteFixture(t *testing.T) *reconstituteFixture {
	t.Helper()
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	fx := &reconstituteFixture{ws: ws}

	feature, err := CreateArtifact(ctx, ws, "Reconstitute covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))
	fx.feature = feature

	ids := make([]string, 0, 6)
	for i := 1; i <= 6; i++ {
		task, taskErr := CreateArtifact(ctx, ws, fmt.Sprintf("Reconstitute task %d", i), "task", WithParent(feature.ID))
		require.NoError(t, taskErr)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))
		fx.tasks = append(fx.tasks, task)
		ids = append(ids, task.ID)
	}

	source, err := CreateShipment(ctx, ws, "Reconstitute source shipment", ids)
	require.NoError(t, err)
	fx.source = source

	otherFeature, err := CreateArtifact(ctx, ws, "Downstream feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, otherFeature))
	downTask, err := CreateArtifact(ctx, ws, "Downstream task", "task", WithParent(otherFeature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, downTask))
	downstream, err := CreateShipment(ctx, ws, "Reconstitute downstream shipment", []string{downTask.ID})
	require.NoError(t, err)
	require.NoError(t, AddDependency(ctx, ws, downstream.ID, source.ID, "blocks"))
	fx.downstream = downstream
	return fx
}

func (fx *reconstituteFixture) splitPlanYAML() string {
	t := fx.tasks
	return fmt.Sprintf(`source_shipments: [%s]
shipments:
  - label: ship-1
    title: Ship one
    feature:
      create:
        title: Split feature one
        related_to: %s
    items: [%s, %s]
  - label: ship-2
    title: Ship two
    feature:
      create:
        title: Split feature two
        related_to: %s
    items: [%s, %s]
    blocks_on: [ship-1]
  - id: %s
    feature: {id: %s}
    items: [%s, %s]
    blocks_on: [ship-2]
`, fx.source.ID, fx.feature.ID, t[0].ID, t[1].ID, fx.feature.ID, t[2].ID, t[3].ID,
		fx.source.ID, fx.feature.ID, t[4].ID, t[5].ID)
}

// workspaceFileSnapshot captures every Markdown and JSONL file under the storage root.
func workspaceFileSnapshot(t *testing.T, ws *Workspace) map[string]string {
	t.Helper()
	root := workspaceStorageRoot(ws)
	out := map[string]string{}
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".jsonl" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	}))
	return out
}

func vetoHook(point hooks.HookPoint) hooks.HookRegistration {
	return hooks.HookRegistration{
		Name: "reconstitute-veto-" + string(point),
		Fn: func(context.Context, hooks.HookContext) error {
			return errors.New("injected veto")
		},
	}
}

func TestU2_ReconstituteShipmentsAtomicExecutionAndRollback(t *testing.T) {
	ctx := context.Background()

	t.Run("yaml_and_path_safety", func(t *testing.T) {
		fx := newReconstituteFixture(t)
		ws := fx.ws
		root, err := filepath.EvalSymlinks(ws.RootPath)
		require.NoError(t, err)

		require.NoError(t, os.MkdirAll(filepath.Join(root, "plans"), 0o755))
		valid := filepath.Join(root, "plans", "reshuffle.yaml")
		require.NoError(t, os.WriteFile(valid, []byte(fx.splitPlanYAML()), 0o644))
		res, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanPath: valid, DryRun: true})
		require.NoError(t, err, "a plan file inside the workspace root outside .backlogit must be accepted")
		require.NotNil(t, res)

		outside := filepath.Join(t.TempDir(), "outside.yaml")
		require.NoError(t, os.WriteFile(outside, []byte(fx.splitPlanYAML()), 0o644))
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanPath: outside, DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation)

		leaf := filepath.Join(root, "plans", "leaf.yaml")
		if os.Symlink(outside, leaf) == nil {
			_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanPath: leaf, DryRun: true})
			assert.ErrorIs(t, err, blerrors.ErrValidation, "leaf symlink must be rejected")
		}
		linkDir := filepath.Join(root, "linked")
		if os.Symlink(filepath.Dir(outside), linkDir) == nil {
			_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{
				PlanPath: filepath.Join(linkDir, "outside.yaml"), DryRun: true})
			assert.ErrorIs(t, err, blerrors.ErrValidation, "intermediate symlink component must be rejected")
		}

		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{
			PlanYAML: fx.splitPlanYAML() + "bogus_field: 1\n", DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "unknown YAML fields must be rejected")
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{
			PlanYAML: "source_shipments: [" + fx.source.ID + "]\nsource_shipments: [" + fx.source.ID + "]\nshipments: []\n",
			DryRun:   true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "duplicate mapping keys must be rejected")
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: "source_shipments: []\nshipments: []\n", DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "empty source_shipments and shipments must be rejected")
	})

	t.Run("source_shipment_status_guards", func(t *testing.T) {
		fx := newReconstituteFixture(t)
		ws := fx.ws
		plan := fx.splitPlanYAML()

		_, err := ClaimShipment(ctx, ws, fx.source.ID)
		require.NoError(t, err)
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: plan, DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrShipmentConflict, "active source requires Force")
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: plan, DryRun: true, Force: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "Force requires a Reason")
		res, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{
			PlanYAML: plan, DryRun: true, Force: true, Reason: "split active shipment"})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.True(t, res.Force)

		_, err = BlockShipment(ctx, ws, fx.source.ID, BlockOptions{Reason: "blocked for test", BlockedBy: "tester"})
		if err == nil {
			_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{
				PlanYAML: plan, DryRun: true, Force: true, Reason: "force on blocked"})
			assert.ErrorIs(t, err, blerrors.ErrShipmentConflict, "blocked source is refused even with Force")
		}
	})

	t.Run("partition_and_membership_validation", func(t *testing.T) {
		fx := newReconstituteFixture(t)
		ws := fx.ws
		tk := fx.tasks
		omit := fmt.Sprintf(`source_shipments: [%s]
shipments:
  - id: %s
    feature: {id: %s}
    items: [%s, %s, %s, %s, %s]
`, fx.source.ID, fx.source.ID, fx.feature.ID, tk[0].ID, tk[1].ID, tk[2].ID, tk[3].ID, tk[4].ID)
		_, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: omit, DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "omitted source member must be rejected")

		withUnassigned := omit + fmt.Sprintf("unassigned_items: [%s]\n", tk[5].ID)
		res, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: withUnassigned, DryRun: true})
		require.NoError(t, err, "omitted member listed in unassigned_items is accepted")
		require.NotNil(t, res)

		dup := fmt.Sprintf(`source_shipments: [%s]
shipments:
  - label: a
    feature: {create: {title: A}}
    items: [%s, %s, %s]
  - id: %s
    feature: {id: %s}
    items: [%s, %s, %s]
    blocks_on: [a]
`, fx.source.ID, tk[0].ID, tk[1].ID, tk[2].ID, fx.source.ID, fx.feature.ID, tk[2].ID, tk[3].ID, tk[4].ID)
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: dup + "unassigned_items: [" + tk[5].ID + "]\n", DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "duplicate item assignment must be rejected")

		split := fmt.Sprintf(`source_shipments: [%s]
shipments:
  - label: a
    feature: {id: %s}
    items: [%s, %s, %s]
  - id: %s
    feature: {id: %s}
    items: [%s, %s, %s]
    blocks_on: [a]
`, fx.source.ID, fx.feature.ID, tk[0].ID, tk[1].ID, tk[2].ID, fx.source.ID, fx.feature.ID, tk[3].ID, tk[4].ID, tk[5].ID)
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: split, DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "same covering feature split across shipments must be rejected")
	})

	t.Run("dag_and_forward_edge_validation", func(t *testing.T) {
		fx := newReconstituteFixture(t)
		ws := fx.ws
		tk := fx.tasks
		cycle := fmt.Sprintf(`source_shipments: [%s]
shipments:
  - label: a
    feature: {create: {title: A}}
    items: [%s, %s, %s]
    blocks_on: [b]
  - label: b
    feature: {create: {title: B}}
    items: [%s, %s, %s]
    blocks_on: [a]
`, fx.source.ID, tk[0].ID, tk[1].ID, tk[2].ID, tk[3].ID, tk[4].ID, tk[5].ID)
		_, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: cycle, DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "dependency cycle must be rejected")

		unsequenced := fmt.Sprintf(`source_shipments: [%s]
shipments:
  - label: a
    feature: {create: {title: A}}
    items: [%s, %s, %s]
  - label: b
    feature: {create: {title: B}}
    items: [%s, %s, %s]
`, fx.source.ID, tk[0].ID, tk[1].ID, tk[2].ID, tk[3].ID, tk[4].ID, tk[5].ID)
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: unsequenced, DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "unsequenced queued shipments must be rejected")

		require.NoError(t, AddDependency(ctx, ws, tk[0].ID, tk[3].ID, "blocks"))
		forward := fmt.Sprintf(`source_shipments: [%s]
shipments:
  - label: a
    feature: {create: {title: A}}
    items: [%s, %s, %s]
  - label: b
    feature: {create: {title: B}}
    items: [%s, %s, %s]
    blocks_on: [a]
`, fx.source.ID, tk[0].ID, tk[1].ID, tk[2].ID, tk[3].ID, tk[4].ID, tk[5].ID)
		_, err = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: forward, DryRun: true})
		assert.ErrorIs(t, err, blerrors.ErrValidation, "forward shipment dependency must be rejected")
	})

	t.Run("dry_run_zero_write_and_diff_projection", func(t *testing.T) {
		fx := newReconstituteFixture(t)
		ws := fx.ws
		before := workspaceFileSnapshot(t, ws)
		res, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML(), DryRun: true})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "dry_run", res.Status)
		assert.True(t, res.DryRun)
		assert.Regexp(t, `^[0-9a-f]{64}$`, res.PlanDigest)
		assert.Len(t, res.CreatedFeatures, 2)
		assert.Len(t, res.CreatedShipments, 2)
		assert.Len(t, res.AdoptedIDMap, 4)
		assert.NotEmpty(t, res.BeforeManifests)
		assert.NotEmpty(t, res.AfterManifests)
		assert.NotEmpty(t, res.DependencyChanges)
		assert.Equal(t, before, workspaceFileSnapshot(t, ws), "dry run must write nothing")
	})

	t.Run("atomic_reconstitution_and_audit_events", func(t *testing.T) {
		fx := newReconstituteFixture(t)
		ws := fx.ws
		res, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.False(t, res.DryRun)
		assert.NotEmpty(t, res.CorrelationID)
		assert.Len(t, res.CreatedFeatures, 2)
		assert.Len(t, res.CreatedShipments, 2)
		assert.Len(t, res.AdoptedIDMap, 4)

		for _, shipmentID := range res.TargetShipments {
			shipment, loadErr := loadArtifact(ctx, ws, shipmentID)
			require.NoError(t, loadErr)
			items := NormalizeShipmentItems(shipment)
			require.NotEmpty(t, items)
			first, loadErr := loadArtifact(ctx, ws, items[0])
			require.NoError(t, loadErr)
			assert.Equal(t, "feature", first.ArtifactType, "covering feature must be first in the manifest")
			logEvents, readErr := events.ReadAllEvents(ctx, WorkspaceLogsRoot(ws.RootPath), shipmentID)
			require.NoError(t, readErr)
			found := false
			for _, event := range logEvents {
				if event.EventType == EventShipmentReconstituted {
					found = true
				}
			}
			assert.True(t, found, "shipment %s must carry a shipment_reconstituted event", shipmentID)
		}

		report, err := Doctor(ctx, ws, &DoctorOptions{CheckOrphans: true, CheckDuplicates: true})
		require.NoError(t, err)
		assert.Empty(t, report.Findings, "doctor must be clean after reconstitution")

		keep := newReconstituteFixture(t)
		keepPlan := fmt.Sprintf(`source_shipments: [%s]
shipments:
  - label: a
    feature: {create: {title: Keep A}}
    keep_id: true
    items: [%s, %s, %s]
  - id: %s
    feature: {id: %s}
    items: [%s, %s, %s]
    blocks_on: [a]
`, keep.source.ID, keep.tasks[0].ID, keep.tasks[1].ID, keep.tasks[2].ID,
			keep.source.ID, keep.feature.ID, keep.tasks[3].ID, keep.tasks[4].ID, keep.tasks[5].ID)
		kres, err := ReconstituteShipments(ctx, keep.ws, ReconstituteOptions{PlanYAML: keepPlan})
		require.NoError(t, err)
		assert.Empty(t, kres.AdoptedIDMap, "keep_id re-parenting must not rename items")

		_, err = ClaimShipment(ctx, keep.ws, keep.downstream.ID)
		_ = err
	})

	t.Run("midway_failure_restores_byte_identical_workspace", func(t *testing.T) {
		for _, point := range []hooks.HookPoint{hooks.HookCreateArtifact, hooks.HookAdoptItem, hooks.HookUpdateArtifact} {
			t.Run(string(point), func(t *testing.T) {
				fx := newReconstituteFixture(t)
				ws := fx.ws
				ws.HookRunner = hooks.NewHookRunner()
				require.NoError(t, AssociateCommit(ctx, ws, events.NewEventWriter(WorkspaceLogsRoot(ws.RootPath)),
					fx.tasks[0].ID, "abc1234", "seed", "tester"))
				before := workspaceFileSnapshot(t, ws)
				ws.HookRunner.Register(point, hooks.PhasePre, vetoHook(point))

				_, err := ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
				require.Error(t, err)
				assert.NotErrorIs(t, err, blerrors.ErrNotImplemented, "failure must come from the injected veto")
				assert.Contains(t, err.Error(), "injected veto")
				assert.Equal(t, before, workspaceFileSnapshot(t, ws), "workspace must be byte-identical after rollback")

				var itemID string
				row := ws.DB.QueryRowContext(ctx, `SELECT item_id FROM commit_links WHERE commit_sha = ?`, "abc1234")
				require.NoError(t, row.Scan(&itemID))
				assert.Equal(t, fx.tasks[0].ID, itemID)
			})
		}
	})

	t.Run("durable_journal_crash_recovery_and_failed_rollback_retention", func(t *testing.T) {
		newJournal := func(t *testing.T, fx *reconstituteFixture, phase, corr string) (string, string) {
			t.Helper()
			pre := cloneArtifact(fx.source)
			journal := shipmentLifecycleJournal{
				SchemaVersion:  "shipment-operation/v1",
				CorrelationID:  corr,
				Phase:          phase,
				Operation:      "reconstitute",
				RecoveryPolicy: "rollback",
				ShipmentID:     fx.source.ID,
				Target:         "queued",
				Preimage:       shipmentLifecyclePreimage{Shipment: pre},
			}
			path, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr), journal)
			require.NoError(t, err)
			shipmentPath, err := FindArtifactPath(ctx, fx.ws, fx.source.ID)
			require.NoError(t, err)
			return path, shipmentPath
		}

		t.Run("intent_rolls_back", func(t *testing.T) {
			fx := newReconstituteFixture(t)
			path, shipmentPath := newJournal(t, fx, "intent", "reconstitute-crash-intent")
			original, err := os.ReadFile(shipmentPath)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(shipmentPath,
				[]byte(strings.Replace(string(original), "Reconstitute source shipment", "Mutated mid-crash", 1)), 0o644))

			reopened, err := NewWorkspace(ctx, fx.ws.RootPath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = reopened.Close() })
			after, err := os.ReadFile(shipmentPath)
			require.NoError(t, err)
			assert.Equal(t, string(original), string(after), "intent journal must roll back to the preimage")
			_, statErr := os.Stat(path)
			assert.True(t, os.IsNotExist(statErr), "journal must be cleaned up after rollback")
		})

		t.Run("committed_finalizes_without_rollback", func(t *testing.T) {
			fx := newReconstituteFixture(t)
			path, shipmentPath := newJournal(t, fx, "committed", "reconstitute-crash-committed")
			original, err := os.ReadFile(shipmentPath)
			require.NoError(t, err)
			mutated := strings.Replace(string(original), "Reconstitute source shipment", "Committed title", 1)
			require.NoError(t, os.WriteFile(shipmentPath, []byte(mutated), 0o644))

			reopened, err := NewWorkspace(ctx, fx.ws.RootPath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = reopened.Close() })
			after, err := os.ReadFile(shipmentPath)
			require.NoError(t, err)
			assert.Equal(t, mutated, string(after), "committed journal must not roll back")
			_, statErr := os.Stat(path)
			assert.True(t, os.IsNotExist(statErr), "committed journal must be finalized and removed")
		})

		t.Run("failed_rollback_retains_journal", func(t *testing.T) {
			fx := newReconstituteFixture(t)
			path, shipmentPath := newJournal(t, fx, "intent", "reconstitute-crash-retained")
			require.NoError(t, os.Remove(shipmentPath))
			require.NoError(t, os.MkdirAll(shipmentPath, 0o755))
			t.Cleanup(func() { _ = os.RemoveAll(shipmentPath) })

			reopened, err := NewWorkspace(ctx, fx.ws.RootPath)
			if err == nil {
				_ = reopened.Close()
			}
			require.Error(t, err, "rollback failure must surface as a recovery error")
			assert.NotErrorIs(t, err, blerrors.ErrValidation, "journal must be a recognised reconstitute operation")
			_, statErr := os.Stat(path)
			assert.NoError(t, statErr, "failed rollback must retain the intent journal")
		})
	})

	t.Run("concurrent_associate_commit_and_post_hook_isolation", func(t *testing.T) {
		fx := newReconstituteFixture(t)
		ws := fx.ws
		ws.HookRunner = hooks.NewHookRunner()
		ew := events.NewEventWriter(WorkspaceLogsRoot(ws.RootPath))

		var postMu sync.Mutex
		posts := map[string]int{}
		ws.HookRunner.Register(hooks.HookCreateArtifact, hooks.PhasePost, hooks.HookRegistration{
			Name: "count-create",
			Fn: func(_ context.Context, hc hooks.HookContext) error {
				postMu.Lock()
				posts[hc.ItemID]++
				postMu.Unlock()
				return nil
			},
		})

		var wg sync.WaitGroup
		var commitErr, reconErr error
		var res *ReconstituteShipmentResult
		wg.Add(2)
		go func() {
			defer wg.Done()
			commitErr = AssociateCommit(ctx, ws, ew, fx.tasks[5].ID, "feedface", "concurrent", "tester")
		}()
		go func() {
			defer wg.Done()
			res, reconErr = ReconstituteShipments(ctx, ws, ReconstituteOptions{PlanYAML: fx.splitPlanYAML()})
		}()
		wg.Wait()
		require.NoError(t, commitErr)
		require.NoError(t, reconErr)
		require.NotNil(t, res)

		logEvents, err := events.ReadAllEvents(ctx, WorkspaceLogsRoot(ws.RootPath), fx.tasks[5].ID)
		require.NoError(t, err)
		tracked := false
		for _, event := range logEvents {
			if strings.Contains(event.EventType, "commit") {
				tracked = true
			}
		}
		assert.True(t, tracked, "concurrent commit event must not be lost")

		postMu.Lock()
		afterRecon := len(posts)
		postMu.Unlock()
		assert.Equal(t, len(res.CreatedFeatures)+len(res.CreatedShipments), afterRecon,
			"buffered post-hooks flush after commit, once per created artifact")

		independent, err := CreateArtifact(ctx, ws, "Independent artifact", "task")
		require.NoError(t, err)
		postMu.Lock()
		assert.Equal(t, 1, posts[independent.ID], "independent CreateArtifact fires post-hooks immediately")
		postMu.Unlock()

		ctxBuf := hooks.WithBufferedPostHooks(ctx)
		ws.HookRunner.FirePost(ctxBuf, hooks.HookCreateArtifact, hooks.HookContext{ItemID: "buffered-probe"})
		postMu.Lock()
		assert.Zero(t, posts["buffered-probe"], "buffered ctx must hold post-hooks until flush")
		postMu.Unlock()
		hooks.FlushBufferedPostHooks(ctxBuf, ws.HookRunner)
		postMu.Lock()
		assert.Equal(t, 1, posts["buffered-probe"], "flush emits buffered post-hooks")
		postMu.Unlock()

		fx2 := newReconstituteFixture(t)
		fx2.ws.HookRunner = hooks.NewHookRunner()
		fx2.ws.HookRunner.Register(hooks.HookCreateArtifact, hooks.PhasePre, vetoHook(hooks.HookCreateArtifact))
		emitted := 0
		fx2.ws.HookRunner.Register(hooks.HookCreateArtifact, hooks.PhasePost, hooks.HookRegistration{
			Name: "count",
			Fn:   func(context.Context, hooks.HookContext) error { emitted++; return nil },
		})
		_, err = ReconstituteShipments(ctx, fx2.ws, ReconstituteOptions{PlanYAML: fx2.splitPlanYAML()})
		require.Error(t, err)
		assert.NotErrorIs(t, err, blerrors.ErrNotImplemented)
		assert.Zero(t, emitted, "vetoed reconstitution must emit zero post-hooks")
	})
}
