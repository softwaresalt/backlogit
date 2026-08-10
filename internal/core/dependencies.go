package core

import (
	"context"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// AddDependency adds a dependency edge from itemID to dependsOn and persists it
// to the source artifact's Markdown frontmatter, which is the source of truth.
//
// The edge is first inserted into the SQLite cache via db.AddDependencyChecked,
// which validates that both items exist and rejects cycles. It is then written
// to the artifact's frontmatter dependencies list as a DependencyEdge so the
// edge — including its dep_type — survives an index rebuild (sync_index /
// Rehydrate clears item_deps and repopulates it from frontmatter). If the
// frontmatter write fails, the cache insert is rolled back to keep the two
// representations consistent.
func AddDependency(ctx context.Context, ws *Workspace, itemID, dependsOn, depType string) error {
	if depType == "" {
		depType = "blocks"
	}
	if !blerrors.ValidDependencyType(depType) {
		return fmt.Errorf("add dependency %s->%s: %w: %q", itemID, dependsOn, blerrors.ErrInvalidDependencyType, depType)
	}
	artifact, err := findArtifact(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("load source artifact %s: %w", itemID, err)
	}
	originalArtifact := cloneArtifact(artifact)

	for i, dep := range artifact.Dependencies {
		if dep.ID == dependsOn {
			if dep.Type == depType || (dep.Type == "" && depType == "blocks") {
				if err := db.AddDependencyChecked(ctx, ws.DB, itemID, dependsOn, depType); err != nil {
					return fmt.Errorf("add dependency %s->%s: %w", itemID, dependsOn, err)
				}
				return nil
			}
			// Edge exists with a different type: update frontmatter to match the new type.
			artifact.Dependencies[i].Type = depType
			return addDependencyWithEnvelope(ctx, ws, itemID, dependsOn, depType, artifact, originalArtifact,
				fmt.Sprintf("persist dep type update %s->%s to frontmatter", itemID, dependsOn))
		}
	}

	artifact.Dependencies = append(artifact.Dependencies, models.DependencyEdge{ID: dependsOn, Type: depType})
	return addDependencyWithEnvelope(ctx, ws, itemID, dependsOn, depType, artifact, originalArtifact,
		fmt.Sprintf("persist dependency %s->%s to frontmatter", itemID, dependsOn))
}

func addDependencyWithEnvelope(
	ctx context.Context,
	ws *Workspace,
	itemID string,
	dependsOn string,
	depType string,
	artifact *models.Artifact,
	originalArtifact *models.Artifact,
	persistContext string,
) error {
	err := MutationEnvelope(ctx, []MutationStep{
		{
			Name: "item-deps-insert",
			Apply: func(ctx context.Context) error {
				if err := db.AddDependencyChecked(ctx, ws.DB, itemID, dependsOn, depType); err != nil {
					return fmt.Errorf("add dependency %s->%s: %w", itemID, dependsOn, err)
				}
				return nil
			},
			Compensate: func(ctx context.Context) error {
				if err := db.DeleteDependency(ctx, ws.DB, itemID, dependsOn); err != nil {
					return fmt.Errorf("remove dependency %s->%s from cache: %w", itemID, dependsOn, err)
				}
				return nil
			},
		},
		{
			Name: "frontmatter-update",
			Apply: func(ctx context.Context) error {
				if err := persistArtifact(ctx, ws, artifact, false); err != nil {
					if blerrors.IsWriteIndeterminate(err) {
						if upsertErr := db.UpsertItem(ctx, ws.DB, artifact); upsertErr != nil {
							err = fmt.Errorf("%w; also items row upsert failed: %w", err, upsertErr)
						}
					}
					return fmt.Errorf("%s: %w", persistContext, err)
				}
				return nil
			},
			Compensate: func(ctx context.Context) error {
				if err := persistArtifact(ctx, ws, cloneArtifact(originalArtifact), false); err != nil {
					return fmt.Errorf("restore dependency frontmatter %s->%s: %w", itemID, dependsOn, err)
				}
				return nil
			},
		},
	})
	if err != nil {
		return fmt.Errorf("add dependency %s->%s: %w", itemID, dependsOn, err)
	}
	return nil
}

// RemoveDependency removes the dependency edge from itemID to dependsOn from
// both the SQLite cache and the source artifact's Markdown frontmatter so the
// edge does not reappear on the next index rebuild.
//
// The dep_type for rollback re-insertion is read from frontmatter (the
// DependencyEdge stored in the artifact) rather than from the SQLite cache.
// Both rollback branches, including the ErrWriteIndeterminate special case,
// keep their current structure.
func RemoveDependency(ctx context.Context, ws *Workspace, itemID, dependsOn string) error {
	// Load the artifact first so we can read the dep_type from frontmatter for
	// a potential rollback re-insert.
	artifact, err := findArtifact(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("load source artifact %s: %w", itemID, err)
	}

	// Locate the edge in frontmatter to capture its type for rollback.
	frontmatterType := "blocks"
	edgeInFrontmatter := false
	for _, dep := range artifact.Dependencies {
		if dep.ID == dependsOn {
			frontmatterType = dep.Type
			if frontmatterType == "" {
				frontmatterType = "blocks"
			}
			edgeInFrontmatter = true
			break
		}
	}

	// Also check whether the edge exists in the cache (for rollback eligibility).
	_, edgeInCache, cacheErr := lookupDependencyType(ctx, ws, itemID, dependsOn)
	if cacheErr != nil {
		return cacheErr
	}

	restoreCacheEdge := func() {
		if edgeInCache {
			_ = db.UpsertDependency(ctx, ws.DB, itemID, dependsOn, frontmatterType)
		}
	}

	if err := db.DeleteDependency(ctx, ws.DB, itemID, dependsOn); err != nil {
		return fmt.Errorf("remove dependency %s->%s: %w", itemID, dependsOn, err)
	}

	if !edgeInFrontmatter {
		return nil
	}

	filtered := make([]models.DependencyEdge, 0, len(artifact.Dependencies))
	removed := false
	for _, dep := range artifact.Dependencies {
		if dep.ID == dependsOn {
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

// AddShipmentBlock adds a shipment-to-shipment execution-blocking edge in the
// direction dependent → prerequisite ("prerequisite must ship before dependent").
//
// Both endpoints must resolve to artifacts of type "shipment". This guard is
// additive: the generic AddDependency path is unchanged and still accepts
// previously-valid non-shipment and mixed-type edges. The new additive guard
// lives only on this path.
//
// The edge is written via AddDependency with dep_type "blocks" so it persists
// to the source artifact's frontmatter and survives sync_index / Rehydrate.
// The reload + ErrWriteIndeterminate two-class contract from AddDependency applies.
func AddShipmentBlock(ctx context.Context, ws *Workspace, dependentID, prerequisiteID string) error {
	dependent, err := findArtifact(ctx, ws, dependentID)
	if err != nil {
		return fmt.Errorf("add shipment block: load dependent %s: %w", dependentID, err)
	}
	if dependent.ArtifactType != "shipment" {
		return fmt.Errorf("add shipment block: dependent %s has type %q; both endpoints must be shipments",
			dependentID, dependent.ArtifactType)
	}

	prerequisite, err := findArtifact(ctx, ws, prerequisiteID)
	if err != nil {
		return fmt.Errorf("add shipment block: load prerequisite %s: %w", prerequisiteID, err)
	}
	if prerequisite.ArtifactType != "shipment" {
		return fmt.Errorf("add shipment block: prerequisite %s has type %q; both endpoints must be shipments",
			prerequisiteID, prerequisite.ArtifactType)
	}

	return AddDependency(ctx, ws, dependentID, prerequisiteID, "blocks")
}

// lookupDependencyType returns the dep_type for the edge itemID→dependsOn from
// the SQLite cache, and whether the edge currently exists in the cache. Defaults
// to "blocks" when the stored dep_type is empty.
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
