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
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

type shipmentOperationJournalKind string

const (
	shipmentLifecycleJournalKind shipmentOperationJournalKind = "shipment_lifecycle"
	returnBlockedJournalKind     shipmentOperationJournalKind = "return_blocked"
)

var (
	shipmentLifecycleJournalNamePattern = regexp.MustCompile(
		`^shipment-operation-([0-9a-f]{32})\.json$`,
	)
	returnBlockedJournalNamePattern = regexp.MustCompile(
		`^return-blocked-([0-9]+(?:\.[0-9]+)*-[A-Za-z]+)-([0-9]+(?:\.[0-9]+)*-[A-Za-z]+)\.json$`,
	)
	returnBlockedCorrelationIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

const (
	shipmentOperationJournalTempPrefix = ".shipment-operation-"
	shipmentOperationJournalTempSuffix = ".tmp"
)

type shipmentOperationJournalRecord struct {
	name          string
	path          string
	kind          shipmentOperationJournalKind
	lifecycle     shipmentLifecycleJournal
	returnBlocked returnBlockedJournal
}

type shipmentOperationJournalValidationError struct {
	path string
	kind shipmentOperationJournalKind
	err  error
}

func (e *shipmentOperationJournalValidationError) Error() string {
	return fmt.Sprintf("shipment operation journal %s is invalid: %v", e.path, e.err)
}

func (e *shipmentOperationJournalValidationError) Unwrap() error {
	return e.err
}

func newShipmentOperationJournalValidationError(
	path string,
	kind shipmentOperationJournalKind,
	err error,
) error {
	return &shipmentOperationJournalValidationError{
		path: path,
		kind: kind,
		err:  err,
	}
}

func shipmentOpsRootForWorkspace(ws *Workspace, create bool) (string, *os.File, error) {
	if ws == nil {
		return "", nil, fmt.Errorf("shipment operations workspace is nil: %w", blerrors.ErrValidation)
	}
	storageRoot := workspaceStorageRoot(ws)
	absStorageRoot, err := filepath.Abs(storageRoot)
	if err != nil {
		return "", nil, fmt.Errorf("resolve shipment operations storage root: %w", err)
	}
	realStorageRoot, err := filepath.EvalSymlinks(absStorageRoot)
	if err != nil {
		return "", nil, fmt.Errorf("resolve shipment operations storage root real path: %w", err)
	}

	opsRoot := filepath.Join(realStorageRoot, "ops")
	info, err := os.Lstat(opsRoot)
	if errors.Is(err, os.ErrNotExist) && create {
		if err := mkdirAllDurable(opsRoot, true); err != nil {
			return "", nil, fmt.Errorf("create shipment operations directory: %w", err)
		}
		info, err = os.Lstat(opsRoot)
	}
	if err != nil {
		return "", nil, fmt.Errorf("inspect shipment operations directory: %w", err)
	}
	redirected, err := IsSymlinkOrReparsePoint(info, opsRoot)
	if err != nil {
		return "", nil, fmt.Errorf("inspect shipment operations directory redirect: %w", err)
	}
	if redirected {
		return "", nil, fmt.Errorf("shipment operations directory is redirected: %w", blerrors.ErrValidation)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("shipment operations path is not a directory: %w", blerrors.ErrValidation)
	}
	realOpsRoot, err := filepath.EvalSymlinks(opsRoot)
	if err != nil {
		return "", nil, fmt.Errorf("resolve shipment operations directory real path: %w", err)
	}
	if filepath.Clean(realOpsRoot) != filepath.Clean(opsRoot) ||
		!pathContained(realStorageRoot, realOpsRoot) {
		return "", nil, fmt.Errorf("shipment operations directory resolves outside the real storage root: %w",
			blerrors.ErrValidation)
	}

	dir, err := openShipmentOpsDirectory(realStorageRoot, realOpsRoot)
	if err != nil {
		return "", nil, err
	}
	return realOpsRoot, dir, nil
}

func validateShipmentOperationJournalName(name string) (shipmentOperationJournalKind, []string, error) {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return "", nil, fmt.Errorf("shipment operation journal name %q is not a base filename: %w",
			name, blerrors.ErrValidation)
	}
	if matches := shipmentLifecycleJournalNamePattern.FindStringSubmatch(name); matches != nil {
		return shipmentLifecycleJournalKind, matches[1:], nil
	}
	if matches := returnBlockedJournalNamePattern.FindStringSubmatch(name); matches != nil {
		return returnBlockedJournalKind, matches[1:], nil
	}
	return "", nil, fmt.Errorf("shipment operation journal name %q is not in an accepted filename class: %w",
		name, blerrors.ErrValidation)
}

func shipmentOperationJournalTempName(journalName string) (string, error) {
	if _, _, err := validateShipmentOperationJournalName(journalName); err != nil {
		return "", err
	}
	return shipmentOperationJournalTempPrefix + journalName + shipmentOperationJournalTempSuffix, nil
}

func shipmentOperationJournalTempTarget(name string) (string, bool) {
	if !strings.HasPrefix(name, shipmentOperationJournalTempPrefix) ||
		!strings.HasSuffix(name, shipmentOperationJournalTempSuffix) {
		return "", false
	}
	target := strings.TrimSuffix(
		strings.TrimPrefix(name, shipmentOperationJournalTempPrefix),
		shipmentOperationJournalTempSuffix,
	)
	if target == "" {
		return "", false
	}
	if _, _, err := validateShipmentOperationJournalName(target); err != nil {
		return "", false
	}
	expected, err := shipmentOperationJournalTempName(target)
	return target, err == nil && expected == name
}

func inspectShipmentOperationJournals(
	ws *Workspace,
) ([]shipmentOperationJournalRecord, []error, error) {
	return inspectShipmentOperationJournalsWithTempCleanup(ws, true)
}

func inspectShipmentOperationJournalsReadOnly(
	ws *Workspace,
) ([]shipmentOperationJournalRecord, []error, error) {
	return inspectShipmentOperationJournalsWithTempCleanup(ws, false)
}

