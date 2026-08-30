package db

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/softwaresalt/backlogit/internal/stash"
)

// rehydrateBatchSize is the number of artifacts inserted per transaction during
// the batch-rebuild phase. Smaller batches hold the write lock for less time,
// reducing contention under concurrent MCP workloads.
const rehydrateBatchSize = 100

// RehydrateOption configures a Rehydrate call.
type RehydrateOption func(*rehydrateConfig)

// rehydrateConfig holds the resolved settings for a single Rehydrate call.
type rehydrateConfig struct {
	logger *slog.Logger
}

// WithLogger injects the *slog.Logger that Rehydrate and its duplicate-source
// warnings write to (070.002-T). Injecting a logger lets tests capture log
// output without mutating the global slog default via slog.SetDefault. When
// omitted, Rehydrate logs to slog.Default(), preserving prior behavior.
func WithLogger(logger *slog.Logger) RehydrateOption {
	return func(c *rehydrateConfig) { c.logger = logger }
}

// newRehydrateConfig applies the given options and resolves an effective logger,
// defaulting to slog.Default() when none is injected.
func newRehydrateConfig(opts ...RehydrateOption) rehydrateConfig {
	cfg := rehydrateConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.logger == nil {
		cfg.logger = slog.Default()
	}
	return cfg
}

// collectedArtifact holds a parsed artifact ready for batch insertion.
type collectedArtifact struct {
	artifact *models.Artifact
}

// warnOnDuplicateSourceIDs emits exactly one warning per artifact ID that is
// declared by two or more source files (066.004-T). Duplicate source files --
// most acutely the same root ID present in both the queue and the archive (the
// 066-F bug) -- are otherwise masked by the PK-keyed upsert, which silently
// collapses them to a single indexed row. The warning is observational only:
// it does not change the rebuild work or the collapse result. IDs are reported
// in sorted order for deterministic output. The logger is injected (070.002-T)
// so callers/tests can capture output without mutating the global slog default.
func warnOnDuplicateSourceIDs(logger *slog.Logger, idToPaths map[string][]string) {
	if logger == nil {
		logger = slog.Default()
	}
	ids := make([]string, 0, len(idToPaths))
	for id, paths := range idToPaths {
		if len(paths) >= 2 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		paths := append([]string(nil), idToPaths[id]...)
		sort.Strings(paths)
		logger.Warn("rehydrate: duplicate source id detected; sources collapse to a single indexed row",
			"id", id, "paths", paths)
	}
}

