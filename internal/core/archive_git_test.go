package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

type gitArchiveFixture struct {
	root            string
	database        *sql.DB
	workspace       *Workspace
	itemID          string
	originalTitle   string
	originalContent string
	queuePath       string
	archivePath     string
	queueRel        string
	archiveRel      string
}

func TestPlanArtifactMove_StrategyDetection(t *testing.T) {
	ctx := context.Background()

	t.Run("non git workspace falls back to filesystem move", func(t *testing.T) {
		root, sourcePath, destPath := setupMoveStrategyFiles(t)

		plan, err := planArtifactMove(ctx, root, sourcePath, destPath)

		require.NoError(t, err)
		assert.Equal(t, artifactMoveFilesystem, plan.kind)
	})

	t.Run("missing git binary falls back to filesystem move", func(t *testing.T) {
		emptyPathDir := t.TempDir()
		t.Setenv("PATH", emptyPathDir)
		root, sourcePath, destPath := setupMoveStrategyFiles(t)

		plan, err := planArtifactMove(ctx, root, sourcePath, destPath)

		require.NoError(t, err)
		assert.Equal(t, artifactMoveFilesystem, plan.kind)
	})

	t.Run("untracked file in git workspace falls back to filesystem move", func(t *testing.T) {
		root, sourcePath, destPath := setupMoveStrategyFiles(t)
		initGitRepo(t, root)

		plan, err := planArtifactMove(ctx, root, sourcePath, destPath)

		require.NoError(t, err)
		assert.Equal(t, artifactMoveFilesystem, plan.kind)
	})

	t.Run("tracked file in git workspace selects git move", func(t *testing.T) {
		root, sourcePath, destPath := setupMoveStrategyFiles(t)
		initGitRepo(t, root)
		sourceRel := filepath.ToSlash(mustRel(t, root, sourcePath))
		runGit(t, root, "add", sourceRel)
		runGit(t, root, "commit", "-m", "track source")

		plan, err := planArtifactMove(ctx, root, sourcePath, destPath)

		require.NoError(t, err)
		assert.Equal(t, artifactMoveGit, plan.kind)
		assert.Equal(t, sourceRel, plan.sourceRel)
		assert.Equal(t, filepath.ToSlash(mustRel(t, root, destPath)), plan.destRel)
	})

	t.Run("unexpected git worktree probe error fails closed", func(t *testing.T) {
		fakeDir := writeFakeGit(t, fakeGitScripts{
			windows: "@echo off\r\necho fatal: object database corrupt 1>&2\r\nexit /b 128\r\n",
			unix:    "#!/bin/sh\necho 'fatal: object database corrupt' >&2\nexit 128\n",
		})
		t.Setenv("PATH", fakeDir)
		root, sourcePath, destPath := setupMoveStrategyFiles(t)

		_, err := planArtifactMove(ctx, root, sourcePath, destPath)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "detect git worktree")
	})

	t.Run("unexpected tracked-file probe error fails closed", func(t *testing.T) {
		root, sourcePath, destPath := setupMoveStrategyFiles(t)
		fakeDir := writeFakeGit(t, fakeGitForUnexpectedLsFiles(root))
		t.Setenv("PATH", fakeDir)

		_, err := planArtifactMove(ctx, root, sourcePath, destPath)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "detect tracked artifact")
	})

	t.Run("git probe timeout fails closed", func(t *testing.T) {
		fakeDir := writeFakeGit(t, fakeGitScripts{
			windows: "@echo off\r\npowershell -NoProfile -Command \"Start-Sleep -Milliseconds 500\"\r\nexit /b 0\r\n",
			unix:    "#!/bin/sh\nsleep 1\nexit 0\n",
		})
		t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		oldTimeout := artifactGitCommandTimeout
		artifactGitCommandTimeout = 10 * time.Millisecond
		t.Cleanup(func() { artifactGitCommandTimeout = oldTimeout })
		root, sourcePath, destPath := setupMoveStrategyFiles(t)

		_, err := planArtifactMove(ctx, root, sourcePath, destPath)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "detect git worktree")
	})
}

func TestArchiveItem_TrackedGitArtifactUsesGitMoveAndPreservesFollowHistory(t *testing.T) {
	fx := newGitArchiveFixture(t, "101-T")
	ctx := context.Background()

	record, err := ArchiveItem(ctx, fx.database, fx.workspace, fx.itemID)

	require.NoError(t, err)
	assert.Equal(t, fx.archivePath, record.ArchivePath)
	assert.NoFileExists(t, fx.queuePath)
	assert.FileExists(t, fx.archivePath)
	assertGitRenameStaged(t, fx.root, fx.queueRel, fx.archiveRel)
	assertGitWorktreeModified(t, fx.root, fx.archiveRel)

	runGit(t, fx.root, "add", fx.archiveRel)
	runGit(t, fx.root, "commit", "-m", "archive tracked task")
	log := runGit(t, fx.root, "log", "--follow", "--format=%s", "--", fx.archiveRel)
	assert.Contains(t, log, "archive tracked task")
	assert.Contains(t, log, "seed tracked task")
}

