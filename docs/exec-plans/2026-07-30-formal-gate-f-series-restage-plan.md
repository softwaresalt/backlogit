---
chunk_strategy: h1-h2-h3
description: 'Restage formal-gate implementation through the bounded F2/F3 foundation shipment: canonical serialization plus authoritative status taxonomy.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-30-formal-gate-f-series-restage-plan.md
title: 'Formal-gate F-series restage: F2/F3 foundations'
---

## Source

* Feature: `106-F` ("Formal-gate implementation"), previously blocked until a
  proceed decision or pivot with a replacement contract and a later Stage
  restaging moved the feature to active.
* Charter:
  `docs/decisions/2026-07-14-formal-gate-architecture-spike.md`.
  The charter forbids formal-gate implementation before a coherent contract is
  recorded and states that no implementation unit may open until the decision
  output records `proceed` or `pivot` with a replacement contract
  (`docs/decisions/2026-07-14-formal-gate-architecture-spike.md:36`,
  `:121`, `:140-141`).
* Findings:
  `docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md`.
  The findings conclude `PIVOT` with a coherent replacement contract and medium
  confidence because evidence authenticity and transactional multi-mutation
  still require bounded design decisions
  (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:25-37`).
* Compound search: no direct formal-gate F-series learning was found. The
  docline frontmatter learning
  `docs/compound/2026-06-26-docline-frontmatter-contract.md` informs this plan's
  born-compliant frontmatter and self-lint posture.

## Problem Frame

PR #239 failed because implementation and review-fix patching began while
foundational trust, binding, canonicalization, status, dependency, atomicity, and
CLI/MCP parity questions were unresolved. The spike pivots away from the prior
plan-digest/waiver direction and toward extending the shipped gate-evidence log
substrate (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:278-289`).

This Stage pass keeps the restart bounded. It activates `106-F` only because the
charter's gate is now satisfied by a pivot with a replacement contract, then
harvests only the two cheap foundational units that the findings order first:
F2 canonical serialization/hash and F3 authoritative status taxonomy
(`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:325-354`).
It explicitly does not harvest F1, F4, F5, or F6.

## Replacement Contract Direction

The replacement contract says a fail-closed formal PASS-only gate is viable only
if all eight requirements below hold
(`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:291-323`):

1. Keep the per-item JSONL evidence log as the durable event source, while
   evidence validity also depends on trusted freshness and anti-replay state
   outside the mutable log.
2. Add an authenticity proof to evidence events and bind the authorizing
   evidence hash to the manifest.
3. Hash a single canonical serialization rather than file-on-disk bytes.
4. Read completion through one authoritative status taxonomy with
   context-specific predicates.
5. Reason only over dependency semantics that are durable in markdown.
6. Treat multi-store mutations as advisory unless wrapped in a journaled or
   idempotent operation.
7. Route every governed operation through one shared core function with
   behavioral parity asserted across MCP and CLI.
8. Use a dedicated formal-admission predicate distinct from the current shared
   `Latest` evidence predicate; it must require an authenticated, non-forced
   real PASS, invalidate prior passes after later block/requeue events, and bind
   a schema-validated formal report into the authenticity proof.

## F-Series Decomposition and Ordering

The findings recommend the following bounded units
(`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:325-354`):

| Unit | Scope | Stage decision for this shipment |
|---|---|---|
| F1 | Evidence authenticity primitive, including external proof and anti-replay state | Deferred. Requires its own micro-decision before implementation because the exact authenticity mechanism is unresolved (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:330-335`, `:358-363`). |
| F2 | Canonical serialization plus hash primitive for evidence and manifest hashing | Harvest now. It is cheap, foundational, and unblocks F1. |
| F3 | Authoritative status taxonomy with named predicates for gate, queue, and shipment contexts | Harvest now. It is cheap, foundational, and unblocks F1. |
| F4 | Durable dependency type in markdown frontmatter | Deferred to a later staging cycle after F2/F3 and F1. |
| F5 | Journaled multi-mutation wrapper | Deferred. Requires its own micro-decision before implementation because journal versus idempotent replay is unresolved (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:343-345`, `:364-365`). |
| F6 | Governed-operation parity hardening through one commit-association operation | Deferred to a later staging cycle after F2/F3 and alongside F4 where appropriate. |

