package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// snapshotShipmentReconcileImpl captures the pre-mutation archive bytes and
// full items-table row for 167.016-T / #423.
func snapshotShipmentReconcileImpl(ctx context.Context, ws *Workspace, shipmentID string) (shipmentReconcileSnapshot, error) {
	if err := validateShipmentReconcileSnapshotWorkspace(ws, shipmentID); err != nil {
		return shipmentReconcileSnapshot{}, err
	}

	fileBytes, err := snapshotShipmentReconcileArchiveFile(ws, shipmentID)
	if err != nil {
		return shipmentReconcileSnapshot{}, err
	}
	rowPresent, row, err := snapshotShipmentReconcileRow(ctx, ws, shipmentID)
	if err != nil {
		return shipmentReconcileSnapshot{}, err
	}

	return shipmentReconcileSnapshot{
		ShipmentID: shipmentID,
		FileBytes:  fileBytes,
		RowPresent: rowPresent,
		Row:        row,
	}, nil
}

// restoreShipmentReconcileImpl restores a snapshot using the production
// handle-relative archive writer from 167.006-T plus an exact items-row write.
func restoreShipmentReconcileImpl(ctx context.Context, ws *Workspace, snapshot shipmentReconcileSnapshot) error {
	return restoreShipmentReconcileWithSeams(ctx, ws, snapshot, defaultShipmentReconcileFSSeams())
}

func restoreShipmentReconcileWithSeams(ctx context.Context, ws *Workspace, snapshot shipmentReconcileSnapshot, seams shipmentReconcileFSSeams) error {
	if err := validateShipmentReconcileRestoreSnapshot(ws, snapshot); err != nil {
		return err
	}

	fileErr := restoreShipmentReconcileArchiveFile(ctx, ws, snapshot, seams)
	if fileErr != nil && !blerrors.IsWriteIndeterminate(fileErr) {
		return fileErr
	}

	rowErr := restoreShipmentReconcileRow(ctx, ws, snapshot)
	if fileErr != nil && rowErr != nil {
		return errors.Join(fileErr, rowErr)
	}
	if rowErr != nil {
		return rowErr
	}
	if fileErr != nil {
		return fileErr
	}
	return nil
}

func validateShipmentReconcileSnapshotWorkspace(ws *Workspace, shipmentID string) error {
	if ws == nil {
		return fmt.Errorf("shipment reconcile snapshot: workspace is required: %w", blerrors.ErrValidation)
	}
	if ws.DB == nil {
		return fmt.Errorf("shipment reconcile snapshot %q: workspace database is required: %w", shipmentID, blerrors.ErrValidation)
	}
	if ws.StorageRoot == "" && ws.RootPath == "" {
		return fmt.Errorf("shipment reconcile snapshot %q: workspace root is required: %w", shipmentID, blerrors.ErrValidation)
	}
	if shipmentID == "" {
		return fmt.Errorf("shipment reconcile snapshot: shipment id is required: %w", blerrors.ErrValidation)
	}
	if filepath.Base(shipmentID) != shipmentID || shipmentID == "." || shipmentID == ".." {
		return fmt.Errorf("%w: shipment id %q is not a safe filename component", blerrors.ErrValidation, shipmentID)
	}
	return nil
}

func validateShipmentReconcileRestoreSnapshot(ws *Workspace, snapshot shipmentReconcileSnapshot) error {
	if err := validateShipmentReconcileSnapshotWorkspace(ws, snapshot.ShipmentID); err != nil {
		return err
	}
	if snapshot.RowPresent {
		if len(snapshot.Row) == 0 {
			return fmt.Errorf("restore shipment reconcile %s: snapshot row is required when RowPresent is true: %w", snapshot.ShipmentID, blerrors.ErrValidation)
		}
		rowID, ok := shipmentReconcileSnapshotRowID(snapshot.Row)
		if !ok {
			return fmt.Errorf("restore shipment reconcile %s: snapshot row id is required: %w", snapshot.ShipmentID, blerrors.ErrValidation)
		}
		if rowID != snapshot.ShipmentID {
			return fmt.Errorf("restore shipment reconcile %s: snapshot row id %q does not match shipment id: %w", snapshot.ShipmentID, rowID, blerrors.ErrValidation)
		}
		return nil
	}
	if snapshot.Row != nil {
		return fmt.Errorf("restore shipment reconcile %s: absent-row snapshot must not carry row values: %w", snapshot.ShipmentID, blerrors.ErrValidation)
	}
	return nil
}

