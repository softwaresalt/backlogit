package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func durableTestArtifact() models.Artifact {
	return models.Artifact{
		ID:           "920.001-T",
		Title:        "Durable write task",
		ArtifactType: "task",
		ParentID:     "920-F",
		Status:       models.StatusQueued,
		CreatedAt:    models.NowUTC(),
		UpdatedAt:    models.NowUTC(),
	}
}

// TestWriteArtifactFileWithOptions_DurableWritesContentAtomically asserts the
// durable-on rewrite persists the content and leaves no temp residue.
func TestWriteArtifactFileWithOptions_DurableWritesContentAtomically(t *testing.T) {
	t.Parallel()
	artifact := durableTestArtifact()
	dir := t.TempDir()
	path := filepath.Join(dir, "920.001-T.md")

	require.NoError(t, core.WriteArtifactFileWithOptions(&artifact, path, true))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fmOut, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	assert.Equal(t, "queued", fmOut["status"])

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temp residue must remain after a durable rewrite")
}

// TestWriteArtifactFile_WrapperDurableOffUnchanged asserts the back-compat
// wrapper still writes content correctly (durable off).
func TestWriteArtifactFile_WrapperDurableOffUnchanged(t *testing.T) {
	t.Parallel()
	artifact := durableTestArtifact()
	path := filepath.Join(t.TempDir(), "920.001-T.md")

	require.NoError(t, core.WriteArtifactFile(&artifact, path))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "status: queued")
}

// TestWriteArtifactFile_ProvenanceGuardSharedByBothEntrypoints asserts the
// archive-provenance guard rejects an archived artifact missing provenance
// through BOTH the wrapper and the options variant.
func TestWriteArtifactFile_ProvenanceGuardSharedByBothEntrypoints(t *testing.T) {
	t.Parallel()
	artifact := durableTestArtifact()
	artifact.Status = models.StatusArchived // archived but no archived_from/archived_status
	dir := t.TempDir()

	t.Run("wrapper", func(t *testing.T) {
		err := core.WriteArtifactFile(&artifact, filepath.Join(dir, "w.md"))
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrValidation)
	})
	t.Run("with-options-durable", func(t *testing.T) {
		err := core.WriteArtifactFileWithOptions(&artifact, filepath.Join(dir, "o.md"), true)
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrValidation)
	})
}

// TestWriteArtifactFileWithOptions_NotAppliedOnPreRenameFailure asserts a
// failure before the rename commits (an unwritable/absent parent directory)
// propagates the U2 ErrWriteNotApplied class to the caller.
func TestWriteArtifactFileWithOptions_NotAppliedOnPreRenameFailure(t *testing.T) {
	t.Parallel()
	artifact := durableTestArtifact()
	// A path under a non-existent directory makes the temp-file create fail,
	// which is a pre-rename failure classified not-applied.
	path := filepath.Join(t.TempDir(), "missing-subdir", "920.001-T.md")

	err := core.WriteArtifactFileWithOptions(&artifact, path, true)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteNotApplied(err),
		"a pre-rename failure must propagate ErrWriteNotApplied to the caller")
}
