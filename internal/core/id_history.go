package core

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/softwaresalt/backlogit/internal/config"
)

// historicalIDQueries lists the SQLite sources whose IDs must be reserved so an
// allocator never reuses an ID that was retired (deleted, archived, or
// adopted away) but is still recorded in history. Each query yields one
// nullable TEXT column.
var historicalIDQueries = []string{
	`SELECT item_id FROM item_logs`,
	`SELECT DISTINCT item_id FROM item_log_entries`,
	`SELECT DISTINCT json_extract(delta_json, '$.old_id') FROM item_log_entries WHERE delta_json LIKE '%old_id%' AND json_valid(delta_json)`,
	`SELECT DISTINCT item_id FROM stash_links`,
}

// historicalIDsFromDB returns IDs recorded in SQLite history tables.
func historicalIDsFromDB(ctx context.Context, db *sql.DB) (ids []string, err error) {
	if db == nil {
		return nil, nil
	}
	for _, query := range historicalIDQueries {
		collected, qErr := collectNullableStrings(ctx, db, query)
		if qErr != nil {
			return nil, qErr
		}
		ids = append(ids, collected...)
	}
	return ids, nil
}

func collectNullableStrings(ctx context.Context, db *sql.DB, query string) (out []string, err error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		// Minimal databases (for example hand-built test indexes) may lack the
		// history tables; absence of a table means there is no history to reserve.
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("query historical ids: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close historical id rows: %w", cerr))
		}
	}()
	for rows.Next() {
		var value sql.NullString
		if scanErr := rows.Scan(&value); scanErr != nil {
			return nil, fmt.Errorf("scan historical id: %w", scanErr)
		}
		if value.Valid && value.String != "" {
			out = append(out, value.String)
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate historical ids: %w", rowsErr)
	}
	return out, nil
}

// historicalIDsFromDisk returns every artifact ID visible in the workspace
// files: queue/archive Markdown file names and frontmatter ids, log file
// names, event item_id and delta.old_id values, the stash archive, and the
// provenance corrections file.
func historicalIDsFromDisk(ws *Workspace) ([]string, error) {
	if ws == nil {
		return nil, nil
	}
	storageRoot := workspaceStorageRoot(ws)
	var ids []string

	dirSet := map[string]bool{}
	var dirs []string
	if searchDirs, err := artifactSearchDirs(ws); err == nil {
		for _, d := range searchDirs {
			dirSet[filepath.Clean(d)] = true
			dirs = append(dirs, d)
		}
	}
	archiveDir := filepath.Join(storageRoot, "archive")
	if !dirSet[filepath.Clean(archiveDir)] {
		dirs = append(dirs, archiveDir)
	}
	queueDir := filepath.Join(storageRoot, "queue")
	if !dirSet[filepath.Clean(queueDir)] {
		dirs = append(dirs, queueDir)
	}
	for _, dir := range dirs {
		found, err := markdownIDsInDir(dir)
		if err != nil {
			return nil, err
		}
		ids = append(ids, found...)
	}

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	entries, err := os.ReadDir(logsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read logs dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".jsonl"))
		found, readErr := eventLogIDs(filepath.Join(logsDir, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		ids = append(ids, found...)
	}

	stashIDs, err := jsonlStringFields(filepath.Join(archiveDir, stashArchiveFileName),
		"harvested_artifact_id")
	if err != nil {
		return nil, err
	}
	ids = append(ids, stashIDs...)
	correctionIDs, err := jsonlStringFields(filepath.Join(archiveDir, "provenance_corrections.jsonl"),
		"historical_artifact_id", "canonical_delivery_artifact_id")
	if err != nil {
		return nil, err
	}
	ids = append(ids, correctionIDs...)
	return ids, nil
}

func markdownIDsInDir(dir string) ([]string, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", dir, err)
	}
	var ids []string
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		ids = append(ids, strings.TrimSuffix(d.Name(), ".md"))
		if id := frontmatterIDLine(path); id != "" {
			ids = append(ids, id)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s for historical ids: %w", dir, walkErr)
	}
	return ids, nil
}

// frontmatterIDLine returns the top-level id: value from a Markdown file's
// frontmatter without parsing the whole document, or "" when absent.
func frontmatterIDLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			if strings.TrimSpace(line) != "---" {
				return ""
			}
			continue
		}
		if strings.TrimSpace(line) == "---" {
			return ""
		}
		if value, ok := strings.CutPrefix(line, "id:"); ok {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}

// forEachJSONLine calls fn with every non-empty line of a JSONL file. A
// missing file yields no calls; a symlinked file is refused.
func forEachJSONLine(path string, fn func(line []byte)) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%q is a symlink; refusing to read through it", path)
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			fn(trimmed)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return fmt.Errorf("read %s: %w", path, readErr)
		}
	}
}