// Rehydrate walks the workspace directory tree and rebuilds the SQLite index
// from the Markdown source files. Files that fail to parse are skipped with a
// debug log entry. Returns the number of artifacts successfully indexed.
//
// The rebuild is split into three phases to reduce write-lock hold time:
//
//  1. Collect: walk the filesystem and parse all Markdown files into memory.
//     No database interaction occurs during this phase.
//  2. Clear: a single IMMEDIATE transaction deletes all existing index rows so
//     that removed or renamed Markdown files do not leave ghost entries.
//  3. Batch-insert: parsed artifacts are inserted in batches of
//     rehydrateBatchSize per transaction. Each batch acquires and releases the
//     write lock independently, allowing concurrent readers to make progress
//     between batches.
//
// Note: between the clear commit and the final batch commit the index is empty
// or partially populated. This is acceptable because backlogit.db is an
// ephemeral cache that can be rebuilt at any time.
func Rehydrate(ctx context.Context, workspacePath string, db *sql.DB, opts ...RehydrateOption) (int, error) {
	logger := newRehydrateConfig(opts...).logger
	harvestedStash := make(map[string]StashRecord)

	// ── Phase 1: Collect ──────────────────────────────────────────────────────
	var collected []collectedArtifact
	// 066.004-T: Track every source file per ID so duplicate source files (e.g.
	// the same root ID present in both queue and archive -- the 066-F bug) can be
	// surfaced as a warning. This is purely observational: it does not change the
	// clear+rebuild work below or the PK-keyed upsert that collapses duplicates
	// to a single indexed row.
	idToPaths := make(map[string][]string)
	if walkErr := filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			logger.Debug("walk error, skipping", "path", path, "error", walkErr)
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), ".stash.md") {
			return nil
		}

		artifact, parseErr := parseMarkdownArtifact(path)
		if parseErr != nil {
			logger.Debug("skipping unparseable file", "path", path, "error", parseErr)
			return nil
		}
		if artifact == nil {
			return nil
		}

		collected = append(collected, collectedArtifact{artifact: artifact})
		idToPaths[artifact.ID] = append(idToPaths[artifact.ID], path)
		if record, ok := stashRecordFromArtifact(artifact); ok {
			harvestedStash[record.ID] = record
		}
		return nil
	}); walkErr != nil {
		return 0, fmt.Errorf("rehydrate walk: %w", walkErr)
	}

	// 066.004-T: Emit exactly one warning per duplicated source ID before any
	// database work starts. The clear + batch-insert rebuild below (a clear
	// transaction followed by batched insert transactions) is left untouched;
	// this warning is observational only and does not change rebuild behavior.
	warnOnDuplicateSourceIDs(logger, idToPaths)

	// ── Phase 2: Clear ────────────────────────────────────────────────────────
	if clearErr := RetryWrite(ctx, func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin clear transaction: %w", err)
		}
		defer tx.Rollback() //nolint:errcheck

		if err := deleteAllItemLogs(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM items`); err != nil {
			return fmt.Errorf("clear items: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_deps`); err != nil {
			return fmt.Errorf("clear item_deps: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_links`); err != nil {
			return fmt.Errorf("clear item_links: %w", err)
		}
		return tx.Commit()
	}); clearErr != nil {
		return 0, clearErr
	}

	// ── Phase 3: Batch-insert ─────────────────────────────────────────────────
	count := 0
	for i := 0; i < len(collected); i += rehydrateBatchSize {
		end := i + rehydrateBatchSize
		if end > len(collected) {
			end = len(collected)
		}
		batch := collected[i:end]

		batchCount := 0
		batchErr := RetryWrite(ctx, func() error {
			// Reset batchCount at the start of each attempt so that a retried
			// transaction does not double-count rows inserted in the failed attempt.
			batchCount = 0
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("begin batch transaction at offset %d: %w", i, err)
			}
			defer tx.Rollback() //nolint:errcheck

			for _, ca := range batch {
				artifact := ca.artifact
				if upsertErr := upsertItemTx(ctx, tx, artifact); upsertErr != nil {
					logger.Warn("failed to upsert artifact", "id", artifact.ID, "error", upsertErr)
					continue
				}

				if artifact.Level == 0 && isHierarchicalID(artifact.ID) {
					level := strings.Count(artifact.ID, ".") + 1
					hierarchyPath := hierarchyPathFromID(artifact.ID)
					if _, execErr := tx.ExecContext(ctx,
						`UPDATE items SET level = ?, hierarchy_path = ? WHERE id = ?`,
						level, hierarchyPath, artifact.ID,
					); execErr != nil {
						logger.Warn("failed to set level/hierarchy_path", "id", artifact.ID, "error", execErr)
					}
				}

				for _, dep := range artifact.Dependencies {
					if dep.ID == "" {
						continue
					}
					depType := dep.Type
					if depType == "" {
						depType = "blocks"
					}
					if depErr := upsertDependencyTx(ctx, tx, artifact.ID, dep.ID, depType); depErr != nil {
						logger.Warn("failed to upsert dependency", "item_id", artifact.ID, "dep_id", dep.ID, "error", depErr)
					}
				}
				for _, link := range artifact.Links {
					if strings.TrimSpace(link.TargetID) == "" || strings.TrimSpace(link.LinkType) == "" {
						continue
					}
					if !isValidLinkType(link.LinkType) {
						logger.Warn("rehydration: skipping invalid link_type",
							"source_id", artifact.ID, "target_id", link.TargetID, "link_type", link.LinkType)
						continue
					}
					if _, execErr := tx.ExecContext(ctx,
						`INSERT OR IGNORE INTO item_links (source_id, target_id, link_type) VALUES (?, ?, ?)`,
						artifact.ID, link.TargetID, link.LinkType,
					); execErr != nil {
						logger.Warn("failed to upsert link", "source_id", artifact.ID, "target_id", link.TargetID, "link_type", link.LinkType, "error", execErr)
					}
				}

				batchCount++
			}

			return tx.Commit()
		})
		if batchErr != nil {
			return count, fmt.Errorf("rehydration batch at offset %d: %w", i, batchErr)
		}
		count += batchCount
	}

	stashCount, stashErr := rehydrateStash(ctx, workspacePath, db, harvestedStash)
	if stashErr != nil {
		return count, stashErr
	}
	count += stashCount

	itemEvents, err := rehydrateItemLogs(ctx, workspacePath, db)
	if err != nil {
		return count, err
	}

	// Q3.2 (083.005.003-ST): rebuild the derived gate_evidence projection from the
	// same per-item events rehydrateItemLogs already parsed — no second log walk.
	// Disposable read-model; logs stay source of truth.
	if err := rehydrateGateEvidence(ctx, db, itemEvents); err != nil {
		return count, err
	}

	return count, nil
}

