package core

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const (
	idSequenceLedgerFileName   = "id-sequences.json"
	idAllocatorLockFileName    = "id_allocator.lock"
	idAllocatorLockBoundedWait = 30 * time.Second
)

type idAllocatorLockContextKey struct{}

type idAllocatorLockState struct {
	dirAbs    string
	locksRoot *os.Root
}

// shipmentOperationBeforeCreateContextKey carries an optional callback
// func(context.Context, string) error invoked inside CreateArtifact after
// artifactID is allocated under id_allocator.lock and before MutationEnvelope
// publishes the artifact file or id-sequences.json reservation.
type shipmentOperationBeforeCreateContextKey struct{}

type branchIDCacheEntry struct {
	fingerprint string
	ids         []string
}

var (
	idSequenceTempSeq atomic.Uint64
	branchIDScanCache sync.Map // key: realRoot -> branchIDCacheEntry (bounded 1 entry per root)
)

type idSequenceLedger struct {
	SchemaVersion int                 `json:"schema_version"`
	ReservedIDs   []string            `json:"reserved_ids"`
	Reservations  map[string][]string `json:"reservations,omitempty"`
}

// writeIDSequenceReservationFn is an injectable seam for deterministic
// ledger-write failure and parent-swap testing.
var writeIDSequenceReservationFn = writeIDSequenceReservation

// historicalIDQueries lists the SQLite sources whose IDs must be reserved so an
// allocator never reuses an ID that was retired (deleted, archived, or
// adopted away) but is still recorded in history. Each query yields one
// nullable TEXT column.
var historicalIDQueries = []string{
	`SELECT item_id FROM item_logs`,
	`SELECT DISTINCT item_id FROM item_log_entries`,
	`SELECT DISTINCT json_extract(delta_json, '$.old_id') FROM item_log_entries WHERE delta_json LIKE '%old_id%' AND json_valid(delta_json)`,
	`SELECT DISTINCT item_id FROM stash_links`,
}

// historicalIDsFromDB returns IDs recorded in SQLite history tables.
func historicalIDsFromDB(ctx context.Context, db *sql.DB) (ids []string, err error) {
	if db == nil {
		return nil, nil
	}
	for _, query := range historicalIDQueries {
		collected, qErr := collectNullableStrings(ctx, db, query)
		if qErr != nil {
			return nil, qErr
		}
		ids = append(ids, collected...)
	}
	return ids, nil
}

func collectNullableStrings(ctx context.Context, db *sql.DB, query string) (out []string, err error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		// Minimal databases (for example hand-built test indexes) may lack the
		// history tables; absence of a table means there is no history to reserve.
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("query historical ids: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close historical id rows: %w", cerr))
		}
	}()
	for rows.Next() {
		var value sql.NullString
		if scanErr := rows.Scan(&value); scanErr != nil {
			return nil, fmt.Errorf("scan historical id: %w", scanErr)
		}
		if value.Valid && value.String != "" {
			out = append(out, value.String)
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate historical ids: %w", rowsErr)
	}
	return out, nil
}

// historicalIDsFromDisk returns every artifact ID visible in the workspace
// files: queue/archive Markdown file names and frontmatter ids, log file
// names, event item_id and delta.old_id values, the stash archive, and the
// provenance corrections file.
func historicalIDsFromDisk(ws *Workspace) ([]string, error) {
	return historicalIDsFromDiskContext(context.Background(), ws)
}

func historicalIDsFromDiskContext(ctx context.Context, ws *Workspace) ([]string, error) {
	if ws == nil {
		return nil, nil
	}
	storageRoot := workspaceStorageRoot(ws)
	var ids []string

	dirSet := map[string]bool{}
	var dirs []string
	searchDirs, searchErr := artifactSearchDirs(ws)
	if searchErr != nil {
		if ws.Config != nil {
			return nil, fmt.Errorf("resolve artifact search dirs: %w", searchErr)
		}
	} else {
		for _, d := range searchDirs {
			dirSet[filepath.Clean(d)] = true
			dirs = append(dirs, d)
		}
	}
	archiveDir := filepath.Join(storageRoot, "archive")
	if !dirSet[filepath.Clean(archiveDir)] {
		dirs = append(dirs, archiveDir)
	}
	queueDir := filepath.Join(storageRoot, "queue")
	if !dirSet[filepath.Clean(queueDir)] {
		dirs = append(dirs, queueDir)
	}
	for _, dir := range dirs {
		found, err := markdownIDsInDir(dir)
		if err != nil {
			return nil, err
		}
		ids = append(ids, found...)
	}

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	entries, err := os.ReadDir(logsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read logs dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".jsonl"))
		found, readErr := eventLogIDs(filepath.Join(logsDir, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		ids = append(ids, found...)
	}

	stashIDs, err := jsonlStringFields(filepath.Join(archiveDir, stashArchiveFileName),
		"harvested_artifact_id")
	if err != nil {
		return nil, err
	}
	ids = append(ids, stashIDs...)
	correctionIDs, err := jsonlStringFields(filepath.Join(archiveDir, "provenance_corrections.jsonl"),
		"historical_artifact_id", "canonical_delivery_artifact_id")
	if err != nil {
		return nil, err
	}
	ids = append(ids, correctionIDs...)

	reservedIDs, err := readReservedSequenceIDs(ws)
	if err != nil {
		return nil, err
	}
	ids = append(ids, reservedIDs...)

	pendingIDs, err := pendingReconstituteIDs(ws)
	if err != nil {
		return nil, err
	}
	ids = append(ids, pendingIDs...)

	branchIDs, err := scanLocalBranchArtifactIDsContext(ctx, ws)
	if err != nil {
		return nil, err
	}
	ids = append(ids, branchIDs...)

	return ids, nil
}

