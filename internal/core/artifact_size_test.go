package core_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

// U7 (071.007-T): core.SetArtifactSize is the single body-preserving seam for the
// size field. It must (a) enum-validate before any write, (b) preserve the body
// byte-for-byte, (c) keep non-size index columns intact through the full-row
// UpsertItem, and (d) acquire the per-task advisory lock.

// goldenTaskFile is handcrafted with a distinctive body (heading, trailing
// spaces, bullets, blank lines) so the golden assertion is non-vacuous.
const goldenTaskFile = "---\n" +
	"artifact_type: task\n" +
	"created_at: 2026-06-30T22:51:20.664-07:00\n" +
	"id: 900.001-T\n" +
	"parent_id: 900-F\n" +
	"priority: high\n" +
	"status: active\n" +
	"title: Golden body task\n" +
	"updated_at: 2026-06-30T22:51:20.664-07:00\n" +
	"---\n" +
	"\n" +
	"# Heading\n" +
	"\n" +
	"Line with trailing spaces:   \n" +
	"- bullet one\n" +
	"- bullet two\n" +
	"\n" +
	"Final paragraph.\n"

func newRecoveryFreeFixtureWorkspace(ctx context.Context, root string) (*core.Workspace, error) {
	ws, err := core.NewWorkspace(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("new recovery-free fixture workspace: %w", err)
	}
	return ws, nil
}

func setupSizeWorkspace(t *testing.T) (ws *core.Workspace, id, path string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := newRecoveryFreeFixtureWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	path = filepath.Join(backlogitDir, "queue", "900.001-T.md")
	require.NoError(t, os.WriteFile(path, []byte(goldenTaskFile), 0o644))

	// Seed the index row so we can prove the non-size columns survive.
	art := &models.Artifact{
		ID:           "900.001-T",
		Title:        "Golden body task",
		Status:       models.StatusActive,
		ArtifactType: "task",
		ParentID:     "900-F",
		Priority:     "high",
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, art))
	return ws, "900.001-T", path
}

func TestRecoveryFreeFixture_SkipsShipmentRecoveryButLoadsTemplates(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	opsPath := filepath.Join(backlogitDir, "ops")
	require.NoFileExists(t, opsPath)
	require.NoDirExists(t, opsPath)
	require.NoError(t, os.WriteFile(opsPath, []byte("not a directory"), 0o644))

	recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelRecovery()
	recoveryWS, recoveryErr := core.NewWorkspace(recoveryCtx, root)
	if recoveryWS != nil {
		t.Cleanup(func() { require.NoError(t, recoveryWS.Close()) })
	}
	require.Error(t, recoveryErr)
	require.ErrorContains(t, recoveryErr, "recover shipment operations")
	require.ErrorIs(t, recoveryErr, blerrors.ErrValidation)
	require.Nil(t, recoveryWS)

	isolatedCtx, cancelIsolated := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelIsolated()
	isolatedWS, isolatedErr := newRecoveryFreeFixtureWorkspace(isolatedCtx, root)
	require.NoError(t, isolatedErr)
	t.Cleanup(func() { require.NoError(t, isolatedWS.Close()) })
	require.NotEmpty(t, isolatedWS.Templates)
}

func TestSetArtifactSize_PersistsAndPreservesIndexColumns(t *testing.T) {
	ctx := context.Background()
	ws, id, path := setupSizeWorkspace(t)

	before, err := db.GetItem(ctx, ws.DB, id)
	require.NoError(t, err)

	_, err = core.SetArtifactSize(ctx, ws, id, "M")
	require.NoError(t, err)

	// File carries custom_fields.size.
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	md, err := mdfront.Decode(raw)
	require.NoError(t, err)
	cf, ok := md.Frontmatter["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields map must be present")
	assert.Equal(t, "M", cf["size"])

	// Index reflects size AND preserves the other columns (full-row upsert guard:
	// a partial {ID,CustomFields} stub would null these).
	after, err := db.GetItem(ctx, ws.DB, id)
	require.NoError(t, err)
	assert.Equal(t, before.Title, after.Title, "title must be unchanged")
	assert.Equal(t, before.Status, after.Status, "status must be unchanged")
	assert.Equal(t, before.Priority, after.Priority, "priority must be unchanged")
	assert.Equal(t, "M", after.CustomFields["size"])
}

func TestSetArtifactSize_RejectsInvalidValueBeforeWrite(t *testing.T) {
	ctx := context.Background()
	ws, id, path := setupSizeWorkspace(t)

	rawBefore, err := os.ReadFile(path)
	require.NoError(t, err)

	_, err = core.SetArtifactSize(ctx, ws, id, "XXL")
	require.Error(t, err, "out-of-enum size must be rejected")

	rawAfter, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, rawBefore, rawAfter, "no write may occur on an invalid value")
}

func TestSetArtifactSize_GoldenBodyPreserved(t *testing.T) {
	ctx := context.Background()
	ws, id, path := setupSizeWorkspace(t)

	rawBefore, err := os.ReadFile(path)
	require.NoError(t, err)
	mdBefore, err := mdfront.Decode(rawBefore)
	require.NoError(t, err)

	_, err = core.SetArtifactSize(ctx, ws, id, "L")
	require.NoError(t, err)

	rawAfter, err := os.ReadFile(path)
	require.NoError(t, err)
	mdAfter, err := mdfront.Decode(rawAfter)
	require.NoError(t, err)

	// Body bytes must be byte-identical.
	assert.Equal(t, mdBefore.Body, mdAfter.Body, "body must be preserved byte-for-byte")

	// Frontmatter must be semantically equal to the original plus custom_fields.size.
	expected := map[string]any{}
	for k, v := range mdBefore.Frontmatter {
		expected[k] = v
	}
	expected["custom_fields"] = map[string]any{"size": "L"}
	assert.Equal(t, expected, mdAfter.Frontmatter)
}

func TestSetArtifactSize_Idempotent(t *testing.T) {
	ctx := context.Background()
	ws, id, path := setupSizeWorkspace(t)

	_, err := core.SetArtifactSize(ctx, ws, id, "S")
	require.NoError(t, err)
	rawFirst, err := os.ReadFile(path)
	require.NoError(t, err)

	_, err = core.SetArtifactSize(ctx, ws, id, "S")
	require.NoError(t, err)
	rawSecond, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, rawFirst, rawSecond, "re-applying the same size must be a no-op on disk")
}

func TestSetArtifactSize_BusyLockReturnsErrTaskBusy(t *testing.T) {
	ctx := context.Background()
	ws, id, _ := setupSizeWorkspace(t)

	// Simulate a concurrently held shared artifact lock with the same
	// OS-level advisory primitive used by production.
	locksDir := filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), ".locks", "artifacts")
	require.NoError(t, os.MkdirAll(locksDir, 0o755))
	stableKey := filepath.Join(locksDir, hex.EncodeToString([]byte(id)))
	sidecar := filepath.Join(filepath.Dir(stableKey), "."+filepath.Base(stableKey)+".lock")
	release, err := holdAdvisoryLock(sidecar)
	require.NoError(t, err)
	t.Cleanup(release)

	_, err = core.SetArtifactSize(ctx, ws, id, "M")
	require.Error(t, err)
	assert.ErrorIs(t, err, core.ErrTaskBusy)
}
