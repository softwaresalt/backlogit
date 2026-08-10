package core

// doctor.go provides workspace integrity diagnostics for the backlogit workspace.
// 025.016-T (Unit 4): Implements the Doctor function that detects structural problems
// such as orphaned level-2 artifacts, duplicate IDs across queue and archive,
// and nil-layout guard failures.

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/mdfront"
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

	// FindingWorkspaceRootConflict indicates that both supported workspace
	// storage root names are present under the same repository root.
	FindingWorkspaceRootConflict DoctorFindingType = "workspace_root_conflict"

	// FindingArchivedFromSelfRef indicates an archive record whose archived_from
	// resolves to its own archive path (self-referential). UnarchiveItem cannot
	// restore such a record to the queue without the read-time self-heal, and the
	// stored value is the migration target for doctor --fix-archived-from.
	FindingArchivedFromSelfRef DoctorFindingType = "archived_from_self_ref"

	// FindingArchivedFromMalformed indicates an archive record whose archived_from
	// is present but is not a markdown path (e.g. a stray status token). Flagged for
	// manual review only; never auto-repaired.
	FindingArchivedFromMalformed DoctorFindingType = "archived_from_malformed"

	// FindingMissingGateEvidence indicates a terminal task/subtask that lacks a
	// passing/forced pre-task-completion gate evidence event while gates are
	// configured (082-F ST4.3). Advisory only: it never changes the doctor exit
	// code. A strict, index-backed variant is the separate follow-up 7ED9CE1A.
	FindingMissingGateEvidence DoctorFindingType = "missing_gate_evidence"

	// FindingOverArchivedCoveringFeature indicates a feature that is currently
	// done/archived even though it was never an explicit member of any
	// shipment manifest and has at least one descendant that was returned to
	// the backlog by a partial-feature ship (133-F). This is the check-only
	// detection surface for the ShipShipment covering-feature over-archive
	// invariant: see
	// docs/exec-plans/2026-07-31-shipshipment-partial-feature-archive-cascade-plan.md.
	// Detection only; the destructive repair is deferred out of this release
	// unit and, if ever built, must be CLI-only (Constitution VII).
	//
	// Known limitation (heuristic, not proof; review-fix, PR #327 Copilot
	// finding): both membership and returned-event lookups are lifetime-wide
	// unions across every shipment manifest and every JSONL log ever written,
	// not scoped to the specific shipment that closed this feature. A feature
	// can accumulate a returned_to_backlog event from an EARLIER partial
	// shipment and later be legitimately archived as a genuine descendant of
	// a DIFFERENT, later shipment's explicit-member ancestor feature -- the
	// exact case TestShipShipment_LegitimatelyArchivesNestedFeatureDescendantOfMember
	// establishes as correct behavior. This finding cannot currently
	// distinguish that legitimate case from a real over-archive regression;
	// correlating each event and each archival to its originating shipment ID
	// would resolve the ambiguity and is tracked as a follow-up rather than
	// attempted in this release unit, to avoid expanding this fix's scope.
	// Operators must verify a reported feature's actual archival history
	// before treating this finding as confirmed corruption.
	FindingOverArchivedCoveringFeature DoctorFindingType = "over_archived_covering_feature"

	// FindingPartialCommitAssociation indicates a commit_links row exists without
	// a matching commit_tracked event in the authoritative JSONL log.
	FindingPartialCommitAssociation DoctorFindingType = "partial_commit_association"

	// FindingInconsistentShipmentMembership indicates the items list in a
	// shipment's frontmatter does not match the items list stored in the SQLite
	// index fast-path (custom_fields.items). A mismatch in either direction
	// suggests a partial AddItemToShipment write where the frontmatter write
	// succeeded (possibly indeterminate) but the SQLite index was not yet
	// refreshed, or a concurrent out-of-band edit to one representation.
	// Advisory; never blocking.
	FindingInconsistentShipmentMembership DoctorFindingType = "inconsistent_shipment_membership"

	// FindingInconsistentDependencyEdge indicates frontmatter dependency edges and
	// cached item_deps edges disagree for the same artifact.
	FindingInconsistentDependencyEdge DoctorFindingType = "inconsistent_dependency_edge"
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

	// FixArchivedFromCleared indicates a malformed archived_from field was removed
	// (body-preserving) by doctor --fix-malformed. The malformed class are archive
	// records with no queue restore target (e.g. deliberation artifacts stamped with
	// a stray status token), so clearing the field is correct: there is no path to
	// stamp, and a blank field is fieldless-tolerant in the invertibility audit.
	FixArchivedFromCleared FixActionType = "archived_from_cleared"
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
	// FixMalformed clears (removes) malformed archived_from fields on archive
	// records that have no queue restore target. Body-preserving and CLI-only.
	// Requires CheckArchivedFrom to be true.
	FixMalformed bool
	// CheckGateEvidence enables the advisory pre-task-completion gate-evidence
	// audit (082-F ST4.3): terminal task/subtask artifacts are scanned for a
	// passing/forced gate evidence event while gates are configured. Advisory only
	// — findings never change the doctor exit code. Safe to expose on MCP (read-only).
	CheckGateEvidence bool
	// CheckOverArchivedFeatures enables the read-only over-archived
	// covering-feature audit (133-F): flags a feature that is done/archived,
	// was never an explicit shipment manifest member, yet has at least one
	// descendant returned to the backlog by a partial-feature ship. Detection
	// only; no mutation. CLI-only, matching CheckArchivedFrom/CheckGateEvidence
	// (not wired to the backlogit_doctor MCP tool).
	CheckOverArchivedFeatures bool
	// CheckPartialMutations enables detection of residual partial-mutation state
	// for governed commit association and dependency-linking paths. Advisory only:
	// findings never change the doctor exit code.
	CheckPartialMutations bool
	// CheckWorkspaceRootConflict enables the read-only dual-root conflict check.
	CheckWorkspaceRootConflict bool
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
	if opts.FixMalformed && !opts.CheckArchivedFrom {
		return nil, fmt.Errorf("doctor: FixMalformed requires CheckArchivedFrom to be true")
	}

	report := &DoctorReport{
		Findings:  []DoctorFinding{},
		CheckedAt: time.Now().UTC(),
	}

	if opts.CheckWorkspaceRootConflict {
		findings, err := checkWorkspaceRootConflict(ws.RootPath)
		if err != nil {
			return nil, fmt.Errorf("doctor: check workspace root conflict: %w", err)
		}
		report.Findings = append(report.Findings, findings...)
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
				malformedDesc := fmt.Sprintf("archive record %q has a malformed archived_from %q (not a markdown path); flagged for manual review, not auto-repaired", r.id, r.value)
				if opts.FixMalformed {
					malformedDesc = fmt.Sprintf("archive record %q has a malformed archived_from %q (not a markdown path); auto-repaired by clearing (no queue restore target)", r.id, r.value)
				}
				report.Findings = append(report.Findings, DoctorFinding{
					Type:        FindingArchivedFromMalformed,
					ArtifactID:  r.id,
					Description: malformedDesc,
				})
			}
		}

		// 067.005-T (U5): destructive repair, CLI-gated by the caller. Rewrite each
		// self-referential record's archived_from to its canonical queue restore
		// path. Continue-on-error per record; malformed records are flagged only.
		if opts.FixArchivedFrom {
			report.FixActions = append(report.FixActions, repairArchivedFrom(ws, records)...)
		}
		if opts.FixMalformed {
			report.FixActions = append(report.FixActions, clearMalformedArchivedFrom(ws, records)...)
		}
	}

	// 082-F ST4.3: advisory pre-task-completion gate-evidence audit. When gates are
	// configured (not disabled), every terminal task/subtask SHOULD carry a
	// passing/forced gate evidence event in its item log. Missing evidence is a
	// WARNING only — it never changes the exit code (advisory mode).
	//
	// Q3.3 (083.005.004-ST): the audit prefers the derived gate_evidence
	// projection's POSITIVE index (passed/forced/forced_no_run) over scanning
	// each item's logs. Item logs are append-only, so a positive projection row
	// is always safe to trust. Any item ABSENT from the positive index — never
	// gated, gated since the last sync, or projected "missing" (which can be
	// stale in the pass direction) — falls back to the authoritative log-scan so
	// the logs remain the single source of truth and a stale/absent projection
	// never yields a false positive or false negative.
	if opts.CheckGateEvidence && ws.gateConfig.Enabled != "false" {
		var passing map[string]string
		if ws.DB != nil {
			if p, perr := bldb.LoadPassingGateEvidence(ctx, ws.DB); perr != nil {
				slog.WarnContext(ctx, "doctor: gate-evidence audit: projection load failed, falling back to log-scan", "error", perr)
			} else {
				passing = p
			}
		}
		for _, info := range artifacts {
			if info.artifactType != "task" && info.artifactType != "subtask" {
				continue
			}
			if !ws.isGateTerminalStatus(info.status) {
				continue
			}
			missing, evErr := gateEvidenceMissing(ctx, logsDir, info.id, passing)
			if evErr != nil {
				slog.WarnContext(ctx, "doctor: gate-evidence audit: read events failed", "id", info.id, "error", evErr)
				continue
			}
			if missing {
				report.Findings = append(report.Findings, DoctorFinding{
					Type:        FindingMissingGateEvidence,
					ArtifactID:  info.id,
					Description: fmt.Sprintf("terminal %s %q has no passing/forced pre-task-completion gate evidence while gates are configured (advisory; exit code unaffected)", info.artifactType, info.id),
				})
			}
		}
	}

	// 133.005-T (Unit 3): check-only audit for the ShipShipment
	// covering-feature over-archive invariant (133-F). A feature that is
	// currently done/archived, was never itself an explicit member of any
	// shipment manifest, yet has at least one descendant returned to the
	// backlog by a partial-feature ship, was over-closed by the cascade rollup
	// seam that Unit 2 now neutralizes going forward. Detection deliberately
	// reconstructs membership from the shipment manifest and the
	// returned_to_backlog event provenance -- NOT from parent_id, which
	// returnUnreleasedFeatureItems clears on the returned item, making
	// parent_id unusable for this reconstruction. No mutation is performed.
	if opts.CheckOverArchivedFeatures {
		explicitMembers := scanShipmentManifestFeatureIDs(refs)
		returnedFeatureIDs, scanErr := scanReturnedFeatureIDs(logsDir)
		if scanErr != nil {
			slog.WarnContext(ctx, "doctor: over-archived-feature audit: scan returned-to-backlog events failed", "error", scanErr)
		}
		for _, info := range artifacts {
			if info.artifactType != "feature" {
				continue
			}
			if info.status != string(models.StatusDone) && info.status != string(models.StatusArchived) {
				continue
			}
			if _, isMember := explicitMembers[info.id]; isMember {
				continue
			}
			if _, hasReturned := returnedFeatureIDs[info.id]; !hasReturned {
				continue
			}
			report.Findings = append(report.Findings, DoctorFinding{
				Type:       FindingOverArchivedCoveringFeature,
				ArtifactID: info.id,
				Description: fmt.Sprintf(
					"feature %q is %q but was never an explicit shipment manifest member and has descendant work returned to the backlog by a partial-feature ship (heuristic, lifetime-wide correlation: verify actual archival history before treating as a regression -- a feature legitimately archived later as a descendant of a different explicit-member shipment can also match this pattern)",
					info.id, info.status),
			})
		}
	}

	if opts.CheckPartialMutations && ws.DB != nil {
		commitFindings, commitErr := detectPartialCommitAssociations(ctx, ws, logsDir)
		if commitErr != nil {
			return nil, fmt.Errorf("doctor: detect partial commit associations: %w", commitErr)
		}
		report.Findings = append(report.Findings, commitFindings...)

		dependencyFindings, depErr := detectInconsistentDependencyEdges(ctx, ws, idToFiles, ids)
		if depErr != nil {
			return nil, fmt.Errorf("doctor: detect inconsistent dependency edges: %w", depErr)
		}
		report.Findings = append(report.Findings, dependencyFindings...)

		shipmentFindings, shipmentErr := detectInconsistentShipmentMembership(ctx, ws, idToFiles, ids)
		if shipmentErr != nil {
			return nil, fmt.Errorf("doctor: detect inconsistent shipment membership: %w", shipmentErr)
		}
		report.Findings = append(report.Findings, shipmentFindings...)
	}

	return report, nil
}