Recommended ordering is preserved exactly: F2 and F3 first; F1 next; F4/F6 in
parallel; F5 last
(`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:353-354`).

## Requirements Trace

| Replacement requirement | Implementation action in this shipment |
|---|---|
| Requirement 3: one canonical serialization for hashes | F2 is not primitive-only: it defines one canonicalizer and routes the governed evidence/report/manifest hash seams that exist today through that helper. If a manifest hash seam is only preparatory, the task must still add manifest-shaped golden tests so later F1 consumers cannot reintroduce raw hashing. |
| Requirement 4: one authoritative status taxonomy | F3 centralizes the existing status sets into named context-specific predicates for dependency unblocking, shipment releasability, and gate targets, with explicit archived/archived_status composition rules. It is bounded by compatibility wrappers and must halt for restaging if full consumer wiring exceeds the 2-hour/file-count boundary. |
| Requirement 1/2/8: authenticity and formal admission | Deferred to F1 micro-decision and implementation. F2/F3 provide prerequisites but do not claim to solve authenticity or admission. |
| Requirement 5: durable dependency semantics | Deferred to F4. This plan names the dependency-type problem but does not harvest it. |
| Requirement 6: journaled/idempotent mutations | Deferred to F5 micro-decision and implementation. This plan prevents premature harvest. |
| Requirement 7: governed-operation parity | Deferred to F6. This plan does not alter CLI/MCP mutation behavior. |

## Implementation Units

### F2 - Canonical serialization + hash primitive

