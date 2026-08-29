package events

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckpointContext_U5_MarshalJSON_MalformedExtra (148.004-T / U5)
// directly constructs a CheckpointContext with a malformed json.RawMessage
// in Extra and asserts MarshalJSON returns an error rather than silently
// producing invalid JSON bytes.
func TestCheckpointContext_U5_MarshalJSON_MalformedExtra(t *testing.T) {
	ctx := CheckpointContext{
		ShipmentID: "001-S",
		Extra: map[string]json.RawMessage{
			"bad_key": json.RawMessage(`{malformed`),
		},
	}

	_, err := ctx.MarshalJSON()
	require.Error(t, err, "MarshalJSON must error on malformed Extra value")
}

// TestCheckpointContext_U5_Keys_MalformedExtra asserts Keys() returns an
// error for a directly-constructed CheckpointContext with malformed Extra.
func TestCheckpointContext_U5_Keys_MalformedExtra(t *testing.T) {
	ctx := CheckpointContext{
		ShipmentID: "001-S",
		Extra: map[string]json.RawMessage{
			"bad_key": json.RawMessage(`nil_value`),
		},
	}

	_, err := ctx.Keys()
	require.Error(t, err, "Keys must error on malformed Extra value")
}

// TestCheckpointContext_U5_MarshalJSON_ValidExtra asserts that valid Extra
// values are emitted unchanged, preserving the open-context design.
func TestCheckpointContext_U5_MarshalJSON_ValidExtra(t *testing.T) {
	ctx := CheckpointContext{
		ShipmentID: "001-S",
		Extra: map[string]json.RawMessage{
			"my_key": json.RawMessage(`"my_value"`),
		},
	}

	data, err := ctx.MarshalJSON()
	require.NoError(t, err, "valid Extra must marshal without error")

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	assert.Equal(t, "my_value", result["my_key"], "valid Extra value must be emitted")
}

// TestCheckpointContext_U5_Keys_ErrorNamesOffendingKey asserts the error
// message includes the offending key name to aid diagnosis.
func TestCheckpointContext_U5_Keys_ErrorNamesOffendingKey(t *testing.T) {
	ctx := CheckpointContext{
		Extra: map[string]json.RawMessage{
			"offending_field": json.RawMessage(`{broken`),
		},
	}

	_, err := ctx.Keys()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "offending_field",
		"error must name the offending Extra key")
}
