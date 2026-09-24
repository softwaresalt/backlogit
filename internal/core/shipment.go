package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/config"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ShipmentStatus represents the lifecycle state of a shipment.
type ShipmentStatus string

const (
	// ShipmentQueued indicates the shipment is created but not yet started.
	ShipmentQueued ShipmentStatus = "queued"
	// ShipmentActive indicates the shipment is in progress.
	ShipmentActive ShipmentStatus = "active"
	// ShipmentBlocked indicates the shipment is paused.
	ShipmentBlocked ShipmentStatus = "blocked"
	// ShipmentShipped indicates the shipment has been delivered.
	ShipmentShipped ShipmentStatus = "shipped"
	// ShipmentAbandoned indicates the shipment was cancelled.
	ShipmentAbandoned ShipmentStatus = "abandoned"
)

// BlockOptions contains the declaration-only inputs for blocking a shipment.
type BlockOptions struct {
	Reason              string // REQUIRED non-empty -> blocked_reason (opaque YAML scalar)
	BlockedBy           string // advisory actor -> blocked_by (NOT an auth credential)
	ResumeCheckpointRef string // optional -> resume_checkpoint_ref
}

// UnblockOptions contains the declaration-only inputs for unblocking a shipment.
type UnblockOptions struct {
	Target      ShipmentStatus // REQUIRED; ShipmentQueued or ShipmentActive
	Confirm     bool           // REQUIRED true for BOTH targets (ShipmentQueued and ShipmentActive)
	UnblockedBy string         // advisory actor
}

type shipmentLifecyclePreimage struct {
	Shipment *models.Artifact   `json:"shipment"`
	Members  []*models.Artifact `json:"members"`
	Related  []*models.Artifact `json:"related,omitempty"`
}

type shipmentLifecycleJournal struct {
	SchemaVersion  string                    `json:"schema_version"`
	CorrelationID  string                    `json:"correlation_id"`
	Phase          string                    `json:"phase"`
	Operation      string                    `json:"operation"`
	RecoveryPolicy string                    `json:"recovery_policy"`
	ShipmentID     string                    `json:"shipment_id"`
	Target         string                    `json:"target"`
	Reason         string                    `json:"reason,omitempty"`
	BlockedBy      string                    `json:"blocked_by,omitempty"`
	SnapshotRef    string                    `json:"snapshot_ref,omitempty"`
	Preimage       shipmentLifecyclePreimage `json:"preimage"`
}

const shipmentLifecycleGlobalLockID = "shipment-lifecycle-global"

type governedShipmentActivationContextKey struct{}

type shipmentLifecycleGlobalLockContextKey struct{}

type shipmentLifecycleGlobalLockHookContextKey struct{}

type shipmentLifecycleHeldLocksContextKey struct{}

type shipmentLifecycleHeldLocks struct {
	workspaceRoot string
	ordered       []string
}

// ShipmentLifecycleLockOrderError reports an attempted shipment-lifecycle lock
// acquisition that contradicts the canonical global-before-artifact order.
type ShipmentLifecycleLockOrderError struct {
	RequestedLock string
	CallerName    string
	OrderedHeld   []string
	Reason        string
}

// Error implements error.
func (e *ShipmentLifecycleLockOrderError) Error() string {
	return fmt.Sprintf(
		"shipment lifecycle lock order violation: lock=%s caller=%s held=%v: %s",
		e.RequestedLock,
		e.CallerName,
		e.OrderedHeld,
		e.Reason,
	)
}

// LockID returns the lock whose acquisition was rejected.
func (e *ShipmentLifecycleLockOrderError) LockID() string {
	return e.RequestedLock
}

// Caller returns the function that attempted the rejected acquisition.
func (e *ShipmentLifecycleLockOrderError) Caller() string {
	return e.CallerName
}

// HeldLocks returns the locks held by the caller in acquisition order.
func (e *ShipmentLifecycleLockOrderError) HeldLocks() []string {
	return append([]string(nil), e.OrderedHeld...)
}

func shipmentLifecycleLockCaller() string {
	pc, _, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	function := runtime.FuncForPC(pc)
	if function == nil {
		return "unknown"
	}
	return function.Name()
}

func artifactMutationLockID(artifactID string) string {
	return "artifact-mutation:" + artifactID
}

func orderedArtifactMutationLockIDs(ctx context.Context) []string {
	set, _ := ctx.Value(artifactMutationLockContextKey{}).(artifactMutationLockSet)
	held := make([]string, 0, len(set))
	for id := range set {
		held = append(held, artifactMutationLockID(id))
	}
	sort.Strings(held)
	return held
}

func shipmentLifecycleLockOrderError(
	ctx context.Context,
	caller string,
	reason string,
) error {
	held := orderedArtifactMutationLockIDs(ctx)
	if token, ok := ctx.Value(shipmentLifecycleHeldLocksContextKey{}).(*shipmentLifecycleHeldLocks); ok {
		held = append([]string(nil), token.ordered...)
	}
	return &ShipmentLifecycleLockOrderError{
		RequestedLock: shipmentLifecycleGlobalLockID,
		CallerName:    caller,
		OrderedHeld:   held,
		Reason:        reason,
	}
}

func validateShipmentLifecycleGlobalReentry(
	ctx context.Context,
	ws *Workspace,
	caller string,
) (bool, error) {
	_, markedHeld := ctx.Value(shipmentLifecycleGlobalLockContextKey{}).(struct{})
	token, hasToken := ctx.Value(shipmentLifecycleHeldLocksContextKey{}).(*shipmentLifecycleHeldLocks)
	artifactLocks := orderedArtifactMutationLockIDs(ctx)

	if !markedHeld {
		if hasToken || len(artifactLocks) > 0 {
			return false, shipmentLifecycleLockOrderError(
				ctx,
				caller,
				"shipment lifecycle global lock is absent while artifact mutation locks are held",
			)
		}
		return false, nil
	}
	if !hasToken {
		if len(artifactLocks) > 0 {
			return false, shipmentLifecycleLockOrderError(
				ctx,
				caller,
				"global held-lock marker has no ordered token for the artifact locks",
			)
		}
		// Legacy direct global acquisitions in the lifecycle producers set the
		// marker before taking any artifact lock. The artifact-lock helper
		// upgrades this state to an ordered token before returning its context.
		return true, nil
	}
	if token.workspaceRoot != filepath.Clean(ws.RootPath) {
		return false, shipmentLifecycleLockOrderError(
			ctx,
			caller,
			"held-lock token belongs to a different workspace",
		)
	}
	if len(token.ordered) == 0 || token.ordered[0] != shipmentLifecycleGlobalLockID {
		return false, shipmentLifecycleLockOrderError(
			ctx,
			caller,
			"held-lock token does not begin with shipment lifecycle global",
		)
	}
	orderedSet := make(map[string]struct{}, len(token.ordered))
	for _, lockID := range token.ordered {
		orderedSet[lockID] = struct{}{}
	}
	for _, lockID := range artifactLocks {
		if _, exists := orderedSet[lockID]; !exists {
			return false, shipmentLifecycleLockOrderError(
				ctx,
				caller,
				"held-lock token omits an acquired artifact mutation lock",
			)
		}
	}
	return true, nil
}

func withShipmentLifecycleHeldLocks(
	ctx context.Context,
	ws *Workspace,
	lockIDs ...string,
) context.Context {
	existing, _ := ctx.Value(shipmentLifecycleHeldLocksContextKey{}).(*shipmentLifecycleHeldLocks)
	ordered := make([]string, 0, len(lockIDs)+1)
	if existing != nil {
		ordered = append(ordered, existing.ordered...)
	} else if _, globalHeld := ctx.Value(shipmentLifecycleGlobalLockContextKey{}).(struct{}); globalHeld {
		ordered = append(ordered, shipmentLifecycleGlobalLockID)
	}
	seen := make(map[string]struct{}, len(ordered)+len(lockIDs))
	for _, lockID := range ordered {
		seen[lockID] = struct{}{}
	}
	for _, lockID := range lockIDs {
		if lockID == "" {
			continue
		}
		if _, exists := seen[lockID]; exists {
			continue
		}
		ordered = append(ordered, lockID)
		seen[lockID] = struct{}{}
	}
	return context.WithValue(ctx, shipmentLifecycleHeldLocksContextKey{}, &shipmentLifecycleHeldLocks{
		workspaceRoot: filepath.Clean(ws.RootPath),
		ordered:       ordered,
	})
}

func lockShipmentLifecycleGlobal(
	ctx context.Context,
	ws *Workspace,
) (context.Context, func() error, error) {
	caller := shipmentLifecycleLockCaller()
	alreadyHeld, err := validateShipmentLifecycleGlobalReentry(ctx, ws, caller)
	if err != nil {
		return ctx, nil, err
	}
	lockedCtx, unlock, err := lockShipmentLifecycleGlobalRaw(ctx, ws, caller)
	if err != nil {
		return ctx, nil, err
	}
	if alreadyHeld {
		return lockedCtx, unlock, nil
	}
	if err := recoverPendingShipmentOperations(lockedCtx, ws); err != nil {
		if unlockErr := unlock(); unlockErr != nil {
			err = errors.Join(err, fmt.Errorf("release shipment lifecycle lock after recovery failure: %w", unlockErr))
		}
		return ctx, nil, fmt.Errorf("run pending shipment recovery barrier: %w", err)
	}
	return lockedCtx, unlock, nil
}

func lockShipmentLifecycleGlobalRaw(
	ctx context.Context,
	ws *Workspace,
	caller string,
) (context.Context, func() error, error) {
	held, err := validateShipmentLifecycleGlobalReentry(ctx, ws, caller)
	if err != nil {
		return ctx, nil, err
	}
	if held {
		if _, hasToken := ctx.Value(shipmentLifecycleHeldLocksContextKey{}).(*shipmentLifecycleHeldLocks); !hasToken {
			ctx = withShipmentLifecycleHeldLocks(ctx, ws, shipmentLifecycleGlobalLockID)
		}
		return ctx, func() error { return nil }, nil
	}
	hook, _ := ctx.Value(shipmentLifecycleGlobalLockHookContextKey{}).(func(string))
	if hook != nil {
		hook("attempt")
	}
	unlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
	if err != nil {
		return ctx, nil, err
	}
	if hook != nil {
		hook("acquired")
	}
	lockedCtx := context.WithValue(ctx, shipmentLifecycleGlobalLockContextKey{}, struct{}{})
	lockedCtx = withShipmentLifecycleHeldLocks(lockedCtx, ws, shipmentLifecycleGlobalLockID)
	return lockedCtx, unlock, nil
}

