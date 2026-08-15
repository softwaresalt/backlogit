package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/fsutil"
)

// EventEstimateHistory records size/provenance mutation audit entries.
const EventEstimateHistory = "estimate_history"

// Event represents a state change or comment in the event stream.
type Event struct {
	Timestamp time.Time      `json:"timestamp"`
	Actor     string         `json:"actor"`
	ItemID    string         `json:"item_id"`
	EventType string         `json:"event_type"`
	Delta     map[string]any `json:"delta"`
	CommitSHA string         `json:"commit_sha,omitempty"`
}

type itemLogLockContextKey struct{}
type itemLogFileLockContextKey struct{}

type itemLogLockSet map[string]struct{}

type itemLogFileLockSet map[string]struct{}

var itemLogLockRegistry sync.Map

const (
	itemLogLockStaleTTL = 60 * time.Second
	itemLogLockWait     = 3 * time.Second
)

// EventWriter provides goroutine-safe append-only writes to per-item JSONL log files.
type EventWriter struct {
	logsDir string
	mu      sync.Mutex
	// durable enables the durable_writes fsync protocol (123-F). It is set at
	// construction via WithDurableWrites and never mutated afterward, so the
	// writer is immutable with respect to its durability mode.
	durable bool
	// dirSyncEnabled controls whether durable appends fsync the parent directory.
	// It defaults to (runtime.GOOS != "windows") because Windows has no
	// directory-handle flush (dirent durability is best-effort there). It is a
	// field, not a runtime check, so the platform-gated ordering is exercisable
	// in-process on any host (a process kill cannot simulate power loss).
	dirSyncEnabled bool
	// fsyncFileImpl and fsyncDirImpl are injectable seams (nil-safe via the
	// helpers below) so durable ordering and post-write fsync failures are
	// unit-testable without process kills.
	fsyncFileImpl func(*os.File) error
	fsyncDirImpl  func(string) error
}

// Option configures an EventWriter at construction time.
type Option func(*EventWriter)

// WithDurableWrites enables the durable_writes fsync protocol on the writer:
// when set, every append fsyncs the log file and (on POSIX) conservatively
// fsyncs the parent logs directory, creating and fsyncing any new ancestor
// directories level-by-level. Durability is fixed at construction (the writer
// is immutable) so a default-off flag never leaks into an existing writer.
func WithDurableWrites(durable bool) Option {
	return func(w *EventWriter) { w.durable = durable }
}

