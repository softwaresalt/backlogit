package db_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

const custodyShipmentID = "211-S"

// custodyEnvelope returns a workflow_custody payload with one receipt, using the
// JSON shape core writes under custom_fields.workflow_custody.
func custodyEnvelope(revision int, receiptStatus, receiptID string) map[string]any {
	return map[string]any{
		"schema_version":   "workflow-custody/v1",
		"workflow_id":      "wf-u5",
		"shipment_id":      custodyShipmentID,
		"scope_branch":     "feat/211-s-u5",
		"base_commit_sha":  "0123456789abcdef0123456789abcdef01234567",
		"custody_revision": revision,
		"receipts": []any{map[string]any{
			"receipt_id":           receiptID,
			"revision":             revision,
			"kind":                 "stage_to_ship_handoff",
			"authority_profile":    "agent_workflow_handoff",
			"status":               receiptStatus,
			"producer_role":        "stage",
			"accepting_role":       "ship",
			"allowed_action":       "claim_shipment",
			"candidate_set_digest": "digest-u5",
			"offered_at_utc":       "2026-11-01T00:00:00Z",
			"accepted_at_utc":      "2026-11-01T01:00:00Z",
			"expires_at_utc":       "2026-11-08T00:00:00Z",
			"entries":              []any{map[string]any{"path": "docs/x.md", "change_kind": "modified"}},
		}},
	}
}

// custodyShipmentArtifact wraps a workflow_custody payload in a shipment artifact.
func custodyShipmentArtifact(env map[string]any) *models.Artifact {
	now := time.Now().UTC().Truncate(time.Second)
	return &models.Artifact{
		ID:           custodyShipmentID,
		Title:        "Custody shipment",
		Status:       models.StatusActive,
		ArtifactType: "shipment",
		CreatedAt:    now,
		UpdatedAt:    now,
		CustomFields: map[string]any{"workflow_custody": env},
	}
}

// writeCustodyShipmentMarkdown writes the shipment Markdown file under <ws>/queue
// with workflow_custody carried as a JSON flow mapping inside frontmatter.
func writeCustodyShipmentMarkdown(t *testing.T, ws string, env map[string]any) string {
	t.Helper()
	fields, err := json.Marshal(map[string]any{"workflow_custody": env})
	require.NoError(t, err)
	body := "---\nid: " + custodyShipmentID + "\ntitle: Custody shipment\nstatus: active\n" +
		"artifact_type: shipment\ncustom_fields: " + string(fields) + "\n---\n"
	dir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, custodyShipmentID+".md")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

// custodyOpenDB opens a fresh database without ensuring the schema.
func custodyOpenDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "custody.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	return database
}

