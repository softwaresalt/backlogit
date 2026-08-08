package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateproof"
)

// gateEvidenceCounterStaleTTL is the age after which a gate-evidence counter
// lock sidecar file is treated as stale (left by a crashed process) and
// removed automatically, mirroring events.HookEventWriter's hookLockStaleTTL.
const gateEvidenceCounterStaleTTL = 60 * time.Second

// gateEvidenceCounterMu serializes concurrent counter allocation across
// goroutines within this process. A cross-process advisory sidecar-file lock
// (acquired inside nextGateEvidenceCounter) additionally covers separate
// backlogit processes racing on the same item (106-F F1/U4).
var gateEvidenceCounterMu sync.Mutex

// nextGateEvidenceCounter allocates the next monotonic per-item counter for a
// formal-gate-evidence proof. It holds a combined in-process mutex and
// cross-process sidecar-file lock across the read-current-counter, increment,
// and (by the caller) sign-and-append sequence, so two concurrent callers can
// never allocate — and therefore sign and append — the same counter for the
// same item. This mirrors events.HookEventWriter's AppendHookEvent locking
// pattern (internal/events/hook_events.go).
//
// The caller MUST call the returned unlock exactly once, after the sign+append
// step completes (success or failure), and must not allocate a second counter
// for the same item while still holding the first lock.
func nextGateEvidenceCounter(ctx context.Context, ws *Workspace, itemID string) (counter int64, unlock func(), err error) {
	gateEvidenceCounterMu.Lock()

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	logPath := events.LogPathForItem(logsDir, itemID)
	lockPath := logPath + ".gateproof.lock"

	release, lockErr := acquireGateEvidenceCounterLock(lockPath)
	if lockErr != nil {
		gateEvidenceCounterMu.Unlock()
		return 0, nil, fmt.Errorf("gate evidence counter locked for %s: %w", itemID, lockErr)
	}

	max, scanErr := scanMaxGateEvidenceCounter(ctx, logsDir, itemID)
	if scanErr != nil {
		release()
		gateEvidenceCounterMu.Unlock()
		return 0, nil, fmt.Errorf("scan gate evidence counter for %s: %w", itemID, scanErr)
	}

	unlockOnce := sync.Once{}
	unlockFn := func() {
		unlockOnce.Do(func() {
			release()
			gateEvidenceCounterMu.Unlock()
		})
	}
	return max + 1, unlockFn, nil
}

// acquireGateEvidenceCounterLock acquires a cross-process advisory lock via an
// O_CREATE|O_EXCL sidecar file, recovering a lock left behind by a crashed
// process once it exceeds gateEvidenceCounterStaleTTL. Mirrors
// events.HookEventWriter's lock-acquisition logic exactly (same stale-recovery
// shape) so the two independent locking call sites stay behaviorally
// consistent.
func acquireGateEvidenceCounterLock(lockPath string) (release func(), err error) {
	if mkdirErr := os.MkdirAll(filepath.Dir(lockPath), 0o755); mkdirErr != nil {
		return nil, fmt.Errorf("create gate evidence log dir: %w", mkdirErr)
	}

	lf, openErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if openErr != nil {
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > gateEvidenceCounterStaleTTL {
			slog.Warn("removing stale gate evidence counter lock file", "path", lockPath)
			recoveringPath := lockPath + ".recovering"
			_ = os.Remove(recoveringPath)
			if renameErr := os.Rename(lockPath, recoveringPath); renameErr == nil {
				lf, openErr = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
				_ = os.Remove(recoveringPath)
			} else {
				lf, openErr = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			}
		}
		if openErr != nil {
			return nil, fmt.Errorf("gate evidence counter locked by another process: %w", openErr)
		}
	}
	_ = lf.Close()

	return func() {
		if rmErr := os.Remove(lockPath); rmErr != nil && !os.IsNotExist(rmErr) {
			slog.Warn("failed to remove gate evidence counter lock file", "path", lockPath, "error", rmErr)
		}
	}, nil
}

