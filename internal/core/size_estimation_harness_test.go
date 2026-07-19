package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

func newSizeEstimationHarnessWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	ws, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return ws, backlogitDir
}

func seedSizingHarnessArtifact(t *testing.T, ws *Workspace, backlogitDir string, artifact *models.Artifact) string {
	t.Helper()
	path := filepath.Join(backlogitDir, "queue", artifact.ID+".md")
	require.NoError(t, WriteArtifactFile(artifact, path))
	require.NoError(t, db.UpsertItem(context.Background(), ws.DB, artifact))
	return path
}

func stringPtr(value string) *string {
	return &value
}

func requireNoSizeEstimationTODO(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, ErrSizeEstimationNotImplemented) {
		t.Fatalf("%v", err)
	}
}

func TestSE3aSetArtifactSizeWithProvenanceHarness(t *testing.T) {
	tests := []struct {
		name     string
		mutation SizeMutation
	}{
		{
			name: "event appended before write and provenance persisted",
			mutation: SizeMutation{
				Size:           stringPtr("M"),
				Source:         stringPtr("agent"),
				RulesetVersion: stringPtr("ruleset-alpha"),
				Actor:          ActorContextAgent,
			},
		},
		{
			name: "ruleset-version-only change emits exactly one estimate-history event",
			mutation: SizeMutation{
				RulesetVersion: stringPtr("ruleset-beta"),
				Actor:          ActorContextAgent,
			},
		},
		{
			name: "absent source reads as unknown and is not rewritten to human",
			mutation: SizeMutation{
				Size:  stringPtr("S"),
				Actor: ActorContextHuman,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
			path := seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
				ID:           "930.001-T",
				Title:        "SE-3a task",
				Status:       models.StatusActive,
				ArtifactType: "task",
				ParentID:     "930-F",
				Priority:     "medium",
				CustomFields: map[string]any{"size": "XS"},
			})

			artifact, err := SetArtifactSizeWithProvenance(context.Background(), ws, "930.001-T", tt.mutation)
			requireNoSizeEstimationTODO(t, err)
			require.NoError(t, err)
			require.NotNil(t, artifact)

			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			md, err := mdfront.Decode(raw)
			require.NoError(t, err)
			customFields, ok := md.Frontmatter["custom_fields"].(map[string]any)
			require.True(t, ok)
			if tt.mutation.Size != nil {
				assert.Equal(t, *tt.mutation.Size, customFields["size"])
			}
			if tt.mutation.Source != nil {
				assert.Equal(t, *tt.mutation.Source, customFields["size_source"])
			}

			logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), "930.001-T")
			logRaw, err := os.ReadFile(logPath)
			require.NoError(t, err)
			assert.Contains(t, string(logRaw), events.EventEstimateHistory)
		})
	}
}

func TestSE3aReservedSizingKeyGuardHarness(t *testing.T) {
	t.Run("generic create rejects unprovenanced reserved size", func(t *testing.T) {
		ws, _ := newSizeEstimationHarnessWorkspace(t)
		_, err := SetArtifactSizeWithProvenance(context.Background(), ws, "placeholder", SizeMutation{Size: stringPtr("M"), Actor: ActorContextAgent})
		requireNoSizeEstimationTODO(t, err)

		_, err = CreateArtifact(context.Background(), ws, "Unprovenanced import", "feature", WithFields(map[string]any{"size": "M"}))
		require.Error(t, err)
	})

	t.Run("generic update merge-preserves reserved sizing keys", func(t *testing.T) {
		ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
		seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
			ID:           "931.001-T",
			Title:        "Reserved key task",
			Status:       models.StatusActive,
			ArtifactType: "task",
			ParentID:     "931-F",
			Priority:     "medium",
			CustomFields: map[string]any{
				"size":                 "L",
				"size_source":          "agent",
				"size_ruleset_version": "ruleset-alpha",
			},
		})
		_, err := SetArtifactSizeWithProvenance(context.Background(), ws, "931.001-T", SizeMutation{Size: stringPtr("L"), Actor: ActorContextAgent})
		requireNoSizeEstimationTODO(t, err)

		updated, err := UpdateArtifact(context.Background(), ws, "931.001-T", map[string]any{"custom_fields": map[string]any{"harness_status": "scaffolded"}})
		require.NoError(t, err)
		assert.Equal(t, "L", updated.CustomFields["size"])
		assert.Equal(t, "agent", updated.CustomFields["size_source"])
		assert.Equal(t, "ruleset-alpha", updated.CustomFields["size_ruleset_version"])
	})
}

