package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
)

// U4 (078.004-T, folds 7ECBAC7E): CLI `shipment list --format json` must emit
// custom_fields.items as a JSON array ([]) — never null and never absent — for a
// shipment whose stored representation carries a non-array items value, matching
// the MCP list surface. SQLite JSON round-trips treat empty/absent arrays as
// lossy (see docs/compound/go-patterns/f015-shipment-stash-patterns.md), so a shipment whose
// stored custom_fields.items is null reaches the read edge as null. The MCP list
// handler already normalizes on that edge; the CLI list handler must do the same
// so both surfaces marshal an identical array shape through
// db.QueryItems -> core.NormalizeShipmentItems -> core.NewShipmentViews.

// itemsShapeForShipment returns the raw JSON value of custom_fields.items for the
// shipment with id shipID within a marshaled shipment-list array. A missing
// shipment fails the test; a present shipment with a null/absent items yields the
// literal "null"/"absent" sentinel so the caller can assert the never-null rule.
func itemsShapeForShipment(t *testing.T, listJSON, shipID string) string {
	t.Helper()
	var arr []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(listJSON), &arr))
	for _, m := range arr {
		var id string
		require.NoError(t, json.Unmarshal(m["id"], &id))
		if id != shipID {
			continue
		}
		cfRaw, ok := m["custom_fields"]
		if !ok {
			return "absent"
		}
		var cf map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(cfRaw, &cf))
		itemsRaw, ok := cf["items"]
		if !ok {
			return "absent"
		}
		return strings.TrimSpace(string(itemsRaw))
	}
	t.Fatalf("shipment %s not found in list output", shipID)
	return ""
}

// writeRawShipmentWithNullItems writes a shipment Markdown artifact directly into
// the workspace queue with custom_fields.items explicitly set to null, bypassing
// the CreateShipment write-path normalizer. This reproduces the lossy read-edge
// condition (a stored shipment whose items round-trips to null) that the MCP
// surface already defends against.
func writeRawShipmentWithNullItems(t *testing.T, root, shipID, title string) {
	t.Helper()
	content := fmt.Sprintf(`---
artifact_type: shipment
created_at: 2026-07-03T00:00:00Z
custom_fields:
    items: null
id: %s
status: queued
title: %s
updated_at: 2026-07-03T00:00:00Z
---
`, shipID, title)
	queueDir := filepath.Join(root, ".backlogit", "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, shipID+".md"), []byte(content), 0o644))
}

// U4 red->green scenario: a shipment whose stored items value is null must render
// items:[] on the CLI JSON surface — never null, never absent.
func TestShipmentList_NullStoredItems_CLINeverNull(t *testing.T) {
	root := setupCLIWorkspace(t)
	const shipID = "900-S"
	writeRawShipmentWithNullItems(t, root, shipID, "Null items shipment")

	// Index the hand-written artifact so the read edge (db.QueryItems) sees it.
	_ = runCLIStdout(t, root, "sync")

	listJSON := runCLIStdout(t, root, "shipment", "list", "--format", "json")
	shape := itemsShapeForShipment(t, listJSON, shipID)

	assert.NotEqual(t, "null", shape, "CLI shipment list must never emit items:null")
	assert.NotEqual(t, "absent", shape, "CLI shipment list must always carry an items array")
	assert.Equal(t, "[]", shape, "null-stored shipment must render items as an empty JSON array")
}

// U4 scenario: an empty shipment created via the CLI (which normalizes on write)
// must also render items:[] on the CLI JSON surface.
func TestShipmentList_EmptyItems_CLINeverNull(t *testing.T) {
	root := setupCLIWorkspace(t)
	shipID := cliCreateShipment(t, root, "Empty shipment", "")

	listJSON := runCLIStdout(t, root, "shipment", "list", "--format", "json")
	shape := itemsShapeForShipment(t, listJSON, shipID)

	assert.NotEqual(t, "null", shape, "CLI shipment list must never emit items:null")
	assert.NotEqual(t, "absent", shape, "CLI shipment list must always carry an items array")
	assert.Equal(t, "[]", shape, "empty shipment must render items as an empty JSON array")
}

// U4 cross-surface invariant: the MCP list surface marshals through the exact
// same exported pipeline (db.QueryItems -> core.NormalizeShipmentItems ->
// core.NewShipmentViews). Replicating that pipeline here proves the null-items
// edge is array-shaped on the MCP side too, so a future third consumer of
// NewShipmentViews inherits the same guarantee.
func TestShipmentList_NullStoredItems_MCPPipelineNeverNull(t *testing.T) {
	root := setupCLIWorkspace(t)
	const shipID = "901-S"
	writeRawShipmentWithNullItems(t, root, shipID, "Null items shipment")
	_ = runCLIStdout(t, root, "sync")

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	shipments, err := bldb.QueryItems(ctx, ws.DB, bldb.QueryFilters{Type: "shipment"})
	require.NoError(t, err)

	// Mirror internal/mcp handleListShipments exactly.
	for _, shipment := range shipments {
		if shipment.CustomFields == nil {
			shipment.CustomFields = map[string]any{}
		}
		shipment.CustomFields["items"] = core.NormalizeShipmentItems(shipment)
	}

	data, err := json.Marshal(core.NewShipmentViews(ctx, ws, shipments))
	require.NoError(t, err)
	shape := itemsShapeForShipment(t, string(data), shipID)
	assert.Equal(t, "[]", shape, "MCP list pipeline must render null-stored items as []")
}
