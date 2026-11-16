package core

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

const (
	// benchMemberCount is the release-scope size profiled by the U6 spike.
	benchMemberCount = 45
	// benchProgressEventsPerMember is the number of progress comments written to
	// each member log before its gate-pass evidence, approximating real task history.
	benchProgressEventsPerMember = 6
)

// newBenchMemberGateFixture builds a temporary, non-git workspace whose release
// scope holds benchMemberCount completed task members. Each member log carries
// creation, activation, progress comments, and one passing gate evidence event,
// the shape validateMemberGateEvidence reads at ship time.
func newBenchMemberGateFixture(b *testing.B) (*Workspace, []string) {
	b.Helper()
	ctx := context.Background()
	silenceBenchLogs(b)

	root := b.TempDir()
	backlogDir := filepath.Join(root, ".backlogit")
	require.NoError(b, os.MkdirAll(backlogDir, 0o755))
	require.NoError(b, config.WriteDefaults(backlogDir))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(b, err)
	b.Cleanup(func() { _ = ws.Close() })
	// Keep the fixture off the executable gate broker; members are completed
	// through the ungated path in seedBenchMember.
	ws.GateBroker = nil

	feat, err := CreateArtifact(ctx, ws, "U6 benchmark release feature", "feature")
	require.NoError(b, err)

	scope := make([]string, 0, benchMemberCount)
	for i := range benchMemberCount {
		scope = append(scope, seedBenchMember(ctx, b, ws, feat.ID, i))
	}
	return ws, scope
}

// silenceBenchLogs discards slog output for the rest of the benchmark. Fixture
// logs would otherwise share a line with the benchmark name and break the ns/op
// line that the U6 completion gate parses.
func silenceBenchLogs(b *testing.B) {
	b.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	b.Cleanup(func() { slog.SetDefault(prev) })
}

// seedBenchMember creates one task under parentID, drives it through its real
// lifecycle with progress comments, and appends a passing gate evidence event.
// It returns the member ID.
func seedBenchMember(ctx context.Context, b *testing.B, ws *Workspace, parentID string, index int) string {
	b.Helper()
	task, err := CreateArtifact(ctx, ws, fmt.Sprintf("U6 benchmark member %02d", index), "task", WithParent(parentID))
	require.NoError(b, err)
	_, err = UpdateArtifact(ctx, ws, task.ID, map[string]any{"status": "active"})
	require.NoError(b, err)
	for j := range benchProgressEventsPerMember {
		require.NoError(b, AppendComment(ctx, ws, nil, task.ID, "ship", fmt.Sprintf("progress note %d", j), ""))
	}
	_, err = updateArtifactUngated(ctx, ws, task.ID, map[string]any{"status": "done"})
	require.NoError(b, err)
	require.NoError(b, appendItemEventErr(ctx, ws, task.ID, EventGatePassed, map[string]any{
		"outcome": "passed",
		"ran":     true,
	}))
	return task.ID
}

// BenchmarkValidateMemberGateEvidence profiles ship-time member validation over a
// 45-member release scope. The shipment head is empty, so the per-member git
// lineage check is skipped and the timed loop isolates artifact lookup and
// event-log reads.
func BenchmarkValidateMemberGateEvidence(b *testing.B) {
	ctx := context.Background()
	ws, scope := newBenchMemberGateFixture(b)

	b.ResetTimer()
	for range b.N {
		if err := validateMemberGateEvidence(ctx, ws, scope, ""); err != nil {
			b.Fatalf("validate member gate evidence: %v", err)
		}
	}
}

func newU3aWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "archive"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return ws, backlogitDir
}

func seedU3aArtifact(t *testing.T, ws *Workspace, filePath string, artifact *models.Artifact) {
	t.Helper()
	require.NoError(t, WriteArtifactFile(artifact, filePath))
	require.NoError(t, db.UpsertItem(context.Background(), ws.DB, artifact))
}

