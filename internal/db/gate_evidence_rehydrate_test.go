package db_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
)

// writeGateLog writes a per-item JSONL event log under <ws>/logs/<id>.jsonl.
func writeGateLog(t *testing.T, ws, itemID string, evs ...events.Event) string {
	t.Helper()
	logsDir := filepath.Join(ws, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	path := filepath.Join(logsDir, itemID+".jsonl")
	var buf []byte
	for _, e := range evs {
		e.ItemID = itemID
		line, err := json.Marshal(e)
		require.NoError(t, err)
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	require.NoError(t, os.WriteFile(path, buf, 0o644))
	return path
}

func gateEvt(eventType string, ran bool, hash, head string) events.Event {
	d := map[string]any{"ran": ran}
	if hash != "" {
		d["gate_report_hash"] = hash
	}
	if head != "" {
		d["head_sha"] = head
	}
	return events.Event{Actor: "backlogit", EventType: eventType, Delta: d}
}

type gateRow struct {
	status, evidenceSHA, headSHA string
}

func queryGateEvidence(t *testing.T, database *sql.DB) map[string]gateRow {
	t.Helper()
	rows, err := database.Query(`SELECT item_id, gate_status, evidence_sha, head_sha FROM gate_evidence`)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]gateRow{}
	for rows.Next() {
		var id, status, esha, hsha string
		require.NoError(t, rows.Scan(&id, &status, &esha, &hsha))
		out[id] = gateRow{status: status, evidenceSHA: esha, headSHA: hsha}
	}
	require.NoError(t, rows.Err())
	return out
}

// TestRehydrateGateEvidence_PopulatesProjection pins Q3.2 (083.005.003-ST): after
// Rehydrate, each item that went through the gate has one gate_evidence row whose
// status matches the Q3.0 predicate (passed/forced/forced_no_run/missing), the
// projection is fully rebuilt (idempotent across two syncs), and the item log
// JSONL is never mutated. Only the status token + SHAs are stored.
func TestRehydrateGateEvidence_PopulatesProjection(t *testing.T) {
	ws := t.TempDir()
	// Gated items with distinct outcomes under the composed predicate.
	writeGateLog(t, ws, "Q32-PASS", gateEvt(gateevidence.EventGatePassed, true, "sha:pass", "head:pass"))
	writeGateLog(t, ws, "Q32-FORCED", gateEvt(gateevidence.EventGateForced, true, "sha:forced", "head:forced"))
	writeGateLog(t, ws, "Q32-FORCEDNORUN", gateEvt(gateevidence.EventGateForced, false, "sha:fnr", "head:fnr"))
	// Gated but no valid evidence -> missing rows.
	failOpenLogPath := writeGateLog(t, ws, "Q32-FAILOPEN", gateEvt(gateevidence.EventGatePassed, false, "sha:x", "head:x"))
	writeGateLog(t, ws, "Q32-BLOCKONLY", gateEvt(gateevidence.EventGateBlocked, false, "", ""))
	// Non-gate event only -> NOT indexed (never went through the gate).
	nonGate := events.Event{Actor: "backlogit", EventType: "status_changed", Delta: map[string]any{"new_status": "done"}}
	writeGateLog(t, ws, "Q32-NONGATE", nonGate)

	database := setupTestDB(t)
	ctx := context.Background()

	// Capture the FAILOPEN log bytes before rehydration to prove immutability.
	before, err := os.ReadFile(failOpenLogPath)
	require.NoError(t, err)

	_, err = db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	first := queryGateEvidence(t, database)

	want := map[string]gateRow{
		"Q32-PASS":        {"passed", "sha:pass", "head:pass"},
		"Q32-FORCED":      {"forced", "sha:forced", "head:forced"},
		"Q32-FORCEDNORUN": {"forced_no_run", "sha:fnr", "head:fnr"},
		"Q32-FAILOPEN":    {"missing", "", ""},
		"Q32-BLOCKONLY":   {"missing", "", ""},
	}
	assert.Equal(t, want, first, "projection rows must match the Q3.0 predicate per gated item")
	_, hasNonGate := first["Q32-NONGATE"]
	assert.False(t, hasNonGate, "an item that never went through the gate must not be indexed")

	// (2) Idempotent: a second sync yields identical rows (full rebuild).
	_, err = db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	second := queryGateEvidence(t, database)
	assert.Equal(t, first, second, "running sync twice must yield identical projection rows")

	// (3) Item log JSONL is never mutated.
	after, err := os.ReadFile(failOpenLogPath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "rehydration must not mutate item log JSONL")
}