// BlockShipment performs the governed active-to-blocked shipment transition.
func BlockShipment(ctx context.Context, ws *Workspace, shipmentID string, opts BlockOptions) (*models.Artifact, error) {
	if strings.TrimSpace(opts.Reason) == "" {
		return nil, fmt.Errorf("block shipment %s requires a non-empty reason: %w", shipmentID, blerrors.ErrValidation)
	}
	opts.BlockedBy = normalizeBlockedEnvelopeOptionalString(opts.BlockedBy)
	opts.ResumeCheckpointRef = normalizeBlockedEnvelopeOptionalString(opts.ResumeCheckpointRef)

	globalUnlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment lifecycle: %w", err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockContextKey{}, struct{}{})
	if err := recoverPendingShipmentOperations(ctx, ws); err != nil {
		return nil, fmt.Errorf("recover pending shipment operations before block: %w", err)
	}

	membershipUnlock, err := lockShipmentMembership(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s membership: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := membershipUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment membership lock", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()

	shipment, err := findArtifact(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("load shipment %s: %w", shipmentID, err)
	}
	if shipment.ArtifactType != "shipment" {
		return nil, fmt.Errorf("block shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
	}
	if !isValidShipmentTransition(shipment.Status, ShipmentBlocked) {
		return nil, fmt.Errorf(
			"block shipment %s from %s: %w",
			shipmentID,
			shipment.Status,
			blerrors.ErrShipmentConflict,
		)
	}

	memberIDs := NormalizeShipmentItems(shipment)
	lockIDs := append([]string{shipmentID}, memberIDs...)
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, lockIDs)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s aggregate: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment aggregate locks", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()

	shipment, err = findArtifact(lockedCtx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("reload shipment %s under lock: %w", shipmentID, err)
	}
	if shipment.Status != models.StatusActive {
		return nil, fmt.Errorf(
			"block shipment %s from %s: %w",
			shipmentID,
			shipment.Status,
			blerrors.ErrShipmentConflict,
		)
	}
	if got := NormalizeShipmentItems(shipment); !slices.Equal(got, memberIDs) {
		return nil, fmt.Errorf("shipment %s membership changed while acquiring locks: %w", shipmentID, blerrors.ErrShipmentConflict)
	}
	branch, err := blockedEnvelopeOptionalString(shipment.CustomFields, "branch")
	if err != nil {
		return nil, fmt.Errorf("block shipment %s: %w", shipmentID, err)
	}

	preimage := shipmentLifecyclePreimage{
		Shipment: cloneArtifact(shipment),
		Members:  make([]*models.Artifact, 0, len(memberIDs)),
	}
	memberStatuses := make(map[string]string, len(memberIDs))
	for _, memberID := range memberIDs {
		member, loadErr := findArtifact(lockedCtx, ws, memberID)
		if loadErr != nil {
			return nil, fmt.Errorf("load shipment %s member %s: %w", shipmentID, memberID, loadErr)
		}
		preimage.Members = append(preimage.Members, cloneArtifact(member))
		memberStatuses[memberID] = string(member.Status)
	}

	var correlationBytes [16]byte
	if _, err := rand.Read(correlationBytes[:]); err != nil {
		return nil, fmt.Errorf("generate block shipment correlation id: %w", err)
	}
	correlationID := hex.EncodeToString(correlationBytes[:])
	journal := shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  correlationID,
		Phase:          "intent",
		Operation:      "block",
		RecoveryPolicy: "rollback",
		ShipmentID:     shipmentID,
		Target:         string(ShipmentBlocked),
		Reason:         opts.Reason,
		BlockedBy:      opts.BlockedBy,
		SnapshotRef:    opts.ResumeCheckpointRef,
		Preimage:       preimage,
	}
	journalName := shipmentLifecycleJournalName(correlationID)
	_, err = writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal)
	if err != nil {
		return nil, fmt.Errorf("persist block shipment %s intent: %w", shipmentID, err)
	}

	operationCtx := withShipmentOperation(lockedCtx, correlationID)
	eventDelta := map[string]any{
		"correlation_id":         correlationID,
		"operation":              "block",
		"target":                 string(ShipmentBlocked),
		"reason":                 opts.Reason,
		"member_status_snapshot": memberStatuses,
	}
	setBlockedEnvelopeOptionalString(eventDelta, "blocked_by", opts.BlockedBy)
	setBlockedEnvelopeOptionalString(eventDelta, "resume_checkpoint_ref", opts.ResumeCheckpointRef)
	blockedAt := time.Now().UTC().Format(time.RFC3339)
	eventDelta["blocked_at"] = blockedAt
	setBlockedEnvelopeOptionalString(eventDelta, "branch", branch)
	mutationApplied := false
	compensate := func(cause error) error {
		if blerrors.IsWriteIndeterminate(cause) {
			return fmt.Errorf("block shipment %s requires recovery after an indeterminate write: %w", shipmentID, cause)
		}
		compensateCtx := context.WithoutCancel(operationCtx)
		compensateCtx = context.WithValue(compensateCtx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
			correlationID:                 correlationID,
			operation:                     "block_compensation",
			allowGovernedShipmentMutation: true,
		})
		var compensationErr error
		if mutationApplied {
			if restoreErr := persistArtifact(compensateCtx, ws, cloneArtifact(preimage.Shipment), true); restoreErr != nil {
				compensationErr = errors.Join(compensationErr, fmt.Errorf("restore shipment %s: %w", shipmentID, restoreErr))
			}
			for _, member := range preimage.Members {
				if restoreErr := persistArtifact(compensateCtx, ws, cloneArtifact(member), true); restoreErr != nil {
					compensationErr = errors.Join(compensationErr, fmt.Errorf("restore member %s: %w", member.ID, restoreErr))
				}
			}
		}
		if compensationErr == nil {
			journal.Phase = "compensated"
			if _, journalErr := writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal); journalErr != nil {
				compensationErr = errors.Join(compensationErr, fmt.Errorf("persist compensation journal: %w", journalErr))
			}
			compensatedDelta := maps.Clone(eventDelta)
			compensatedDelta["phase"] = "compensated"
			compensatedDelta["target"] = string(preimage.Shipment.Status)
			if eventErr := appendItemEventWithActorErr(
				compensateCtx,
				ws,
				shipmentID,
				opts.BlockedBy,
				"shipment_lifecycle",
				compensatedDelta,
			); eventErr != nil {
				compensationErr = errors.Join(compensationErr, fmt.Errorf("append compensation event: %w", eventErr))
			}
		}
		if compensationErr != nil {
			return fmt.Errorf("block shipment %s: %w; compensation failed: %w", shipmentID, cause, compensationErr)
		}
		return fmt.Errorf("block shipment %s: %w", shipmentID, cause)
	}

	intentDelta := maps.Clone(eventDelta)
	intentDelta["phase"] = "intent"
	if err := appendItemEventWithActorErr(operationCtx, ws, shipmentID, opts.BlockedBy, "shipment_lifecycle", intentDelta); err != nil {
		return nil, compensate(fmt.Errorf("append lifecycle intent: %w", err))
	}

	for _, member := range preimage.Members {
		if member.Status != models.StatusActive && member.Status != models.StatusReview {
			continue
		}
		updated := cloneArtifact(member)
		updated.Status = models.StatusQueued
		updated.UpdatedAt = models.NowUTC()
		if err := persistArtifact(operationCtx, ws, updated, true); err != nil {
			return nil, compensate(fmt.Errorf("queue shipment member %s: %w", member.ID, err))
		}
		mutationApplied = true
	}

	blocked := cloneArtifact(shipment)
	blocked.Status = models.StatusBlocked
	blocked.UpdatedAt = models.NowUTC()
	if blocked.CustomFields == nil {
		blocked.CustomFields = map[string]any{}
	}
	blocked.CustomFields["blocked_reason"] = opts.Reason
	blocked.CustomFields["blocked_at"] = blockedAt
	blocked.CustomFields["member_status_snapshot"] = memberStatuses
	setBlockedEnvelopeOptionalString(blocked.CustomFields, "branch", branch)
	setBlockedEnvelopeOptionalString(blocked.CustomFields, "blocked_by", opts.BlockedBy)
	setBlockedEnvelopeOptionalString(blocked.CustomFields, "resume_checkpoint_ref", opts.ResumeCheckpointRef)
	governedCtx := context.WithValue(operationCtx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
		correlationID:                 correlationID,
		operation:                     "block_shipment",
		changes:                       eventDelta,
		allowGovernedShipmentMutation: true,
	})
	if err := persistArtifact(governedCtx, ws, blocked, true); err != nil {
		return nil, compensate(fmt.Errorf("persist blocked shipment: %w", err))
	}
	mutationApplied = true

	statusDelta := maps.Clone(eventDelta)
	statusDelta["phase"] = "applied"
	statusDelta["status"] = string(ShipmentBlocked)
	if err := appendItemEventWithActorErr(
		operationCtx,
		ws,
		shipmentID,
		opts.BlockedBy,
		"shipment_status_changed",
		statusDelta,
	); err != nil {
		return nil, compensate(fmt.Errorf("append blocked status event: %w", err))
	}

	commitDelta := maps.Clone(eventDelta)
	commitDelta["phase"] = "committed"
	if err := appendItemEventWithActorErr(operationCtx, ws, shipmentID, opts.BlockedBy, "shipment_lifecycle", commitDelta); err != nil {
		return nil, compensate(fmt.Errorf("append lifecycle commit: %w", err))
	}
	journal.Phase = "committed"
	if _, err := writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal); err != nil {
		return nil, compensate(fmt.Errorf("persist block shipment commit: %w", err))
	}

	return blocked, nil
}

// UnblockShipment performs a confirmed governed blocked-to-queued or
// blocked-to-active shipment transition.
func UnblockShipment(ctx context.Context, ws *Workspace, shipmentID string, opts UnblockOptions) (*models.Artifact, error) {
	if opts.Target != ShipmentQueued && opts.Target != ShipmentActive {
		return nil, fmt.Errorf("unblock shipment %s to unsupported target %s: %w", shipmentID, opts.Target, blerrors.ErrValidation)
	}
	if !opts.Confirm {
		return nil, fmt.Errorf("unblock shipment %s to %s requires confirmation: %w", shipmentID, opts.Target, blerrors.ErrConfirmationRequired)
	}

	globalUnlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment lifecycle: %w", err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockContextKey{}, struct{}{})
	if err := recoverPendingShipmentOperations(ctx, ws); err != nil {
		return nil, fmt.Errorf("recover pending shipment operations before unblock: %w", err)
	}

	membershipUnlock, err := lockShipmentMembership(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s membership: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := membershipUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment membership lock", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()

	shipment, err := findArtifact(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("load shipment %s: %w", shipmentID, err)
	}
	if shipment.ArtifactType != "shipment" {
		return nil, fmt.Errorf("unblock shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
	}
	if shipment.Status != models.StatusBlocked {
		return nil, fmt.Errorf("unblock shipment %s from %s: %w", shipmentID, shipment.Status, blerrors.ErrShipmentConflict)
	}
	if opts.Target == ShipmentActive {
		if err := ensureShipmentActiveSlotAvailable(ws, shipmentID); err != nil {
			return nil, fmt.Errorf("unblock shipment %s to active: %w", shipmentID, err)
		}
	}

	memberIDs := NormalizeShipmentItems(shipment)
	lockIDs := append([]string{shipmentID}, memberIDs...)
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, lockIDs)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s aggregate: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment aggregate locks", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()

	shipment, err = findArtifact(lockedCtx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("reload shipment %s under lock: %w", shipmentID, err)
	}
	if shipment.Status != models.StatusBlocked {
		return nil, fmt.Errorf("unblock shipment %s drifted to %s: %w", shipmentID, shipment.Status, blerrors.ErrShipmentConflict)
	}
	if got := NormalizeShipmentItems(shipment); !slices.Equal(got, memberIDs) {
		return nil, fmt.Errorf("shipment %s membership changed while acquiring locks: %w", shipmentID, blerrors.ErrShipmentConflict)
	}

	envelope, err := validatePersistedBlockedShipmentEnvelope(lockedCtx, ws, shipment)
	if err != nil {
		return nil, fmt.Errorf("unblock shipment %s: %w", shipmentID, err)
	}
	memberStatuses := envelope.memberStatuses

	preimage := shipmentLifecyclePreimage{
		Shipment: cloneArtifact(shipment),
		Members:  make([]*models.Artifact, 0, len(memberIDs)),
	}
	desiredStatuses := make(map[string]models.ArtifactStatus, len(memberIDs))
	for _, memberID := range memberIDs {
		snapshotStatus, found := memberStatuses[memberID]
		if !found {
			return nil, fmt.Errorf("unblock shipment %s snapshot is missing member %s: %w", shipmentID, memberID, blerrors.ErrShipmentConflict)
		}
		member, loadErr := findArtifact(lockedCtx, ws, memberID)
		if loadErr != nil {
			return nil, fmt.Errorf("load shipment %s member %s: %w", shipmentID, memberID, loadErr)
		}
		expectedBlockedStatus := models.ArtifactStatus(snapshotStatus)
		if expectedBlockedStatus == models.StatusActive || expectedBlockedStatus == models.StatusReview {
			expectedBlockedStatus = models.StatusQueued
		}
		if member.Status != expectedBlockedStatus {
			return nil, fmt.Errorf(
				"unblock shipment %s member %s drifted from expected blocked status %s to %s: %w",
				shipmentID,
				memberID,
				expectedBlockedStatus,
				member.Status,
				blerrors.ErrShipmentConflict,
			)
		}
		desiredStatus := models.StatusQueued
		if opts.Target == ShipmentActive {
			desiredStatus = models.ArtifactStatus(snapshotStatus)
		}
		candidate := cloneArtifact(member)
		candidate.Status = desiredStatus
		if validateErr := candidate.Validate(); validateErr != nil {
			return nil, fmt.Errorf("unblock shipment %s snapshot status for member %s: %w", shipmentID, memberID, blerrors.ErrShipmentConflict)
		}
		preimage.Members = append(preimage.Members, cloneArtifact(member))
		desiredStatuses[memberID] = desiredStatus
	}

	var correlationBytes [16]byte
	if _, err := rand.Read(correlationBytes[:]); err != nil {
		return nil, fmt.Errorf("generate unblock shipment correlation id: %w", err)
	}
	correlationID := hex.EncodeToString(correlationBytes[:])
	resumeCheckpointRef, _ := shipment.CustomFields["resume_checkpoint_ref"].(string)
	journal := shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  correlationID,
		Phase:          "intent",
		Operation:      "unblock",
		RecoveryPolicy: "rollback",
		ShipmentID:     shipmentID,
		Target:         string(opts.Target),
		BlockedBy:      opts.UnblockedBy,
		SnapshotRef:    resumeCheckpointRef,
		Preimage:       preimage,
	}
	journalName := shipmentLifecycleJournalName(correlationID)
	_, err = writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal)
	if err != nil {
		return nil, fmt.Errorf("persist unblock shipment %s intent: %w", shipmentID, err)
	}

	operationCtx := withShipmentOperation(lockedCtx, correlationID)
	eventDelta := map[string]any{
		"correlation_id":         correlationID,
		"operation":              "unblock",
		"target":                 string(opts.Target),
		"unblocked_by":           opts.UnblockedBy,
		"resume_checkpoint_ref":  resumeCheckpointRef,
		"member_status_snapshot": memberStatuses,
	}
	mutationApplied := false
	compensate := func(cause error) error {
		if blerrors.IsWriteIndeterminate(cause) {
			return fmt.Errorf("unblock shipment %s requires recovery after an indeterminate write: %w", shipmentID, cause)
		}
		compensateCtx := context.WithoutCancel(operationCtx)
		compensateCtx = context.WithValue(compensateCtx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
			correlationID:                 correlationID,
			operation:                     "unblock_compensation",
			allowGovernedShipmentMutation: true,
		})
		var compensationErr error
		if mutationApplied {
			for _, member := range preimage.Members {
				if restoreErr := persistArtifact(compensateCtx, ws, cloneArtifact(member), true); restoreErr != nil {
					compensationErr = errors.Join(compensationErr, fmt.Errorf("restore member %s: %w", member.ID, restoreErr))
				}
			}
			if restoreErr := persistArtifact(compensateCtx, ws, cloneArtifact(preimage.Shipment), true); restoreErr != nil {
				compensationErr = errors.Join(compensationErr, fmt.Errorf("restore shipment %s: %w", shipmentID, restoreErr))
			}
		}
		if compensationErr == nil {
			journal.Phase = "compensated"
			if _, journalErr := writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal); journalErr != nil {
				compensationErr = errors.Join(compensationErr, fmt.Errorf("persist compensation journal: %w", journalErr))
			}
			compensatedDelta := maps.Clone(eventDelta)
			compensatedDelta["phase"] = "compensated"
			compensatedDelta["target"] = string(preimage.Shipment.Status)
			if eventErr := appendItemEventWithActorErr(
				compensateCtx,
				ws,
				shipmentID,
				opts.UnblockedBy,
				"shipment_lifecycle",
				compensatedDelta,
			); eventErr != nil {
				compensationErr = errors.Join(compensationErr, fmt.Errorf("append compensation event: %w", eventErr))
			}
		}
		if compensationErr != nil {
			return fmt.Errorf("unblock shipment %s: %w; compensation failed: %w", shipmentID, cause, compensationErr)
		}
		return fmt.Errorf("unblock shipment %s: %w", shipmentID, cause)
	}

	intentDelta := maps.Clone(eventDelta)
	intentDelta["phase"] = "intent"
	if err := appendItemEventWithActorErr(operationCtx, ws, shipmentID, opts.UnblockedBy, "shipment_lifecycle", intentDelta); err != nil {
		return nil, compensate(fmt.Errorf("append lifecycle intent: %w", err))
	}

	unblocked := cloneArtifact(shipment)
	unblocked.Status = models.ArtifactStatus(opts.Target)
	unblocked.UpdatedAt = models.NowUTC()
	delete(unblocked.CustomFields, "blocked_reason")
	delete(unblocked.CustomFields, "blocked_at")
	delete(unblocked.CustomFields, "blocked_by")
	governedCtx := context.WithValue(operationCtx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
		correlationID:                 correlationID,
		operation:                     "unblock_shipment",
		changes:                       eventDelta,
		allowGovernedShipmentMutation: true,
	})
	if err := persistArtifact(governedCtx, ws, unblocked, true); err != nil {
		return nil, compensate(fmt.Errorf("persist unblocked shipment: %w", err))
	}
	mutationApplied = true

	for _, member := range preimage.Members {
		desiredStatus := desiredStatuses[member.ID]
		if member.Status == desiredStatus {
			continue
		}
		updated := cloneArtifact(member)
		updated.Status = desiredStatus
		updated.UpdatedAt = models.NowUTC()
		if err := persistArtifact(operationCtx, ws, updated, true); err != nil {
			return nil, compensate(fmt.Errorf("restore shipment member %s to %s: %w", member.ID, desiredStatus, err))
		}
	}

	statusDelta := maps.Clone(eventDelta)
	statusDelta["phase"] = "applied"
	statusDelta["status"] = string(opts.Target)
	if err := appendItemEventWithActorErr(
		operationCtx,
		ws,
		shipmentID,
		opts.UnblockedBy,
		"shipment_status_changed",
		statusDelta,
	); err != nil {
		return nil, compensate(fmt.Errorf("append unblocked status event: %w", err))
	}

	commitDelta := maps.Clone(eventDelta)
	commitDelta["phase"] = "committed"
	if err := appendItemEventWithActorErr(operationCtx, ws, shipmentID, opts.UnblockedBy, "shipment_lifecycle", commitDelta); err != nil {
		return nil, compensate(fmt.Errorf("append lifecycle commit: %w", err))
	}
	journal.Phase = "committed"
	if _, err := writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal); err != nil {
		return nil, compensate(fmt.Errorf("persist unblock shipment commit: %w", err))
	}

	return unblocked, nil
}