// NewEventWriter creates an event writer for the given logs directory. With no
// options it is durable-off and byte-for-byte behavior-compatible with prior
// releases; pass WithDurableWrites(true) to opt into the fsync protocol.
func NewEventWriter(logsDir string, opts ...Option) *EventWriter {
	w := &EventWriter{
		logsDir:        logsDir,
		dirSyncEnabled: runtime.GOOS != "windows",
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// LogPathForItem returns the JSONL log path for a work item ID.
func LogPathForItem(logsDir, itemID string) string {
	return filepath.Join(logsDir, itemID+".jsonl")
}

// Durable reports whether the writer fsyncs each append under the durable_writes
// protocol. It is fixed at construction (WithDurableWrites) and never changes, so
// callers and diagnostics can observe the writer's durability mode.
func (w *EventWriter) Durable() bool { return w.durable }

func itemLogLockKey(logsDir, itemID string) string {
	return filepath.Clean(LogPathForItem(logsDir, itemID))
}

func itemLogMutex(key string) *sync.Mutex {
	if existing, ok := itemLogLockRegistry.Load(key); ok {
		return existing.(*sync.Mutex)
	}
	created := &sync.Mutex{}
	actual, _ := itemLogLockRegistry.LoadOrStore(key, created)
	return actual.(*sync.Mutex)
}

// LockItemLog serializes a read/restore/reindex sequence with all event appends
// for one item log. The returned context carries ownership so AppendEvent and
// ReindexItemLog can participate in the same critical section without
// attempting to lock the non-reentrant mutex a second time.
func LockItemLog(ctx context.Context, logsDir, itemID string) (context.Context, func()) {
	key := itemLogLockKey(logsDir, itemID)
	if existing, ok := ctx.Value(itemLogLockContextKey{}).(itemLogLockSet); ok {
		if _, held := existing[key]; held {
			return ctx, func() {}
		}
	}
	mutex := itemLogMutex(key)
	mutex.Lock()
	set := make(itemLogLockSet)
	if existing, ok := ctx.Value(itemLogLockContextKey{}).(itemLogLockSet); ok {
		for heldKey := range existing {
			set[heldKey] = struct{}{}
		}
	}
	set[key] = struct{}{}
	lockedCtx := context.WithValue(ctx, itemLogLockContextKey{}, set)
	var once sync.Once
	return lockedCtx, func() { once.Do(mutex.Unlock) }
}

func itemLogLockHeld(ctx context.Context, key string) bool {
	set, ok := ctx.Value(itemLogLockContextKey{}).(itemLogLockSet)
	if !ok {
		return false
	}
	_, held := set[key]
	return held
}

func itemLogFileLockHeld(ctx context.Context, key string) bool {
	set, ok := ctx.Value(itemLogFileLockContextKey{}).(itemLogFileLockSet)
	if !ok {
		return false
	}
	_, held := set[key]
	return held
}

func withItemLogFileLock(ctx context.Context, key string) context.Context {
	set := make(itemLogFileLockSet)
	if existing, ok := ctx.Value(itemLogFileLockContextKey{}).(itemLogFileLockSet); ok {
		for heldKey := range existing {
			set[heldKey] = struct{}{}
		}
	}
	set[key] = struct{}{}
	return context.WithValue(ctx, itemLogFileLockContextKey{}, set)
}

func itemLogLockSidecarPath(logsDir, itemID string) string {
	path := LogPathForItem(logsDir, itemID)
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".lock")
}

func acquireItemLogFileLock(ctx context.Context, logsDir, itemID string) (func(), error) {
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create item logs directory: %w", err)
	}
	lockPath := itemLogLockSidecarPath(logsDir, itemID)
	deadline := time.Now().Add(itemLogLockWait)
	backoff := 20 * time.Millisecond
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = file.Close()
			var once sync.Once
			stop := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				ticker := time.NewTicker(itemLogLockStaleTTL / 3)
				defer ticker.Stop()
				for {
					select {
					case <-stop:
						return
					case now := <-ticker.C:
						if touchErr := os.Chtimes(lockPath, now, now); touchErr != nil && !os.IsNotExist(touchErr) {
							slog.Warn("item log lock heartbeat failed", "path", lockPath, "error", touchErr)
						}
					}
				}
			}()
			return func() {
				once.Do(func() {
					close(stop)
					wg.Wait()
					if removeErr := os.Remove(lockPath); removeErr != nil && !os.IsNotExist(removeErr) {
						slog.Warn("failed to remove item log lock", "path", lockPath, "error", removeErr)
					}
				})
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create item log lock %s: %w", lockPath, err)
		}
		info, statErr := os.Stat(lockPath)
		if statErr == nil && time.Since(info.ModTime()) > itemLogLockStaleTTL {
			if removeErr := os.Remove(lockPath); removeErr == nil || os.IsNotExist(removeErr) {
				continue
			}
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("item log lock %s is busy", lockPath)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 250*time.Millisecond {
			backoff *= 2
		}
	}
}

// LockItemLogCrossProcess serializes a read/restore/reindex sequence with all
// event appends for one item log, including writers in other processes.
func LockItemLogCrossProcess(ctx context.Context, logsDir, itemID string) (context.Context, func(), error) {
	key := itemLogLockKey(logsDir, itemID)
	if itemLogLockHeld(ctx, key) {
		if itemLogFileLockHeld(ctx, key) {
			return ctx, func() {}, nil
		}
		fileUnlock, err := acquireItemLogFileLock(ctx, logsDir, itemID)
		if err != nil {
			return ctx, nil, err
		}
		return withItemLogFileLock(ctx, key), fileUnlock, nil
	}
	lockedCtx, processUnlock := LockItemLog(ctx, logsDir, itemID)
	fileUnlock, err := acquireItemLogFileLock(lockedCtx, logsDir, itemID)
	if err != nil {
		processUnlock()
		return ctx, nil, err
	}
	lockedCtx = withItemLogFileLock(lockedCtx, key)
	var once sync.Once
	return lockedCtx, func() {
		once.Do(func() {
			fileUnlock()
			processUnlock()
		})
	}, nil
}

