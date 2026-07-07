package db

// 086.001-T: Malformed-JSONL-line handling convergence between the SQLite
// rehydration parser (parseItemLogFile) and the doctor-fallback parser
// (events.ReadAllEvents).
//
// These tests lock the unified skip-with-warning policy:
//
//   - a malformed JSON line is skipped (not errored) with a structured
//     slog.Warn carrying the item ID + 1-based line number, on BOTH paths;
//   - blank / whitespace-only lines are skipped silently (no warning);
//   - valid lines are retained and both parsers agree.
//
// The convergence subtest is the true red gate (Principle II): on the
// pre-implementation code parseItemLogFile returns a non-nil error and 0 events
// while events.ReadAllEvents returns 2 events; after the shared
// events.ParseEventLine refactor both return exactly the 2 valid events and a
// nil error. The valid-unaffected and whitespace-only subtests are convergence
// regression guards (they already hold on today's code) rather than red gates.
//
// Test-safety (Go/Constitution review): the observability subtest mutates the
// global slog default via slog.SetDefault and restores it with t.Cleanup, so
// this file must NOT call t.Parallel().

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

const malformedLineTestItemID = "086.001-T"

// validEventLine returns a valid serialized Event JSONL line for the fixture.
func validEventLine(eventType string) string {
	return `{"timestamp":"2026-07-07T10:00:00Z","actor":"tester","item_id":"` +
		malformedLineTestItemID + `","event_type":"` + eventType + `","delta":{}}`
}

// writeItemLog writes the given raw lines (joined with "\n" and a trailing
// newline, matching the append-only log format) to logs/<itemID>.jsonl under a
// fresh temp workspace, returning the logs dir and the full file path so both
// parsers resolve the same file.
func writeItemLog(t *testing.T, lines []string) (logsDir, path string) {
	t.Helper()
	logsDir = filepath.Join(t.TempDir(), "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	path = filepath.Join(logsDir, malformedLineTestItemID+".jsonl")
	content := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return logsDir, path
}

// captureSlog installs a text-handler slog default writing into the returned
// buffer and restores the previous default on cleanup. It mutates the global
// default, so callers must NOT use t.Parallel().
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func TestMalformedJSONLLineHandling_Convergence(t *testing.T) {
	// Scenario 1 — convergence (RED GATE). On today's code parseItemLogFile
	// errors with 0 events while ReadAllEvents skips and returns 2; after the
	// shared-helper refactor both return exactly the 2 valid events, nil error.
	t.Run("convergence_malformed_line_skipped_both_paths", func(t *testing.T) {
		lines := []string{
			validEventLine("created"),
			"NOT_VALID_JSON_@@@",
			"   ",
			validEventLine("updated"),
		}
		logsDir, path := writeItemLog(t, lines)

		rehydrated, rehErr := parseItemLogFile(path, malformedLineTestItemID)
		require.NoError(t, rehErr, "parseItemLogFile must skip the malformed line, not error")

		doctor, docErr := events.ReadAllEvents(context.Background(), logsDir, malformedLineTestItemID)
		require.NoError(t, docErr, "ReadAllEvents must skip the malformed line, not error")

		assert.Len(t, rehydrated, 2, "rehydration retains the 2 valid events")
		assert.Len(t, doctor, 2, "doctor retains the 2 valid events")

		// Both parsers agree: same count and same per-event identity.
		require.Equal(t, len(doctor), len(rehydrated), "both parsers return the same event count")
		for i := range rehydrated {
			assert.Equal(t, doctor[i].EventType, rehydrated[i].EventType, "event %d type agrees", i)
			assert.Equal(t, doctor[i].ItemID, rehydrated[i].ItemID, "event %d item agrees", i)
		}
	})

	// Scenario 2 — observability: the malformed skip is warned with the item ID
	// and the 1-based line number on BOTH paths (closes the pre-existing silent
	// skip in ReadAllEvents).
	t.Run("observability_malformed_skip_warns_with_item_and_line", func(t *testing.T) {
		lines := []string{
			validEventLine("created"),
			"NOT_VALID_JSON_@@@",
			"   ",
			validEventLine("updated"),
		}
		logsDir, path := writeItemLog(t, lines)

		rehBuf := captureSlog(t)
		_, rehErr := parseItemLogFile(path, malformedLineTestItemID)
		require.NoError(t, rehErr)
		rehLog := rehBuf.String()
		assert.Contains(t, rehLog, "level=WARN", "rehydration emits a warning for the malformed line")
		assert.Contains(t, rehLog, malformedLineTestItemID, "rehydration warning carries the item ID")
		assert.Contains(t, rehLog, "line=2", "rehydration warning carries the 1-based malformed line number")

		docBuf := captureSlog(t)
		_, docErr := events.ReadAllEvents(context.Background(), logsDir, malformedLineTestItemID)
		require.NoError(t, docErr)
		docLog := docBuf.String()
		assert.Contains(t, docLog, "level=WARN", "doctor emits a warning for the malformed line")
		assert.Contains(t, docLog, malformedLineTestItemID, "doctor warning carries the item ID")
		assert.Contains(t, docLog, "line=2", "doctor warning carries the 1-based malformed line number")
	})

	// Scenario 3 — valid lines unaffected: all-valid fixture returns every event
	// with a nil error and no malformed-skip warning from either parser.
	t.Run("valid_lines_unaffected_no_warning", func(t *testing.T) {
		lines := []string{
			validEventLine("created"),
			validEventLine("updated"),
			validEventLine("done"),
		}
		logsDir, path := writeItemLog(t, lines)

		rehBuf := captureSlog(t)
		rehydrated, rehErr := parseItemLogFile(path, malformedLineTestItemID)
		require.NoError(t, rehErr)
		assert.Len(t, rehydrated, 3, "all valid rehydration events retained")

		docBuf := captureSlog(t)
		doctor, docErr := events.ReadAllEvents(context.Background(), logsDir, malformedLineTestItemID)
		require.NoError(t, docErr)
		assert.Len(t, doctor, 3, "all valid doctor events retained")

		assert.NotContains(t, rehBuf.String(), "skipping malformed", "no malformed warning for all-valid rehydration")
		assert.NotContains(t, docBuf.String(), "skipping malformed", "no malformed warning for all-valid doctor read")
	})

	// Scenario 4 — whitespace-only convergence lock: a whitespace-only line is
	// skipped SILENTLY (no warning) on both paths, and the surrounding valid
	// events are retained. Guards the CRLF/blank convergence.
	t.Run("whitespace_only_line_skipped_silently_both_paths", func(t *testing.T) {
		lines := []string{
			validEventLine("created"),
			"   \t  ",
			validEventLine("updated"),
		}
		logsDir, path := writeItemLog(t, lines)

		rehBuf := captureSlog(t)
		rehydrated, rehErr := parseItemLogFile(path, malformedLineTestItemID)
		require.NoError(t, rehErr)
		assert.Len(t, rehydrated, 2, "whitespace-only line skipped, valid rehydration events retained")

		docBuf := captureSlog(t)
		doctor, docErr := events.ReadAllEvents(context.Background(), logsDir, malformedLineTestItemID)
		require.NoError(t, docErr)
		assert.Len(t, doctor, 2, "whitespace-only line skipped, valid doctor events retained")

		assert.NotContains(t, rehBuf.String(), "skipping malformed", "whitespace-only line must not warn (rehydration)")
		assert.NotContains(t, docBuf.String(), "skipping malformed", "whitespace-only line must not warn (doctor)")
	})
}