// custodyObjectExists reports whether a named object of the given type exists in sqlite_master.
func custodyObjectExists(t *testing.T, database *sql.DB, objType, name string) bool {
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

type custodyRow struct {
	receiptID, status, scopeBranch, workflowID, entries string
	revision                                            int
}

// queryCustodyRows returns the custody_receipts rows for the test shipment, ordered by receipt_id.
func queryCustodyRows(t *testing.T, database *sql.DB) []custodyRow {
	t.Helper()
	rows, err := database.Query(
		`SELECT receipt_id, status, scope_branch, workflow_id, revision, entries_json
		   FROM custody_receipts WHERE shipment_id = ? ORDER BY receipt_id`, custodyShipmentID)
	require.NoError(t, err)
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close rows: %v", err)
		}
	}()
	var out []custodyRow
	for rows.Next() {
		var r custodyRow
		require.NoError(t, rows.Scan(&r.receiptID, &r.status, &r.scopeBranch, &r.workflowID, &r.revision, &r.entries))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// TestU5_CustodyReceiptsSchemaAndRehydration pins the custody_receipts projection:
// schema creation, live UpsertItem projection in the same transaction, Markdown
// rehydration, stale-row clearing, and introspection.
func TestU5_CustodyReceiptsSchemaAndRehydration(t *testing.T) {
	t.Run("ensure_schema_creates_custody_receipts_idempotently", func(t *testing.T) {
		database := custodyOpenDB(t)
		require.NoError(t, db.EnsureSchema(database))
		require.NoError(t, db.EnsureSchema(database))
		require.NoError(t, db.EnsureSchema(database))

		assert.True(t, custodyObjectExists(t, database, "table", "custody_receipts"),
			"EnsureSchema must create custody_receipts")
		assert.True(t, custodyObjectExists(t, database, "index", "idx_custody_receipts_shipment"),
			"EnsureSchema must create idx_custody_receipts_shipment")
		assert.True(t, custodyObjectExists(t, database, "index", "idx_custody_receipts_workflow"),
			"EnsureSchema must create idx_custody_receipts_workflow")

		var objects int
		require.NoError(t, database.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('custody_receipts','idx_custody_receipts_shipment','idx_custody_receipts_workflow')`,
		).Scan(&objects))
		assert.Equal(t, 3, objects, "repeated EnsureSchema must not duplicate custody objects")

		rows, err := database.Query(`PRAGMA table_info(custody_receipts)`)
		require.NoError(t, err)
		var cols []string
		pk := map[string]int{}
		for rows.Next() {
			var cid, notnull, pkPos int
			var name, ctype string
			var dflt sql.NullString
			require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pkPos))
			cols = append(cols, name)
			if pkPos > 0 {
				pk[name] = pkPos
			}
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())

		assert.ElementsMatch(t, []string{
			"shipment_id", "receipt_id", "workflow_id", "scope_branch", "base_commit_sha",
			"revision", "kind", "authority_profile", "status", "producer_role", "accepting_role",
			"allowed_action", "candidate_set_digest", "offered_at_utc", "accepted_at_utc",
			"expires_at_utc", "entries_json",
		}, cols)
		assert.Equal(t, map[string]int{"shipment_id": 1, "receipt_id": 2}, pk,
			"PRIMARY KEY must be (shipment_id, receipt_id)")
	})

	t.Run("rehydrate_and_live_upsert_item_rebuild_custody_receipts_and_clear_ghosts", func(t *testing.T) {
		database := setupTestDB(t)
		ctx := context.Background()

		// Live UpsertItem populates the projection in the same transaction, before any Rehydrate.
		require.NoError(t, db.UpsertItem(ctx, database, custodyShipmentArtifact(custodyEnvelope(1, "offered", "rcpt-u5-1"))))
		rows := queryCustodyRows(t, database)
		require.Len(t, rows, 1, "UpsertItem must project one custody receipt row")
		assert.Equal(t, "rcpt-u5-1", rows[0].receiptID)
		assert.Equal(t, "offered", rows[0].status)
		assert.Equal(t, "feat/211-s-u5", rows[0].scopeBranch)
		assert.Equal(t, "wf-u5", rows[0].workflowID)
		assert.Equal(t, 1, rows[0].revision)
		assert.JSONEq(t, `[{"change_kind":"modified","path":"docs/x.md"}]`, rows[0].entries)

		// A subsequent live update changes the projected row.
		require.NoError(t, db.UpsertItem(ctx, database, custodyShipmentArtifact(custodyEnvelope(2, "accepted", "rcpt-u5-1"))))
		rows = queryCustodyRows(t, database)
		require.Len(t, rows, 1)
		assert.Equal(t, "accepted", rows[0].status, "live update must replace the projected status")
		assert.Equal(t, 2, rows[0].revision, "live update must replace the projected revision")

		// Rehydrate rebuilds custody_receipts from the shipment Markdown, replacing live rows.
		ws := t.TempDir()
		mdPath := writeCustodyShipmentMarkdown(t, ws, custodyEnvelope(3, "accepted", "rcpt-md-3"))
		_, err := db.Rehydrate(ctx, ws, database)
		require.NoError(t, err)
		rows = queryCustodyRows(t, database)
		require.Len(t, rows, 1, "Rehydrate must rebuild exactly the receipts present in Markdown")
		assert.Equal(t, "rcpt-md-3", rows[0].receiptID)
		assert.Equal(t, 3, rows[0].revision)

		// Removing the shipment Markdown must clear its stale custody rows.
		require.NoError(t, os.Remove(mdPath))
		_, err = db.Rehydrate(ctx, ws, database)
		require.NoError(t, err)
		assert.Empty(t, queryCustodyRows(t, database), "removed shipment must leave no custody_receipts rows")
	})

	t.Run("introspect_schema_includes_custody_receipts", func(t *testing.T) {
		database := custodyOpenDB(t)
		ctx := context.Background()
		require.NoError(t, db.EnsureSchema(database))

		first, err := db.IntrospectSchema(ctx, database)
		require.NoError(t, err)
		second, err := db.IntrospectSchema(ctx, database)
		require.NoError(t, err)
		assert.Equal(t, first, second, "schema introspection must be deterministic")

		var table *db.TableSchema
		for i := range first {
			if first[i].Name == "custody_receipts" {
				table = &first[i]
				break
			}
		}
		require.NotNil(t, table, "IntrospectSchema must report custody_receipts")

		indexes := map[string][]string{}
		for _, ix := range table.Indexes {
			indexes[ix.Name] = ix.Columns
		}
		assert.Equal(t, []string{"shipment_id"}, indexes["idx_custody_receipts_shipment"])
		assert.Equal(t, []string{"workflow_id"}, indexes["idx_custody_receipts_workflow"])
	})
}