func pendingReconstituteIDs(ws *Workspace) ([]string, error) {
	realStorage, err := resolveValidatedStorageRoot(ws)
	if err != nil {
		return nil, err
	}
	if realStorage == "" {
		return nil, nil
	}
	opsDir := filepath.Join(realStorage, "ops")
	info, err := os.Lstat(opsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect pending shipment journals: %w", err)
	}
	if link, linkErr := IsSymlinkOrReparsePoint(info, opsDir); linkErr != nil || link || !info.IsDir() {
		return nil, fmt.Errorf("shipment ops directory %s is redirected or not a directory: %w", opsDir, blerrors.ErrValidation)
	}
	entries, err := os.ReadDir(opsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pending shipment journals: %w", err)
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() || !shipmentLifecycleJournalNamePattern.MatchString(entry.Name()) {
			continue
		}
		entryPath := filepath.Join(opsDir, entry.Name())
		entryInfo, lstatErr := os.Lstat(entryPath)
		if lstatErr != nil {
			if os.IsNotExist(lstatErr) {
				continue
			}
			return nil, fmt.Errorf("inspect pending journal %s: %w", entry.Name(), lstatErr)
		}
		if link, linkErr := IsSymlinkOrReparsePoint(entryInfo, entryPath); linkErr != nil || link || !entryInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("pending journal %s is redirected or not regular: %w", entry.Name(), blerrors.ErrValidation)
		}
		raw, readErr := os.ReadFile(entryPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, fmt.Errorf("read pending journal %s: %w", entry.Name(), readErr)
		}
		var journal shipmentLifecycleJournal
		if unmarshalErr := json.Unmarshal(raw, &journal); unmarshalErr != nil {
			return nil, fmt.Errorf("decode pending journal %s: %w: %w", entry.Name(), unmarshalErr, blerrors.ErrValidation)
		}
		if journal.Phase != shipmentOperationPhaseIntent || journal.Reconstitute == nil {
			continue
		}
		ids = append(ids, journal.Reconstitute.CreatedIDs...)
		for _, newID := range journal.Reconstitute.Adoptions {
			if newID != "" {
				ids = append(ids, newID)
			}
		}
	}
	return ids, nil
}

func resolveValidatedStorageRoot(ws *Workspace) (string, error) {
	if ws == nil {
		return "", fmt.Errorf("workspace is required: %w", blerrors.ErrValidation)
	}
	storageRoot := workspaceStorageRoot(ws)
	if strings.TrimSpace(storageRoot) == "" {
		return "", nil
	}
	absStorage, err := filepath.Abs(storageRoot)
	if err != nil {
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	absStorage = filepath.Clean(absStorage)

	var realWS string
	if strings.TrimSpace(ws.RootPath) != "" {
		absWS, wsErr := filepath.Abs(ws.RootPath)
		if wsErr != nil {
			return "", fmt.Errorf("resolve workspace root: %w", wsErr)
		}
		absWS = filepath.Clean(absWS)
		wsInfo, wsLstatErr := os.Lstat(absWS)
		if wsLstatErr != nil && !os.IsNotExist(wsLstatErr) {
			return "", fmt.Errorf("lstat workspace root: %w", wsLstatErr)
		}
		if wsLstatErr == nil {
			if link, linkErr := IsSymlinkOrReparsePoint(wsInfo, absWS); linkErr != nil || link || !wsInfo.IsDir() {
				return "", fmt.Errorf("workspace root %q is redirected or not a directory: %w", absWS, blerrors.ErrValidation)
			}
		}
		evalWS, wsEvalErr := filepath.EvalSymlinks(absWS)
		if wsEvalErr != nil {
			if !os.IsNotExist(wsEvalErr) {
				return "", fmt.Errorf("eval workspace root symlinks: %w", wsEvalErr)
			}
			realWS = absWS
		} else {
			realWS = filepath.Clean(evalWS)
		}
	}

	info, lstatErr := os.Lstat(absStorage)
	if lstatErr != nil {
		if os.IsNotExist(lstatErr) {
			if realWS != "" && !pathContained(realWS, absStorage) && !sameLocksDirPath(realWS, absStorage) {
				return "", fmt.Errorf("storage root %q is outside workspace %q: %w", absStorage, realWS, blerrors.ErrValidation)
			}
			return absStorage, nil
		}
		return "", fmt.Errorf("lstat storage root: %w", lstatErr)
	}
	if link, linkErr := IsSymlinkOrReparsePoint(info, absStorage); linkErr != nil || link || !info.IsDir() {
		return "", fmt.Errorf("storage root %q is redirected or not a directory: %w", absStorage, blerrors.ErrValidation)
	}
	realStorage, err := filepath.EvalSymlinks(absStorage)
	if err != nil {
		return "", fmt.Errorf("eval storage root symlinks: %w", err)
	}
	realStorage = filepath.Clean(realStorage)
	if realWS != "" && !pathContained(realWS, realStorage) && !sameLocksDirPath(realWS, realStorage) {
		return "", fmt.Errorf("storage root %q resolves outside workspace %q: %w", realStorage, realWS, blerrors.ErrValidation)
	}
	return realStorage, nil
}

func sameLocksDirPath(a, b string) bool {
	ca := filepath.Clean(a)
	cb := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(ca, cb)
	}
	return ca == cb
}