// parseMarkdownArtifact reads a Markdown file and extracts the artifact using
// the models layer directly, avoiding an import of the parser package (which
// would create an import cycle through core).
func parseMarkdownArtifact(path string) (*models.Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	fm, body, err := models.ParseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
	}
	if fm == nil {
		return nil, nil
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err != nil {
		return nil, fmt.Errorf("artifact from frontmatter %s: %w", path, err)
	}

	return artifact, nil
}

func upsertDependencyTx(ctx context.Context, tx *sql.Tx, itemID, dependsOn, depType string) error {
	if depType == "" {
		depType = "blocks"
	}
	_, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, ?)`,
		itemID, dependsOn, depType,
	)
	return err
}

// isHierarchicalID reports whether an ID uses the dot-separated numeric hierarchy
// format (e.g., "001", "001.002", "001.002.003"). Non-hierarchical IDs such as
// "T001" or "BUG-3" are excluded.
func isHierarchicalID(id string) bool {
	if id == "" {
		return false
	}
	for _, seg := range strings.Split(id, ".") {
		if seg == "" {
			return false
		}
		numeric := leadingDigits(seg)
		if numeric == "" {
			return false
		}
		suffix := strings.TrimPrefix(seg, numeric)
		if suffix == "" {
			continue
		}
		if !strings.HasPrefix(suffix, "-") {
			return false
		}
		for _, ch := range strings.TrimPrefix(suffix, "-") {
			if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
				return false
			}
		}
	}
	return true
}

// hierarchyPathFromID builds the ancestor path for a hierarchical ID.
// "001.002.003" → "001/001.002/001.002.003".
func hierarchyPathFromID(id string) string {
	parts := strings.Split(id, ".")
	numericParts := make([]string, len(parts))
	for i := range parts {
		numericParts[i] = leadingDigits(parts[i])
	}
	segments := make([]string, len(parts))
	for i := range numericParts {
		segments[i] = strings.Join(numericParts[:i+1], ".")
	}
	return strings.Join(segments, "/")
}

func leadingDigits(value string) string {
	var digits strings.Builder
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			break
		}
		digits.WriteRune(ch)
	}
	return digits.String()
}

func rehydrateItemLogs(ctx context.Context, workspacePath string, database *sql.DB) (map[string][]events.Event, error) {
	logsDir := filepath.Join(workspacePath, "logs")
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return nil, nil
	}

	// Collect all log events first, then index them in a single transaction.
	// Without batching, each IndexEvent call auto-commits with a disk sync,
	// causing O(n) fsyncs that make rehydration extremely slow on workspaces
	// with hundreds of log entries. The per-item event map is returned so the
	// caller can rebuild derived projections (e.g. gate_evidence) from these
	// already-parsed events instead of walking and re-parsing every log file.
	type logEvent struct {
		event events.Event
		path  string
	}
	var allEvents []logEvent
	perItem := make(map[string][]events.Event)

	if walkErr := filepath.WalkDir(logsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Debug("log walk error, skipping", "path", path, "error", walkErr)
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		itemID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		eventsForItem, err := parseItemLogFile(path, itemID)
		if err != nil {
			slog.Warn("failed to parse item log", "path", path, "error", err)
			return nil
		}
		perItem[itemID] = append(perItem[itemID], eventsForItem...)
		for _, event := range eventsForItem {
			allEvents = append(allEvents, logEvent{event: event, path: path})
		}
		return nil
	}); walkErr != nil {
		return nil, walkErr
	}

	if len(allEvents) == 0 {
		return perItem, nil
	}

	if err := RetryWrite(ctx, func() error {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin log rehydration transaction: %w", err)
		}
		defer tx.Rollback() //nolint:errcheck

		for _, le := range allEvents {
			if err := indexEventTx(ctx, tx, logsDir, le.event); err != nil {
				slog.Warn("failed to index item log event", "item_id", le.event.ItemID, "path", le.path, "error", err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit log rehydration: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return perItem, nil
}

// rehydrateGateEvidence rebuilds the derived gate_evidence projection (Q3.2 /
// 083.005.003-ST) from the per-item event map already parsed by
// rehydrateItemLogs — it does not re-walk or re-parse the JSONL logs. For every
// item whose log contains at least one gate-family event, it computes the shared
// Q3.0 predicate (gateevidence.Latest) and writes one row: gate_status token plus
// the evidence and head SHAs — nothing else (no report JSON/stderr/force_reason).
// Items that never went through the gate are not indexed. The table is cleared
// and fully repopulated each call so it inherits Rehydrate's idempotency; the
// item log JSONL is never mutated.
func rehydrateGateEvidence(ctx context.Context, database *sql.DB, itemEvents map[string][]events.Event) error {
	type projectedRow struct {
		itemID string
		ev     gateevidence.Evidence
	}
	var rows []projectedRow

	for itemID, evs := range itemEvents {
		gated := false
		for i := range evs {
			if gateevidence.IsGateEvent(evs[i].EventType) {
				gated = true
				break
			}
		}
		if !gated {
			continue
		}
		rows = append(rows, projectedRow{itemID: itemID, ev: gateevidence.Latest(evs)})
	}

	// Clear-and-rebuild in a single transaction so the projection is a pure
	// function of the current logs (fully disposable read-model).
	return RetryWrite(ctx, func() error {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin gate-evidence rehydration transaction: %w", err)
		}
		defer tx.Rollback() //nolint:errcheck

		if _, err := tx.ExecContext(ctx, `DELETE FROM gate_evidence`); err != nil {
			return fmt.Errorf("clear gate_evidence: %w", err)
		}
		for _, r := range rows {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO gate_evidence (item_id, gate_status, evidence_sha, head_sha) VALUES (?, ?, ?, ?)`,
				r.itemID, r.ev.Status, r.ev.EvidenceSHA, r.ev.HeadSHA,
			); err != nil {
				// Fail the whole rebuild: the deferred Rollback aborts the
				// transaction so the projection is never left partially written.
				// The disposable read-model is rebuilt in full on the next sync.
				return fmt.Errorf("upsert gate_evidence row for %s: %w", r.itemID, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit gate-evidence rehydration: %w", err)
		}
		return nil
	})
}

