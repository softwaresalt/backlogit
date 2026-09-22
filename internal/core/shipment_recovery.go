package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ShipmentBlockedSnapshotSchemaVersion identifies the machine-readable input
// accepted by NormalizeBlockedShipment and roll-forward recovery.
const ShipmentBlockedSnapshotSchemaVersion = "shipment-bootstrap-snapshot/v1"

// ShipmentBlockedSnapshot is the authoritative normalizer input for
// reconstructing a blocked shipment and its member-status preimage.
type ShipmentBlockedSnapshot struct {
	SchemaVersion       string            `json:"schema_version"`
	ShipmentID          string            `json:"shipment_id"`
	Branch              string            `json:"branch,omitempty"`
	Target              ShipmentStatus    `json:"target"`
	BlockedReason       string            `json:"blocked_reason"`
	BlockedAt           string            `json:"blocked_at"`
	BlockedBy           string            `json:"blocked_by,omitempty"`
	ResumeCheckpointRef string            `json:"resume_checkpoint_ref,omitempty"`
	Members             map[string]string `json:"members"`
}

func readShipmentBlockedSnapshot(
	ws *Workspace,
	reference string,
	shipmentID string,
	memberIDs []string,
) (ShipmentBlockedSnapshot, error) {
	var snapshot ShipmentBlockedSnapshot
	if strings.TrimSpace(reference) == "" || filepath.IsAbs(reference) {
		return snapshot, fmt.Errorf("shipment %s snapshot reference must be a non-empty relative path: %w",
			shipmentID, blerrors.ErrValidation)
	}
	cleanReference := filepath.Clean(filepath.FromSlash(reference))
	if cleanReference == "." || cleanReference == ".." ||
		strings.HasPrefix(cleanReference, ".."+string(filepath.Separator)) {
		return snapshot, fmt.Errorf("shipment %s snapshot reference escapes the workspace: %w",
			shipmentID, blerrors.ErrValidation)
	}
	snapshotPath, err := resolveContainedArtifactPath(
		ws,
		filepath.Join(WorkspaceStorageRoot(ws.RootPath), cleanReference),
	)
	if err != nil {
		return snapshot, fmt.Errorf("resolve shipment %s snapshot reference: %w", shipmentID, err)
	}
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		return snapshot, fmt.Errorf("read shipment %s snapshot: %w", shipmentID, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return snapshot, fmt.Errorf("decode shipment %s snapshot: %w", shipmentID, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return snapshot, fmt.Errorf("decode shipment %s snapshot trailing content: %w",
			shipmentID, blerrors.ErrValidation)
	}
	if snapshot.SchemaVersion != ShipmentBlockedSnapshotSchemaVersion ||
		snapshot.ShipmentID != shipmentID ||
		snapshot.Target != ShipmentBlocked ||
		strings.TrimSpace(snapshot.BlockedReason) == "" {
		return snapshot, fmt.Errorf("shipment %s snapshot does not satisfy %s: %w",
			shipmentID, ShipmentBlockedSnapshotSchemaVersion, blerrors.ErrValidation)
	}
	if _, err := time.Parse(time.RFC3339, snapshot.BlockedAt); err != nil {
		return snapshot, fmt.Errorf("shipment %s snapshot blocked_at: %w", shipmentID, blerrors.ErrValidation)
	}
	if len(snapshot.Members) != len(memberIDs) {
		return snapshot, fmt.Errorf("shipment %s snapshot does not exactly cover its manifest: %w",
			shipmentID, blerrors.ErrShipmentConflict)
	}
	for _, memberID := range memberIDs {
		status, found := snapshot.Members[memberID]
		if !found {
			return snapshot, fmt.Errorf("shipment %s snapshot is missing member %s: %w",
				shipmentID, memberID, blerrors.ErrShipmentConflict)
		}
		switch models.ArtifactStatus(status) {
		case models.StatusQueued, models.StatusActive, models.StatusBlocked, models.StatusReview,
			models.StatusDone, models.StatusAccepted, models.StatusRejected, models.StatusArchived,
			models.StatusShipped, models.StatusAbandoned:
		default:
			return snapshot, fmt.Errorf("shipment %s snapshot member %s has invalid status %q: %w",
				shipmentID, memberID, status, blerrors.ErrValidation)
		}
	}
	return snapshot, nil
}

func reconcileShipmentLifecycleIntent(
	ctx context.Context,
	ws *Workspace,
	journalPath string,
	journal shipmentLifecycleJournal,
) (*models.Artifact, error) {
	if journal.SchemaVersion != "shipment-operation/v1" ||
		journal.Phase != "intent" ||
		journal.CorrelationID == "" ||
		journal.ShipmentID == "" ||
		journal.Preimage.Shipment == nil ||
		journal.Preimage.Shipment.ID != journal.ShipmentID ||
		journal.Preimage.Shipment.ArtifactType != "shipment" {
		return nil, fmt.Errorf("shipment lifecycle journal %s is incomplete: %w",
			journalPath, blerrors.ErrValidation)
	}
	if journal.Operation != "block" && journal.Operation != "unblock" && journal.Operation != "normalize" {
		return nil, fmt.Errorf("shipment lifecycle journal %s has unsupported operation %q: %w",
			journalPath, journal.Operation, blerrors.ErrValidation)
	}

	memberIDs := NormalizeShipmentItems(journal.Preimage.Shipment)
	preimageMembers := make(map[string]*models.Artifact, len(journal.Preimage.Members))
	for _, member := range journal.Preimage.Members {
		if member == nil || member.ID == "" {
			return nil, fmt.Errorf("shipment lifecycle journal %s has an incomplete member preimage: %w",
				journalPath, blerrors.ErrValidation)
		}
		if _, duplicate := preimageMembers[member.ID]; duplicate {
			return nil, fmt.Errorf("shipment lifecycle journal %s repeats member %s: %w",
				journalPath, member.ID, blerrors.ErrValidation)
		}
		preimageMembers[member.ID] = member
	}
	if len(preimageMembers) != len(memberIDs) {
		return nil, fmt.Errorf("shipment lifecycle journal %s preimage does not cover the manifest: %w",
			journalPath, blerrors.ErrShipmentConflict)
	}
	for _, memberID := range memberIDs {
		if preimageMembers[memberID] == nil {
			return nil, fmt.Errorf("shipment lifecycle journal %s is missing member %s: %w",
				journalPath, memberID, blerrors.ErrShipmentConflict)
		}
	}

	var snapshot *ShipmentBlockedSnapshot
	if journal.RecoveryPolicy == "roll_forward" {
		loadedSnapshot, err := readShipmentBlockedSnapshot(ws, journal.SnapshotRef, journal.ShipmentID, memberIDs)
		if err != nil {
			return nil, err
		}
		snapshot = &loadedSnapshot
	}
	if err := validateShipmentLifecycleRecoveryCAS(
		ctx,
		ws,
		journal,
		memberIDs,
		preimageMembers,
		snapshot,
	); err != nil {
		return nil, err
	}

	operationCtx := withShipmentOperation(ctx, journal.CorrelationID)
	governedCtx := context.WithValue(operationCtx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
		correlationID:                 journal.CorrelationID,
		operation:                     journal.Operation + "_recovery",
		allowGovernedShipmentMutation: true,
	})
	var result *models.Artifact
	switch journal.RecoveryPolicy {
	case "rollback":
		if journal.Operation == "unblock" {
			for _, memberID := range memberIDs {
				if err := persistArtifact(operationCtx, ws, cloneArtifact(preimageMembers[memberID]), true); err != nil {
					return nil, fmt.Errorf("restore shipment %s member %s preimage: %w",
						journal.ShipmentID, memberID, err)
				}
			}
			if err := persistArtifact(governedCtx, ws, cloneArtifact(journal.Preimage.Shipment), true); err != nil {
				return nil, fmt.Errorf("restore shipment %s preimage: %w", journal.ShipmentID, err)
			}
		} else {
			if err := persistArtifact(governedCtx, ws, cloneArtifact(journal.Preimage.Shipment), true); err != nil {
				return nil, fmt.Errorf("restore shipment %s preimage: %w", journal.ShipmentID, err)
			}
			for _, memberID := range memberIDs {
				if err := persistArtifact(operationCtx, ws, cloneArtifact(preimageMembers[memberID]), true); err != nil {
					return nil, fmt.Errorf("restore shipment %s member %s preimage: %w",
						journal.ShipmentID, memberID, err)
				}
			}
		}
		result = cloneArtifact(journal.Preimage.Shipment)
		journal.Phase = "compensated"
	case "roll_forward":
		for _, memberID := range memberIDs {
			member := cloneArtifact(preimageMembers[memberID])
			member.Status = models.ArtifactStatus(snapshot.Members[memberID])
			if member.Status == models.StatusActive || member.Status == models.StatusReview {
				member.Status = models.StatusQueued
			}
			member.UpdatedAt = models.NowUTC()
			if err := persistArtifact(operationCtx, ws, member, true); err != nil {
				return nil, fmt.Errorf("roll forward shipment %s member %s: %w",
					journal.ShipmentID, memberID, err)
			}
		}
		blocked := cloneArtifact(journal.Preimage.Shipment)
		blocked.Status = models.StatusBlocked
		blocked.UpdatedAt = models.NowUTC()
		if blocked.CustomFields == nil {
			blocked.CustomFields = map[string]any{}
		}
		if snapshot.Branch != "" {
			blocked.CustomFields["branch"] = snapshot.Branch
		}
		blocked.CustomFields["blocked_reason"] = snapshot.BlockedReason
		blocked.CustomFields["blocked_at"] = snapshot.BlockedAt
		blocked.CustomFields["blocked_by"] = snapshot.BlockedBy
		blocked.CustomFields["member_status_snapshot"] = maps.Clone(snapshot.Members)
		if snapshot.ResumeCheckpointRef == "" {
			delete(blocked.CustomFields, "resume_checkpoint_ref")
		} else {
			blocked.CustomFields["resume_checkpoint_ref"] = snapshot.ResumeCheckpointRef
		}
		if err := persistArtifact(governedCtx, ws, blocked, true); err != nil {
			return nil, fmt.Errorf("roll forward shipment %s: %w", journal.ShipmentID, err)
		}
		result = blocked
		journal.Phase = "committed"
	default:
		return nil, fmt.Errorf("shipment lifecycle journal %s has unsupported recovery policy %q: %w",
			journalPath, journal.RecoveryPolicy, blerrors.ErrValidation)
	}

	target := journal.Target
	if journal.Phase == "compensated" {
		target = string(journal.Preimage.Shipment.Status)
	}
	eventDelta := map[string]any{
		"correlation_id": journal.CorrelationID,
		"operation":      journal.Operation,
		"phase":          "applied",
		"status":         target,
		"target":         target,
		"snapshot_ref":   journal.SnapshotRef,
	}
	if err := appendItemEventWithActorErr(
		operationCtx,
		ws,
		journal.ShipmentID,
		journal.BlockedBy,
		"shipment_status_changed",
		eventDelta,
	); err != nil {
		return nil, fmt.Errorf("append shipment %s recovery status evidence: %w", journal.ShipmentID, err)
	}
	terminalDelta := maps.Clone(eventDelta)
	terminalDelta["phase"] = journal.Phase
	if err := appendItemEventWithActorErr(
		operationCtx,
		ws,
		journal.ShipmentID,
		journal.BlockedBy,
		"shipment_lifecycle",
		terminalDelta,
	); err != nil {
		return nil, fmt.Errorf("append shipment %s recovery terminal evidence: %w", journal.ShipmentID, err)
	}
	if _, err := writeShipmentLifecycleJournalForWorkspace(ws, filepath.Base(journalPath), journal); err != nil {
		return nil, fmt.Errorf("persist shipment %s recovery terminal journal: %w", journal.ShipmentID, err)
	}
	return result, nil
}

