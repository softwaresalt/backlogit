package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCopilotReview_NewFileCustodyWriteRefused covers PR 641 review thread on
// internal/core/artifacts.go: when the destination does not exist, the public
// artifact writers skipped the custody guard and could persist a
// workflow_custody envelope without a governed handoff.
func TestCopilotReview_NewFileCustodyWriteRefused(t *testing.T) {
	newArtifact := func(fields map[string]any) *models.Artifact {
		return &models.Artifact{
			ID:           "211-S",
			ArtifactType: "shipment",
			Title:        "copilot review regression",
			Status:       models.StatusQueued,
			CustomFields: fields,
		}
	}

	t.Run("public_writer_refuses_new_file_carrying_custody_envelope", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "211-S.md")
		art := newArtifact(map[string]any{custodyArtifactKey: map[string]any{"schema_version": "workflow-custody/v1"}})

		err := WriteArtifactFileWithOptions(art, path, false)
		require.ErrorIs(t, err, ErrCustodyUngovernedMutation)
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "a refused custody write must not create the file")
	})

	t.Run("public_writer_refuses_new_file_via_write_artifact_file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "211-S.md")
		art := newArtifact(map[string]any{custodyArtifactKey: map[string]any{"schema_version": "workflow-custody/v1"}})

		err := WriteArtifactFile(art, path)
		require.ErrorIs(t, err, ErrCustodyUngovernedMutation)
	})

	t.Run("new_file_without_custody_envelope_still_writes", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "211-S.md")
		art := newArtifact(map[string]any{"priority": "high"})

		require.NoError(t, WriteArtifactFileWithOptions(art, path, false))
		_, err := os.Stat(path)
		require.NoError(t, err)
	})

	t.Run("governed_custody_write_of_new_file_is_still_permitted", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "211-S.md")
		art := newArtifact(map[string]any{custodyArtifactKey: map[string]any{"schema_version": "workflow-custody/v1"}})

		err := writeArtifactFileGoverned(art, path, path, false,
			artifactWriteEnvelope{allowCustodyMutation: true}, nil)
		require.NoError(t, err)
	})
}