type fileSnapshot struct {
	Path    string
	Exists  bool
	Content []byte
}

type returnBlockedJournal struct {
	SchemaVersion  string           `json:"schema_version,omitempty"`
	CorrelationID  string           `json:"correlation_id,omitempty"`
	Phase          string           `json:"phase,omitempty"`
	ShipmentID     string           `json:"shipment_id,omitempty"`
	ItemID         string           `json:"item_id,omitempty"`
	Shipment       *models.Artifact `json:"shipment"`
	Item           *models.Artifact `json:"item"`
	TargetShipment *models.Artifact `json:"target_shipment,omitempty"`
	TargetItem     *models.Artifact `json:"target_item,omitempty"`
	Reason         string           `json:"reason,omitempty"`
}

const returnBlockedJournalSchemaVersion = "return-blocked/v2"

// CreateShipment creates a new shipment artifact in the workspace with the given
// title and associates the specified item IDs with it. The shipment owns the items
// list as an aggregate root.
//
// Optional Option values (e.g. WithPriority) are accepted via the variadic opts
// parameter and will be forwarded to CreateArtifact at the create path.
// Zero options yield the current behavior — backward-compatible.
//
// Worker: Create a new shipment Markdown artifact with YAML frontmatter containing
// the items list, set status to queued, generate ID with S prefix, write to queue
// directory, and upsert into the database index.
func CreateShipment(ctx context.Context, ws *Workspace, title string, itemIDs []string, opts ...Option) (*models.Artifact, error) {
	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("lock shipment lifecycle for create: %w", err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after create", "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	items := uniqueNonEmptyStrings(itemIDs)
	if err := validateShipmentItemIDs(ctx, ws, "", items); err != nil {
		return nil, fmt.Errorf("create shipment %q: %w", title, err)
	}

	// Apply caller opts first (e.g. WithPriority) so items is always set last
	// and cannot be overridden by a caller-supplied WithFields option. The
	// validated items list must be the authoritative state written to the artifact.
	createOpts := make([]Option, 0, len(opts)+1)
	createOpts = append(createOpts, opts...)
	createOpts = append(createOpts, WithFields(map[string]any{"items": items}))
	shipment, err := CreateArtifact(ctx, ws, title, "shipment", createOpts...)
	if err != nil {
		return nil, fmt.Errorf("create shipment %q: %w", title, err)
	}
	normalizeShipmentArtifact(shipment)
	if err := bldb.UpsertItem(ctx, ws.DB, shipment); err != nil {
		return nil, fmt.Errorf("create shipment %s: %w", shipment.ID, err)
	}

	appendItemEvent(ctx, ws, shipment.ID, "shipment_created", map[string]any{
		"title": title,
		"items": items,
	})

	return shipment, nil
}

// GetShipment retrieves a shipment artifact by ID from the workspace.
//
// Worker: Look up the shipment by ID in the database index, parse the Markdown
// source file, and return the populated Artifact with items list in CustomFields.
func GetShipment(ctx context.Context, ws *Workspace, shipmentID string) (*models.Artifact, error) {
	artifact, err := loadArtifact(ctx, ws, shipmentID)
	if err != nil {
		if errors.Is(err, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("get shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
		}
		return nil, fmt.Errorf("get shipment %s: %w", shipmentID, err)
	}
	if artifact.ArtifactType != "shipment" {
		return nil, fmt.Errorf("get shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
	}

	normalizeShipmentArtifact(artifact)
	return artifact, nil
}

// MoveShipmentStatus transitions a shipment's status. Valid transitions:
// queued->active, active->shipped, active->abandoned.
//
// Worker: Validate the transition is legal, update the Markdown frontmatter,
// rewrite the file atomically, and upsert the new status into the database.
// Emit slog.Info for status transition and events.jsonl record.
func MoveShipmentStatus(ctx context.Context, ws *Workspace, shipmentID string, newStatus ShipmentStatus) error {
	return moveShipmentStatusWithTopLevel(ctx, ws, shipmentID, newStatus, true)
}

// moveShipmentStatusWithTopLevel is the internal variant that accepts an explicit
// topLevel flag. Top-level callers use true; nested callers (e.g. ShipShipment)
// use false to suppress duplicate post-hook event emission. It carries no
// repository-ref CAS guard of its own — see moveShipmentStatusWithHeadGuard.
func moveShipmentStatusWithTopLevel(ctx context.Context, ws *Workspace, shipmentID string, newStatus ShipmentStatus, topLevel bool) error {
	return moveShipmentStatusWithHeadGuard(ctx, ws, shipmentID, newStatus, topLevel, "")
}

// moveShipmentStatusWithHeadGuard is moveShipmentStatusWithTopLevel's full
// variant, additionally accepting expectedHeadSHA: the repository HEAD that
// the caller's own gate evaluation already validated as stable (106.033-T).
//
// gateShipmentCompletion's pre/post headDriftError bracket only covers ITS
// OWN evaluation window (the shipment-level gate Evaluate call plus the
// member-evidence scan). Once that call returns, ShipShipment still performs
// a series of further, in-process operations before this function's own
// persist — completing the release scope, returning unreleased feature
// items, cascading feature statuses, and firing this function's own pre-hook
// — during which a concurrent commit could still advance the repository's
// real HEAD out from under the completing shipment. A signed manifest-binding
// proof would then attest to a reviewed commit while the shipment's own
// declared status transition completes against a HEAD that has already moved
// on.
//
// When expectedHeadSHA is non-empty, this function re-resolves HEAD ONE MORE
// TIME and passes the guard into the shared persistence path. The persistence
// path performs validation, link merging, path resolution, and file snapshots
// first, then invokes the guard immediately before its first artifact-data
// filesystem operation; lock infrastructure is intentionally prepared before
// the guard. This narrows, but does not eliminate, the residual
// window: git offers no atomic "read HEAD and complete our write" primitive,
// so a commit can still land after the final HEAD check and before the write
// completes. The guard remains shared across task, feature, and shipment
// persistence so all callers use the same pre-mutation ordering.
//
// An empty expectedHeadSHA (every existing call site other than ShipShipment's
// active->shipped transition, and that transition itself whenever
// gateShipmentCompletion's own bracket did not run — broker nil, gate not
// enforced, or genuine no-repo) leaves this guard inert, identical to
// moveShipmentStatusWithTopLevel's prior behavior.
func moveShipmentStatusWithHeadGuard(ctx context.Context, ws *Workspace, shipmentID string, newStatus ShipmentStatus, topLevel bool, expectedHeadSHA string) error {
	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return fmt.Errorf("lock shipment lifecycle for move %s: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after move", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	shipment, err := findArtifact(ctx, ws, shipmentID)
	if err != nil {
		return fmt.Errorf("load shipment %s: %w", shipmentID, err)
	}
	if shipment.ArtifactType != "shipment" {
		return fmt.Errorf("load shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
	}
	normalizeShipmentArtifact(shipment)
	oldShipmentStatus := shipment.Status

	if topLevel &&
		shipment.ArtifactType == "shipment" &&
		(newStatus == ShipmentBlocked || oldShipmentStatus == models.StatusBlocked) {
		return fmt.Errorf(
			"move shipment %s between blocked status via ungoverned path: %w",
			shipmentID,
			blerrors.ErrShipmentBlockedRequiresEnvelope,
		)
	}
	_, governedActivation := ctx.Value(governedShipmentActivationContextKey{}).(struct{})
	if shipment.ArtifactType == "shipment" &&
		newStatus == ShipmentActive &&
		!governedActivation {
		return fmt.Errorf(
			"move shipment %s to active via ungoverned path: %w",
			shipmentID,
			blerrors.ErrShipmentConflict,
		)
	}

	if !isValidShipmentTransition(shipment.Status, newStatus) {
		return fmt.Errorf(
			"move shipment %s from %s to %s: %w",
			shipmentID,
			shipment.Status,
			newStatus,
			blerrors.ErrShipmentConflict,
		)
	}

	// 144-F guard 1 (move seam): ungoverned shipped is unconditionally refused.
	// ShipShipment is the only governed caller and passes topLevel=false; the
	// exported MoveShipmentStatus passes topLevel=true and is blocked here so no
	// surface (MCP move_item, CLI backlogit move) can ship without the envelope.
	if topLevel && newStatus == ShipmentShipped {
		return fmt.Errorf("move shipment %s to shipped via ungoverned path: %w",
			shipmentID, blerrors.ErrShipmentShippedRequiresEnvelope)
	}

	// Fire pre-move-shipment-status hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       shipmentID,
			ArtifactType: "shipment",
			OldValues:    map[string]any{"status": string(oldShipmentStatus)},
			NewValues:    map[string]any{"status": string(newStatus)},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     topLevel,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookMoveShipmentStatus, hookCtx); err != nil {
			return fmt.Errorf("pre-move-shipment-status hook: %w", err)
		}
	}

	shipment.Status = models.ArtifactStatus(newStatus)
	shipment.UpdatedAt = models.NowUTC()
	ctx = context.WithValue(ctx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
		operation: "move_shipment_status",
		changes: map[string]any{
			"status": string(newStatus),
		},
		allowGovernedShipmentMutation: oldShipmentStatus != models.StatusBlocked &&
			models.ArtifactStatus(newStatus) != models.StatusBlocked,
	})
	var guard func(context.Context) error
	if expectedHeadSHA != "" {
		guard = func(guardCtx context.Context) error {
			return checkShipmentPersistHeadGuard(guardCtx, ws, shipmentID, expectedHeadSHA)
		}
	}
	if err := persistArtifactWithGuard(ctx, ws, shipment, true, guard); err != nil {
		return fmt.Errorf("move shipment %s: %w", shipmentID, err)
	}

	slog.InfoContext(ctx, "shipment status changed", "shipment_id", shipmentID, "new_status", newStatus)
	// 143.002-T / 143.003-T: the shipment's own status event flows through the
	// shipment-scoped, error-returning appender so the outcome is routable and
	// classifiable.
	//
	// The fail-closed branch is gated on the GOVERNED path only
	// (newStatus == ShipmentShipped && !topLevel). ShipShipment is the sole
	// caller passing topLevel=false for ShipmentShipped; the exported
	// MoveShipmentStatus passes true and stays best-effort. Gating on the status
	// alone would make the exported entry point fail closed WITHOUT the
	// classification and compensation that only ShipShipment's locked closure
	// provides, persisting the status and then returning a bare error — an
	// uncompensated residue on a path this change declares ungoverned.
	//
	// The append is never retried. events.EventWriter.AppendEvent's contract is
	// that a partial write or a post-write fsync failure is
	// blerrors.ErrWriteIndeterminate and is not safe to retry blindly; a
	// pre-append lock failure is already tagged not-applied by
	// appendShipmentEventErr and compensates. There is no third outcome class.
	//
	// The fail-closed shipped path intentionally suppresses the
	// HookMoveShipmentStatus POST hook, which today always fires because the
	// append error is swallowed. Firing a "status changed to shipped" post-hook
	// for a transition that is about to be compensated (or explicitly reported
	// as indeterminate) would misinform external integrations.
	//
	// On a platform without item-log file-lock support
	// (internal/events/item_log_lock_other.go always errors), this branch turns
	// every governed ship into a hard, clearly-classified refusal rather than a
	// silent audit gap. Windows and Unix are the supported build targets.
	appendErr := ws.appendShipmentEvent(ctx, shipmentID, "shipment_status_changed", map[string]any{
		"status": string(newStatus),
	})
	if appendErr != nil {
		if newStatus == ShipmentShipped && !topLevel {
			return &shipmentEventAppendError{shipmentID: shipmentID, cause: appendErr}
		}
		slog.WarnContext(ctx, "append shipment status event", "shipment_id", shipmentID, "new_status", newStatus, "error", appendErr)
	}

	// Fire post-move-shipment-status hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       shipmentID,
			ArtifactType: "shipment",
			OldValues:    map[string]any{"status": string(oldShipmentStatus)},
			NewValues:    map[string]any{"status": string(newStatus)},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     topLevel,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookMoveShipmentStatus, hookCtx)
	}

	return nil
}

