package core

// doctor.go provides workspace integrity diagnostics for the backlogit workspace.
// 025.016-T (Unit 4): Implements the Doctor function that detects structural problems
// such as orphaned level-2 artifacts, duplicate IDs across queue and archive,
// and nil-layout guard failures.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/softwaresalt/backlogit/internal/events"
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
)

// DoctorFinding describes a single integrity issue detected by Doctor.
type DoctorFinding struct {
	Type        DoctorFindingType `json:"type"`
	ArtifactID  string            `json:"artifact_id"`
	Description string            `json:"description"`
}

// DoctorReport summarises the results of a Doctor run.
type DoctorReport struct {
	Findings  []DoctorFinding `json:"findings"`
	CheckedAt time.Time       `json:"checked_at"`
}

// DoctorOptions controls which checks Doctor performs.
type DoctorOptions struct {
	// CheckOrphans enables the orphaned-artifact check.
	CheckOrphans bool
	// CheckDuplicates enables the queue/archive duplicate-ID check.
	CheckDuplicates bool
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

	searchDirs, err := artifactSearchDirs(ws)
	if err != nil {
		return nil, fmt.Errorf("doctor: %w", err)
	}

	type artifactInfo struct {
		id           string
		artifactType string
		parentID     string
		level        int // stored level from frontmatter (0 when absent)
	}

	var artifacts []artifactInfo
	idToFiles := make(map[string][]string)

	for _, dir := range searchDirs {
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			continue
		}
		walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return err
			}
			a, _, parseErr := parseFile(path)
			if parseErr != nil || a.ID == "" {
				return nil
			}
			// Track first occurrence for orphan checking; track all paths for duplicate detection.
			if _, seen := idToFiles[a.ID]; !seen {
				artifacts = append(artifacts, artifactInfo{
					id:           a.ID,
					artifactType: a.ArtifactType,
					parentID:     a.ParentID,
					level:        a.Level,
				})
			}
			idToFiles[a.ID] = append(idToFiles[a.ID], path)
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("doctor: walk %s: %w", dir, walkErr)
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
			// Legacy artifacts may have a child type (e.g., "task") but reside
			// at root level (level 1) in frontmatter. These are not orphans;
			// they predate the hierarchical layout and should be skipped.
			if info.level == 1 {
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
	}

	if opts.CheckDuplicates {
		reported := make(map[string]bool)
		for id, paths := range idToFiles {
			if len(paths) < 2 || reported[id] {
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
			report.Findings = append(report.Findings, DoctorFinding{
				Type:        FindingDuplicateID,
				ArtifactID:  id,
				Description: fmt.Sprintf("artifact ID %q appears in %d locations: %v", id, len(relPaths), relPaths),
			})
			reported[id] = true
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
