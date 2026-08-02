package core

import (
	"context"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
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
//
// Note: only the edge itself is durable. The frontmatter dependencies list
// records target IDs without a dep_type, so a non-"blocks" depType is honored
// only in the live cache until the next Rehydrate, which rebuilds every edge as
// "blocks". Callers must not rely on a custom depType surviving sync_index.
func AddDependency(ctx context.Context, ws *Workspace, itemID, dependsOn, depType string) error {
	if depType == "" {
		depType = "blocks"
	}
	if err := db.AddDependencyChecked(ctx, ws.DB, itemID, dependsOn, depType); err != nil {
		return fmt.Errorf("add dependency %s->%s: %w", itemID, dependsOn, err)
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
		// ErrWriteIndeterminate: the MD write committed (rename applied) but fsync
		// failed — the dependency is likely persisted. Do NOT roll back the DB edge;
		// rolling it back would diverge the index from the (likely-written) MD.
		// Reconcile the stale items.dependencies column so DB-fast-path mutations
		// see the correct dependency list; join any upsert error with the durability error.
		if !blerrors.IsWriteIndeterminate(err) {
			_ = db.DeleteDependency(ctx, ws.DB, itemID, dependsOn)
		} else {
			if upsertErr := db.UpsertItem(ctx, ws.DB, artifact); upsertErr != nil {
				err = fmt.Errorf("%w; also items row upsert failed: %w", err, upsertErr)
			}
		}
		return fmt.Errorf("persist dependency %s->%s to frontmatter: %w", itemID, dependsOn, err)
	}
	return nil
}

// RemoveDependency removes the dependency edge from itemID to dependsOn from
// both the SQLite cache and the source artifact's Markdown frontmatter so the
// edge does not reappear on the next index rebuild.
//
// The cache edge is deleted before the frontmatter is updated. If either the
// source artifact cannot be loaded or the frontmatter write fails, the cache
// deletion is rolled back with a best-effort re-insert of the original edge so
// the cache stays consistent with the frontmatter source of truth (otherwise a
// later Rehydrate would resurrect the edge from frontmatter, leaving the two
// representations out of sync until then).
func RemoveDependency(ctx context.Context, ws *Workspace, itemID, dependsOn string) error {
	// Capture the existing edge's dep_type so a rollback can restore it faithfully.
	depType, edgeInCache, err := lookupDependencyType(ctx, ws, itemID, dependsOn)
	if err != nil {
		return err
	}

	restoreCacheEdge := func() {
		if edgeInCache {
			_ = db.UpsertDependency(ctx, ws.DB, itemID, dependsOn, depType)
		}
	}

	if err := db.DeleteDependency(ctx, ws.DB, itemID, dependsOn); err != nil {
		return fmt.Errorf("remove dependency %s->%s: %w", itemID, dependsOn, err)
	}

	artifact, err := findArtifact(ctx, ws, itemID)
	if err != nil {
		restoreCacheEdge()
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
		// ErrWriteIndeterminate: the MD write committed but fsync failed — the
		// removal is likely persisted. Do NOT restore the cache edge; restoring it
		// would diverge the index from the (likely-updated) MD.
		// Reconcile the stale items.dependencies column so a subsequent Rehydrate
		// does not incorrectly restore the removed edge.
		if !blerrors.IsWriteIndeterminate(err) {
			restoreCacheEdge()
		} else {
			if upsertErr := db.UpsertItem(ctx, ws.DB, artifact); upsertErr != nil {
				err = fmt.Errorf("%w; also items row upsert failed: %w", err, upsertErr)
			}
		}
		return fmt.Errorf("persist dependency removal %s->%s to frontmatter: %w", itemID, dependsOn, err)
	}
	return nil
}

// AddShipmentBlock is the stub entry point for shipment-to-shipment blocking.
// Full implementation is in U4 (134.004-T). This stub exists only to make
// U3 test files compile; tests calling it will fail because no edge is created
// and no endpoint validation is performed.
func AddShipmentBlock(_ context.Context, _ *Workspace, _, _ string) error {
	return fmt.Errorf("AddShipmentBlock: not yet implemented (stub for U3 compilation)")
}

// whether the edge currently exists in the cache. Defaults to "blocks" when the
// stored dep_type is empty so a rollback re-insert preserves a valid type.
func lookupDependencyType(ctx context.Context, ws *Workspace, itemID, dependsOn string) (string, bool, error) {
	edges, err := db.GetDependencies(ctx, ws.DB, itemID)
	if err != nil {
		return "", false, fmt.Errorf("load dependencies for %s: %w", itemID, err)
	}
	for _, e := range edges {
		if e.DependsOn == dependsOn {
			depType := e.DepType
			if depType == "" {
				depType = "blocks"
			}
			return depType, true, nil
		}
	}
	return "", false, nil
}
