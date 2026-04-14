package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/softwaresalt/backlogit/internal/models"
)

// UpsertItemsTx inserts or replaces one or more artifacts within an existing
// SQL transaction. All writes share the same transaction so a rollback reverts
// every artifact atomically.
func UpsertItemsTx(ctx context.Context, tx *sql.Tx, artifacts ...*models.Artifact) error {
	for _, artifact := range artifacts {
		if err := upsertItemTx(ctx, tx, artifact); err != nil {
			return err
		}
	}
	return nil
}

// upsertItemTx executes a single artifact INSERT OR REPLACE inside tx using
// the same field mapping as UpsertItem.
func upsertItemTx(ctx context.Context, tx *sql.Tx, artifact *models.Artifact) error {
	if artifact == nil {
		return fmt.Errorf("upsert item in tx: nil artifact")
	}
	cf, err := json.Marshal(artifact.CustomFields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}
	labelsJSON, err := json.Marshal(artifact.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}
	depsJSON, err := json.Marshal(artifact.Dependencies)
	if err != nil {
		return fmt.Errorf("marshal dependencies: %w", err)
	}
	refsJSON, err := json.Marshal(artifact.References)
	if err != nil {
		return fmt.Errorf("marshal references: %w", err)
	}

	labelsVal := nullString(string(labelsJSON))
	if string(labelsJSON) == "null" {
		labelsVal = sql.NullString{}
	}
	depsVal := nullString(string(depsJSON))
	if string(depsJSON) == "null" {
		depsVal = sql.NullString{}
	}
	refsVal := nullString(string(refsJSON))
	if string(refsJSON) == "null" {
		refsVal = sql.NullString{}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO items
			(id, title, status, artifact_type, parent_id, sprint, priority, description,
			 custom_fields, created_at, updated_at,
			 assigned_to, owner, labels, dependencies, "references", "commit",
			 level, hierarchy_path)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ID,
		artifact.Title,
		string(artifact.Status),
		artifact.ArtifactType,
		nullString(artifact.ParentID),
		nullString(artifact.Sprint),
		nullString(artifact.Priority),
		nullString(artifact.Description),
		string(cf),
		artifact.CreatedAt.Format(time.RFC3339Nano),
		artifact.UpdatedAt.Format(time.RFC3339Nano),
		nullString(artifact.AssignedTo),
		nullString(artifact.Owner),
		labelsVal,
		depsVal,
		refsVal,
		nullString(artifact.Commit),
		nullInt64(artifact.Level),
		nullString(artifact.HierarchyPath),
	)
	if err != nil {
		return fmt.Errorf("upsert item %s in tx: %w", artifact.ID, err)
	}
	return nil
}