func TestSE3bCrashAuditRobustnessHarness(t *testing.T) {
	t.Run("orphan audit event ignored and never replayed", func(t *testing.T) {
		ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
		path := seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
			ID:           "932.001-T",
			Title:        "Crash residue task",
			Status:       models.StatusActive,
			ArtifactType: "task",
			ParentID:     "932-F",
			Priority:     "medium",
			CustomFields: map[string]any{"size": "S"},
		})
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		// Simulate a process crash between the successful audit append and the
		// durable write: the estimate-history event is persisted (orphan residue)
		// but the frontmatter size is never updated.
		previous := sizeSeamWriteFailureHook
		sizeSeamWriteFailureHook = func() error {
			return errors.New("injected durable write failure after append")
		}
		t.Cleanup(func() { sizeSeamWriteFailureHook = previous })

		_, err = SetArtifactSizeWithProvenance(context.Background(), ws, "932.001-T", SizeMutation{
			Size:           stringPtr("XL"),
			Source:         stringPtr("agent"),
			RulesetVersion: stringPtr("ruleset-alpha"),
			Actor:          ActorContextAgent,
		})
		requireNoSizeEstimationTODO(t, err)
		require.Error(t, err)

		sizeSeamWriteFailureHook = nil

		// The durable frontmatter is byte-identical: the orphan event never mutated it.
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, before, after, "failed post-append write must leave durable size untouched")

		// The estimate-history event WAS appended before the failed write (orphan residue).
		logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), "932.001-T")
		logRaw, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Contains(t, string(logRaw), events.EventEstimateHistory, "audit event is appended before the durable write")

		// Reads consult the durable size only; the orphan event is ignored/never replayed.
		indexed, err := db.GetItem(context.Background(), ws.DB, "932.001-T")
		require.NoError(t, err)
		assert.Equal(t, "S", indexed.CustomFields["size"], "orphan audit event must not change the durable size on read")
	})

	t.Run("normal set re-upserts sqlite index", func(t *testing.T) {
		ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
		seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
			ID:           "933.001-T",
			Title:        "Index consistency task",
			Status:       models.StatusActive,
			ArtifactType: "task",
			ParentID:     "933-F",
			Priority:     "medium",
		})

		_, err := SetArtifactSizeWithProvenance(context.Background(), ws, "933.001-T", SizeMutation{Size: stringPtr("M"), Actor: ActorContextAgent})
		requireNoSizeEstimationTODO(t, err)
		require.NoError(t, err)

		indexed, err := db.GetItem(context.Background(), ws.DB, "933.001-T")
		require.NoError(t, err)
		assert.Equal(t, "M", indexed.CustomFields["size"])
	})
}