// shipmentEventAppendError is the private boundary value carrying a governed
// shipped-event append failure out of moveShipmentStatusWithHeadGuard so
// ShipShipment's rollback defer classifies ONLY that error. Every other closure
// error -- including untagged pre-append failures from completeReleaseScope and
// the status cascades -- keeps the existing unconditional rollback.
//
// It is declared here, in the unit that constructs it, rather than a unit
// earlier: an unexported type with no constructor is reported by staticcheck's
// unused analyzer.
type shipmentEventAppendError struct {
	shipmentID string
	cause      error
}

func (e *shipmentEventAppendError) Error() string {
	return fmt.Sprintf("append shipped event for shipment %s: %v", e.shipmentID, e.cause)
}

func (e *shipmentEventAppendError) Unwrap() error { return e.cause }

func checkShipmentPersistHeadGuard(ctx context.Context, ws *Workspace, shipmentID, expectedHeadSHA string) error {
	currentHead, headErr := ws.headSHABounded(ctx)
	if headErr != nil {
		if auditErr := ws.appendGateEvent(ctx, shipmentID, EventGateBlocked, map[string]any{
			"level":         "shipment",
			"outcome":       "blocked",
			"reason":        "head-resolve-error-before-persist",
			"expected_head": expectedHeadSHA,
		}); auditErr != nil {
			slog.WarnContext(ctx, "shipment head guard: failed to append blocked evidence", "shipment_id", shipmentID, "error", auditErr)
		}
		return headResolveError(shipmentID, headErr)
	}
	if driftErr := headDriftError(shipmentID, expectedHeadSHA, currentHead); driftErr != nil {
		if auditErr := ws.appendGateEvent(ctx, shipmentID, EventGateBlocked, map[string]any{
			"level":         "shipment",
			"outcome":       "blocked",
			"reason":        "head-drift-before-persist",
			"expected_head": expectedHeadSHA,
			"observed_head": currentHead,
		}); auditErr != nil {
			slog.WarnContext(ctx, "shipment head guard: failed to append blocked evidence", "shipment_id", shipmentID, "error", auditErr)
		}
		return driftErr
	}
	return nil
}

// Shipment membership lock design notes:
// shipment.CustomFields["items"] — reusing task_lock.go's per-file-path
// keyed-mutex-plus-sidecar mechanism (despite its "task" naming, it is
// generic: keyed by resolved file path, not by artifact type). Every
// function that reads-then-writes shipment membership, or that signs a
// manifest-binding proof over a membership snapshot, MUST hold this lock for
// the duration of that critical section. Without it, a concurrent membership
// mutation can land in the unprotected window between a snapshot and a later
// re-read/signing step, letting an added member ride inside a signed
// manifest-binding proof whose gate evidence was never actually validated
// (106-F F1 review finding).
//
// The lock key is a STABLE synthetic path derived from the workspace root and
// shipment ID alone — NOT the shipment's actual current markdown file path.
// persistArtifact relocates a shipment's file whenever its target directory
// changes (e.g. archival, or a registry configured to route active/shipped
// shipments to different directories), and moveShipmentStatusWithTopLevel
// performs exactly such a persist while ShipShipment still holds this lock.
// Locking on the real file path would be unsafe: a lock acquired against the
// pre-relocation path and one acquired (by a later caller, once the file has
// already moved) against the post-relocation path are DIFFERENT mutexes and
// DIFFERENT sidecar files that never contend with each other, silently
// reopening the membership race across any relocation regardless of how far
// the caller's own critical section has been extended (106-F F1 review
// finding, third pass). A key derived only from the workspace root and
// shipment ID never changes, so it keeps contending correctly no matter
// where the underlying file currently lives.
type artifactMutationLockContextKey struct{}

type artifactMutationLockSet map[string]struct{}

const artifactMutationLocksDirName = "artifacts"

func withArtifactMutationLocks(ctx context.Context, ids []string) context.Context {
	set := make(artifactMutationLockSet, len(ids))
	if existing, ok := ctx.Value(artifactMutationLockContextKey{}).(artifactMutationLockSet); ok {
		for id := range existing {
			set[id] = struct{}{}
		}
	}
	for _, id := range ids {
		if id != "" {
			set[id] = struct{}{}
		}
	}
	return context.WithValue(ctx, artifactMutationLockContextKey{}, set)
}

func artifactMutationLockHeld(ctx context.Context, id string) bool {
	set, ok := ctx.Value(artifactMutationLockContextKey{}).(artifactMutationLockSet)
	if !ok {
		return false
	}
	_, ok = set[id]
	return ok
}

func artifactMutationLockPath(ws *Workspace, artifactID string) (string, error) {
	storageRoot, err := filepath.Abs(WorkspaceStorageRoot(ws.RootPath))
	if err != nil {
		return "", fmt.Errorf("resolve artifact mutation storage root: %w", err)
	}
	storageRoot = filepath.Clean(storageRoot)
	realStorageRoot, err := filepath.EvalSymlinks(storageRoot)
	if err != nil {
		return "", fmt.Errorf("resolve artifact mutation storage root symlinks: %w", err)
	}
	locksDir := filepath.Join(realStorageRoot, shipmentMembershipLocksDirName)
	if existing, evalErr := filepath.EvalSymlinks(locksDir); evalErr == nil {
		if !pathContained(realStorageRoot, existing) {
			return "", fmt.Errorf("artifact mutation locks parent resolves outside workspace: %w", blerrors.ErrValidation)
		}
		locksDir = existing
	} else if !os.IsNotExist(evalErr) {
		return "", fmt.Errorf("resolve artifact mutation locks parent: %w", evalErr)
	}
	if !pathContained(realStorageRoot, locksDir) {
		return "", fmt.Errorf("artifact mutation locks parent is outside workspace: %w", blerrors.ErrValidation)
	}
	if err := os.MkdirAll(locksDir, 0o755); err != nil {
		return "", fmt.Errorf("create artifact mutation locks parent: %w", err)
	}
	realLocksDir, err := filepath.EvalSymlinks(locksDir)
	if err != nil {
		return "", fmt.Errorf("resolve artifact mutation locks parent containment: %w", err)
	}
	if !pathContained(realStorageRoot, realLocksDir) {
		return "", fmt.Errorf("artifact mutation locks parent resolves outside workspace: %w", blerrors.ErrValidation)
	}
	artifactLocksDir := filepath.Join(realLocksDir, artifactMutationLocksDirName)
	if existing, evalErr := filepath.EvalSymlinks(artifactLocksDir); evalErr == nil {
		if !pathContained(realLocksDir, existing) {
			return "", fmt.Errorf("artifact mutation locks directory resolves outside parent: %w", blerrors.ErrValidation)
		}
		artifactLocksDir = existing
	} else if !os.IsNotExist(evalErr) {
		return "", fmt.Errorf("resolve artifact mutation locks directory: %w", evalErr)
	}
	if !pathContained(realLocksDir, artifactLocksDir) {
		return "", fmt.Errorf("artifact mutation locks directory is outside parent: %w", blerrors.ErrValidation)
	}
	if err := os.MkdirAll(artifactLocksDir, 0o755); err != nil {
		return "", fmt.Errorf("create artifact mutation locks directory: %w", err)
	}
	realArtifactLocksDir, err := filepath.EvalSymlinks(artifactLocksDir)
	if err != nil {
		return "", fmt.Errorf("resolve artifact mutation locks directory containment: %w", err)
	}
	// Encode the caller-controlled ID so the synthetic lock key cannot escape
	// the contained directory even if a future caller passes an unvalidated ID.
	stableKey := filepath.Join(realArtifactLocksDir, hex.EncodeToString([]byte(artifactID)))
	if !pathContained(realArtifactLocksDir, stableKey) {
		return "", fmt.Errorf("artifact ID %q resolves outside the artifact mutation locks directory: %w", artifactID, blerrors.ErrValidation)
	}
	return stableKey, nil
}

func lockArtifactMutation(ctx context.Context, ws *Workspace, artifactID string) (func() error, error) {
	if artifactMutationLockHeld(ctx, artifactID) {
		return func() error { return nil }, nil
	}
	stableKey, err := artifactMutationLockPath(ws, artifactID)
	if err != nil {
		return nil, err
	}
	return lockTaskFile(stableKey)
}

func lockArtifactMutations(ctx context.Context, ws *Workspace, ids []string) (context.Context, func() error, error) {
	// Aggregate mutation locks share the lifecycle-global domain. Acquiring the
	// barrier here keeps less-obvious callers (archive reconciliation, adoption,
	// and unarchive) on the same global -> artifact order as shipment writers.
	lockedLifecycleCtx, globalUnlock, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return ctx, nil, fmt.Errorf("lock shipment lifecycle before artifact mutations: %w", err)
	}
	ctx = lockedLifecycleCtx

	releaseAfterFailure := func(lockErr error, unlocks []func() error) error {
		errs := []error{lockErr}
		for i := len(unlocks) - 1; i >= 0; i-- {
			errs = append(errs, unlocks[i]())
		}
		errs = append(errs, globalUnlock())
		return errors.Join(errs...)
	}

	uniqueIDs := uniqueNonEmptyStrings(ids)
	sort.Strings(uniqueIDs)
	unlocks := make([]func() error, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		if artifactMutationLockHeld(ctx, id) {
			continue
		}
		stableKey, err := artifactMutationLockPath(ws, id)
		if err != nil {
			return ctx, nil, releaseAfterFailure(err, unlocks)
		}
		unlock, err := lockTaskFileWithHeartbeat(ctx, stableKey, defaultGateLockBoundedWait, defaultGateLockHeartbeat)
		if err != nil {
			return ctx, nil, releaseAfterFailure(fmt.Errorf("lock artifact %s: %w", id, err), unlocks)
		}
		unlocks = append(unlocks, unlock)
	}
	lockedCtx := withArtifactMutationLocks(ctx, uniqueIDs)
	orderedLockIDs := make([]string, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		orderedLockIDs = append(orderedLockIDs, artifactMutationLockID(id))
	}
	lockedCtx = withShipmentLifecycleHeldLocks(lockedCtx, ws, orderedLockIDs...)
	return lockedCtx, func() error {
		var errs []error
		for i := len(unlocks) - 1; i >= 0; i-- {
			if err := unlocks[i](); err != nil {
				errs = append(errs, err)
			}
		}
		errs = append(errs, globalUnlock())
		return errors.Join(errs...)
	}, nil
}

