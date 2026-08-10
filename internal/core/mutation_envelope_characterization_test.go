package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

func associateCommitWithoutEnvelope(ctx context.Context, ws *core.Workspace, ew *events.EventWriter, itemID, sha, message, author string) error {
	if _, err := core.UpdateArtifact(ctx, ws, itemID, map[string]any{"commit": sha}); err != nil {
		return fmt.Errorf("associate commit step 1 (frontmatter scalar): %w", err)
	}
	if _, err := ws.DB.ExecContext(ctx,
		`INSERT INTO commit_links (item_id, commit_sha, message, author)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(item_id, commit_sha) DO UPDATE SET
		  message = CASE WHEN excluded.message != '' THEN excluded.message ELSE commit_links.message END,
		  author  = CASE WHEN excluded.author  != '' THEN excluded.author  ELSE commit_links.author  END`,
		itemID, sha, message, author,
	); err != nil {
		return fmt.Errorf("associate commit step 2 (commit_links upsert): %w", err)
	}
	if err := ew.AppendEvent(ctx, events.Event{
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: "commit_tracked",
		Delta: map[string]any{
			"commit_sha": sha,
			"message":    message,
			"author":     author,
		},
	}); err != nil {
		return fmt.Errorf("associate commit step 3 (JSONL append): %w", err)
	}
	return nil
}

func TestCharacterization_AssociateCommit_JSONLWriteFailure_SQLiteSucceeds(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "Commit feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Commit task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	logsPath := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(logsPath, []byte("blocking file"), 0o644))
	writer := events.NewEventWriter(logsPath)

	err = associateCommitWithoutEnvelope(ctx, ws, writer, artifact.ID, "abc123def", "feat: implement T001", "test@example.com")
	require.Error(t, err)

	var pathErr *os.PathError
	assert.ErrorAs(t, err, &pathErr, "pre-envelope failure should wrap the JSONL write error")

	links, getErr := core.GetCommitLinks(ctx, ws.DB, artifact.ID)
	require.NoError(t, getErr)
	require.Len(t, links, 1)
	assert.Equal(t, "abc123def", links[0].CommitSHA)

	item, itemErr := db.GetItem(ctx, ws.DB, artifact.ID)
	require.NoError(t, itemErr)
	assert.Equal(t, "abc123def", item.Commit)

	var partialErr *blerrors.MutationPartialError
	assert.False(t, errors.As(err, &partialErr), "current behavior should not surface a typed partial result")
}

func TestCharacterization_AssociateCommit_NoTypedPartialResult(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "Commit feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Commit task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	logsPath := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(logsPath, []byte("blocking file"), 0o644))
	writer := events.NewEventWriter(logsPath)

	err = associateCommitWithoutEnvelope(ctx, ws, writer, artifact.ID, "abc123def", "feat: implement T001", "test@example.com")
	require.Error(t, err)

	var partialErr *blerrors.MutationPartialError
	assert.False(t, errors.As(err, &partialErr))
}
