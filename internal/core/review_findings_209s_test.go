package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

func TestU1_HistoricalStandaloneOrdinalAcceptsSluggedIDs(t *testing.T) {
	taskCfg := &config.ArtifactTypeConfig{Prefix: "T", Suffix: "-T"}
	tests := []struct {
		name   string
		ids    []string
		cfg    *config.ArtifactTypeConfig
		expect int
	}{
		{"strict suffix", []string{"001-T"}, taskCfg, 1},
		{"slugged suffix", []string{"007-T-implement-jwt"}, taskCfg, 7},
		{"slugged nested", []string{"001.012-T-some-slug"}, taskCfg, 12},
		{"other type not attributed", []string{"009-F-feature-slug", "008-TX-foo"}, taskCfg, 0},
		{"max across forms", []string{"003-T", "005-T-x", "004-T-y"}, taskCfg, 5},
		{"prefix-only slugged", []string{"T004-slug"}, &config.ArtifactTypeConfig{Prefix: "T"}, 4},
		{"prefix-only other", []string{"X004-slug"}, &config.ArtifactTypeConfig{Prefix: "T"}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, maxStandaloneOrdinalAmong(tc.ids, tc.cfg))
		})
	}
}

func TestU1_NextIDReservesSluggedHistoricalID(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	_, err := ws.DB.ExecContext(ctx,
		`INSERT INTO item_logs (item_id, log_path, updated_at) VALUES ('004-T-implement-jwt', 'logs/x.jsonl', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	cfg := &config.ArtifactTypeConfig{Prefix: "T", Suffix: "-T", NameFormat: "{NNN}{suffix}-{title_slug}"}
	next, err := NextID(ctx, ws.DB, "task", cfg)
	require.NoError(t, err)
	assert.Equal(t, 5, next)
}

func TestU1_AdoptPropagationErrorRollsBackAndReleasesLock(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	archive := filepath.Join(ws.StorageRoot, "archive")
	require.NoError(t, os.MkdirAll(archive, 0o755))
	stashPath := filepath.Join(archive, stashArchiveFileName)
	corrPath := filepath.Join(archive, "provenance_corrections.jsonl")
	stashBefore := `{"id":"S1","priority":"high","kind":"bug","text":"t","archived_at":"2026-01-01T00:00:00Z","reason":"harvested","harvested_artifact_id":"001.001-T"}` + "\n"
	corrBefore := "{not json\n"
	require.NoError(t, os.WriteFile(stashPath, []byte(stashBefore), 0o644))
	require.NoError(t, os.WriteFile(corrPath, []byte(corrBefore), 0o644))

	txn, err := propagateAdoptToStashArchive(ws, "001.001-T", "002.001-T")
	require.Error(t, err)
	assert.Nil(t, txn)

	assert.Equal(t, stashBefore, string(u2ReadFile(t, stashPath)), "stash archive must be restored")
	assert.Equal(t, corrBefore, string(u2ReadFile(t, corrPath)))

	done := make(chan error, 1)
	go func() {
		unlock, lockErr := lockStashFile(stashPath)
		if lockErr == nil {
			lockErr = unlock()
		}
		done <- lockErr
	}()
	select {
	case lockErr := <-done:
		require.NoError(t, lockErr)
	case <-time.After(10 * time.Second):
		t.Fatal("stash lock was not released after propagation failure")
	}

	matches, globErr := filepath.Glob(filepath.Join(archive, "*adopt-tmp*"))
	require.NoError(t, globErr)
	assert.Empty(t, matches, "no temp files may be left behind")
}

func TestU1_WriteFileAtomicBytesIgnoresPlantedSymlinkAtLegacyTempPath(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	require.NoError(t, os.WriteFile(victim, []byte("keep"), 0o644))
	target := filepath.Join(dir, "stash.jsonl")
	if err := os.Symlink(victim, target+".adopt-tmp"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	require.NoError(t, writeFileAtomicBytes(target, []byte("new")))
	assert.Equal(t, "keep", string(u2ReadFile(t, victim)))
	assert.Equal(t, "new", string(u2ReadFile(t, target)))
}

func TestU2_ForcedCorrectionResumesIncompleteMutation(t *testing.T) {
	ctx := context.Background()
	ws, target, corrections := u2StashFixture(t)
	before := u2ReadFile(t, target)

	_, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
	require.NoError(t, err)
	ledger := u2ReadFile(t, corrections)

	// Simulate a crash after the ledger append but before the stamp completed.
	require.NoError(t, os.WriteFile(target, before, 0o644))
	_, err = ws.DB.ExecContext(ctx, `DELETE FROM stash_links WHERE stash_id = 'ST0001'`)
	require.NoError(t, err)

	res, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
	require.NoError(t, err)
	assert.Equal(t, StashProvenanceCorrected, res.Outcome, "incomplete forced mutation must be resumed, not reported as no-op")
	assert.Equal(t, "ST0001", u2CustomFields(t, target)["source_stash_id"])
	var itemID string
	require.NoError(t, ws.DB.QueryRowContext(ctx, `SELECT item_id FROM stash_links WHERE stash_id='ST0001'`).Scan(&itemID))
	assert.Equal(t, "002-F", itemID)
	assert.Equal(t, string(ledger), string(u2ReadFile(t, corrections)), "resume must not append a duplicate ledger entry")

	again, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
	require.NoError(t, err)
	assert.Equal(t, StashProvenanceNoOp, again.Outcome)
}

func TestU2_DoctorManifestIgnoresArchiveAncestorOfWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive", "proj")
	bl := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(bl, 0o755))
	require.NoError(t, config.WriteDefaults(bl))
	ctx := context.Background()
	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })
	disableExecGateForTest(t, ws)

	u2Artifact(t, ws, "010-F", "feature", "active", "", "")
	u2ShipmentFM(t, ws, "901-S", "queued", "", "010-F", "999-T")
	report, err := Doctor(ctx, ws, &DoctorOptions{})
	require.NoError(t, err)
	assert.Len(t, u2Find(report, FindingDanglingShipmentMember), 1,
		"a checkout beneath an 'archive' directory must still be checked")
}

func TestU2_PathUnderArchiveRelativeToStorageRoot(t *testing.T) {
	root := filepath.Join("x", "archive", "proj", ".backlogit")
	assert.False(t, pathUnderArchive(root, filepath.Join(root, "queue", "a.md")))
	assert.True(t, pathUnderArchive(root, filepath.Join(root, "archive", "a.md")))
	assert.True(t, pathUnderArchive(root, filepath.Join(root, "archive", "sub", "a.md")))
}
