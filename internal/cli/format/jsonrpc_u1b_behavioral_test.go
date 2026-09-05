// Package format_test contains U1b behavioral tests for WrapErrorData.
// These tests require WrapErrorData to be declared in the production code
// and are therefore added after the source-shape harness turns green.
package format_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli/format"
)

// TestU1b_WrapErrorDataNilByteIdenticalToWrapError verifies that calling
// WrapErrorData with a nil data argument produces output that is byte-identical
// to WrapError — i.e. the "data" key is absent (omitempty works as intended).
func TestU1b_WrapErrorDataNilByteIdenticalToWrapError(t *testing.T) {
	const (
		id   = "backlogit get"
		code = format.ErrCodeInvalidParams
		msg  = "invalid params"
	)

	wrapErr, err := format.WrapError(id, code, msg)
	require.NoError(t, err, "WrapError must not return an error")

	wrapData, err := format.WrapErrorData(id, code, msg, nil)
	require.NoError(t, err, "WrapErrorData(nil) must not return an error")

	assert.True(t, bytes.Equal(wrapErr, wrapData),
		"WrapErrorData(nil) output must be byte-identical to WrapError\n"+
			"WrapError:     %s\n"+
			"WrapErrorData: %s", wrapErr, wrapData)
}

// TestU1b_TypedNilNoDataNull verifies the typed-nil guard in WrapErrorData.
// A typed-nil struct pointer passed as data must NOT produce "data":null in JSON.
func TestU1b_TypedNilNoDataNull(t *testing.T) {
	type diagnostic struct {
		Cause string `json:"cause"`
	}

	var typedNil *diagnostic // typed nil: (*diagnostic)(nil)

	b, err := format.WrapErrorData("backlogit get", format.ErrCodeInternalError, "oops", typedNil)
	require.NoError(t, err)

	assert.False(t, strings.Contains(string(b), `"data"`),
		"typed-nil must not produce a \"data\" key; got: %s", b)
	assert.False(t, strings.Contains(string(b), "null"),
		"typed-nil must not produce null in output; got: %s", b)
}

// TestU1b_WrapErrorDataNonNilEmbedsObject verifies that a non-nil data payload
// is embedded under the "data" key inside the error object.
func TestU1b_WrapErrorDataNonNilEmbedsObject(t *testing.T) {
	type detail struct {
		Key string `json:"k"`
	}

	b, err := format.WrapErrorData(
		"backlogit classify",
		format.ErrCodeInvalidParams,
		"bad input",
		detail{Key: "v"},
	)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(b, &resp), "response must be valid JSON")

	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok, "response must have an error object")

	dataObj, hasData := errObj["data"]
	require.True(t, hasData, "error object must have a data key; got: %s", b)

	dataMap, ok := dataObj.(map[string]any)
	require.True(t, ok, "data must be an object")
	assert.Equal(t, "v", dataMap["k"], `data["k"] must equal "v"`)
}

// TestU1b_LegacyWrapErrorGoldenUnchanged verifies that adding the Data field
// to JSONRPCError does not alter the byte output of the existing WrapError
// function — backward-compatibility golden test.
func TestU1b_LegacyWrapErrorGoldenUnchanged(t *testing.T) {
	cases := []struct {
		name string
		id   string
		code int
		msg  string
	}{
		{
			name: "invalid_params",
			id:   "backlogit get",
			code: format.ErrCodeInvalidParams,
			msg:  "invalid params",
		},
		{
			name: "internal_error",
			id:   "backlogit sync",
			code: format.ErrCodeInternalError,
			msg:  "internal error",
		},
		{
			name: "method_not_found",
			id:   "backlogit list",
			code: format.ErrCodeMethodNotFound,
			msg:  "method not found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := format.WrapError(tc.id, tc.code, tc.msg)
			require.NoError(t, err)

			// The response must be valid JSON-RPC 2.0 with no "data" key in the
			// error object — confirming omitempty + nil Data = key absent.
			var resp map[string]any
			require.NoError(t, json.Unmarshal(b, &resp))

			errObj, ok := resp["error"].(map[string]any)
			require.True(t, ok, "must have error object")

			_, hasData := errObj["data"]
			assert.False(t, hasData,
				"WrapError output must NOT include a \"data\" key; got: %s", b)

			assert.Equal(t, "2.0", resp["jsonrpc"])
			assert.Equal(t, tc.id, resp["id"])
			assert.Equal(t, float64(tc.code), errObj["code"])
			assert.Equal(t, tc.msg, errObj["message"])
		})
	}
}
