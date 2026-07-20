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
// shipment (108-F SE-4). Size estimation is task-only, so the rollup counts only
// TASK members: feature membership is its direct task children by parent_id, and
// shipment membership is the explicit custom_fields.items manifest with each
// feature member expanded into its child tasks (the feature itself, a rollup
// parent, is never counted). Members are de-duplicated so a manifest listing a
// feature and its explicit child tasks counts each once. An existing task member
// with no size increments Unsized; an unresolved manifest id is warn-skipped
// (counted in neither Histogram nor Unsized). The result is computed on read and
// never written to disk. ruleset_version is always null until a canonical ruleset
// is owned.
func SizeComposition(ctx context.Context, ws *Workspace, artifact *models.Artifact) (*SizeCompositionResult, error) {
	if artifact == nil {
		return nil, fmt.Errorf("size composition: artifact is required")
	}
	if ws == nil {
		return nil, fmt.Errorf("size composition: workspace is required")
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

// compositionMemberIDs resolves the canonical member IDs for a size rollup. Size
// estimation is task-only, so this yields only task members: for a feature, its
// direct task children by parent_id; for a shipment, the explicit manifest with
// directly-listed tasks kept and each feature member expanded into its child
// tasks (rollup-parent types such as the feature itself are excluded).
func compositionMemberIDs(ctx context.Context, ws *Workspace, artifact *models.Artifact) ([]string, error) {
	switch artifact.ArtifactType {
	case "feature":
		return childIDsByParent(ctx, ws, artifact.ID)
	case "shipment":
		var ids []string
		for _, memberID := range NormalizeShipmentItems(artifact) {
			member, ferr := findArtifact(ctx, ws, memberID)
			if ferr != nil {
				// Dangling manifest id: keep it so the main loop records it as
				// skipped (warn + counted in neither histogram nor unsized).
				ids = append(ids, memberID)
				continue
			}
			switch member.ArtifactType {
			case "task":
				// A directly-listed task is a sizable member.
				ids = append(ids, memberID)
			case "feature":
				// A feature is a rollup parent, not a sizable unit: expand it into
				// its child tasks and do NOT count the feature itself.
				childIDs, cerr := childIDsByParent(ctx, ws, memberID)
				if cerr != nil {
					return nil, cerr
				}
				ids = append(ids, childIDs...)
			default:
				// Any other manifest member type (subtask, review, ...) is not a
				// sizable unit and is excluded from the rollup.
			}
		}
		return ids, nil
	default:
		return nil, fmt.Errorf("size composition: unsupported artifact type %q", artifact.ArtifactType)
	}
}

// childIDsByParent returns the IDs of the direct TASK children (the only sizable
// unit) of parentID from the SQLite index. Non-task children (e.g. review) are
// excluded so they never skew the rollup as spurious unsized members.
func childIDsByParent(ctx context.Context, ws *Workspace, parentID string) ([]string, error) {
	if ws == nil || ws.DB == nil {
		return nil, nil
	}
	rows, err := ws.DB.QueryContext(ctx, `SELECT id FROM items WHERE parent_id = ? AND artifact_type = 'task'`, parentID)
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