func openStorageRootPinned(ws *Workspace, realStorage string) (*os.Root, error) {
	if ws != nil && strings.TrimSpace(ws.RootPath) != "" {
		absWS, err := filepath.Abs(ws.RootPath)
		if err != nil {
			return nil, fmt.Errorf("resolve workspace root: %w", err)
		}
		absWS = filepath.Clean(absWS)
		wsInfo, err := os.Lstat(absWS)
		if err != nil {
			return nil, fmt.Errorf("inspect workspace root: %w", err)
		}
		if link, linkErr := IsSymlinkOrReparsePoint(wsInfo, absWS); linkErr != nil || link || !wsInfo.IsDir() {
			return nil, fmt.Errorf("workspace root %q is redirected or not a directory: %w", absWS, blerrors.ErrValidation)
		}
		realWS, err := filepath.EvalSymlinks(absWS)
		if err != nil {
			return nil, fmt.Errorf("eval workspace root symlinks: %w", err)
		}
		relStorage, err := filepath.Rel(realWS, realStorage)
		if err != nil || relStorage == ".." || strings.HasPrefix(relStorage, ".."+string(filepath.Separator)) || filepath.IsAbs(relStorage) {
			return nil, fmt.Errorf("storage root %q escapes workspace %q: %w", realStorage, realWS, blerrors.ErrValidation)
		}
		wsRoot, err := os.OpenRoot(realWS)
		if err != nil {
			return nil, fmt.Errorf("open workspace root: %w", err)
		}
		defer func() {
			// Workspace root only pins storageRoot opening; storageHandle keeps its own dirfd.
			_ = wsRoot.Close()
		}()
		openedWS, err := wsRoot.Stat(".")
		if err != nil || !os.SameFile(wsInfo, openedWS) {
			return nil, fmt.Errorf("workspace root %q identity changed after open: %w", absWS, blerrors.ErrValidation)
		}
		storageInfo, err := wsRoot.Lstat(relStorage)
		if err != nil {
			return nil, fmt.Errorf("inspect storage root %q: %w", relStorage, err)
		}
		if link, linkErr := IsSymlinkOrReparsePoint(storageInfo, realStorage); linkErr != nil || link || !storageInfo.IsDir() {
			return nil, fmt.Errorf("storage root %q is redirected or not a directory: %w", realStorage, blerrors.ErrValidation)
		}
		storageHandle, err := wsRoot.OpenRoot(relStorage)
		if err != nil {
			return nil, fmt.Errorf("open storage root: %w", err)
		}
		openedStorage, err := storageHandle.Stat(".")
		if err != nil || !os.SameFile(storageInfo, openedStorage) {
			_ = storageHandle.Close()
			return nil, fmt.Errorf("storage root %q identity changed after open: %w", realStorage, blerrors.ErrValidation)
		}
		return storageHandle, nil
	}
	storageInfo, err := os.Lstat(realStorage)
	if err != nil {
		return nil, fmt.Errorf("inspect storage root %q: %w", realStorage, err)
	}
	if link, linkErr := IsSymlinkOrReparsePoint(storageInfo, realStorage); linkErr != nil || link || !storageInfo.IsDir() {
		return nil, fmt.Errorf("storage root %q is redirected or not a directory: %w", realStorage, blerrors.ErrValidation)
	}
	storageHandle, err := os.OpenRoot(realStorage)
	if err != nil {
		return nil, fmt.Errorf("open storage root: %w", err)
	}
	openedStorage, err := storageHandle.Stat(".")
	if err != nil || !os.SameFile(storageInfo, openedStorage) {
		_ = storageHandle.Close()
		return nil, fmt.Errorf("storage root %q identity changed after open: %w", realStorage, blerrors.ErrValidation)
	}
	return storageHandle, nil
}

func verifyPinnedLocksRoot(locksRoot *os.Root, locksDirAbs string) error {
	dirFile, err := locksRoot.Open(".")
	if err != nil {
		return fmt.Errorf("open pinned locks directory handle: %w", err)
	}
	defer func() {
		// Verification handle; close immediately so it never pins directory deletion.
		_ = dirFile.Close()
	}()
	dirStat, err := dirFile.Stat()
	if err != nil {
		return fmt.Errorf("stat pinned locks directory handle: %w", err)
	}
	rootStat, err := locksRoot.Stat(".")
	if err != nil {
		return fmt.Errorf("stat pinned locks root: %w", err)
	}
	if !os.SameFile(dirStat, rootStat) || !dirStat.IsDir() {
		return fmt.Errorf("pinned locks directory identity mismatch: %w", blerrors.ErrValidation)
	}
	return verifyLocksDirHandle(dirFile, locksDirAbs)
}

func openLocksRootReadOnly(ws *Workspace) (*os.Root, *os.Root, string, error) {
	realStorage, err := resolveValidatedStorageRoot(ws)
	if err != nil || realStorage == "" {
		return nil, nil, "", err
	}
	if _, statErr := os.Lstat(realStorage); os.IsNotExist(statErr) {
		return nil, nil, "", nil
	}
	storageHandle, err := openStorageRootPinned(ws, realStorage)
	if err != nil {
		return nil, nil, "", err
	}
	locksDirAbs := filepath.Join(realStorage, shipmentMembershipLocksDirName)
	locksInfo, lstatErr := storageHandle.Lstat(shipmentMembershipLocksDirName)
	if lstatErr != nil {
		_ = storageHandle.Close()
		if os.IsNotExist(lstatErr) {
			return nil, nil, "", nil
		}
		return nil, nil, "", fmt.Errorf("lstat .locks directory: %w", lstatErr)
	}
	if link, linkErr := IsSymlinkOrReparsePoint(locksInfo, locksDirAbs); linkErr != nil || link || !locksInfo.IsDir() {
		_ = storageHandle.Close()
		return nil, nil, "", fmt.Errorf(".locks directory is redirected or not a directory: %w", blerrors.ErrValidation)
	}
	locksRoot, err := storageHandle.OpenRoot(shipmentMembershipLocksDirName)
	if err != nil {
		_ = storageHandle.Close()
		return nil, nil, "", fmt.Errorf("open .locks root: %w", err)
	}
	if verifyErr := verifyPinnedLocksRoot(locksRoot, locksDirAbs); verifyErr != nil {
		_ = locksRoot.Close()
		_ = storageHandle.Close()
		return nil, nil, "", verifyErr
	}
	return storageHandle, locksRoot, locksDirAbs, nil
}

func openLocksRootForWrite(ws *Workspace) (*os.Root, *os.Root, string, error) {
	realStorage, err := resolveValidatedStorageRoot(ws)
	if err != nil {
		return nil, nil, "", err
	}
	if realStorage == "" {
		return nil, nil, "", fmt.Errorf("workspace storage root unavailable: %w", blerrors.ErrValidation)
	}
	storageHandle, err := openStorageRootPinned(ws, realStorage)
	if err != nil {
		return nil, nil, "", err
	}
	locksDirAbs := filepath.Join(realStorage, shipmentMembershipLocksDirName)
	locksInfo, lstatErr := storageHandle.Lstat(shipmentMembershipLocksDirName)
	if os.IsNotExist(lstatErr) {
		if mkErr := storageHandle.Mkdir(shipmentMembershipLocksDirName, 0o755); mkErr != nil && !os.IsExist(mkErr) {
			_ = storageHandle.Close()
			return nil, nil, "", fmt.Errorf("create .locks directory: %w", mkErr)
		}
		locksInfo, lstatErr = storageHandle.Lstat(shipmentMembershipLocksDirName)
	}
	if lstatErr != nil {
		_ = storageHandle.Close()
		return nil, nil, "", fmt.Errorf("lstat .locks directory: %w", lstatErr)
	}
	if link, linkErr := IsSymlinkOrReparsePoint(locksInfo, locksDirAbs); linkErr != nil || link || !locksInfo.IsDir() {
		_ = storageHandle.Close()
		return nil, nil, "", fmt.Errorf(".locks directory is redirected or not a directory: %w", blerrors.ErrValidation)
	}
	locksRoot, err := storageHandle.OpenRoot(shipmentMembershipLocksDirName)
	if err != nil {
		_ = storageHandle.Close()
		return nil, nil, "", fmt.Errorf("open .locks root: %w", err)
	}
	if verifyErr := verifyPinnedLocksRoot(locksRoot, locksDirAbs); verifyErr != nil {
		_ = locksRoot.Close()
		_ = storageHandle.Close()
		return nil, nil, "", verifyErr
	}
	return storageHandle, locksRoot, locksDirAbs, nil
}

