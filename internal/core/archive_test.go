package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func setupArchiveWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	queueDir := filepath.Join(backlogDir, "queue")
	archiveDir := filepath.Join(backlogDir, "archive")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	// Write config.yaml
	configData := []byte("artifact_types:\n  task:\n    prefix: T\n    suffix: \"-T\"\n    name_format: \"{NNN}{suffix}\"\n  bug:\n    prefix: B\n    suffix: \"-B\"\n    name_format: \"{NNN}{suffix}\"\nmax_slug_length: 60\n")
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	// Seed a completed task file
	taskContent := "---\nid: 001-T\ntitle: Completed task\nstatus: done\nartifact_type: task\n---\nDone task body\n"
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "001-T.md"), []byte(taskContent), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "001-T", Title: "Completed task", Status: models.StatusDone, ArtifactType: "task",
	}))

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestArchiveItem_MovesToArchive(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	// Act
	record, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "001-T", record.ID)
	assert.NotEmpty(t, record.ArchivePath)
	assert.FileExists(t, record.ArchivePath)

	raw, readErr := os.ReadFile(record.ArchivePath)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	assert.Equal(t, ".backlogit/queue/001-T.md", fm["archived_from"])
}

func TestUnarchiveItem_RestoresFromArchive(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)

	// Act
	err = core.UnarchiveItem(ctx, ws.DB, ws, "001-T")

	// Assert
	require.NoError(t, err)
	originalPath := filepath.Join(ws.RootPath, ".backlogit", "queue", "001-T.md")
	assert.FileExists(t, originalPath)
}

func TestArchiveItem_ExcludedFromDefaultList(t *testing.T) {
	// GIVEN an archived item in the workspace
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)

	// WHEN querying with the default (empty) filters
	items, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{})

	// THEN the archived item must not appear
	require.NoError(t, err)
	for _, item := range items {
		assert.NotEqual(t, "001-T", item.ID, "archived item 001-T must be excluded from default query")
	}
}

func TestArchiveItem_IncludedWhenExplicitlyRequested(t *testing.T) {
	// GIVEN an archived item
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)

	// WHEN querying with IncludeArchived: true
	items, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{IncludeArchived: true})

	// THEN the archived item must be present
	require.NoError(t, err)
	found := false
	for _, item := range items {
		if item.ID == "001-T" {
			found = true
			assert.Equal(t, models.StatusArchived, item.Status)
		}
	}
	assert.True(t, found, "archived item 001-T must appear when IncludeArchived is true")
}

func TestUnarchiveItem_RestoresSuffixedFilenameByFrontmatterID(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	originalPath := filepath.Join(ws.RootPath, ".backlogit", "queue", "001-T.md")
	suffixedPath := filepath.Join(ws.RootPath, ".backlogit", "queue", "001-T-completed-task.md")
	require.NoError(t, os.Rename(originalPath, suffixedPath))

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)
	assert.FileExists(t, record.ArchivePath)

	err = core.UnarchiveItem(ctx, ws.DB, ws, "001-T")

	require.NoError(t, err)
	assert.FileExists(t, suffixedPath)
}

func TestUnarchiveItem_RejectsActiveArtifact(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	err := core.UnarchiveItem(ctx, ws.DB, ws, "001-T")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not archived")
}

// TestArchiveItem_ItemAlreadyInArchiveDir verifies that ArchiveItem does not
// delete the archive file when the item's current path is already inside the
// archive directory. This can happen when a registry routes a terminal status
// (e.g. "done") to the archive/ directory: completeReleaseScope moves the file
// there first, then archiveItems calls ArchiveItem a second time on the same
// item. Without the same-path guard, os.Remove(currentPath) would delete the
// file that was just written.
func TestArchiveItem_ItemAlreadyInArchiveDir(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	backlogDir := filepath.Join(ws.RootPath, ".backlogit")
	archiveDir := filepath.Join(backlogDir, "archive")
	queueDir := filepath.Join(backlogDir, "queue")

	// Seed item directly in archive/ (simulating status-routing move)
	alreadyArchivedContent := "---\nid: 002-T\ntitle: Already archived task\nstatus: done\nartifact_type: task\n---\nBody\n"
	archiveFilePath := filepath.Join(archiveDir, "002-T.md")
	require.NoError(t, os.WriteFile(archiveFilePath, []byte(alreadyArchivedContent), 0o644))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "002-T", Title: "Already archived task", Status: models.StatusDone, ArtifactType: "task",
	}))

	// Act: ArchiveItem on an item already in archive/
	record, err := core.ArchiveItem(ctx, ws.DB, ws, "002-T")

	// Assert: no error, archive file still exists, queue file not created
	require.NoError(t, err)
	assert.FileExists(t, archiveFilePath, "archive file must survive ArchiveItem when already in archive/")
	assert.Equal(t, archiveFilePath, record.ArchivePath)
	assert.NoFileExists(t, filepath.Join(queueDir, "002-T.md"), "no queue file should be created")
}