func inspectShipmentOperationJournalsWithTempCleanup(
	ws *Workspace,
	cleanupTempResidue bool,
) ([]shipmentOperationJournalRecord, []error, error) {
	opsRoot, dir, err := shipmentOpsRootForWorkspace(ws, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer dir.Close()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, nil, fmt.Errorf("enumerate shipment operations directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	records := make([]shipmentOperationJournalRecord, 0, len(entries))
	var validationErrs []error
	for _, entry := range entries {
		name := entry.Name()
		if _, temp := shipmentOperationJournalTempTarget(name); temp {
			info, infoErr := entry.Info()
			if infoErr != nil {
				validationErrs = append(validationErrs,
					fmt.Errorf("inspect shipment operation temp file %s: %w", name, infoErr))
				continue
			}
			redirected, redirectErr := IsSymlinkOrReparsePoint(info, filepath.Join(opsRoot, name))
			if redirectErr != nil {
				validationErrs = append(validationErrs,
					fmt.Errorf("inspect shipment operation temp file %s redirect: %w", name, redirectErr))
				continue
			}
			if redirected || !info.Mode().IsRegular() {
				validationErrs = append(validationErrs,
					fmt.Errorf("shipment operation temp file %s is not a regular unredirected file: %w",
						name, blerrors.ErrValidation))
				continue
			}
			if cleanupTempResidue {
				if removeErr := removeShipmentOperationJournalTempFile(dir, opsRoot, name); removeErr != nil {
					validationErrs = append(validationErrs,
						fmt.Errorf("remove shipment operation temp residue %s: %w", name, removeErr))
				}
			}
			continue
		}
		kind, captures, nameErr := validateShipmentOperationJournalName(name)
		if nameErr != nil {
			validationErrs = append(validationErrs, nameErr)
			continue
		}
		journalPath := filepath.Join(opsRoot, name)
		info, infoErr := entry.Info()
		if infoErr != nil {
			validationErrs = append(validationErrs,
				newShipmentOperationJournalValidationError(
					journalPath,
					kind,
					fmt.Errorf("inspect journal: %w", infoErr),
				))
			continue
		}
		redirected, redirectErr := IsSymlinkOrReparsePoint(info, journalPath)
		if redirectErr != nil {
			validationErrs = append(validationErrs,
				newShipmentOperationJournalValidationError(
					journalPath,
					kind,
					fmt.Errorf("inspect journal redirect: %w", redirectErr),
				))
			continue
		}
		if redirected || !info.Mode().IsRegular() {
			validationErrs = append(validationErrs,
				newShipmentOperationJournalValidationError(
					journalPath,
					kind,
					fmt.Errorf("journal is not a regular unredirected file: %w", blerrors.ErrValidation),
				))
			continue
		}

		data, readErr := readShipmentOperationJournalFile(dir, opsRoot, name)
		if readErr != nil {
			validationErrs = append(validationErrs,
				newShipmentOperationJournalValidationError(
					journalPath,
					kind,
					fmt.Errorf("read journal: %w", readErr),
				))
			continue
		}
		record := shipmentOperationJournalRecord{
			name: name,
			path: journalPath,
			kind: kind,
		}
		switch kind {
		case shipmentLifecycleJournalKind:
			if decodeErr := decodeShipmentOperationJournal(data, &record.lifecycle); decodeErr != nil {
				validationErrs = append(validationErrs,
					newShipmentOperationJournalValidationError(
						journalPath,
						kind,
						fmt.Errorf("parse lifecycle journal: %v: %w", decodeErr, blerrors.ErrValidation),
					))
				continue
			}
			if validationErr := validateShipmentLifecycleJournalRecord(record.lifecycle, captures[0]); validationErr != nil {
				validationErrs = append(validationErrs,
					newShipmentOperationJournalValidationError(journalPath, kind, validationErr))
				continue
			}
		case returnBlockedJournalKind:
			if decodeErr := decodeShipmentOperationJournal(data, &record.returnBlocked); decodeErr != nil {
				validationErrs = append(validationErrs,
					newShipmentOperationJournalValidationError(
						journalPath,
						kind,
						fmt.Errorf("parse return-blocked journal: %v: %w", decodeErr, blerrors.ErrValidation),
					))
				continue
			}
			if validationErr := validateReturnBlockedJournal(record.returnBlocked, captures); validationErr != nil {
				validationErrs = append(validationErrs,
					newShipmentOperationJournalValidationError(journalPath, kind, validationErr))
				continue
			}
		}
		records = append(records, record)
	}
	return records, validationErrs, nil
}

func validateShipmentLifecycleJournalRecord(
	journal shipmentLifecycleJournal,
	filenameCorrelationID string,
) error {
	if journal.SchemaVersion != "shipment-operation/v1" {
		return fmt.Errorf("unsupported lifecycle journal schema %q: %w",
			journal.SchemaVersion, blerrors.ErrValidation)
	}
	if journal.CorrelationID != filenameCorrelationID {
		return fmt.Errorf("correlation id does not match filename: %w", blerrors.ErrValidation)
	}
	switch journal.Phase {
	case "intent", "committed", "compensated":
	default:
		return fmt.Errorf("unsupported lifecycle phase %q: %w", journal.Phase, blerrors.ErrValidation)
	}
	switch journal.Operation {
	case "block", "unblock", "normalize", "claim", "dispose",
		shipmentOperationRemoveItem, shipmentOperationMoveItem:
	default:
		return fmt.Errorf("unsupported lifecycle operation %q: %w", journal.Operation, blerrors.ErrValidation)
	}
	switch journal.RecoveryPolicy {
	case "rollback", "roll_forward":
	default:
		return fmt.Errorf("unsupported lifecycle recovery policy %q: %w",
			journal.RecoveryPolicy, blerrors.ErrValidation)
	}
	if journal.ShipmentID == "" ||
		journal.Preimage.Shipment == nil ||
		journal.Preimage.Shipment.ID != journal.ShipmentID ||
		journal.Preimage.Shipment.ArtifactType != "shipment" {
		return fmt.Errorf("lifecycle journal cannot prove shipment ownership: %w", blerrors.ErrValidation)
	}
	if journal.Target == "" {
		return fmt.Errorf("lifecycle journal target is empty: %w", blerrors.ErrValidation)
	}
	if journal.Operation == "claim" && journal.Phase == "intent" && len(journal.Preimage.Related) != 0 {
		return fmt.Errorf("nonterminal claim lifecycle journal contains related preimages: %w",
			blerrors.ErrValidation)
	}
	switch journal.Operation {
	case shipmentOperationRemoveItem:
		if journal.RecoveryPolicy != shipmentOperationRecoveryPolicyRollback ||
			len(journal.Preimage.Related) != 0 ||
			!containsString(NormalizeShipmentItems(journal.Preimage.Shipment), journal.Target) {
			return fmt.Errorf("remove_item lifecycle tuple must be rollback with a manifest member target and no related preimages: %w",
				blerrors.ErrValidation)
		}
	case shipmentOperationMoveItem:
		if journal.RecoveryPolicy != shipmentOperationRecoveryPolicyRollback ||
			journal.Target == journal.ShipmentID ||
			len(journal.Preimage.Related) != 1 ||
			journal.Preimage.Related[0] == nil ||
			journal.Preimage.Related[0].ID != journal.Target ||
			journal.Preimage.Related[0].ArtifactType != "shipment" {
			return fmt.Errorf("move_item lifecycle tuple must be rollback with the target shipment as its only related preimage: %w",
				blerrors.ErrValidation)
		}
	case "claim":
		if journal.RecoveryPolicy != "rollback" || journal.Target != "active" {
			return fmt.Errorf("claim lifecycle tuple must be rollback/active: %w", blerrors.ErrValidation)
		}
	case "dispose":
		if journal.RecoveryPolicy != "rollback" || journal.Target != "abandoned" {
			return fmt.Errorf("dispose lifecycle tuple must be rollback/abandoned: %w", blerrors.ErrValidation)
		}
	case "block":
		if journal.Target != "blocked" ||
			(journal.RecoveryPolicy != "rollback" && journal.RecoveryPolicy != "roll_forward") {
			return fmt.Errorf("block lifecycle tuple must target blocked with rollback or roll_forward: %w",
				blerrors.ErrValidation)
		}
		if journal.RecoveryPolicy == "roll_forward" && strings.TrimSpace(journal.SnapshotRef) == "" {
			return fmt.Errorf("block roll_forward lifecycle tuple requires a snapshot: %w",
				blerrors.ErrValidation)
		}
	case "unblock":
		if journal.RecoveryPolicy != "rollback" ||
			(journal.Target != "queued" && journal.Target != "active") {
			return fmt.Errorf("unblock lifecycle tuple must be rollback/queued or rollback/active: %w",
				blerrors.ErrValidation)
		}
	case "normalize":
		if journal.RecoveryPolicy != "roll_forward" ||
			journal.Target != "blocked" ||
			strings.TrimSpace(journal.SnapshotRef) == "" {
			return fmt.Errorf("normalize lifecycle tuple must be roll_forward/blocked with snapshot: %w",
				blerrors.ErrValidation)
		}
	}

	memberIDs := NormalizeShipmentItems(journal.Preimage.Shipment)
	members := make(map[string]struct{}, len(journal.Preimage.Members))
	for _, member := range journal.Preimage.Members {
		if member == nil || member.ID == "" {
			return fmt.Errorf("lifecycle journal has an incomplete member preimage: %w",
				blerrors.ErrValidation)
		}
		if _, duplicate := members[member.ID]; duplicate {
			return fmt.Errorf("lifecycle journal repeats member %s: %w",
				member.ID, blerrors.ErrValidation)
		}
		members[member.ID] = struct{}{}
	}
	if len(members) != len(memberIDs) {
		return fmt.Errorf("lifecycle journal member preimage does not cover the manifest: %w",
			blerrors.ErrValidation)
	}
	for _, memberID := range memberIDs {
		if _, found := members[memberID]; !found {
			return fmt.Errorf("lifecycle journal is missing member %s: %w",
				memberID, blerrors.ErrValidation)
		}
	}
	return nil
}

func validateReturnBlockedJournal(journal returnBlockedJournal, captures []string) error {
	if journal.Shipment == nil || journal.Item == nil ||
		len(captures) != 2 ||
		journal.Shipment.ID != captures[0] ||
		journal.Item.ID != captures[1] {
		return fmt.Errorf("journal does not match its filename: %w", blerrors.ErrValidation)
	}
	if journal.SchemaVersion == "" {
		return nil
	}
	if journal.SchemaVersion != returnBlockedJournalSchemaVersion ||
		!returnBlockedCorrelationIDPattern.MatchString(journal.CorrelationID) ||
		(journal.Phase != "intent" && journal.Phase != "committed") ||
		journal.ShipmentID != journal.Shipment.ID ||
		journal.ItemID != journal.Item.ID ||
		journal.TargetShipment == nil ||
		journal.TargetItem == nil ||
		journal.TargetShipment.ID != journal.Shipment.ID ||
		journal.TargetItem.ID != journal.Item.ID {
		return fmt.Errorf("journal durable intent is incomplete: %w", blerrors.ErrValidation)
	}
	if err := validateReturnBlockedJournalTargets(journal); err != nil {
		return err
	}
	return nil
}

func loadShipmentOperationJournals(ws *Workspace) ([]shipmentOperationJournalRecord, error) {
	records, validationErrs, err := inspectShipmentOperationJournals(ws)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(validationErrs...); err != nil {
		return nil, err
	}
	return records, nil
}

func lifecycleJournalValidationErrors(validationErrs []error) error {
	var lifecycleErrs []error
	for _, validationErr := range validationErrs {
		var journalErr *shipmentOperationJournalValidationError
		if errors.As(validationErr, &journalErr) && journalErr.kind == shipmentLifecycleJournalKind {
			lifecycleErrs = append(lifecycleErrs, journalErr)
		}
	}
	return errors.Join(lifecycleErrs...)
}

func decodeShipmentOperationJournal(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing content: %w", blerrors.ErrValidation)
	}
	return nil
}

func writeShipmentLifecycleJournalForWorkspace(
	ws *Workspace,
	name string,
	journal shipmentLifecycleJournal,
) (string, error) {
	kind, captures, err := validateShipmentOperationJournalName(name)
	if err != nil {
		return "", err
	}
	if kind != shipmentLifecycleJournalKind || journal.CorrelationID != captures[0] {
		return "", fmt.Errorf("shipment lifecycle journal %s does not match correlation id %q: %w",
			name, journal.CorrelationID, blerrors.ErrValidation)
	}
	opsRoot, dir, err := shipmentOpsRootForWorkspace(ws, true)
	if err != nil {
		return "", err
	}
	defer dir.Close()

	payload, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal shipment lifecycle journal: %w", err)
	}
	if err := writeShipmentOperationJournalFile(dir, opsRoot, name, payload); err != nil {
		return "", err
	}
	return filepath.Join(opsRoot, name), nil
}