func acquireIDAllocatorLock(ctx context.Context, ws *Workspace) (context.Context, func() error, error) {
	realStorage, err := resolveValidatedStorageRoot(ws)
	if err != nil {
		return ctx, nil, err
	}
	if realStorage == "" {
		return ctx, func() error { return nil }, nil
	}
	locksDirAbs := filepath.Join(realStorage, shipmentMembershipLocksDirName)
	if held, ok := ctx.Value(idAllocatorLockContextKey{}).(idAllocatorLockState); ok &&
		sameLocksDirPath(held.dirAbs, locksDirAbs) {
		return ctx, func() error { return nil }, nil
	}

	deadline := time.Now().Add(idAllocatorLockBoundedWait)
	mu := taskMutexFor(filepath.Join(locksDirAbs, idAllocatorLockFileName))
	backoff := 5 * time.Millisecond
	for !mu.TryLock() {
		if !time.Now().Before(deadline) {
			return ctx, nil, fmt.Errorf("id allocator in-process lock %s: %w", locksDirAbs, blerrors.ErrGateInProgress)
		}
		select {
		case <-ctx.Done():
			return ctx, nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 100*time.Millisecond {
			backoff *= 2
		}
	}

	unlockInProc := func() {
		mu.Unlock()
	}

	storageHandle, locksRoot, canonLocksDirAbs, err := openLocksRootForWrite(ws)
	if err != nil {
		unlockInProc()
		return ctx, nil, err
	}

	backoff = 10 * time.Millisecond
	for {
		info, lstatErr := locksRoot.Lstat(idAllocatorLockFileName)
		if lstatErr == nil {
			if link, linkErr := IsSymlinkOrReparsePoint(info, filepath.Join(canonLocksDirAbs, idAllocatorLockFileName)); linkErr != nil || link || !info.Mode().IsRegular() {
				_ = locksRoot.Close()
				_ = storageHandle.Close()
				unlockInProc()
				return ctx, nil, fmt.Errorf("id allocator lock %q is redirected or not a regular file: %w", idAllocatorLockFileName, blerrors.ErrValidation)
			}
		} else if !os.IsNotExist(lstatErr) {
			_ = locksRoot.Close()
			_ = storageHandle.Close()
			unlockInProc()
			return ctx, nil, fmt.Errorf("lstat id allocator lock: %w", lstatErr)
		}

		if verifyErr := verifyPinnedLocksRoot(locksRoot, canonLocksDirAbs); verifyErr != nil {
			_ = locksRoot.Close()
			_ = storageHandle.Close()
			unlockInProc()
			return ctx, nil, verifyErr
		}

		f, openErr := locksRoot.OpenFile(idAllocatorLockFileName, os.O_CREATE|os.O_RDWR, 0o600)
		if openErr != nil {
			_ = locksRoot.Close()
			_ = storageHandle.Close()
			unlockInProc()
			return ctx, nil, fmt.Errorf("open id allocator lock: %w", openErr)
		}

		leafStat, statErr := f.Stat()
		lstatNow, lstatNowErr := locksRoot.Lstat(idAllocatorLockFileName)
		if statErr != nil || lstatNowErr != nil || !os.SameFile(leafStat, lstatNow) {
			_ = f.Close()
			_ = locksRoot.Close()
			_ = storageHandle.Close()
			unlockInProc()
			return ctx, nil, fmt.Errorf("id allocator lock identity mismatch: %w", blerrors.ErrValidation)
		}
		if leafErr := verifyLocksLeafHandle(f); leafErr != nil {
			_ = f.Close()
			_ = locksRoot.Close()
			_ = storageHandle.Close()
			unlockInProc()
			return ctx, nil, leafErr
		}

		busy, lockErr := tryLockAllocatorFileHandle(f)
		if lockErr != nil {
			_ = f.Close()
			_ = locksRoot.Close()
			_ = storageHandle.Close()
			unlockInProc()
			return ctx, nil, lockErr
		}
		if !busy {
			var once sync.Once
			var releaseErr error
			lockedCtx := context.WithValue(ctx, idAllocatorLockContextKey{}, idAllocatorLockState{
				dirAbs:    canonLocksDirAbs,
				locksRoot: locksRoot,
			})
			return lockedCtx, func() error {
				once.Do(func() {
					releaseErr = errors.Join(
						unlockAllocatorFileHandle(f),
						f.Close(),
						locksRoot.Close(),
						storageHandle.Close(),
					)
					unlockInProc()
				})
				return releaseErr
			}, nil
		}
		_ = f.Close()
		if !time.Now().Before(deadline) {
			_ = locksRoot.Close()
			_ = storageHandle.Close()
			unlockInProc()
			return ctx, nil, fmt.Errorf("id allocator lock %s: %w", canonLocksDirAbs, blerrors.ErrGateInProgress)
		}
		select {
		case <-ctx.Done():
			_ = locksRoot.Close()
			_ = storageHandle.Close()
			unlockInProc()
			return ctx, nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 250*time.Millisecond {
			backoff *= 2
		}
	}
}

func readIDSequenceLedger(locksRoot *os.Root, locksDirAbs string) (bool, []byte, idSequenceLedger, error) {
	ledger := idSequenceLedger{SchemaVersion: 1}
	if locksRoot == nil {
		return false, nil, ledger, nil
	}
	if err := verifyPinnedLocksRoot(locksRoot, locksDirAbs); err != nil {
		return false, nil, ledger, err
	}
	info, err := locksRoot.Lstat(idSequenceLedgerFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, ledger, nil
		}
		return false, nil, ledger, fmt.Errorf("lstat %s: %w", idSequenceLedgerFileName, err)
	}
	if link, linkErr := IsSymlinkOrReparsePoint(info, filepath.Join(locksDirAbs, idSequenceLedgerFileName)); linkErr != nil || link || !info.Mode().IsRegular() {
		return false, nil, ledger, fmt.Errorf("%s is redirected or not a regular file: %w", idSequenceLedgerFileName, blerrors.ErrValidation)
	}
	f, err := locksRoot.OpenFile(idSequenceLedgerFileName, os.O_RDONLY, 0)
	if err != nil {
		return false, nil, ledger, fmt.Errorf("open %s: %w", idSequenceLedgerFileName, err)
	}
	defer func() {
		// Read-only ledger file close.
		_ = f.Close()
	}()
	openedStat, err := f.Stat()
	if err != nil || !os.SameFile(openedStat, info) {
		return false, nil, ledger, fmt.Errorf("%s identity mismatch: %w", idSequenceLedgerFileName, blerrors.ErrValidation)
	}
	if err := verifyLocksLeafHandle(f); err != nil {
		return false, nil, ledger, err
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return false, nil, ledger, fmt.Errorf("read %s: %w", idSequenceLedgerFileName, err)
	}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &ledger); err != nil {
			return false, nil, ledger, fmt.Errorf("decode %s: %w", idSequenceLedgerFileName, err)
		}
	}
	if ledger.SchemaVersion == 0 {
		ledger.SchemaVersion = 1
	}
	return true, raw, ledger, nil
}