func TestArchiveItem_UntrackedGitArtifactUsesFilesystemFallback(t *testing.T) {
	fx := newGitArchiveFixture(t, "102-T", withoutTrackedArtifact())
	ctx := context.Background()

	record, err := ArchiveItem(ctx, fx.database, fx.workspace, fx.itemID)

	require.NoError(t, err)
	assert.Equal(t, fx.archivePath, record.ArchivePath)
	assert.NoFileExists(t, fx.queuePath)
	assert.FileExists(t, fx.archivePath)
	cached := runGit(t, fx.root, "diff", "--cached", "--name-status", "--find-renames")
	assert.NotRegexp(t, `(?m)^R[0-9]*\s+`, cached)
}

func TestUnarchiveItem_TrackedGitArtifactUsesGitMoveBack(t *testing.T) {
	fx := newGitArchiveFixture(t, "103-T")
	ctx := context.Background()
	_, err := ArchiveItem(ctx, fx.database, fx.workspace, fx.itemID)
	require.NoError(t, err)
	runGit(t, fx.root, "add", fx.archiveRel)
	runGit(t, fx.root, "commit", "-m", "archive tracked task")
	requireGitClean(t, fx.root)

	err = UnarchiveItem(ctx, fx.database, fx.workspace, fx.itemID)

	require.NoError(t, err)
	assert.FileExists(t, fx.queuePath)
	assert.NoFileExists(t, fx.archivePath)
	assertGitRenameStaged(t, fx.root, fx.archiveRel, fx.queueRel)
	assertGitWorktreeModified(t, fx.root, fx.queueRel)

	runGit(t, fx.root, "add", fx.queueRel)
	runGit(t, fx.root, "commit", "-m", "restore tracked task")
	log := runGit(t, fx.root, "log", "--follow", "--format=%s", "--", fx.queueRel)
	assert.Contains(t, log, "restore tracked task")
	assert.Contains(t, log, "archive tracked task")
	assert.Contains(t, log, "seed tracked task")
}

func TestPerformArtifactMove_GitMoveErrorsFailClosed(t *testing.T) {
	tests := []struct {
		name      string
		operation string
	}{
		{name: "archive move", operation: "archive"},
		{name: "restore move", operation: "restore"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			initGitRepo(t, root)
			sourcePath := filepath.Join(root, "source.md")
			destPath := filepath.Join(root, "dest.md")
			require.NoError(t, os.WriteFile(sourcePath, []byte("source\n"), 0o644))
			require.NoError(t, os.WriteFile(destPath, []byte("dest\n"), 0o644))
			runGit(t, root, "add", "source.md", "dest.md")
			runGit(t, root, "commit", "-m", "seed files")
			plan := artifactMovePlan{
				kind:         artifactMoveGit,
				workTreeRoot: root,
				sourceRel:    "source.md",
				destRel:      "dest.md",
			}

			err := performArtifactMove(context.Background(), plan, tc.operation)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.operation+" git mv")
			assert.FileExists(t, sourcePath)
			assert.FileExists(t, destPath)
		})
	}
}