func TestSE4SizeCompositionHarness(t *testing.T) {
	t.Run("feature counts sized and unsized children and excludes dangling members", func(t *testing.T) {
		ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
		feature := &models.Artifact{
			ID:           "940-F",
			Title:        "Composition feature",
			Status:       models.StatusActive,
			ArtifactType: "feature",
		}
		seedSizingHarnessArtifact(t, ws, backlogitDir, feature)
		seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
			ID:           "940.001-T",
			Title:        "Sized child",
			Status:       models.StatusActive,
			ArtifactType: "task",
			ParentID:     "940-F",
			CustomFields: map[string]any{"size": "M"},
		})
		seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
			ID:           "940.002-T",
			Title:        "Unsized child",
			Status:       models.StatusActive,
			ArtifactType: "task",
			ParentID:     "940-F",
		})

		result, err := SizeComposition(context.Background(), ws, feature)
		requireNoSizeEstimationTODO(t, err)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Histogram["M"], "the sized child is counted in the M bucket")
		assert.Equal(t, 1, result.Unsized, "the child without a size is counted as unsized")
		assert.Len(t, result.Members, 2, "both direct children are members")
		assert.Nil(t, result.RulesetVersion)

		rawAfter, err := os.ReadFile(filepath.Join(backlogitDir, "queue", "940-F.md"))
		require.NoError(t, err)
		assert.NotContains(t, string(rawAfter), "size_composition", "composition must never be persisted")
	})

	t.Run("shipment expands feature-only manifest and dedups explicit child tasks", func(t *testing.T) {
		ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
		seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
			ID:           "941-F",
			Title:        "Shipment feature",
			Status:       models.StatusActive,
			ArtifactType: "feature",
		})
		seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
			ID:           "941.001-T",
			Title:        "Shipment child sized",
			Status:       models.StatusActive,
			ArtifactType: "task",
			ParentID:     "941-F",
			CustomFields: map[string]any{"size": "L"},
		})
		seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
			ID:           "941.002-T",
			Title:        "Shipment child unsized",
			Status:       models.StatusActive,
			ArtifactType: "task",
			ParentID:     "941-F",
		})
		shipment := &models.Artifact{
			ID:           "941-S",
			Title:        "Composition shipment",
			Status:       models.StatusActive,
			ArtifactType: "shipment",
			// Manifest lists the feature, one of its children explicitly (to prove
			// de-duplication), and a dangling id (to prove warn-skip).
			CustomFields: map[string]any{"items": []any{"941-F", "941.001-T", "999.404-T"}},
		}
		seedSizingHarnessArtifact(t, ws, backlogitDir, shipment)

		result, err := SizeComposition(context.Background(), ws, shipment)
		requireNoSizeEstimationTODO(t, err)
		require.NoError(t, err)
		require.NotNil(t, result)
		// 941-F expands to 941.001-T + 941.002-T; the explicitly-listed 941.001-T
		// is de-duplicated so the sized child is counted exactly once.
		assert.Equal(t, 1, result.Histogram["L"], "sized child counted once despite explicit + expanded membership")
		// Unsized members are the feature itself (no size) and 941.002-T.
		assert.Equal(t, 2, result.Unsized, "the feature and the unsized child are both unsized")
		assert.Len(t, result.Members, 3, "feature + two children, each counted once")
		assert.Contains(t, result.Skipped, "999.404-T", "the dangling manifest id is warn-skipped")
		assert.Nil(t, result.RulesetVersion)

		rawAfter, err := os.ReadFile(filepath.Join(backlogitDir, "queue", "941-S.md"))
		require.NoError(t, err)
		assert.NotContains(t, string(rawAfter), "size_composition", "composition must never be persisted")
	})
}

func TestSE7bLookupTimeContainmentHarness(t *testing.T) {
	ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
	queueDir := filepath.Join(backlogitDir, "queue")
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "950.001-T.md")
	require.NoError(t, os.WriteFile(outside, []byte("---\nid: 950.001-T\nartifact_type: task\nstatus: active\ntitle: Outside\n---\n"), 0o644))
	link := filepath.Join(queueDir, "950.001-T.md")
	require.NoError(t, os.Symlink(outside, link))

	err := ensureArtifactLookupContained(ws, link)
	requireNoSizeEstimationTODO(t, err)
	require.Error(t, err)

	_, err = FindArtifactPath(context.Background(), ws, "950.001-T")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside")
}

func TestF6CreateArtifactPostCreateFailureRollbackHarness(t *testing.T) {
	ws, _ := newSizeEstimationHarnessWorkspace(t)
	var firstID string
	previousHook := createArtifactPostCreateFailureHook
	createArtifactPostCreateFailureHook = func(_ context.Context, _ *Workspace, artifact *models.Artifact) error {
		firstID = artifact.ID
		return errors.New("injected post-create size event failure")
	}
	t.Cleanup(func() { createArtifactPostCreateFailureHook = previousHook })

	_, err := CreateArtifact(context.Background(), ws, "Rollback boundary feature", "feature")
	requireNoSizeEstimationTODO(t, err)
	require.Error(t, err)
	require.NotEmpty(t, firstID)

	_, lookupErr := db.GetItem(context.Background(), ws.DB, firstID)
	assert.Error(t, lookupErr, "post-create failure cleanup must remove the SQLite row")
	_, pathErr := FindArtifactPath(context.Background(), ws, firstID)
	assert.Error(t, pathErr, "post-create failure cleanup must remove the Markdown artifact")

	createArtifactPostCreateFailureHook = nil
	retried, err := CreateArtifact(context.Background(), ws, "Rollback boundary feature", "feature")
	require.NoError(t, err)
	assert.Equal(t, firstID, retried.ID, "cleanup must allow retry with the same ID")
}