func marshalIDSequenceLedger(ledger idSequenceLedger) ([]byte, error) {
	if ledger.SchemaVersion == 0 {
		ledger.SchemaVersion = 1
	}
	seen := make(map[string]struct{}, len(ledger.ReservedIDs))
	unique := make([]string, 0, len(ledger.ReservedIDs))
	for _, id := range ledger.ReservedIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	sort.Strings(unique)
	ledger.ReservedIDs = unique
	if len(ledger.Reservations) == 0 {
		ledger.Reservations = nil
	} else {
		cleaned := make(map[string][]string, len(ledger.Reservations))
		for corrID, corrIDs := range ledger.Reservations {
			if corrID == "" || len(corrIDs) == 0 {
				continue
			}
			cseen := make(map[string]struct{}, len(corrIDs))
			cuniq := make([]string, 0, len(corrIDs))
			for _, id := range corrIDs {
				if id == "" {
					continue
				}
				if _, ok := cseen[id]; ok {
					continue
				}
				cseen[id] = struct{}{}
				cuniq = append(cuniq, id)
			}
			sort.Strings(cuniq)
			if len(cuniq) > 0 {
				cleaned[corrID] = cuniq
			}
		}
		if len(cleaned) == 0 {
			ledger.Reservations = nil
		} else {
			ledger.Reservations = cleaned
		}
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", idSequenceLedgerFileName, err)
	}
	return append(data, '\n'), nil
}

