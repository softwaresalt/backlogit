package db_test

import (
	"database/sql"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

// objectExists reports whether a named object of the given type is present in
// sqlite_master.
func objectExists(t *testing.T, database *sql.DB, objType, name string) bool {
	t.Helper()
	var got string
	err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = ? AND name = ?`, objType, name,
	).Scan(&got)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return got == name
}

// TestEnsureSchema_GateEvidenceProjection pins Q3.1 (083.005.002-ST): EnsureSchema
// creates the derived gate_evidence(item_id, gate_status, evidence_sha, head_sha)
// projection table plus the gate_status discriminator index, and re-running
// EnsureSchema is a no-op (idempotent).
func TestEnsureSchema_GateEvidenceProjection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gate_evidence_schema.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })
	require.NoError(t, db.EnsureSchema(database))

	// (1) table + index exist.
	assert.True(t, objectExists(t, database, "table", "gate_evidence"),
		"expected gate_evidence table to be created")
	assert.True(t, objectExists(t, database, "index", "idx_gate_evidence_status"),
		"expected idx_gate_evidence_status index to be created")

	// Column shape: item_id (PK), gate_status, evidence_sha, head_sha.
	rows, err := database.Query(`PRAGMA table_info(gate_evidence)`)
	require.NoError(t, err)
	var cols []string
	pkCols := map[string]int{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		cols = append(cols, name)
		if pk > 0 {
			pkCols[name] = pk
		}
	}
	require.NoError(t, rows.Err())
	rows.Close()
	sort.Strings(cols)
	assert.Equal(t, []string{"evidence_sha", "gate_status", "head_sha", "item_id"}, cols,
		"gate_evidence must store only the status token + SHAs (no report JSON/stderr/force_reason)")
	assert.Equal(t, 1, pkCols["item_id"], "item_id must be the primary key")

	// (2) idempotent: re-running EnsureSchema is a no-op with no error.
	require.NoError(t, db.EnsureSchema(database))
	require.NoError(t, db.EnsureSchema(database))
	assert.True(t, objectExists(t, database, "table", "gate_evidence"))
	assert.True(t, objectExists(t, database, "index", "idx_gate_evidence_status"))

	// A row can be written and read back (advisory-derived projection is usable).
	_, err = database.Exec(
		`INSERT INTO gate_evidence(item_id, gate_status, evidence_sha, head_sha) VALUES (?,?,?,?)`,
		"001.001-T", "passed", "sha:aa", "head:11")
	require.NoError(t, err)
	var status, esha, hsha string
	require.NoError(t, database.QueryRow(
		`SELECT gate_status, evidence_sha, head_sha FROM gate_evidence WHERE item_id = ?`, "001.001-T",
	).Scan(&status, &esha, &hsha))
	assert.Equal(t, "passed", status)
	assert.Equal(t, "sha:aa", esha)
	assert.Equal(t, "head:11", hsha)
}