// lockShipmentMembership acquires the stable per-shipment advisory lock used
// to serialize membership reads and writes across relocation.
func lockShipmentMembership(ctx context.Context, ws *Workspace, shipmentID string) (unlock func() error, err error) {
	locksDir := filepath.Join(WorkspaceStorageRoot(ws.RootPath), shipmentMembershipLocksDirName)
	if mkErr := os.MkdirAll(locksDir, 0o755); mkErr != nil {
		return nil, fmt.Errorf("create shipment membership locks directory: %w", mkErr)
	}
	// A lexical/pathContained check on locksDir alone is insufficient:
	// .backlogit/.locks (or an ancestor) could itself be a symlink to a
	// directory outside the workspace (planted by a prior compromise or a
	// misconfigured setup), which a string-only check cannot see. Resolve it
	// through any symlinks and verify the REAL path still resolves within
	// the (also symlink-resolved) workspace storage root BEFORE any lock
	// operation runs, reusing the same containment pattern
	// resolveContainedArtifactPath already establishes for artifact leaves —
	// otherwise every subsequent lock operation (create, touch,
	// stale-reclaim-and-delete) would actually operate through that external
	// symlink target (106-F F1 review finding, round 6). All further
	// operations are bound to the resolved path, not the original locksDir.
	realLocksDir, containErr := resolveContainedArtifactPath(ws, locksDir)
	if containErr != nil {
		return nil, fmt.Errorf("resolve shipment membership locks directory containment: %w", containErr)
	}
	// stableKey need not (and must not) point at a real artifact file —
	// lockTaskFile only ever creates a ".<name>.lock" sidecar adjacent to
	// the path it is given; it never requires the path itself to exist.
	stableKey := filepath.Join(realLocksDir, shipmentID)
	// shipmentID is caller-controlled (CLI/MCP argument) and reaches this
	// function BEFORE any upstream GetShipment/artifact-ID validation runs.
	// A value containing path-traversal segments (e.g. "../../escape") could
	// otherwise resolve stableKey outside locksDir entirely, letting this
	// code create, touch, or stale-reclaim-and-delete a lock artifact outside
	// the workspace — a path traversal / workspace escape (Constitution
	// Principle III), not merely a lock-key naming concern. Fail closed
	// rather than trusting filepath.Join's result unchecked (106-F F1 review
	// finding, round 5).
	if !pathContained(realLocksDir, stableKey) {
		return nil, fmt.Errorf("shipment ID %q resolves outside the shipment membership locks directory: %w", shipmentID, blerrors.ErrValidation)
	}
	return lockTaskFileWithHeartbeat(ctx, stableKey, defaultGateLockBoundedWait, defaultGateLockHeartbeat)
}

// shipmentMembershipLocksDirName is the fixed, dotfile-prefixed subdirectory
// (under the workspace's backlogit storage root) holding ONLY the synthetic
// lock-key placeholders lockShipmentMembership uses — never real artifact
// content. The leading dot keeps it alongside the existing `.*.lock`
// .gitignore convention for advisory lock sidecars.
const shipmentMembershipLocksDirName = ".locks"

// AddItemToShipment associates an artifact ID with a shipment. The item must not
// already belong to another active shipment.
//
// Worker: Check the item is not already assigned to another shipment (return
// ErrItemAlreadyAssigned if so), append the item ID to the shipment's items list
// in frontmatter, rewrite the file atomically, and upsert into the database.
// Emit slog.Debug for item association and events.jsonl record.
func AddItemToShipment(ctx context.Context, ws *Workspace, shipmentID, itemID string) error {
	lockedCtx, globalUnlock, lockErr := lockShipmentLifecycleGlobal(ctx, ws)
	if lockErr != nil {
		return fmt.Errorf("add item %s to shipment %s: lock shipment lifecycle: %w", itemID, shipmentID, lockErr)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after add", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	if err := recoverPendingShipmentOperations(ctx, ws); err != nil {
		return fmt.Errorf("recover pending shipment operations before adding item %s to shipment %s: %w",
			itemID, shipmentID, err)
	}

	unlock, lockErr := lockShipmentMembership(ctx, ws, shipmentID)
	if lockErr != nil {
		return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, lockErr)
	}
	defer func() { _ = unlock() }()

	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return err
	}
	if shipmentMutationBlocked(shipment.Status) {
		return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, blerrors.ErrShipmentConflict)
	}

	items := NormalizeShipmentItems(shipment)
	if containsString(items, itemID) {
		return nil
	}
	if err := validateShipmentItemIDs(ctx, ws, shipmentID, []string{itemID}); err != nil {
		return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, err)
	}

	items = append(items, itemID)
	originalShipment := cloneArtifact(shipment)
	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	shipment.CustomFields["items"] = items
	shipment.UpdatedAt = models.NowUTC()
	if err := MutationEnvelope(ctx, []MutationStep{
		{
			Name: "frontmatter-update",
			Apply: func(ctx context.Context) error {
				if err := persistArtifact(ctx, ws, shipment, false); err != nil {
					return fmt.Errorf("persist shipment membership %s->%s: %w", shipmentID, itemID, err)
				}
				return nil
			},
			Compensate: func(ctx context.Context) error {
				if err := persistArtifact(ctx, ws, cloneArtifact(originalShipment), false); err != nil {
					return fmt.Errorf("restore shipment membership %s->%s: %w", shipmentID, itemID, err)
				}
				return nil
			},
		},
		{
			// Append the audit event using a direct EventWriter so JSONL
			// failures propagate to the envelope instead of being swallowed
			// by the fire-and-forget appendItemEvent wrapper.
			// Compensate is a documented no-op: audit trails are never
			// rewritten.
			Name: "jsonl-append",
			Apply: func(ctx context.Context) error {
				logsDir := WorkspaceLogsRoot(ws.RootPath)
				ew := NewWorkspaceEventWriter(ws, logsDir)
				ev := events.Event{
					Actor:     "backlogit",
					ItemID:    shipmentID,
					EventType: "shipment_item_added",
					Delta:     map[string]any{"item_id": itemID},
				}
				if appendErr := ew.AppendEvent(ctx, ev); appendErr != nil {
					return fmt.Errorf("shipment item added JSONL append: %w", appendErr)
				}
				return nil
			},
			Compensate: func(context.Context) error {
				return nil
			},
		},
	}); err != nil {
		return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, err)
	}

	slog.DebugContext(ctx, "shipment item added", "shipment_id", shipmentID, "item_id", itemID)

	return nil
}