func writeIDSequenceReservation(ws *Workspace, locksRoot *os.Root, locksDirAbs string, raw []byte) error {
	if locksRoot == nil {
		return fmt.Errorf("locks root is required: %w", blerrors.ErrWriteNotApplied)
	}
	if err := verifyPinnedLocksRoot(locksRoot, locksDirAbs); err != nil {
		return fmt.Errorf("write %s: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, err)
	}
	if info, err := locksRoot.Lstat(idSequenceLedgerFileName); err == nil {
		if link, linkErr := IsSymlinkOrReparsePoint(info, filepath.Join(locksDirAbs, idSequenceLedgerFileName)); linkErr != nil || link || !info.Mode().IsRegular() {
			return fmt.Errorf("%s is redirected or not regular: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, blerrors.ErrValidation)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, err)
	}

	tmpName := fmt.Sprintf("%s.tmp.%d.%d", idSequenceLedgerFileName, os.Getpid(), idSequenceTempSeq.Add(1))
	f, err := locksRoot.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temp %s: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, err)
	}
	fClosed := false
	tmpRemoved := false
	defer func() {
		if !fClosed {
			_ = f.Close()
		}
		if !tmpRemoved {
			_ = locksRoot.Remove(tmpName)
		}
	}()

	openedStat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat temp %s: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, err)
	}
	lstatAfter, err := locksRoot.Lstat(tmpName)
	if err != nil || !os.SameFile(openedStat, lstatAfter) {
		return fmt.Errorf("temp %s identity mismatch: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, blerrors.ErrValidation)
	}
	written := 0
	for written < len(raw) {
		n, writeErr := f.Write(raw[written:])
		if writeErr != nil {
			return fmt.Errorf("write temp %s: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, writeErr)
		}
		if n == 0 {
			return fmt.Errorf("short write temp %s: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, io.ErrShortWrite)
		}
		written += n
	}
	if WorkspaceDurableWrites(ws) {
		if err := f.Sync(); err != nil {
			return fmt.Errorf("sync temp %s: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, err)
		}
	}
	fClosed = true
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w: %w", idSequenceLedgerFileName, blerrors.ErrWriteNotApplied, err)
	}
	if err := atomicfile.ReplaceTempFileInRoot(locksRoot, locksDirAbs, tmpName, idSequenceLedgerFileName, WorkspaceDurableWrites(ws)); err != nil {
		return fmt.Errorf("replace %s: %w", idSequenceLedgerFileName, err)
	}
	tmpRemoved = true
	return nil
}

func removeIDSequenceReservation(locksRoot *os.Root, locksDirAbs string) error {
	if locksRoot == nil {
		return nil
	}
	if err := verifyPinnedLocksRoot(locksRoot, locksDirAbs); err != nil {
		return err
	}
	info, err := locksRoot.Lstat(idSequenceLedgerFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("lstat %s before remove: %w", idSequenceLedgerFileName, err)
	}
	if link, linkErr := IsSymlinkOrReparsePoint(info, filepath.Join(locksDirAbs, idSequenceLedgerFileName)); linkErr != nil || link || !info.Mode().IsRegular() {
		return fmt.Errorf("%s is redirected or not a regular file: %w", idSequenceLedgerFileName, blerrors.ErrValidation)
	}
	if err := locksRoot.Remove(idSequenceLedgerFileName); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", idSequenceLedgerFileName, err)
	}
	return nil
}

func readReservedSequenceIDs(ws *Workspace) ([]string, error) {
	storageHandle, locksRoot, locksDirAbs, err := openLocksRootReadOnly(ws)
	if err != nil {
		return nil, err
	}
	if locksRoot == nil {
		return nil, nil
	}
	defer func() {
		_ = locksRoot.Close()
		_ = storageHandle.Close()
	}()
	exists, _, ledger, err := readIDSequenceLedger(locksRoot, locksDirAbs)
	if err != nil || !exists {
		return nil, err
	}
	out := make([]string, 0, len(ledger.ReservedIDs))
	out = append(out, ledger.ReservedIDs...)
	for _, corrIDs := range ledger.Reservations {
		out = append(out, corrIDs...)
	}
	return out, nil
}

type idSequencePreimage struct {
	enabled     bool
	hadPreimage bool
	raw         []byte
	ledger      idSequenceLedger
}

func resolveLocksRootForWrite(ctx context.Context, ws *Workspace) (*os.Root, string, func(), error) {
	if held, ok := ctx.Value(idAllocatorLockContextKey{}).(idAllocatorLockState); ok && held.locksRoot != nil {
		realStorage, err := resolveValidatedStorageRoot(ws)
		if err != nil {
			return nil, "", nil, err
		}
		if realStorage == "" {
			return nil, "", nil, fmt.Errorf("workspace storage root unavailable: %w", blerrors.ErrValidation)
		}
		expectedLocksDirAbs := filepath.Join(realStorage, shipmentMembershipLocksDirName)
		if sameLocksDirPath(held.dirAbs, expectedLocksDirAbs) {
			if err := verifyPinnedLocksRoot(held.locksRoot, held.dirAbs); err != nil {
				return nil, "", nil, err
			}
			return held.locksRoot, held.dirAbs, func() {}, nil
		}
	}
	storageHandle, locksRoot, locksDirAbs, err := openLocksRootForWrite(ws)
	if err != nil {
		return nil, "", nil, err
	}
	return locksRoot, locksDirAbs, func() {
		// Cleanup callback; caller errors are already returned by read/write ops.
		_ = locksRoot.Close()
		_ = storageHandle.Close()
	}, nil
}

func captureIDSequencePreimage(ctx context.Context, ws *Workspace) (idSequencePreimage, error) {
	realStorage, err := resolveValidatedStorageRoot(ws)
	if err != nil {
		return idSequencePreimage{}, err
	}
	if realStorage == "" {
		return idSequencePreimage{}, nil
	}
	locksRoot, locksDirAbs, cleanup, err := resolveLocksRootForWrite(ctx, ws)
	if err != nil {
		return idSequencePreimage{}, err
	}
	defer cleanup()
	had, raw, ledger, err := readIDSequenceLedger(locksRoot, locksDirAbs)
	if err != nil {
		return idSequencePreimage{}, err
	}
	return idSequencePreimage{
		enabled:     true,
		hadPreimage: had,
		raw:         raw,
		ledger:      ledger,
	}, nil
}

func reserveIDSequenceWithPreimage(ctx context.Context, ws *Workspace, pre idSequencePreimage, artifactID string) error {
	if !pre.enabled || artifactID == "" {
		return nil
	}
	locksRoot, locksDirAbs, cleanup, err := resolveLocksRootForWrite(ctx, ws)
	if err != nil {
		return err
	}
	defer cleanup()
	ledger := idSequenceLedger{
		SchemaVersion: pre.ledger.SchemaVersion,
		ReservedIDs:   append([]string(nil), pre.ledger.ReservedIDs...),
	}
	if len(pre.ledger.Reservations) > 0 {
		ledger.Reservations = make(map[string][]string, len(pre.ledger.Reservations))
		for k, v := range pre.ledger.Reservations {
			ledger.Reservations[k] = append([]string(nil), v...)
		}
	}
	ledger.ReservedIDs = append(ledger.ReservedIDs, artifactID)
	if corrID := shipmentOperationID(ctx); corrID != "" {
		if ledger.Reservations == nil {
			ledger.Reservations = make(map[string][]string)
		}
		ledger.Reservations[corrID] = append(ledger.Reservations[corrID], artifactID)
	}
	raw, err := marshalIDSequenceLedger(ledger)
	if err != nil {
		return err
	}
	return writeIDSequenceReservationFn(ws, locksRoot, locksDirAbs, raw)
}

func restoreIDSequencePreimage(ctx context.Context, ws *Workspace, pre idSequencePreimage) error {
	if !pre.enabled {
		return nil
	}
	locksRoot, locksDirAbs, cleanup, err := resolveLocksRootForWrite(ctx, ws)
	if err != nil {
		return err
	}
	defer cleanup()
	if !pre.hadPreimage {
		return removeIDSequenceReservation(locksRoot, locksDirAbs)
	}
	return writeIDSequenceReservationFn(ws, locksRoot, locksDirAbs, pre.raw)
}

// unreserveIDSequences removes rolled-back IDs and/or correlation-tagged
// reservations from id-sequences.json via the pinned .locks handle under
// id_allocator.lock, removing the ledger file when empty.
func unreserveIDSequences(ctx context.Context, ws *Workspace, correlationID string, ids []string) error {
	realStorage, err := resolveValidatedStorageRoot(ws)
	if err != nil {
		return err
	}
	if realStorage == "" {
		return nil
	}
	storageHandle, readLocksRoot, _, readErr := openLocksRootReadOnly(ws)
	if readErr != nil {
		return readErr
	}
	if readLocksRoot == nil {
		return nil
	}
	// Close read-only inspection handles before acquiring the exclusive write lock.
	_ = readLocksRoot.Close()
	_ = storageHandle.Close()

	lockedCtx, unlockAlloc, lockErr := acquireIDAllocatorLock(ctx, ws)
	if lockErr != nil {
		return fmt.Errorf("acquire id allocator lock for unreserve: %w", lockErr)
	}
	defer func() {
		if unlockErr := unlockAlloc(); unlockErr != nil {
			slog.WarnContext(ctx, "release id allocator lock after unreserve", "correlation_id", correlationID, "error", unlockErr)
		}
	}()

	locksRoot, locksDirAbs, cleanup, err := resolveLocksRootForWrite(lockedCtx, ws)
	if err != nil {
		return err
	}
	defer cleanup()
	had, _, ledger, err := readIDSequenceLedger(locksRoot, locksDirAbs)
	if err != nil || !had {
		return err
	}
	dropSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			dropSet[id] = struct{}{}
		}
	}
	if len(dropSet) == 0 && correlationID != "" {
		for _, id := range ledger.Reservations[correlationID] {
			if id != "" {
				dropSet[id] = struct{}{}
			}
		}
	}
	if len(dropSet) > 0 {
		kept := make([]string, 0, len(ledger.ReservedIDs))
		for _, id := range ledger.ReservedIDs {
			if _, drop := dropSet[id]; !drop {
				kept = append(kept, id)
			}
		}
		ledger.ReservedIDs = kept
		for corr, corrIDs := range ledger.Reservations {
			filtered := make([]string, 0, len(corrIDs))
			for _, id := range corrIDs {
				if _, drop := dropSet[id]; !drop {
					filtered = append(filtered, id)
				}
			}
			if len(filtered) == 0 {
				delete(ledger.Reservations, corr)
			} else {
				ledger.Reservations[corr] = filtered
			}
		}
	}
	if len(ledger.ReservedIDs) == 0 && len(ledger.Reservations) == 0 {
		return removeIDSequenceReservation(locksRoot, locksDirAbs)
	}
	raw, err := marshalIDSequenceLedger(ledger)
	if err != nil {
		return err
	}
	return writeIDSequenceReservationFn(ws, locksRoot, locksDirAbs, raw)
}

