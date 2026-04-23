package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/softwaresalt/backlogit/internal/db"
)

// DeleteArtifact removes an artifact from both the filesystem and the SQLite
// index in a crash-safe order. The file is renamed to a temporary path (with
// ".deleting" inserted before the ".md" extension) before the DB delete, so
// that a crash between the rename and the DB operation leaves the artifact
// discoverable via rehydration rather than invisible to artifact scanning.
// If the DB delete fails, the renamed file is restored to its original path.
//
// Callers that previously inlined db.DeleteItemCascade + os.Remove should
// migrate to this function to gain crash-safety guarantees.
func DeleteArtifact(ctx context.Context, ws *Workspace, id string) error {
	if ws == nil || ws.DB == nil {
		return fmt.Errorf("delete artifact: workspace or database is not initialized")
	}

	filePath, err := FindArtifactPath(ctx, ws, id)
	if err != nil {
		return fmt.Errorf("find artifact: %w", err)
	}

	tempPath := strings.TrimSuffix(filePath, ".md") + ".deleting.md"
	if err := os.Rename(filePath, tempPath); err != nil {
		return fmt.Errorf("prepare delete: %w", err)
	}

	if err := db.DeleteItemCascade(ctx, ws.DB, id); err != nil {
		// Restore file so the workspace remains consistent.
		if restoreErr := os.Rename(tempPath, filePath); restoreErr != nil {
			return fmt.Errorf("delete from index: %w (restore failed: %v)", err, restoreErr)
		}
		return fmt.Errorf("delete from index: %w", err)
	}

	if err := os.Remove(tempPath); err != nil {
		// The DB delete already succeeded; the artifact is no longer tracked.
		// Best-effort: rename temp back to its original path so it remains
		// discoverable and can be cleaned up by the next rehydration or sync.
		if renameErr := os.Rename(tempPath, filePath); renameErr != nil {
			slog.Error("delete artifact: DB deleted but temp file stuck; workspace may be inconsistent",
				"temp_path", tempPath, "original_path", filePath, "error", err, "rename_error", renameErr)
		}
		return fmt.Errorf("remove artifact file: %w", err)
	}

	return nil
}