func validateShipmentLifecycleRecoveryCAS(
	ctx context.Context,
	ws *Workspace,
	journal shipmentLifecycleJournal,
	memberIDs []string,
	preimageMembers map[string]*models.Artifact,
	snapshot *ShipmentBlockedSnapshot,
) error {
	// Legacy branch-bootstrap journals use operation=block with an explicitly
	// authoritative roll-forward snapshot. Their contract permits replacement
	// of an otherwise unprovable partial bootstrap image; normalize journals,
	// by contrast, originate from an observed live aggregate and require the
	// preimage/target CAS enforced below.
	if journal.RecoveryPolicy == "roll_forward" && journal.Operation == "block" {
		return nil
	}

	currentShipment, err := findArtifact(ctx, ws, journal.ShipmentID)
	if err != nil {
		return fmt.Errorf("load shipment %s recovery CAS state: %w", journal.ShipmentID, err)
	}
	shipmentCandidates, err := shipmentRecoveryCandidates(journal, currentShipment, snapshot)
	if err != nil {
		return err
	}
	matches, err := recoveryArtifactMatchesAny(currentShipment, shipmentCandidates)
	if err != nil {
		return fmt.Errorf("compare shipment %s recovery CAS state: %w", journal.ShipmentID, err)
	}
	if !matches {
		return fmt.Errorf(
			"shipment %s diverged from its durable lifecycle preimage: %w",
			journal.ShipmentID,
			blerrors.ErrShipmentConflict,
		)
	}

	for _, memberID := range memberIDs {
		currentMember, loadErr := findArtifact(ctx, ws, memberID)
		if loadErr != nil {
			return fmt.Errorf("load shipment %s member %s recovery CAS state: %w",
				journal.ShipmentID, memberID, loadErr)
		}
		memberCandidates, candidateErr := memberRecoveryCandidates(
			journal,
			preimageMembers[memberID],
			snapshot,
		)
		if candidateErr != nil {
			return candidateErr
		}
		matches, compareErr := recoveryArtifactMatchesAny(currentMember, memberCandidates)
		if compareErr != nil {
			return fmt.Errorf("compare shipment %s member %s recovery CAS state: %w",
				journal.ShipmentID, memberID, compareErr)
		}
		if !matches {
			return fmt.Errorf(
				"shipment %s member %s diverged from its durable lifecycle preimage: %w",
				journal.ShipmentID,
				memberID,
				blerrors.ErrShipmentConflict,
			)
		}
	}
	return nil
}

