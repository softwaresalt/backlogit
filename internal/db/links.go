package db

import (
	"context"
	"database/sql"
	"fmt"

	corerrors "github.com/backlogit/backlogit/internal/errors"
)

// LinkEdge represents a directed semantic link between two backlogit artifacts.
type LinkEdge struct {
	SourceID  string `json:"source_id"`
	TargetID  string `json:"target_id"`
	LinkType  string `json:"link_type"`
	CreatedAt string `json:"created_at"`
}

// ValidLinkTypes lists every allowed value for the link_type column in item_links.
var ValidLinkTypes = []string{
	"related_to",
	"duplicate_of",
	"informs",
	"supersedes",
	"spike_ref",
}

// isValidLinkType reports whether lt is an element of ValidLinkTypes.
func isValidLinkType(lt string) bool {
	for _, v := range ValidLinkTypes {
		if v == lt {
			return true
		}
	}
	return false
}

// AddLink inserts a directed semantic link from sourceID to targetID.
// Returns an error when link_type is not in ValidLinkTypes. The insert is
// idempotent: duplicate (sourceID, targetID, linkType) tuples are silently
// ignored via INSERT OR IGNORE.
func AddLink(ctx context.Context, database *sql.DB, sourceID, targetID, linkType string) error {
	if !isValidLinkType(linkType) {
		return fmt.Errorf("%w: %q is not one of %v", corerrors.ErrInvalidLinkType, linkType, ValidLinkTypes)
	}
	_, err := database.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_links (source_id, target_id, link_type) VALUES (?, ?, ?)`,
		sourceID, targetID, linkType,
	)
	if err != nil {
		return fmt.Errorf("add link %s→%s (%s): %w", sourceID, targetID, linkType, err)
	}
	return nil
}

// RemoveLink deletes the directed link matching (sourceID, targetID, linkType).
// A no-op removal (link not found) is not an error.
func RemoveLink(ctx context.Context, database *sql.DB, sourceID, targetID, linkType string) error {
	_, err := database.ExecContext(ctx,
		`DELETE FROM item_links WHERE source_id = ? AND target_id = ? AND link_type = ?`,
		sourceID, targetID, linkType,
	)
	if err != nil {
		return fmt.Errorf("remove link %s→%s (%s): %w", sourceID, targetID, linkType, err)
	}
	return nil
}

// GetLinks returns all outgoing links from sourceID, regardless of type.
func GetLinks(ctx context.Context, database *sql.DB, sourceID string) ([]LinkEdge, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT source_id, target_id, link_type, created_at FROM item_links WHERE source_id = ?`,
		sourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("get links for %s: %w", sourceID, err)
	}
	defer rows.Close()
	return scanLinkEdges(rows)
}

// GetLinksByType returns all outgoing links from sourceID that match linkType.
func GetLinksByType(ctx context.Context, database *sql.DB, sourceID, linkType string) ([]LinkEdge, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT source_id, target_id, link_type, created_at FROM item_links WHERE source_id = ? AND link_type = ?`,
		sourceID, linkType,
	)
	if err != nil {
		return nil, fmt.Errorf("get links for %s by type %s: %w", sourceID, linkType, err)
	}
	defer rows.Close()
	return scanLinkEdges(rows)
}

// scanLinkEdges reads all rows into a LinkEdge slice.
func scanLinkEdges(rows *sql.Rows) ([]LinkEdge, error) {
	var edges []LinkEdge
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.SourceID, &e.TargetID, &e.LinkType, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan link edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}
