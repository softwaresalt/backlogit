package core

// doctor.go provides workspace integrity diagnostics for the backlogit workspace.
// 025.016-T (Unit 4): Implements the Doctor function that detects structural problems
// such as orphaned level-2 artifacts, duplicate IDs across queue and archive,
// and nil-layout guard failures.

import (
	"context"
	"fmt"
	"time"
)

// DoctorFindingType classifies an integrity issue found by Doctor.
type DoctorFindingType string

const (
	// FindingOrphanedArtifact indicates a level-2+ artifact with no parent_id
	// and no evidence of intentional orphaning (e.g., returned_to_backlog event).
	FindingOrphanedArtifact DoctorFindingType = "orphaned_artifact"

	// FindingDuplicateID indicates the same artifact ID exists in both the
	// queue directory and the archive directory simultaneously.
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
func Doctor(_ context.Context, _ *Workspace, _ *DoctorOptions) (*DoctorReport, error) {
	return nil, fmt.Errorf("not implemented: Doctor — 025.016-T")
}
