package events_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// readCheckpointFile reads the raw bytes of a checkpoint file written by
// CreateCheckpoint. Shared by the checkpoint context/marshal/create-strict
// test files so each does not redeclare an identical os.ReadFile wrapper.
func readCheckpointFile(t *testing.T, path string) ([]byte, error) {
	t.Helper()
	return os.ReadFile(path)
}

// extractObjectMember walks a chain of nested JSON object keys starting from
// raw and returns the exact raw bytes of the final member, preserving its
// original byte content (including internal key order). It never reshapes
// through a map[string]any, which would lose key order.
func extractObjectMember(t *testing.T, raw []byte, path ...string) []byte {
	t.Helper()
	cur := raw
	for _, key := range path {
		var obj map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(cur, &obj))
		member, ok := obj[key]
		require.True(t, ok, "missing member %q while walking path %v", key, path)
		cur = member
	}
	return cur
}