func TestU3a_SnapshotShipArtifactsSinglePassContainedDirectLookup(t *testing.T) {
	t.Run("direct_probe_avoids_archive_walk_with_escaping_sibling", func(t *testing.T) {
		ctx := context.Background()
		ws, backlogitDir := newU3aWorkspace(t)

		outsideDir := t.TempDir()
		outsideFile := filepath.Join(outsideDir, "000-unrelated.md")
		require.NoError(t, os.WriteFile(outsideFile, []byte("---\nid: 000-unrelated\nartifact_type: task\nstatus: done\ntitle: Outside\n---\n"), 0o644))

		escapingArchiveLink := filepath.Join(backlogitDir, "archive", "000-unrelated.md")
		if err := os.Symlink(outsideFile, escapingArchiveLink); err != nil {
			t.Skipf("symlink creation skipped: %v", err)
		}

		seedU3aArtifact(t, ws, filepath.Join(backlogitDir, "queue", "901-F.md"), &models.Artifact{
			ID:           "901-F",
			Title:        "U3a feature",
			Status:       models.StatusActive,
			ArtifactType: "feature",
			Priority:     "medium",
		})

		memberIDs := []string{"901-F", "901.001-T", "901.002-T", "901.003-T", "901.004-T"}
		for _, id := range memberIDs[1:] {
			seedU3aArtifact(t, ws, filepath.Join(backlogitDir, "queue", id+".md"), &models.Artifact{
				ID:           id,
				Title:        "U3a task " + id,
				Status:       models.StatusActive,
				ArtifactType: "task",
				ParentID:     "901-F",
				Priority:     "medium",
			})
		}

		snapshots, err := snapshotShipArtifacts(ctx, ws, memberIDs)
		require.NoError(t, err, "direct probe must resolve queue members without walking archive/000-unrelated.md")
		require.Len(t, snapshots, len(memberIDs))
		for _, id := range memberIDs {
			snap, ok := snapshots[id]
			require.True(t, ok, "missing snapshot for %s", id)
			require.NotNil(t, snap.artifact)
			require.NotNil(t, snap.dbItem)
			assert.Equal(t, id, snap.artifact.ID)
			assert.Equal(t, id, snap.dbItem.ID)
			assert.True(t, snap.file.Exists)
		}
	})

	t.Run("direct_lookup_still_enforces_containment_and_walkdir_fallback", func(t *testing.T) {
		ctx := context.Background()

		t.Run("walkdir_fallback_for_slug_filename", func(t *testing.T) {
			ws, backlogitDir := newU3aWorkspace(t)
			slugPath := filepath.Join(backlogitDir, "queue", "901-F-custom-slug.md")
			seedU3aArtifact(t, ws, slugPath, &models.Artifact{
				ID:           "901-F",
				Title:        "U3a slug feature",
				Status:       models.StatusActive,
				ArtifactType: "feature",
				Priority:     "medium",
			})

			found, err := findArtifact(ctx, ws, "901-F")
			require.NoError(t, err)
			require.NotNil(t, found)
			assert.Equal(t, "901-F", found.ID)

			foundPath, err := FindArtifactPath(ctx, ws, "901-F")
			require.NoError(t, err)
			assert.Equal(t, filepath.Clean(slugPath), filepath.Clean(foundPath))
		})

		t.Run("direct_lookup_containment", func(t *testing.T) {
			ws, backlogitDir := newU3aWorkspace(t)

			outsideDir := t.TempDir()
			outsideFile := filepath.Join(outsideDir, "000-unrelated.md")
			require.NoError(t, os.WriteFile(outsideFile, []byte("---\nid: 000-unrelated\nartifact_type: task\nstatus: done\ntitle: Outside\n---\n"), 0o644))

			escapingArchiveLink := filepath.Join(backlogitDir, "archive", "000-unrelated.md")
			if err := os.Symlink(outsideFile, escapingArchiveLink); err != nil {
				t.Skipf("symlink creation skipped: %v", err)
			}

			_, err := findArtifact(ctx, ws, "000-unrelated")
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrValidation)
			assert.Contains(t, err.Error(), "outside")

			_, err = FindArtifactPath(ctx, ws, "000-unrelated")
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrValidation)
			assert.Contains(t, err.Error(), "outside")
		})
	})

	t.Run("snapshotShipArtifacts_uses_findArtifactWithPath_ast", func(t *testing.T) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "shipment_lifecycle.go", nil, 0)
		require.NoError(t, err)

		var snapshotFn *ast.FuncDecl
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == "snapshotShipArtifacts" {
				snapshotFn = fn
				break
			}
		}
		require.NotNil(t, snapshotFn, "snapshotShipArtifacts must be declared in shipment_lifecycle.go")
		require.NotNil(t, snapshotFn.Body)

		var callsFindArtifactWithPath, callsFindArtifact, callsFindArtifactPath bool
		ast.Inspect(snapshotFn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			switch ident.Name {
			case "findArtifactWithPath":
				callsFindArtifactWithPath = true
			case "findArtifact":
				callsFindArtifact = true
			case "FindArtifactPath":
				callsFindArtifactPath = true
			}
			return true
		})

		assert.True(t, callsFindArtifactWithPath, "snapshotShipArtifacts must call findArtifactWithPath for single-pass lookup")
		assert.False(t, callsFindArtifact && callsFindArtifactPath, "snapshotShipArtifacts must not call both findArtifact and FindArtifactPath")
	})
}