// ReturnBlockedItem removes an item from shipment and sets its status to blocked
// with a reason. This is an explicit operation, not a side effect.
//
// Worker: Remove the item ID from the shipment's items list, set the item's status
// to blocked, append a blocked_reason field to the item's frontmatter, rewrite
// both files atomically, and emit slog.Info + events.jsonl for the return.
// Return ErrCannotReturnItem if the item is not in this shipment.
func ReturnBlockedItem(ctx context.Context, ws *Workspace, shipmentID, itemID, reason string) error {
	lockedCtx, globalUnlock, lockErr := lockShipmentLifecycleGlobal(ctx, ws)
	if lockErr != nil {
		return fmt.Errorf("return item %s from shipment %s: lock shipment lifecycle: %w", itemID, shipmentID, lockErr)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after return", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	if err := recoverPendingShipmentOperations(ctx, ws); err != nil {
		return fmt.Errorf("recover pending shipment operations before returning item %s from shipment %s: %w",
			itemID, shipmentID, err)
	}

	unlock, lockErr := lockShipmentMembership(ctx, ws, shipmentID)
	if lockErr != nil {
		return fmt.Errorf("return item %s from shipment %s: %w", itemID, shipmentID, lockErr)
	}
	defer func() { _ = unlock() }()

	shipment, err := findArtifact(ctx, ws, shipmentID)
	if err != nil {
		return fmt.Errorf("load shipment %s: %w", shipmentID, err)
	}
	if shipment.ArtifactType != "shipment" {
		return fmt.Errorf("load shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
	}
	normalizeShipmentArtifact(shipment)
	if shipmentMutationBlocked(shipment.Status) {
		return fmt.Errorf("return item %s from shipment %s: %w", itemID, shipmentID, blerrors.ErrShipmentConflict)
	}

	items := NormalizeShipmentItems(shipment)
	if !containsString(items, itemID) {
		return fmt.Errorf("return item %s from shipment %s: %w", itemID, shipmentID, blerrors.ErrCannotReturnItem)
	}

	item, err := findArtifact(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("load item %s: %w", itemID, err)
	}
	originalShipment := cloneArtifact(shipment)
	originalItem := cloneArtifact(item)

	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	shipment.CustomFields["items"] = removeString(items, itemID)
	shipment.UpdatedAt = models.NowUTC()

	if item.CustomFields == nil {
		item.CustomFields = map[string]any{}
	}
	item.Status = models.StatusBlocked
	item.CustomFields["blocked_reason"] = reason
	item.UpdatedAt = models.NowUTC()

	journal, err := newReturnBlockedJournal(originalShipment, originalItem, shipment, item, reason)
	if err != nil {
		return fmt.Errorf("prepare return of item %s from shipment %s: %w", itemID, shipmentID, err)
	}
	if err := writeReturnBlockedJournalRecord(ws, journal); err != nil {
		return fmt.Errorf("journal return of item %s from shipment %s: %w", itemID, shipmentID, err)
	}

	rolledBack, err := persistReturnedBlockedArtifacts(ctx, ws, originalShipment, shipment, originalItem, item)
	if err != nil {
		if rolledBack {
			removeReturnBlockedJournal(ctx, ws, originalShipment.ID, originalItem.ID)
		}
		return err
	}
	if err := appendReturnBlockedEvidence(ctx, ws, journal); err != nil {
		rollbackErr := rollbackReturnedBlockedArtifacts(ctx, ws, originalShipment, originalItem)
		eventCleanupErr := removeShipmentOperationEvents(
			context.WithoutCancel(ctx),
			ws,
			[]string{shipmentID, itemID},
			journal.CorrelationID,
		)
		if rollbackErr != nil || eventCleanupErr != nil {
			return &blerrors.MutationPartialError{
				Completed:         []string{"shipment-membership", "blocked-item"},
				FailedStep:        "return-blocked-evidence",
				CompensationState: "partially-compensated",
				Class:             "double-fault",
				Cause:             errors.Join(err, rollbackErr, eventCleanupErr),
			}
		}
		removeReturnBlockedJournal(ctx, ws, originalShipment.ID, originalItem.ID)
		return fmt.Errorf("append return-blocked evidence for item %s from shipment %s: %w",
			itemID, shipmentID, err)
	}
	journal.Phase = "committed"
	if err := writeReturnBlockedJournalRecord(ws, journal); err != nil {
		return fmt.Errorf("persist committed return of item %s from shipment %s: %w",
			itemID, shipmentID, err)
	}
	removeReturnBlockedJournal(ctx, ws, originalShipment.ID, originalItem.ID)

	slog.InfoContext(ctx, "shipment item returned blocked", "shipment_id", shipmentID, "item_id", itemID)

	return nil
}

type shipmentOperationContextKey struct{}

const shipmentOperationDeltaKey = "_shipment_operation"

func withShipmentOperation(ctx context.Context, operationID string) context.Context {
	return context.WithValue(ctx, shipmentOperationContextKey{}, operationID)
}

func shipmentOperationID(ctx context.Context) string {
	operationID, _ := ctx.Value(shipmentOperationContextKey{}).(string)
	return operationID
}

func isShipmentOperationEvent(event events.Event, operationID string) bool {
	if operationID == "" || event.Delta == nil {
		return false
	}
	value, _ := event.Delta[shipmentOperationDeltaKey].(string)
	return value == operationID
}

func eventDeltaWithShipmentOperation(ctx context.Context, delta map[string]any) map[string]any {
	operationID := shipmentOperationID(ctx)
	if operationID == "" {
		return delta
	}
	tagged := make(map[string]any, len(delta)+1)
	for key, value := range delta {
		tagged[key] = value
	}
	tagged[shipmentOperationDeltaKey] = operationID
	return tagged
}

func appendItemEvent(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) {
	appendItemEventWithCommit(ctx, ws, itemID, eventType, delta, "")
}

// clearStaleBlockedReason removes blocked_reason metadata once an artifact
// leaves the blocked status. The reason is only meaningful while the item is
// blocked; retaining it after the item re-enters the backlog (queued) or is
// re-activated misrepresents the item's current availability.
func clearStaleBlockedReason(artifact *models.Artifact, previousStatus models.ArtifactStatus) {
	if artifact == nil || artifact.CustomFields == nil {
		return
	}
	if previousStatus == models.StatusBlocked && artifact.Status != models.StatusBlocked {
		delete(artifact.CustomFields, "blocked_reason")
	}
}

func appendItemEventWithCommit(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any, commitSHA string) {
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: eventType,
		Delta:     eventDeltaWithShipmentOperation(ctx, delta),
		CommitSHA: commitSHA,
	}

	lockedCtx, unlock, lockErr := events.LockItemLogCrossProcess(ctx, WorkspaceLocksRoot(ws.RootPath), logsDir, itemID)
	if lockErr != nil {
		slog.WarnContext(ctx, "lock shipment event log", "item_id", itemID, "event_type", eventType, "error", lockErr)
		return
	}
	defer unlock()
	writer := NewWorkspaceEventWriter(ws, logsDir)
	if err := writer.AppendEvent(lockedCtx, event); err != nil {
		slog.WarnContext(ctx, "append shipment event", "item_id", itemID, "event_type", eventType, "error", err)
		return
	}
	if err := bldb.IndexEvent(lockedCtx, ws.DB, logsDir, event); err != nil {
		slog.WarnContext(ctx, "index shipment event", "item_id", itemID, "event_type", eventType, "error", err)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isValidShipmentTransition(current models.ArtifactStatus, next ShipmentStatus) bool {
	switch current {
	case models.StatusQueued:
		return next == ShipmentActive
	case models.StatusActive:
		return next == ShipmentBlocked || next == ShipmentShipped || next == ShipmentAbandoned
	case models.StatusBlocked:
		return next == ShipmentQueued || next == ShipmentActive
	default:
		return false
	}
}

func ensureShipmentActiveSlotAvailable(ws *Workspace, shipmentID string) error {
	searchDirs, err := artifactSearchDirs(ws)
	if err != nil {
		return fmt.Errorf("resolve canonical shipment search directories: %w", err)
	}
	for _, dirPath := range searchDirs {
		if _, statErr := os.Lstat(dirPath); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			return fmt.Errorf("inspect canonical shipment directory %s: %w", dirPath, statErr)
		}
		if err := filepath.WalkDir(dirPath, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".md" {
				return walkErr
			}
			if strings.EqualFold(filepath.Base(path), ".stash.md") {
				return nil
			}
			if err := ensureArtifactLookupContained(ws, path); err != nil {
				return err
			}
			artifact, _, err := parseFile(path)
			if err != nil {
				return fmt.Errorf("parse canonical active-slot candidate %s: %w", path, err)
			}
			if artifact.ArtifactType == "shipment" &&
				artifact.ID != shipmentID &&
				artifact.Status == models.StatusActive {
				return fmt.Errorf("shipment %s already owns the active slot: %w", artifact.ID, blerrors.ErrShipmentConflict)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("scan canonical shipment directory %s: %w", dirPath, err)
		}
	}
	return nil
}

func loadArtifact(ctx context.Context, ws *Workspace, id string) (*models.Artifact, error) {
	artifact, err := bldb.GetItem(ctx, ws.DB, id)
	if err == nil {
		return artifact, nil
	}
	if !errors.Is(err, blerrors.ErrNotFound) {
		return nil, err
	}

	artifact, findErr := findArtifact(ctx, ws, id)
	if findErr != nil {
		if errors.Is(findErr, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("load artifact %s: %w", id, blerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("load artifact %s: %w", id, findErr)
	}
	if upsertErr := bldb.UpsertItem(ctx, ws.DB, artifact); upsertErr != nil {
		return nil, fmt.Errorf("upsert artifact %s: %w", id, upsertErr)
	}
	return artifact, nil
}

func normalizeShipmentArtifact(artifact *models.Artifact) {
	if artifact.CustomFields == nil {
		artifact.CustomFields = map[string]any{}
	}
	artifact.CustomFields["items"] = NormalizeShipmentItems(artifact)
}

// persistArtifactWriteFn is the raw artifact-write seam used only beneath the
// governed writer. Tests that exercise the relocate=false path
// (AddDependency/RemoveDependency) override this seam to inject durable-write
// failures, because neither mkdirDirSyncFn nor mkdirAllDurable fires on that path.
//
// Must not run with t.Parallel: tests that swap this seam read on the production
// write path.
var persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
	fm := artifact.ToFrontmatterMap()
	content := models.SerializeFrontmatter(fm, artifact.Description)
	if err := atomicfile.WriteFileAtomicWithOptions(filePath, []byte(content), atomicfile.Options{DurableWrites: durable}); err != nil {
		return fmt.Errorf("write artifact file: %w", err)
	}
	return nil
}

// persistArtifactPreLockHook, when non-nil, is invoked by
// persistArtifactWithLinkPolicyAndGuard immediately BEFORE it attempts to
// acquire the artifact-mutation lock (B), with the artifact ID being
// persisted. It exists solely so tests can deterministically synchronize a
// "concurrent writer already committed a different archived_status" scenario
// (167.019-T) without a timing-based sleep race: the hook runs synchronously,
// in the SAME goroutine, so a test can perform its own concurrent mutation
// (even via a recursive persistArtifact call, which is safe here because lock
// B is not yet held at this point) and have it land on disk before this call
// proceeds to acquire lock B and run its guard. Nil in production (no-op).
//
// Must not run with t.Parallel: tests that set this read/write a shared
// package var.
var persistArtifactPreLockHook func(artifactID string)

func persistArtifact(ctx context.Context, ws *Workspace, artifact *models.Artifact, relocate bool) error {
	return persistArtifactWithLinkPolicyAndGuard(ctx, ws, artifact, relocate, true, nil)
}

func persistArtifactWithoutDBOnlyLinks(ctx context.Context, ws *Workspace, artifact *models.Artifact, relocate bool) error {
	return persistArtifactWithLinkPolicyAndGuard(ctx, ws, artifact, relocate, false, nil)
}

func persistArtifactWithGuard(ctx context.Context, ws *Workspace, artifact *models.Artifact, relocate bool, guard func(context.Context) error) error {
	return persistArtifactWithLinkPolicyAndGuard(ctx, ws, artifact, relocate, true, guard)
}

func persistArtifactWithLinkPolicyAndGuard(ctx context.Context, ws *Workspace, artifact *models.Artifact, relocate, preserveDBOnlyLinks bool, guard func(context.Context) error) error {
	if persistArtifactPreLockHook != nil {
		persistArtifactPreLockHook(artifact.ID)
	}
	if artifact != nil && artifact.ArtifactType == "shipment" {
		lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobalRaw(
			ctx,
			ws,
			"persistArtifactWithLinkPolicyAndGuard",
		)
		if err != nil {
			return fmt.Errorf("lock shipment lifecycle for artifact %s: %w", artifact.ID, err)
		}
		defer func() {
			if unlockErr := globalUnlock(); unlockErr != nil {
				slog.WarnContext(ctx, "release shipment lifecycle lock after persist", "artifact_id", artifact.ID, "error", unlockErr)
			}
		}()
		ctx = lockedCtx
	}
	unlock, err := lockArtifactMutation(ctx, ws, artifact.ID)
	if err != nil {
		return fmt.Errorf("lock artifact %s: %w", artifact.ID, err)
	}
	defer func() { _ = unlock() }()

	if err := guardBlockedShipmentMemberStatusMutation(ctx, ws, artifact); err != nil {
		return err
	}
	if err := artifact.Validate(); err != nil {
		return fmt.Errorf("validate artifact: %w", err)
	}
	if preserveDBOnlyLinks {
		if err := mergeDBOnlyLinksIntoArtifact(ctx, ws, artifact); err != nil {
			return fmt.Errorf("preserve database-only links: %w", err)
		}
	}

	currentPath, targetPath, err := resolveArtifactPersistPaths(ctx, ws, artifact, relocate)
	if err != nil {
		return err
	}
	currentSnapshot, err := snapshotFile(currentPath)
	if err != nil {
		return fmt.Errorf("snapshot current artifact file: %w", err)
	}
	targetSnapshot, err := snapshotFile(targetPath)
	if err != nil {
		return fmt.Errorf("snapshot target artifact file: %w", err)
	}
	if artifact.ArtifactType == "shipment" && currentSnapshot.Exists {
		current, _, parseErr := parseFile(currentPath)
		if parseErr != nil {
			return fmt.Errorf("parse current shipment %s before persist: %w", artifact.ID, parseErr)
		}
		if err := guardBlockedShipmentMembershipMutation(current, artifact); err != nil {
			return err
		}
	}

	if guard != nil {
		if err := guard(ctx); err != nil {
			return err
		}
	}

	envelope, hasEnvelope := ctx.Value(artifactWriteEnvelopeContextKey{}).(artifactWriteEnvelope)
	if envelope.audit {
		if envelope.operation == "" {
			return fmt.Errorf("governed artifact write %s has no operation: %w", artifact.ID, blerrors.ErrValidation)
		}
		if envelope.correlationID == "" {
			var correlationBytes [16]byte
			if _, err := rand.Read(correlationBytes[:]); err != nil {
				return fmt.Errorf("generate artifact write correlation id: %w", err)
			}
			envelope.correlationID = hex.EncodeToString(correlationBytes[:])
		}
		delta := map[string]any{
			"correlation_id": envelope.correlationID,
			"operation":      envelope.operation,
			"phase":          "intent",
			"changes":        envelope.changes,
		}
		if err := appendItemEventErr(ctx, ws, artifact.ID, "artifact_mutation", delta); err != nil {
			return fmt.Errorf("append artifact write intent %s: %w", artifact.ID, err)
		}
	}

	if currentPath != targetPath {
		if err := mkdirAllDurable(filepath.Dir(targetPath), WorkspaceDurableWrites(ws)); err != nil {
			return fmt.Errorf("create directory %s: %w", filepath.Dir(targetPath), err)
		}
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear target artifact path: %w", err)
		}
	}
	if !hasEnvelope {
		envelope = artifactWriteEnvelope{}
	}
	if err := writeArtifactFileGoverned(
		artifact,
		targetPath,
		currentPath,
		WorkspaceDurableWrites(ws),
		envelope,
		persistArtifactWriteFn,
	); err != nil {
		return fmt.Errorf("write artifact file: %w", err)
	}
	if currentPath != targetPath {
		if err := os.Remove(currentPath); err != nil {
			cleanupErr := os.Remove(targetPath)
			if cleanupErr != nil && !os.IsNotExist(cleanupErr) {
				return fmt.Errorf("remove old artifact file: %w; cleanup new artifact file: %v", err, cleanupErr)
			}
			return fmt.Errorf("remove old artifact file: %w", err)
		}
		// Durable cross-directory move: WriteArtifactFileWithOptions already made
		// the new dirent in the destination parent durable. Now fsync the SOURCE
		// parent so the removal of the old entry is durable too; otherwise a POSIX
		// power loss could resurrect the old dirent alongside the new one, leaving a
		// duplicate canonical artifact. This runs BEFORE the sole DB upsert below,
		// so surfacing ErrWriteIndeterminate here keeps the caller's error path
		// consistent — the upsert does not run and no completed-move rollback is
		// disturbed.
		if srcDir := filepath.Dir(currentPath); srcDir != filepath.Dir(targetPath) {
			if err := fsyncDirIfDurable(srcDir, WorkspaceDurableWrites(ws)); err != nil {
				return fmt.Errorf("fsync source dir after move: %w",
					fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, err))
			}
		}
	}
	if err := bldb.UpsertItem(ctx, ws.DB, artifact); err != nil {
		if restoreErr := restorePersistedArtifactFiles(currentSnapshot, targetSnapshot, currentPath, targetPath); restoreErr != nil {
			return fmt.Errorf("upsert item: %w; restore files: %v", err, restoreErr)
		}
		return fmt.Errorf("upsert item: %w", err)
	}
	if envelope.audit {
		delta := map[string]any{
			"correlation_id": envelope.correlationID,
			"operation":      envelope.operation,
			"phase":          "committed",
			"changes":        envelope.changes,
		}
		if err := appendItemEventErr(ctx, ws, artifact.ID, "artifact_mutation", delta); err != nil {
			return fmt.Errorf("append artifact write commit %s: %w", artifact.ID, err)
		}
	}
	return nil
}

func resolveArtifactPersistPaths(ctx context.Context, ws *Workspace, artifact *models.Artifact, relocate bool) (string, string, error) {
	currentPath, err := FindArtifactPath(ctx, ws, artifact.ID)
	if err != nil {
		return "", "", fmt.Errorf("find artifact path: %w", err)
	}
	if !relocate {
		return currentPath, currentPath, nil
	}

	backlogitDir := WorkspaceStorageRoot(ws.RootPath)
	registry, err := config.LoadRegistry(backlogitDir)
	if err != nil {
		return "", "", fmt.Errorf("load registry: %w", err)
	}
	targetDir := ResolveTargetDir(registry, artifact.ArtifactType, string(artifact.Status))

	currentRel, err := filepath.Rel(backlogitDir, filepath.Dir(currentPath))
	if err != nil {
		return "", "", fmt.Errorf("resolve relative path: %w", err)
	}
	if filepath.Clean(currentRel) == filepath.Clean(targetDir) {
		return currentPath, currentPath, nil
	}

	targetDirAbs := filepath.Join(backlogitDir, targetDir)
	targetPath := filepath.Join(targetDirAbs, filepath.Base(currentPath))
	return currentPath, targetPath, nil
}

func persistReturnedBlockedArtifacts(ctx context.Context, ws *Workspace, originalShipment, updatedShipment, originalItem, updatedItem *models.Artifact) (bool, error) {
	if err := persistArtifact(ctx, ws, updatedShipment, false); err != nil {
		return false, fmt.Errorf("update shipment %s: %w", updatedShipment.ID, err)
	}
	if err := persistArtifact(ctx, ws, updatedItem, true); err != nil {
		if rollbackErr := rollbackReturnedBlockedArtifacts(ctx, ws, originalShipment, originalItem); rollbackErr != nil {
			return false, &blerrors.MutationPartialError{
				Completed:         []string{"shipment-membership"},
				FailedStep:        "return-blocked-item",
				CompensationState: "partially-compensated",
				Class:             "double-fault",
				Cause:             errors.Join(err, rollbackErr),
			}
		}
		return true, fmt.Errorf("update item %s: %w", updatedItem.ID, err)
	}
	return false, nil
}

func rollbackReturnedBlockedArtifacts(ctx context.Context, ws *Workspace, originalShipment, originalItem *models.Artifact) error {
	ctx, cancel := boundedShipmentRecoveryContext(ctx)
	defer cancel()

	var rollbackErrs []error
	if err := persistArtifact(ctx, ws, originalItem, true); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("restore item %s: %w", originalItem.ID, err))
	}
	if err := persistArtifact(ctx, ws, originalShipment, false); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("restore shipment %s: %w", originalShipment.ID, err))
	}
	return errors.Join(rollbackErrs...)
}

