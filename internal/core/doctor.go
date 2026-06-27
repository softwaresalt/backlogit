package core

// doctor.go provides workspace integrity diagnostics for the backlogit workspace.
// 025.016-T (Unit 4): Implements the Doctor function that detects structural problems
// such as orphaned level-2 artifacts, duplicate IDs across queue and archive,
// and nil-layout guard failures.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// DoctorFindingType classifies an integrity issue found by Doctor.
type DoctorFindingType string

const (
	// FindingOrphanedArtifact indicates a level-2+ artifact with no parent_id
	// and no evidence of intentional orphaning (e.g., returned_to_backlog event).
	FindingOrphanedArtifact DoctorFindingType = "orphaned_artifact"

	// FindingDuplicateID indicates the same artifact ID appears in two or more
	// workspace directories simultaneously (e.g., both queue and archive, or
	// across multiple registry-routed directories).
	FindingDuplicateID DoctorFindingType = "duplicate_id"

	// FindingRootIDCollision indicates a level-1 (root) work-item ID is present
	// in both the archive directory and at least one non-archive (e.g. queue)
	// location. This is the acute, data-loss-prone case from 066-F: archiving the
	// non-archive copy would overwrite a distinct archived item that shares the
	// filename. It is emitted in addition to FindingDuplicateID so the
	// queue/archive root collision is explicitly distinguishable in the report.
	FindingRootIDCollision DoctorFindingType = "root_id_collision"

	// FindingArchivedFromSelfRef indicates an archive record whose archived_from
	// resolves to its own archive path (self-referential). UnarchiveItem cannot
	// restore such a record to the queue without the read-time self-heal, and the
	// stored value is the migration target for doctor --fix-archived-from.
	FindingArchivedFromSelfRef DoctorFindingType = "archived_from_self_ref"

	// FindingArchivedFromMalformed indicates an archive record whose archived_from
	// is present but is not a markdown path (e.g. a stray status token). Flagged for
	// manual review only; never auto-repaired.
	FindingArchivedFromMalformed DoctorFindingType = "archived_from_malformed"
)

// DoctorFinding describes a single integrity issue detected by Doctor.
type DoctorFinding struct {
	Type        DoctorFindingType `json:"type"`
	ArtifactID  string            `json:"artifact_id"`
	Description string            `json:"description"`
}

// FixActionType classifies a repair action taken by Doctor.
type FixActionType string

const (
	// FixArchived indicates the item was archived by the fix-orphans path.
	FixArchived FixActionType = "archived"

	// FixArchivedFromRepaired indicates a self-referential archived_from field was
	// rewritten to its canonical queue restore path by doctor --fix-archived-from.
	FixArchivedFromRepaired FixActionType = "archived_from_repaired"
)

// FixAction describes a repair action taken by Doctor in fix mode.
type FixAction struct {
	Type       FixActionType `json:"type"`
	ArtifactID string        `json:"artifact_id"`
	Detail     string        `json:"detail"`
}

// DoctorReport summarises the results of a Doctor run.
type DoctorReport struct {
	Findings   []DoctorFinding `json:"findings"`
	FixActions []FixAction     `json:"fix_actions,omitempty"`
	CheckedAt  time.Time       `json:"checked_at"`
}

// DoctorOptions controls which checks Doctor performs.
type DoctorOptions struct {
	// CheckOrphans enables the orphaned-artifact check.
	CheckOrphans bool
	// CheckDuplicates enables the queue/archive duplicate-ID check.
	CheckDuplicates bool
	// FixOrphans archives orphaned artifacts instead of just reporting them.
	// Requires CheckOrphans to be true.
	FixOrphans bool
	// CheckArchivedFrom enables the read-only archived_from invertibility audit
	// (self-referential and malformed archive records). Detection only; safe to
	// expose on the MCP doctor tool.
	CheckArchivedFrom bool
	// FixArchivedFrom rewrites self-referential archived_from records to their
	// canonical queue restore path. Destructive: CLI-only (never wired to the MCP
	// tool, whose params are model-settable). Requires CheckArchivedFrom to be true.
	FixArchivedFrom bool
}