// scanMaxGateEvidenceCounter returns the highest "counter" delta value already
// recorded among the item's gate-evidence events, or 0 if none carry one
// (either because no formal-gate evidence has been recorded yet, or the
// workspace only recently enabled formal admission).
func scanMaxGateEvidenceCounter(ctx context.Context, logsDir, itemID string) (int64, error) {
	evs, err := events.ReadAllEvents(ctx, logsDir, itemID)
	if err != nil {
		return 0, fmt.Errorf("read events for %s: %w", itemID, err)
	}
	var max int64
	for _, ev := range evs {
		raw, ok := ev.Delta["counter"]
		if !ok {
			continue
		}
		var c int64
		switch v := raw.(type) {
		case int64:
			c = v
		case int:
			c = int64(v)
		case float64:
			c = int64(v)
		default:
			continue
		}
		if c > max {
			max = c
		}
	}
	return max, nil
}

// workspaceIdentity derives a stable, deterministic workspace identity for the
// gateproof envelope's workspace_id field from the workspace's absolute root
// path. It requires no new persisted state: the same workspace location
// always yields the same identity, and a relocated or differently-rooted copy
// yields a different one (a deliberate, conservative choice for a
// trust-boundary identifier).
func workspaceIdentity(rootPath string) string {
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		abs = rootPath
	}
	sum := sha256.Sum256([]byte(filepath.ToSlash(abs)))
	return hex.EncodeToString(sum[:16])
}

// augmentDeltaWithFormalProof adds proof, key_id, proof_schema, and counter to
// delta when formal-gate-evidence admission is enabled by workspace config or
// required by the environment anchor (106-F F1/U4). When it is neither, delta
// is returned unchanged so existing workspaces see a byte-identical evidence
// delta.
//
// Returns an error wrapping ErrFormalGateRequired when enforcement applies but
// the key cannot be resolved or the proof cannot be produced — callers MUST
// refuse the transition rather than persist unauthenticated evidence; there
// is no unauthenticated fallback under enforcement.
func (ws *Workspace) augmentDeltaWithFormalProof(ctx context.Context, itemID, eventType string, outcome *GateOutcome, delta map[string]any) error {
	var formalCfg config.FormalGateConfig
	if ws.Config != nil && ws.Config.FormalGate != nil {
		formalCfg = *ws.Config.FormalGate
	}
	if !config.FormalGateEnforced(formalCfg) {
		return nil
	}

	key, keyErr := config.ResolveFormalGateKey()
	if keyErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrFormalGateRequired, keyErr)
	}

	counter, unlock, counterErr := nextGateEvidenceCounter(ctx, ws, itemID)
	if counterErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrFormalGateRequired, counterErr)
	}
	defer unlock()

	env := gateproof.Envelope{
		Magic:        gateproof.Magic,
		Purpose:      gateproof.PurposeTask,
		Schema:       gateproof.Schema,
		Alg:          gateproof.AlgHMACSHA256,
		KeyID:        formalCfg.KeyID,
		WorkspaceID:  workspaceIdentity(ws.RootPath),
		ItemID:       itemID,
		EventType:    eventType,
		Ran:          outcome.Ran,
		Actor:        "backlogit",
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
		HeadSHA:      outcome.HeadSHA,
		// ReportDigest interim source: outcome.GateReportHash (raw-report hash,
		// already computed for the existing gate_report_hash delta field). U5
		// introduces a schema-validated report contract whose digest this must
		// switch to before formal admission (U6) can trust it as "validated".
		ReportDigest: outcome.GateReportHash,
		Counter:      counter,
	}

	proof, signErr := gateproof.Sign(env, key)
	if signErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrFormalGateRequired, signErr)
	}

	delta["proof"] = proof
	delta["key_id"] = formalCfg.KeyID
	delta["proof_schema"] = gateproof.Schema
	delta["counter"] = counter
	return nil
}
