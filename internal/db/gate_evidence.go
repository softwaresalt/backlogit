package db

import (
	"context"
	"database/sql"
	"fmt"
)

// LoadGateEvidence loads the entire derived gate_evidence projection as a map of
// item_id -> gate_status token (Q3.2/Q3.3). It backs the advisory doctor
// gate-evidence audit, which prefers this index over scanning each item's logs.
//
// An item absent from the returned map is not indexed — either it never went
// through the gate, or it was gated since the last `sync` (the live completion
// path indexes events incrementally but does not touch this disposable
// projection). Callers MUST treat absence as "unknown" and fall back to the
// authoritative per-item log-scan rather than assuming the item lacks evidence,
// so a stale or absent projection never produces a false negative.
func LoadGateEvidence(ctx context.Context, database *sql.DB) (map[string]string, error) {
	rows, err := database.QueryContext(ctx, `SELECT item_id, gate_status FROM gate_evidence`)
	if err != nil {
		return nil, fmt.Errorf("query gate_evidence: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var id, status string
		if scanErr := rows.Scan(&id, &status); scanErr != nil {
			return nil, fmt.Errorf("scan gate_evidence row: %w", scanErr)
		}
		out[id] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gate_evidence rows: %w", err)
	}
	return out, nil
}
