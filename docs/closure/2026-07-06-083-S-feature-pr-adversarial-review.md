# Adversarial Review — 083-S Feature PR (pre-push)

**Date:** 2026-07-06
**Shipment:** 083-S — Gate-Broker Phase-2 Hardening
**Branch:** `feat/gate-broker-phase2-hardening`
**Gate:** Pre-push multi-model adversarial review (operator-mandated for every PR)
**Scope reviewed:** `git diff be8d93f..HEAD -- internal/ cmd/` — 20 files, 1269 insertions

## Method

Three independent reviewers on three different models, each adversarial and
instructed to verify against real symbols (no guessing):

| Reviewer | Lens | Model | Verdict |
|---|---|---|---|
| A | Go correctness & safety | claude-opus-4.8 (high) | PASS |
| B | Logic & design-decision scrutiny | gpt-5.4 (high) | **BLOCK** (2× P1) |
| C | Architecture, layering & SQL/data-model | gemini-3.1-pro (high) | PASS |

The Q3.2 positive-index layering deviation (rehydration writes projection rows
only for items with ≥1 gate event, vs the plan's literal "store missing for all
terminal items") was explicitly surfaced to all three reviewers for scrutiny.

## Findings by confidence tier + disposition

### HIGH confidence — P1 (gate-blocking) — ALL REMEDIATED

1. **[P1 HIGH · Reviewer B] Doctor trusted a stale projection `missing` row over authoritative logs.**
   Scenario: item gated+blocked → `sync` writes `missing` row → item later completed
   through the gate (log now has a pass) with no intervening `sync` → doctor read the
   stale `missing` row and flagged the item, ignoring the passing logs. Violates "logs
   remain source of truth."
   **Disposition: FIXED.** Item logs are append-only, so a `passed`/`forced`/`forced_no_run`
   row can never be stale-wrong, but a `missing` row can. `LoadGateEvidence` →
   `LoadPassingGateEvidence` now loads only positive-evidence rows via
   `gate_status IN ('passed','forced','forced_no_run')`; `gateEvidenceMissing`
   re-verifies every non-positive/absent item against the authoritative log-scan.
   Test `TestDoctorGateEvidence_StaleMissingRowLogsWin` pins the corrected contract.

2. **[P1 HIGH · Reviewer B] F5 fix did not reach the `shipment ship` CLI exit code.**
   `newShipmentShipCmd` wrapped the typed `*GateError` with `fmt.Errorf("ship shipment: %w")`,
   so `main`'s `ExitCodeFor` collapsed shipment-completion gate refusals to exit 1 instead
   of the versioned 6/7/8.
   **Disposition: FIXED.** Added `shipmentShipGateError` (mirrors `moveGateError`) to map
   the refusal to `*ExitError{6|7|8}`. Test `TestShipmentShipGateError_PreservesExitCodes`.

### HIGH confidence — P2/P3 (non-blocking)

3. **[P2 HIGH · Reviewer C] `idx_gate_evidence_status` was unused** (LoadGateEvidence did a
   full table scan). **Disposition: FIXED as a side effect of finding #1** — the new
   `WHERE gate_status IN (...)` predicate is served by the index.

4. **[P3 HIGH · Reviewer C] `rehydrateGateEvidence` swallowed INSERT errors and committed a
   partial projection.** **Disposition: FIXED.** The loop now returns the error so the
   deferred `Rollback` aborts; the disposable read-model is rebuilt in full on next sync.

5. **[P3 HIGH · Reviewer A] `retryable` omitempty makes `retryable:false` indistinguishable
   from absent.** **Disposition: ACCEPTED (intentional/spec-conformant)** — matches the
   `*GateBlockedError` payload shape; golden tests
   `TestRenderGateErrorJSON_ConfigOmitsRetryable`/`_TimeoutPresent` already assert the
   contract (config omits the key; timeout emits `retryable:true`).

### MEDIUM confidence — P2/P3

6. **[P2 MEDIUM · Reviewer A] Member-evidence staleness check is fail-open when a non-forced
   pass has an empty `head_sha`** (`shipment_gate.go:152-156`). Pre-existing behavior, not
   introduced by 083-S. **Disposition: DEFERRED follow-up** (anti-replay hardening; stashed
   for Stage triage — out of the 083-S hardening scope).

7. **[P3 MEDIUM · Reviewer A] Empty-diff legitimate pass could be reclassified as missing if
   it carried `ran=false`.** **Disposition: VERIFIED — NOT a regression.** Confirmed via
   `internal/core/gate/broker.go:32-34,131`: `Ran` reports whether the gate *process
   executed*, not diff-emptiness. A real pass (enforced, or auto with the binary present)
   sets `ran=true` regardless of an empty diff; only a genuine fail-open (binary missing
   under auto) yields `ran=false`, which F4 is *designed* to reject.

8. **[P3 MEDIUM · Reviewer A] Divergent malformed-line handling** — `parseItemLogFile`
   errors on a bad JSON line (projection skips the file) while `events.ReadAllEvents` skips
   bad lines (fallback still parses). Self-healing via the fallback; **Disposition: DEFERRED
   follow-up** (unify the two parsers for byte-for-byte consistency).

### LOW confidence — P3

9. **[P3 LOW · Reviewer A] `Latest` selects by slice/append order, not `Timestamp`.**
   **Disposition: ACCEPTED** — relies on the append-only JSONL invariant consistent with the
   rest of the codebase; documented on the predicate.

## Deviation verdicts

- **Q3.2 positive-index deviation** — Reviewer B: the deviation itself does **not** create the
  hypothesized false negative for a never-gated terminal item (absent-row fallback +
  `ReadAllEvents` returning `(nil,nil)` on a missing log still flags it correctly);
  `forced_no_run` is produced and consumed correctly end-to-end. The only unsafe part was the
  Q3.3 stale-row trust (finding #1), now fixed. Reviewer C: layering instinct correct
  (decouples db from `gateConfig.TerminalStatuses`); leaf package `internal/gateevidence` is
  **SOUND** (imports only `internal/events`, no cycle, single source of truth via aliasing).
  Reviewer C's suggested "store missing for all as a superset" is **incompatible** with
  finding #1's correctness fix (a missing row must never be trusted), so it was not adopted;
  the log-scan for non-positive/legacy items is the required correctness cost of "logs are
  source of truth," acceptable for an advisory audit.

## Consensus outcome

All HIGH-confidence P0/P1 gate-blockers remediated before push. Non-blocking findings are
either fixed opportunistically (#3, #4), verified not-a-bug (#7), accepted with rationale
(#5, #9), or deferred as follow-up stash for Stage (#6, #8). Post-remediation gates:
`go test ./...` green, `go vet` 0, `golangci-lint` 0, `gofmt` clean; projection rebuilds
idempotently on sync (9 passed rows).

**GATE: PASS (post-remediation).** Cleared to push and open the feature PR.

## Deferred follow-ups to stash (for Stage)

- Member-evidence staleness: treat an empty `head_sha` on a non-forced pass as
  unverifiable/stale rather than silently accepting (finding #6). Source: this artifact.
- Unify `parseItemLogFile` malformed-line handling with `events.ReadAllEvents`
  skip-and-continue so the projection and the doctor fallback agree byte-for-byte
  (finding #8). Source: this artifact.
