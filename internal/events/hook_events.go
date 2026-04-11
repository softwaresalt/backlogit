package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// hookLockStaleTTL is the age after which a .lock sidecar file is treated as
// stale (left by a crashed process) and removed automatically.
const hookLockStaleTTL = 60 * time.Second

// v1 hook event type constants. Deferred types (stash_overflow, shipment_ready)
// require lifecycle hook points from 007-DL and are not included in v1.
const (
	HookEventFeatureReviewReady = "feature_review_ready"
	HookEventPostMergeClosure   = "post_merge_closure"
	HookEventBlockedStale       = "blocked_stale"
)

// HookEvent represents a signal emitted to the agent hook event queue.
type HookEvent struct {
	Seq       int64          `json:"seq"`
	Timestamp time.Time      `json:"timestamp"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}

// HookEventWriter provides goroutine-safe, cross-process-safe sequenced appends
// to the shared hook event queue at .backlogit/hooks_queue.jsonl.
// Sequence numbers are allocated under a cross-process lock on every append.
type HookEventWriter struct {
	queuePath string
	mu        sync.Mutex
}

// NewHookEventWriter creates a writer targeting hooks_queue.jsonl under backlogitDir.
func NewHookEventWriter(backlogitDir string) *HookEventWriter {
	return &HookEventWriter{
		queuePath: filepath.Join(backlogitDir, "hooks_queue.jsonl"),
	}
}

// AppendHookEvent appends a sequenced hook event to the queue.
// The sequence counter is determined by scanning existing events and incremented
// under a combined in-process mutex and cross-process sidecar lock to prevent
// duplicate sequence numbers across concurrent writers.
// Returns the assigned monotonic sequence number.
func (w *HookEventWriter) AppendHookEvent(ctx context.Context, event HookEvent) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Cross-process advisory lock via sidecar file.
	lockPath := w.queuePath + ".lock"
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		// Remove a stale lock left by a crashed process before retrying.
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > hookLockStaleTTL {
			slog.Warn("removing stale hook lock file", "path", lockPath)
			_ = os.Remove(lockPath)
			lf, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		}
		if err != nil {
			return 0, fmt.Errorf("hook queue locked by another process: %w", err)
		}
	}
	_ = lf.Close()
	defer func() {
		if rmErr := os.Remove(lockPath); rmErr != nil && !os.IsNotExist(rmErr) {
			slog.Warn("failed to remove hook lock file", "path", lockPath, "error", rmErr)
		}
	}()

	if mkdirErr := os.MkdirAll(filepath.Dir(w.queuePath), 0o755); mkdirErr != nil {
		return 0, fmt.Errorf("create hook queue dir: %w", mkdirErr)
	}

	maxSeq, err := w.scanMaxSeq(ctx)
	if err != nil {
		return 0, fmt.Errorf("read max seq: %w", err)
	}

	nextSeq := maxSeq + 1
	event.Seq = nextSeq
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	line, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("marshal hook event: %w", err)
	}
	line = append(line, '\n')

	qf, err := os.OpenFile(w.queuePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open hook queue: %w", err)
	}
	defer qf.Close()

	if _, writeErr := qf.Write(line); writeErr != nil {
		return 0, fmt.Errorf("write hook event: %w", writeErr)
	}
	return nextSeq, nil
}

// ReadHookEvents reads all events from the queue file in append order.
// Returns nil if the queue file does not exist.
func (w *HookEventWriter) ReadHookEvents(_ context.Context) ([]HookEvent, error) {
	qf, err := os.Open(w.queuePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open hook queue: %w", err)
	}
	defer qf.Close()

	var evs []HookEvent
	scanner := bufio.NewScanner(qf)
	// Raise the default 64 KiB token limit to 1 MiB to handle large event payloads.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev HookEvent
		if unmarshalErr := json.Unmarshal(line, &ev); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshal hook event: %w", unmarshalErr)
		}
		evs = append(evs, ev)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("scan hook queue: %w", scanErr)
	}
	return evs, nil
}

// scanMaxSeq returns the highest sequence number stored in the queue, or 0 if empty.
// Must be called while the writer holds its lock.
func (w *HookEventWriter) scanMaxSeq(ctx context.Context) (int64, error) {
	evs, err := w.ReadHookEvents(ctx)
	if err != nil {
		return 0, err
	}
	var max int64
	for _, ev := range evs {
		if ev.Seq > max {
			max = ev.Seq
		}
	}
	return max, nil
}