func validateShipmentItemIDs(ctx context.Context, ws *Workspace, currentShipmentID string, itemIDs []string) error {
	normalizedIDs := uniqueNonEmptyStrings(itemIDs)
	if len(normalizedIDs) == 0 {
		return nil
	}

	shipments, err := bldb.QueryItems(ctx, ws.DB, bldb.QueryFilters{Type: "shipment", IncludeArchived: true})
	if err != nil {
		return fmt.Errorf("list shipments: %w", err)
	}

	activeAssignments := make(map[string]string)
	for _, existing := range shipments {
		if existing.ID == currentShipmentID || !shipmentStatusBlocksAssignment(existing.Status) {
			continue
		}
		for _, assignedItemID := range NormalizeShipmentItems(existing) {
			activeAssignments[assignedItemID] = existing.ID
		}
	}

	for _, itemID := range normalizedIDs {
		if itemID == currentShipmentID {
			return fmt.Errorf("shipment %s cannot include itself: %w", currentShipmentID, blerrors.ErrValidation)
		}

		artifact, loadErr := loadArtifact(ctx, ws, itemID)
		if loadErr != nil {
			return loadErr
		}
		if artifact.ArtifactType == "shipment" {
			return fmt.Errorf("artifact %s is a shipment and cannot be nested in a shipment: %w", itemID, blerrors.ErrValidation)
		}
		if assignedShipmentID, ok := activeAssignments[itemID]; ok {
			return fmt.Errorf("item %s already assigned to shipment %s: %w", itemID, assignedShipmentID, blerrors.ErrItemAlreadyAssigned)
		}
	}

	return nil
}

func shipmentStatusBlocksAssignment(status models.ArtifactStatus) bool {
	return status != models.StatusAbandoned && status != models.StatusShipped && status != models.StatusArchived
}

