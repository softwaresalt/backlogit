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
)

type shipmentOperationJournalRecord struct {
	name          string
	path          string
	kind          shipmentOperationJournalKind
	lifecycle     shipmentLifecycleJournal
	returnBlocked returnBlockedJournal
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

func loadShipmentOperationJournals(ws *Workspace) ([]shipmentOperationJournalRecord, error) {
	opsRoot, dir, err := shipmentOpsRootForWorkspace(ws, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("enumerate shipment operations directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	records := make([]shipmentOperationJournalRecord, 0, len(entries))
	var validationErrs []error
	for _, entry := range entries {
		name := entry.Name()
		kind, captures, nameErr := validateShipmentOperationJournalName(name)
		if nameErr != nil {
			validationErrs = append(validationErrs, nameErr)
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			validationErrs = append(validationErrs,
				fmt.Errorf("inspect shipment operation journal %s: %w", name, infoErr))
			continue
		}
		redirected, redirectErr := IsSymlinkOrReparsePoint(info, filepath.Join(opsRoot, name))
		if redirectErr != nil {
			validationErrs = append(validationErrs,
				fmt.Errorf("inspect shipment operation journal %s redirect: %w", name, redirectErr))
			continue
		}
		if redirected || !info.Mode().IsRegular() {
			validationErrs = append(validationErrs,
				fmt.Errorf("shipment operation journal %s is not a regular unredirected file: %w",
					name, blerrors.ErrValidation))
			continue
		}

		data, readErr := readShipmentOperationJournalFile(dir, opsRoot, name)
		if readErr != nil {
			validationErrs = append(validationErrs,
				fmt.Errorf("read shipment operation journal %s: %w", name, readErr))
			continue
		}
		record := shipmentOperationJournalRecord{
			name: name,
			path: filepath.Join(opsRoot, name),
			kind: kind,
		}
		switch kind {
		case shipmentLifecycleJournalKind:
			if decodeErr := decodeShipmentOperationJournal(data, &record.lifecycle); decodeErr != nil {
				validationErrs = append(validationErrs,
					fmt.Errorf("parse shipment lifecycle journal %s: %w", name, decodeErr))
				continue
			}
			if record.lifecycle.CorrelationID != captures[0] {
				validationErrs = append(validationErrs,
					fmt.Errorf("shipment lifecycle journal %s correlation id does not match its filename: %w",
						name, blerrors.ErrValidation))
				continue
			}
		case returnBlockedJournalKind:
			if decodeErr := decodeShipmentOperationJournal(data, &record.returnBlocked); decodeErr != nil {
				validationErrs = append(validationErrs,
					fmt.Errorf("parse return-blocked journal %s: %w", name, decodeErr))
				continue
			}
			if record.returnBlocked.Shipment == nil || record.returnBlocked.Item == nil ||
				record.returnBlocked.Shipment.ID != captures[0] ||
				record.returnBlocked.Item.ID != captures[1] {
				validationErrs = append(validationErrs,
					fmt.Errorf("return-blocked journal %s does not match its filename: %w",
						name, blerrors.ErrValidation))
				continue
			}
		}
		records = append(records, record)
	}
	if err := errors.Join(validationErrs...); err != nil {
		return nil, err
	}
	return records, nil
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
