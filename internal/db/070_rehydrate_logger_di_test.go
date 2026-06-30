package db_test

// 070.002-T harness: Rehydrate dependency-injects a *slog.Logger so tests can
// capture the duplicate-source warning WITHOUT mutating the global slog default
// via slog.SetDefault. RED until db.WithLogger is honored by Rehydrate and
// warnOnDuplicateSourceIDs.

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

// TestRehydrate_WithInjectedLogger_CapturesDuplicateWarning proves the
// duplicate-source warning is routed to the injected logger. The test never
// touches the global slog default; if Rehydrate fell back to it, the injected
// buffer would stay empty and this test would fail.
func TestRehydrate_WithInjectedLogger_CapturesDuplicateWarning(t *testing.T) {
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

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database, db.WithLogger(logger))
	require.NoError(t, err)

	logged := buf.String()
	assert.Equal(t, 1, strings.Count(logged, dupWarnMarker),
		"duplicate-source warning must be written to the injected logger; got:\n%s", logged)
	assert.Contains(t, logged, "066-F", "warning must name the duplicated id")
	assert.Contains(t, logged, "queue", "warning must reference the queue source path")
	assert.Contains(t, logged, "archive", "warning must reference the archive source path")
}

// TestRehydrate_NoLogger_DefaultsToGlobal verifies the no-logger path is
// preserved: when no logger is injected, Rehydrate logs to slog.Default(), so an
// existing slog.SetDefault-based capture still works (behavior unchanged).
func TestRehydrate_NoLogger_DefaultsToGlobal(t *testing.T) {
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

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(buf.String(), dupWarnMarker),
		"with no injected logger Rehydrate must fall back to slog.Default()")
}
