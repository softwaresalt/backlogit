package core

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ancestryCheckTimeout is the HARD CAP on each git lineage/head-resolution
// subprocess. The shipment ship path is unbounded and holds the workspace lock
// across completion, so an unbounded (or long-running) git child would pin the
// lock (a denial of service). git merge-base/rev-parse are near-instant LOCAL
// reads, so 5s is generous. GateBroker.TimeoutSeconds is sized for build/test
// gate COMMANDS (default 600s) and must never be adopted verbatim for these
// metadata reads; boundedHelperTimeout caps at this value while still honoring a
// smaller configured gate timeout. Every helper that spawns git here derives its
// OWN deadline — it never relies on the caller imposing one.
const ancestryCheckTimeout = 5 * time.Second

// gitObjectNameRe matches exactly the full-length object names git rev-parse can
// produce: a 40-hex SHA-1 or a 64-hex SHA-256. Abbreviations, refs, and any value
// containing a leading dash or non-hex byte are rejected, so a tampered on-disk
// head_sha can never be handed to git as an option or an ambiguous ref.
var gitObjectNameRe = regexp.MustCompile(`^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

// isGitObjectName reports whether s is a full-length git object name (SHA-1 or
// SHA-256). It is the input-validation guard applied to the untrusted recorded
// member head_sha before it is passed to git (argument-injection defense: "data
// must not choose the args").
func isGitObjectName(s string) bool {
	return gitObjectNameRe.MatchString(s)
}

// boundedHelperTimeout returns the deadline for the fast git metadata helpers
// (isAncestor, headSHABounded). ancestryCheckTimeout is a hard cap: a configured
// GateBroker.TimeoutSeconds (sized for build/test gate commands, 600s by default)
// must not let one of these near-instant local reads hold the workspace lock for
// minutes. A smaller configured gate timeout is still honored.
func (ws *Workspace) boundedHelperTimeout() time.Duration {
	d := ancestryCheckTimeout
	if ws.GateBroker != nil && ws.GateBroker.TimeoutSeconds > 0 {
		if configured := time.Duration(ws.GateBroker.TimeoutSeconds) * time.Second; configured < d {
			d = configured
		}
	}
	return d
}

// isAncestor reports whether ancestor is an ancestor of (or equal to) descendant
// by running `git merge-base --is-ancestor ancestor descendant` under a mandatory
// self-derived deadline. It is a security guard on the shipment ship path, so it
// FAILS CLOSED: any timeout, cancellation, exec failure, or non-{0,1} exit code
// returns a non-nil error (never a silent pass). Exit-code semantics:
//
//	0 -> ancestor or equal          -> (true, nil)
//	1 -> definitively not-ancestor  -> (false, nil)
//	other / exec error / timeout    -> (false, error)  [fail closed]
//
// argv-array exec + gate.MinimalEnv() preserve the workspace exec trust boundary
// (no shell, allowlisted env). Both operands are expected to already satisfy
// isGitObjectName / trusted-provenance at the call site.
func (ws *Workspace) isAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	runCtx, cancel := context.WithTimeout(ctx, ws.boundedHelperTimeout())
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, "git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = ws.RootPath
	cmd.Env = gate.MinimalEnv()
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr == nil {
		return true, nil // exit 0: ancestor or equal.
	}

	// A timeout OR cancellation MUST be detected before any exit code is read: a
	// context-killed git reports a platform-dependent exit code (e.g. 1 on
	// Windows, -1 on POSIX) that must never be misread as the exit-1
	// "not-an-ancestor" signal. Both DeadlineExceeded and Canceled fail closed.
	if ctxErr := runCtx.Err(); ctxErr != nil {
		return false, fmt.Errorf("ancestor check aborted: %w", ctxErr)
	}

	var ee *exec.ExitError
	if stderrors.As(runErr, &ee) {
		if ee.ExitCode() == 1 {
			return false, nil // definitively not an ancestor.
		}
		// Exit 128 (bad object / shallow boundary) and any other non-{0,1} code
		// are unverifiable lineage -> fail closed, preserving git's diagnostic.
		return false, fmt.Errorf("git merge-base --is-ancestor exit %d: %s: %w",
			ee.ExitCode(), bytes.TrimSpace(stderr.Bytes()), runErr)
	}
	// git binary missing or any other non-ExitError failure -> fail closed.
	return false, fmt.Errorf("run git merge-base --is-ancestor: %w", runErr)
}

// headSHABounded resolves the current HEAD SHA under a mandatory self-derived
// deadline (same source/fallback as isAncestor). The shipment ship path is
// unbounded, so a hung `git rev-parse` must not stall completion under the
// workspace lock. It distinguishes a bounded-read failure from a legacy
// resolution failure so the NEW timeout path fails closed without widening the
// pre-existing (FLAGGED) non-context empty-head skip:
//
//	("", ctxErr) -> the bounded read timed out or was cancelled: the caller MUST
//	                fail closed (never a silent staleness skip).
//	(sha, nil)   -> a real HEAD SHA.
//	("", nil)    -> a LEGACY non-context resolution failure (e.g. non-repo test
//	                harness): the pre-existing skip is preserved so no-repo tests
//	                do not regress.
func (ws *Workspace) headSHABounded(ctx context.Context) (string, error) {
	bctx, cancel := context.WithTimeout(ctx, ws.boundedHelperTimeout())
	defer cancel()
	h := ws.headSHA(bctx)
	if h == "" && bctx.Err() != nil {
		// Timeout/cancel: a bounded-read failure the caller fails closed on.
		return "", bctx.Err()
	}
	// A real SHA, or a legacy "" from a non-context resolution error (legacy skip).
	return h, nil
}

// inGitWorktreeBounded reports whether ws.RootPath resolves inside a real git
// work tree, under a mandatory self-derived deadline (same source/cap as
// isAncestor / headSHABounded). It is the repo-presence discriminator on the
// enforced shipment-gate empty-head path: ev.Enforced does NOT track work-tree
// presence (the test broker fakes the git probe), so an empty shipment/member
// head under enforcement must be distinguished between a real worktree (fail
// closed — lineage cannot be proven) and a genuine no-repo / non-autoharness
// environment (preserve the legacy skip). Being a security guard on the ship
// path it FAILS CLOSED: any timeout, cancellation, exec failure, corrupt-repo
// exit, or missing git returns a non-nil error. Exit semantics:
//
//	runCtx.Err() != nil (checked FIRST)                 -> (false, ctxErr) [fail closed]
//	exit 0, stdout "true"                                -> (true, nil)     [real worktree]
//	exit 0, stdout != "true" (bare repo / .git)          -> (false, nil)    [not-a-worktree skip]
//	exit != 0, `.git` at RootPath present OR its         -> (false, err)    [fail closed;
//	           presence indeterminate (stat != IsNotExist:                    message-independent]
//	           present-but-broken, or permission/IO)
//	exit 128, `.git` DEFINITIVELY absent (IsNotExist),   -> (false, nil)    [genuine no-repo skip]
//	           stderr ~ "not a git repository (or any of
//	           the parent directories)"
//	exit 128, `.git` absent, any OTHER stderr             -> (false, err)    [fail closed]
//	git missing / other non-ExitError / other exit        -> (false, err)    [fail closed]
//
// The `.git`-presence stat is the PRIMARY broken-repo discriminator (defends
// against git message/locale/version drift); the stderr marker only distinguishes
// a genuine "outside any repository" (no `.git` present) from other fatals.
// `--is-inside-work-tree` is chosen over `--git-dir`: the latter returns exit 0
// inside a bare repo and inside a `.git` dir, so it cannot distinguish "can
// resolve a work-tree HEAD" from "is under some git dir". argv-array exec +
// gate.MinimalEnv() + cmd.Dir = ws.RootPath preserve the workspace exec trust
// boundary (no shell, allowlisted env). The probe forces LC_ALL=C / LANG=C so the
// no-repo discriminator below matches git's stable ENGLISH diagnostic regardless
// of any inherited host locale. Parsing uses bytes (no strings import).
func (ws *Workspace) inGitWorktreeBounded(ctx context.Context) (bool, error) {
	runCtx, cancel := context.WithTimeout(ctx, ws.boundedHelperTimeout())
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = ws.RootPath
	// Force the C locale so git's diagnostics are the stable English form the
	// no-repo discriminator matches. MinimalEnv passes through any host LANG/LC_ALL,
	// so strip those and pin LC_ALL=C last (duplicate env keys are not reliably
	// last-wins across platforms, so the inherited value must be removed, not just
	// shadowed). A localized "not a git repository" would otherwise be misread.
	cmd.Env = withCLocale(gate.MinimalEnv())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr == nil {
		// exit 0: `true` means a real work tree; anything else (bare repo /
		// inside `.git`) is a not-a-worktree skip, not a real ship scenario.
		if bytes.Equal(bytes.TrimSpace(stdout.Bytes()), []byte("true")) {
			return true, nil
		}
		return false, nil
	}

	// A timeout OR cancellation MUST be detected before any exit code is read: a
	// context-killed git reports a platform-dependent exit code (e.g. 1 on
	// Windows) that must never be misread as a clean signal. Fail closed.
	if ctxErr := runCtx.Err(); ctxErr != nil {
		return false, fmt.Errorf("repo-presence probe aborted: %w", ctxErr)
	}

	var ee *exec.ExitError
	if stderrors.As(runErr, &ee) {
		// Message-INDEPENDENT broken-repo guard (primary discriminator): reaching
		// this branch means git did NOT resolve a work tree. If a `.git` entry
		// nonetheless exists at RootPath, it is a present-but-broken repo (an
		// empty/corrupt `.git` dir, or a broken gitfile pointer) and MUST fail
		// closed — regardless of git's diagnostic wording. This defends against
		// message/locale/git-version drift: a genuine "outside any repository" has
		// NO `.git` entry here (an ancestor repo would have made git return "true"
		// at exit 0, never reaching this branch). ONLY a definitive os.IsNotExist
		// (the `.git` entry is genuinely absent) may proceed to the no-repo marker
		// check; a successful stat (present) OR any other stat error (permission /
		// IO — presence indeterminate) fails closed, because for an ENFORCEMENT
		// discriminator "cannot rule out a present-but-broken repo" is fail-closed.
		if _, statErr := os.Stat(filepath.Join(ws.RootPath, ".git")); !os.IsNotExist(statErr) {
			return false, fmt.Errorf(
				"git rev-parse --is-inside-work-tree exit %d; .git at %s present-but-unresolved or presence indeterminate (stat: %v): %s: %w",
				ee.ExitCode(), ws.RootPath, statErr, bytes.TrimSpace(stderr.Bytes()), runErr)
		}
		// `.git` is definitively ABSENT at RootPath: ONLY git's stable "outside any
		// repository" marker — the parenthetical "(or any of the parent
		// directories)" — is a genuine no-repo skip. LC_ALL=C above guarantees this
		// English form. Any OTHER exit-128 stderr is an unexpected fatal and fails
		// closed.
		if ee.ExitCode() == 128 &&
			bytes.Contains(bytes.ToLower(stderr.Bytes()),
				[]byte("not a git repository (or any of the parent directories)")) {
			return false, nil // genuine no-repo skip preserved.
		}
		return false, fmt.Errorf("git rev-parse --is-inside-work-tree exit %d: %s: %w",
			ee.ExitCode(), bytes.TrimSpace(stderr.Bytes()), runErr)
	}
	// git binary missing or any other non-ExitError failure -> fail closed.
	return false, fmt.Errorf("run git rev-parse --is-inside-work-tree: %w", runErr)
}

// withCLocale returns env with any inherited LC_ALL / LANG entries removed and
// LC_ALL=C / LANG=C appended, so a git child emits deterministic English
// diagnostics for stderr-substring discrimination. It never mutates the input.
func withCLocale(env []string) []string {
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		if hasEnvKey(kv, "LC_ALL") || hasEnvKey(kv, "LANG") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "LC_ALL=C", "LANG=C")
}

// hasEnvKey reports whether a "KEY=VALUE" entry has the given key (exact,
// case-sensitive — env var names are case-sensitive on the POSIX targets that
// localize git output; Windows git honors LC_ALL/LANG in this same casing).
func hasEnvKey(kv, key string) bool {
	return len(kv) > len(key) && kv[len(key)] == '=' && kv[:len(key)] == key
}

// headResolveError builds a fail-closed refusal for a bounded HEAD resolution that
// timed out or was cancelled. A shipment head that cannot be read under the
// deadline must block completion rather than silently skip the staleness guard.
func headResolveError(shipmentID string, cause error) error {
	be := &blerrors.GateBlockedError{
		ItemID:       shipmentID,
		Outcome:      "blocked",
		OldStatus:    string(models.StatusActive),
		NewStatus:    string(models.StatusActive),
		StateChanged: false,
	}
	return fmt.Errorf("shipment %s refused: cannot resolve shipment head: %v: %w", shipmentID, cause, be)
}

// formalGateShipmentRefusal builds a refusal error for shipment completion
// when formal-gate-evidence admission is enforced but the broker
// infrastructure cannot supply what enforcement requires — either the broker
// is nil (disabled/unwired) or the current environment would otherwise fail
// open (106-F F1/U6). It deliberately wraps ONLY the plain ErrFormalGateRequired
// sentinel (not the typed GateError struct used by the setup/config/timeout/
// in_progress classes) so the MCP layer's formalGateErrorResult dispatch
// handles it with its own distinct error_type and remediation, rather than
// being intercepted by the pre-existing "gate_setup" class handling, whose
// remediation text ("install or repair the autoharness binary") would be
// misleading here.
func formalGateShipmentRefusal(shipmentID, reason string) error {
	return fmt.Errorf("shipment %s refused: %s: %w", shipmentID, reason, blerrors.ErrFormalGateRequired)
}

// formalGateMemberRefusal builds a refusal error for a shipment member whose
// gate evidence failed the FormalAdmit predicate, wrapping res.Err — the
// TYPED cause (ErrProofInvalid or ErrProofUnverifiable) FormalAdmit
// classified the refusal as — rather than the *GateBlockedError struct
// shipmentMemberEvidenceError uses for every OTHER member-scan refusal
// reason (missing evidence, stale lineage, malformed head). gateErrorResult
// dispatches GateBlockedError BEFORE the formal-gate sentinels, so wrapping
// THIS specific refusal in GateBlockedError would collapse it to the
// generic gate_blocked MCP error_type, discarding whether the cause was a
// tampered/replayed proof or one that could not be evaluated at all —
// defeating the specific formal_gate_proof_invalid/
// formal_gate_proof_unverifiable MCP contract U8 introduced for exactly
// this refusal family (106-F F1 review finding).
func formalGateMemberRefusal(memberID string, res gateevidence.FormalResult) error {
	cause := res.Err
	if cause == nil {
		cause = blerrors.ErrProofInvalid
	}
	return fmt.Errorf("shipment refused: member %s formal gate evidence proof did not verify: %s: %w", memberID, res.Reason, cause)
}

// shipmentHeadUnresolvedInRepoError builds a fail-closed refusal for an ENFORCED
// shipment whose HEAD resolves to empty INSIDE a real git work tree (1AEA2B0E).
// This is distinct from headResolveError (a bounded-read timeout/cancel): here the
// repo is present but HEAD is unresolvable for a non-context reason (an unborn
// branch, or a transient `git rev-parse HEAD` failure), so member lineage cannot be
// proven against a resolved shipment head and the ship must FAIL CLOSED rather than
// silently skip the member-lineage/drift guard. A dedicated constructor (Decision 3)
// gives operators a message assertably distinct from the timeout path, without
// shoe-horning a synthetic cause into headResolveError's `%v: %w` shape.
func shipmentHeadUnresolvedInRepoError(shipmentID string) error {
	be := &blerrors.GateBlockedError{
		ItemID:       shipmentID,
		Outcome:      "blocked",
		OldStatus:    string(models.StatusActive),
		NewStatus:    string(models.StatusActive),
		StateChanged: false,
	}
	return fmt.Errorf("shipment %s refused: cannot resolve shipment head in repository: %w", shipmentID, be)
}

// headDriftError reports whether HEAD advanced across the gate evaluation window.
// It returns nil when pre == post (no drift, including the both-empty no-repo
// case) and a typed *GateBlockedError naming both observed heads otherwise, so
// any HEAD movement between the single pre-resolution and the last read before
// success fails closed under enforcement.
func headDriftError(shipmentID, pre, post string) error {
	if pre == post {
		return nil
	}
	be := &blerrors.GateBlockedError{
		ItemID:       shipmentID,
		Outcome:      "blocked",
		OldStatus:    string(models.StatusActive),
		NewStatus:    string(models.StatusActive),
		StateChanged: false,
	}
	return fmt.Errorf("shipment %s refused: shipment head drifted during gate evaluation (%s -> %s): %w",
		shipmentID, pre, post, be)
}

// gateShipmentCompletion enforces the shipment-level two-level gate before a
// shipment is marked shipped (082-F ST4.2). It runs only when a gate broker is
// wired AND gates are enforceable; under enabled:false or a fail-open (auto with
// no usable autoharness / no resolvable base) it returns nil so the pre-gate ship
// behavior is preserved.
//
// Two independent checks, BOTH of which must pass:
//
//  1. member-evidence: every task/subtask member in the release scope MUST already
//     be terminal AND carry a passing (or forced) pre-task-completion gate
//     evidence event. This is the reconciliation guarantee — the ship path never
//     auto-completes an ungated member through completeReleaseScope, so release
//     finalization can never become a gate bypass.
//  2. shipment-diff: a shipment-level `autoharness gate check` over the full
//     shipment diff (no --task) must return a proceed decision.
//
// originalManifestItems is the shipment's declared manifest (NormalizeShipmentItems,
// deduplicated) as ShipShipment captured it BEFORE this function ran — the exact
// snapshot releaseScope was derived from. Under formal enforcement, it is
// re-checked against a fresh reload immediately before signing the manifest-binding
// proof (106-F F1 review finding F3): without that re-check, a concurrent, unlocked
// membership mutation (e.g. backlogit_add_to_shipment) landing after
// validateMemberGateEvidence already validated the ORIGINAL members, but before
// this function signs, would let the signed proof attest to a membership containing
// a member whose gate evidence was never checked at all.
//
// On refusal it returns a typed gate error and performs NO shipment state change.
func gateShipmentCompletion(ctx context.Context, ws *Workspace, shipmentID string, releaseScope, originalManifestItems []string) error {
	if ws == nil {
		return nil // no workspace at all; nothing to check or enforce.
	}
	if ws.GateBroker == nil {
		// Enumerated early-return #1 (106-F F1/U6): a nil broker means the
		// gate is disabled (enabled:false) or unwired. Under ordinary
		// (non-formal) operation this silently preserves pre-gate ship
		// behavior. But when formal gate evidence is enforced, silently
		// proceeding here would let a shipment ship with no enforceable gate
		// at all — refuse instead.
		if ws.formalGateEnforced() {
			return formalGateShipmentRefusal(shipmentID, "gate broker is not wired (disabled or unconfigured) but formal gate evidence is enforced")
		}
		return nil
	}

	// Resolve the shipment head ONCE, bounded, before Evaluate so the member
	// lineage check (#1) and the aggregate full-diff check (#2) are bracketed to a
	// single observed head. headErr is a bounded-read (timeout/cancel) failure that
	// must fail closed under enforcement; a legacy non-context "" preserves the
	// pre-existing no-repo skip (see headSHABounded). It is checked only after the
	// fail-open early return below, so a non-enforcing environment is never blocked
	// by a head-resolution timeout.
	shipmentHead, headErr := ws.headSHABounded(ctx)

	// Shipment-level aggregate gate check over the full diff. NoCount: this
	// aggregate invocation is advisory to autoharness's per-task failure counter
	// (the per-task completion path is authoritative; we never stack a second
	// breaker at the shipment level).
	ev, err := ws.GateBroker.Evaluate(ctx, gate.Request{
		WorkspaceRoot: ws.RootPath,
		NoCount:       true,
	})
	if err != nil {
		class := shipmentGateErrorClass(err)
		ws.appendGateErrorEvidence(ctx, shipmentID, class, err.Error(), nil, nil)
		ge := gateErrorFromClass(class, shipmentID, nil, nil)
		ge.Message = fmt.Sprintf("shipment %s gate check: %s", shipmentID, err.Error())
		return ge
	}
	if !ev.Enforced {
		// Enumerated early-return #2 (106-F F1/U6): gates are not enforceable
		// in this environment (auto fail-open) — ordinarily this silently
		// skips member-evidence/shipment-diff enforcement. Under formal gate
		// enforcement, a fail-open environment must not be allowed to ship
		// unauthenticated: refuse instead.
		if ws.formalGateEnforced() {
			return formalGateShipmentRefusal(shipmentID, "gates are not enforceable in this environment (auto fail-open) but formal gate evidence is required")
		}
		return nil
	}

	// Enforced: a bounded-read failure on the single pre-resolution (timeout or
	// cancel) MUST fail closed — never silently skip the staleness guard.
	if headErr != nil {
		return headResolveError(shipmentID, headErr)
	}

	// 1AEA2B0E: an EMPTY shipment head under enforcement is a LEGACY non-context
	// resolution failure (headSHABounded returned "" with headErr == nil, e.g. an
	// unborn branch or a transient `git rev-parse HEAD` failure). ev.Enforced does
	// NOT track work-tree presence — the test broker fakes the git probe, so
	// Enforced can be true in a no-repo temp dir — so a bounded repo-presence probe
	// is the discriminator:
	//   - real work tree  -> member lineage cannot be proven against a resolved head
	//                        -> FAIL CLOSED (closes the pre-existing fail-open hole).
	//   - probe error      -> cannot even determine repo presence under the deadline
	//                        -> FAIL CLOSED (consistent with isAncestor).
	//   - genuine no-repo  -> preserve the LEGACY skip (member scan + drift guard
	//                        remain inert). Production strict + genuinely-no-repo
	//                        already fails closed upstream at Evaluate/ResolveBaseRef,
	//                        so this skip only preserves the test harness + the
	//                        non-autoharness edge (see plan Enforcement-mode note).
	// Both fail-closed sub-cases emit an EventGateBlocked evidence event AND a
	// slog.WarnContext so the empty-shipment-head over-refusal monitoring signal is
	// real rather than a silent refusal (Constitution Principle V).
	if shipmentHead == "" {
		inRepo, probeErr := ws.inGitWorktreeBounded(ctx)
		if probeErr != nil || inRepo {
			slog.WarnContext(ctx, "shipment gate: empty shipment head under enforcement",
				"shipment", shipmentID, "in_repo", inRepo, "probe_error", probeErr)
			if aerr := ws.appendGateEvent(ctx, shipmentID, EventGateBlocked, map[string]any{
				"level":   "shipment",
				"outcome": "blocked",
				"reason":  "empty-shipment-head",
			}); aerr != nil {
				// Best-effort audit on a refusal path: the ship is already blocked,
				// so a failed evidence append must not mask the refusal below.
				slog.WarnContext(ctx, "shipment gate: failed to append blocked evidence",
					"shipment", shipmentID, "error", aerr)
			}
			if probeErr != nil {
				return headResolveError(shipmentID,
					fmt.Errorf("cannot determine repository presence: %w", probeErr))
			}
			return shipmentHeadUnresolvedInRepoError(shipmentID)
		}
		// !inRepo && probeErr == nil: genuine no-repo -> legacy skip preserved.
	}

	// (1) member-evidence validation (cheap log scan, no state change), against the
	// single pre-resolved shipment head.
	if merr := validateMemberGateEvidence(ctx, ws, releaseScope, shipmentHead); merr != nil {
		return merr
	}

	// (2) shipment-diff decision. Redirects have no meaning at the shipment level,
	// so every non-proceed, non-error decision collapses to a blocked refusal that
	// leaves shipment state unchanged.
	//
	// F5 (083.003-T): a setup/config/timeout-class DecisionError must preserve its
	// exit 7/8 class fidelity rather than collapsing to a GateBlockedError (exit 6).
	// This mirrors the task-level errorGate (gate_transition.go) and the broker
	// Evaluate-error branch above. A shipment-level timeout reaches here as
	// Kind==DecisionError with a nil Evaluate error, so this is the correct seam.
	if ev.Decision.Kind == gate.DecisionError {
		class := string(ev.Decision.ErrorClass)
		if class == "" {
			class = "config"
		}
		ws.appendGateErrorEvidence(ctx, shipmentID, class, "", ev.Decision.ReportJSON, ev.Decision.Stderr)
		ge := gateErrorFromClass(class, shipmentID, ev.Decision.ReportJSON, ev.Decision.Stderr)
		ge.Message = fmt.Sprintf("shipment %s gate check %s error", shipmentID, class)
		return ge
	}
	if ev.Decision.Kind != gate.DecisionProceed {
		be := &blerrors.GateBlockedError{
			ItemID:       shipmentID,
			OldStatus:    string(models.StatusActive),
			NewStatus:    string(models.StatusActive),
			Outcome:      "blocked",
			StateChanged: false,
			BaseRef:      ev.Base.Ref,
			HeadRef:      ev.HeadRef,
			ExitCode:     ev.Decision.ExitCode,
			ReportJSON:   ev.Decision.ReportJSON,
			Stderr:       ev.Decision.Stderr,
			Repeated:     toErrRepeated(ev.Decision.RepeatedFailure),
		}
		if aerr := ws.appendGateEvent(ctx, shipmentID, EventGateBlocked, map[string]any{
			"level":    "shipment",
			"outcome":  "blocked",
			"base_ref": ev.Base.Ref,
			"head_ref": ev.HeadRef,
		}); aerr != nil {
			// Best-effort audit on a refusal path: the ship is already blocked, so a
			// failed evidence append must not mask the GateBlockedError below.
			slog.WarnContext(ctx, "shipment gate: failed to append blocked evidence", "shipment", shipmentID, "error", aerr)
		}
		return fmt.Errorf("shipment %s blocked by shipment-level gate check: %w", shipmentID, be)
	}

	// Stable-head assertion — the LAST read before the success path. Re-resolving
	// here (after the block/error branches, before the passing-evidence append)
	// brackets the ENTIRE evaluation window: Evaluate (#2) plus the member scan
	// (#1). Any HEAD advance across that window, or a bounded-read timeout/cancel,
	// fails closed under enforcement, so ancestor-aware can never admit a member
	// whose old head became an ancestor of an advanced HEAD. When shipmentHead is a
	// legacy "" (no-repo), the guard is inert and existing no-repo tests are
	// unaffected.
	//
	// ev.Enforced is already guaranteed true here (the !ev.Enforced fail-open early
	// return above forecloses the false case); it is retained as an explicit
	// invariant marker so this stable-head assertion reads as unconditionally
	// scoped to the enforced path. shipmentHead != "" is the load-bearing guard
	// (no-repo legacy skip).
	if ev.Enforced && shipmentHead != "" {
		postHead, postErr := ws.headSHABounded(ctx)
		if postErr != nil {
			return headResolveError(shipmentID, postErr)
		}
		if derr := headDriftError(shipmentID, shipmentHead, postHead); derr != nil {
			return derr
		}
	}

	// Both checks passed: record shipment-level passing evidence.
	passDelta := map[string]any{
		"level":    "shipment",
		"outcome":  "passed",
		"base_ref": ev.Base.Ref,
		"head_ref": ev.HeadRef,
		"ran":      ev.Ran,
	}
	// Manifest binding (106-F F1/U7): bind the ordered manifest membership,
	// covering feature, and resolved shipment head into a purpose=shipment
	// proof, additive to the existing head_sha ancestry and head-drift guards
	// above (both preserved unchanged). A nil-safe best-effort GetShipment
	// lookup failure is treated as ErrFormalGateRequired under enforcement
	// (cannot bind a manifest we cannot read) rather than silently skipping
	// the binding.
	if ws.formalGateEnforced() {
		shipment, getErr := GetShipment(ctx, ws, shipmentID)
		if getErr != nil {
			return fmt.Errorf("%w: resolve shipment for manifest binding: %v", blerrors.ErrFormalGateRequired, getErr)
		}
		// TOCTOU guard (106-F F1 review finding F3): validateMemberGateEvidence
		// (above) already validated every member of originalManifestItems — the
		// manifest snapshot ShipShipment captured BEFORE this function ran. This
		// re-reloaded shipment could have gained (or lost) a member via a
		// concurrent, unlocked backlogit_add_to_shipment call in the window
		// between that snapshot and this reload. Without this check, the
		// signed manifest-binding proof would attest to the FRESH membership —
		// including a member whose gate evidence was NEVER validated — while
		// the signature falsely implies otherwise. Refuse (fail closed) rather
		// than silently signing a manifest that has drifted from the one whose
		// members were actually checked.
		currentManifestItems := uniqueNonEmptyStrings(NormalizeShipmentItems(shipment))
		if !manifestItemsUnchanged(originalManifestItems, currentManifestItems) {
			return formalGateShipmentRefusal(shipmentID, "shipment manifest membership changed after evidence validation and before signing (concurrent modification) — refusing to bind a proof to unvalidated members")
		}
		unlock, aerr := ws.augmentShipmentDeltaWithFormalProof(ctx, shipment, shipmentID, shipmentHead, passDelta)
		if aerr != nil {
			return aerr
		}
		// unlock MUST be deferred here (covering the manifest self-check and the
		// real append below), not inside augmentShipmentDeltaWithFormalProof —
		// see that function's doc comment for why releasing the counter lock any
		// earlier reopens the counter-uniqueness TOCTOU (106-F F1 review finding).
		defer unlock()
		if verr := ws.verifyShipmentManifestBinding(ctx, shipment, shipmentID, shipmentHead, passDelta); verr != nil {
			return fmt.Errorf("shipment %s manifest binding verification failed, refusing ship: %w", shipmentID, verr)
		}
	}
	if aerr := ws.appendGateEvent(ctx, shipmentID, EventGatePassed, passDelta); aerr != nil && ws.gateConfig.EvidenceRequiredValue() {
		return fmt.Errorf("shipment %s gate evidence append failed, refusing ship: %w", shipmentID, aerr)
	}
	return nil
}

// manifestItemsUnchanged reports whether current is IDENTICAL to original —
// same length, same values, same order. Any difference (an added member, a
// removed member, or even a reorder) is treated as a change: computeManifestDigest
// itself treats manifest order as semantically significant (reordering members
// changes the digest), so this comparison is deliberately exact rather than a
// set-equality check, keeping it consistent with what the signed digest actually
// commits to (106-F F1 review finding F3).
func manifestItemsUnchanged(original, current []string) bool {
	if len(original) != len(current) {
		return false
	}
	for i := range original {
		if original[i] != current[i] {
			return false
		}
	}
	return true
}

// validateMemberGateEvidence verifies every task/subtask member in the release
// scope is terminal AND carries passing (or forced) gate evidence. When
// shipmentHead is non-empty, the member's recorded evidence head must be an
// ANCESTOR OF (or equal to) that shipment head — i.e. the gated commit is
// contained in the shipment history (verified with git merge-base
// --is-ancestor). This replaces the prior strict head_sha equality, which falsely
// rejected valid post-merge evidence (a member's build commit is an ancestor of,
// not equal to, the shipment's merge commit). A genuinely divergent (non-ancestor)
// head, a malformed head_sha, an unverifiable lineage (git error/timeout/cancel),
// or — as of 085-F (B85DAEE8) — an EMPTY member head is refused (fail closed).
// Non-gated member types (feature/other) are skipped.
//
// Empty-member-head fail-closed invariant (085-F): the empty-head refusal executes
// ONLY inside the `shipmentHead != ""` block. A non-empty shipmentHead is produced
// solely by headSHABounded resolving a real HEAD, which itself proves a real work
// tree with a committed HEAD — so the empty-member-head refusal can never fire in a
// no-repo / unresolved-head context (there shipmentHead == "" and this block is
// skipped entirely). inGitWorktreeBounded is the discriminator on the *empty*
// shipmentHead branch in gateShipmentCompletion, NOT a precondition of this
// function. A future caller that passes a non-empty shipmentHead NOT obtained from
// a resolved HEAD would break this invariant.
func validateMemberGateEvidence(ctx context.Context, ws *Workspace, releaseScope []string, shipmentHead string) error {
	logsRoot := WorkspaceLogsRoot(ws.RootPath)

	// 106-F F1/U6: when formal gate evidence is enforced, EVERY member's own
	// gate-pass evidence must additionally be formally admissible —
	// gateevidence.Latest alone (used below to select the candidate and its
	// head_sha for the pre-existing lineage check) never verifies a proof, so
	// a hand-authored or replayed JSONL record would otherwise satisfy ship-time
	// reconciliation identically to genuine evidence, defeating the shipment's
	// authenticity guarantee at exactly the point (ship time, per member) it
	// matters most. The key is resolved ONCE, outside the loop, so a missing/
	// invalid key refuses immediately rather than partway through the scan.
	enforced := ws.formalGateEnforced()
	var formalKey []byte
	if enforced {
		key, keyErr := config.ResolveFormalGateKey()
		if keyErr != nil {
			return wrapFormalGateRequired(keyErr)
		}
		formalKey = key
	}

	for _, id := range releaseScope {
		item, err := loadArtifact(ctx, ws, id)
		if err != nil {
			return fmt.Errorf("validate member evidence: load %s: %w", id, err)
		}
		if item.ArtifactType != "task" && item.ArtifactType != "subtask" {
			continue
		}
		// A non-terminal gated member has not been completed through the gate;
		// shipping MUST NOT silently auto-complete it (release finalization is not
		// a gate bypass).
		if !isTerminalReleaseStatus(item.Status) {
			return shipmentMemberEvidenceError(id, fmt.Sprintf("is %s (not completed through the gate)", item.Status))
		}
		evs, rerr := events.ReadAllEvents(ctx, logsRoot, id)
		if rerr != nil {
			return fmt.Errorf("validate member evidence: read events for %s: %w", id, rerr)
		}
		latest := latestGatePassEvidence(evs)
		if latest == nil {
			// A GENUINELY DESCOPED member (archived directly from a
			// DESCOPE-ELIGIBLE status — an in-flight status or a non-completion
			// terminal) was taken out of the release rather than completed
			// through the gate, so it carries no per-member evidence and MUST NOT block
			// the shipment. releaseScopeItemIDs expands a feature to ALL descendants
			// (IncludeArchived: true), so a task scaffolded-then-descoped lands in the
			// release scope even when the shipment manifest excludes it; demanding
			// evidence for it would permanently block the parent feature's ship with no
			// operator recourse (archived is a terminal sink with no allowed status
			// transitions, so it cannot be force-gated).
			//
			// The exemption is deliberately narrow: it applies ONLY when the member was
			// archived from a DESCOPE-ELIGIBLE status — an in-flight status (queued,
			// active, blocked, review) or a non-completion terminal (abandoned,
			// rejected). ArchiveItem accepts completed items too and preserves the
			// pre-archive status in archived_status, so a member driven to a COMPLETION
			// status (done/accepted/shipped) with NO valid evidence — e.g. whose only
			// "pass" is a fail-open EventGatePassed{ran:false} rejected by the F4
			// predicate — and then archived MUST still refuse; exempting it on the bare
			// archived status would bypass that predicate. archived_status is read from
			// the Markdown source because the DB-backed loadArtifact omits it; a
			// missing/empty archived_status fails closed (not a proven descope). The
			// shipment-level aggregate diff gate still covers the full shipment diff.
			if item.Status == models.StatusArchived {
				descoped, derr := archivedFromDescopeEligibleStatus(ctx, ws, id)
				if derr != nil {
					return fmt.Errorf("validate member evidence: %s: %w", id, derr)
				}
				if descoped {
					continue
				}
			}
			return shipmentMemberEvidenceError(id, "missing passing gate evidence")
		}
		if enforced {
			// FormalAdmit is deliberately stricter than Latest: it verifies the
			// HMAC proof against ctx.Key/WorkspaceID/ItemID, refuses a
			// forced-only history, and enforces counter monotonicity. A member
			// whose only qualifying evidence is a plain (Latest-only) pass or a
			// tampered/replayed proof is refused here even though the lineage
			// check below would otherwise have accepted it.
			res := gateevidence.FormalAdmit(evs, gateevidence.FormalContext{
				WorkspaceID: workspaceIdentity(ws.RootPath),
				ItemID:      id,
				Key:         formalKey,
			})
			if !res.Admitted {
				return formalGateMemberRefusal(id, res)
			}
		}
		if shipmentHead != "" {
			h, _ := latest.Delta["head_sha"].(string)
			if h == "" {
				// B85DAEE8: an EMPTY member head under enforcement FAILS CLOSED. A
				// non-empty shipmentHead proves headSHABounded resolved a real HEAD
				// (a real work tree), so a member with no recorded head_sha cannot
				// prove its gated commit is contained in the shipment history —
				// shipping it would be an unverifiable-lineage bypass. Emit an
				// EventGateBlocked evidence event AND a slog.WarnContext so the
				// empty-member-head over-refusal monitoring signal is real (not a
				// silent refusal), mirroring the ST2 shipment-level emission
				// (Constitution Principle V).
				slog.WarnContext(ctx, "member evidence has no recorded head_sha",
					"member", id, "shipment_head", shipmentHead)
				if aerr := ws.appendGateEvent(ctx, id, EventGateBlocked, map[string]any{
					"level":   "member",
					"outcome": "blocked",
					"reason":  "empty-member-head",
					"member":  id,
				}); aerr != nil {
					// Best-effort audit on a refusal path: the ship is already
					// blocked, so a failed append must not mask the refusal below.
					slog.WarnContext(ctx, "shipment gate: failed to append blocked evidence",
						"member", id, "error", aerr)
				}
				return shipmentMemberEvidenceError(id,
					"gate evidence has no recorded head_sha (cannot verify lineage under enforcement)")
			}
			if h != shipmentHead {
				// h == shipmentHead is the equality fast-path: an equal head never
				// enters this block, so a single-commit shipment needs no repo access
				// and no subprocess.
				if !isGitObjectName(h) {
					// The recorded head comes from tamperable on-disk evidence JSONL;
					// a value that is not a git object name is never handed to git.
					slog.WarnContext(ctx, "member evidence head_sha is malformed",
						"member", id, "member_head", h, "shipment_head", shipmentHead)
					return shipmentMemberEvidenceError(id,
						"gate evidence head_sha is malformed (not a git object name)")
				}
				included, aerr := ws.isAncestor(ctx, h, shipmentHead)
				if aerr != nil {
					// A security guard must never silently pass on an unverifiable
					// lineage (git error / timeout / cancel): fail closed.
					slog.WarnContext(ctx, "member evidence lineage check failed",
						"member", id, "member_head", h, "shipment_head", shipmentHead, "error", aerr)
					return shipmentMemberEvidenceError(id,
						fmt.Sprintf("cannot verify gate evidence lineage: %v", aerr))
				}
				if !included {
					// A real, reachable, but non-ancestor head: the gated work is not
					// contained in the shipment history — genuinely stale/divergent.
					return shipmentMemberEvidenceError(id,
						"gate evidence is stale (recorded at a divergent head)")
				}
			}
		}
	}
	return nil
}

// archivedFromDescopeEligibleStatus reports whether the artifact identified by id was
// archived from a DESCOPE-ELIGIBLE status — an in-flight status (queued/active/blocked/
// review) or a non-completion terminal (abandoned/rejected) — i.e. it is a GENUINE
// DESCOPE (removed from the release before shipping a deliverable) rather than a
// completed-then-archived deliverable. It reads archived_status directly from the
// Markdown source because the DB-backed loadArtifact projection omits that field.
//
// A member archived from a COMPLETION status (done/accepted/shipped) is NOT a descope:
// exempting such a member from the per-member gate-evidence requirement would bypass
// the F4 fail-open evidence predicate (a completed member whose only "pass" is an
// EventGatePassed{ran:false} carries no valid evidence yet could be archived after the
// fact). A member archived from a status that the gate is CONFIGURED to run at
// (terminal_statuses, consulted via isGateTerminalStatus) is likewise NOT a descope —
// reaching a configured gate terminal status means valid evidence is expected, so the
// exemption is disabled for it even when it is statically descope-eligible (e.g. a
// workspace that lists "rejected" in terminal_statuses). The `archived` sink is itself
// an accepted terminal_statuses value, and ArchiveItem writes it without the gated path,
// so when `archived` is configured as a gate terminal the exemption is disabled for ALL
// provenance statuses (archival is expected to carry evidence). A missing/empty archived_status
// fails closed (reported as NOT a descope) because the provenance cannot prove the member
// was removed before completion. An UNRECOGNIZED archived_status (a typo or a malformed
// value a future serializer bug might emit) also fails closed: isDescopeEligibleStatus
// returns false for any unknown value, and the explicit isRecognizedReleaseStatus guard
// rejects garbage provenance so it cannot be misclassified as a proven descope. Only a
// RECOGNIZED, descope-eligible status that is NOT a configured gate terminal status (and
// only when `archived` itself is not gate-terminal) is a proven descope.
func archivedFromDescopeEligibleStatus(ctx context.Context, ws *Workspace, id string) (bool, error) {
	path, err := FindArtifactPath(ctx, ws, id)
	if err != nil {
		return false, fmt.Errorf("resolve archived member: %w", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read archived member: %w", err)
	}
	fm, _, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return false, fmt.Errorf("parse archived member: %w", err)
	}
	archivedStatus, _ := fm["archived_status"].(string)
	status := models.ArtifactStatus(archivedStatus)
	// Fail closed on absent OR unrecognized provenance: an empty archived_status
	// cannot prove a descope, and a malformed/typo value must not be misclassified as
	// a descope. Only a recognized, descope-eligible status is exempt.
	if archivedStatus == "" || !isRecognizedReleaseStatus(status) {
		return false, nil
	}
	if !isDescopeEligibleStatus(status) {
		return false, nil
	}
	// Config-aware guard: a status the gate is CONFIGURED to run at (terminal_statuses)
	// is NOT a descope even when it is statically descope-eligible. gateApplies runs the
	// pre-task-completion gate whenever a task enters a configured terminal status, so
	// reaching such a status means valid F4 evidence is expected. A member archived from
	// a configured gate terminal status without evidence must still refuse — otherwise a
	// workspace that lists e.g. "rejected" in terminal_statuses could turn missing F4
	// evidence into an exemption and bypass the gate.
	//
	// The `archived` SINK status is itself a valid terminal_statuses value (the config
	// schema's gateKnownStatuses accepts it). ArchiveItem writes status=archived directly
	// without routing through the gated-completion path, so when an operator configures
	// `archived` as a gate terminal status they intend archival to require valid evidence.
	// The exemption must therefore also be suppressed whenever `archived` is a configured
	// gate terminal status, independent of the provenance status — otherwise archiving a
	// descope-eligible member would bypass that configured archival gate contract.
	//
	// Under the default terminal_statuses (["done"]) neither any descope-eligible status
	// nor the `archived` sink is a gate terminal status, so the common-case exemption is
	// unchanged.
	if ws.isGateTerminalStatus(string(status)) || ws.isGateTerminalStatus(string(models.StatusArchived)) {
		return false, nil
	}
	return true, nil
}

// latestGatePassEvidence returns the most recent gate evidence event that
// satisfies the composed member-evidence predicate (082-F F4 hardening,
// 083.002-T). As of Q3.0 (083.005.001-ST) the predicate is owned by the shared
// internal/gateevidence leaf so core and db derive evidence identically across
// the one-way core->db boundary; this wrapper delegates and returns the selected
// event (nil when no qualifying event is present) to preserve the existing
// caller contract (nil-check + head_sha staleness read).
func latestGatePassEvidence(evs []events.Event) *events.Event {
	return gateevidence.Latest(evs).Event
}

// shipmentMemberEvidenceError builds a typed blocked refusal for a member that
// cannot be released, wrapping the member context so callers route via errors.As
// while surfacing the offending member and reason.
func shipmentMemberEvidenceError(memberID, reason string) error {
	be := &blerrors.GateBlockedError{
		ItemID:       memberID,
		Outcome:      "blocked",
		OldStatus:    string(models.StatusActive),
		NewStatus:    string(models.StatusActive),
		StateChanged: false,
	}
	return fmt.Errorf("shipment refused: member %s %s: %w", memberID, reason, be)
}

// shipmentGateErrorClass classifies a broker Evaluate error (probe / base-ref
// resolution) into a gate error class for evidence and typed-error construction.
func shipmentGateErrorClass(err error) string {
	var ge *blerrors.GateError
	if stderrors.As(err, &ge) {
		return ge.Class
	}
	switch {
	case stderrors.Is(err, blerrors.ErrGateSetup):
		return "setup"
	case stderrors.Is(err, blerrors.ErrGateTimeout):
		return "timeout"
	default:
		return "config"
	}
}
