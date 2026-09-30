package core

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU3_RollbackDeliverableHasBothScenarios(t *testing.T) {
	source, err := os.ReadFile("shipment_claim_marker_rollback_test.go")
	if err != nil {
		t.Fatalf("read rollback test deliverable: %v", err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "shipment_claim_marker_rollback_test.go", source, 0)
	if err != nil {
		t.Fatalf("parse rollback test deliverable: %v", err)
	}

	type scenarioShape struct {
		name        string
		identifiers []string
		selectors   []string
	}

	scenarios := []scenarioShape{
		{
			name: "TestU3_InProcessSnapshotRollbackRestoresPreimage",
			identifiers: []string{
				"ClaimShipment",
				"CreateShipment",
				"persistArtifactWriteFn",
				"schedulerBaselineClaimKey",
				"requireP1C6DurableStateRestored",
			},
			selectors: []string{"Cleanup", "StatusActive", "StatusQueued"},
		},
		{
			name: "TestU3_JournalRecoveryRollbackRestoresPreimage",
			identifiers: []string{
				"ClaimShipment",
				"CreateShipment",
				"persistArtifactWriteFn",
				"restoreShipmentSnapshotFn",
				"NewWorkspace",
				"bldb",
				"schedulerBaselineClaimKey",
				"requireP1C6DurableStateRestored",
			},
			selectors: []string{"Cleanup", "GetItem", "Path", "StatusActive", "StatusQueued"},
		},
	}

	functions := make(map[string]*ast.FuncDecl, len(scenarios))
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok {
			functions[function.Name.Name] = function
		}
	}

	for _, scenario := range scenarios {
		function, ok := functions[scenario.name]
		if !ok || function.Body == nil {
			t.Errorf("rollback scenario %s is not declared in shipment_claim_marker_rollback_test.go", scenario.name)
			continue
		}

		identifiers := make(map[string]bool)
		selectors := make(map[string]bool)
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch expression := node.(type) {
			case *ast.Ident:
				identifiers[expression.Name] = true
			case *ast.SelectorExpr:
				selectors[expression.Sel.Name] = true
			}
			return true
		})

		for _, name := range scenario.identifiers {
			if !identifiers[name] {
				t.Errorf("%s must exercise %s", scenario.name, name)
			}
		}
		for _, name := range scenario.selectors {
			if !selectors[name] {
				t.Errorf("%s must exercise selector %s", scenario.name, name)
			}
		}
	}
}

func TestU3_InProcessSnapshotRollbackRestoresPreimage(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)

	parent, err := CreateArtifact(ctx, ws, "U3 in-process feature", "feature")
	require.NoError(t, err)
	member1, err := CreateArtifact(ctx, ws, "U3 in-process member 1", "task", WithParent(parent.ID))
	require.NoError(t, err)
	member1.CustomFields = nil
	require.NoError(t, persistArtifact(ctx, ws, member1, false))

	member2, err := CreateArtifact(ctx, ws, "U3 in-process member 2", "task", WithParent(parent.ID))
	require.NoError(t, err)
	member2.CustomFields = map[string]any{"retained": "member2-preimage"}
	require.NoError(t, persistArtifact(ctx, ws, member2, false))

	shipment, err := CreateShipment(ctx, ws, "U3 in-process shipment", []string{member1.ID, member2.ID})
	require.NoError(t, err)
	require.Equal(t, []string{member1.ID, member2.ID}, NormalizeShipmentItems(shipment),
		"the manifest must order member1 before member2")
	require.Equal(t, models.StatusQueued, member1.Status)
	require.Equal(t, models.StatusQueued, member2.Status)

	before := snapshotURGovernedState(t, ws, []string{shipment.ID, member1.ID, member2.ID})
	injectedForwardErr := errors.New("injected member2 activation failure")
	originalWriter := persistArtifactWriteFn
	member1MarkedWrites := 0
	member1MarkedBeforeFailure := 0
	failureInjected := false
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == member1.ID &&
			artifact.Status == models.StatusActive &&
			artifact.CustomFields[schedulerBaselineClaimKey] == shipment.ID {
			if err := originalWriter(artifact, filePath, durable); err != nil {
				return err
			}
			member1MarkedWrites++
			return nil
		}
		if artifact.ID == member2.ID && artifact.Status == models.StatusActive {
			member1MarkedBeforeFailure = member1MarkedWrites
			failureInjected = true
			return injectedForwardErr
		}
		return originalWriter(artifact, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	_, claimErr := ClaimShipment(ctx, ws, shipment.ID)

	require.ErrorIs(t, claimErr, injectedForwardErr)
	require.False(t, errors.Is(claimErr, blerrors.ErrWriteIndeterminate),
		"the injected forward failure is known not to have applied")
	require.True(t, failureInjected, "member2 activation must trigger the injected failure")
	require.Equal(t, 1, member1MarkedBeforeFailure,
		"member1's marked activation write must complete before member2 fails")
	requireP1C6DurableStateRestored(t, ws, before)

	restoredMember1, err := findArtifact(ctx, ws, member1.ID)
	require.NoError(t, err)
	require.Nil(t, restoredMember1.CustomFields,
		"rollback must restore member1's nil custom_fields")
}

