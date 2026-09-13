package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

var supportedLegacyShippedPreStates = map[string]struct{}{
	string(ShipmentActive): {},
}

var supportedLegacyReconcileMemberTerminalStatuses = map[models.ArtifactStatus]struct{}{
	models.StatusDone:     {},
	models.StatusAccepted: {},
}

// validateShipmentReconcilePreconditions implements 167.014-T's U2 precondition
// gate for governed archived-shipment reconciliation (#423). It is a pure
// validation step: it reloads the archived shipment and every manifest member
// from Markdown, re-confirms branch 2d under the locked batch via the shared
// classifier, and returns the validated shipment artifact without performing any
// writes.
func validateShipmentReconcilePreconditions(ctx context.Context, ws *Workspace, req ShipmentShippedReconcileRequest, requestIdentityDigest string) (*models.Artifact, error) {
	if ws == nil {
		return nil, fmt.Errorf("validate shipment reconcile preconditions: workspace is required: %w", blerrors.ErrValidation)
	}
	if err := validateShipmentReconcileApproverSeparation(req); err != nil {
		return nil, err
	}
	requestIdentityDigest = strings.TrimSpace(requestIdentityDigest)
	if requestIdentityDigest == "" {
		return nil, fmt.Errorf("validate shipment reconcile preconditions: request identity digest is required: %w", blerrors.ErrValidation)
	}

	shipment, shipmentFrontmatter, err := loadShipmentReconcileArchivedShipment(ctx, ws, req.ShipmentID)
	if err != nil {
		return nil, err
	}
	if err := validateShipmentReconcileShipmentPreState(shipment); err != nil {
		return nil, err
	}
	if err := validateShipmentReconcileClassifierState(ws, shipment, shipmentFrontmatter, req, requestIdentityDigest); err != nil {
		return nil, err
	}
	if err := validateShipmentReconcileManifestMembers(ctx, ws, NormalizeShipmentItems(shipment)); err != nil {
		return nil, err
	}
	return shipment, nil
}

func validateShipmentReconcileApproverSeparation(req ShipmentShippedReconcileRequest) error {
	actor := strings.TrimSpace(req.Actor)
	secondApprover := strings.TrimSpace(req.SecondApprover)
	if req.SecondApprover != "" && secondApprover == "" {
		return fmt.Errorf("validate shipment reconcile preconditions: second_approver must be non-empty when supplied: %w", blerrors.ErrValidation)
	}
	if secondApprover != "" && secondApprover == actor {
		return fmt.Errorf("validate shipment reconcile preconditions: second_approver must differ from actor: %w", blerrors.ErrValidation)
	}
	return nil
}

func loadShipmentReconcileArchivedShipment(ctx context.Context, ws *Workspace, shipmentID string) (*models.Artifact, map[string]any, error) {
	shipment, err := findArtifact(ctx, ws, shipmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("validate shipment reconcile preconditions: load shipment %s from markdown: %w", shipmentID, err)
	}
	if shipment.ArtifactType != "shipment" {
		return nil, nil, fmt.Errorf("validate shipment reconcile preconditions: artifact %s is not a shipment: %w", shipmentID, blerrors.ErrValidation)
	}

	shipmentPath, err := FindArtifactPath(ctx, ws, shipmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("validate shipment reconcile preconditions: resolve shipment %s path: %w", shipmentID, err)
	}
	archiveDir := filepath.Join(workspaceStorageRoot(ws), "archive")
	if !pathWithinDir(shipmentPath, archiveDir) {
		return nil, nil, fmt.Errorf("validate shipment reconcile preconditions: shipment %s must resolve under %s: %w", shipmentID, archiveDir, blerrors.ErrValidation)
	}
	if err := ensureShipmentReconcileArchiveOnlyLocation(ws, shipmentID, archiveDir); err != nil {
		return nil, nil, err
	}

	raw, err := os.ReadFile(shipmentPath)
	if err != nil {
		return nil, nil, fmt.Errorf("validate shipment reconcile preconditions: read shipment %s frontmatter: %w", shipmentID, err)
	}
	frontmatter, _, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("validate shipment reconcile preconditions: parse shipment %s frontmatter: %w", shipmentID, err)
	}
	if frontmatter == nil {
		frontmatter = map[string]any{}
	}
	return shipment, frontmatter, nil
}

func pathWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// ensureShipmentReconcileArchiveOnlyLocation fails closed if shipmentID
// resolves to any location OTHER than archiveDir (167.014-T, PR #440 review
// round 2, finding 5). FindArtifactPath returns only the FIRST match across
// the registry's search roots (archive is searched before queue/live
// locations per the default registry config, see config.DefaultRegistry);
// on its own that does NOT prove the id exists ONLY in the archive. A
// duplicate record for the same id in a live/queue location should never
// legitimately occur, but must not be silently trusted: proceeding to
// reconcile the archived copy while a live record for the same id still
// exists elsewhere is precisely the ambiguous state the "requires no live
// queue record" precondition exists to prevent. This performs a targeted
// check against the registry's OTHER known search roots (not a full
// re-scan duplicating FindArtifactPath's own multi-directory search).
func ensureShipmentReconcileArchiveOnlyLocation(ws *Workspace, shipmentID, archiveDir string) error {
	searchDirs, err := artifactSearchDirs(ws)
	if err != nil {
		return fmt.Errorf("validate shipment reconcile preconditions: enumerate registry search directories for %s: %w", shipmentID, err)
	}
	archiveDirAbs, err := filepath.Abs(archiveDir)
	if err != nil {
		return fmt.Errorf("validate shipment reconcile preconditions: resolve archive directory for %s: %w", shipmentID, err)
	}
	for _, dirPath := range searchDirs {
		dirAbs, absErr := filepath.Abs(dirPath)
		if absErr != nil {
			return fmt.Errorf("validate shipment reconcile preconditions: resolve search directory %s for %s: %w", dirPath, shipmentID, absErr)
		}
		if dirAbs == archiveDirAbs {
			continue
		}
		found, findErr := findArtifactInSearchDir(ws, dirPath, shipmentID)
		if findErr != nil {
			return fmt.Errorf("validate shipment reconcile preconditions: search %s for a duplicate record of %s: %w", dirPath, shipmentID, findErr)
		}
		if found != "" {
			return fmt.Errorf("validate shipment reconcile preconditions: shipment %s resolves to more than one location (archive and %s); ambiguous/indeterminate reconciliation target: %w", shipmentID, found, blerrors.ErrShipmentReconcileConflict)
		}
	}
	return nil
}

// validateShipmentReconcileShipmentPreState runs both the identity and
// legacy-value pre-state checks in sequence (identity first). It is used by
// validateShipmentReconcilePreconditions (167.014-T's standalone precondition
// gate), where both checks must always run unconditionally regardless of the
// classifier's outcome.
func validateShipmentReconcileShipmentPreState(shipment *models.Artifact) error {
	if err := validateShipmentReconcileShipmentIdentity(shipment); err != nil {
		return err
	}
	return validateShipmentReconcileShipmentLegacyPreState(shipment)
}

// validateShipmentReconcileShipmentIdentity confirms the shipment is
// currently located under the governed archived namespace (the top-level
// `status` field). This is the identity/location gate: it is always safe to
// run unconditionally, even for replay/no_op/conflict continuations, because
// reconcile only ever mutates the `archived_status` sub-field — never this
// top-level `status` — so it is invariant across the whole reconcile
// lifecycle for a legacy-repair target.
func validateShipmentReconcileShipmentIdentity(shipment *models.Artifact) error {
	if shipment.Status != models.StatusArchived {
		return fmt.Errorf("validate shipment reconcile preconditions: shipment %s status=%q must be archived for governed reconciliation: %w", shipment.ID, shipment.Status, blerrors.ErrUnsupportedLegacyPreState)
	}
	return nil
}

