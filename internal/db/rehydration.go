package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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

		// Derive level and hierarchy_path from the ID for hierarchical IDs
		// (segments separated by "."), e.g. "001" → level=1, "001.002" → level=2.
		// Only update when the artifact does not already carry these values.
		if artifact.Level == 0 && isHierarchicalID(artifact.ID) {
			level := strings.Count(artifact.ID, ".") + 1
			hierarchyPath := hierarchyPathFromID(artifact.ID)
			if _, execErr := db.ExecContext(ctx,
				`UPDATE items SET level = ?, hierarchy_path = ? WHERE id = ?`,
				level, hierarchyPath, artifact.ID,
			); execErr != nil {
				slog.Warn("failed to set level/hierarchy_path", "id", artifact.ID, "error", execErr)
			}
		}

		// Upsert dependency edges from frontmatter.
		if len(artifact.Dependencies) > 0 {
			for _, depID := range artifact.Dependencies {
				if depID == "" {
					continue
				}
				// Best-effort: skip if target doesn't exist yet (will be linked on subsequent rehydration).
				if depErr := upsertDependencyBestEffort(ctx, db, artifact.ID, depID); depErr != nil {
					slog.Warn("failed to upsert dependency", "item_id", artifact.ID, "dep_id", depID, "error", depErr)
				}
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

// isHierarchicalID reports whether an ID uses the dot-separated numeric hierarchy
// format (e.g., "001", "001.002", "001.002.003"). Non-hierarchical IDs such as
// "T001" or "BUG-3" are excluded.
func isHierarchicalID(id string) bool {
	if id == "" {
		return false
	}
	for _, seg := range strings.Split(id, ".") {
		if seg == "" {
			return false
		}
		for _, ch := range seg {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}

// hierarchyPathFromID builds the ancestor path for a hierarchical ID.
// "001.002.003" → "001/001.002/001.002.003".
func hierarchyPathFromID(id string) string {
	parts := strings.Split(id, ".")
	segments := make([]string, len(parts))
	for i := range parts {
		segments[i] = strings.Join(parts[:i+1], ".")
	}
	return strings.Join(segments, "/")
}