// Doctor scans the workspace for structural integrity issues and returns a
// DoctorReport summarising any findings. An empty Findings slice indicates
// the workspace is clean.
func Doctor(ctx context.Context, ws *Workspace, opts *DoctorOptions) (*DoctorReport, error) {
	if ws == nil {
		return nil, fmt.Errorf("doctor: workspace is nil")
	}
	if opts == nil {
		opts = &DoctorOptions{}
	}

	// 067.005-T: FixArchivedFrom is destructive and only meaningful alongside the
	// archived_from audit. Reject the contradictory combination explicitly rather
	// than silently skipping the repair, which would be surprising for a
	// destructive fix flag and hard to debug in automation.
	if opts.FixArchivedFrom && !opts.CheckArchivedFrom {
		return nil, fmt.Errorf("doctor: FixArchivedFrom requires CheckArchivedFrom to be true")
	}

	report := &DoctorReport{
		Findings:  []DoctorFinding{},
		CheckedAt: time.Now().UTC(),
	}

	type artifactInfo struct {
		id           string
		artifactType string
		parentID     string
		status       string
		level        int // effective level (frontmatter level, or derived from ID)
	}

	// 066.001-T: a single shared canonical scan feeds the orphan, duplicate, and
	// root-ID collision checks. One recursive walk over the full artifactSearchDirs
	// set replaces the per-check walks so files are parsed exactly once.
	refs, err := scanCanonicalArtifacts(ws)
	if err != nil {
		return nil, fmt.Errorf("doctor: %w", err)
	}

	// Deterministic ID ordering keeps orphan/fix/duplicate finding order stable
	// regardless of the (unordered) scan map iteration.
	ids := make([]string, 0, len(refs))
	for id := range refs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var artifacts []artifactInfo
	idToFiles := make(map[string][]string, len(refs))
	for _, id := range ids {
		group := refs[id]
		first := group[0]
		artifacts = append(artifacts, artifactInfo{
			id:           first.id,
			artifactType: first.artifactType,
			parentID:     first.parentID,
			status:       first.status,
			level:        first.level,
		})
		for _, r := range group {
			idToFiles[id] = append(idToFiles[id], r.path)
		}
	}

	logsDir := WorkspaceLogsRoot(ws.RootPath)

	if opts.CheckOrphans && ws.Config != nil {
		for _, info := range artifacts {
			// Determine whether this artifact type is a child type (level >= 2
			// or has allowed parents). When QueueLayout is present, use hierarchy
			// level; otherwise fall back to allowedParentTypes so the check works
			// even when QueueLayout is nil — matching validateArtifactParent logic.
			isChildType := false
			if ws.Config.QueueLayout != nil {
				if level, levelErr := LevelForType(ws.Config.QueueLayout, info.artifactType); levelErr == nil && level >= 2 {
					isChildType = true
				}
			} else {
				isChildType = len(allowedParentTypes(ws, info.artifactType)) > 0
			}
			if !isChildType {
				continue
			}
			// Legacy artifacts may have a child type (e.g., "task") but reside at
			// root level in the workspace. Real legacy artifacts do not include a
			// level field in frontmatter, so info.level is 0; derive the effective
			// level from the ID structure in that case ("001-T" -> 1, "001.001-T" -> 2).
			effectiveLevel := info.level
			if effectiveLevel == 0 {
				effectiveLevel = levelFromID(info.id)
			}
			if effectiveLevel == 1 {
				continue
			}
			if info.parentID != "" {
				continue
			}
			if hasReturnedToBacklogEvent(logsDir, info.id) {
				continue
			}
			report.Findings = append(report.Findings, DoctorFinding{
				Type:        FindingOrphanedArtifact,
				ArtifactID:  info.id,
				Description: fmt.Sprintf("artifact %q (type %q) has no parent_id and no returned_to_backlog event", info.id, info.artifactType),
			})
		}

		// Fix orphans by archiving them when requested.
		if opts.FixOrphans && ws.DB != nil {
			duplicateIDs := make(map[string]bool)
			for id, paths := range idToFiles {
				if len(paths) >= 2 {
					duplicateIDs[id] = true
				}
			}

			// Build status lookup from parsed artifacts to skip already-archived
			// items without a DB round-trip.
			statusByID := make(map[string]string, len(artifacts))
			for _, info := range artifacts {
				statusByID[info.id] = info.status
			}

			for _, f := range report.Findings {
				if f.Type != FindingOrphanedArtifact {
					continue
				}
				if duplicateIDs[f.ArtifactID] {
					continue
				}
				if statusByID[f.ArtifactID] == string(models.StatusArchived) {
					continue
				}
				if _, archErr := ArchiveItem(ctx, ws.DB, ws, f.ArtifactID); archErr != nil {
					slog.Warn("doctor: fix-orphans: failed to archive",
						"artifact_id", f.ArtifactID, "error", archErr)
					continue
				}
				report.FixActions = append(report.FixActions, FixAction{
					Type:       FixArchived,
					ArtifactID: f.ArtifactID,
					Detail:     fmt.Sprintf("orphaned artifact %q archived by doctor --fix-orphans", f.ArtifactID),
				})
			}
		}
	}

	if opts.CheckDuplicates {
		// Iterate the pre-sorted id list (not the idToFiles map) so duplicate-ID
		// findings are emitted in deterministic order, matching the root-collision
		// pass below.
		for _, id := range ids {
			paths := idToFiles[id]
			if len(paths) < 2 {
				continue
			}
			// Convert to workspace-relative paths to keep the report portable.
			relPaths := make([]string, 0, len(paths))
			for _, p := range paths {
				if rel, relErr := filepath.Rel(ws.RootPath, p); relErr == nil {
					relPaths = append(relPaths, rel)
				} else {
					relPaths = append(relPaths, p)
				}
			}
			// scanCanonicalArtifacts returns paths in filesystem-walk order, which
			// is not guaranteed stable across runs. Sort so the finding description
			// (which embeds relPaths) is fully deterministic, matching the sorted
			// id iteration above.
			sort.Strings(relPaths)
			report.Findings = append(report.Findings, DoctorFinding{
				Type:        FindingDuplicateID,
				ArtifactID:  id,
				Description: fmt.Sprintf("artifact ID %q appears in %d locations: %v", id, len(relPaths), relPaths),
			})
		}

		// 066.001-T: Root-ID collision is the acute, data-loss-prone case from
		// 066-F. A level-1 (root) work-item ID present in BOTH the archive
		// directory and a non-archive location means archiving the non-archive
		// copy would overwrite a distinct archived item that shares the filename.
		// Emitted in addition to FindingDuplicateID so the queue/archive root
		// collision is explicitly distinguishable. Level-2+ duplicates are routed
		// into per-parent subdirectories and do not share the same destination,
		// so they are intentionally excluded here.
		archiveDir := filepath.Clean(filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive"))
		isUnderArchive := func(p string) bool {
			cp := filepath.Clean(p)
			return cp == archiveDir || strings.HasPrefix(cp, archiveDir+string(filepath.Separator))
		}
		for _, id := range ids {
			group := refs[id]
			if len(group) < 2 {
				continue
			}
			effectiveLevel := group[0].level
			if effectiveLevel == 0 {
				effectiveLevel = levelFromID(id)
			}
			if effectiveLevel != 1 {
				continue
			}
			var inArchive, outsideArchive bool
			for _, r := range group {
				if isUnderArchive(r.path) {
					inArchive = true
				} else {
					outsideArchive = true
				}
			}
			if !inArchive || !outsideArchive {
				continue
			}
			report.Findings = append(report.Findings, DoctorFinding{
				Type:        FindingRootIDCollision,
				ArtifactID:  id,
				Description: fmt.Sprintf("root work-item ID %q occupies both the archive and a non-archive location; archiving the live copy would overwrite the distinct archived item sharing the filename", id),
			})
		}
	}

	// 067.004-T (U4): read-only archived_from invertibility audit. Flags archive
	// records whose archived_from self-references its own archive path or is
	// malformed (not a markdown path). Detection only — safe to surface on MCP.
	if opts.CheckArchivedFrom {
		records, afErr := scanArchivedFrom(ws)
		if afErr != nil {
			return nil, fmt.Errorf("doctor: %w", afErr)
		}
		for _, r := range records {
			switch r.kind {
			case archivedFromSelfRef:
				report.Findings = append(report.Findings, DoctorFinding{
					Type:        FindingArchivedFromSelfRef,
					ArtifactID:  r.id,
					Description: fmt.Sprintf("archive record %q has a self-referential archived_from %q (resolves to its own archive path); unarchive cannot restore it to the queue without the read-time self-heal", r.id, r.value),
				})
			case archivedFromMalformed:
				report.Findings = append(report.Findings, DoctorFinding{
					Type:        FindingArchivedFromMalformed,
					ArtifactID:  r.id,
					Description: fmt.Sprintf("archive record %q has a malformed archived_from %q (not a markdown path); flagged for manual review, not auto-repaired", r.id, r.value),
				})
			}
		}

		// 067.005-T (U5): destructive repair, CLI-gated by the caller. Rewrite each
		// self-referential record's archived_from to its canonical queue restore
		// path. Continue-on-error per record; malformed records are flagged only.
		if opts.FixArchivedFrom {
			report.FixActions = append(report.FixActions, repairArchivedFrom(ws, records)...)
		}
	}

	return report, nil
}

// archivedFromKind classifies an archive record's archived_from value for the
// invertibility audit.
type archivedFromKind int

const (
	archivedFromOK archivedFromKind = iota
	archivedFromSelfRef
	archivedFromMalformed
)

// archivedFromRecord is a single archive record flagged by the archived_from
// audit, retaining the absolute file path and raw value so the --fix repair can
// rewrite it in place.
type archivedFromRecord struct {
	path  string // absolute archive file path
	id    string
	value string // raw archived_from value
	kind  archivedFromKind
}

// scanArchivedFrom walks the archive directory and returns the records whose
// archived_from is self-referential (resolves to the record's own archive path)
// or malformed (not a markdown path). Fieldless and canonical/legitimate records
// are excluded. The self-reference comparison mirrors ArchiveItem/UnarchiveItem
// (archive.go) exactly. Results are sorted by path for deterministic reporting.
func scanArchivedFrom(ws *Workspace) ([]archivedFromRecord, error) {
	archiveDir := filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive")
	var records []archivedFromRecord
	walkErr := filepath.WalkDir(archiveDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // archive dir absent: nothing to audit
			}
			return err
		}
		if d.IsDir() || filepath.Ext(d.Name()) != ".md" {
			return nil
		}
		raw, readErr := os.ReadFile(p)
		if readErr != nil {
			slog.Warn("doctor: archived_from audit: read failed", "path", p, "error", readErr)
			return nil
		}
		fm, _, parseErr := models.ParseFrontmatter(string(raw))
		if parseErr != nil {
			slog.Warn("doctor: archived_from audit: parse failed", "path", p, "error", parseErr)
			return nil
		}
		value := ""
		if v, ok := fm["archived_from"].(string); ok {
			value = strings.TrimSpace(v)
		}
		if value == "" {
			return nil // fieldless: not an invertibility hazard
		}
		id, _ := fm["id"].(string)
		if id == "" {
			id = strings.TrimSuffix(d.Name(), ".md")
		}
		rec := archivedFromRecord{path: p, id: id, value: value, kind: archivedFromOK}
		switch {
		case filepath.Clean(resolveWorkspacePath(ws.RootPath, value)) == filepath.Clean(p):
			rec.kind = archivedFromSelfRef
		case filepath.Ext(value) != ".md":
			rec.kind = archivedFromMalformed
		}
		if rec.kind != archivedFromOK {
			records = append(records, rec)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("archived_from audit: %w", walkErr)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].path < records[j].path })
	return records, nil
}