// validateShipmentReconcileShipmentLegacyPreState confirms the shipment's
// `archived_status` sub-field is one of the supported pre-repair legacy
// values. Unlike the identity check, this is NOT invariant across the
// reconcile lifecycle: a successful reconcile rewrites `archived_status` to
// "shipped", so this check must only be run in the fresh-repair
// (ShipmentReconcileOutcomeReconciled) path — never unconditionally ahead of
// classification — or every legitimate no_op/conflict replay against an
// already-shipped shipment would be rejected before the classifier ever runs.
func validateShipmentReconcileShipmentLegacyPreState(shipment *models.Artifact) error {
	if _, ok := supportedLegacyShippedPreStates[shipment.ArchivedStatus]; !ok {
		return fmt.Errorf("validate shipment reconcile preconditions: shipment %s archived_status=%q is not in supportedLegacyShippedPreStates={%s}: %w", shipment.ID, shipment.ArchivedStatus, string(ShipmentActive), blerrors.ErrUnsupportedLegacyPreState)
	}
	return nil
}

func validateShipmentReconcileClassifierState(ws *Workspace, shipment *models.Artifact, frontmatter map[string]any, req ShipmentShippedReconcileRequest, requestIdentityDigest string) error {
	logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), shipment.ID)
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("validate shipment reconcile preconditions: read shipment %s item log: %w", shipment.ID, err)
		}
		logBytes = nil
	}

	outcome, err := classifyShipmentReconcileState(logBytes, frontmatter, req.IdempotencyKey, requestIdentityDigest, shipment.ID)
	if err != nil {
		return fmt.Errorf("validate shipment reconcile preconditions: classify shipment %s: %w", shipment.ID, err)
	}
	if outcome != ShipmentReconcileOutcomeReconciled {
		return fmt.Errorf("validate shipment reconcile preconditions: shipment %s re-confirmed as %s instead of reconciled under locked batch: %w", shipment.ID, outcome, blerrors.ErrShipmentReconcileConflict)
	}
	return nil
}

func validateShipmentReconcileManifestMembers(ctx context.Context, ws *Workspace, memberIDs []string) error {
	if len(memberIDs) == 0 {
		return fmt.Errorf("validate shipment reconcile preconditions: manifest must be non-empty: %w", blerrors.ErrValidation)
	}

	for _, memberID := range memberIDs {
		member, err := findArtifact(ctx, ws, memberID)
		if err != nil {
			return fmt.Errorf("validate shipment reconcile preconditions: reload shipment member %s from markdown: %w", memberID, err)
		}
		if err := validateShipmentReconcileManifestMember(member); err != nil {
			return err
		}
	}
	return nil
}

func validateShipmentReconcileManifestMember(member *models.Artifact) error {
	if _, ok := supportedLegacyReconcileMemberTerminalStatuses[member.Status]; ok {
		return nil
	}
	if member.Status == models.StatusRejected {
		return fmt.Errorf("validate shipment reconcile preconditions: shipment member %s is rejected and v1 cannot prove legacy descopes: %w", member.ID, blerrors.ErrUnsupportedLegacyDescope)
	}
	if member.Status == models.StatusAbandoned {
		return fmt.Errorf("validate shipment reconcile preconditions: shipment member %s is abandoned and v1 cannot prove legacy descopes: %w", member.ID, blerrors.ErrUnsupportedLegacyDescope)
	}
	if member.Status == models.StatusArchived {
		archivedStatus := models.ArtifactStatus(member.ArchivedStatus)
		if _, ok := supportedLegacyReconcileMemberTerminalStatuses[archivedStatus]; ok {
			return nil
		}
		return fmt.Errorf("validate shipment reconcile preconditions: shipment member %s archived_status=%q is not in supported terminal allowlist {done, accepted}: %w", member.ID, member.ArchivedStatus, blerrors.ErrUnsupportedLegacyDescope)
	}
	return fmt.Errorf("validate shipment reconcile preconditions: shipment member %s status=%q is not in supported terminal allowlist {done, accepted}: %w", member.ID, member.Status, blerrors.ErrValidation)
}
