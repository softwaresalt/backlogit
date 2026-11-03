package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
// the same field mapping as UpsertItem. The custody_receipts projection is
// written first, so a malformed workflow_custody envelope fails before any row
// is written for the artifact.
func upsertItemTx(ctx context.Context, tx *sql.Tx, artifact *models.Artifact) error {
	if artifact == nil {
		return fmt.Errorf("upsert item in tx: nil artifact")
	}
	if err := upsertCustodyReceiptsFromArtifactTx(ctx, tx, artifact); err != nil {
		return err
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

	stmt, args, err := buildUpsertItemStatement(ctx, tx, artifact, string(cf), labelsVal, depsVal, refsVal)
	if err != nil {
		return fmt.Errorf("build upsert item %s in tx: %w", artifact.ID, err)
	}
	_, err = tx.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("upsert item %s in tx: %w", artifact.ID, err)
	}
	return nil
}

// custodyFieldKey is the custom_fields key that carries the workflow-custody/v1 envelope.
const custodyFieldKey = "workflow_custody"

// custodyProjection is the part of the workflow-custody/v1 envelope stored in the
// custody_receipts read model. Core owns validation; this decodes the JSON shape
// core writes and never mutates the envelope.
type custodyProjection struct {
	WorkflowID    string           `json:"workflow_id"`
	ScopeBranch   string           `json:"scope_branch"`
	BaseCommitSHA string           `json:"base_commit_sha"`
	Receipts      []custodyReceipt `json:"receipts"`
}

// custodyReceipt is one receipt entry of a custodyProjection.
type custodyReceipt struct {
	ReceiptID          string          `json:"receipt_id"`
	Revision           int             `json:"revision"`
	Kind               string          `json:"kind"`
	AuthorityProfile   string          `json:"authority_profile"`
	Status             string          `json:"status"`
	ProducerRole       string          `json:"producer_role"`
	AcceptingRole      string          `json:"accepting_role"`
	AllowedAction      string          `json:"allowed_action"`
	CandidateSetDigest string          `json:"candidate_set_digest"`
	OfferedAtUTC       string          `json:"offered_at_utc"`
	AcceptedAtUTC      string          `json:"accepted_at_utc"`
	ExpiresAtUTC       string          `json:"expires_at_utc"`
	Entries            json.RawMessage `json:"entries"`
}

// upsertCustodyReceiptsFromArtifactTx replaces the custody_receipts rows for
// artifact.ID with the receipts in its custom_fields.workflow_custody envelope.
// An artifact without a workflow_custody envelope has its rows removed, so a
// shipment whose custody state is gone leaves no stale receipts. The envelope is
// decoded and validated before any write.
func upsertCustodyReceiptsFromArtifactTx(ctx context.Context, tx *sql.Tx, artifact *models.Artifact) error {
	proj, present, err := decodeCustodyProjection(artifact)
	if err != nil {
		return fmt.Errorf("decode %s for %s: %w", custodyFieldKey, artifact.ID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM custody_receipts WHERE shipment_id = ?`, artifact.ID); err != nil {
		return fmt.Errorf("clear custody_receipts for %s: %w", artifact.ID, err)
	}
	if !present {
		return nil
	}
	for _, rc := range proj.Receipts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO custody_receipts (
				shipment_id, receipt_id, workflow_id, scope_branch, base_commit_sha, revision,
				kind, authority_profile, status, producer_role, accepting_role, allowed_action,
				candidate_set_digest, offered_at_utc, accepted_at_utc, expires_at_utc, entries_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			artifact.ID, rc.ReceiptID, proj.WorkflowID, proj.ScopeBranch, proj.BaseCommitSHA, rc.Revision,
			rc.Kind, rc.AuthorityProfile, rc.Status, rc.ProducerRole, rc.AcceptingRole, rc.AllowedAction,
			rc.CandidateSetDigest, rc.OfferedAtUTC, rc.AcceptedAtUTC, rc.ExpiresAtUTC, string(rc.Entries),
		); err != nil {
			return fmt.Errorf("insert custody receipt %s for %s: %w", rc.ReceiptID, artifact.ID, err)
		}
	}
	return nil
}

// decodeCustodyProjection decodes artifact's workflow_custody envelope. present is
// false when the artifact carries no envelope. A duplicate receipt_id is rejected
// here, before any write, because the primary key cannot represent it.
func decodeCustodyProjection(artifact *models.Artifact) (proj custodyProjection, present bool, err error) {
	raw, ok := artifact.CustomFields[custodyFieldKey]
	if !ok || raw == nil {
		return custodyProjection{}, false, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return custodyProjection{}, false, fmt.Errorf("marshal envelope: %w", err)
	}
	if err := json.Unmarshal(data, &proj); err != nil {
		return custodyProjection{}, false, fmt.Errorf("decode envelope: %w", err)
	}
	seen := make(map[string]struct{}, len(proj.Receipts))
	for _, rc := range proj.Receipts {
		if _, dup := seen[rc.ReceiptID]; dup {
			return custodyProjection{}, false, fmt.Errorf("duplicate receipt_id %q", rc.ReceiptID)
		}
		seen[rc.ReceiptID] = struct{}{}
	}
	return proj, true, nil
}