// repairArchivedFrom rewrites each self-referential archive record's archived_from
// to its canonical queue restore path. It is destructive and must only be reached
// when the caller explicitly opted in (CLI-only --fix-archived-from). Safety rules:
// a record whose final path component is itself a symlink is refused (Lstat); every
// other record is resolved with EvalSymlinks (collapsing any intermediate symlinked
// components) and skipped unless the resulting realpath — and the recomputed restore
// target — are provably contained within the workspace storage root. Repair is
// continue-on-error per record (each failure is logged, not fatal) and emits one
// FixAction per repaired record so the structured report is the authoritative
// migration manifest. Malformed records are flagged only (never rewritten).
func repairArchivedFrom(ws *Workspace, records []archivedFromRecord) []FixAction {
	storageRoot := WorkspaceStorageRoot(ws.RootPath)
	realRoot, rootErr := filepath.EvalSymlinks(storageRoot)
	if rootErr != nil {
		realRoot = storageRoot
	}

	var actions []FixAction
	for _, r := range records {
		if r.kind != archivedFromSelfRef {
			continue // malformed/OK records are never auto-repaired
		}
		// Symlink refusal: never rewrite through a symlinked record.
		info, lerr := os.Lstat(r.path)
		if lerr != nil {
			slog.Warn("doctor: fix-archived-from: lstat failed", "path", r.path, "error", lerr)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			slog.Warn("doctor: fix-archived-from: refusing symlinked record", "path", r.path)
			continue
		}
		// Realpath containment: resolve symlinks on both sides and require the record
		// to live under the workspace storage root.
		real, evalErr := filepath.EvalSymlinks(r.path)
		if evalErr != nil {
			slog.Warn("doctor: fix-archived-from: evalsymlinks failed", "path", r.path, "error", evalErr)
			continue
		}
		if !pathContained(realRoot, real) {
			slog.Warn("doctor: fix-archived-from: record escapes workspace storage", "path", r.path)
			continue
		}
		// Recompute the canonical restore target and prove it is workspace-contained
		// (lexical: the target file does not exist yet).
		target := canonicalRestorePath(ws, filepath.Base(r.path))
		if !pathContained(storageRoot, resolveWorkspacePath(ws.RootPath, target)) {
			slog.Warn("doctor: fix-archived-from: recomputed target not contained", "path", r.path, "target", target)
			continue
		}

		raw, readErr := os.ReadFile(r.path)
		if readErr != nil {
			slog.Warn("doctor: fix-archived-from: read failed", "path", r.path, "error", readErr)
			continue
		}
		out, rewriteErr := rewriteArchivedFromField(raw, target)
		if rewriteErr != nil {
			slog.Warn("doctor: fix-archived-from: rewrite failed", "path", r.path, "error", rewriteErr)
			continue
		}
		if err := atomicWriteArchiveFile(r.path, out); err != nil {
			slog.Warn("doctor: fix-archived-from: write failed", "path", r.path, "error", err)
			continue
		}
		actions = append(actions, FixAction{
			Type:       FixArchivedFromRepaired,
			ArtifactID: r.id,
			Detail:     fmt.Sprintf("rewrote self-referential archived_from to %q", target),
		})
	}
	return actions
}

