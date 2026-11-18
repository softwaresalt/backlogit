package events

// 086.001-T: focused contract test for the shared ParseEventLine helper — the
// single source of truth for the malformed-JSONL-line skip-with-warning policy
// used by both ReadAllEvents (doctor fallback) and rehydration's
// parseItemLogFile (SQLite projection).

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestU2_ReadAllEventsSmallInitialBufferGrowsToOneMiBMax(t *testing.T) {
	const itemID = "216.002-T"
	ctx := context.Background()

	t.Run("small_log_avoids_eager_one_mib_allocation", func(t *testing.T) {
		logsDir := t.TempDir()
		logPath := LogPathForItem(logsDir, itemID)
		content := strings.Join([]string{
			`{"timestamp":"2026-11-16T09:00:00Z","actor":"agent","item_id":"216.002-T","event_type":"created","delta":{"status":"queued"}}`,
			`{"timestamp":"2026-11-16T09:01:00Z","actor":"agent","item_id":"216.002-T","event_type":"comment","delta":{"comment":"small"}}`,
		}, "\n") + "\n"
		require.NoError(t, os.WriteFile(logPath, []byte(content), 0o644))

		// Warm up before measuring deterministic allocations.
		warm, err := ReadAllEvents(ctx, logsDir, itemID)
		require.NoError(t, err)
		require.Len(t, warm, 2)

		runtime.GC()

		var minAlloc uint64 = math.MaxUint64
		for i := 0; i < 3; i++ {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			got, readErr := ReadAllEvents(ctx, logsDir, itemID)
			runtime.ReadMemStats(&after)
			require.NoError(t, readErr)
			require.Len(t, got, 2)
			delta := after.TotalAlloc - before.TotalAlloc
			if delta < minAlloc {
				minAlloc = delta
			}
		}
		require.Less(t, minAlloc, uint64(256*1024),
			"ReadAllEvents on a 2-line log must allocate < 256 KiB instead of eagerly allocating 1 MiB")
	})

	t.Run("line_larger_than_64kib_grows_and_parses", func(t *testing.T) {
		logsDir := t.TempDir()
		logPath := LogPathForItem(logsDir, itemID)
		payload := strings.Repeat("p", 256*1024)
		ev := Event{
			Timestamp: time.Date(2026, 11, 16, 9, 2, 0, 0, time.UTC),
			Actor:     "agent",
			ItemID:    itemID,
			EventType: "comment",
			Delta:     map[string]any{"comment": payload},
		}
		line, err := json.Marshal(ev)
		require.NoError(t, err)
		require.Greater(t, len(line), 64*1024)
		require.NoError(t, os.WriteFile(logPath, append(line, '\n'), 0o644))

		got, err := ReadAllEvents(ctx, logsDir, itemID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, payload, got[0].Delta["comment"])
	})

	t.Run("oversized_line_fails_closed_with_err_too_long", func(t *testing.T) {
		logsDir := t.TempDir()
		logPath := LogPathForItem(logsDir, itemID)
		oversized := strings.Repeat("x", (1<<20)+1) + "\n"
		require.NoError(t, os.WriteFile(logPath, []byte(oversized), 0o644))

		_, err := ReadAllEvents(ctx, logsDir, itemID)
		require.Error(t, err)
		require.ErrorIs(t, err, bufio.ErrTooLong)
	})

	t.Run("canceled_context_fails_closed", func(t *testing.T) {
		logsDir := t.TempDir()
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		_, err := ReadAllEvents(canceledCtx, logsDir, itemID)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}