type recoveryArtifactCandidate struct {
	artifact        *models.Artifact
	ignoreUpdatedAt bool
}

func recoveryArtifactMatchesAny(
	current *models.Artifact,
	candidates []recoveryArtifactCandidate,
) (bool, error) {
	for _, candidate := range candidates {
		left := cloneArtifact(current)
		right := cloneArtifact(candidate.artifact)
		if left == nil || right == nil {
			continue
		}
		if candidate.ignoreUpdatedAt {
			left.UpdatedAt = time.Time{}
			right.UpdatedAt = time.Time{}
		}
		leftJSON, err := json.Marshal(left)
		if err != nil {
			return false, fmt.Errorf("marshal current artifact: %w", err)
		}
		rightJSON, err := json.Marshal(right)
		if err != nil {
			return false, fmt.Errorf("marshal recovery candidate: %w", err)
		}
		if bytes.Equal(leftJSON, rightJSON) {
			return true, nil
		}
	}
	return false, nil
}

func shipmentRecoveryCandidates(
	journal shipmentLifecycleJournal,
	current *models.Artifact,
	snapshot *ShipmentBlockedSnapshot,
) ([]recoveryArtifactCandidate, error) {
	candidates := []recoveryArtifactCandidate{{artifact: journal.Preimage.Shipment}}
	target := cloneArtifact(journal.Preimage.Shipment)
	switch {
	case journal.RecoveryPolicy == "rollback" && journal.Operation == "block":
		if current.Status != models.StatusBlocked {
			return candidates, nil
		}
		blockedAt, found := current.CustomFields["blocked_at"]
		if !found || !validRecoveryBlockedAt(blockedAt) {
			return candidates, nil
		}
		target.Status = models.StatusBlocked
		target.UpdatedAt = current.UpdatedAt
		if target.CustomFields == nil {
			target.CustomFields = map[string]any{}
		}
		target.CustomFields["blocked_reason"] = journal.Reason
		target.CustomFields["blocked_at"] = blockedAt
		target.CustomFields["blocked_by"] = journal.BlockedBy
		memberStatuses := make(map[string]string, len(journal.Preimage.Members))
		for _, member := range journal.Preimage.Members {
			memberStatuses[member.ID] = string(member.Status)
		}
		target.CustomFields["member_status_snapshot"] = memberStatuses
		if journal.SnapshotRef == "" {
			delete(target.CustomFields, "resume_checkpoint_ref")
		} else {
			target.CustomFields["resume_checkpoint_ref"] = journal.SnapshotRef
		}
	case journal.RecoveryPolicy == "rollback" && journal.Operation == "unblock":
		target.Status = models.ArtifactStatus(journal.Target)
		delete(target.CustomFields, "blocked_reason")
		delete(target.CustomFields, "blocked_at")
		delete(target.CustomFields, "blocked_by")
	case journal.RecoveryPolicy == "roll_forward" && journal.Operation == "normalize":
		if snapshot == nil {
			return nil, fmt.Errorf("normalize shipment %s recovery has no snapshot: %w",
				journal.ShipmentID, blerrors.ErrShipmentConflict)
		}
		target.Status = models.StatusBlocked
		if target.CustomFields == nil {
			target.CustomFields = map[string]any{}
		}
		if snapshot.Branch != "" {
			target.CustomFields["branch"] = snapshot.Branch
		}
		target.CustomFields["blocked_reason"] = snapshot.BlockedReason
		target.CustomFields["blocked_at"] = snapshot.BlockedAt
		target.CustomFields["blocked_by"] = snapshot.BlockedBy
		target.CustomFields["member_status_snapshot"] = maps.Clone(snapshot.Members)
		if snapshot.ResumeCheckpointRef == "" {
			delete(target.CustomFields, "resume_checkpoint_ref")
		} else {
			target.CustomFields["resume_checkpoint_ref"] = snapshot.ResumeCheckpointRef
		}
	default:
		return nil, fmt.Errorf(
			"shipment %s recovery cannot prove operation %s policy %s: %w",
			journal.ShipmentID,
			journal.Operation,
			journal.RecoveryPolicy,
			blerrors.ErrShipmentConflict,
		)
	}
	return append(candidates, recoveryArtifactCandidate{artifact: target, ignoreUpdatedAt: true}), nil
}