// rewriteArchivedFromField rewrites ONLY the archived_from frontmatter field of a
// markdown record to newValue while preserving the body bytes verbatim. It mirrors
// the internal/docline Decode/Encode codec, which core cannot import directly:
// internal/docline depends on internal/core (docline/service.go), so importing it
// would introduce a package import cycle. The frontmatter block is re-marshaled with
// yaml's deterministic sorted keys; the body after the closing fence is never
// touched, so CRLF, trailing whitespace, and horizontal rules survive unchanged.
func rewriteArchivedFromField(raw []byte, newValue string) ([]byte, error) {
	openLen := frontmatterOpenLen(raw)
	if openLen == 0 {
		return nil, fmt.Errorf("no opening frontmatter fence")
	}
	yamlBlock, body, ok := splitAtFrontmatterFence(raw[openLen:])
	if !ok {
		return nil, fmt.Errorf("no closing frontmatter fence")
	}
	fm := map[string]any{}
	normalized := bytes.ReplaceAll(yamlBlock, []byte("\r\n"), []byte("\n"))
	if len(bytes.TrimSpace(normalized)) > 0 {
		if err := yaml.Unmarshal(normalized, &fm); err != nil {
			return nil, fmt.Errorf("parse frontmatter: %w", err)
		}
	}
	if fm == nil {
		fm = map[string]any{}
	}
	fm["archived_from"] = newValue
	data, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}
	var buf bytes.Buffer
	buf.Grow(len(data) + len(body) + 8)
	buf.WriteString("---\n")
	buf.Write(data) // yaml.Marshal output is LF-terminated
	buf.WriteString("---\n")
	buf.Write(body)
	return buf.Bytes(), nil
}

