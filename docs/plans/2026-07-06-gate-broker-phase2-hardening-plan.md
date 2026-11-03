---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for the pre-task-completion gate-broker phase-2 hardening covering feature: four mutually-independent single-file test-first remediations (F1 advisory dual-base warning in baseref.go; F4 shipment member-evidence composed ran==true predicate in shipment_gate.go; F5 shipment DecisionError class fidelity in shipment_gate.go; F7 move --json GateError payload in gate_exit.go/move.go) plus one larger DB read-model task (Q3 derived indexed gate-evidence projection) decomposed into four ordered test-first subtasks (Q3.0 shared leaf predicate, Q3.1 gate_evidence table, Q3.2 sync-population, Q3.3 doctor indexed-query with log-scan fallback). Incorporates plan-review adjudications (single-source predicate across the one-way core->db boundary, dedicated projection table, forced_no_run visibility, advisory-derived-only marker). Includes a plan-hardening section for the Q3 read-model (rebuild/idempotency/backward-compat/rollback).'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-06-gate-broker-phase2-hardening-plan.md
title: 'Pre-task-completion gate broker — phase-2 hardening implementation plan'
---

# Pre-task-completion gate broker — phase-2 hardening implementation plan

Source documents (authoritative):

- `docs/decisions/2026-07-06-gate-broker-phase2-hardening-deliberation.md` — this bundle's
  deliberation: coherence verdict, covering-feature name, per-finding direction, and the two
  load-bearing design decisions (F4 force semantics, F1 advisory-only surface).
- `docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md` — the 082-F
  adversarial review; findings F1/F4/F5/F7 with exact file:line and remediation guidance.
- `docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md` — locked design; §Resolved open
  questions #3 sanctions the Q3 read-model follow-up (`7ED9CE1A`).
- `docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md` — durable integration
  contract; records the F4 `ran=false` gap as deferred and the base-ref config-first precedence.

Source stash entries (traceability): `162F5548` (F1), `9822F787` (F4), `7C5EADA6` (F5),
`83B885EE` (F7), `7ED9CE1A` (Q3).

**Requires plan hardening: yes** — the Q3 subtasks (Q3.1 schema, Q3.2 sync-population) mutate the
SQLite cache schema and the rehydration pipeline; the `## Plan Hardening` section below covers
rebuild/idempotency/backward-compat/rollback. The four F1/F4/F5/F7 remediations are low-risk
single-file changes and do not individually require hardening.

## Constitution alignment

- **II. Test-First (NON-NEGOTIABLE):** every task/subtask writes a failing (red) test proving the
  current gap or the missing projection **before** the fix, then makes it green. No standalone
  "tests" task — tests are colocated `*_test.go` in the same package as each change.
- **III/IV. Workspace isolation / CLI containment:** all edits are within-tree Go files under
  `internal/`; no out-of-tree writes. Stage authors only planning artifacts (P-010) — Ship
  implements.
- **VI. Single Responsibility:** each task is one skill domain (code) touching one primary file;
  no new external dependencies (all changes reuse existing helpers/patterns).
- **IX. Git-Friendly Persistence:** Q3 is a derived, disposable projection rebuilt on `sync`; item
  logs remain the append-only source of truth, so completion writes stay merge-conflict-free.

## Task breakdown (maps 1:1 to the harvest hierarchy)

### Task T-F1 — Advisory warning when both config `base_ref` and `--gate-base` are set
- **Source:** stash `162F5548` (adversarial F1). **File:** `internal/core/gate/baseref.go`
  (+ caller in `internal/core/gate_transition.go` for the user-facing/log surface).
- **Behavior (unchanged precedence):** config-first precedence is preserved exactly
  (`ConfigBaseRef` non-auto still wins). Add a **structured signal** to `ResolvedBase` (per
  Decision 1: e.g. `OverrideShadowed bool` set true iff a non-auto `ConfigBaseRef` **and** a
  non-empty `GateBase` are both supplied), and have the core caller emit an advisory warning that
  the operator's `--gate-base` is shadowed by the pinned config `base_ref`. Advisory only — NO
  behavior/precedence change.