const (
	shipmentOperationPhaseIntent    = "intent"
	shipmentOperationPhaseCommitted = "committed"

	shipmentOperationRemoveItem = "remove_item"
	shipmentOperationMoveItem   = "move_item"

	shipmentOperationRecoveryPolicyRollback = "rollback"
)

// ShipmentManifestOptions carries the governed-override and preview inputs
// shared by the manifest replanning operations.
type ShipmentManifestOptions struct {
	Force  bool   // overrides the queued-only guard (never the blocked seal)
	Reason string // REQUIRED non-empty whenever Force is set
	DryRun bool   // read-only preview, zero writes
}

// RemoveFromShipmentResult reports a RemoveFromShipment outcome or preview.
type RemoveFromShipmentResult struct {
	ShipmentID    string   `json:"shipment_id"`
	ItemID        string   `json:"item_id"`
	Status        string   `json:"status"`
	DryRun        bool     `json:"dry_run"`
	Force         bool     `json:"force,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	Requeued      bool     `json:"requeued,omitempty"`
	CorrelationID string   `json:"correlation_id,omitempty"`
	ItemsBefore   []string `json:"items_before"`
	ItemsAfter    []string `json:"items_after"`
}

// MoveShipmentItemResult reports a MoveShipmentItem outcome or preview.
type MoveShipmentItemResult struct {
	ItemID         string   `json:"item_id"`
	FromShipmentID string   `json:"from_shipment_id"`
	ToShipmentID   string   `json:"to_shipment_id"`
	Status         string   `json:"status"`
	DryRun         bool     `json:"dry_run"`
	Force          bool     `json:"force,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	Requeued       bool     `json:"requeued,omitempty"`
	CorrelationID  string   `json:"correlation_id,omitempty"`
	FromBefore     []string `json:"from_before"`
	FromAfter      []string `json:"from_after"`
	ToBefore       []string `json:"to_before"`
	ToAfter        []string `json:"to_after"`
}