func detectPartialCommitAssociations(ctx context.Context, ws *Workspace, logsDir string) ([]DoctorFinding, error) {
	rows, err := ws.DB.QueryContext(ctx, `SELECT item_id, commit_sha FROM commit_links ORDER BY item_id, commit_sha`)
	if err != nil {
		return nil, fmt.Errorf("query commit links: %w", err)
	}
	defer rows.Close()

	eventCache := make(map[string][]events.Event)
	findings := make([]DoctorFinding, 0)
	for rows.Next() {
		var itemID string
		var commitSHA string
		if err := rows.Scan(&itemID, &commitSHA); err != nil {
			return nil, fmt.Errorf("scan commit link: %w", err)
		}
		itemEvents, ok := eventCache[itemID]
		if !ok {
			loadedEvents, err := events.ReadAllEvents(ctx, logsDir, itemID)
			if err != nil {
				return nil, fmt.Errorf("read events for %s: %w", itemID, err)
			}
			itemEvents = loadedEvents
			eventCache[itemID] = itemEvents
		}
		if hasCommitTrackedEvent(itemEvents, commitSHA) {
			continue
		}
		findings = append(findings, DoctorFinding{
			Type:       FindingPartialCommitAssociation,
			ArtifactID: itemID,
			Description: fmt.Sprintf(
				"commit_links contains %q for %q, but the item JSONL log has no matching commit_tracked event",
				commitSHA, itemID,
			),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate commit links: %w", err)
	}
	return findings, nil
}

func detectInconsistentDependencyEdges(
	ctx context.Context,
	ws *Workspace,
	idToFiles map[string][]string,
	ids []string,
) ([]DoctorFinding, error) {
	findings := make([]DoctorFinding, 0)
	for _, id := range ids {
		paths := idToFiles[id]
		if len(paths) == 0 {
			continue
		}
		artifact, _, err := parseFile(paths[0])
		if err != nil {
			return nil, fmt.Errorf("parse artifact %s: %w", id, err)
		}
		dbEdges, err := bldb.GetDependencies(ctx, ws.DB, id)
		if err != nil {
			return nil, fmt.Errorf("load cached dependencies for %s: %w", id, err)
		}

		frontmatterSet := make(map[string]struct{}, len(artifact.Dependencies))
		for _, dep := range artifact.Dependencies {
			frontmatterSet[dependencyKey(dep.ID, dep.Type)] = struct{}{}
		}
		cacheSet := make(map[string]struct{}, len(dbEdges))
		for _, dep := range dbEdges {
			cacheSet[dependencyKey(dep.DependsOn, dep.DepType)] = struct{}{}
		}

		missingInCache := dependencyDiff(frontmatterSet, cacheSet)
		missingInFrontmatter := dependencyDiff(cacheSet, frontmatterSet)
		if len(missingInCache) == 0 && len(missingInFrontmatter) == 0 {
			continue
		}

		description := fmt.Sprintf("dependency edge mismatch for %q", id)
		if len(missingInCache) > 0 {
			description += fmt.Sprintf("; frontmatter-only edges: %v", missingInCache)
		}
		if len(missingInFrontmatter) > 0 {
			description += fmt.Sprintf("; cache-only edges: %v", missingInFrontmatter)
		}
		findings = append(findings, DoctorFinding{
			Type:        FindingInconsistentDependencyEdge,
			ArtifactID:  id,
			Description: description,
		})
	}
	return findings, nil
}

func hasCommitTrackedEvent(itemEvents []events.Event, commitSHA string) bool {
	for _, event := range itemEvents {
		if event.EventType != "commit_tracked" {
			continue
		}
		if loggedSHA, _ := event.Delta["commit_sha"].(string); loggedSHA == commitSHA {
			return true
		}
	}
	return false
}

// detectInconsistentShipmentMembership compares the items list in each
// shipment's frontmatter against the items list stored in the SQLite index
// (custom_fields.items). A mismatch in either direction indicates a potential
// partial AddItemToShipment write or an out-of-band edit to one representation.
// This check covers both mismatch directions: frontmatter-only items (the
// frontmatter write succeeded but the index is stale or the JSONL event was
// lost) and index-only items (the index was somehow updated without a
// corresponding frontmatter write, which should not occur in normal operation).
// Advisory only — findings never change the doctor exit code.
func detectInconsistentShipmentMembership(
	ctx context.Context,
	ws *Workspace,
	idToFiles map[string][]string,
	ids []string,
) ([]DoctorFinding, error) {
	findings := make([]DoctorFinding, 0)
	for _, id := range ids {
		paths := idToFiles[id]
		if len(paths) == 0 {
			continue
		}
		artifact, _, err := parseFile(paths[0])
		if err != nil {
			return nil, fmt.Errorf("parse artifact %s: %w", id, err)
		}
		if artifact.ArtifactType != "shipment" {
			continue
		}

		// Load the items list from the SQLite index custom_fields column.
		dbArtifact, dbErr := bldb.GetItem(ctx, ws.DB, id)
		if dbErr != nil {
			// Item may not be indexed yet; skip silently.
			continue
		}

		frontmatterItems := NormalizeShipmentItems(artifact)
		dbItems := NormalizeShipmentItems(dbArtifact)

		frontmatterSet := make(map[string]struct{}, len(frontmatterItems))
		for _, item := range frontmatterItems {
			frontmatterSet[item] = struct{}{}
		}
		dbSet := make(map[string]struct{}, len(dbItems))
		for _, item := range dbItems {
			dbSet[item] = struct{}{}
		}

		var frontmatterOnly, dbOnly []string
		for item := range frontmatterSet {
			if _, ok := dbSet[item]; !ok {
				frontmatterOnly = append(frontmatterOnly, item)
			}
		}
		for item := range dbSet {
			if _, ok := frontmatterSet[item]; !ok {
				dbOnly = append(dbOnly, item)
			}
		}
		if len(frontmatterOnly) == 0 && len(dbOnly) == 0 {
			continue
		}

		description := fmt.Sprintf("shipment membership mismatch for %q", id)
		if len(frontmatterOnly) > 0 {
			sort.Strings(frontmatterOnly)
			description += fmt.Sprintf("; frontmatter-only items: %v", frontmatterOnly)
		}
		if len(dbOnly) > 0 {
			sort.Strings(dbOnly)
			description += fmt.Sprintf("; index-only items: %v", dbOnly)
		}
		findings = append(findings, DoctorFinding{
			Type:        FindingInconsistentShipmentMembership,
			ArtifactID:  id,
			Description: description,
		})
	}
	return findings, nil
}

func dependencyKey(depID, depType string) string {
	return depID + "|" + normalizedDependencyType(depType)
}

func normalizedDependencyType(depType string) string {
	if depType == "" {
		return "blocks"
	}
	return depType
}

func dependencyDiff(left, right map[string]struct{}) []string {
	diff := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; ok {
			continue
		}
		diff = append(diff, key)
	}
	sort.Strings(diff)
	return diff
}

// gateEvidenceMissing reports whether a terminal gated member lacks valid gate
// evidence (Q3.3). It consults the derived gate_evidence projection's POSITIVE
// index first: an item present in `passing` carries a passed/forced/forced_no_run
// pass, which is safe to trust because item logs are append-only. Any item ABSENT
// from the positive index — never gated, gated since the last sync, or projected
// as "missing" (which can be stale in the pass direction) — falls back to the
// authoritative per-item log-scan so the item logs remain the single source of
// truth and a stale/absent projection never produces a false positive or false
// negative.
func gateEvidenceMissing(ctx context.Context, logsDir, id string, passing map[string]string) (bool, error) {
	if _, ok := passing[id]; ok {
		return false, nil
	}
	evs, err := events.ReadAllEvents(ctx, logsDir, id)
	if err != nil {
		return false, err
	}
	return latestGatePassEvidence(evs) == nil, nil
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
		if err := atomicfile.WriteFileAtomic(r.path, out); err != nil {
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

// clearMalformedArchivedFrom removes the malformed archived_from field from each
// malformed archive record, preserving body bytes verbatim. The malformed class
// are records whose archived_from is not a markdown path (e.g. deliberation
// artifacts with no queue restore target); clearing the field is the correct
// disposition — a blank/absent value is fieldless-tolerant in the invertibility
// audit. It is destructive and must only be reached when the caller explicitly
// opted in (CLI-only --fix-malformed). Same safety rails as repairArchivedFrom:
// symlink refusal, EvalSymlinks containment, continue-on-error per record, one
// FixAction per cleared record. Self-ref/OK records are never touched.
func clearMalformedArchivedFrom(ws *Workspace, records []archivedFromRecord) []FixAction {
	storageRoot := WorkspaceStorageRoot(ws.RootPath)
	realRoot, rootErr := filepath.EvalSymlinks(storageRoot)
	if rootErr != nil {
		realRoot = storageRoot
	}

	var actions []FixAction
	for _, r := range records {
		if r.kind != archivedFromMalformed {
			continue // only malformed records are cleared
		}
		info, lerr := os.Lstat(r.path)
		if lerr != nil {
			slog.Warn("doctor: fix-malformed: lstat failed", "path", r.path, "error", lerr)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			slog.Warn("doctor: fix-malformed: refusing symlinked record", "path", r.path)
			continue
		}
		real, evalErr := filepath.EvalSymlinks(r.path)
		if evalErr != nil {
			slog.Warn("doctor: fix-malformed: evalsymlinks failed", "path", r.path, "error", evalErr)
			continue
		}
		if !pathContained(realRoot, real) {
			slog.Warn("doctor: fix-malformed: record escapes workspace storage", "path", r.path)
			continue
		}
		raw, readErr := os.ReadFile(r.path)
		if readErr != nil {
			slog.Warn("doctor: fix-malformed: read failed", "path", r.path, "error", readErr)
			continue
		}
		out, rewriteErr := removeArchivedFromField(raw)
		if rewriteErr != nil {
			slog.Warn("doctor: fix-malformed: rewrite failed", "path", r.path, "error", rewriteErr)
			continue
		}
		if err := atomicfile.WriteFileAtomic(r.path, out); err != nil {
			slog.Warn("doctor: fix-malformed: write failed", "path", r.path, "error", err)
			continue
		}
		actions = append(actions, FixAction{
			Type:       FixArchivedFromCleared,
			ArtifactID: r.id,
			Detail:     fmt.Sprintf("cleared malformed archived_from %q (no restore target)", r.value),
		})
	}
	return actions
}

// rewriteArchivedFromField rewrites ONLY the archived_from frontmatter field of a
// markdown record to newValue while preserving the body bytes verbatim. It uses
// the shared internal/mdfront body-preserving codec (the single canonical
// implementation, now importable here because mdfront is a stdlib-only leaf
// package with no dependency on internal/core). The frontmatter block is
// re-marshaled with yaml's deterministic sorted keys; the body after the closing
// fence is never touched, so CRLF, trailing whitespace, and horizontal rules
// survive unchanged.
//
// F1 guard: mdfront.Decode returns HasFrontmatter=false with a nil Frontmatter
// map and NO error on a fence-less record, whereas this repair's contract
// requires an error so the caller SKIPS the record. Without the explicit
// HasFrontmatter guard a fence-less record would either nil-map-panic on the
// archived_from assignment or be wrapped in a synthetic frontmatter block and
// written atomically (corruption). The guard preserves the error->skip parity.
func rewriteArchivedFromField(raw []byte, newValue string) ([]byte, error) {
	md, err := mdfront.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decode archive record: %w", err)
	}
	if !md.HasFrontmatter {
		return nil, fmt.Errorf("no frontmatter fence")
	}
	if md.Frontmatter == nil {
		md.Frontmatter = map[string]any{}
	}
	md.Frontmatter["archived_from"] = newValue
	out, err := md.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode archive record: %w", err)
	}
	return out, nil
}

// removeArchivedFromField deletes ONLY the archived_from frontmatter field of a
// markdown record while preserving the body bytes verbatim, using the same shared
// mdfront body-preserving codec as rewriteArchivedFromField. Used by --fix-malformed
// to clear bogus archived_from values on records with no restore target. A record
// with no frontmatter fence is refused so the caller skips it; an already-absent
// field re-encodes via the deterministic codec (idempotent on canonical frontmatter).
func removeArchivedFromField(raw []byte) ([]byte, error) {
	md, err := mdfront.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decode archive record: %w", err)
	}
	if !md.HasFrontmatter {
		return nil, fmt.Errorf("no frontmatter fence")
	}
	if md.Frontmatter != nil {
		delete(md.Frontmatter, "archived_from")
	}
	out, err := md.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode archive record: %w", err)
	}
	return out, nil
}

