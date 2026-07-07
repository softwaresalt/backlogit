package events

// 086.001-T: focused contract test for the shared ParseEventLine helper — the
// single source of truth for the malformed-JSONL-line skip-with-warning policy
// used by both ReadAllEvents (doctor fallback) and rehydration's
// parseItemLogFile (SQLite projection).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEventLine(t *testing.T) {
	const itemID = "001.001-T"

	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantErr bool
	}{
		{
			name:    "valid event with explicit item id",
			line:    `{"timestamp":"2026-07-07T10:00:00Z","actor":"a","item_id":"001.001-T","event_type":"created","delta":{}}`,
			wantOK:  true,
			wantErr: false,
		},
		{
			name:    "valid event backfills missing item id",
			line:    `{"timestamp":"2026-07-07T10:00:00Z","actor":"a","event_type":"created","delta":{}}`,
			wantOK:  true,
			wantErr: false,
		},
		{
			name:    "blank line skipped silently",
			line:    "",
			wantOK:  false,
			wantErr: false,
		},
		{
			name:    "whitespace-only line skipped silently",
			line:    "   \t ",
			wantOK:  false,
			wantErr: false,
		},
		{
			name:    "malformed json returns error for caller to warn+skip",
			line:    "NOT_VALID_JSON_@@@",
			wantOK:  false,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, ok, err := ParseEventLine(tc.line, itemID)
			assert.Equal(t, tc.wantOK, ok, "ok classification")
			if tc.wantErr {
				require.Error(t, err, "malformed line must surface the parse error to the caller")
			} else {
				require.NoError(t, err, "blank/valid lines must not error")
			}
			if ok {
				assert.Equal(t, itemID, e.ItemID, "item id is present or backfilled on a valid event")
			}
		})
	}
}