func eventLogIDs(path string) ([]string, error) {
	var ids []string
	err := forEachJSONLine(path, func(line []byte) {
		var rec struct {
			ItemID string         `json:"item_id"`
			Delta  map[string]any `json:"delta"`
		}
		if json.Unmarshal(line, &rec) != nil {
			return
		}
		if rec.ItemID != "" {
			ids = append(ids, rec.ItemID)
		}
		for _, key := range []string{"old_id", "new_id"} {
			if v, ok := rec.Delta[key].(string); ok && v != "" {
				ids = append(ids, v)
			}
		}
	})
	return ids, err
}

func jsonlStringFields(path string, fields ...string) ([]string, error) {
	var ids []string
	err := forEachJSONLine(path, func(line []byte) {
		var rec map[string]any
		if json.Unmarshal(line, &rec) != nil {
			return
		}
		for _, field := range fields {
			if v, ok := rec[field].(string); ok && v != "" {
				ids = append(ids, v)
			}
		}
	})
	return ids, err
}

// maxTypedOrdinalAmong returns the highest ordinal among ids that sit directly
// under parentPath (empty for root level) and use the given type affixes.
func maxTypedOrdinalAmong(ids []string, parentPath, prefix, suffix string) int {
	maxOrdinal := 0
	for _, id := range ids {
		idParent := ""
		if idx := strings.LastIndex(id, "."); idx >= 0 {
			idParent = id[:idx]
		}
		if idParent != parentPath {
			continue
		}
		if ordinal, ok := typedSegmentOrdinal(lastIDSegment(id), prefix, suffix); ok && ordinal > maxOrdinal {
			maxOrdinal = ordinal
		}
	}
	return maxOrdinal
}

// maxStandaloneOrdinalAmong returns the highest ordinal among ids whose last
// segment strictly matches the type affixes. Types without any affix are
// ignored because their numeric IDs cannot be attributed to the type.
func maxStandaloneOrdinalAmong(ids []string, typeCfg *config.ArtifactTypeConfig) int {
	if typeCfg == nil || (typeCfg.Prefix == "" && typeCfg.Suffix == "") {
		return 0
	}
	maxOrdinal := 0
	for _, id := range ids {
		if ordinal, ok := historicalStandaloneOrdinal(lastIDSegment(id), typeCfg.Prefix, typeCfg.Suffix); ok && ordinal > maxOrdinal {
			maxOrdinal = ordinal
		}
	}
	return maxOrdinal
}

// historicalStandaloneOrdinal parses a standalone ID segment the way NextID's
// naming format produces it: the strict typed form ("001-T" / "T001") or the
// slugged form ("001-T-implement-jwt" / "T001-implement-jwt"). The type
// affix must match exactly and be followed by the end of the segment or a
// "-" slug separator, so IDs of other types are never attributed to this type.
func historicalStandaloneOrdinal(segment, prefix, suffix string) (int, bool) {
	if ordinal, ok := typedSegmentOrdinal(segment, prefix, suffix); ok {
		return ordinal, true
	}
	var digitsAndRest string
	var rest string
	if suffix != "" {
		digitsAndRest = segment
		digits := leadingDigits(digitsAndRest)
		if digits == "" {
			return 0, false
		}
		rest = strings.TrimPrefix(digitsAndRest, digits)
		if !strings.HasPrefix(rest, suffix) {
			return 0, false
		}
		rest = strings.TrimPrefix(rest, suffix)
		if !strings.HasPrefix(rest, "-") {
			return 0, false
		}
		n, err := strconv.Atoi(digits)
		return n, err == nil
	}
	if !strings.HasPrefix(segment, prefix) {
		return 0, false
	}
	digitsAndRest = strings.TrimPrefix(segment, prefix)
	digits := leadingDigits(digitsAndRest)
	if digits == "" || !strings.HasPrefix(strings.TrimPrefix(digitsAndRest, digits), "-") {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	return n, err == nil
}

// MaxDiskArtifactSequence returns the highest ordinal recorded for the
// artifact type anywhere in workspace disk history (queue, archive, logs,
// stash archive, and provenance corrections).
func MaxDiskArtifactSequence(ws *Workspace, typeCfg *config.ArtifactTypeConfig) (int, error) {
	ids, err := historicalIDsFromDisk(ws)
	if err != nil {
		return 0, err
	}
	return maxStandaloneOrdinalAmong(ids, typeCfg), nil
}

// errIDHistoryScan marks a failure to read workspace ID history; callers must
// not fall back to an ID that could collide with retired history.
var errIDHistoryScan = errors.New("scan id history")

// allocateTypedHierarchicalID is NextTypedHierarchicalID extended with
// workspace disk history so retired IDs are never reused.
func allocateTypedHierarchicalID(
	ctx context.Context,
	ws *Workspace,
	parentID, artifactType string,
	typeCfg *config.ArtifactTypeConfig,
	layout *config.QueueLayoutConfig,
) (string, error) {
	diskIDs, err := historicalIDsFromDisk(ws)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errIDHistoryScan, err)
	}
	return nextTypedHierarchicalID(ctx, ws.DB, diskIDs, parentID, artifactType, typeCfg, layout)
}