// frontmatterOpenLen returns the byte length of the opening "---" fence line
// (including its terminator) when raw begins with it, or 0.
func frontmatterOpenLen(raw []byte) int {
	switch {
	case bytes.HasPrefix(raw, []byte("---\n")):
		return 4
	case bytes.HasPrefix(raw, []byte("---\r\n")):
		return 5
	default:
		return 0
	}
}

// splitAtFrontmatterFence scans rest line-by-line and splits at the first line whose
// content is exactly "---" (a trailing CR is tolerated; leading whitespace is not, so
// an indented block-scalar fence is never mistaken for the closing fence). It returns
// the frontmatter YAML bytes, the verbatim body bytes after the fence line, and
// whether a closing fence was found.
func splitAtFrontmatterFence(rest []byte) (yamlBlock, body []byte, ok bool) {
	for i := 0; i < len(rest); {
		nl := bytes.IndexByte(rest[i:], '\n')
		var lineEnd, nextStart int
		if nl == -1 {
			lineEnd, nextStart = len(rest), len(rest)
		} else {
			lineEnd, nextStart = i+nl, i+nl+1
		}
		line := rest[i:lineEnd]
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		if string(line) == "---" {
			return rest[:i], rest[nextStart:], true
		}
		i = nextStart
	}
	return nil, nil, false
}