// TestUnarchiveItem_ArchiveFromEqualsArchivePath verifies that UnarchiveItem
// does not delete the file when archived_from resolves to the same path as
// the current archive location. This can happen when ArchiveItem encountered
// an item already in archive/ (same-path scenario) and stored the archive-dir
// path in the archived_from field. Without the same-path guard, os.Remove
// would delete the file that Rename just wrote in place.
func TestUnarchiveItem_ArchiveFromEqualsArchivePath(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	backlogDir := filepath.Join(ws.RootPath, ".backlogit")
	archiveDir := filepath.Join(backlogDir, "archive")

	// archived_from stores the archive-dir path (same as current location)
	archivePath := filepath.Join(archiveDir, "003-T.md")
	content := fmt.Sprintf(
		"---\nid: 003-T\ntitle: Self-archived task\nstatus: archived\nartifact_type: task\narchived_from: %s\n---\nBody\n",
		archivePath,
	)
	require.NoError(t, os.WriteFile(archivePath, []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "003-T", Title: "Self-archived task", Status: models.StatusArchived, ArtifactType: "task",
	}))

	// Act
	err := core.UnarchiveItem(ctx, ws.DB, ws, "003-T")

	// Assert: no error, file survives in archive dir
	require.NoError(t, err)
	assert.FileExists(t, archivePath, "archive file must survive when archived_from == archivePath")

	// Content should be readable and no longer contain archived_from
	raw, readErr := os.ReadFile(archivePath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(raw), "archived_from", "restored file must not contain archived_from field")
}

func TestAutoArchive_ProcessesExpiredItems(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	policy := &core.ArchivePolicy{
		TerminalStatuses: []string{"done", "accepted", "rejected"},
		RetentionDays:    0, // archive immediately
		ArchiveDir:       "archive",
	}

	// Act
	count, err := core.AutoArchive(ctx, ws.DB, ws, policy)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should archive the one completed task")
}

// setupArchiveCascadeWorkspace creates a workspace with a feature, two child tasks,
// and one subtask under the first task, all in queue/.
func setupArchiveCascadeWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	queueDir := filepath.Join(backlogDir, "queue")
	archiveDir := filepath.Join(backlogDir, "archive")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	configData := []byte(`artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
  task:
    prefix: T
    suffix: "-T"
    name_format: "{NNN}{suffix}"
  subtask:
    prefix: ST
    suffix: "-ST"
    name_format: "{NNN}{suffix}"
max_slug_length: 60
queue_layout:
  root_dir: queue
  levels:
    - depth: 1
      types: [feature]
    - depth: 2
      types: [task]
    - depth: 3
      types: [subtask]
`)
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	ctx := context.Background()
	items := []struct {
		id, title, atype, parentID string
	}{
		{"010-F", "Parent feature", "feature", ""},
		{"010.001-T", "Child task 1", "task", "010-F"},
		{"010.002-T", "Child task 2", "task", "010-F"},
		{"010.001.001-ST", "Subtask under task 1", "subtask", "010.001-T"},
	}
	for _, item := range items {
		content := fmt.Sprintf("---\nid: %s\ntitle: %s\nstatus: active\nartifact_type: %s\nparent_id: %s\n---\nBody\n", item.id, item.title, item.atype, item.parentID)
		if item.parentID == "" {
			content = fmt.Sprintf("---\nid: %s\ntitle: %s\nstatus: active\nartifact_type: %s\n---\nBody\n", item.id, item.title, item.atype)
		}
		require.NoError(t, os.WriteFile(filepath.Join(queueDir, item.id+".md"), []byte(content), 0o644))
		a := &models.Artifact{
			ID: item.id, Title: item.title, Status: models.StatusActive,
			ArtifactType: item.atype, ParentID: item.parentID,
		}
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestArchiveItem_CascadeArchivesChildren(t *testing.T) {
	ws := setupArchiveCascadeWorkspace(t)
	ctx := context.Background()

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "010-F", core.WithCascade(true))

	require.NoError(t, err)
	assert.Equal(t, "010-F", record.ID)
	assert.Len(t, record.CascadedItems, 3, "should cascade to 2 tasks + 1 subtask")
	assert.Contains(t, record.CascadedItems, "010.001.001-ST")
	assert.Contains(t, record.CascadedItems, "010.001-T")
	assert.Contains(t, record.CascadedItems, "010.002-T")
	assert.Empty(t, record.FailedItems)

	// All files should be in archive/
	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive")
	for _, id := range []string{"010-F", "010.001-T", "010.002-T", "010.001.001-ST"} {
		assert.FileExists(t, filepath.Join(archiveDir, id+".md"), "expected %s in archive", id)
	}
}