- **Test-first:** table test in `internal/core/gate/baseref_test.go` asserting the signal is set
  iff both inputs are present (and NOT set when only one, or when config is `auto`/empty), and that
  the resolved `Ref`/`Source` remain config-first in all cases. Optional core-side test that the
  warning is emitted.
- **Acceptance criteria:**
  1. When a non-auto config `base_ref` and a non-empty `--gate-base` are both supplied, the
     resolver signals the shadowed override (and the core layer warns), while the resolved base is
     still the config value (`Source == "config"`).
  2. No warning/signal when only one of the two is supplied, or when config is `auto`/empty.
  3. Precedence and resolved `Ref` are byte-for-byte unchanged from current behavior in all
     existing `baseref_test.go` cases (no regression).
- **Posture:** test-first. **Independent** of F4/F5/F7/Q3.

### Task T-F4 — Shipment member-evidence rejects fail-open `ran=false` passes
- **Source:** stash `9822F787` (adversarial F4). **File:** `internal/core/shipment_gate.go`.
- **Behavior:** in `latestGatePassEvidence` (shipment_gate.go:146-155) — or its caller
  `validateMemberGateEvidence` — apply the **composed predicate**: an event counts as valid member
  gate evidence iff `EventType == EventGateForced` **OR** (`EventType == EventGatePassed` **AND**
  `ran == true`). Per Decision 2, `EventGateForced` acceptance is unconditional (audited
  break-glass). A member whose only pass is a fail-open `ran=false` `EventGatePassed` no-run is then
  treated as missing evidence and the shipment is refused with the existing
  `shipmentMemberEvidenceError` typed block.
- **Go correctness (Go persona P1):** read the boolean with the comma-ok assertion —
  `ran, _ := ev.Delta["ran"].(bool)` — never a bare `ev.Delta["ran"].(bool)` (which panics on a
  missing key or non-bool). Comma-ok yields `false` for missing/non-bool → correctly treated as
  not-ran. Mirror the existing `head_sha` idiom at shipment_gate.go:136. (No `float64` pitfall:
  `encoding/json` decodes a JSON bool into a Go `bool` for `any` targets, and `ran` is always
  written as a Go bool at gate_transition.go:396 / shipment_gate.go:99.)
- **Selection-order note (Go persona P2 / SecLens P2):** filtering `EventGatePassed{ran:false}`
  out of the "latest" selection can promote an earlier `EventGateForced` to the returned latest
  event, which then drives the `head_sha` staleness check (shipment_gate.go:135-138) against the
  forced event's head. This is **intended** (forced evidence is unconditional per Decision 2).
- **Test-first:** in `internal/core/shipment_gate_test.go`, add a red test: a terminal member with
  a single `EventGatePassed{ran:false}` currently passes the member scan; assert it must be
  refused (`shipmentMemberEvidenceError`). Add positive cases: `EventGatePassed{ran:true}` passes;
  `EventGateForced{ran:false}` still passes (force preserved); and an **interleaved** case
  `[EventGateForced, EventGatePassed{ran:false}]` to pin the composed-predicate selection semantics
  (must still pass via the forced event, not be masked/rejected ambiguously).
- **Acceptance criteria:**
  1. A member whose latest pass evidence is `EventGatePassed` with `ran != true` (or missing/non-bool
     `ran`) is rejected as missing/insufficient gate evidence.
  2. A member with `EventGatePassed{ran:true}` still passes.
  3. A member with `EventGateForced` (regardless of `ran`) still passes (force semantics intact).
  4. The interleaved `[Forced, Passed{ran:false}]` fixture passes via the forced event (no panic,
     deterministic selection).
  5. The shared `doctor --check-gate-evidence` advisory reflects the same predicate (no crash;
     consistency verified).
- **Posture:** test-first. **Independent** of F1/F5/F7 and of the Q3.1/Q3.3 subtasks. (Q3.0 extracts
  this finalized predicate into a shared leaf helper — see Decision below — so Q3.0 blocks-on F4.)