// hasBlockedShipmentSeal reports whether a shipment is blocked or still carries
// a blocked-manifest envelope; either seals its membership.
func hasBlockedShipmentSeal(shipment *models.Artifact) bool {
	if shipment == nil {
		return false
	}
	if shipment.Status == models.StatusBlocked {
		return true
	}
	return shipment.CustomFields != nil && shipment.CustomFields["blocked_at"] != nil
}

// validateShipmentManifestQueueGuard enforces the queued-only default for
// manifest replanning. A blocked shipment is refused unconditionally (even with
// Force) until the governed unblock runs. Other non-queued shipments require
// Force and a non-empty Reason.
func validateShipmentManifestQueueGuard(
	shipment *models.Artifact,
	opts ShipmentManifestOptions,
	action string,
) error {
	if hasBlockedShipmentSeal(shipment) {
		return fmt.Errorf("shipment %s membership is sealed while blocked; run governed unblock before %s: %w",
			shipment.ID, action, blerrors.ErrShipmentConflict)
	}
	if shipment.Status == models.StatusQueued {
		return nil
	}
	if !opts.Force {
		return fmt.Errorf("%s on non-queued shipment %s (status=%s) requires force and reason: %w",
			action, shipment.ID, shipment.Status, blerrors.ErrShipmentConflict)
	}
	if strings.TrimSpace(opts.Reason) == "" {
		return fmt.Errorf("%s on non-queued shipment %s with force requires a non-empty reason: %w",
			action, shipment.ID, blerrors.ErrValidation)
	}
	return nil
}

func validateShipmentManifestOptions(opts ShipmentManifestOptions) error {
	if opts.Force && strings.TrimSpace(opts.Reason) == "" {
		return fmt.Errorf("force requires a non-empty reason: %w", blerrors.ErrValidation)
	}
	return nil
}

// ensureNoPendingShipmentRecoveryReadOnly refuses a preview while any unfinished
// recovery journal exists. It never runs recovery and never writes.
func ensureNoPendingShipmentRecoveryReadOnly(ws *Workspace) error {
	records, validationErrs, err := inspectShipmentOperationJournalsReadOnly(ws)
	if err != nil {
		return fmt.Errorf("inspect pending shipment recovery journals: %w",
			errors.Join(err, blerrors.ErrShipmentConflict))
	}
	if err := errors.Join(validationErrs...); err != nil {
		return fmt.Errorf("pending shipment recovery journals are invalid: %w",
			errors.Join(err, blerrors.ErrShipmentConflict))
	}
	for _, record := range records {
		if record.kind == shipmentLifecycleJournalKind && record.lifecycle.Phase != shipmentOperationPhaseIntent {
			continue
		}
		return fmt.Errorf("unfinished shipment recovery journal %s blocks preview; run a lifecycle operation to recover: %w",
			filepath.Base(record.path), blerrors.ErrShipmentConflict)
	}
	return nil
}

// readArtifactFromDiskReadOnly parses the canonical Markdown artifact without
// touching SQLite, lock files, or recovery.
func readArtifactFromDiskReadOnly(ws *Workspace, id string) (*models.Artifact, error) {
	return findArtifact(context.Background(), ws, id)
}

func readShipmentFromDiskReadOnly(ws *Workspace, id string) (*models.Artifact, error) {
	shipment, err := readArtifactFromDiskReadOnly(ws, id)
	if err != nil {
		if errors.Is(err, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("get shipment %s: %w", id, blerrors.ErrShipmentNotFound)
		}
		return nil, fmt.Errorf("get shipment %s: %w", id, err)
	}
	if shipment.ArtifactType != "shipment" {
		return nil, fmt.Errorf("get shipment %s: %w", id, blerrors.ErrShipmentNotFound)
	}
	normalizeShipmentArtifact(shipment)
	return shipment, nil
}

// workStartedInCurrentClaimEpoch reports whether the item log carries a
// WORK_STARTED comment after the most recent activation.
func workStartedInCurrentClaimEpoch(ctx context.Context, ws *Workspace, itemID string) (bool, error) {
	logged, err := events.ReadAllEvents(ctx, WorkspaceLogsRoot(ws.RootPath), itemID)
	if err != nil {
		return false, fmt.Errorf("read item %s log: %w", itemID, err)
	}
	epochStart := 0
	for index, event := range logged {
		if event.EventType == "status_changed" && event.Delta["to"] == string(models.StatusActive) {
			epochStart = index + 1
		}
	}
	for _, event := range logged[epochStart:] {
		if event.EventType != "comment" {
			continue
		}
		comment, _ := event.Delta["comment"].(string)
		if strings.HasPrefix(strings.TrimSpace(comment), "WORK_STARTED:") {
			return true, nil
		}
	}
	return false, nil
}