func TestArchiveItem_NoCascadeByDefault(t *testing.T) {
	ws := setupArchiveCascadeWorkspace(t)
	ctx := context.Background()

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "010-F")

	require.NoError(t, err)
	assert.Empty(t, record.CascadedItems, "cascade should not happen by default")

	// Children should still be in queue/
	queueDir := filepath.Join(ws.RootPath, ".backlogit", "queue")
	assert.FileExists(t, filepath.Join(queueDir, "010.001-T.md"))
	assert.FileExists(t, filepath.Join(queueDir, "010.002-T.md"))
}

func TestArchiveItem_CascadeSkipsAlreadyArchived(t *testing.T) {
	ws := setupArchiveCascadeWorkspace(t)
	ctx := context.Background()

	// Pre-archive one child
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "010.001.001-ST")
	require.NoError(t, err)

	// Cascade from parent
	record, err := core.ArchiveItem(ctx, ws.DB, ws, "010-F", core.WithCascade(true))

	require.NoError(t, err)
	// The subtask was already archived, so it should be skipped (not in cascaded or failed)
	assert.NotContains(t, record.CascadedItems, "010.001.001-ST",
		"already-archived subtask should be skipped, not re-archived")
	assert.Empty(t, record.FailedItems)
}

func TestArchiveItem_ArchivesLinkedStashEntries(t *testing.T) {
	ws := setupArchiveCascadeWorkspace(t)
	ctx := context.Background()

	// Create a stash file with an entry linked to the feature.
	stashFile := filepath.Join(ws.RootPath, ".backlogit", "stash.jsonl")
	stashContent := `{"id":"AABB1122","priority":"medium","kind":"task","text":"linked stash idea"}` + "\n"
	require.NoError(t, os.WriteFile(stashFile, []byte(stashContent), 0o644))

	// Index the stash entry and link it to "010-F".
	require.NoError(t, db.UpsertStashEntry(ctx, ws.DB, "AABB1122", "medium", "task", "linked stash idea", "", "active", "stash.jsonl", time.Now()))
	require.NoError(t, db.LinkStashEntry(ctx, ws.DB, "AABB1122", "010-F", time.Now()))

	// Archive the feature (no cascade needed — tests root-level link cleanup).
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "010-F")
	require.NoError(t, err)

	// The stash entry should now be removed from the stash file.
	remaining, err := readStashJSONL(stashFile)
	require.NoError(t, err)
	assert.Empty(t, remaining, "linked stash entry should be removed from stash file")

	// Verify the stash archive file was written.
	stashArchive := filepath.Join(ws.RootPath, ".backlogit", "archive", "stash.jsonl")
	assert.FileExists(t, stashArchive, "stash archive should be created")
}

func TestArchiveItem_NoStashLinkIsNoOp(t *testing.T) {
	ws := setupArchiveCascadeWorkspace(t)
	ctx := context.Background()

	// Create a stash file with an unlinked entry.
	stashFile := filepath.Join(ws.RootPath, ".backlogit", "stash.jsonl")
	stashContent := `{"id":"CCDD3344","priority":"low","kind":"bug","text":"unrelated idea"}` + "\n"
	require.NoError(t, os.WriteFile(stashFile, []byte(stashContent), 0o644))
	require.NoError(t, db.UpsertStashEntry(ctx, ws.DB, "CCDD3344", "low", "bug", "unrelated idea", "", "active", "stash.jsonl", time.Now()))

	// Archive a feature with no stash links.
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "010-F")
	require.NoError(t, err)

	// The unlinked stash entry should remain.
	remaining, err := readStashJSONL(stashFile)
	require.NoError(t, err)
	assert.Len(t, remaining, 1, "unlinked stash entry should remain")
}

// readStashJSONL reads stash entries from a JSONL file (test helper).
func readStashJSONL(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			return nil, err
		}
		entries = append(entries, m)
	}
	return entries, nil
}
