package contract_test

// Contract tests for shipment MCP tools (F015 / T003 / ST018).
// Each tool is tested for:
//   - missing required parameter validation
//   - success path with real data
//   - pre-init descriptive error (tools must not be hidden)

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// backlogit_create_shipment
// ---------------------------------------------------------------------------

func TestCreateShipment_MissingTitle(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Act
	result, err := callToolForTest(t, s, "backlogit_create_shipment", map[string]any{})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing title should return error")
}

func TestCreateShipment_Success(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Act
	data := callToolAndParseJSON(t, s, "backlogit_create_shipment", map[string]any{
		"title": "Contract test shipment",
	})

	// Assert
	id, ok := data["id"].(string)
	require.True(t, ok, "response must have string id")
	assert.Contains(t, id, "S", "shipment ID must contain S prefix")
	assert.Equal(t, "shipment", data["artifact_type"])
	assert.Equal(t, "queued", data["status"])
}

// ---------------------------------------------------------------------------
// backlogit_get_shipment
// ---------------------------------------------------------------------------

func TestGetShipment_MissingID(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Act
	result, err := callToolForTest(t, s, "backlogit_get_shipment", map[string]any{})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing id should return error")
}

func TestGetShipment_NotFound(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Act
	result, err := callToolForTest(t, s, "backlogit_get_shipment", map[string]any{
		"id": "S999",
	})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "non-existent shipment should return error")
}

func TestGetShipment_Success(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)
	created := callToolAndParseJSON(t, s, "backlogit_create_shipment", map[string]any{
		"title": "Get test",
	})
	id := created["id"].(string)

	// Act
	data := callToolAndParseJSON(t, s, "backlogit_get_shipment", map[string]any{
		"id": id,
	})

	// Assert
	assert.Equal(t, id, data["id"])
	assert.Equal(t, "Get test", data["title"])
}

// ---------------------------------------------------------------------------
// backlogit_list_shipments
// ---------------------------------------------------------------------------

func TestListShipments_Success(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)
	_ = callToolAndParseJSON(t, s, "backlogit_create_shipment", map[string]any{
		"title": "Listed shipment",
	})

	// Act
	data := callToolAndParseJSONSlice(t, s, "backlogit_list_shipments", map[string]any{})

	// Assert
	require.NotEmpty(t, data, "must return at least one shipment")
	found := false
	for _, item := range data {
		if item["title"] == "Listed shipment" {
			found = true
		}
	}
	assert.True(t, found, "created shipment must appear in list")
}

// ---------------------------------------------------------------------------
// backlogit_claim_shipment
// ---------------------------------------------------------------------------

func TestClaimShipment_Success(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)
	created := callToolAndParseJSON(t, s, "backlogit_create_shipment", map[string]any{
		"title": "Claimable shipment",
	})
	id := created["id"].(string)

	// Act
	data := callToolAndParseJSON(t, s, "backlogit_claim_shipment", map[string]any{
		"id": id,
	})

	// Assert
	assert.Equal(t, "active", data["status"], "claimed shipment must be active")
}

// ---------------------------------------------------------------------------
// backlogit_return_blocked
// ---------------------------------------------------------------------------

func TestReturnBlocked_MissingParams(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Act
	result, err := callToolForTest(t, s, "backlogit_return_blocked", map[string]any{})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing params should return error")
}

func TestReturnBlocked_Success(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Create a shipment with an item
	created := callToolAndParseJSON(t, s, "backlogit_create_shipment", map[string]any{
		"title": "Return test shipment",
	})
	shipmentID := created["id"].(string)

	taskData := callToolAndParseJSON(t, s, "backlogit_create_item", map[string]any{
		"title":         "Returnable task",
		"artifact_type": "task",
		"status":        "queued",
	})
	taskID := taskData["id"].(string)

	_ = callToolAndParseJSON(t, s, "backlogit_add_to_shipment", map[string]any{
		"shipment_id": shipmentID,
		"item_id":     taskID,
	})

	// Act
	data := callToolAndParseJSON(t, s, "backlogit_return_blocked", map[string]any{
		"shipment_id": shipmentID,
		"item_id":     taskID,
		"reason":      "dependency blocked",
	})

	// Assert
	assert.Equal(t, "blocked", data["item_status"], "returned item must be blocked")
}
