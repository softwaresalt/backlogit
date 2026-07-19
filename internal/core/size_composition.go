package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/softwaresalt/backlogit/internal/models"
)

// SizeCompositionMember describes one canonical member included in a size rollup.
type SizeCompositionMember struct {
	ID           string `json:"id"`
	ArtifactType string `json:"artifact_type"`
	Size         string `json:"size,omitempty"`
}

// SizeCompositionResult is the computed-on-read size rollup for a feature or shipment.
type SizeCompositionResult struct {
	Histogram      map[string]int          `json:"histogram"`
	Unsized        int                     `json:"unsized"`
	Members        []SizeCompositionMember `json:"members"`
	Skipped        []string                `json:"skipped,omitempty"`
	RulesetVersion *string                 `json:"ruleset_version"`
}

// SizeComposition computes the never-persisted size rollup for a feature or
// shipment (108-F SE-4). Feature membership is direct children by parent_id;
// shipment membership is the explicit custom_fields.items manifest with each
// feature member expanded into its child tasks. Members are de-duplicated so a
// manifest listing a feature and its explicit child tasks counts each once. An
// existing member with no size increments Unsized; an unresolved manifest id is
// warn-skipped (counted in neither Histogram nor Unsized). The result is computed
// on read and never written to disk. ruleset_version is always null until a
// canonical ruleset is owned.
func SizeComposition(ctx context.Context, ws *Workspace, artifact *models.Artifact) (*SizeCompositionResult, error) {
	if artifact == nil {
		return nil, fmt.Errorf("size composition: artifact is required")
	}
	result := &SizeCompositionResult{
		Histogram: map[string]int{},
		Members:   []SizeCompositionMember{},
		Skipped:   []string{},
	}

	memberIDs, err := compositionMemberIDs(ctx, ws, artifact)
	if err != nil {
		return nil, err
	}
	for _, id := range uniqueNonEmptyStrings(memberIDs) {
		member, ferr := findArtifact(ctx, ws, id)
		if ferr != nil {
			// Unresolved manifest id (ErrNotFound) or any other resolution error:
			// warn + skip; not counted in the histogram or unsized.
			slog.WarnContext(ctx, "size composition: skipping unresolved member", "member_id", id, "error", ferr)
			result.Skipped = append(result.Skipped, id)
			continue
		}
		size, _ := member.CustomFields["size"].(string)
		result.Members = append(result.Members, SizeCompositionMember{
			ID:           member.ID,
			ArtifactType: member.ArtifactType,
			Size:         size,
		})
		if size == "" {
			result.Unsized++
		} else {
			result.Histogram[size]++
		}
	}
	return result, nil
}

// compositionMemberIDs resolves the canonical member IDs for a size rollup. For a
// feature this is its direct children by parent_id; for a shipment it is the
// explicit manifest with each feature member expanded into its child tasks.
func compositionMemberIDs(ctx context.Context, ws *Workspace, artifact *models.Artifact) ([]string, error) {
	switch artifact.ArtifactType {
	case "feature":
		return childIDsByParent(ctx, ws, artifact.ID)
	case "shipment":
		var ids []string
		for _, memberID := range NormalizeShipmentItems(artifact) {
			ids = append(ids, memberID)
			member, ferr := findArtifact(ctx, ws, memberID)
			if ferr != nil {
				// Dangling manifest id; the main loop records it as skipped.
				continue
			}
			if member.ArtifactType == "feature" {
				childIDs, cerr := childIDsByParent(ctx, ws, memberID)
				if cerr != nil {
					return nil, cerr
				}
				ids = append(ids, childIDs...)
			}
		}
		return ids, nil
	default:
		return nil, fmt.Errorf("size composition: unsupported artifact type %q", artifact.ArtifactType)
	}
}

// childIDsByParent returns the IDs of the direct children of parentID from the
// SQLite index.
func childIDsByParent(ctx context.Context, ws *Workspace, parentID string) ([]string, error) {
	if ws == nil || ws.DB == nil {
		return nil, nil
	}
	rows, err := ws.DB.QueryContext(ctx, `SELECT id FROM items WHERE parent_id = ?`, parentID)
	if err != nil {
		return nil, fmt.Errorf("query children of %s: %w", parentID, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan child id of %s: %w", parentID, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate children of %s: %w", parentID, err)
	}
	return ids, nil
}
