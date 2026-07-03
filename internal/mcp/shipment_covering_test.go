package mcp

// 075.003-T: MCP shipment tools surface the read-only covering feature.
//
// These tests use package-internal access to call the unexported handlers
// directly, mirroring shipment_response_test.go. They assert the top-level
// covering_feature projection, list==get same-shape (with and without a covering
// feature), the read-only invariant, and CLI==MCP field parity via the shared
// core.NewShipmentView shaper (the identical type the CLI marshals).

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
)

// seedCoveredShipment creates a root feature and a shipment covering it,
// returning (shipmentID, featureID, featureTitle).
func seedCoveredShipment(t *testing.T, ws *core.Workspace, title string) (string, string, string) {
	t.Helper()
	ctx := context.Background()
	feat, err := core.CreateArtifact(ctx, ws, "Covered feature", "feature")
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, title, []string{feat.ID})
	require.NoError(t, err)
	return shipment.ID, feat.ID, feat.Title
}

func findShipmentInList(t *testing.T, list []map[string]any, id string) map[string]any {
	t.Helper()
	for _, m := range list {
		if m["id"] == id {
			return m
		}
	}
	t.Fatalf("shipment %s not found in list result", id)
	return nil
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// readShipmentManifest locates and returns the stored Markdown file bytes for an
// artifact under the workspace .backlogit tree.
func readShipmentManifest(t *testing.T, ws *core.Workspace, id string) string {
	t.Helper()
	base := id + ".md"
	var found string
	require.NoError(t, filepath.WalkDir(filepath.Join(ws.RootPath, ".backlogit"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == base {
			found = path
		}
		return nil
	}))
	require.NotEmpty(t, found, "stored file for %s not found", id)
	raw, err := os.ReadFile(found)
	require.NoError(t, err)
	return string(raw)
}

func shipmentRowJSON(t *testing.T, ws *core.Workspace, id string) string {
	t.Helper()
	art, err := bldb.GetItem(context.Background(), ws.DB, id)
	require.NoError(t, err)
	raw, err := json.Marshal(art)
	require.NoError(t, err)
	return string(raw)
}

// Unit 3 scenario 1: get + list include a top-level covering_feature object;
// custom_fields.items remains a non-null array; no leak into custom_fields.
func TestShipment_MCP_IncludesCoveringFeature(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	shipID, featID, featTitle := seedCoveredShipment(t, ws, "Covered shipment")

	getRes, err := s.handleGetShipment(ctx, shipmentRequest(map[string]any{"id": shipID}))
	require.NoError(t, err)
	getMap := shipmentFromResult(t, getRes)
	cf, ok := getMap["covering_feature"].(map[string]any)
	require.True(t, ok, "get: covering_feature must be present at top level")
	assert.Equal(t, featID, cf["id"])
	assert.Equal(t, featTitle, cf["title"])
	require.NotNil(t, itemsField(t, getMap), "custom_fields.items must remain non-null")

	custom, _ := getMap["custom_fields"].(map[string]any)
	_, leaked := custom["covering_feature"]
	assert.False(t, leaked, "covering_feature must never leak into custom_fields")

	listRes, err := s.handleListShipments(ctx, shipmentRequest(map[string]any{}))
	require.NoError(t, err)
	listMap := findShipmentInList(t, shipmentsFromResult(t, listRes), shipID)
	lcf, ok := listMap["covering_feature"].(map[string]any)
	require.True(t, ok, "list: covering_feature must be present at top level")
	assert.Equal(t, featID, lcf["id"])
	require.NotNil(t, itemsField(t, listMap), "list custom_fields.items must remain non-null")
}

// Unit 3 scenario 2: list==get same-shape for a shipment WITH a covering feature.
func TestShipment_MCP_SameShape_WithCoveringFeature(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	shipID, _, _ := seedCoveredShipment(t, ws, "Same-shape shipment")

	getRes, err := s.handleGetShipment(ctx, shipmentRequest(map[string]any{"id": shipID}))
	require.NoError(t, err)
	getMap := shipmentFromResult(t, getRes)

	listRes, err := s.handleListShipments(ctx, shipmentRequest(map[string]any{}))
	require.NoError(t, err)
	listMap := findShipmentInList(t, shipmentsFromResult(t, listRes), shipID)

	assert.ElementsMatch(t, mapKeys(getMap), mapKeys(listMap), "top-level key sets must match between get and list")
	assert.Equal(t, getMap["covering_feature"], listMap["covering_feature"], "covering_feature must be identical between get and list")
}

// Unit 3 scenario 3: list==get same-shape for a ZERO-feature shipment; both omit
// covering_feature entirely.
func TestShipment_MCP_SameShape_ZeroFeature_BothOmit(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	feat, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Only a task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, "Zero-feature shipment", []string{task.ID})
	require.NoError(t, err)

	getRes, err := s.handleGetShipment(ctx, shipmentRequest(map[string]any{"id": shipment.ID}))
	require.NoError(t, err)
	getMap := shipmentFromResult(t, getRes)

	listRes, err := s.handleListShipments(ctx, shipmentRequest(map[string]any{}))
	require.NoError(t, err)
	listMap := findShipmentInList(t, shipmentsFromResult(t, listRes), shipment.ID)

	_, getHas := getMap["covering_feature"]
	_, listHas := listMap["covering_feature"]
	assert.False(t, getHas, "get must omit covering_feature for a zero-feature shipment")
	assert.False(t, listHas, "list must omit covering_feature for a zero-feature shipment")
	assert.ElementsMatch(t, mapKeys(getMap), mapKeys(listMap), "zero-feature list==get key sets must match")
}