// AppendEvent marshals and appends an event to the item's JSONL log file. In
// durable mode the append is fsynced (file + POSIX parent dir) before returning;
// a partial write or a post-write file/dir fsync failure is surfaced as
// ErrWriteIndeterminate because an append is not atomic and is not safe to
// blindly retry.
func (w *EventWriter) AppendEvent(ctx context.Context, event Event) error {
	if event.ItemID == "" {
		return fmt.Errorf("append event: item_id is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	key := itemLogLockKey(w.logsDir, event.ItemID)
	if itemLogLockHeld(ctx, key) {
		if !itemLogFileLockHeld(ctx, key) {
			fileUnlock, lockErr := acquireItemLogFileLock(ctx, w.logsDir, event.ItemID)
			if lockErr != nil {
				return lockErr
			}
			defer fileUnlock()
		}
	} else {
		_, unlock, lockErr := LockItemLogCrossProcess(ctx, w.logsDir, event.ItemID)
		if lockErr != nil {
			return lockErr
		}
		defer unlock()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.durable {
		return w.appendDurable(event.ItemID, data)
	}
	return w.appendFast(event.ItemID, data)
}

// appendFast is the durable-off path: it preserves the historical behavior
// (MkdirAll + O_APPEND|O_CREATE open + Fprintf, no fsync) byte-for-byte.
func (w *EventWriter) appendFast(itemID string, data []byte) error {
	if err := os.MkdirAll(w.logsDir, 0o755); err != nil {
		return fmt.Errorf("create logs dir: %w", err)
	}
	path := LogPathForItem(w.logsDir, itemID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open item log file: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// appendDurable performs a fsync-backed append. Failures before any bytes are
// written (mkdir/open) are ErrWriteNotApplied (nothing appended, safe to retry);
// a partial write or post-write file/dir fsync failure is ErrWriteIndeterminate.
// The file open/write/fsync/close is delegated to the shared fsutil append
// primitive (syncAppendLineDetailed) so this item-log path cannot drift from the
// hook-event queue path; the level-by-level durable mkdir and the post-append
// logs-dir fsync below remain stream-specific.
//
// Parent-dir durability is re-confirmed inside MkdirAllDurable (pre-write) so
// a repeated parent-fsync failure remains ErrWriteNotApplied and does not
// accumulate duplicate log entries across retries.
func (w *EventWriter) appendDurable(itemID string, data []byte) error {
	syncDir := func(path string) error {
		if w.fsyncDirImpl != nil {
			return w.fsyncDirImpl(path)
		}
		return fsutil.FsyncDir(path)
	}
	if err := fsutil.MkdirAllDurable(w.logsDir, true, w.dirSyncEnabled, syncDir); err != nil {
		return fmt.Errorf("create logs dir: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err))
	}
	path := LogPathForItem(w.logsDir, itemID)

	line := make([]byte, 0, len(data)+1)
	line = append(line, data...)
	line = append(line, '\n')

	res := syncAppendLineDetailed(path, line, w.syncFile)
	if res.err != nil {
		if res.preWrite {
			// The open failed before any bytes were written: nothing was appended,
			// so the failed append is safe to retry.
			return fmt.Errorf("open item log file: %w",
				fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, res.err))
		}
		// Bytes may be partially written or the fsync/close failed: an append is
		// not atomic, so the outcome is indeterminate (do not blindly retry).
		return fmt.Errorf("append event: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, res.err))
	}
	// Conservatively fsync the parent logs dir on every durable append so the
	// (possibly new) log dirent is durable. POSIX only; Windows is best-effort.
	if err := w.syncDirIfEnabled(w.logsDir); err != nil {
		return fmt.Errorf("append event fsync dir: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, err))
	}
	return nil
}

// syncFile fsyncs f through the injectable seam (defaulting to f.Sync()).
func (w *EventWriter) syncFile(f *os.File) error {
	if w.fsyncFileImpl != nil {
		return w.fsyncFileImpl(f)
	}
	return f.Sync()
}

// syncDirIfEnabled fsyncs the directory at path when directory syncing is
// enabled (POSIX); on Windows it is a no-op (best-effort dirent durability).
func (w *EventWriter) syncDirIfEnabled(path string) error {
	if !w.dirSyncEnabled {
		return nil
	}
	if w.fsyncDirImpl != nil {
		return w.fsyncDirImpl(path)
	}
	return fsutil.FsyncDir(path)
}