func sanitizedGitBranchScanEnv() []string {
	base := config.ChildProcessEnv()
	env := make([]string, 0, len(base)+2)
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(key)
		switch upper {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
			"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_CONFIG_GLOBAL",
			"GIT_CONFIG_NOSYSTEM":
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	return env
}

func scanLocalBranchArtifactIDs(ws *Workspace) ([]string, error) {
	return scanLocalBranchArtifactIDsContext(context.Background(), ws)
}

func scanLocalBranchArtifactIDsContext(parentCtx context.Context, ws *Workspace) ([]string, error) {
	if ws == nil || strings.TrimSpace(ws.RootPath) == "" {
		return nil, nil
	}
	absRoot, err := filepath.Abs(ws.RootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root for branch scan: %w", err)
	}
	absRoot = filepath.Clean(absRoot)
	// Require .git directly at ws.RootPath (and reject symlinks/reparse points) so
	// nested test workspaces or redirected .git entries never scan an external repo.
	dotGitPath := filepath.Join(absRoot, ".git")
	dotGitInfo, statErr := os.Lstat(dotGitPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect .git for branch scan: %w", statErr)
	}
	if link, linkErr := IsSymlinkOrReparsePoint(dotGitInfo, dotGitPath); linkErr != nil || link {
		return nil, fmt.Errorf(".git entry is redirected: %w", blerrors.ErrValidation)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("eval workspace root symlinks for branch scan: %w", err)
	}
	timeout := ws.boundedHelperTimeout()
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	gitEnv := sanitizedGitBranchScanEnv()
	refsCmd := exec.CommandContext(ctx, "git", "--no-replace-objects", "--no-optional-locks",
		"for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads")
	refsCmd.Dir = realRoot
	refsCmd.Env = gitEnv
	refsOut, err := refsCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list local git branches for id history: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(refsOut)), "\n")
	type branchRef struct {
		name string
		oid  string
	}
	var refs []branchRef
	var fpLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		refs = append(refs, branchRef{name: fields[0], oid: fields[1]})
		fpLines = append(fpLines, fields[0]+"="+fields[1])
	}
	if len(refs) == 0 {
		return nil, nil
	}
	sort.Strings(fpLines)
	fingerprint := strings.Join(fpLines, ";")
	if cached, ok := branchIDScanCache.Load(realRoot); ok {
		entry := cached.(branchIDCacheEntry)
		if entry.fingerprint == fingerprint {
			return append([]string(nil), entry.ids...), nil
		}
	}

	seen := make(map[string]struct{})
	var collected []string
	lsTreeBaseArgs := append([]string{"--no-replace-objects", "--no-optional-locks", "ls-tree", "-r", "--name-only"}, "")
	for _, ref := range refs {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("scan local git branch %s for id history: %w", ref.name, ctxErr)
		}
		lsTreeBaseArgs[len(lsTreeBaseArgs)-1] = ref.oid
		lsArgs := append(append([]string(nil), lsTreeBaseArgs...), "--")
		lsArgs = append(lsArgs, workspaceRootCandidates[:]...)
		lsCmd := exec.CommandContext(ctx, "git", lsArgs...)
		lsCmd.Dir = realRoot
		lsCmd.Env = gitEnv
		lsOut, lsErr := lsCmd.Output()
		if lsErr != nil {
			return nil, fmt.Errorf("scan git tree %s (%s) for id history: %w", ref.name, ref.oid, lsErr)
		}
		for _, path := range strings.Split(string(lsOut), "\n") {
			path = strings.TrimSpace(path)
			if path == "" || filepath.Ext(path) != ".md" {
				continue
			}
			stem := strings.TrimSuffix(filepath.Base(path), ".md")
			if stem == "" {
				continue
			}
			if _, ok := seen[stem]; !ok {
				seen[stem] = struct{}{}
				collected = append(collected, stem)
			}
		}
	}
	branchIDScanCache.Store(realRoot, branchIDCacheEntry{
		fingerprint: fingerprint,
		ids:         collected,
	})
	return append([]string(nil), collected...), nil
}

func markdownIDsInDir(dir string) ([]string, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", dir, err)
	}
	var ids []string
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		ids = append(ids, strings.TrimSuffix(d.Name(), ".md"))
		if id := frontmatterIDLine(path); id != "" {
			ids = append(ids, id)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s for historical ids: %w", dir, walkErr)
	}
	return ids, nil
}

// frontmatterIDLine returns the top-level id: value from a Markdown file's
// frontmatter without parsing the whole document, or "" when absent.
func frontmatterIDLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			if strings.TrimSpace(line) != "---" {
				return ""
			}
			continue
		}
		if strings.TrimSpace(line) == "---" {
			return ""
		}
		if value, ok := strings.CutPrefix(line, "id:"); ok {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}

