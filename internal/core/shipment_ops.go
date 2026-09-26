package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
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
	case "block", "unblock", "normalize", "claim":
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
	case "claim":
		if journal.RecoveryPolicy != "rollback" || journal.Target != "active" {
			return fmt.Errorf("claim lifecycle tuple must be rollback/active: %w", blerrors.ErrValidation)
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

func shipmentLifecycleJournalName(correlationID string) string {
	return "shipment-operation-" + strings.ToLower(correlationID) + ".json"
}