func snapshotShipmentReconcileArchiveFile(ws *Workspace, shipmentID string) ([]byte, error) {
	archiveDir := filepath.Join(workspaceStorageRoot(ws), shipmentReconcileArchiveDirName)
	if info, err := os.Lstat(archiveDir); err == nil {
		symlink, linkErr := IsSymlinkOrReparsePoint(info, archiveDir)
		if linkErr != nil {
			return nil, fmt.Errorf("snapshot shipment reconcile %s archive directory: %w", shipmentID, linkErr)
		}
		if symlink {
			return nil, fmt.Errorf("snapshot shipment reconcile %s archive directory: %w", shipmentID, blerrors.ErrValidation)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("snapshot shipment reconcile %s archive directory: %w", shipmentID, err)
	}

	// readShipmentReconcileArchiveSnapshotFile (platform-specific) opens the
	// archive file exactly once and reads from that single handle, rather
	// than the prior Lstat-then-ReadFile pair: two separate pathname
	// operations left a check/use race window in which the file could be
	// replaced by a symlink between the symlink check and the read, and this
	// snapshot is later WRITTEN BACK verbatim by restoreShipmentReconcile, so
	// a race-won read here would persist outside-workspace bytes as the
	// source of truth for rollback (167.016-T hardening).
	content, err := readShipmentReconcileArchiveSnapshotFile(archiveDir, shipmentID+".md")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("snapshot shipment reconcile %s archive file: %w", shipmentID, err)
	}
	if content == nil {
		return []byte{}, nil
	}
	return append([]byte(nil), content...), nil
}

func snapshotShipmentReconcileRow(ctx context.Context, ws *Workspace, shipmentID string) (bool, map[string]any, error) {
	rows, err := ws.DB.QueryContext(ctx, `SELECT * FROM items WHERE id = ?`, shipmentID)
	if err != nil {
		return false, nil, fmt.Errorf("snapshot shipment reconcile %s row: query items row: %w", shipmentID, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return false, nil, fmt.Errorf("snapshot shipment reconcile %s row: list columns: %w", shipmentID, err)
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return false, nil, fmt.Errorf("snapshot shipment reconcile %s row: list column types: %w", shipmentID, err)
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, nil, fmt.Errorf("snapshot shipment reconcile %s row: iterate rows: %w", shipmentID, err)
		}
		return false, nil, nil
	}

	scanValues := make([]shipmentReconcileRowScanValue, len(columnTypes))
	destinations := make([]any, len(columnTypes))
	for i, columnType := range columnTypes {
		scanValues[i] = newShipmentReconcileRowScanValue(columnType.DatabaseTypeName())
		destinations[i] = scanValues[i].dest
	}
	if err := rows.Scan(destinations...); err != nil {
		return false, nil, fmt.Errorf("snapshot shipment reconcile %s row: scan items row: %w", shipmentID, err)
	}

	row := make(map[string]any, len(columns))
	for i, column := range columns {
		row[column] = scanValues[i].value()
	}
	if rows.Next() {
		return false, nil, fmt.Errorf("snapshot shipment reconcile %s row: multiple rows returned for a primary-key lookup", shipmentID)
	}
	if err := rows.Err(); err != nil {
		return false, nil, fmt.Errorf("snapshot shipment reconcile %s row: finalize iteration: %w", shipmentID, err)
	}
	return true, row, nil
}

type shipmentReconcileRowScanValue struct {
	dest  any
	value func() any
}

