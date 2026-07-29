package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
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

// AppendEvent marshals and appends an event to the item's JSONL log file. In
// durable mode the append is fsynced (file + POSIX parent dir) before returning;
// a partial write or a post-write file/dir fsync failure is surfaced as
// ErrWriteIndeterminate because an append is not atomic and is not safe to
// blindly retry.
func (w *EventWriter) AppendEvent(_ context.Context, event Event) error {
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
func (w *EventWriter) appendDurable(itemID string, data []byte) error {
	if err := w.mkdirAllDurable(w.logsDir); err != nil {
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
	// Conservatively fsync the parent of logsDir so the logsDir dirent itself is
	// durable — this re-confirms durability on retry when a previous attempt created
	// logsDir but the parent fsync failed (U3 fix: ErrWriteNotApplied safe-retry
	// contract). POSIX only; Windows is best-effort.
	if err := w.syncDirIfEnabled(filepath.Dir(w.logsDir)); err != nil {
		return fmt.Errorf("append event fsync parent dir: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, err))
	}
	// Conservatively fsync the parent logs dir on every durable append so the
	// (possibly new) log dirent is durable. POSIX only; Windows is best-effort.
	if err := w.syncDirIfEnabled(w.logsDir); err != nil {
		return fmt.Errorf("append event fsync dir: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, err))
	}
	return nil
}

// mkdirAllDurable creates any missing ancestors of dir and, on POSIX, fsyncs
// each newly created directory's parent so the new dirent is durable. Existing
// ancestors are left untouched.
func (w *EventWriter) mkdirAllDurable(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dir, err)
	}
	var missing []string
	cur := dir
	for {
		if _, err := os.Stat(cur); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", cur, err)
		}
		missing = append(missing, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	// Create shallowest-first so each parent already exists when its child lands.
	for i := len(missing) - 1; i >= 0; i-- {
		d := missing[i]
		if err := os.Mkdir(d, 0o755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		if err := w.syncDirIfEnabled(filepath.Dir(d)); err != nil {
			return fmt.Errorf("fsync parent of %s: %w", d, err)
		}
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
	return fsyncDir(path)
}

// fsyncDir opens the directory at path and fsyncs its handle so a rename or new
// dirent within it is durable. POSIX-only; callers gate on dirSyncEnabled.
func fsyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", path, err)
	}
	syncErr := d.Sync()
	closeErr := d.Close()
	if syncErr != nil {
		return fmt.Errorf("fsync dir %s: %w", path, syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close dir %s: %w", path, closeErr)
	}
	return nil
}