func memberRecoveryCandidates(
	journal shipmentLifecycleJournal,
	preimage *models.Artifact,
	snapshot *ShipmentBlockedSnapshot,
) ([]recoveryArtifactCandidate, error) {
	candidates := []recoveryArtifactCandidate{{artifact: preimage}}
	target := cloneArtifact(preimage)
	switch {
	case journal.RecoveryPolicy == "rollback" && journal.Operation == "block":
		if preimage.Status != models.StatusActive && preimage.Status != models.StatusReview {
			return candidates, nil
		}
		target.Status = models.StatusQueued
	case journal.RecoveryPolicy == "rollback" && journal.Operation == "unblock":
		target.Status = models.StatusQueued
		if journal.Target == string(ShipmentActive) {
			statuses, err := recoveryMemberStatusSnapshot(journal.Preimage.Shipment)
			if err != nil {
				return nil, err
			}
			status, found := statuses[preimage.ID]
			if !found {
				return nil, fmt.Errorf("shipment %s recovery snapshot is missing member %s: %w",
					journal.ShipmentID, preimage.ID, blerrors.ErrShipmentConflict)
			}
			target.Status = models.ArtifactStatus(status)
		}
	case journal.RecoveryPolicy == "roll_forward" && journal.Operation == "normalize":
		if snapshot == nil {
			return nil, fmt.Errorf("normalize shipment %s recovery has no snapshot: %w",
				journal.ShipmentID, blerrors.ErrShipmentConflict)
		}
		target.Status = models.ArtifactStatus(snapshot.Members[preimage.ID])
		if target.Status == models.StatusActive || target.Status == models.StatusReview {
			target.Status = models.StatusQueued
		}
	default:
		return nil, fmt.Errorf(
			"shipment %s recovery cannot prove member %s operation %s policy %s: %w",
			journal.ShipmentID,
			preimage.ID,
			journal.Operation,
			journal.RecoveryPolicy,
			blerrors.ErrShipmentConflict,
		)
	}
	return append(candidates, recoveryArtifactCandidate{artifact: target, ignoreUpdatedAt: true}), nil
}