func parseItemLogFile(path, itemID string) ([]events.Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read item log %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	result := make([]events.Event, 0, len(lines))
	for i, line := range lines {
		event, ok, perr := events.ParseEventLine(line, itemID)
		if perr != nil {
			slog.Warn("skipping malformed event log line",
				"item_id", itemID, "path", path, "line", i+1, "error", perr)
			continue
		}
		if !ok {
			continue
		}
		result = append(result, event)
	}
	return result, nil
}

func rehydrateStash(ctx context.Context, workspacePath string, database *sql.DB, harvested map[string]StashRecord) (int, error) {
	stashPath := filepath.Join(workspacePath, "queue", stash.FileName)
	jsonlPath := filepath.Join(workspacePath, stash.JSONLFileName)

	activeEntries := []StashRecord{}
	activeIndex := make(map[string]int)
	appendActive := func(entry StashRecord) {
		if idx, exists := activeIndex[entry.ID]; exists {
			activeEntries[idx] = entry
			return
		}
		activeIndex[entry.ID] = len(activeEntries)
		activeEntries = append(activeEntries, entry)
	}
	now := time.Now().UTC()

	// Attempt to read stash.jsonl. When it contains at least one entry the
	// workspace is considered migrated and the file is used exclusively.
	// An empty stash.jsonl (e.g. created by backlogit init on a legacy
	// workspace) is treated as absent so that .stash.md entries are not
	// silently lost.
	var jsonlEntries []stash.Entry
	if _, statErr := os.Stat(jsonlPath); statErr == nil {
		f, openErr := os.Open(jsonlPath)
		if openErr != nil {
			return 0, fmt.Errorf("open stash jsonl: %w", openErr)
		}
		var readErr error
		jsonlEntries, readErr = stash.ReadJSONL(f)
		closeErr := f.Close()
		if readErr != nil {
			return 0, fmt.Errorf("read stash jsonl: %w", readErr)
		}
		if closeErr != nil {
			return 0, fmt.Errorf("close stash jsonl: %w", closeErr)
		}
	} else if !os.IsNotExist(statErr) {
		return 0, fmt.Errorf("stat stash jsonl: %w", statErr)
	}

	if len(jsonlEntries) > 0 {
		// Migrated workspace: stash.jsonl has entries; use it exclusively.
		if _, err := os.Stat(stashPath); err == nil {
			slog.Debug("skipping legacy stash.md: stash.jsonl has entries", "workspace", workspacePath)
		}
		for _, e := range jsonlEntries {
			appendActive(activeStashRecord(e, filepath.ToSlash(stash.JSONLFileName), now))
		}
		slog.Debug("indexed stash from jsonl", "count", len(jsonlEntries))
	} else {
		// Legacy or fresh workspace: stash.jsonl absent or empty — read .stash.md.
		if _, err := os.Stat(stashPath); err == nil {
			_, entries, parseErr := stash.ParseFile(stashPath)
			if parseErr != nil {
				return 0, fmt.Errorf("parse stash file: %w", parseErr)
			}
			legacySource := filepath.ToSlash(filepath.Join("queue", stash.FileName))
			for _, entry := range entries {
				appendActive(activeStashRecord(entry, legacySource, now))
			}
		} else if !os.IsNotExist(err) {
			return 0, fmt.Errorf("stat stash file: %w", err)
		}
	}

	if err := RehydrateStashIndex(ctx, database, activeEntries, harvested); err != nil {
		return 0, err
	}

	// Apply provenance corrections: read provenance_corrections.jsonl and
	// override stash_links for any corrected entries so that the canonical
	// delivery artifact is returned instead of the historical harvest.
	correctionsPath := filepath.Join(workspacePath, "archive", "provenance_corrections.jsonl")
	if err := applyProvenanceCorrections(ctx, database, correctionsPath); err != nil {
		// Non-fatal: log the error but do not fail the rehydration. The
		// stash_links index remains usable; only the correction overrides
		// are missing. The operator can re-sync after inspecting the file.
		slog.WarnContext(ctx, "apply provenance corrections: partial rehydration",
			"path", correctionsPath, "error", err)
	}
	return len(activeEntries), nil
}