// forEachJSONLine calls fn with every non-empty line of a JSONL file. A
// missing file yields no calls; a symlinked file is refused.
func forEachJSONLine(path string, fn func(line []byte)) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%q is a symlink; refusing to read through it", path)
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			fn(trimmed)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return fmt.Errorf("read %s: %w", path, readErr)
		}
	}
}

func eventLogIDs(path string) ([]string, error) {
	var ids []string
	err := forEachJSONLine(path, func(line []byte) {
		var rec struct {
			ItemID    string         `json:"item_id"`
			EventType string         `json:"event_type"`
			Delta     map[string]any `json:"delta"`
		}
		if json.Unmarshal(line, &rec) != nil {
			return
		}
		if rec.ItemID != "" {
			ids = append(ids, rec.ItemID)
		}
		for _, key := range []string{"old_id", "new_id"} {
			if v, ok := rec.Delta[key].(string); ok && v != "" {
				ids = append(ids, v)
			}
		}
		for _, listKey := range []string{"created_features", "created_shipments"} {
			if rawList, ok := rec.Delta[listKey].([]any); ok {
				for _, elem := range rawList {
					if s, ok := elem.(string); ok && s != "" {
						ids = append(ids, s)
					}
				}
			}
		}
		if adoptedMap, ok := rec.Delta["adopted_id_map"].(map[string]any); ok {
			for k, v := range adoptedMap {
				if k != "" {
					ids = append(ids, k)
				}
				if s, ok := v.(string); ok && s != "" {
					ids = append(ids, s)
				}
			}
		}
	})
	return ids, err
}

func jsonlStringFields(path string, fields ...string) ([]string, error) {
	var ids []string
	err := forEachJSONLine(path, func(line []byte) {
		var rec map[string]any
		if json.Unmarshal(line, &rec) != nil {
			return
		}
		for _, field := range fields {
			if v, ok := rec[field].(string); ok && v != "" {
				ids = append(ids, v)
			}
		}
	})
	return ids, err
}

// maxTypedOrdinalAmong returns the highest ordinal among ids that sit directly
// under parentPath (empty for root level) and use the given type affixes.
// Supports both canonical ("001.001-T") and slugged ("001.001-T-title") stems.
func maxTypedOrdinalAmong(ids []string, parentPath, prefix, suffix string) int {
	maxOrdinal := 0
	for _, id := range ids {
		idParent := ""
		if idx := strings.LastIndex(id, "."); idx >= 0 {
			idParent = id[:idx]
		}
		if idParent != parentPath {
			continue
		}
		seg := lastIDSegment(id)
		ordinal, ok := typedSegmentOrdinal(seg, prefix, suffix)
		if !ok && (prefix != "" || suffix != "") {
			ordinal, ok = historicalStandaloneOrdinal(seg, prefix, suffix)
		}
		if ok && ordinal > maxOrdinal {
			maxOrdinal = ordinal
		}
	}
	return maxOrdinal
}

// maxStandaloneOrdinalAmong returns the highest ordinal among ids whose last
// segment strictly matches the type affixes. Types without any affix are
// ignored because their numeric IDs cannot be attributed to the type.
func maxStandaloneOrdinalAmong(ids []string, typeCfg *config.ArtifactTypeConfig) int {
	if typeCfg == nil || (typeCfg.Prefix == "" && typeCfg.Suffix == "") {
		return 0
	}
	maxOrdinal := 0
	for _, id := range ids {
		if ordinal, ok := historicalStandaloneOrdinal(lastIDSegment(id), typeCfg.Prefix, typeCfg.Suffix); ok && ordinal > maxOrdinal {
			maxOrdinal = ordinal
		}
	}
	return maxOrdinal
}

// historicalStandaloneOrdinal parses a standalone ID segment the way NextID's
// naming format produces it: the strict typed form ("001-T" / "T001") or the
// slugged form ("001-T-implement-jwt" / "T001-implement-jwt"). The type
// affix must match exactly and be followed by the end of the segment or a
// "-" slug separator, so IDs of other types are never attributed to this type.
func historicalStandaloneOrdinal(segment, prefix, suffix string) (int, bool) {
	if ordinal, ok := typedSegmentOrdinal(segment, prefix, suffix); ok {
		return ordinal, true
	}
	var digitsAndRest string
	var rest string
	if suffix != "" {
		digitsAndRest = segment
		digits := leadingDigits(digitsAndRest)
		if digits == "" {
			return 0, false
		}
		rest = strings.TrimPrefix(digitsAndRest, digits)
		if !strings.HasPrefix(rest, suffix) {
			return 0, false
		}
		rest = strings.TrimPrefix(rest, suffix)
		if !strings.HasPrefix(rest, "-") {
			return 0, false
		}
		n, err := strconv.Atoi(digits)
		return n, err == nil
	}
	if !strings.HasPrefix(segment, prefix) {
		return 0, false
	}
	digitsAndRest = strings.TrimPrefix(segment, prefix)
	digits := leadingDigits(digitsAndRest)
	if digits == "" || !strings.HasPrefix(strings.TrimPrefix(digitsAndRest, digits), "-") {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	return n, err == nil
}

// MaxDiskArtifactSequence returns the highest ordinal recorded for the
// artifact type anywhere in workspace disk history (queue, archive, logs,
// stash archive, and provenance corrections).
func MaxDiskArtifactSequence(ws *Workspace, typeCfg *config.ArtifactTypeConfig) (int, error) {
	ids, err := historicalIDsFromDisk(ws)
	if err != nil {
		return 0, err
	}
	return maxStandaloneOrdinalAmong(ids, typeCfg), nil
}

// errIDHistoryScan marks a failure to read workspace ID history; callers must
// not fall back to an ID that could collide with retired history.
var errIDHistoryScan = errors.New("scan id history")

// allocateTypedHierarchicalID is NextTypedHierarchicalID extended with
// workspace disk history so retired IDs are never reused.
func allocateTypedHierarchicalID(
	ctx context.Context,
	ws *Workspace,
	parentID, artifactType string,
	typeCfg *config.ArtifactTypeConfig,
	layout *config.QueueLayoutConfig,
) (string, error) {
	diskIDs, err := historicalIDsFromDiskContext(ctx, ws)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errIDHistoryScan, err)
	}
	return nextTypedHierarchicalID(ctx, ws.DB, diskIDs, parentID, artifactType, typeCfg, layout)
}