### Task T-F5 — Shipment `DecisionError` preserves exit 7/8 class fidelity
- **Source:** stash `7C5EADA6` (adversarial F5). **File:** `internal/core/shipment_gate.go`.
- **Behavior:** in `gateShipmentCompletion`, before the block branch (shipment_gate.go:66), handle
  `ev.Decision.Kind == gate.DecisionError` separately: derive the class as
  `class := string(ev.Decision.ErrorClass)` (a defined `gate.ErrorClass` string type; default to
  `"config"` when empty — mirroring `errorGate` exactly), call the existing
  `ws.appendGateErrorEvidence(ctx, shipmentID, class, "", ev.Decision.ReportJSON, ev.Decision.Stderr)`,
  and return `gateErrorFromClass(class, shipmentID, ev.Decision.ReportJSON, ev.Decision.Stderr)`
  (optionally set `.Message`). This mirrors the task-level `errorGate` (gate_transition.go:288-298)
  and the identical routing already present in this file's broker-`Evaluate`-error branch
  (shipment_gate.go:46-50). Non-error non-proceed decisions still collapse to `GateBlockedError`
  (exit 6) unchanged. (Confirmed: a shipment-level timeout reaches line 66 as
  `Kind==DecisionError` with a **nil** `Evaluate` error, so this branch is the correct seam.)
- **Test-first:** in `internal/core/shipment_gate_test.go`, add red tests driving the broker to a
  `DecisionError{config}` (→ typed `*GateError` config, exit-7 class) and `DecisionError{timeout}`
  (→ retryable `*GateError`, exit-8 class), asserting the returned error is a `*GateError` of the
  right class rather than a `*GateBlockedError`. Keep an existing exit-1 block case asserting it
  still returns `*GateBlockedError`.
- **Acceptance criteria:**
  1. A shipment-level `DecisionError{config}` returns a typed `*GateError` (config class), not a
     `GateBlockedError`.
  2. A shipment-level `DecisionError{timeout}` returns a retryable `*GateError` (timeout class).
  3. A genuine exit-1 block still returns `*GateBlockedError` (exit 6) — no regression.
  4. Error-class gate evidence (`EventGateError`) is appended for the error path.
- **Posture:** test-first. **Independent** of F1/F4/F7/Q3.

### Task T-F7 — `move --json` emits a structured payload for the `*GateError` class
- **Source:** stash `83B885EE` (adversarial F7). **Files:** `internal/cli/gate_exit.go`,
  `internal/cli/move.go`.
- **Behavior:** add `renderGateErrorJSON(id string, ge *corerrors.GateError) (string, error)` in
  `gate_exit.go`, mirroring `renderGateBlockedJSON`, marshalling a `gateJSONPayload` with
  `{ID, Outcome:"error", Error: ge.Error(), Retryable: ge.Retryable()}` (fields already declared
  at gate_exit.go:59-60). In `moveGateError` (move.go:112-119), under `--json`, check
  `errors.As(err, &be)` for `*GateBlockedError` **first** (preserving the `gateExitError`
  blocked-before-error precedence, exit 6 before 7/8), then `errors.As(err, &ge)` for `*GateError`,
  printing the rendered payload before returning the mapped `*ExitError`. Exit codes (7/8) and the
  human/stderr path are unchanged.