func activeStashRecord(entry stash.Entry, sourcePath string, updatedAt time.Time) StashRecord {
	return StashRecord{
		ID:             entry.ID,
		Priority:       entry.Priority,
		Kind:           entry.Kind,
		Text:           entry.Text,
		DeliberationID: entry.DeliberationID,
		State:          "active",
		SourcePath:     sourcePath,
		UpdatedAt:      updatedAt,
	}
}

func stashRecordFromArtifact(artifact *models.Artifact) (StashRecord, bool) {
	if artifact == nil || artifact.CustomFields == nil {
		return StashRecord{}, false
	}
	stashID, _ := artifact.CustomFields["source_stash_id"].(string)
	if stashID == "" {
		return StashRecord{}, false
	}
	kind, _ := artifact.CustomFields["source_stash_kind"].(string)
	text, _ := artifact.CustomFields["source_stash_text"].(string)
	record := StashRecord{
		ID:             stashID,
		Priority:       stash.DefaultPriority,
		Kind:           kind,
		Text:           text,
		State:          "harvested",
		SourcePath:     filepath.ToSlash(stash.JSONLFileName),
		ItemID:         artifact.ID,
		UpdatedAt:      artifact.UpdatedAt,
		DeliberationID: "",
	}
	if priority, _ := artifact.CustomFields["source_stash_priority"].(string); priority != "" {
		record.Priority = priority
	}
	if sourcePath, _ := artifact.CustomFields["source_stash_path"].(string); strings.TrimSpace(sourcePath) != "" {
		record.SourcePath = filepath.ToSlash(strings.TrimSpace(sourcePath))
	}
	if deliberationID, _ := artifact.CustomFields["source_deliberation_id"].(string); deliberationID != "" {
		record.DeliberationID = deliberationID
	}
	linkedAt := artifact.UpdatedAt
	record.LinkedAt = &linkedAt
	return record, true
}

