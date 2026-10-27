package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestReadShipmentBlockedSnapshot_DuplicateMemberNamesRejected proves a
// snapshot whose JSON object repeats a member name is rejected before decode:
// encoding/json silently keeps the last value, and matches struct field names
// case-insensitively, so a duplicate could otherwise change the authoritative
// content unnoticed.
func TestReadShipmentBlockedSnapshot_DuplicateMemberNamesRejected(t *testing.T) {
	const shipmentID = "201-S"
	memberIDs := []string{"201.001-T"}
	const head = `"schema_version":"shipment-bootstrap-snapshot/v1","shipment_id":"201-S","target":"blocked",`
	const tail = `"blocked_reason":"test","blocked_at":"2026-01-01T00:00:00Z"`

	tests := []struct {
		name      string
		raw       string
		wantError bool
	}{
		{"valid snapshot decodes", `{` + head + tail + `,"members":{"201.001-T":"queued"}}`, false},
		{"duplicate top-level member", `{` + head + `"shipment_id":"201-S",` + tail + `,"members":{"201.001-T":"queued"}}`, true},
		{"top-level names differing only in case", `{` + head + `"Shipment_ID":"201-S",` + tail + `,"members":{"201.001-T":"queued"}}`, true},
		{"duplicate nested member", `{` + head + tail + `,"members":{"201.001-T":"queued","201.001-T":"done"}}`, true},
		{"nested names differing only in case", `{` + head + tail + `,"members":{"201.001-T":"queued","201.001-t":"done"}}`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			path := filepath.Join(workspaceStorageRoot(ws), "snapshots", "snap.json")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, []byte(tc.raw), 0o644))

			_, err := readShipmentBlockedSnapshot(ws, filepath.Join("snapshots", "snap.json"), shipmentID, memberIDs)
			if !tc.wantError {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrValidation)
		})
	}
}
