package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/backlogit/backlogit/internal/models"
)

// Rehydrate walks the workspace directory tree and rebuilds the SQLite index
// from the Markdown source files. Files that fail to parse are skipped with a
// debug log entry. Returns the number of artifacts successfully indexed.
func Rehydrate(ctx context.Context, workspacePath string, db *sql.DB) (int, error) {
	count := 0

	err := filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Debug("walk error, skipping", "path", path, "error", walkErr)
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		artifact, parseErr := parseMarkdownArtifact(path)
		if parseErr != nil {
			slog.Debug("skipping unparseable file", "path", path, "error", parseErr)
			return nil
		}
		if artifact == nil {
			return nil
		}

		if upsertErr := UpsertItem(ctx, db, artifact); upsertErr != nil {
			slog.Warn("failed to upsert artifact", "path", path, "error", upsertErr)
			return nil
		}

		// Upsert dependency edges from frontmatter.
		if len(artifact.Dependencies) > 0 {
			for _, depID := range artifact.Dependencies {
				if depID == "" {
					continue
				}
				// Best-effort: skip if target doesn't exist yet (will be linked on subsequent rehydration).
				_ = upsertDependencyBestEffort(ctx, db, artifact.ID, depID)
			}
		}

		count++
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("rehydrate walk: %w", err)
	}

	return count, nil
}

// parseMarkdownArtifact reads a Markdown file and extracts the artifact using
// the models layer directly, avoiding an import of the parser package (which
// would create an import cycle through core).
func parseMarkdownArtifact(path string) (*models.Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	fm, body, err := models.ParseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
	}
	if fm == nil {
		return nil, nil
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err != nil {
		return nil, fmt.Errorf("artifact from frontmatter %s: %w", path, err)
	}

	return artifact, nil
}

func upsertDependencyBestEffort(ctx context.Context, db *sql.DB, itemID, dependsOn string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, 'blocks')`,
		itemID, dependsOn,
	)
	return err
}