// requireUnstartedClaimedMember verifies an active member is an unstarted
// claim-assigned member of sourceShipmentID.
func requireUnstartedClaimedMember(
	ctx context.Context,
	ws *Workspace,
	member *models.Artifact,
	sourceShipmentID, action string,
) error {
	if member.Status != models.StatusActive {
		return nil
	}
	marker, _ := member.CustomFields[schedulerBaselineClaimKey].(string)
	if marker != sourceShipmentID {
		return fmt.Errorf("%s: active item %s lacks the claim marker of shipment %s: %w",
			action, member.ID, sourceShipmentID, blerrors.ErrShipmentConflict)
	}
	started, err := workStartedInCurrentClaimEpoch(ctx, ws, member.ID)
	if err != nil {
		return err
	}
	if started {
		return fmt.Errorf("%s: item %s already started work in its current claim epoch: %w",
			action, member.ID, blerrors.ErrShipmentConflict)
	}
	return nil
}

// refuseRootCoveringFeatureRemoval refuses detaching a root covering feature
// while remaining manifest members descend from it.
func refuseRootCoveringFeatureRemoval(
	ctx context.Context,
	ws *Workspace,
	item *models.Artifact,
	remaining []string,
	shipmentID, action string,
) error {
	if !isRootCoveringFeature(item) {
		return nil
	}
	for _, memberID := range remaining {
		current, err := findArtifact(ctx, ws, memberID)
		for depth := 0; err == nil && current.ParentID != "" && depth < 32; depth++ {
			if current.ParentID == item.ID {
				return fmt.Errorf("%s: root covering feature %s of shipment %s still has dependent member %s: %w",
					action, item.ID, shipmentID, memberID, blerrors.ErrShipmentConflict)
			}
			current, err = findArtifact(ctx, ws, current.ParentID)
		}
	}
	return nil
}

// findThirdShipmentAssignment returns the ID of a non-terminal shipment other
// than the ignored ones whose manifest holds itemID, read from Markdown only.
func findThirdShipmentAssignment(ws *Workspace, itemID string, ignore ...string) (string, error) {
	refs, err := scanCanonicalArtifacts(ws)
	if err != nil {
		return "", fmt.Errorf("scan shipments for item %s assignment: %w", itemID, err)
	}
	for _, candidates := range refs {
		for _, candidate := range candidates {
			if candidate.artifactType != "shipment" || containsString(ignore, candidate.id) ||
				!shipmentStatusBlocksAssignment(models.ArtifactStatus(candidate.status)) {
				continue
			}
			shipment, _, parseErr := parseFile(candidate.path)
			if parseErr != nil {
				return "", fmt.Errorf("parse shipment %s: %w", candidate.id, parseErr)
			}
			if containsString(NormalizeShipmentItems(shipment), itemID) {
				return candidate.id, nil
			}
		}
	}
	return "", nil
}

// planMemberDetach computes the member artifact after it leaves sourceID. A
// nil destination means plain removal. It reports whether the member changed
// and whether it was requeued from active.
func planMemberDetach(
	member *models.Artifact,
	sourceID string,
	destination *models.Artifact,
) (updated *models.Artifact, changed, requeued bool) {
	updated = cloneArtifact(member)
	if updated.CustomFields == nil {
		updated.CustomFields = map[string]any{}
	}
	marker, _ := member.CustomFields[schedulerBaselineClaimKey].(string)
	switch {
	case member.Status == models.StatusActive && destination != nil && destination.Status == models.StatusActive:
		if marker != destination.ID {
			updated.CustomFields[schedulerBaselineClaimKey] = destination.ID
			changed = true
		}
	case member.Status == models.StatusActive:
		updated.Status = models.StatusQueued
		delete(updated.CustomFields, schedulerBaselineClaimKey)
		changed, requeued = true, true
	case marker == sourceID:
		delete(updated.CustomFields, schedulerBaselineClaimKey)
		changed = true
	}
	if changed {
		updated.UpdatedAt = models.NowUTC()
	}
	return updated, changed, requeued
}

type removalPlan struct {
	shipment     *models.Artifact
	member       *models.Artifact
	updated      *models.Artifact
	memberChange bool
	requeued     bool
	before       []string
	after        []string
}

func planRemoval(
	ctx context.Context,
	ws *Workspace,
	shipment *models.Artifact,
	itemID string,
	opts ShipmentManifestOptions,
) (*removalPlan, error) {
	if err := validateShipmentManifestQueueGuard(shipment, opts, "remove item"); err != nil {
		return nil, err
	}
	before := NormalizeShipmentItems(shipment)
	if !containsString(before, itemID) {
		return nil, fmt.Errorf("item %s is not in shipment %s: %w", itemID, shipment.ID, blerrors.ErrNotFound)
	}
	member, err := findArtifact(ctx, ws, itemID)
	if err != nil {
		return nil, fmt.Errorf("load item %s: %w", itemID, err)
	}
	if err := requireUnstartedClaimedMember(ctx, ws, member, shipment.ID,
		fmt.Sprintf("remove item %s from shipment %s", itemID, shipment.ID)); err != nil {
		return nil, err
	}
	after := removeString(before, itemID)
	if err := refuseRootCoveringFeatureRemoval(ctx, ws, member, after, shipment.ID, "remove item"); err != nil {
		return nil, err
	}
	updated, changed, requeued := planMemberDetach(member, shipment.ID, nil)
	return &removalPlan{
		shipment: shipment, member: member, updated: updated,
		memberChange: changed, requeued: requeued, before: before, after: after,
	}, nil
}

type movePlan struct {
	from, to     *models.Artifact
	member       *models.Artifact
	updated      *models.Artifact
	memberChange bool
	requeued     bool
	fromBefore   []string
	fromAfter    []string
	toBefore     []string
	toAfter      []string
}

