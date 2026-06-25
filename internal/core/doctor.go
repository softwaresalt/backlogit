package core

// doctor.go provides workspace integrity diagnostics for the backlogit workspace.
// 025.016-T (Unit 4): Implements the Doctor function that detects structural problems
// such as orphaned level-2 artifacts, duplicate IDs across queue and archive,
// and nil-layout guard failures.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

	return report, nil
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