func shipmentMutationBlocked(status models.ArtifactStatus) bool {
	return status == models.StatusBlocked ||
		status == models.StatusShipped ||
		status == models.StatusAbandoned ||
		status == models.StatusArchived
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

// NormalizeShipmentItems is the single source of truth for reading a shipment's
// custom_fields["items"] into a normalized []string. It is a PURE READ: it does
// NOT mutate the artifact (unlike the mutator normalizeShipmentArtifact, which
// wraps this reader to canonicalize items on the CREATE/GET write path).
//
// It maps the lossy on-the-way-out representations of the SQLite JSON array
// (see docs/compound/go-patterns/f015-shipment-stash-patterns.md — treat
// []interface{} shipment CustomFields as lossy and normalize on every read):
// a []string is cloned, a []any is filtered to its string elements
// order-preserving, and nil/absent/unknown inputs yield an empty slice.
//
// CONTRACT: this function NEVER returns nil. An empty result is a non-nil
// []string{}. This is a JSON wire-shape invariant, not a stylistic choice: a
// nil []string marshals to null, whereas a non-nil empty slice marshals to [].
// The shipment items field is emitted through core.ShipmentView, which is
// marshaled by BOTH the CLI and the MCP list/get surfaces, so a nil here would
// surface as items: null on the wire. The end-to-end guard for this is
// TestListShipments_EmptyItems_NeverNull (internal/mcp). Do NOT "simplify" the
// []string branch back to the nil-able append([]string(nil), ...) form — that
// silently reintroduces the null-on-empty regression this consolidation removed.
func NormalizeShipmentItems(artifact *models.Artifact) []string {
	if artifact == nil || artifact.CustomFields == nil {
		return []string{}
	}

	raw, ok := artifact.CustomFields["items"]
	if !ok || raw == nil {
		return []string{}
	}

	switch items := raw.(type) {
	case []string:
		out := make([]string, len(items))
		copy(out, items)
		return out
	case []any:
		normalized := make([]string, 0, len(items))
		for _, item := range items {
			if value, ok := item.(string); ok {
				normalized = append(normalized, value)
			}
		}
		return normalized
	default:
		return []string{}
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneArtifact(artifact *models.Artifact) *models.Artifact {
	if artifact == nil {
		return nil
	}

	clone := *artifact
	clone.Labels = append([]string(nil), artifact.Labels...)
	clone.Dependencies = append([]models.DependencyEdge(nil), artifact.Dependencies...)
	clone.References = append([]string(nil), artifact.References...)
	if artifact.CustomFields != nil {
		clone.CustomFields = maps.Clone(artifact.CustomFields)
	}
	return &clone
}

func snapshotFile(path string) (fileSnapshot, error) {
	snapshot := fileSnapshot{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return snapshot, err
	}
	if info.IsDir() {
		return snapshot, fmt.Errorf("%s is a directory", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return snapshot, err
	}
	snapshot.Exists = true
	snapshot.Content = content
	return snapshot, nil
}

func restorePersistedArtifactFiles(currentSnapshot, targetSnapshot fileSnapshot, currentPath, targetPath string) error {
	var errs []error
	if currentPath == targetPath {
		if err := restoreSnapshot(currentSnapshot); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("remove target %s: %w", targetPath, err))
	}
	if err := restoreSnapshot(currentSnapshot); err != nil {
		errs = append(errs, err)
	}
	if err := restoreSnapshot(targetSnapshot); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func restoreSnapshot(snapshot fileSnapshot) error {
	if snapshot.Path == "" {
		return nil
	}
	if !snapshot.Exists {
		if err := os.Remove(snapshot.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", snapshot.Path, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(snapshot.Path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", snapshot.Path, err)
	}
	if err := os.WriteFile(snapshot.Path, snapshot.Content, 0o644); err != nil {
		return fmt.Errorf("restore %s: %w", snapshot.Path, err)
	}
	return nil
}

func shipmentOpsRoot(rootPath string) string {
	return filepath.Join(WorkspaceStorageRoot(rootPath), "ops")
}

func guardBlockedShipmentMemberStatusMutation(ctx context.Context, ws *Workspace, artifact *models.Artifact) error {
	if artifact == nil || artifact.ArtifactType == "shipment" {
		return nil
	}
	if shipmentOperationID(ctx) != "" {
		return nil
	}
	current, err := findArtifact(ctx, ws, artifact.ID)
	if err != nil {
		return fmt.Errorf("load current artifact %s for blocked shipment guard: %w", artifact.ID, err)
	}
	if current.Status == artifact.Status {
		return nil
	}

	refs, err := scanCanonicalArtifacts(ws)
	if err != nil {
		return fmt.Errorf("scan shipments guarding member %s: %w", artifact.ID, err)
	}
	for _, candidates := range refs {
		for _, candidate := range candidates {
			if candidate.artifactType != "shipment" || candidate.status != string(ShipmentBlocked) {
				continue
			}
			shipment, _, parseErr := parseFile(candidate.path)
			if parseErr != nil {
				return fmt.Errorf("parse blocked shipment %s guarding member %s: %w", candidate.id, artifact.ID, parseErr)
			}
			if containsString(NormalizeShipmentItems(shipment), artifact.ID) {
				return fmt.Errorf(
					"change status of member %s from %s to %s while shipment %s is blocked: %w",
					artifact.ID,
					current.Status,
					artifact.Status,
					shipment.ID,
					blerrors.ErrShipmentConflict,
				)
			}
		}
	}
	return nil
}

func returnBlockedJournalPath(rootPath, shipmentID, itemID string) string {
	return filepath.Join(shipmentOpsRoot(rootPath), fmt.Sprintf("return-blocked-%s-%s.json", shipmentID, itemID))
}

func newReturnBlockedJournal(
	shipment *models.Artifact,
	item *models.Artifact,
	targetShipment *models.Artifact,
	targetItem *models.Artifact,
	reason string,
) (returnBlockedJournal, error) {
	var correlationBytes [16]byte
	if _, err := rand.Read(correlationBytes[:]); err != nil {
		return returnBlockedJournal{}, fmt.Errorf("generate return-blocked correlation id: %w", err)
	}
	return returnBlockedJournal{
		SchemaVersion:  returnBlockedJournalSchemaVersion,
		CorrelationID:  hex.EncodeToString(correlationBytes[:]),
		Phase:          "intent",
		ShipmentID:     shipment.ID,
		ItemID:         item.ID,
		Shipment:       cloneArtifact(shipment),
		Item:           cloneArtifact(item),
		TargetShipment: cloneArtifact(targetShipment),
		TargetItem:     cloneArtifact(targetItem),
		Reason:         reason,
	}, nil
}

func validateReturnBlockedJournalTargets(journal returnBlockedJournal) error {
	if !containsString(NormalizeShipmentItems(journal.Shipment), journal.Item.ID) {
		return fmt.Errorf("return-blocked preimage does not contain item %s: %w",
			journal.Item.ID, blerrors.ErrValidation)
	}
	expectedShipment := cloneArtifact(journal.Shipment)
	if expectedShipment.CustomFields == nil {
		expectedShipment.CustomFields = map[string]any{}
	}
	expectedShipment.CustomFields["items"] = removeString(
		NormalizeShipmentItems(expectedShipment),
		journal.Item.ID,
	)
	expectedShipment.UpdatedAt = journal.TargetShipment.UpdatedAt
	shipmentMatches, err := recoveryArtifactMatchesAny(
		journal.TargetShipment,
		[]recoveryArtifactCandidate{{artifact: expectedShipment}},
	)
	if err != nil {
		return fmt.Errorf("compare return-blocked shipment target: %w", err)
	}

	expectedItem := cloneArtifact(journal.Item)
	if expectedItem.CustomFields == nil {
		expectedItem.CustomFields = map[string]any{}
	}
	expectedItem.Status = models.StatusBlocked
	expectedItem.CustomFields["blocked_reason"] = journal.Reason
	expectedItem.UpdatedAt = journal.TargetItem.UpdatedAt
	itemMatches, err := recoveryArtifactMatchesAny(
		journal.TargetItem,
		[]recoveryArtifactCandidate{{artifact: expectedItem}},
	)
	if err != nil {
		return fmt.Errorf("compare return-blocked item target: %w", err)
	}
	if !shipmentMatches || !itemMatches {
		return fmt.Errorf("return-blocked target is not derived from its durable preimage: %w",
			blerrors.ErrValidation)
	}
	return nil
}

func writeReturnBlockedJournal(ws *Workspace, shipment, item *models.Artifact, reasons ...string) error {
	reason := ""
	if len(reasons) > 0 {
		reason = reasons[0]
	}
	return writeReturnBlockedJournalRecord(ws, returnBlockedJournal{
		Shipment: cloneArtifact(shipment),
		Item:     cloneArtifact(item),
		Reason:   reason,
	})
}

func writeReturnBlockedJournalRecord(ws *Workspace, journal returnBlockedJournal) error {
	payload, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}
	if journal.Shipment == nil || journal.Item == nil {
		return fmt.Errorf("return-blocked journal preimage is incomplete: %w", blerrors.ErrValidation)
	}
	name := filepath.Base(returnBlockedJournalPath(ws.RootPath, journal.Shipment.ID, journal.Item.ID))
	if _, _, err := validateShipmentOperationJournalName(name); err != nil {
		return err
	}
	opsRoot, dir, err := shipmentOpsRootForWorkspace(ws, true)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := writeShipmentOperationJournalFile(dir, opsRoot, name, payload); err != nil {
		return fmt.Errorf("write return-blocked journal: %w", err)
	}
	return nil
}

func appendReturnBlockedEvidence(ctx context.Context, ws *Workspace, journal returnBlockedJournal) error {
	operationCtx := withShipmentOperation(ctx, journal.CorrelationID)
	for _, expected := range returnBlockedEvidenceSpecs(journal) {
		present, err := returnBlockedEvidencePresent(operationCtx, ws, journal, expected.itemID, expected.eventType,
			expected.delta)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if err := appendItemEventErr(operationCtx, ws, expected.itemID, expected.eventType, expected.delta); err != nil {
			return fmt.Errorf("append %s return-blocked evidence: %w", expected.itemID, err)
		}
	}
	return nil
}

type returnBlockedEvidenceSpec struct {
	itemID    string
	eventType string
	delta     map[string]any
}

func returnBlockedEvidenceSpecs(journal returnBlockedJournal) []returnBlockedEvidenceSpec {
	return []returnBlockedEvidenceSpec{
		{
			itemID:    journal.Shipment.ID,
			eventType: "shipment_item_returned_blocked",
			delta: map[string]any{
				"correlation_id": journal.CorrelationID,
				"evidence_id":    journal.CorrelationID + ":shipment",
				"item_id":        journal.Item.ID,
				"operation":      "return_blocked",
				"phase":          "applied",
				"reason":         journal.Reason,
			},
		},
		{
			itemID:    journal.Item.ID,
			eventType: "item_blocked",
			delta: map[string]any{
				"blocked_reason": journal.Reason,
				"correlation_id": journal.CorrelationID,
				"evidence_id":    journal.CorrelationID + ":item",
				"operation":      "return_blocked",
				"phase":          "applied",
				"shipment_id":    journal.Shipment.ID,
			},
		},
	}
}

func validateReturnBlockedEvidence(ctx context.Context, ws *Workspace, journal returnBlockedJournal) error {
	operationCtx := withShipmentOperation(ctx, journal.CorrelationID)
	for _, expected := range returnBlockedEvidenceSpecs(journal) {
		present, err := returnBlockedEvidencePresent(
			operationCtx,
			ws,
			journal,
			expected.itemID,
			expected.eventType,
			expected.delta,
		)
		if err != nil {
			return err
		}
		if !present {
			return fmt.Errorf(
				"committed return-blocked correlation %s lacks %s evidence on %s: %w",
				journal.CorrelationID,
				expected.eventType,
				expected.itemID,
				blerrors.ErrShipmentConflict,
			)
		}
	}
	return nil
}

func returnBlockedEvidencePresent(
	ctx context.Context,
	ws *Workspace,
	journal returnBlockedJournal,
	itemID string,
	eventType string,
	expected map[string]any,
) (bool, error) {
	itemEvents, err := events.ReadAllEvents(ctx, WorkspaceLogsRoot(ws.RootPath), itemID)
	if err != nil {
		return false, fmt.Errorf("read return-blocked evidence for %s: %w", itemID, err)
	}
	for _, event := range itemEvents {
		if !isShipmentOperationEvent(event, journal.CorrelationID) {
			continue
		}
		if event.EventType != eventType {
			return false, fmt.Errorf("return-blocked correlation %s has unexpected event type %s on %s: %w",
				journal.CorrelationID, event.EventType, itemID, blerrors.ErrShipmentConflict)
		}
		for key, value := range expected {
			if event.Delta[key] != value {
				return false, fmt.Errorf(
					"return-blocked correlation %s has conflicting %s evidence on %s: %w",
					journal.CorrelationID, key, itemID, blerrors.ErrShipmentConflict,
				)
			}
		}
		return true, nil
	}
	return false, nil
}

func removeReturnBlockedJournal(ctx context.Context, ws *Workspace, shipmentID, itemID string) {
	removeShipmentOperationJournal(ctx, ws, returnBlockedJournalPath(ws.RootPath, shipmentID, itemID))
}

func removeShipmentOperationJournal(ctx context.Context, ws *Workspace, journalPath string) {
	remove := os.Remove
	if ws != nil && ws.removeShipmentOperationJournal != nil {
		remove = ws.removeShipmentOperationJournal
	}
	if err := remove(journalPath); err != nil && !os.IsNotExist(err) {
		slog.WarnContext(ctx, "remove shipment operation journal", "path", journalPath, "error", err)
	}
}

func recoverPendingShipmentOperations(ctx context.Context, ws *Workspace) error {
	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobalRaw(
		ctx,
		ws,
		"recoverPendingShipmentOperations",
	)
	if err != nil {
		return fmt.Errorf("lock shipment lifecycle recovery: %w", err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle recovery lock", "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	records, err := loadShipmentOperationJournals(ws)
	if err != nil {
		return fmt.Errorf("validate shipment operation journals: %w", err)
	}
	currentOperationID := shipmentOperationID(ctx)
	var recoveryErrs []error
	for _, record := range records {
		if record.kind == shipmentLifecycleJournalKind {
			journal := record.lifecycle
			if journal.Phase != "intent" ||
				(currentOperationID != "" && journal.CorrelationID == currentOperationID) {
				continue
			}
			if journal.Preimage.Shipment == nil || journal.ShipmentID == "" {
				recoveryErrs = append(recoveryErrs,
					fmt.Errorf("shipment lifecycle journal %s is incomplete: %w",
						record.path, blerrors.ErrValidation))
				continue
			}
			membershipUnlock, lockErr := lockShipmentMembership(ctx, ws, journal.ShipmentID)
			if lockErr != nil {
				recoveryErrs = append(recoveryErrs,
					fmt.Errorf("lock shipment %s recovery membership: %w", journal.ShipmentID, lockErr))
				continue
			}
			lockIDs := append([]string{journal.ShipmentID}, NormalizeShipmentItems(journal.Preimage.Shipment)...)
			for _, related := range journal.Preimage.Related {
				if related != nil {
					lockIDs = append(lockIDs, related.ID)
				}
			}
			lockedCtx, artifactUnlock, lockErr := lockArtifactMutations(ctx, ws, lockIDs)
			if lockErr == nil {
				_, lockErr = reconcileShipmentLifecycleIntent(lockedCtx, ws, record.path, journal)
			}
			unlockErr := artifactUnlock
			if unlockErr != nil {
				lockErr = errors.Join(lockErr, unlockErr())
			}
			lockErr = errors.Join(lockErr, membershipUnlock())
			if lockErr != nil {
				recoveryErrs = append(recoveryErrs,
					fmt.Errorf("recover shipment %s lifecycle journal: %w", journal.ShipmentID, lockErr))
			}
			continue
		}
		journal := record.returnBlocked
		if currentOperationID != "" && journal.CorrelationID == currentOperationID {
			continue
		}
		if journal.Shipment == nil || journal.Item == nil {
			recoveryErrs = append(recoveryErrs,
				fmt.Errorf("shipment journal %s is incomplete: %w", record.path, blerrors.ErrValidation))
			continue
		}
		membershipUnlock, lockErr := lockShipmentMembership(ctx, ws, journal.Shipment.ID)
		if lockErr != nil {
			recoveryErrs = append(recoveryErrs,
				fmt.Errorf("lock shipment %s return-blocked recovery membership: %w",
					journal.Shipment.ID, lockErr))
			continue
		}
		lockedCtx, artifactUnlock, lockErr := lockArtifactMutations(
			ctx,
			ws,
			[]string{journal.Shipment.ID, journal.Item.ID},
		)
		if lockErr == nil {
			lockedCtx = withShipmentOperation(lockedCtx, journal.CorrelationID)
			lockErr = recoverReturnBlockedJournal(lockedCtx, ws, record)
		}
		if artifactUnlock != nil {
			lockErr = errors.Join(lockErr, artifactUnlock())
		}
		lockErr = errors.Join(lockErr, membershipUnlock())
		if lockErr != nil {
			recoveryErrs = append(recoveryErrs,
				fmt.Errorf("recover shipment %s return-blocked journal: %w",
					journal.Shipment.ID, lockErr))
		}
	}
	return errors.Join(recoveryErrs...)
}

func recoverReturnBlockedJournal(ctx context.Context, ws *Workspace, record shipmentOperationJournalRecord) error {
	journal := record.returnBlocked
	if journal.Shipment == nil || journal.Item == nil {
		return fmt.Errorf("shipment journal %s is incomplete", record.path)
	}
	if journal.SchemaVersion == "" {
		return recoverLegacyReturnBlockedJournal(ctx, ws, record)
	}
	currentShipment, err := findArtifact(ctx, ws, journal.Shipment.ID)
	if err != nil {
		return fmt.Errorf("load shipment %s return recovery state: %w", journal.Shipment.ID, err)
	}
	currentItem, err := findArtifact(ctx, ws, journal.Item.ID)
	if err != nil {
		return fmt.Errorf("load item %s return recovery state: %w", journal.Item.ID, err)
	}
	shipmentPreimage, err := recoveryArtifactMatchesAny(
		currentShipment,
		[]recoveryArtifactCandidate{{artifact: journal.Shipment}},
	)
	if err != nil {
		return fmt.Errorf("compare shipment %s return recovery state: %w", journal.Shipment.ID, err)
	}
	shipmentTarget, err := recoveryArtifactMatchesAny(
		currentShipment,
		[]recoveryArtifactCandidate{{artifact: journal.TargetShipment}},
	)
	if err != nil {
		return fmt.Errorf("compare shipment %s return recovery target: %w", journal.Shipment.ID, err)
	}
	itemPreimage, err := recoveryArtifactMatchesAny(
		currentItem,
		[]recoveryArtifactCandidate{{artifact: journal.Item}},
	)
	if err != nil {
		return fmt.Errorf("compare item %s return recovery state: %w", journal.Item.ID, err)
	}
	itemTarget, err := recoveryArtifactMatchesAny(
		currentItem,
		[]recoveryArtifactCandidate{{artifact: journal.TargetItem}},
	)
	if err != nil {
		return fmt.Errorf("compare item %s return recovery target: %w", journal.Item.ID, err)
	}
	if (!shipmentPreimage && !shipmentTarget) || (!itemPreimage && !itemTarget) {
		return fmt.Errorf(
			"return-blocked journal %s diverged from its durable intent: %w",
			record.path,
			blerrors.ErrShipmentConflict,
		)
	}

	if journal.Phase == "committed" {
		if !shipmentTarget || !itemTarget {
			return fmt.Errorf(
				"committed return-blocked journal %s does not match its target state: %w",
				record.path,
				blerrors.ErrShipmentConflict,
			)
		}
		if err := validateReturnBlockedEvidence(ctx, ws, journal); err != nil {
			return err
		}
		removeReturnBlockedJournal(ctx, ws, journal.Shipment.ID, journal.Item.ID)
		return nil
	}

	if shipmentTarget && itemTarget {
		if err := appendReturnBlockedEvidence(ctx, ws, journal); err != nil {
			return fmt.Errorf("complete return-blocked evidence for journal %s: %w", record.path, err)
		}
		journal.Phase = "committed"
		if err := writeReturnBlockedJournalRecord(ws, journal); err != nil {
			return fmt.Errorf("persist recovered return-blocked commit %s: %w", record.path, err)
		}
		removeReturnBlockedJournal(ctx, ws, journal.Shipment.ID, journal.Item.ID)
		return nil
	}

	recoveryCtx, cancel := boundedShipmentRecoveryContext(ctx)
	defer cancel()
	if err := persistArtifact(recoveryCtx, ws, journal.Item, true); err != nil {
		return fmt.Errorf("restore item %s from journal: %w", journal.Item.ID, err)
	}
	if err := persistArtifact(recoveryCtx, ws, journal.Shipment, false); err != nil {
		return fmt.Errorf("restore shipment %s from journal: %w", journal.Shipment.ID, err)
	}
	if err := removeShipmentOperationEvents(
		recoveryCtx,
		ws,
		[]string{journal.Shipment.ID, journal.Item.ID},
		journal.CorrelationID,
	); err != nil {
		return fmt.Errorf("remove rolled-back return-blocked evidence: %w", err)
	}
	removeReturnBlockedJournal(recoveryCtx, ws, journal.Shipment.ID, journal.Item.ID)
	return nil
}

func recoverLegacyReturnBlockedJournal(
	ctx context.Context,
	ws *Workspace,
	record shipmentOperationJournalRecord,
) error {
	journal := record.returnBlocked
	currentShipment, err := findArtifact(ctx, ws, journal.Shipment.ID)
	if err != nil {
		return fmt.Errorf("load shipment %s legacy return recovery state: %w", journal.Shipment.ID, err)
	}
	currentItem, err := findArtifact(ctx, ws, journal.Item.ID)
	if err != nil {
		return fmt.Errorf("load item %s legacy return recovery state: %w", journal.Item.ID, err)
	}
	shipmentTarget := cloneArtifact(journal.Shipment)
	shipmentTarget.CustomFields["items"] = removeString(NormalizeShipmentItems(shipmentTarget), journal.Item.ID)
	itemTargets := []recoveryArtifactCandidate{{artifact: journal.Item}}
	if journal.Reason != "" {
		itemTarget := cloneArtifact(journal.Item)
		itemTarget.Status = models.StatusBlocked
		if itemTarget.CustomFields == nil {
			itemTarget.CustomFields = map[string]any{}
		}
		itemTarget.CustomFields["blocked_reason"] = journal.Reason
		itemTargets = append(itemTargets, recoveryArtifactCandidate{artifact: itemTarget, ignoreUpdatedAt: true})
	}
	shipmentMatches, err := recoveryArtifactMatchesAny(currentShipment, []recoveryArtifactCandidate{
		{artifact: journal.Shipment},
		{artifact: shipmentTarget, ignoreUpdatedAt: true},
	})
	if err != nil {
		return fmt.Errorf("compare shipment %s legacy return recovery state: %w", journal.Shipment.ID, err)
	}
	itemMatches, err := recoveryArtifactMatchesAny(currentItem, itemTargets)
	if err != nil {
		return fmt.Errorf("compare item %s legacy return recovery state: %w", journal.Item.ID, err)
	}
	if !shipmentMatches || !itemMatches {
		return fmt.Errorf(
			"return-blocked journal %s diverged from its durable preimage: %w",
			record.path,
			blerrors.ErrShipmentConflict,
		)
	}

	recoveryCtx, cancel := boundedShipmentRecoveryContext(ctx)
	defer cancel()
	if err := persistArtifact(recoveryCtx, ws, journal.Shipment, false); err != nil {
		return fmt.Errorf("restore shipment %s from legacy journal: %w", journal.Shipment.ID, err)
	}
	if err := persistArtifact(recoveryCtx, ws, journal.Item, true); err != nil {
		return fmt.Errorf("restore item %s from legacy journal: %w", journal.Item.ID, err)
	}
	removeReturnBlockedJournal(recoveryCtx, ws, journal.Shipment.ID, journal.Item.ID)
	return nil
}