func recoveryMemberStatusSnapshot(shipment *models.Artifact) (map[string]string, error) {
	if shipment == nil || shipment.CustomFields == nil {
		return nil, fmt.Errorf("shipment recovery preimage has no member status snapshot: %w",
			blerrors.ErrShipmentConflict)
	}
	rawSnapshot, found := shipment.CustomFields["member_status_snapshot"]
	if !found {
		return nil, fmt.Errorf("shipment recovery preimage has no member status snapshot: %w",
			blerrors.ErrShipmentConflict)
	}
	statuses := make(map[string]string)
	switch values := rawSnapshot.(type) {
	case map[string]string:
		maps.Copy(statuses, values)
	case map[string]any:
		for memberID, value := range values {
			status, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("shipment recovery member %s status is not a string: %w",
					memberID, blerrors.ErrShipmentConflict)
			}
			statuses[memberID] = status
		}
	default:
		return nil, fmt.Errorf("shipment recovery preimage has an invalid member status snapshot: %w",
			blerrors.ErrShipmentConflict)
	}
	return statuses, nil
}

func validRecoveryBlockedAt(value any) bool {
	switch blockedAt := value.(type) {
	case string:
		_, err := time.Parse(time.RFC3339, blockedAt)
		return err == nil
	case time.Time:
		return !blockedAt.IsZero()
	default:
		return false
	}
}

