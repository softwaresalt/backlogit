package cli_test

// 153.006-T / S1 U6 — CLI coverage: resolve on already-abandoned checkpoint
// (DBBA62AA, test-only).
//
// This unit adds CLI coverage for `backlogit checkpoint resolve` on an
// already-abandoned checkpoint document. The ResolveCheckpoint production
// function already refuses abandoned checkpoints with
// ErrCheckpointCannotResolveAbandoned; this test pins that the CLI surfaces
// the refusal with a non-zero exit and a message that identifies the
// abandonment reason. No production delta.

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU6_S1_ResolveCLI_AlreadyAbandonedExitsNonZero (153.006-T / S1 U6)
// asserts that `backlogit checkpoint resolve` on an already-abandoned
// checkpoint exits non-zero. The abandonment disposition is terminal; resolve
// must not silently flip it back to "resolved".
func TestU6_S1_ResolveCLI_AlreadyAbandonedExitsNonZero(t *testing.T) {
	root := setupCLIWorkspace(t)

	// Write a checkpoint that already carries an abandoned disposition.
	// The file is pre-populated with all required disposition fields so the
	// parser reads it as a fully abandoned document.
	writeCLICheckpoint(t, root, "checkpoint-u6-s1-abandoned.json",
		`{"schema_version":1,"agent":"ship","session_id":"u6-s1-test","phase":"build",`+
			`"status":"abandoned","disposition":"abandoned",`+
			`"disposition_reason":"superseded by newer session","disposition_operator":"operator@example.com",`+
			`"disposition_at":"2026-09-03T00:00:00Z",`+
			`"created_at":"2026-09-03T00:00:00Z","updated_at":"2026-09-03T00:00:00Z"}`)

	err := runCLIErr(t, root, "checkpoint", "resolve", "checkpoint-u6-s1-abandoned.json")

	require.Error(t, err,
		"resolving an already-abandoned checkpoint must exit non-zero")
	assert.Contains(t, err.Error(), "abandoned",
		"the error message must name the abandoned state")
}

// TestU6_S1_ResolveCLI_AbandonedFileIsUnchanged asserts the file bytes are
// unchanged after the refused resolve (the disposition terminal state is
// preserved).
func TestU6_S1_ResolveCLI_AbandonedFileIsUnchanged(t *testing.T) {
	root := setupCLIWorkspace(t)
	body := `{"schema_version":1,"agent":"ship","session_id":"u6-s1-bytes","phase":"build",` +
		`"status":"abandoned","disposition":"abandoned",` +
		`"disposition_reason":"stale","disposition_operator":"op@example.com",` +
		`"disposition_at":"2026-09-03T00:00:00Z",` +
		`"created_at":"2026-09-03T00:00:00Z","updated_at":"2026-09-03T00:00:00Z"}`

	cpPath := writeCLICheckpoint(t, root, "checkpoint-u6-s1-bytes.json", body)

	_ = runCLIErr(t, root, "checkpoint", "resolve", "checkpoint-u6-s1-bytes.json")

	// The file must exist with its original content unchanged.
	after, readErr := os.ReadFile(cpPath)
	require.NoError(t, readErr, "file must still exist after a refused resolve")
	assert.Equal(t, []byte(body), after, "file bytes must be unchanged after refusal")
}