func planMove(
	ctx context.Context,
	ws *Workspace,
	from, to *models.Artifact,
	itemID string,
	opts ShipmentManifestOptions,
) (*movePlan, error) {
	if err := validateShipmentManifestQueueGuard(from, opts, "move item"); err != nil {
		return nil, err
	}
	if err := validateShipmentManifestQueueGuard(to, opts, "move item"); err != nil {
		return nil, err
	}
	fromBefore := NormalizeShipmentItems(from)
	if !containsString(fromBefore, itemID) {
		return nil, fmt.Errorf("item %s is not in shipment %s: %w", itemID, from.ID, blerrors.ErrNotFound)
	}
	member, err := findArtifact(ctx, ws, itemID)
	if err != nil {
		return nil, fmt.Errorf("load item %s: %w", itemID, err)
	}
	if member.ArtifactType == "shipment" {
		return nil, fmt.Errorf("artifact %s is a shipment and cannot be moved: %w", itemID, blerrors.ErrValidation)
	}
	third, err := findThirdShipmentAssignment(ws, itemID, from.ID, to.ID)
	if err != nil {
		return nil, err
	}
	if third != "" {
		return nil, fmt.Errorf("item %s is also assigned to shipment %s: %w",
			itemID, third, blerrors.ErrShipmentConflict)
	}
	if err := requireUnstartedClaimedMember(ctx, ws, member, from.ID,
		fmt.Sprintf("move item %s from shipment %s", itemID, from.ID)); err != nil {
		return nil, err
	}
	fromAfter := removeString(fromBefore, itemID)
	if err := refuseRootCoveringFeatureRemoval(ctx, ws, member, fromAfter, from.ID, "move item"); err != nil {
		return nil, err
	}
	toBefore := NormalizeShipmentItems(to)
	toAfter := append(append(make([]string, 0, len(toBefore)+1), toBefore...), itemID)
	if containsString(toBefore, itemID) {
		toAfter = toBefore
	}
	updated, changed, requeued := planMemberDetach(member, from.ID, to)
	return &movePlan{
		from: from, to: to, member: member, updated: updated, memberChange: changed, requeued: requeued,
		fromBefore: fromBefore, fromAfter: fromAfter, toBefore: toBefore, toAfter: toAfter,
	}, nil
}

// RemoveFromShipment removes itemID from a shipment manifest. Queued shipments
// need no override; an active shipment requires Force and Reason, and a blocked
// shipment is always refused. An eligible unstarted claim-assigned active
// member is requeued. The mutation runs under a durable remove_item journal.
func RemoveFromShipment(
	ctx context.Context,
	ws *Workspace,
	shipmentID, itemID string,
	opts ShipmentManifestOptions,
) (*RemoveFromShipmentResult, error) {
	if ws == nil {
		return nil, fmt.Errorf("remove from shipment workspace is nil: %w", blerrors.ErrValidation)
	}
	if strings.TrimSpace(shipmentID) == "" || strings.TrimSpace(itemID) == "" {
		return nil, fmt.Errorf("shipment id and item id are required: %w", blerrors.ErrValidation)
	}
	if err := validateShipmentManifestOptions(opts); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(opts.Reason)
	result := &RemoveFromShipmentResult{
		ShipmentID: shipmentID, ItemID: itemID, DryRun: opts.DryRun, Force: opts.Force, Reason: reason,
	}

	if opts.DryRun {
		if err := ensureNoPendingShipmentRecoveryReadOnly(ws); err != nil {
			return nil, err
		}
		shipment, err := readShipmentFromDiskReadOnly(ws, shipmentID)
		if err != nil {
			return nil, err
		}
		plan, err := planRemoval(ctx, ws, shipment, itemID, opts)
		if err != nil {
			return nil, err
		}
		result.Status, result.Requeued = "dry_run", plan.requeued
		result.ItemsBefore, result.ItemsAfter = plan.before, plan.after
		return result, nil
	}

	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("remove item %s from shipment %s: lock shipment lifecycle: %w", itemID, shipmentID, err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after remove", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	membershipUnlock, err := lockShipmentMembership(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s membership: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := membershipUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment membership lock", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()

	if _, err := GetShipment(ctx, ws, shipmentID); err != nil {
		return nil, err
	}
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, []string{shipmentID, itemID})
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s removal aggregate: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment removal locks", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	shipment, err := readShipmentFromDiskReadOnly(ws, shipmentID)
	if err != nil {
		return nil, err
	}
	plan, err := planRemoval(ctx, ws, shipment, itemID, opts)
	if err != nil {
		return nil, err
	}

	memberPreimages, err := manifestMemberPreimages(ctx, ws, plan.before)
	if err != nil {
		return nil, err
	}
	correlationID, err := newShipmentManifestCorrelationID()
	if err != nil {
		return nil, err
	}
	updatedShipment := cloneArtifact(shipment)
	updatedShipment.CustomFields["items"] = plan.after
	updatedShipment.UpdatedAt = models.NowUTC()

	mutation := &manifestMutation{
		operation:     shipmentOperationRemoveItem,
		eventType:     EventShipmentItemRemoved,
		correlationID: correlationID,
		reason:        reason,
		journal: shipmentLifecycleJournal{
			SchemaVersion:  "shipment-operation/v1",
			CorrelationID:  correlationID,
			Phase:          shipmentOperationPhaseIntent,
			Operation:      shipmentOperationRemoveItem,
			RecoveryPolicy: shipmentOperationRecoveryPolicyRollback,
			ShipmentID:     shipmentID,
			Target:         itemID,
			Reason:         reason,
			Preimage: shipmentLifecyclePreimage{
				Shipment: cloneArtifact(shipment),
				Members:  memberPreimages,
			},
		},
		shipments:         []*models.Artifact{updatedShipment},
		shipmentPreimages: []*models.Artifact{cloneArtifact(shipment)},
		member:            plan.updated,
		memberPreimage:    cloneArtifact(plan.member),
		memberChanged:     plan.memberChange,
		memberRequeued:    plan.requeued,
		deltas: []map[string]any{{
			"item_id":      itemID,
			"shipment_id":  shipmentID,
			"items_before": plan.before,
			"items_after":  plan.after,
		}},
		force: opts.Force,
	}
	if err := mutation.run(ctx, ws); err != nil {
		return nil, err
	}
	result.Status, result.Requeued, result.CorrelationID = "removed", plan.requeued, correlationID
	result.ItemsBefore, result.ItemsAfter = plan.before, plan.after
	return result, nil
}

