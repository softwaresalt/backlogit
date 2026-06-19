package core

import (
	"context"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/db"
)

// AddDependency adds a dependency edge from itemID to dependsOn and persists it
// to the source artifact's Markdown frontmatter, which is the source of truth.
//
// The edge is first inserted into the SQLite cache via db.AddDependencyChecked,
// which validates that both items exist and rejects cycles. It is then written
// to the artifact's frontmatter dependencies list so the edge survives an index
// rebuild (sync_index / Rehydrate clears item_deps and repopulates it solely
// from frontmatter). If the frontmatter write fails, the cache insert is rolled
// back to keep the two representations consistent.
func AddDependency(ctx context.Context, ws *Workspace, itemID, dependsOn, depType string) error {
	if depType == "" {
		depType = "blocks"
	}
	if err := db.AddDependencyChecked(ctx, ws.DB, itemID, dependsOn, depType); err != nil {
		return err
	}

	artifact, err := findArtifact(ctx, ws, itemID)
	if err != nil {
		_ = db.DeleteDependency(ctx, ws.DB, itemID, dependsOn)
		return fmt.Errorf("load source artifact %s: %w", itemID, err)
	}

	for _, dep := range artifact.Dependencies {
		if dep == dependsOn {
			// Already recorded in frontmatter; cache insert above is sufficient.
			return nil
		}
	}

	artifact.Dependencies = append(artifact.Dependencies, dependsOn)
	if err := persistArtifact(ctx, ws, artifact, false); err != nil {
		_ = db.DeleteDependency(ctx, ws.DB, itemID, dependsOn)
		return fmt.Errorf("persist dependency %s->%s to frontmatter: %w", itemID, dependsOn, err)
	}
	return nil
}

// RemoveDependency removes the dependency edge from itemID to dependsOn from
// both the SQLite cache and the source artifact's Markdown frontmatter so the
// edge does not reappear on the next index rebuild.
func RemoveDependency(ctx context.Context, ws *Workspace, itemID, dependsOn string) error {
	if err := db.DeleteDependency(ctx, ws.DB, itemID, dependsOn); err != nil {
		return err
	}

	artifact, err := findArtifact(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("load source artifact %s: %w", itemID, err)
	}

	filtered := make([]string, 0, len(artifact.Dependencies))
	removed := false
	for _, dep := range artifact.Dependencies {
		if dep == dependsOn {
			removed = true
			continue
		}
		filtered = append(filtered, dep)
	}
	if !removed {
		return nil
	}

	artifact.Dependencies = filtered
	if err := persistArtifact(ctx, ws, artifact, false); err != nil {
		return fmt.Errorf("persist dependency removal %s->%s to frontmatter: %w", itemID, dependsOn, err)
	}
	return nil
}