func newShipmentReconcileRowScanValue(databaseType string) shipmentReconcileRowScanValue {
	normalized := strings.ToUpper(databaseType)
	switch {
	case strings.Contains(normalized, "INT"):
		value := &sql.NullInt64{}
		return shipmentReconcileRowScanValue{
			dest: value,
			value: func() any {
				if !value.Valid {
					return nil
				}
				return value.Int64
			},
		}
	case strings.Contains(normalized, "REAL"), strings.Contains(normalized, "FLOA"), strings.Contains(normalized, "DOUB"), strings.Contains(normalized, "NUMERIC"), strings.Contains(normalized, "DECIMAL"):
		value := &sql.NullFloat64{}
		return shipmentReconcileRowScanValue{
			dest: value,
			value: func() any {
				if !value.Valid {
					return nil
				}
				return value.Float64
			},
		}
	case strings.Contains(normalized, "BLOB"):
		var value []byte
		return shipmentReconcileRowScanValue{
			dest: &value,
			value: func() any {
				if value == nil {
					return nil
				}
				return append([]byte(nil), value...)
			},
		}
	default:
		value := &sql.NullString{}
		return shipmentReconcileRowScanValue{
			dest: value,
			value: func() any {
				if !value.Valid {
					return nil
				}
				return value.String
			},
		}
	}
}

func restoreShipmentReconcileArchiveFile(ctx context.Context, ws *Workspace, snapshot shipmentReconcileSnapshot, seams shipmentReconcileFSSeams) error {
	// FileBytes == nil encodes "no archive file existed at snapshot time".
	// A distinct, handle-bound delete primitive does not exist in 167.016-T,
	// so rollback leaves file absence restoration to a later owner task rather
	// than re-opening the path-based race this transaction is designed to avoid.
	if snapshot.FileBytes == nil {
		return nil
	}
	if err := writeShipmentReconcileArchiveFileWithSeams(ctx, ws, snapshot.ShipmentID, snapshot.FileBytes, seams); err != nil {
		return fmt.Errorf("restore shipment reconcile %s archive file: %w", snapshot.ShipmentID, err)
	}
	return nil
}

func restoreShipmentReconcileRow(ctx context.Context, ws *Workspace, snapshot shipmentReconcileSnapshot) error {
	if snapshot.RowPresent {
		return restoreShipmentReconcilePresentRow(ctx, ws, snapshot)
	}
	return restoreShipmentReconcileAbsentRow(ctx, ws, snapshot.ShipmentID)
}

func restoreShipmentReconcilePresentRow(ctx context.Context, ws *Workspace, snapshot shipmentReconcileSnapshot) error {
	columns := make([]string, 0, len(snapshot.Row))
	for column := range snapshot.Row {
		columns = append(columns, column)
	}
	sort.Strings(columns)

	quotedColumns := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	args := make([]any, len(columns))
	for i, column := range columns {
		quotedColumns[i] = quoteShipmentReconcileIdentifier(column)
		placeholders[i] = "?"
		args[i] = cloneShipmentReconcileSnapshotValue(snapshot.Row[column])
	}
	stmt := `INSERT OR REPLACE INTO items (` + strings.Join(quotedColumns, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)`

	err := bldb.RetryWrite(ctx, func() error {
		if _, execErr := ws.DB.ExecContext(ctx, stmt, args...); execErr != nil {
			return fmt.Errorf("%w: exact-row restore for shipment %s: %w", blerrors.ErrWriteIndeterminate, snapshot.ShipmentID, execErr)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("restore shipment reconcile %s row: %w", snapshot.ShipmentID, err)
	}
	return nil
}

func restoreShipmentReconcileAbsentRow(ctx context.Context, ws *Workspace, shipmentID string) error {
	err := bldb.RetryWrite(ctx, func() error {
		if _, execErr := ws.DB.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, shipmentID); execErr != nil {
			return fmt.Errorf("%w: single-row delete for shipment %s: %w", blerrors.ErrWriteIndeterminate, shipmentID, execErr)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("restore shipment reconcile %s row absence: %w", shipmentID, err)
	}
	return nil
}

func shipmentReconcileSnapshotRowID(row map[string]any) (string, bool) {
	value, ok := row["id"]
	if !ok || value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, v != ""
	case []byte:
		return string(v), len(v) > 0
	default:
		return "", false
	}
}

func quoteShipmentReconcileIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func cloneShipmentReconcileSnapshotValue(value any) any {
	if bytes, ok := value.([]byte); ok {
		return append([]byte(nil), bytes...)
	}
	return value
}
