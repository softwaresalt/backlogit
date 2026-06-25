package db_test

// 066.004-T (Unit 4) harness: Rehydrate warns (once) when two source files map
// to the same ID, without altering the clear+batch-insert rebuild or the
// collapse result. RED until Rehydrate emits the duplicate-source warning.

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

const dupWarnMarker = "duplicate source id"

func writeRehydrateArtifact(t *testing.T, root, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, dir, name), []byte(body), 0o644))
}

func TestRehydrate_WarnsOnDuplicateSourceIDs(t *testing.T) {
	ws := t.TempDir()

	writeRehydrateArtifact(t, ws, "queue", "066-F.md", `---
id: "066-F"
title: "Queued feature"
status: active
artifact_type: feature
---
Queued body.
`)
	writeRehydrateArtifact(t, ws, "archive", "066-F.md", `---
id: "066-F"
title: "Archived feature"
status: archived
artifact_type: feature
---
Archived body.
`)

	// Capture the default slog output for the duration of the rehydrate.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	logged := buf.String()

	// Exactly one warning per duplicated ID (no spam).
	assert.Equal(t, 1, strings.Count(logged, dupWarnMarker),
		"expected exactly one duplicate-source warning; got log:\n%s", logged)
	assert.Contains(t, logged, "066-F", "warning must name the duplicated id")
	// Warning must list all conflicting source paths.
	assert.Contains(t, logged, "queue", "warning must reference the queue source path")
	assert.Contains(t, logged, "archive", "warning must reference the archive source path")

	// Collapse semantics are unchanged: exactly one indexed row survives.
	var rows int
	require.NoError(t, database.QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE id = ?", "066-F").Scan(&rows))
	assert.Equal(t, 1, rows, "duplicate source files must still collapse to one indexed row")
}

func TestRehydrate_NoWarningForUniqueIDs(t *testing.T) {
	ws := t.TempDir()
	writeRehydrateArtifact(t, ws, "queue", "066-F.md", `---
id: "066-F"
title: "Unique feature"
status: active
artifact_type: feature
---
Body.
`)
	writeRehydrateArtifact(t, ws, "queue", "067-F.md", `---
id: "067-F"
title: "Another unique feature"
status: active
artifact_type: feature
---
Body.
`)

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	database := setupTestDB(t)
	ctx := context.Background()
	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	assert.Equal(t, 0, strings.Count(buf.String(), dupWarnMarker),
		"no duplicate-source warning expected when all IDs are unique")
}
