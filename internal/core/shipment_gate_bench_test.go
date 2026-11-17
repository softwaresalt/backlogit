package core

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	identity := append(os.Environ(),
		"GIT_AUTHOR_NAME=Ancestry Test", "GIT_AUTHOR_EMAIL=ancestry@example.com",
		"GIT_COMMITTER_NAME=Ancestry Test", "GIT_COMMITTER_EMAIL=ancestry@example.com",
	)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = identity
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s: %s", strings.Join(args, " "), out)
	return strings.TrimSpace(string(out))
}

func seedDoneMemberWithHead(t *testing.T, ctx context.Context, ws *Workspace, headSHA string) string {
	t.Helper()
	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGatePassed, map[string]any{
		"outcome":  "passed",
		"ran":      true,
		"head_sha": headSHA,
	}))
	return id
}

func TestU3b_ValidateMemberGateEvidenceMemoizesImmutableCommitAncestry(t *testing.T) {
	t.Run("memoizes_across_gate_passes_and_respects_ctx_and_uncached_sha", func(t *testing.T) {
		ctx := context.Background()
		ws := newGateTestWorkspace(t)
		memberHead, shipmentHead, uncachedHead := initGitRepoWithCommits(t, ws.RootPath)

		memberID := seedDoneMemberWithHead(t, ctx, ws, memberHead)
		uncachedMemberID := seedDoneMemberWithHead(t, ctx, ws, uncachedHead)
		memberIDs := []string{memberID}

		require.NoError(t, validateMemberGateEvidence(ctx, ws, memberIDs, shipmentHead))

		objectsDir := filepath.Join(ws.RootPath, ".git", "objects")
		objectsBakDir := filepath.Join(ws.RootPath, ".git", "objects.bak")
		require.NoError(t, os.Rename(objectsDir, objectsBakDir))
		t.Cleanup(func() {
			if _, statErr := os.Stat(objectsBakDir); statErr == nil {
				_ = os.Rename(objectsBakDir, objectsDir)
			}
		})

		require.NoError(t, validateMemberGateEvidence(ctx, ws, memberIDs, shipmentHead),
			"second pass must succeed from Workspace ancestry cache when .git/objects is unavailable")

		uncachedErr := validateMemberGateEvidence(ctx, ws, []string{uncachedMemberID}, shipmentHead)
		require.Error(t, uncachedErr, "uncached commit SHA must still fail closed when .git/objects is moved aside")

		_, ancErr := ws.isAncestor(ctx, uncachedHead, shipmentHead)
		require.Error(t, ancErr, "uncached pair must fail closed in isAncestor when .git/objects is moved aside")

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		cancelErr := validateMemberGateEvidence(canceledCtx, ws, memberIDs, shipmentHead)
		require.Error(t, cancelErr, "pre-canceled context must fail closed even for cached ancestry pair")

		_, isAncCancelErr := ws.isAncestor(canceledCtx, memberHead, shipmentHead)
		require.Error(t, isAncCancelErr, "pre-canceled context must fail closed in isAncestor even for cached pair")
		assert.ErrorIs(t, isAncCancelErr, context.Canceled)
	})

	t.Run("enforces_no_replace_objects_against_refs_replace", func(t *testing.T) {
		ctx := context.Background()
		ws := newGateTestWorkspace(t)
		memberHead, shipmentHead, _ := initGitRepoWithCommits(t, ws.RootPath)

		runTestGit(t, ws.RootPath, "checkout", "--orphan", "unrelated")
		require.NoError(t, os.WriteFile(filepath.Join(ws.RootPath, "unrelated.txt"), []byte("unrelated\n"), 0o644))
		runTestGit(t, ws.RootPath, "add", "unrelated.txt")
		runTestGit(t, ws.RootPath, "commit", "-m", "unrelated root")
		unrelatedRoot := runTestGit(t, ws.RootPath, "rev-parse", "HEAD")
		runTestGit(t, ws.RootPath, "checkout", "main")

		runTestGit(t, ws.RootPath, "update-ref", "refs/replace/"+shipmentHead, unrelatedRoot)

		ok, err := ws.isAncestor(ctx, memberHead, shipmentHead)
		require.NoError(t, err)
		assert.True(t, ok, "isAncestor must enforce --no-replace-objects and ignore refs/replace")
	})

	t.Run("bypasses_and_invalidates_cache_on_shallow_or_grafts", func(t *testing.T) {
		ctx := context.Background()
		ws := newGateTestWorkspace(t)
		c1, c2, _ := initGitRepoWithCommits(t, ws.RootPath)

		ok, err := ws.isAncestor(ctx, c1, c2)
		require.NoError(t, err)
		assert.True(t, ok, "c1 must initially be an ancestor of c2")

		shallowPath := filepath.Join(ws.RootPath, ".git", "shallow")
		require.NoError(t, os.WriteFile(shallowPath, []byte(c2+"\n"), 0o644))

		ok, err = ws.isAncestor(ctx, c1, c2)
		require.NoError(t, err)
		assert.False(t, ok, "shallow boundary on c2 must bypass/invalidate cache and return false")

		require.NoError(t, os.Remove(shallowPath))

		ok, err = ws.isAncestor(ctx, c1, c2)
		require.NoError(t, err)
		assert.True(t, ok, "removing .git/shallow must invalidate prior shallow result and re-evaluate to true")

		graftsPath := filepath.Join(ws.RootPath, ".git", "info", "grafts")
		require.NoError(t, os.MkdirAll(filepath.Dir(graftsPath), 0o755))
		require.NoError(t, os.WriteFile(graftsPath, []byte(c2+"\n"), 0o644))

		ok, err = ws.isAncestor(ctx, c1, c2)
		require.NoError(t, err)
		assert.False(t, ok, "presence of .git/info/grafts must invalidate/bypass cache and return false")
	})

	t.Run("nil_ctx_normalized_and_pre_exec_eligibility_reported", func(t *testing.T) {
		ws := newGateTestWorkspace(t)
		c1, c2, _ := initGitRepoWithCommits(t, ws.RootPath)

		//nolint:staticcheck // intentional: verifies nil context normalization in isAncestor
		ok, err := ws.isAncestor(nil, c1, c2)
		require.NoError(t, err)
		assert.True(t, ok)

		shallowPath := filepath.Join(ws.RootPath, ".git", "shallow")
		require.NoError(t, os.WriteFile(shallowPath, []byte(c2+"\n"), 0o644))
		_, hit, eligible := ws.lookupAncestryCache(strings.ToLower(c1) + "|" + strings.ToLower(c2))
		assert.False(t, hit)
		assert.False(t, eligible, "shallow repo must report preExecEligible=false and clear cache")
	})

	t.Run("symlinked_root_into_shallow_repo_does_not_read_outer_git_or_cache_stale", func(t *testing.T) {
		ctx := context.Background()

		// Outer complete repo that must NOT be resolved when ws.RootPath is a
		// symlink/junction pointing into a different (shallow) repository.
		outerRoot := t.TempDir()
		initGitRepoWithCommits(t, outerRoot)

		// Target repo whose subdirectory will be the symlink/junction target.
		shallowRepo := t.TempDir()
		c1, c2, _ := initGitRepoWithCommits(t, shallowRepo)
		shallowSub := filepath.Join(shallowRepo, "sub")
		require.NoError(t, os.MkdirAll(shallowSub, 0o755))

		wsLink := filepath.Join(outerRoot, "ws-link")
		if err := linkDirForMemberSwapTest(wsLink, shallowSub); err != nil {
			t.Skipf("directory link/junction creation skipped: %v", err)
		}

		ws := &Workspace{RootPath: wsLink}

		// Prime the cache while shallowRepo is complete.
		ok, err := ws.isAncestor(ctx, c1, c2)
		require.NoError(t, err)
		assert.True(t, ok, "c1 must initially be an ancestor of c2 through symlinked root")

		// Mark the target repo shallow. Walking unresolved parents of wsLink
		// would step up into outerRoot/.git (complete) and serve stale cache!
		shallowPath := filepath.Join(shallowRepo, ".git", "shallow")
		require.NoError(t, os.WriteFile(shallowPath, []byte(c2+"\n"), 0o644))

		ok, err = ws.isAncestor(ctx, c1, c2)
		require.NoError(t, err)
		assert.False(t, ok, "shallow target repo must bypass/invalidate cache and not read outer .git")

		_, hit, eligible := ws.lookupAncestryCache(strings.ToLower(c1) + "|" + strings.ToLower(c2))
		assert.False(t, hit)
		assert.False(t, eligible, "symlinked root into shallow repo must report preExecEligible=false")

		// Removing .git/shallow restores normal evaluation.
		require.NoError(t, os.Remove(shallowPath))
		ok, err = ws.isAncestor(ctx, c1, c2)
		require.NoError(t, err)
		assert.True(t, ok, "removing target .git/shallow must re-evaluate to true")

		// Non-existent / dangling root disables caching cleanly without erroring in resolver.
		gitDir, resolvedOK := resolveAncestryGitDir(filepath.Join(outerRoot, "nonexistent-root"))
		assert.Empty(t, gitDir)
		assert.False(t, resolvedOK)
	})

	t.Run("symlinked_root_with_relative_GIT_DIR_resolves_against_target", func(t *testing.T) {
		outerRoot := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(outerRoot, ".git"), 0o755))

		targetRepo := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(targetRepo, ".git"), 0o755))
		targetSub := filepath.Join(targetRepo, "sub")
		require.NoError(t, os.MkdirAll(targetSub, 0o755))

		wsLink := filepath.Join(outerRoot, "ws-link")
		if err := linkDirForMemberSwapTest(wsLink, targetSub); err != nil {
			t.Skipf("directory link/junction creation skipped: %v", err)
		}

		t.Setenv("GIT_DIR", "../.git")

		resolvedTarget, ok := resolveAncestryRootPath(targetRepo)
		require.True(t, ok)
		expectedGitDir := filepath.Join(resolvedTarget, ".git")

		gitDir, resolvedOK := resolveAncestryGitDir(wsLink)
		require.True(t, resolvedOK)
		assert.Equal(t, expectedGitDir, gitDir, "relative GIT_DIR must resolve against symlink target, not lexical symlink parent")
		assert.True(t, isCompleteUngraftedRepo(wsLink))

		shallowPath := filepath.Join(targetRepo, ".git", "shallow")
		require.NoError(t, os.WriteFile(shallowPath, []byte(strings.Repeat("a", 40)+"\n"), 0o644))

		assert.False(t, isCompleteUngraftedRepo(wsLink), "must respect .git/shallow in symlink target repo")
		ws := &Workspace{RootPath: wsLink}
		_, hit, eligible := ws.lookupAncestryCache(strings.Repeat("a", 40) + "|" + strings.Repeat("b", 40))
		assert.False(t, hit)
		assert.False(t, eligible)

		require.NoError(t, os.Remove(shallowPath))
		assert.True(t, isCompleteUngraftedRepo(wsLink))
	})
}
