package db

import (
	"context"
	"database/sql"
	"fmt"
)

// LoadPassingGateEvidence loads the item_ids that carry POSITIVE derived
// gate-evidence — a passed, forced, or forced_no_run row — from the disposable
// gate_evidence projection (Q3.2/Q3.3), keyed to their status token. It backs
// the advisory doctor gate-evidence audit's fast path.
//
// Only positive-evidence rows are returned, and this is deliberate. Item logs
// are append-only, so a pass recorded at the last `sync` is still present in the
// logs; that makes a positive projection row safe to trust WITHOUT re-reading
// logs. A "missing" projection row, by contrast, can be STALE in the pass
// direction — the live completion path appends a pass to the item log but does
// not touch this disposable projection between syncs — so missing/absent items
// are intentionally excluded here and MUST be re-verified by the caller against
// the authoritative per-item log-scan. This keeps the item logs the single
// source of truth while still fast-pathing the common evidenced-terminal case.
//
// The gate_status IN (...) predicate is served by idx_gate_evidence_status. The
// literal status tokens are asserted to match the gateevidence.Status* constants
// by TestLoadPassingGateEvidence_StatusTokensMatchConstants (drift guard).
func LoadPassingGateEvidence(ctx context.Context, database *sql.DB) (map[string]string, error) {
	const query = `SELECT item_id, gate_status FROM gate_evidence
WHERE gate_status IN ('passed', 'forced', 'forced_no_run')`
	rows, err := database.QueryContext(ctx, query)
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