// pathContained reports whether p resolves to a location inside root.
func pathContained(root, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// CheckWorkspaceRootConflict reports whether both supported workspace storage
// roots exist under rootPath.
func CheckWorkspaceRootConflict(rootPath string) ([]DoctorFinding, error) {
	return checkWorkspaceRootConflict(rootPath)
}

func checkWorkspaceRootConflict(rootPath string) ([]DoctorFinding, error) {
	cleanRoot, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}

	entries, err := os.ReadDir(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("read workspace root %s: %w", cleanRoot, err)
	}

	present := make([]string, 0, len(workspaceRootCandidates))
	for _, candidate := range workspaceRootCandidates {
		_, ok, probeErr := probeWorkspaceCandidate(cleanRoot, candidate, entries)
		if probeErr != nil {
			return nil, probeErr
		}
		if ok {
			present = append(present, candidate)
		}
	}
	if len(present) < 2 {
		return nil, nil
	}

	return []DoctorFinding{{
		Type:       FindingWorkspaceRootConflict,
		ArtifactID: "workspace",
		Description: fmt.Sprintf(
			"workspace root contains both %s and %s; set BACKLOGIT_WORKSPACE_DIR to one supported name or remove one",
			present[0],
			present[1],
		),
	}}, nil
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

// scanShipmentManifestFeatureIDs derives the union of every item ID ever
// recorded as an explicit member of any shipment manifest, across every
// shipment regardless of its current status (queued, shipped, or archived).
// It reuses the canonical artifact scan's parsed refs (066.001-T) rather than
// re-walking the filesystem. A feature's presence in the returned set means at
// least one shipment explicitly listed it as a member; absence means no
// shipment ever claimed it directly -- the invariant a covering feature must
// satisfy before it is legitimately closed (133-F). Shipment files that fail
// to parse are skipped rather than aborting the whole audit; other doctor
// checks already surface malformed/duplicate records.
func scanShipmentManifestFeatureIDs(refs map[string][]artifactRef) map[string]struct{} {
	members := make(map[string]struct{})
	for _, group := range refs {
		if len(group) == 0 || group[0].artifactType != "shipment" {
			continue
		}
		artifact, _, parseErr := parseFile(group[0].path)
		if parseErr != nil {
			continue
		}
		for _, itemID := range NormalizeShipmentItems(artifact) {
			members[itemID] = struct{}{}
		}
	}
	return members
}

// scanReturnedFeatureIDs walks the flat per-item JSONL event log directory and
// returns the union of every "feature_id" recorded on a returned_to_backlog
// event (appended by returnUnreleasedFeatureItems when a covering feature's
// child is excluded from a partial-feature shipment). Detection intentionally
// reads this event provenance rather than parent_id, because
// returnUnreleasedFeatureItems clears parent_id on the returned item -- the
// hierarchical ID prefix alone does not reliably reconstruct which feature a
// returned item was released FROM. A missing logs directory is treated as "no
// returned items" rather than an error.
func scanReturnedFeatureIDs(logsDir string) (map[string]struct{}, error) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf("read logs dir: %w", err)
	}

	returned := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		path := filepath.Join(logsDir, entry.Name())
		func() {
			f, openErr := os.Open(path)
			if openErr != nil {
				return
			}
			defer f.Close()

			dec := json.NewDecoder(f)
			for {
				var ev struct {
					EventType string         `json:"event_type"`
					Delta     map[string]any `json:"delta"`
				}
				if decErr := dec.Decode(&ev); decErr != nil {
					return
				}
				if ev.EventType != "returned_to_backlog" {
					continue
				}
				if featureID, ok := ev.Delta["feature_id"].(string); ok && featureID != "" {
					returned[featureID] = struct{}{}
				}
			}
		}()
	}
	return returned, nil
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