* **Goal:** provide one canonical byte representation and hash helper for
  logical gate evidence, formal reports, and future manifest-binding payloads,
  then route the existing governed hash seams through that helper. This task is
  complete only when governed F-series hash paths no longer compute ad hoc raw
  hashes. It must not hash on-disk markdown bytes. The findings identify current drift
  between `mdfront` body preservation and typed frontmatter normalization, and
  note that current `gate_report_hash` hashes raw broker report bytes without
  canonicalization
  (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:139-163`).
* **Likely files touched:**
  * `internal/core/gate/*` for formal report or broker-adjacent canonical
    payload definitions and golden tests.
  * `internal/core/gate_evidence.go` for the `gate_report_hash` seam; the
    existing report hash path must call the canonical helper or the tests fail.
  * `internal/db/manifest.go` and related manifest tests for manifest-shaped
    canonical hash reuse. If no active manifest caller exists yet, this remains
    a golden-test contract rather than a second implementation path.
  * `internal/mdfront/codec.go` and `internal/models/frontmatter.go` are
    characterization references only, not places to add duplicate canonicalizers.
* **Posture:** test-first. Add failing tests that define the canonical bytes and
  hash before production implementation.
* **Expected tests:**
  * LF and CRLF inputs that represent the same logical payload hash identically.
  * Map key order is stable and sorted, including nested maps and arrays.
  * Trailing-newline rule is explicit and enforced.
  * Evidence event, formal report, and manifest-shaped golden payloads all hash
    through the same helper.
  * The current `gate_report_hash` path returns the SHA-256 of canonical bytes,
    not raw broker bytes.
* **Atomic acceptance criteria:**
  * One exported or package-level canonicalization seam exists for later F1 and
    manifest consumers; there is not a second ad hoc hash path.
  * The canonicalizer is deterministic on Windows and non-Windows line endings.
  * Existing gate evidence meaning is unchanged, but governed hash computation
    is canonicalized at the current hash seam instead of left as a raw/ad hoc
    path.
  * Targeted tests fail before implementation and pass after, followed by the
    full Ship quality gate when Ship executes the task.
* **2-hour boundary:** fewer than three production files, one focused primitive,
  and no activation of formal PASS admission.

### F3 - Authoritative status taxonomy with context-specific predicates

* **Goal:** centralize status semantics without collapsing distinct questions
  into one generic terminal boolean. This is a bounded taxonomy-and-wiring task:
  characterize current behavior first, introduce named predicates, and wire only
  the compatibility seams needed for queue, shipment, and gate consumers. If the
  red/characterization phase proves that completing all consumer wiring requires
  more than the bounded task can safely change, Ship must halt and restage the
  excess wiring rather than stretching F3. The findings show artifact statuses in
  `internal/models/artifact.go`, shipment statuses in
  `internal/core/shipment.go`, archive composition through
  `archived_status`, gate terminal defaults in
  `internal/core/gate_transition.go`, queue dependency terminality in
  `internal/core/blocking_cascade.go` and `internal/core/queue.go`, and shipment
  release terminality in `internal/core/shipment_lifecycle.go`
  (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:167-195`).
* **Likely files touched:**
  * `internal/models/artifact.go` or one new `internal/core/status_taxonomy.go`
    for the shared taxonomy definitions.
  * Existing compatibility seams such as `TerminalStatuses`,
    `isTerminalReleaseStatus`, and the gate target status check should delegate
    to the taxonomy instead of duplicating status sets.
  * Call-site churn is capped. If queue, shipment, archive, and gate wiring
    cannot stay within the 2-hour boundary, the task stops after taxonomy plus
    characterization and returns to Stage for explicit follow-up harvest.
* **Posture:** characterization-first, then test-first refactor. First encode
  the existing context-specific truth tables so the refactor cannot silently
  change queue, shipment, or gate behavior; then introduce named predicates.
* **Expected tests:**
  * `queued`, `active`, `blocked`, and `review` remain non-terminal for the
    dependency no-longer-blocking context.
  * `done`, `accepted`, `rejected`, `archived`, `shipped`, and `abandoned` keep
    their existing context-specific meanings.
  * `archived` plus `archived_status` composition is documented and covered,
    including the restore/source-status distinction.
  * Gate target status remains configurable and is not confused with shipment
    releasability.
* **Atomic acceptance criteria:**
  * One authoritative taxonomy names at least these predicates:
    no-longer-blocking, releasable, and gate-target.
  * Call sites use the named predicates instead of locally repeating divergent
    terminal status slices.
  * Existing queue, shipment, and gate decisions remain behaviorally equivalent
    unless an intentional difference is called out in tests and docs.
  * Targeted tests fail before implementation and pass after, followed by the
    full Ship quality gate when Ship executes the task.
* **2-hour boundary:** taxonomy plus compatibility seams only, no dependency
  schema work, no formal admission logic, and no broad rewrite of every status
  caller. Exceeding this boundary is a stop condition, not permission to expand
  F3.

## Dependency Graph

* `F2` and `F3` are independent and may ship in either order inside the same
  shipment.
* Later `F1` depends on both `F2` and `F3` because authenticity and formal
  admission need canonical payloads and a stable status predicate surface.
* `F4` and `F6` are deferred until after the F2/F3 foundation lands and after F1
  is staged.
* `F5` is last because journaled or idempotent mutation design remains open and
  has the widest atomicity blast radius.

## Decisions and Rationale

* **Harvest only F2 and F3.** This follows the findings' explicit ordering and
  preserves the bounded restart. Creating F1 or F5 implementation tasks now
  would violate the residual questions section and repeat the unbounded loop.
* **Preserve behavior while extracting taxonomy.** F3 should begin with
  characterization tests because the current contexts intentionally answer
  different questions. The plan centralizes naming and documentation first, then
  delegates semantic changes or excess call-site rewiring to future reviewed
  tasks if tests expose a mismatch or the file-count boundary is exceeded.
* **Do not add a new storage or auth dependency in F2.** The canonicalizer is a
  prerequisite for the authenticated proof, not the proof itself. Key
  management, anti-replay state, and proof placement stay in F1's micro-decision.

## Risks and Caveats

* **Scope creep into F1/F5.** Mitigation: task descriptions and shipment scope
  name only F2/F3. F1 and F5 are explicitly micro-decision prerequisites.
* **Behavior drift during F3 refactor.** Mitigation: characterization tests must
  lock current queue, shipment, archive, and gate behavior before production code
  changes. Add a smoke-verification note for Ship to check representative queue
  dependency, shipment release, archive/unarchive, and gate-target cases before
  closure.
* **Duplicate canonicalizers.** Mitigation: F2 acceptance requires one seam used
  by later evidence and manifest hashing rather than parallel helpers.
* **Runtime risk is deferred.** This shipment should not activate the formal
  PASS-only gate. It creates primitives and predicates for later work.

## Plan Hardening Signals

* Public API, schema, or contract change: **absent for this shipment.** F2 adds
  an internal canonicalization primitive; F3 names existing status semantics and
  must preserve current behavior through characterization tests.
* Security, auth, permission, or compliance-sensitive behavior: **absent for this
  shipment.** Authenticity proof design is explicitly deferred to F1.
* Migration, backfill, destructive data/config action, or irreversible step:
  **absent.** No data migration or destructive operation belongs in F2/F3.
* External integration, operator checkpoint, or external dependency: **absent.**
  No new external dependency is planned.
* High runtime, rollout, or rollback risk: **absent.** The shipment should not
  activate the formal gate; rollback is removal of the new primitives/predicate
  wiring before merge.

Requires plan hardening: no

## Runtime Verification and Closure

* **F2 runtime surface:** no direct CLI/MCP behavior should change. Verification
  is unit-level determinism across payload key order, LF/CRLF inputs, and
  trailing-newline cases, then Ship's full gates: `go test ./...`,
  `go vet ./...`, `golangci-lint run`, and `gofmt -l .`.
* **F3 runtime surface:** queue dependency resolution, shipment releasability,
  archive composition, and gate target evaluation are runtime decision surfaces.
  Verification must include characterization tests for each context, a smoke
  check of representative queue/shipment/archive/gate-target decisions, and the
  full Ship quality gates. Rollback is reverting the taxonomy wrapper and
  compatibility-seam edits before merge.
* **Closure:** Ship should record that F2/F3 are foundations only. No monitoring
  plan or rollback trigger is needed before formal-gate activation because this
  shipment should not enable the formal PASS-only admission path.

## Constitution Check

* **Principle I - Safety-First Go:** pass. Downstream implementation remains in
  Go, avoids `unsafe`, wraps errors with context, and runs full quality gates in
  Ship.
* **Principle II - Test-First Development:** pass. F2 and F3 both require
  failing tests before production code, with F3 beginning from characterization
  tests to prevent behavior drift.
* **Principle III - Workspace Isolation and Security Boundaries:** pass. All
  planned file operations remain under the repository root, and no secrets or
  credentials are introduced.
* **Principle IV - CLI Workspace Containment:** pass. Stage writes only this plan
  and backlog artifacts inside `C:\Source\GitHub\backlogit`.
* **Principle V - Structured Observability:** pass. Backlog tasks, shipment
  membership, and the plan record the scope and deferred decisions.
* **Principle VI - Single Responsibility:** pass. F2 and F3 add no speculative
  dependencies and each targets one domain.
* **Principle VII - Destructive Command Approval:** N/A. The plan and harvest
  require no destructive commands.
* **Principle VIII - Safety Modes for Risky Work:** pass. The bounded harvest
  explicitly freezes scope to F2/F3 and defers high-blast-radius F1/F5 work to
  micro-decisions.
* **Principle IX - Git-Friendly Persistence:** pass. The plan and backlog
  artifacts are human-readable Markdown/YAML.
* **Principle X - Agent Context Efficiency:** pass. The plan relies on the
  findings document and targeted code-path references rather than broad raw
  scans.
* **Principle XI - Merge Commit History Preservation:** N/A for Stage. Ship must
  enforce merge-commit-only if and when implementation PRs are opened.

Constitution Check: pass

## Plan Review

Rubber-duck review completed after the initial harvest. No P0 findings were
reported. Two P1 findings were accepted and addressed in this revision:

* **P1 - F2 completion was ambiguous.** The plan now states F2 is not
  primitive-only: governed evidence/report/manifest hash seams must route through
  the canonical helper, with shaped golden tests for evidence event, formal
  report, and manifest payloads.
* **P1 - F3 risked exceeding the atomic boundary.** The plan now bounds F3 to
  taxonomy plus compatibility seams and makes excess queue/shipment/archive/gate
  wiring a stop-and-restage condition rather than scope expansion.

Advisory P2/P3 feedback was also folded in: F2 now calls for nested
map/array-shaped golden tests, and F3 runtime verification now includes a
representative queue/shipment/archive/gate-target smoke check plus rollback note.

Plan Review: pass after P1 revisions.