// provenanceCorrection is the on-disk record written by core.CorrectStashProvenance.
// Redeclared here to avoid a db->core import cycle.
type provenanceCorrection struct {
	StashID                     string `json:"stash_id"`
	HistoricalArtifactID        string `json:"historical_artifact_id"`
	CanonicalDeliveryArtifactID string `json:"canonical_delivery_artifact_id"`
	Reason                      string `json:"reason"`
	Actor                       string `json:"actor"`
	CorrectedAt                 string `json:"corrected_at"`
	EventType                   string `json:"event_type"`
}

// applyProvenanceCorrections reads provenance_corrections.jsonl and updates
// the stash_links table so that corrected stash entries resolve to their
// canonical delivery artifact rather than the historical auto-harvest target.
// A missing corrections file is not an error. A malformed line causes the
// function to return an error (fail-closed), consistent with
// readProvenanceCorrections in stash_provenance.go (thread iV).
func applyProvenanceCorrections(ctx context.Context, database *sql.DB, correctionsPath string) error {
	f, err := os.Open(correctionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open provenance corrections: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Collect the last correction per stash_id (later entries override earlier
	// ones so a sequence of corrections converges to the final canonical state).
	final := make(map[string]provenanceCorrection)
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var c provenanceCorrection
		if jsonErr := json.Unmarshal(raw, &c); jsonErr != nil {
			return fmt.Errorf("provenance corrections line %d: malformed JSON: %w", lineNum, jsonErr)
		}
		if c.StashID != "" && c.CanonicalDeliveryArtifactID != "" {
			final[c.StashID] = c
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return fmt.Errorf("scan provenance corrections: %w", scanErr)
	}

	if len(final) == 0 {
		return nil
	}

	// Apply corrections inside a single transaction.
	return RetryWrite(ctx, func() error {
		tx, txErr := database.BeginTx(ctx, nil)
		if txErr != nil {
			return fmt.Errorf("begin provenance corrections tx: %w", txErr)
		}
		defer tx.Rollback() //nolint:errcheck

		now := time.Now().UTC()
		const updateLinkSQL = `INSERT INTO stash_links (stash_id, item_id, linked_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(stash_id) DO UPDATE SET
		   item_id = excluded.item_id,
		   linked_at = excluded.linked_at`
		for _, c := range final {
			if _, execErr := tx.ExecContext(ctx, updateLinkSQL,
				c.StashID, c.CanonicalDeliveryArtifactID, now.Format(time.RFC3339Nano),
			); execErr != nil {
				return fmt.Errorf("apply provenance correction %s: %w", c.StashID, execErr)
			}
		}
		return tx.Commit()
	})
}
