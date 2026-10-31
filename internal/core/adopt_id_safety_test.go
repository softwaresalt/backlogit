package core_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// u1StorageRoot returns the workspace storage root.
func u1StorageRoot(ws *core.Workspace) string {
	return core.WorkspaceStorageRoot(ws.RootPath)
}

// u1Sync rebuilds the SQLite index from disk, mirroring backlogit_sync_index.
func u1Sync(t *testing.T, ws *core.Workspace) {
	t.Helper()
	_, err := bldb.Rehydrate(context.Background(), u1StorageRoot(ws), ws.DB)
	require.NoError(t, err)
}

func u1WriteFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func u1ReadLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func u1Decode(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &m))
	return m
}

func u1LoadArtifact(t *testing.T, ws *core.Workspace, id string) *models.Artifact {
	t.Helper()
	path, err := core.FindArtifactPath(context.Background(), ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fm, body, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	a, err := models.ArtifactFromFrontmatter(fm, body)
	require.NoError(t, err)
	return a
}

type u1Fixture struct {
	ws    *core.Workspace
	feat1 *models.Artifact
	feat2 *models.Artifact
	task  *models.Artifact
}

func u1NewFixture(t *testing.T, taskOpts ...core.Option) u1Fixture {
	t.Helper()
	ws := setupTestWorkspace(t)
	ctx := context.Background()
	feat1, err := core.CreateArtifact(ctx, ws, "U1 origin feature", "feature")
	require.NoError(t, err)
	feat2, err := core.CreateArtifact(ctx, ws, "U1 destination feature", "feature")
	require.NoError(t, err)
	opts := append([]core.Option{core.WithParent(feat1.ID)}, taskOpts...)
	task, err := core.CreateArtifact(ctx, ws, "U1 task", "task", opts...)
	require.NoError(t, err)
	return u1Fixture{ws: ws, feat1: feat1, feat2: feat2, task: task}
}

const u1StashID = "AAAA1111"

func u1StashLine(id, harvested string) string {
	return `{"id":"` + id + `","priority":"medium","kind":"idea","text":"t ` + id +
		`","archived_at":"2026-01-01T00:00:00Z","reason":"harvested","harvested_artifact_id":"` + harvested + `"}`
}

func u1CorrectionLine(stash, hist, canonical string, forced bool) string {
	f := ""
	if forced {
		f = `,"forced":true`
	}
	return `{"stash_id":"` + stash + `","historical_artifact_id":"` + hist +
		`","canonical_delivery_artifact_id":"` + canonical +
		`","reason":"manual","actor":"op","corrected_at":"2026-01-02T00:00:00Z","event_type":"stash_provenance_corrected"` + f + `}`
}

func TestU1_AdoptPropagationKeepIDAndHistorySafeAllocation(t *testing.T) {
	ctx := context.Background()

	t.Run("ShipmentManifestRewrite", func(t *testing.T) {
		fx := u1NewFixture(t)
		other, err := core.CreateArtifact(ctx, fx.ws, "U1 other task", "task", core.WithParent(fx.feat1.ID))
		require.NoError(t, err)
		shipment, err := core.CreateShipment(ctx, fx.ws, "U1 shipment", []string{fx.task.ID, other.ID})
		require.NoError(t, err)

		// Pre-seed the manifest with the future new ID to force dedupe.
		path, err := core.FindArtifactPath(ctx, fx.ws, shipment.ID)
		require.NoError(t, err)
		sa := u1LoadArtifact(t, fx.ws, shipment.ID)
		predictedNew := fx.feat2.ID[:strings.Index(fx.feat2.ID, "-")] + ".001-T"
		sa.CustomFields["items"] = []string{fx.task.ID, other.ID, predictedNew}
		require.NoError(t, core.WriteArtifactFileWithOptions(sa, path, false))
		u1Sync(t, fx.ws)

		res, err := core.AdoptItem(ctx, fx.ws, fx.task.ID, fx.feat2.ID)
		require.NoError(t, err)
		require.Equal(t, predictedNew, res.NewID)
		assert.Contains(t, res.RewrittenArtifactIDs, shipment.ID)

		after := u1LoadArtifact(t, fx.ws, shipment.ID)
		items := core.NormalizeShipmentItems(after)
		require.NotNil(t, items)
		assert.Equal(t, []string{res.NewID, other.ID}, items, "order preserved, old ID replaced, duplicate dropped")
	})

	t.Run("KeepID", func(t *testing.T) {
		fx := u1NewFixture(t)
		before, err := core.FindArtifactPath(ctx, fx.ws, fx.task.ID)
		require.NoError(t, err)

		res, err := core.AdoptItemWithOptions(ctx, fx.ws, fx.task.ID, fx.feat2.ID, core.AdoptItemOptions{KeepID: true})
		require.NoError(t, err)
		assert.Equal(t, fx.task.ID, res.NewID)
		assert.Equal(t, fx.feat2.ID, res.NewParentID)

		afterPath, err := core.FindArtifactPath(ctx, fx.ws, fx.task.ID)
		require.NoError(t, err)
		assert.Equal(t, before, afterPath, "KeepID must not rename the file")

		a := u1LoadArtifact(t, fx.ws, fx.task.ID)
		assert.Equal(t, fx.feat2.ID, a.ParentID)
		assert.Equal(t, fx.feat1.ID, a.CustomFields["origin_feature"])

		logLines := u1ReadLines(t, filepath.Join(u1StorageRoot(fx.ws), "logs", fx.task.ID+".jsonl"))
		found := false
		for _, l := range logLines {
			if u1Decode(t, l)["event_type"] == "adopted" {
				found = true
			}
		}
		assert.True(t, found, "an adopted event must be appended")
	})

	t.Run("StashArchiveAndCorrectionPropagation", func(t *testing.T) {
		fx := u1NewFixture(t)
		archive := filepath.Join(u1StorageRoot(fx.ws), "archive")
		stashPath := filepath.Join(archive, "stash.jsonl")
		corrPath := filepath.Join(archive, "provenance_corrections.jsonl")
		oldID := fx.task.ID
		u1WriteFile(t, stashPath, strings.Join([]string{
			u1StashLine("S1111111", oldID),
			u1StashLine("S2222222", "009.009-T"),
		}, "\n")+"\n")
		u1WriteFile(t, corrPath, strings.Join([]string{
			u1CorrectionLine("S1111111", "009.001-T", oldID, true),
			u1CorrectionLine("S3333333", "009.002-T", oldID, false),
			u1CorrectionLine("S3333333", "009.002-T", "009.003-T", false),
		}, "\n")+"\n")

		res, err := core.AdoptItem(ctx, fx.ws, oldID, fx.feat2.ID)
		require.NoError(t, err)
		newID := res.NewID
		require.NotEqual(t, oldID, newID)

		stashLines := u1ReadLines(t, stashPath)
		require.Len(t, stashLines, 2)
		assert.Equal(t, newID, u1Decode(t, stashLines[0])["harvested_artifact_id"])
		assert.Equal(t, "harvested", u1Decode(t, stashLines[0])["reason"])
		assert.Equal(t, "009.009-T", u1Decode(t, stashLines[1])["harvested_artifact_id"])

		corr := u1ReadLines(t, corrPath)
		require.Len(t, corr, 4, "exactly one successor entry for the latest record pointing at the old ID")
		succ := u1Decode(t, corr[3])
		assert.Equal(t, "S1111111", succ["stash_id"])
		assert.Equal(t, "009.001-T", succ["historical_artifact_id"])
		assert.Equal(t, newID, succ["canonical_delivery_artifact_id"])
		assert.Equal(t, "backlogit", succ["actor"])
		assert.Equal(t, "adopt rename "+oldID+" -> "+newID, succ["reason"])
		assert.Equal(t, true, succ["forced"])
		// Original history is untouched.
		assert.Equal(t, u1CorrectionLine("S1111111", "009.001-T", oldID, true), corr[0])
	})

	t.Run("StashArchiveRollbackOnFailure", func(t *testing.T) {
		fx := u1NewFixture(t)
		dep, err := core.CreateArtifact(ctx, fx.ws, "U1 dependent", "task",
			core.WithParent(fx.feat1.ID), core.WithDependencies([]string{fx.task.ID}))
		require.NoError(t, err)
		archive := filepath.Join(u1StorageRoot(fx.ws), "archive")
		stashPath := filepath.Join(archive, "stash.jsonl")
		corrPath := filepath.Join(archive, "provenance_corrections.jsonl")
		stashBefore := u1StashLine("S1111111", fx.task.ID) + "\n"
		corrBefore := u1CorrectionLine("S1111111", "009.001-T", fx.task.ID, false) + "\n"
		u1WriteFile(t, stashPath, stashBefore)
		u1WriteFile(t, corrPath, corrBefore)

		depPath, err := core.FindArtifactPath(ctx, fx.ws, dep.ID)
		require.NoError(t, err)
		require.NoError(t, os.Chmod(depPath, 0o444))
		t.Cleanup(func() { _ = os.Chmod(depPath, 0o644) })

		_, err = core.AdoptItem(ctx, fx.ws, fx.task.ID, fx.feat2.ID)
		require.Error(t, err)

		gotStash, err := os.ReadFile(stashPath)
		require.NoError(t, err)
		gotCorr, err := os.ReadFile(corrPath)
		require.NoError(t, err)
		assert.Equal(t, stashBefore, string(gotStash), "stash archive must be restored")
		assert.Equal(t, corrBefore, string(gotCorr), "corrections must be restored")
	})

	t.Run("StashArchiveSymlinkRefused", func(t *testing.T) {
		fx := u1NewFixture(t)
		archive := filepath.Join(u1StorageRoot(fx.ws), "archive")
		require.NoError(t, os.MkdirAll(archive, 0o755))
		target := filepath.Join(t.TempDir(), "real-stash.jsonl")
		u1WriteFile(t, target, u1StashLine("S1111111", fx.task.ID)+"\n")
		if err := os.Symlink(target, filepath.Join(archive, "stash.jsonl")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := core.AdoptItem(ctx, fx.ws, fx.task.ID, fx.feat2.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "symlink")

		a := u1LoadArtifact(t, fx.ws, fx.task.ID)
		assert.Equal(t, fx.feat1.ID, a.ParentID, "item must be unchanged after refusal")
		raw, readErr := os.ReadFile(target)
		require.NoError(t, readErr)
		assert.Contains(t, string(raw), fx.task.ID)
	})

	t.Run("CorrectAdoptSyncKeepsStashLinkAtNewID", func(t *testing.T) {
		fx := u1NewFixture(t, core.WithFields(map[string]any{"source_stash_id": u1StashID}))
		hist, err := core.CreateArtifact(ctx, fx.ws, "U1 historical", "task",
			core.WithParent(fx.feat1.ID), core.WithFields(map[string]any{"source_stash_id": u1StashID}))
		require.NoError(t, err)
		stashPath := filepath.Join(u1StorageRoot(fx.ws), "archive", "stash.jsonl")
		u1WriteFile(t, stashPath, u1StashLine(u1StashID, hist.ID)+"\n")

		_, err = core.CorrectStashProvenance(ctx, fx.ws, core.StashProvenanceCorrectionRequest{
			StashID: u1StashID, CanonicalDeliveryArtifactID: fx.task.ID, Reason: "delivery", Actor: "tester",
		})
		require.NoError(t, err)

		res, err := core.AdoptItem(ctx, fx.ws, fx.task.ID, fx.feat2.ID)
		require.NoError(t, err)
		require.NotEqual(t, fx.task.ID, res.NewID)

		u1Sync(t, fx.ws)
		var itemID string
		require.NoError(t, fx.ws.DB.QueryRowContext(ctx,
			`SELECT item_id FROM stash_links WHERE stash_id = ?`, u1StashID).Scan(&itemID))
		assert.Equal(t, res.NewID, itemID)
	})

	t.Run("AdoptThenSyncThenCreateDoesNotReuseRetiredID", func(t *testing.T) {
		fx := u1NewFixture(t)
		require.Equal(t, "001.001-T", fx.task.ID)
		require.Equal(t, "001-F", fx.feat1.ID)
		res, err := core.AdoptItem(ctx, fx.ws, fx.task.ID, fx.feat2.ID)
		require.NoError(t, err)
		require.Equal(t, "002.001-T", res.NewID)
		u1Sync(t, fx.ws)

		next, err := core.CreateArtifact(ctx, fx.ws, "U1 next", "task", core.WithParent(fx.feat1.ID))
		require.NoError(t, err)
		assert.Equal(t, "001.002-T", next.ID)
	})

	// History sources: each reserves 001.0NN-T so the next task under 001-F skips it.
	disk := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  string
	}{
		{"LogFilename", func(t *testing.T, root string) {
			u1WriteFile(t, filepath.Join(root, "logs", "001.005-T.jsonl"), "\n")
		}, "001.006-T"},
		{"EventItemID", func(t *testing.T, root string) {
			u1WriteFile(t, filepath.Join(root, "logs", "misc.jsonl"),
				`{"timestamp":"2026-01-01T00:00:00Z","actor":"a","item_id":"001.007-T","event_type":"created","delta":{}}`+"\n")
		}, "001.008-T"},
		{"EventDeltaOldID", func(t *testing.T, root string) {
			u1WriteFile(t, filepath.Join(root, "logs", "009.001-T.jsonl"),
				`{"timestamp":"2026-01-01T00:00:00Z","actor":"a","item_id":"009.001-T","event_type":"adopted","delta":{"old_id":"001.010-T","new_id":"009.001-T"}}`+"\n")
		}, "001.011-T"},
		{"StashArchive", func(t *testing.T, root string) {
			u1WriteFile(t, filepath.Join(root, "archive", "stash.jsonl"), u1StashLine("S1111111", "001.012-T")+"\n")
		}, "001.013-T"},
		{"ProvenanceCorrections", func(t *testing.T, root string) {
			u1WriteFile(t, filepath.Join(root, "archive", "provenance_corrections.jsonl"),
				u1CorrectionLine("S1111111", "001.014-T", "009.001-T", false)+"\n")
		}, "001.015-T"},
		{"ArchiveFilename", func(t *testing.T, root string) {
			u1WriteFile(t, filepath.Join(root, "archive", "001.016-T.md"),
				"---\nid: 001.016-T\nartifact_type: task\n---\n")
		}, "001.017-T"},
	}
	for _, tc := range disk {
		t.Run("Disk_"+tc.name, func(t *testing.T) {
			ws := setupTestWorkspace(t)
			feat, err := core.CreateArtifact(ctx, ws, "U1 feature", "feature")
			require.NoError(t, err)
			tc.setup(t, u1StorageRoot(ws))
			got, err := core.CreateArtifact(ctx, ws, "U1 next", "task", core.WithParent(feat.ID))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.ID)
		})
	}

	sqlite := []struct {
		name string
		stmt string
		args []any
		want string
	}{
		{"ItemLogs", `INSERT INTO item_logs (item_id, log_path, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
			[]any{"001.020-T", "logs/001.020-T.jsonl"}, "001.021-T"},
		{"ItemLogEntries", `INSERT INTO item_log_entries (item_id, log_path, timestamp, actor, event_type, content, delta_json) VALUES (?, 'logs/x.jsonl', CURRENT_TIMESTAMP, 'a', 'created', '', '')`,
			[]any{"001.022-T"}, "001.023-T"},
		{"ItemLogEntriesOldID", `INSERT INTO item_log_entries (item_id, log_path, timestamp, actor, event_type, content, delta_json) VALUES ('009.001-T', 'logs/y.jsonl', CURRENT_TIMESTAMP, 'a', 'adopted', '', ?)`,
			[]any{`{"old_id":"001.024-T"}`}, "001.025-T"},
		{"StashLinks", `INSERT INTO stash_links (stash_id, item_id, linked_at) VALUES ('S9999999', ?, CURRENT_TIMESTAMP)`,
			[]any{"001.026-T"}, "001.027-T"},
	}
	for _, tc := range sqlite {
		t.Run("SQLite_"+tc.name, func(t *testing.T) {
			ws := setupTestWorkspace(t)
			feat, err := core.CreateArtifact(ctx, ws, "U1 feature", "feature")
			require.NoError(t, err)
			_, err = ws.DB.ExecContext(ctx, tc.stmt, tc.args...)
			require.NoError(t, err)
			got, err := core.CreateArtifact(ctx, ws, "U1 next", "task", core.WithParent(feat.ID))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.ID)
		})
	}
}