// MoveShipmentItem atomically moves itemID between two shipment manifests under
// a durable move_item journal. An eligible unstarted claim-assigned active
// member is requeued when the target is queued, or re-stamped for the target
// when it is active.
func MoveShipmentItem(
	ctx context.Context,
	ws *Workspace,
	itemID, fromShipmentID, toShipmentID string,
	opts ShipmentManifestOptions,
) (*MoveShipmentItemResult, error) {
	if ws == nil {
		return nil, fmt.Errorf("move shipment item workspace is nil: %w", blerrors.ErrValidation)
	}
	if strings.TrimSpace(itemID) == "" || strings.TrimSpace(fromShipmentID) == "" ||
		strings.TrimSpace(toShipmentID) == "" {
		return nil, fmt.Errorf("item id, source shipment id, and target shipment id are required: %w",
			blerrors.ErrValidation)
	}
	if fromShipmentID == toShipmentID {
		return nil, fmt.Errorf("source and target shipment must differ: %w", blerrors.ErrValidation)
	}
	if err := validateShipmentManifestOptions(opts); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(opts.Reason)
	result := &MoveShipmentItemResult{
		ItemID: itemID, FromShipmentID: fromShipmentID, ToShipmentID: toShipmentID,
		DryRun: opts.DryRun, Force: opts.Force, Reason: reason,
	}
	fill := func(plan *movePlan) {
		result.Requeued = plan.requeued
		result.FromBefore, result.FromAfter = plan.fromBefore, plan.fromAfter
		result.ToBefore, result.ToAfter = plan.toBefore, plan.toAfter
	}

	if opts.DryRun {
		if err := ensureNoPendingShipmentRecoveryReadOnly(ws); err != nil {
			return nil, err
		}
		from, err := readShipmentFromDiskReadOnly(ws, fromShipmentID)
		if err != nil {
			return nil, err
		}
		to, err := readShipmentFromDiskReadOnly(ws, toShipmentID)
		if err != nil {
			return nil, err
		}
		plan, err := planMove(ctx, ws, from, to, itemID, opts)
		if err != nil {
			return nil, err
		}
		result.Status = "dry_run"
		fill(plan)
		return result, nil
	}

	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("move item %s: lock shipment lifecycle: %w", itemID, err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after move", "item_id", itemID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	lockOrder := []string{fromShipmentID, toShipmentID}
	sort.Strings(lockOrder)
	for _, shipmentID := range lockOrder {
		unlock, lockErr := lockShipmentMembership(ctx, ws, shipmentID)
		if lockErr != nil {
			return nil, fmt.Errorf("lock shipment %s membership: %w", shipmentID, lockErr)
		}
		defer func(id string) {
			if unlockErr := unlock(); unlockErr != nil {
				slog.WarnContext(ctx, "release shipment membership lock", "shipment_id", id, "error", unlockErr)
			}
		}(shipmentID)
	}

	for _, shipmentID := range []string{fromShipmentID, toShipmentID} {
		if _, err := GetShipment(ctx, ws, shipmentID); err != nil {
			return nil, err
		}
	}
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, []string{fromShipmentID, toShipmentID, itemID})
	if err != nil {
		return nil, fmt.Errorf("lock item %s move aggregate: %w", itemID, err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment move locks", "item_id", itemID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	from, err := readShipmentFromDiskReadOnly(ws, fromShipmentID)
	if err != nil {
		return nil, err
	}
	to, err := readShipmentFromDiskReadOnly(ws, toShipmentID)
	if err != nil {
		return nil, err
	}
	plan, err := planMove(ctx, ws, from, to, itemID, opts)
	if err != nil {
		return nil, err
	}
	memberPreimages, err := manifestMemberPreimages(ctx, ws, plan.fromBefore)
	if err != nil {
		return nil, err
	}
	correlationID, err := newShipmentManifestCorrelationID()
	if err != nil {
		return nil, err
	}
	updatedFrom, updatedTo := cloneArtifact(from), cloneArtifact(to)
	updatedFrom.CustomFields["items"] = plan.fromAfter
	updatedTo.CustomFields["items"] = plan.toAfter
	now := models.NowUTC()
	updatedFrom.UpdatedAt, updatedTo.UpdatedAt = now, now

	common := map[string]any{
		"item_id":          itemID,
		"from_shipment_id": fromShipmentID,
		"to_shipment_id":   toShipmentID,
	}
	fromDelta, toDelta := map[string]any{}, map[string]any{}
	for key, value := range common {
		fromDelta[key], toDelta[key] = value, value
	}
	fromDelta["shipment_id"], toDelta["shipment_id"] = fromShipmentID, toShipmentID
	fromDelta["items_before"], fromDelta["items_after"] = plan.fromBefore, plan.fromAfter
	toDelta["items_before"], toDelta["items_after"] = plan.toBefore, plan.toAfter

	mutation := &manifestMutation{
		operation:     shipmentOperationMoveItem,
		eventType:     EventShipmentItemMoved,
		correlationID: correlationID,
		reason:        reason,
		journal: shipmentLifecycleJournal{
			SchemaVersion:  "shipment-operation/v1",
			CorrelationID:  correlationID,
			Phase:          shipmentOperationPhaseIntent,
			Operation:      shipmentOperationMoveItem,
			RecoveryPolicy: shipmentOperationRecoveryPolicyRollback,
			ShipmentID:     fromShipmentID,
			Target:         toShipmentID,
			Reason:         reason,
			Preimage: shipmentLifecyclePreimage{
				Shipment: cloneArtifact(from),
				Members:  memberPreimages,
				Related:  []*models.Artifact{cloneArtifact(to)},
			},
		},
		shipments:         []*models.Artifact{updatedFrom, updatedTo},
		shipmentPreimages: []*models.Artifact{cloneArtifact(from), cloneArtifact(to)},
		member:            plan.updated,
		memberPreimage:    cloneArtifact(plan.member),
		memberChanged:     plan.memberChange,
		memberRequeued:    plan.requeued,
		deltas:            []map[string]any{fromDelta, toDelta},
		force:             opts.Force,
	}
	if err := mutation.run(ctx, ws); err != nil {
		return nil, err
	}
	result.Status, result.CorrelationID = "moved", correlationID
	fill(plan)
	return result, nil
}

func newShipmentManifestCorrelationID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate shipment manifest correlation id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func manifestMemberPreimages(ctx context.Context, ws *Workspace, memberIDs []string) ([]*models.Artifact, error) {
	preimages := make([]*models.Artifact, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		member, err := findArtifact(ctx, ws, memberID)
		if err != nil {
			return nil, fmt.Errorf("load shipment member %s preimage: %w", memberID, err)
		}
		preimages = append(preimages, cloneArtifact(member))
	}
	return preimages, nil
}

// manifestMutation is the durable remove_item / move_item write protocol:
// intent journal -> member write -> manifest writes -> committed events ->
// committed journal -> journal cleanup.
type manifestMutation struct {
	operation         string
	eventType         string
	correlationID     string
	reason            string
	force             bool
	journal           shipmentLifecycleJournal
	shipments         []*models.Artifact
	shipmentPreimages []*models.Artifact
	member            *models.Artifact
	memberPreimage    *models.Artifact
	memberChanged     bool
	memberRequeued    bool
	deltas            []map[string]any
}

func (m *manifestMutation) artifactIDs() []string {
	ids := make([]string, 0, len(m.shipments)+1)
	for _, shipment := range m.shipments {
		ids = append(ids, shipment.ID)
	}
	return append(ids, m.memberPreimage.ID)
}

func (m *manifestMutation) run(ctx context.Context, ws *Workspace) error {
	journalName := shipmentLifecycleJournalName(m.correlationID)
	journalPath, err := writeShipmentOperationJournal(ws, journalName, m.journal)
	if err != nil {
		return fmt.Errorf("persist %s intent: %w", m.operation, err)
	}
	operationCtx := withShipmentOperation(ctx, m.correlationID)

	fail := func(cause error, aborted bool) error {
		if blerrors.IsWriteIndeterminate(cause) {
			// Outcome unknown: skip compensation and keep the intent journal so
			// the lifecycle barrier decides between rollback and finalize.
			return fmt.Errorf("%s %s: %w", m.operation, m.journal.ShipmentID, cause)
		}
		return m.rollback(ctx, ws, journalPath, cause, aborted)
	}

	if m.memberChanged {
		previous := m.memberPreimage.Status
		relocate := shouldRelocateOnStatusChange(previous, m.member.Status)
		if err := persistArtifact(operationCtx, ws, cloneArtifact(m.member), relocate); err != nil {
			return fail(fmt.Errorf("persist item %s: %w", m.member.ID, err), false)
		}
		if previous != m.member.Status {
			reason := m.reason
			if reason == "" {
				reason = "shipment manifest replan"
			}
			if err := ws.appendShipmentEvent(operationCtx, m.member.ID, "status_changed", map[string]any{
				"from":           string(previous),
				"to":             string(m.member.Status),
				"reason":         reason,
				"correlation_id": m.correlationID,
				"operation":      m.operation,
			}); err != nil {
				return fail(fmt.Errorf("append item %s status event: %w", m.member.ID, err), false)
			}
		}
	}
	for _, shipment := range m.shipments {
		if err := persistArtifact(operationCtx, ws, cloneArtifact(shipment), false); err != nil {
			return fail(fmt.Errorf("persist shipment %s manifest: %w", shipment.ID, err), false)
		}
	}
	for index, shipment := range m.shipments {
		delta := m.deltas[index]
		delta["correlation_id"] = m.correlationID
		delta["operation"] = m.operation
		delta["phase"] = shipmentOperationPhaseCommitted
		if m.reason != "" {
			delta["reason"] = m.reason
		}
		if m.force {
			delta["force"] = true
		}
		if m.memberRequeued {
			delta["requeued"] = true
		}
		if err := ws.appendShipmentEvent(operationCtx, shipment.ID, m.eventType, delta); err != nil {
			return fail(fmt.Errorf("append %s event for shipment %s: %w", m.eventType, shipment.ID, err), index > 0)
		}
	}

	committed := m.journal
	committed.Phase = shipmentOperationPhaseCommitted
	if _, err := writeShipmentOperationJournal(ws, journalName, committed); err != nil {
		// Every write and audit event landed; the barrier finalizes from the
		// committed events, so never roll back here.
		return fmt.Errorf("%s %s: persist committed journal: %w",
			m.operation, m.journal.ShipmentID, errors.Join(err, blerrors.ErrWriteIndeterminate))
	}
	cleanupShipmentOperationJournal(ctx, ws, journalPath)
	return nil
}

// rollback restores every preimage, scrubs this operation's tagged events, and
// removes the journal. When a committed event was already appended it records
// a compensating aborted event on the first shipment.
func (m *manifestMutation) rollback(
	ctx context.Context,
	ws *Workspace,
	journalPath string,
	cause error,
	aborted bool,
) error {
	restoreCtx := context.WithoutCancel(ctx)
	operationCtx := withShipmentOperation(restoreCtx, m.correlationID)
	governedCtx := context.WithValue(operationCtx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
		correlationID:                 m.correlationID,
		operation:                     m.operation + "_rollback",
		allowGovernedShipmentMutation: true,
	})
	var errs []error
	if m.memberChanged {
		if err := persistArtifact(operationCtx, ws, cloneArtifact(m.memberPreimage), true); err != nil {
			errs = append(errs, fmt.Errorf("restore item %s preimage: %w", m.memberPreimage.ID, err))
		}
	}
	for index := len(m.shipmentPreimages) - 1; index >= 0; index-- {
		if err := persistArtifact(governedCtx, ws, cloneArtifact(m.shipmentPreimages[index]), false); err != nil {
			errs = append(errs, fmt.Errorf("restore shipment %s preimage: %w", m.shipmentPreimages[index].ID, err))
		}
	}
	if err := removeShipmentOperationEvents(operationCtx, ws, m.artifactIDs(), m.correlationID); err != nil {
		errs = append(errs, fmt.Errorf("scrub %s events: %w", m.operation, err))
	}
	if aborted && len(errs) == 0 {
		if err := ws.appendShipmentEvent(restoreCtx, m.shipments[0].ID, m.eventType, map[string]any{
			"correlation_id":   m.correlationID,
			"operation":        m.operation,
			"phase":            "aborted",
			"item_id":          m.memberPreimage.ID,
			"from_shipment_id": m.shipments[0].ID,
			"to_shipment_id":   m.shipments[len(m.shipments)-1].ID,
		}); err != nil {
			errs = append(errs, fmt.Errorf("append aborted event: %w", err))
		}
	}
	if len(errs) > 0 {
		// Compensation did not finish; keep the intent journal for the barrier.
		return fmt.Errorf("%s %s rolled back with errors (cause: %v): %w",
			m.operation, m.journal.ShipmentID, cause,
			errors.Join(append(errs, blerrors.ErrWriteIndeterminate)...))
	}
	cleanupShipmentOperationJournal(restoreCtx, ws, journalPath)
	return fmt.Errorf("%s %s: %w", m.operation, m.journal.ShipmentID, cause)
}

// writeShipmentOperationJournal durably writes a lifecycle journal record.
func writeShipmentOperationJournal(
	ws *Workspace,
	name string,
	journal shipmentLifecycleJournal,
) (string, error) {
	return writeShipmentLifecycleJournalForWorkspace(ws, name, journal)
}

// cleanupShipmentOperationJournal removes a finished lifecycle journal. A
// failure is logged only: the committed phase makes a leftover journal safe.
func cleanupShipmentOperationJournal(ctx context.Context, ws *Workspace, journalPath string) {
	removeShipmentOperationJournal(ctx, ws, journalPath)
}

func shipmentLifecycleJournalName(correlationID string) string {
	return "shipment-operation-" + strings.ToLower(correlationID) + ".json"
}
