package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestDetectDuplicateJSONMembers_MatchesEncodingJSONKeyBinding proves the
// shared walker treats keys as duplicates exactly when encoding/json would
// bind them to the same field, and fails closed on malformed input.
func TestDetectDuplicateJSONMembers_MatchesEncodingJSONKeyBinding(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantError bool
	}{
		{"distinct keys", `{"a":1,"b":{"c":2},"d":[{"e":3},{"e":4}]}`, false},
		{"exact duplicate", `{"a":1,"a":2}`, true},
		{"case only", `{"item_id":1,"Item_ID":2}`, true},
		{"long s alias literal", `{"status":1,"ſtatus":2}`, true},
		{"long s alias escaped", `{"status":1,"\u017ftatus":2}`, true},
		{"kelvin alias literal", `{"k":1,"K":2}`, true},
		{"kelvin alias escaped", `{"k":1,"\u212a":2}`, true},
		{"escaped equals literal", `{"a":1,"\u0061":2}`, true},
		{"nested object", `{"a":{"b":1,"B":2}}`, true},
		{"nested in array", `{"a":[{"b":1},{"b":1,"B":2}]}`, true},
		{"delta nested alias", `{"delta":{"ſ":1,"s":2}}`, true},
		{"same key in sibling objects", `{"a":{"b":1},"c":{"b":2}}`, false},
		{"scalars arrays and null", `{"a":[1,"x",null,[2,{"k":true}]],"b":null}`, false},
		{"malformed nested", `{"a":{"b":1,"c":}}`, true},
		{"truncated", `{"a":{"b":1`, true},
		{"trailing data", `{"a":1} {"b":2}`, true},
		{"trailing garbage", `{"a":1}x`, true},
		{"empty", ``, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := detectDuplicateJSONMembers([]byte(tc.raw))
			if !tc.wantError {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrValidation)
		})
	}
}

func TestRejectDuplicateJSONMembers_EnforcesNestingDepthLimit(t *testing.T) {
	build := func(depth int) []byte {
		return []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))
	}
	require.NoError(t, rejectDuplicateJSONMembers(build(maxJSONNestingDepth)))

	err := rejectDuplicateJSONMembers(build(maxJSONNestingDepth + 1))
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}