// Unit 3 scenario 4: rendering the MCP shipment tools mutates neither the stored
// manifest nor the DB row, and never persists covering_feature (no upsert).
func TestShipment_MCP_ReadOnly_NoStoreMutation(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	shipID, _, _ := seedCoveredShipment(t, ws, "Read-only shipment")

	fileBefore := readShipmentManifest(t, ws, shipID)
	rowBefore := shipmentRowJSON(t, ws, shipID)

	_, err := s.handleGetShipment(ctx, shipmentRequest(map[string]any{"id": shipID}))
	require.NoError(t, err)
	_, err = s.handleListShipments(ctx, shipmentRequest(map[string]any{}))
	require.NoError(t, err)

	fileAfter := readShipmentManifest(t, ws, shipID)
	rowAfter := shipmentRowJSON(t, ws, shipID)

	assert.Equal(t, fileBefore, fileAfter, "stored manifest must be byte-for-byte unchanged")
	assert.JSONEq(t, rowBefore, rowAfter, "DB row must be unchanged after rendering")
	assert.NotContains(t, fileAfter, "covering_feature", "covering_feature must never be persisted")
}

// Unit 3 scenario 5: CLI==MCP parity. Both the CLI and MCP surfaces marshal the
// identical core.ShipmentView type, so the MCP covering_feature projection must
// be byte-equal to what the shared shaper (which the CLI also marshals) produces:
// identical field name, {id,title} semantics, and omit behavior.
func TestShipment_CLIMCPParity_SharedShaper(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	shipID, featID, featTitle := seedCoveredShipment(t, ws, "Parity shipment")

	getRes, err := s.handleGetShipment(ctx, shipmentRequest(map[string]any{"id": shipID}))
	require.NoError(t, err)
	mcpCF, ok := shipmentFromResult(t, getRes)["covering_feature"].(map[string]any)
	require.True(t, ok, "MCP surface must emit covering_feature")

	shipment, err := core.GetShipment(ctx, ws, shipID)
	require.NoError(t, err)
	shaped, err := json.Marshal(core.NewShipmentView(ctx, ws, shipment))
	require.NoError(t, err)
	var shapedMap map[string]any
	require.NoError(t, json.Unmarshal(shaped, &shapedMap))
	sharedCF, ok := shapedMap["covering_feature"].(map[string]any)
	require.True(t, ok, "shared shaper must emit covering_feature")

	assert.Equal(t, sharedCF, mcpCF, "MCP covering_feature must match the shared shaper the CLI also marshals")
	assert.ElementsMatch(t, []string{"id", "title"}, mapKeys(mcpCF), "covering_feature must expose exactly id and title")
	assert.Equal(t, featID, mcpCF["id"])
	assert.Equal(t, featTitle, mcpCF["title"])
}