// pathContained reports whether p resolves to a location inside root.
func pathContained(root, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// atomicWriteArchiveFile writes data to path via a temp file and rename, mirroring
// the atomic-write pattern used by ArchiveItem/UnarchiveItem and SaveCheckpoint.
// On POSIX, os.Rename atomically replaces the destination. On Windows, os.Rename
// can fail when the destination already exists (the convention documented in
// internal/telemetry/checkpoint.go); on failure it removes the destination and
// retries once. The complete new content lives in tmpPath until the rename
// commits, and the destination is only removed after a failed rename (when it
// still exists), so the original record is never lost in the success path.
func atomicWriteArchiveFile(path string, data []byte) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if runtime.GOOS == "windows" {
			_ = os.Remove(path)
			err = os.Rename(tmpPath, path)
		}
		if err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("rename temp: %w", err)
		}
	}
	return nil
}

// hasReturnedToBacklogEvent reports whether the item's event log contains a
// returned_to_backlog entry, indicating intentional orphaning by ShipShipment.
func hasReturnedToBacklogEvent(logsDir, itemID string) bool {
	path := events.LogPathForItem(logsDir, itemID)
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	var ev struct {
		EventType string `json:"event_type"`
	}
	dec := json.NewDecoder(f)
	for {
		if decErr := dec.Decode(&ev); decErr != nil {
			break
		}
		if ev.EventType == "returned_to_backlog" {
			return true
		}
	}
	return false
}

// levelFromID derives a hierarchy depth from an artifact ID when the
// frontmatter level field is absent (zero). It counts dot-separated segments
// in the numeric prefix before the last dash (e.g. "001.002-T" -> 2, "001-T" -> 1).
// The minimum depth returned is 1.
func levelFromID(id string) int {
	dash := strings.LastIndex(id, "-")
	if dash < 0 {
		return 1
	}
	return strings.Count(id[:dash], ".") + 1
}