// NormalizeBlockedShipment reconstructs a blocked shipment from an
// authoritative snapshot, requeues in-flight members, and records canonical
// lifecycle evidence. It refuses when the snapshot cannot prove the current
// aggregate state.
func NormalizeBlockedShipment(
	ctx context.Context,
	ws *Workspace,
	shipmentID string,
	snapshotRef string,
	actor string,
) (*models.Artifact, error) {
	return normalizeBlockedShipment(ctx, ws, shipmentID, snapshotRef, actor, true)
}

// NormalizeBlockedShipmentForRecovery normalizes a blocked shipment without
// first auto-recovering unrelated journals. It is reserved for diagnostic
// remediation when ordinary workspace initialization is blocked by a poison
// journal; the target aggregate still undergoes the normal snapshot CAS.
func NormalizeBlockedShipmentForRecovery(
	ctx context.Context,
	ws *Workspace,
	shipmentID string,
	snapshotRef string,
	actor string,
) (*models.Artifact, error) {
	return normalizeBlockedShipment(ctx, ws, shipmentID, snapshotRef, actor, false)
}

func normalizeBlockedShipment(
	ctx context.Context,
	ws *Workspace,
	shipmentID string,
	snapshotRef string,
	actor string,
	recoverPending bool,
) (*models.Artifact, error) {
	globalUnlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment lifecycle: %w", err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			_ = unlockErr
		}
	}()
	ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockContextKey{}, struct{}{})
	if recoverPending {
		if err := recoverPendingShipmentOperations(ctx, ws); err != nil {
			return nil, fmt.Errorf("recover pending shipment operations before normalize: %w", err)
		}
	}

	membershipUnlock, err := lockShipmentMembership(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s membership: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := membershipUnlock(); unlockErr != nil {
			_ = unlockErr
		}
	}()
	shipment, err := findArtifact(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("load shipment %s: %w", shipmentID, err)
	}
	if shipment.ArtifactType != "shipment" {
		return nil, fmt.Errorf("normalize shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
	}
	if shipment.Status != models.StatusBlocked {
		return nil, fmt.Errorf("normalize shipment %s from %s: %w",
			shipmentID, shipment.Status, blerrors.ErrShipmentConflict)
	}
	memberIDs := NormalizeShipmentItems(shipment)
	lockIDs := append([]string{shipmentID}, memberIDs...)
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, lockIDs)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s aggregate: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			_ = unlockErr
		}
	}()
	shipment, err = findArtifact(lockedCtx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("reload shipment %s under lock: %w", shipmentID, err)
	}
	if shipment.Status != models.StatusBlocked || !slices.Equal(NormalizeShipmentItems(shipment), memberIDs) {
		return nil, fmt.Errorf("shipment %s changed while acquiring normalization locks: %w",
			shipmentID, blerrors.ErrShipmentConflict)
	}
	snapshot, err := readShipmentBlockedSnapshot(ws, snapshotRef, shipmentID, memberIDs)
	if err != nil {
		return nil, err
	}
	preimage := shipmentLifecyclePreimage{
		Shipment: cloneArtifact(shipment),
		Members:  make([]*models.Artifact, 0, len(memberIDs)),
	}
	for _, memberID := range memberIDs {
		member, loadErr := findArtifact(lockedCtx, ws, memberID)
		if loadErr != nil {
			return nil, fmt.Errorf("load shipment %s member %s: %w", shipmentID, memberID, loadErr)
		}
		snapshotStatus := models.ArtifactStatus(snapshot.Members[memberID])
		blockedStatus := snapshotStatus
		if blockedStatus == models.StatusActive || blockedStatus == models.StatusReview {
			blockedStatus = models.StatusQueued
		}
		if member.Status != snapshotStatus && member.Status != blockedStatus {
			return nil, fmt.Errorf(
				"shipment %s member %s state %s cannot be proven from snapshot status %s: %w",
				shipmentID, memberID, member.Status, snapshotStatus, blerrors.ErrShipmentConflict,
			)
		}
		preimage.Members = append(preimage.Members, cloneArtifact(member))
	}

	var correlationBytes [16]byte
	if _, err := rand.Read(correlationBytes[:]); err != nil {
		return nil, fmt.Errorf("generate normalize shipment correlation id: %w", err)
	}
	correlationID := hex.EncodeToString(correlationBytes[:])
	journal := shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  correlationID,
		Phase:          "intent",
		Operation:      "normalize",
		RecoveryPolicy: "roll_forward",
		ShipmentID:     shipmentID,
		Target:         string(ShipmentBlocked),
		Reason:         snapshot.BlockedReason,
		BlockedBy:      actor,
		SnapshotRef:    snapshotRef,
		Preimage:       preimage,
	}
	journalName := shipmentLifecycleJournalName(correlationID)
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal)
	if err != nil {
		return nil, fmt.Errorf("persist normalize shipment %s intent: %w", shipmentID, err)
	}
	return reconcileShipmentLifecycleIntent(lockedCtx, ws, journalPath, journal)
}