func TestU3_JournalRecoveryRollbackRestoresPreimage(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	disableExecGateForTest(t, ws)
	workspaceClosed := false
	t.Cleanup(func() {
		if !workspaceClosed {
			require.NoError(t, ws.Close())
		}
	})

	parent, err := CreateArtifact(ctx, ws, "U3 recovery feature", "feature")
	require.NoError(t, err)
	member1, err := CreateArtifact(ctx, ws, "U3 recovery member 1", "task", WithParent(parent.ID))
	require.NoError(t, err)
	member1.CustomFields = nil
	require.NoError(t, persistArtifact(ctx, ws, member1, false))

	member2, err := CreateArtifact(ctx, ws, "U3 recovery member 2", "task", WithParent(parent.ID))
	require.NoError(t, err)
	member2.CustomFields = map[string]any{"retained": "member2-preimage"}
	require.NoError(t, persistArtifact(ctx, ws, member2, false))

	shipment, err := CreateShipment(ctx, ws, "U3 recovery shipment", []string{member1.ID, member2.ID})
	require.NoError(t, err)
	require.Equal(t, []string{member1.ID, member2.ID}, NormalizeShipmentItems(shipment),
		"the manifest must order member1 before member2")
	require.Equal(t, models.StatusQueued, member1.Status)
	require.Equal(t, models.StatusQueued, member2.Status)

	member1Path, err := FindArtifactPath(ctx, ws, member1.ID)
	require.NoError(t, err)
	member1EventLogPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), member1.ID)
	before := snapshotURGovernedState(t, ws, []string{shipment.ID, member1.ID, member2.ID})

	injectedForwardErr := errors.New("injected member2 activation failure")
	injectedRestoreErr := errors.New("injected member1 artifact-file restore failure")
	originalWriter := persistArtifactWriteFn
	member1MarkedWrites := 0
	member1MarkedBeforeFailure := 0
	forwardFailureInjected := false
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == member1.ID &&
			artifact.Status == models.StatusActive &&
			artifact.CustomFields[schedulerBaselineClaimKey] == shipment.ID {
			if err := originalWriter(artifact, filePath, durable); err != nil {
				return err
			}
			member1MarkedWrites++
			return nil
		}
		if artifact.ID == member2.ID && artifact.Status == models.StatusActive {
			member1MarkedBeforeFailure = member1MarkedWrites
			forwardFailureInjected = true
			return injectedForwardErr
		}
		return originalWriter(artifact, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	originalRestore := restoreShipmentSnapshotFn
	restoreFailureCount := 0
	restoreFailurePath := ""
	restoreShipmentSnapshotFn = func(snapshot fileSnapshot) error {
		if restoreFailureCount == 0 &&
			filepath.Clean(snapshot.Path) == filepath.Clean(member1Path) {
			restoreFailureCount++
			restoreFailurePath = snapshot.Path
			return injectedRestoreErr
		}
		return originalRestore(snapshot)
	}
	t.Cleanup(func() { restoreShipmentSnapshotFn = originalRestore })

	_, claimErr := ClaimShipment(ctx, ws, shipment.ID)

	require.True(t, forwardFailureInjected, "member2 activation must trigger the injected failure")
	require.Equal(t, 1, member1MarkedBeforeFailure,
		"member1's marked activation write must complete before member2 fails")
	require.Equal(t, 1, restoreFailureCount,
		"compensation must fail once on member1's artifact file")
	require.Equal(t, filepath.Clean(member1Path), filepath.Clean(restoreFailurePath))
	require.NotEqual(t, filepath.Clean(member1EventLogPath), filepath.Clean(restoreFailurePath),
		"the injected restore failure must target the artifact file, not its event log")

	var partial *blerrors.MutationPartialError
	require.ErrorAs(t, claimErr, &partial)
	require.Equal(t, "double-fault", partial.Class)
	require.Equal(t, "partially-compensated", partial.CompensationState)
	require.ErrorIs(t, claimErr, injectedForwardErr)
	require.ErrorIs(t, claimErr, injectedRestoreErr)
	require.False(t, errors.Is(claimErr, blerrors.ErrWriteIndeterminate),
		"the forward failure is known not to have applied")
	requireClaimRecoveryIntent(t, ws, shipment.ID)

	frontmatterMember1, _, err := parseFile(member1Path)
	require.NoError(t, err)
	require.Equal(t, models.StatusActive, frontmatterMember1.Status,
		"member1's artifact file must retain its forward active state before recovery")
	frontmatterMarker, frontmatterHasMarker := frontmatterMember1.CustomFields[schedulerBaselineClaimKey]
	require.True(t, frontmatterHasMarker,
		"member1's artifact file must retain the claim marker before recovery")
	require.Equal(t, shipment.ID, frontmatterMarker)

	indexedMember1, err := bldb.GetItem(ctx, ws.DB, member1.ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusQueued, indexedMember1.Status,
		"compensation must restore member1's queued index preimage")
	_, indexHasMarker := indexedMember1.CustomFields[schedulerBaselineClaimKey]
	require.False(t, indexHasMarker,
		"compensation must restore member1's unmarked index preimage")

	persistArtifactWriteFn = originalWriter
	restoreShipmentSnapshotFn = originalRestore
	require.NoError(t, ws.Close())
	workspaceClosed = true
	recovered, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, recovered.Close()) })

	requireP1C6DurableStateRestored(t, recovered, before)
	recoveredMember1, err := findArtifact(ctx, recovered, member1.ID)
	require.NoError(t, err)
	require.Nil(t, recoveredMember1.CustomFields,
		"journal recovery must restore member1's nil custom_fields")

	_, err = ClaimShipment(ctx, recovered, shipment.ID)
	require.NoError(t, err, "a claim must succeed after journal recovery restores the preimage")
}