func TestArchiveItem_GitMoveDBFailureRollsBackWorktreeAndIndex(t *testing.T) {
	fx := newGitArchiveFixture(t, "104-T")
	requireGitClean(t, fx.root)
	require.NoError(t, fx.database.Close())

	_, err := ArchiveItem(context.Background(), fx.database, fx.workspace, fx.itemID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sync archive state")
	assert.FileExists(t, fx.queuePath)
	assert.NoFileExists(t, fx.archivePath)
	raw, readErr := os.ReadFile(fx.queuePath)
	require.NoError(t, readErr)
	assert.Equal(t, fx.originalContent, string(raw))
	requireGitClean(t, fx.root)
}

func TestUnarchiveItem_GitMoveDBFailureRollsBackWorktreeAndIndex(t *testing.T) {
	fx := newGitArchiveFixture(t, "105-T")
	ctx := context.Background()
	_, err := ArchiveItem(ctx, fx.database, fx.workspace, fx.itemID)
	require.NoError(t, err)
	runGit(t, fx.root, "add", fx.archiveRel)
	runGit(t, fx.root, "commit", "-m", "archive tracked task")
	archiveRaw, readErr := os.ReadFile(fx.archivePath)
	require.NoError(t, readErr)
	requireGitClean(t, fx.root)
	require.NoError(t, fx.database.Close())

	err = UnarchiveItem(ctx, fx.database, fx.workspace, fx.itemID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sync unarchive state")
	assert.FileExists(t, fx.archivePath)
	assert.NoFileExists(t, fx.queuePath)
	raw, readErr := os.ReadFile(fx.archivePath)
	require.NoError(t, readErr)
	assert.Equal(t, string(archiveRaw), string(raw))
	requireGitClean(t, fx.root)
}

type gitArchiveFixtureOption func(*gitArchiveFixtureConfig)

type gitArchiveFixtureConfig struct {
	trackArtifact bool
}

func withoutTrackedArtifact() gitArchiveFixtureOption {
	return func(cfg *gitArchiveFixtureConfig) {
		cfg.trackArtifact = false
	}
}

func newGitArchiveFixture(t *testing.T, itemID string, opts ...gitArchiveFixtureOption) gitArchiveFixture {
	t.Helper()
	cfg := gitArchiveFixtureConfig{trackArtifact: true}
	for _, opt := range opts {
		opt(&cfg)
	}

	root := t.TempDir()
	backlogDir := filepath.Join(root, ".backlogit")
	queueDir := filepath.Join(backlogDir, "queue")
	archiveDir := filepath.Join(backlogDir, "archive")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".backlogit/backlogit.db\n.backlogit/backlogit.db*\n.backlogit/logs/\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), []byte("artifact_types:\n  task:\n    prefix: T\n    suffix: \"-T\"\n    name_format: \"{NNN}{suffix}\"\nmax_slug_length: 60\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() {
		// Best-effort cleanup; rollback tests close the database before cleanup.
		_ = database.Close()
	})

	title := "Git tracked task " + itemID
	body := strings.Repeat("Stable history body line\n", 20)
	content := fmt.Sprintf("---\nid: %s\ntitle: %s\nstatus: done\nartifact_type: task\n---\n%s", itemID, title, body)
	queuePath := filepath.Join(queueDir, itemID+".md")
	archivePath := filepath.Join(archiveDir, itemID+".md")
	require.NoError(t, os.WriteFile(queuePath, []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID:           itemID,
		Title:        title,
		Status:       models.StatusDone,
		ArtifactType: "task",
	}))

	initGitRepo(t, root)
	runGit(t, root, "add", ".gitignore", ".backlogit/config.yaml", ".backlogit/header-def.yaml", ".backlogit/registry.yaml")
	if cfg.trackArtifact {
		runGit(t, root, "add", filepath.ToSlash(mustRel(t, root, queuePath)))
	}
	runGit(t, root, "commit", "-m", "seed tracked task")

	return gitArchiveFixture{
		root:            root,
		database:        database,
		workspace:       &Workspace{RootPath: root, DB: database},
		itemID:          itemID,
		originalTitle:   title,
		originalContent: content,
		queuePath:       queuePath,
		archivePath:     archivePath,
		queueRel:        filepath.ToSlash(mustRel(t, root, queuePath)),
		archiveRel:      filepath.ToSlash(mustRel(t, root, archivePath)),
	}
}

func setupMoveStrategyFiles(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	sourcePath := filepath.Join(root, ".backlogit", "queue", "001-T.md")
	destPath := filepath.Join(root, ".backlogit", "archive", "001-T.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(sourcePath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(destPath), 0o755))
	require.NoError(t, os.WriteFile(sourcePath, []byte("body\n"), 0o644))
	return root, sourcePath, destPath
}

type fakeGitScripts struct {
	windows string
	unix    string
}

func fakeGitForUnexpectedLsFiles(root string) fakeGitScripts {
	escapedRoot := strings.ReplaceAll(root, `\`, `\\`)
	return fakeGitScripts{
		windows: fmt.Sprintf("@echo off\r\nif \"%%3\"==\"rev-parse\" (\r\n  echo %s\r\n  exit /b 0\r\n)\r\necho fatal: index file corrupt 1>&2\r\nexit /b 128\r\n", root),
		unix:    fmt.Sprintf("#!/bin/sh\nif [ \"$3\" = \"rev-parse\" ]; then\n  printf '%%s\\n' '%s'\n  exit 0\nfi\necho 'fatal: index file corrupt' >&2\nexit 128\n", escapedRoot),
	}
}

func writeFakeGit(t *testing.T, scripts fakeGitScripts) string {
	t.Helper()
	dir := t.TempDir()
	name := "git"
	body := scripts.unix
	if runtime.GOOS == "windows" {
		name = "git.bat"
		body = scripts.windows
	}
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
	return dir
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	requireGit(t)
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "backlogit-tests@example.invalid")
	runGit(t, root, "config", "user.name", "Backlogit Tests")
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	requireGit(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		require.NoError(t, ctx.Err(), "git %s timed out", strings.Join(args, " "))
	}
	require.NoErrorf(t, err, "git %s failed:\n%s", strings.Join(args, " "), string(out))
	return string(out)
}

func assertGitRenameStaged(t *testing.T, root, fromRel, toRel string) {
	t.Helper()
	cached := runGit(t, root, "diff", "--cached", "--name-status", "--find-renames")
	assert.Regexp(t, `(?m)^R[0-9]*\s+`, cached)
	assert.Contains(t, cached, fromRel)
	assert.Contains(t, cached, toRel)
}

func assertGitWorktreeModified(t *testing.T, root, rel string) {
	t.Helper()
	unstaged := runGit(t, root, "diff", "--name-only")
	assert.Contains(t, unstaged, rel)
}

func requireGitClean(t *testing.T, root string) {
	t.Helper()
	status := strings.TrimSpace(runGit(t, root, "status", "--short"))
	require.Empty(t, status)
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	require.NoError(t, err)
	return rel
}
