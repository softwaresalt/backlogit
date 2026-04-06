package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// StashRecord represents an indexed stash entry with any harvested link.
type StashRecord struct {
	ID             string     `json:"id"`
	Priority       string     `json:"priority"`
	DeliberationID string     `json:"deliberation_id,omitempty"`
	Kind           string     `json:"kind"`
	Text           string     `json:"text"`
	State          string     `json:"state"`
	SourcePath     string     `json:"source_path"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ItemID         string     `json:"item_id,omitempty"`
	LinkedAt       *time.Time `json:"linked_at,omitempty"`
}

// ClearStashIndex deletes stash entries and stash links before rehydration.
func ClearStashIndex(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `DELETE FROM stash_links`); err != nil {
		return fmt.Errorf("clear stash links: %w", err)
	}
	if _, err := database.ExecContext(ctx, `DELETE FROM stash_entries`); err != nil {
		return fmt.Errorf("clear stash entries: %w", err)
	}
	return nil
}

// UpsertStashEntry writes or updates a stash entry record.
func UpsertStashEntry(ctx context.Context, database *sql.DB, stashID, priority, kind, text, deliberationID, state, sourcePath string, updatedAt time.Time) error {
	_, err := database.ExecContext(ctx,
		`INSERT INTO stash_entries (stash_id, priority, kind, text, deliberation_id, state, source_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(stash_id) DO UPDATE SET
		   priority = excluded.priority,
		   kind = excluded.kind,
		   text = excluded.text,
		   deliberation_id = excluded.deliberation_id,
		   state = excluded.state,
		   source_path = excluded.source_path,
		   updated_at = excluded.updated_at`,
		stashID,
		priority,
		kind,
		text,
		deliberationID,
		state,
		sourcePath,
		updatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert stash entry %s: %w", stashID, err)
	}
	return nil
}

// LinkStashEntry records a harvested stash-to-item relationship.
func LinkStashEntry(ctx context.Context, database *sql.DB, stashID, itemID string, linkedAt time.Time) error {
	_, err := database.ExecContext(ctx,
		`INSERT INTO stash_links (stash_id, item_id, linked_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(stash_id) DO UPDATE SET
		   item_id = excluded.item_id,
		   linked_at = excluded.linked_at`,
		stashID,
		itemID,
		linkedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("link stash entry %s: %w", stashID, err)
	}
	return nil
}

// ListStashEntries returns indexed stash entries with any harvested item links.
func ListStashEntries(ctx context.Context, database *sql.DB, includeHarvested bool) ([]StashRecord, error) {
	query := `SELECT se.stash_id, se.priority, se.kind, se.text, se.deliberation_id, se.state, se.source_path, se.updated_at, sl.item_id, sl.linked_at
		FROM stash_entries se
		LEFT JOIN stash_links sl ON sl.stash_id = se.stash_id`
	args := []any{}
	if !includeHarvested {
		query += ` WHERE se.state = ?`
		args = append(args, "active")
	}
	query += ` ORDER BY se.updated_at ASC, se.stash_id ASC`

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list stash entries: %w", err)
	}
	defer rows.Close()

	var records []StashRecord
	for rows.Next() {
		var (
			record         StashRecord
			updatedAt      string
			deliberationID sql.NullString
			itemID, linked sql.NullString
		)
		if err := rows.Scan(&record.ID, &record.Priority, &record.Kind, &record.Text, &deliberationID, &record.State, &record.SourcePath, &updatedAt, &itemID, &linked); err != nil {
			return nil, fmt.Errorf("scan stash entry: %w", err)
		}
		record.UpdatedAt = mustParseTime(updatedAt)
		if deliberationID.Valid {
			record.DeliberationID = deliberationID.String
		}
		if itemID.Valid {
			record.ItemID = itemID.String
		}
		if linked.Valid {
			parsed := mustParseTime(linked.String)
			record.LinkedAt = &parsed
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// RehydrateStashIndex rebuilds the stash index from the active stash records and artifact provenance.
// The entire clear-and-rebuild sequence runs inside a single transaction to prevent partial state.
func RehydrateStashIndex(ctx context.Context, database *sql.DB, activeEntries []StashRecord, harvested map[string]StashRecord) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin stash rehydration tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM stash_links`); err != nil {
		return fmt.Errorf("clear stash links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM stash_entries`); err != nil {
		return fmt.Errorf("clear stash entries: %w", err)
	}

	now := time.Now().UTC()
	const upsertStashSQL = `INSERT INTO stash_entries (stash_id, priority, kind, text, deliberation_id, state, source_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(stash_id) DO UPDATE SET
		   priority = excluded.priority,
		   kind = excluded.kind,
		   text = excluded.text,
		   deliberation_id = excluded.deliberation_id,
		   state = excluded.state,
		   source_path = excluded.source_path,
		   updated_at = excluded.updated_at`
	const upsertLinkSQL = `INSERT INTO stash_links (stash_id, item_id, linked_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(stash_id) DO UPDATE SET
		   item_id = excluded.item_id,
		   linked_at = excluded.linked_at`

	for _, entry := range activeEntries {
		if _, err := tx.ExecContext(ctx, upsertStashSQL,
			entry.ID, entry.Priority, entry.Kind, entry.Text, entry.DeliberationID, "active", entry.SourcePath, now.Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("upsert stash entry %s: %w", entry.ID, err)
		}
	}
	for _, record := range harvested {
		if _, err := tx.ExecContext(ctx, upsertStashSQL,
			record.ID, record.Priority, record.Kind, record.Text, record.DeliberationID, "harvested", record.SourcePath, record.UpdatedAt.Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("upsert harvested stash entry %s: %w", record.ID, err)
		}
		linkedAt := record.UpdatedAt
		if record.LinkedAt != nil {
			linkedAt = *record.LinkedAt
		}
		if _, err := tx.ExecContext(ctx, upsertLinkSQL,
			record.ID, record.ItemID, linkedAt.Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("link stash entry %s: %w", record.ID, err)
		}
	}
	return tx.Commit()
}

func mustParseTime(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	slog.Warn("stash: failed to parse timestamp; zero time used", "value", value)
	return time.Time{}
}