- **`omitempty` correctness (Go persona P1):** `gateJSONPayload.Retryable` is tagged
  `json:"retryable,omitempty"`, so a config-class `*GateError` (`Retryable()==false`) **omits** the
  key entirely — output is `{"id":...,"outcome":"error","error":...}`. Do **NOT** drop `omitempty`
  on the shared struct (that would inject `retryable:false` into every `*GateBlockedError` payload
  and regress AC #3). Tests MUST assert the **parsed** value / key-absence semantics
  (`json.Unmarshal` → `retryable == false` for config; `retryable == true` present for timeout),
  not a brittle literal-substring match.
- **Test-first:** in `internal/cli/gate_exit_test.go` / `move_test.go`, add a red test asserting
  that a `*GateError` (config and timeout) under `--json` currently emits empty stdout; assert the
  new payload is emitted with `outcome:"error"`, populated `error`, and the parsed `retryable`
  matching the class, while the exit code stays 7/8.
- **Acceptance criteria:**
  1. `move ... --json` on a config-class `*GateError` prints a payload whose parsed `outcome` is
     `"error"`, `error` is populated, `retryable` parses to `false` (key may be omitted per
     `omitempty`), and exits 7.
  2. `move ... --json` on a timeout-class `*GateError` prints `retryable:true` (present) and exits 8.
  3. The `*GateBlockedError` `--json` payload and all non-JSON output are unchanged (no regression).
- **Posture:** test-first. **Independent** of F1/F4/F5/Q3.

### Task T-Q3 — Derived indexed gate-evidence read-model (parent of Q3.0–Q3.3)
- **Source:** stash `7ED9CE1A` (Q3 follow-up). **Files:** `internal/events` or a new leaf package,
  `internal/db/*`, `internal/core/doctor`.
- Container task; delivered via the four ordered subtasks below. Deliverable: a derived, disposable,
  indexed projection of per-item gate-evidence status/SHA rebuilt from item logs on every
  `backlogit sync`, plus a doctor advisory that consumes it. Logs remain source of truth.
- **Adjudications folded in from plan-review:**
  - **Shape (Architecture P1 + SecLens P2 + SQLite):** the projection predicate must be a **single
    source of truth**. Because the dependency edge is strictly one-way `core → db`
    (`internal/core/gate_evidence.go` imports `internal/db`; `db` cannot import `core`), `db` cannot
    reuse core's `latestGatePassEvidence`/event constants. Q3.0 extracts the composed post-F4
    predicate + the gate-evidence event-type constants into a **leaf package** both `core` and `db`
    consume, eliminating the duplication-drift hazard rather than papering over it with a
    consistency test.
  - **Column vs. table (Architecture P2 vs SQLite):** adjudicated to a **dedicated `gate_evidence`
    table** keyed by `item_id`. The SQLite persona's column-on-`items` option is viable for the 1:1
    cardinality, but the Architecture persona surfaced a decisive concern the column option carries:
    `items` is populated from Markdown frontmatter in the batch-insert phase, while gate evidence is
    sourced from JSONL logs in the later `rehydrateItemLogs` phase — columns on `items` force a
    cross-phase `UPDATE` that silently no-ops for any item with logs but no frontmatter row. A
    dedicated table sourced purely in the log phase avoids that ordering coupling, keeps the
    projection cleanly disposable (its own `DELETE`/rebuild), and does not pollute the
    frontmatter-projection table. (Column option remains an acceptable fallback if Ship finds the
    cross-phase concern immaterial; document whichever is chosen.)
  - **Non-authoritative marker (SecLens P3 + Constitution P3):** the table/columns carry a schema
    comment marking them **advisory-derived-only**; store **only** a status token + evidence SHA —
    never report JSON, stderr, or `force_reason` — so no sensitive gate output leaks into the cache
    and nothing invites a future caller to treat it as a gate input. The projection lives only in
    the ephemeral (gitignored) `backlogit.db` cache, never in committed state.

#### Subtask Q3.0 — Extract a shared gate-evidence predicate + event-type constants (leaf package)
- **File:** `internal/events` (leaf, already imported by both `core` and `db`) or a new
  `internal/gateevidence` leaf.
- **Behavior:** move/define the gate-evidence event-type constants and a single exported predicate
  helper (e.g. `HasValidGateEvidence(evs) (status, sha)`), encoding the **finalized F4 composed
  predicate** — latest event satisfying `EventGateForced` OR (`EventGatePassed` AND `ran==true`),
  plus the `forced_no_run` distinction (see Q3.2). Repoint core's `latestGatePassEvidence`
  (post-F4) to delegate to this helper (pure refactor; existing core tests stay green).
- **Test-first:** table test for the shared predicate covering pass/forced/`ran=false`/interleaved/
  missing cases; assert core's member scan and doctor still behave identically after the repoint.
- **Acceptance criteria:** (1) one exported predicate + constants live in a leaf package importable
  by both `core` and `db`; (2) core's member scan (F4) delegates to it with no behavior change vs
  the F4 tests; (3) no import cycle introduced (`go build ./...` clean).
- **Posture:** test-first (code/refactor domain). **Depends on T-F4** (encodes its finalized
  semantics).

#### Subtask Q3.1 — Add the derived `gate_evidence` projection table + index to the cache schema
- **File:** `internal/db/schema.go` (`EnsureSchema`).
- **Behavior:** add a dedicated `gate_evidence(item_id TEXT PRIMARY KEY, gate_status TEXT,
  evidence_sha TEXT, head_sha TEXT)` table (advisory-derived-only per adjudication) following the
  existing create-if-not-exists pattern, plus a supporting index on `gate_status` (advisory query
  filters the discriminator first; e.g. `idx_gate_evidence_status`). Additive and backward-compatible
  (no change to existing tables/queries). If Ship instead chooses the column-on-`items` fallback, use
  the proven best-effort `ALTER TABLE items ADD COLUMN` idempotent migration pattern (schema.go:410-426).
- **Test-first:** schema test asserting `EnsureSchema` creates the table/index and is idempotent
  across repeated calls (and on an already-populated DB).
- **Acceptance criteria:** (1) `EnsureSchema` adds the `gate_evidence` table + index; (2) re-running
  `EnsureSchema` is a no-op (idempotent, no error); (3) existing schema tests still pass.
- **Posture:** test-first (config/schema domain). No dependency (schema is independent of Q3.0).

#### Subtask Q3.2 — Populate the projection from item logs during rehydration
- **File:** `internal/db/rehydration.go` (log-rehydration phase, after `rehydrateItemLogs`).
- **Behavior:** during `Rehydrate` (clear-and-rebuild), compute each item's gate-evidence status via
  the **Q3.0 shared predicate** and write one `gate_evidence` row: `gate_status` ∈
  {`passed`, `forced`, `forced_no_run`, `missing`}, plus the evidence SHA and the recorded `head_sha`.
  The distinct `forced_no_run` token surfaces members that shipped on an audited force rather than a
  real run (SecLens P2 audit-visibility). The projection table is fully rebuilt each sync (its own
  clear+repopulate), inheriting `Rehydrate`'s idempotency; item log JSONL is never mutated.
- **Test-first:** rehydration test: seed item logs with pass/forced/`ran=false`/`forced_no_run`/none
  cases, run `Rehydrate` twice, assert the projected rows match the expected status/SHA for each item
  and are identical across both runs (idempotent); assert logs are unmodified.
- **Acceptance criteria:** (1) after sync, each terminal task/subtask's `gate_evidence` row matches
  its logs under the Q3.0 predicate (incl. `forced_no_run`); (2) running sync twice yields identical
  projection rows; (3) item log JSONL is never mutated; (4) only status token + SHAs are stored (no
  report JSON/stderr/force_reason).
- **Posture:** test-first (code domain). **Depends on Q3.0 and Q3.1.**

#### Subtask Q3.3 — Repoint the advisory `doctor --check-gate-evidence` to the indexed projection
- **File:** `internal/core/doctor.go` (flag already exists in `internal/cli/doctor.go`).
- **Behavior:** switch the advisory `--check-gate-evidence` check (doctor.go:398-420) to query the
  `gate_evidence` projection instead of scanning each item's logs, preserving the advisory-only
  contract (findings are WARNINGS; exit code unaffected). **Correctness fallback (Architecture P2):**
  when the projection is **absent or stale** (e.g. items gated since the last `sync`, since the live
  completion path indexes events incrementally but does not touch the projection), fall back to the
  authoritative **log-scan** rather than no-op — a no-op would silently stop auditing. Optionally
  surface `forced_no_run` members as an informational advisory. Does **not** add strict-mode
  enforcement.
- **Test-first:** doctor test asserting the indexed check produces the same
  `FindingMissingGateEvidence` findings as the log-scan for the same fixture; that a stale/absent
  projection falls back to the log-scan (still flags correctly); and that the exit code is still
  unaffected (advisory).
- **Acceptance criteria:** (1) the indexed check flags the same terminal items lacking gate evidence
  as the log-scan baseline; (2) on absent/stale projection it falls back to the log-scan (no false
  negatives); (3) exit code unchanged (advisory).
- **Posture:** test-first (code domain). **Depends on Q3.2.** Lands after the projection exists
  (operator constraint: column/table before doctor-query use).

## Dependency edges (for harvest)

- T-F1, T-F4, T-F5, T-F7 — **mutually independent** (no edges among them).
- Q3.0 **blocks-on** T-F4 (`dep add Q3.0 T-F4`) — the shared predicate encodes F4's finalized
  composed semantics.
- Q3.1 (schema) — **independent** (no dependency).
- Q3.2 **blocks-on** Q3.0 **and** Q3.1 (`dep add Q3.2 Q3.0`, `dep add Q3.2 Q3.1`).
- Q3.3 **blocks-on** Q3.2 (`dep add Q3.3 Q3.2`).

## Suggested execution order (Ship)

1. Any order / parallel: **T-F1, T-F5, T-F7** (independent, single-file, low-risk) and **T-F4**.
2. **Q3.1** (schema) may proceed in parallel with the remediations.
3. After T-F4: **Q3.0** (shared predicate) → **Q3.2** (projection; also needs Q3.1) → **Q3.3**
   (doctor consumption). Strict order within the Q3 chain: predicate + schema before projection
   before doctor consumption (operator constraint: column/table lands before doctor-query use).

## Plan Hardening

Applies to the Q3 read-model subtasks (Q3.0 shared predicate, Q3.1 schema, Q3.2 projection). The
four F1/F4/F5/F7 remediations are low-risk single-file changes and are excluded here.

- **Single-source predicate (primary correctness risk):** the gate-evidence predicate MUST have one
  definition (Q3.0 leaf helper) consumed by both the authoritative core member scan (F4) and the
  derived db projection (Q3.2). This structurally prevents the advisory doctor read-model from
  disagreeing with the enforcing shipment gate. The one-way `core → db` boundary makes a shared
  **leaf** package (not a core import) the only sound placement.
- **Rebuild safety / idempotency:** the `gate_evidence` projection is a derived cache. It MUST be
  recomputed from scratch on every `Rehydrate` (its own clear+repopulate inside the existing
  clear-and-rebuild path) and MUST be byte-identical across repeated syncs for unchanged logs.
  Q3.2's run-twice-identical acceptance criterion is the explicit guard.
- **Backward compatibility:** a dedicated additive `gate_evidence` table (or, in the fallback,
  additive nullable columns via best-effort `ALTER TABLE ADD COLUMN`) leaves every existing table
  and query unaffected; pre-existing DBs upgrade in place with no destructive migration.
- **Advisory-derived-only + no sensitive data (SecLens):** the projection is marked
  advisory-derived-only in the schema and stores only a status token + evidence SHA — never report
  JSON, stderr, or `force_reason`. Nothing may treat it as an authoritative gate input.
- **Staleness correctness (doctor):** Q3.3 falls back to the authoritative log-scan when the
  projection is absent/stale, so the advisory never emits false negatives between syncs.
- **Rollback:** the projection is disposable and logs are authoritative — reverting the code and
  running `backlogit sync` rebuilds the index without the table/columns; no data migration or
  backfill, and no source-of-truth data can be lost. There is no destructive drop of authoritative
  data (the `items` cache and `gate_evidence` table are both derived).
- **Blast radius:** confined to the SQLite cache layer + one shared leaf predicate + one advisory
  doctor check. No change to the append-only log writer, the gate decision logic, or any
  completion/ship state transition.

## Quality-gate exit criteria (per unit, executed by Ship)

Each task/subtask lands green through the constitutional gate order before it is considered done:
`gofmt -l .` (clean) → `go vet ./...` → `golangci-lint run` → `go test ./...` (the touched packages
at minimum). Test-first ordering per unit: add the compiling-but-failing red test/harness (stub any
new symbol — `ResolvedBase.OverrideShadowed`, `renderGateErrorJSON`, the Q3.0 helper — with an inert
default so the suite compiles and the assertion fails), observe red, implement, observe green.

## Readiness (Step 3.3)

All units are backlog-sized (each < 3 files, single skill domain, test-first, ≥1 acceptance
criterion), dependency-aware (Q3 chain ordered; remediations independent), and ready for Ship to
execute. Grounded in real symbols verified against the shipped 082-F code. Advance to plan-review.

<!-- plan-review-attempt: 1 — verdict PASS (3 PASS + 3 ADVISORY, 0 FAIL); all material advisories incorporated. See docs/reviews/2026-07-06-gate-broker-phase2-hardening-plan-review.md -->
