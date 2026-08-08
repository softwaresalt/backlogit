package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
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
//
// Hashing routes through internal/canonical.Hash (not a direct crypto/sha256
// call) so this governed gate-evidence payload path stays on the one shared,
// deterministic hashing seam (106-F F1; see internal/canonical/guard_test.go).
// canonical.Hash never fails on a plain string input, but the error is still
// checked and handled fail-closed rather than ignored.
func workspaceIdentity(rootPath string) string {
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		abs = rootPath
	}
	full, hashErr := canonical.Hash(filepath.ToSlash(abs))
	if hashErr != nil {
		return ""
	}
	if len(full) > 32 {
		return full[:32]
	}
	return full
}

// formalGateEnforced reports whether formal-gate-evidence admission is
// currently enforced for this workspace, consulting both workspace config
// (FormalGate.Enabled) and the environment anchor
// (BACKLOGIT_FORMAL_GATE_REQUIRED) via config.FormalGateEnforced. Centralized
// here so every enforcement check point (evidence signing, shipment-level
// verification) reads the same nil-safe config lookup consistently. Safe to
// call on a nil receiver — a nil workspace can never enforce anything.
func (ws *Workspace) formalGateEnforced() bool {
	if ws == nil {
		return false
	}
	return config.FormalGateEnforced(ws.resolvedFormalGateConfig())
}

// resolvedFormalGateConfig returns the workspace's formal-gate config,
// defaulting to the zero value when unset. Centralized (rather than the same
// nil-check-and-dereference repeated at every call site) so
// formalGateEnforced, augmentDeltaWithFormalProof, and
// augmentShipmentDeltaWithFormalProof read one consistent nil-safe lookup.
func (ws *Workspace) resolvedFormalGateConfig() config.FormalGateConfig {
	if ws != nil && ws.Config != nil && ws.Config.FormalGate != nil {
		return *ws.Config.FormalGate
	}
	return config.FormalGateConfig{}
}

// wrapFormalGateRequired wraps cause under bkerrors.ErrFormalGateRequired,
// preserving cause's own error chain via a second %w verb (Go 1.20+ multi-wrap)
// rather than discarding it with %v. This lets a caller still discover a more
// specific sentinel inside cause — e.g. gateproof.Sign's ErrProofInvalid or
// ErrProofUnverifiable — via errors.Is/errors.As, so
// internal/mcp/formal_gate_errors.go's cause-specific dispatch (U8) can route
// to the correct, more actionable error code instead of always falling back
// to the generic formal_gate_required classification.
func wrapFormalGateRequired(cause error) error {
	return fmt.Errorf("%w: %w", bkerrors.ErrFormalGateRequired, cause)
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
//
// The returned unlock is ALWAYS non-nil (a no-op when no counter lock was
// acquired) and MUST be deferred by the caller AFTER performing the real
// durable event append, never before. The counter lock (in-process mutex plus
// cross-process sidecar file) is scoped by nextGateEvidenceCounter to cover
// "allocate, then durably persist" as one atomic critical section — releasing
// it here, before the caller's append, would let a second concurrent
// transition for the same item scan the not-yet-persisted log, allocate the
// SAME counter value, and durably append a colliding proof (a real TOCTOU,
// not just a Go-memory-model data race, so `go test -race` cannot catch a
// regression here — only an end-to-end concurrency test can).
func (ws *Workspace) augmentDeltaWithFormalProof(ctx context.Context, itemID, eventType string, outcome *GateOutcome, delta map[string]any) (unlock func(), err error) {
	noop := func() {}
	if !ws.formalGateEnforced() {
		return noop, nil
	}
	formalCfg := ws.resolvedFormalGateConfig()

	key, keyErr := config.ResolveFormalGateKey()
	if keyErr != nil {
		return noop, wrapFormalGateRequired(keyErr)
	}

	// The report_digest bound into the proof must reflect a report that has
	// actually passed the schema-validated formal report contract (106-F
	// F1/U5) — but only for EventGatePassed, the sole event type the U6
	// admission predicate ever trusts. Other event types (forced, blocked,
	// requeued, escalated, base-override) still get a tamper-evident proof
	// for audit purposes, using the existing best-effort report hash, since
	// they can never be formally admitted regardless of report shape.
	reportDigest := outcome.GateReportHash
	if eventType == EventGatePassed {
		validated, valErr := gate.ValidateFormalReport(outcome.ReportJSON)
		if valErr != nil {
			return noop, fmt.Errorf("%w: formal report: %w", bkerrors.ErrFormalGateRequired, valErr)
		}
		digest, digestErr := gate.FormalReportDigest(*validated)
		if digestErr != nil {
			return noop, wrapFormalGateRequired(digestErr)
		}
		reportDigest = digest
	}

	counter, countUnlock, counterErr := nextGateEvidenceCounter(ctx, ws, itemID)
	if counterErr != nil {
		return noop, wrapFormalGateRequired(counterErr)
	}

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
		ReportDigest: reportDigest,
		Counter:      counter,
	}

	proof, signErr := gateproof.Sign(env, key)
	if signErr != nil {
		countUnlock()
		return noop, wrapFormalGateRequired(signErr)
	}

	delta["proof"] = proof
	delta["key_id"] = formalCfg.KeyID
	delta["proof_schema"] = gateproof.Schema
	delta["counter"] = counter
	// timestamp_utc and report_digest are bound inside the MAC but are not
	// otherwise derivable from the rest of the delta (timestamp_utc is a
	// live value; report_digest may differ from the pre-existing
	// gate_report_hash field once EventGatePassed uses the validated-report
	// digest). Both must be persisted verbatim so a verifier can reconstruct
	// the exact signed envelope later.
	delta["timestamp_utc"] = env.TimestampUTC
	delta["report_digest"] = reportDigest
	return countUnlock, nil
}
