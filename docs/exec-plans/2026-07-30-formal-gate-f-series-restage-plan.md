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

PR #239 was closed unmerged. It never landed working formal-gate
implementation; its design and review-fix iteration churned indefinitely because
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
| Requirement 3: one canonical serialization for hashes | F2 is not primitive-only: it defines one canonicalizer and routes the governed evidence/report hash seams that exist today (e.g. `gateReportHash`) through that helper. No gate/shipment manifest hash seam exists yet — the manifest-binding payload is defined later by F1 — so F2 adds manifest-shaped golden tests over a **logical** payload (not `internal/db/manifest.go`, which is the merge-sync filesystem snapshot) so later F1 consumers cannot reintroduce raw hashing. |
| Requirement 4: one authoritative status taxonomy | F3 centralizes the existing status sets into named context-specific predicates for dependency unblocking, shipment releasability, and gate targets, with explicit archived/archived_status composition rules. It is bounded by compatibility wrappers and must halt for restaging if full consumer wiring exceeds the 2-hour/file-count boundary. |
| Requirement 1/2/8: authenticity and formal admission | Deferred to F1 micro-decision and implementation. F2/F3 provide prerequisites but do not claim to solve authenticity or admission. |
| Requirement 5: durable dependency semantics | Deferred to F4. This plan names the dependency-type problem but does not harvest it. |
| Requirement 6: journaled/idempotent mutations | Deferred to F5 micro-decision and implementation. This plan prevents premature harvest. |
| Requirement 7: governed-operation parity | Deferred to F6. This plan does not alter CLI/MCP mutation behavior. |

## Implementation Units

### F2 - Canonical serialization + hash primitive

* **Goal:** provide one canonical byte representation and hash helper for
  logical gate evidence, formal reports, and future manifest-binding payloads,
  as a dependency-free primitive with committed golden vectors. It must not hash
  on-disk markdown bytes. **Re-routing the existing `gateReportHash` seam is
  explicitly deferred to F1**, not performed in F2: `gateReportHash` currently
  hashes raw broker report bytes, and that path writes a durable, append-only
  evidence JSONL that F1 will depend on. Changing the recorded hash value now
  would create mixed-provenance hashes with no scheme-version tag, and the broker
  report payload contract (structured-JSON vs opaque normalized string; number
  domain) is owned by F1. F2 therefore delivers the primitive plus a
  characterization test that *documents* the current raw-hash drift without
  changing any recorded value; F1 later re-routes the seam and adds a
  canonicalization/hash-scheme version field to the evidence event so old and new
  hashes stay distinguishable. This task is complete only when the canonical
  primitive and its committed golden vectors exist and pass. The findings
  identify current drift between `mdfront` body preservation and typed
  frontmatter normalization, and note that `gateReportHash` hashes raw broker
  report bytes without canonicalization
  (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:139-163`).
* **Likely files touched:**
  * A new **stdlib-only** leaf package `internal/canonical` (imports zero
    internal packages — a stricter form of the shipment 068-S leaf-extraction
    pattern that broke the import cycle) that homes the canonical
    serialization + SHA-256 primitive. It is deliberately **not** placed under
    `internal/core/gate/*`: the canonicalizer is a domain-agnostic utility and
    future db-side manifest-binding consumers must be able to depend on it
    without a wrong-direction `db -> core` dependency or import cycle. The
    established one-way `core -> db` boundary is preserved by keeping this an
    import-free leaf; an architecture note plus a package-import assertion test
    keep it stdlib-only. Behavior preservation is proven with differential
    golden byte-equality tests, per the extraction learning
    (`docs/compound/2026-06-28-codec-extraction-leaf-packages.md`).
  * `internal/core/gate_evidence.go` is a **characterization reference only** in
    F2: a test documents that `gateReportHash` currently hashes raw broker
    bytes. F2 does not modify this seam; re-routing it is F1 scope (see Goal).
  * A **second structurally-distinct** logical golden payload (a deep
    nested-map/array shape) hashed through the same helper, deliberately **not**
    labeled as F1's manifest-binding payload, so F1 remains free to define the
    real manifest-binding shape. Do **not** target `internal/db/manifest.go`:
    that file is the merge-sync filesystem snapshot (`FileEntry`
    path/kind/size/mtime/itemID) and has no gate hash seam, so coupling F2 to it
    would bind gate canonicalization to unrelated DB-sync metadata.
  * `internal/mdfront/codec.go` and `internal/models/frontmatter.go` are
    characterization references only, not places to add duplicate canonicalizers.
* **Canonical byte contract (fixed here, not delegated to the implementer):**
  * **Encoding:** UTF-8, no BOM.
  * **Line endings:** LF (`\n`) only; any CR is normalized out before hashing.
  * **Object keys:** sorted lexicographically by UTF-8 code-unit order,
    recursively at every nesting level.
  * **Array order:** preserved as-is (array order is semantic and never sorted).
  * **Strings:** standard JSON escaping with minimal escapes, UTF-8; characters
    that do not require escaping are emitted literally.
  * **Numbers:** integers only in v1, rendered in canonical decimal with no
    leading zeros, no explicit sign for non-negative values, no fraction, and no
    exponent. Non-integer numbers are out of scope for v1 and must be carried as
    strings by callers. The canonicalizer **fail-closes**: it returns an error on
    any non-integer numeric input rather than lossily coercing it, so the
    "carry as strings" rule is enforced at the seam, not by caller convention.
    Fail-closed rejection is surfaced through an exported sentinel/typed error
    (e.g. `canonical.ErrNonIntegerNumber`, plus `canonical.ErrUnsupportedType`
    for inputs that are not map/slice/string/bool/integer) wrapped with `%w`, so
    tests assert via `errors.Is`/`errors.As` rather than brittle string matching
    (recorded so F1 cannot silently reintroduce float ambiguity).
  * **Whitespace:** compact form — no insignificant whitespace between tokens.
  * **Trailing newline:** exactly one trailing LF is appended to the serialized
    payload before the SHA-256 is computed.
  * **Hash:** SHA-256 over the canonical UTF-8 bytes, lowercase hex.
  * **Standards note (deliberate JCS divergence):** this contract intentionally
    diverges from RFC 8785 (JSON Canonicalization Scheme) in three ways — a
    single trailing LF, integer-only numbers, and UTF-8 byte-order key sorting
    (JCS sorts by UTF-16 code units). These divergences are recorded here so
    that if F1's authenticity/external-proof design ever verifies hashes with a
    standard JCS tool, it does so knowing this seam is not RFC 8785 compatible.
  * **Empty collections / optional fields:** hashed payload types must **not**
    use `omitempty` on collection or optional fields; present-empty (`[]`/`{}`)
    and absent are distinct canonical forms. Golden vectors pin the
    empty-collection and null/absent cases so an `omitempty`-driven collapse of
    empty vs absent is caught
    (`docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`).
  * **Timestamps:** any timestamp in a hashed payload is canonical UTC with a
    trailing `Z` (emitted via `models.NowUTC()`); a golden vector asserts the
    `Z` form and that no `[+-]dd:dd` offset appears, so the same logical evidence
    hashes identically on a local-offset dev machine and a UTC CI runner
    (`docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md`).
  * **Go surface:** one exported canonicalize function returning
    `([]byte, error)` and one hash helper returning `(string, error)`, accepting
    JSON-like `any` values (documented `json.Number` handling); no
    `context.Context` on this pure CPU function.
* **Posture:** test-first, using table-driven `t.Run` subtests with a
  `t.Helper()` golden-vector comparison helper. Add failing tests that pin the
  canonical bytes and hash before production implementation.
* **Expected tests (with committed golden vectors):**
  * At least two committed golden vectors: the exact canonical byte string and
    its lowercase-hex SHA-256 for (a) a nested-map/array evidence-event payload
    and (b) a second structurally-distinct deep nested-map/array payload,
    deliberately **not** labeled as F1's manifest-binding payload (so F1 stays
    free to define the real shape).
  * Two inputs differing only by LF vs CRLF and by unsorted vs sorted key order
    produce identical canonical bytes and identical hashes.
  * Reordering array elements changes the hash (array order is significant).
  * A non-integer numeric input causes the canonicalizer to return an error
    (fail-closed number-domain enforcement).
  * The trailing-newline rule is enforced: removing the trailing LF changes the
    hash, and a doubled trailing newline is normalized to exactly one.
  * Evidence event, formal report, and the second generic payload all hash
    through the same helper (one seam, no parallel path).
  * A characterization test documents that the current `gateReportHash` path
    still hashes raw broker bytes; F2 does **not** change that recorded value
    (re-routing is deferred to F1).
* **Atomic acceptance criteria:**
  * The only canonicalization+hash seam for the **new F2 payloads** lives in the
    stdlib-only leaf `internal/canonical`; those payloads never compute an ad hoc
    hash. (`gateReportHash`'s existing raw path legitimately persists through F2
    and is re-routed in F1.) A structural guard — a grep/lint check forbidding
    `crypto/sha256` on governed payload paths outside `internal/canonical` —
    enforces single-seam usage durably, rather than relying on a unit test to
    prove a negative.
  * The canonicalizer is deterministic on Windows and non-Windows line endings.
  * F2 does not modify `gateReportHash`; existing gate evidence hashes are
    unchanged. Re-routing that seam (and adding a hash-scheme version field to
    the evidence event) is deferred to F1, which owns the report payload
    contract.
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
  `internal/core/gate_transition.go`, the parent/child completion cascade in
  `internal/core/blocking_cascade.go` (its `TerminalStatuses` slice gates both
  `CheckChildrenTerminal` parent-completion and queue dependency resolution),
  queue dependency terminality in `internal/core/queue.go`, and shipment
  release terminality in `internal/core/shipment_lifecycle.go`
  (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md:167-195`).
* **Likely files touched:**
  * `internal/models/artifact.go` or one new `internal/core/status_taxonomy.go`
    for the shared taxonomy definitions.
  * Existing compatibility seams such as `TerminalStatuses`,
    `isTerminalReleaseStatus`, and the gate target status check delegate to the
    taxonomy instead of duplicating status sets. **Preserve the documented,
    intentional divergence** between the 6-status `core.TerminalStatuses`
    ({done, accepted, archived, shipped, abandoned, rejected}) and the 4-status
    `isTerminalReleaseStatus` ({done, accepted, rejected, archived} — omits
    shipped/abandoned): a naive "one releasable predicate" would silently unify
    the two sets and change release-progression behavior. F3 introduces a
    **dedicated descope predicate distinct from releasable/terminal** and keeps
    the five `isTerminalReleaseStatus` release-progression call sites
    behaviorally unchanged, per the tracked follow-up (stash `A3C349DD`) and the
    learning
    (`docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`).
    Within F3's bounded scope the divergence is preserved and characterized, not
    reconciled.
  * Call-site churn is capped. If queue, shipment, archive, and gate wiring
    cannot stay within the 2-hour boundary, the task stops after taxonomy plus
    characterization and returns to Stage for explicit follow-up harvest.
* **Posture:** characterization-first, then test-first refactor. First encode
  the existing context-specific truth tables so the refactor cannot silently
  change queue, shipment, or gate behavior; then introduce named predicates.
  Note the test-first nuance: characterization tests lock current behavior on a
  **green baseline** (they pass immediately by design); the red-before-green
  cycle (NON-NEGOTIABLE Principle II) applies to the **new** named predicates and
  the refactored seam delegation, which must have failing tests before their
  production code exists.
* **Expected tests:**
  * `queued`, `active`, `blocked`, and `review` remain non-terminal for the
    dependency no-longer-blocking context.
  * `done`, `accepted`, `rejected`, `archived`, `shipped`, and `abandoned` keep
    their existing context-specific meanings.
  * The divergent sets are pinned **exactly as they differ today** before any
    refactor: the 6-status `core.TerminalStatuses` and the 4-status
    `isTerminalReleaseStatus` (omitting shipped/abandoned) each get a
    characterization test, so the extraction cannot silently unify them.
  * `archived` terminality is keyed off the literal `archived` status: the
    no-longer-blocking, children-terminal, and releasable predicates treat
    `status == "archived"` literally and **ignore** `archived_status`.
    `archived_status` composition stays scoped to the archive/unarchive restore
    provenance path only and must not leak into terminality evaluation, unless an
    intentional change is explicitly documented in tests and docs. Because the
    DB index projection omits `archived_status` and `loadArtifact` is
    index-first, any predicate that needs `archived_status` reads the Markdown
    source directly and **fails closed** on empty/unrecognized status via a
    recognized-status allowlist (never defaulting an unknown value to
    non-terminal)
    (`docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`).
  * Gate target status remains configurable: the gate-target predicate accepts
    the workspace-configured terminal set as a parameter (it is not a
    parameter-free pure-status predicate), and its test asserts the configured
    set is honored rather than a static `['done']` default. It is not confused
    with shipment releasability.
  * Parent/child completion cascade: the `CheckChildrenTerminal` truth table
    (`internal/core/blocking_cascade.go:55-70`) is characterized and preserved
    under its **own** named predicate (children-terminal / parent-completion),
    kept distinct from no-longer-blocking even where it initially aliases the
    same status set, so a parent may still move terminal only when every child is
    terminal and a future dependency-semantics change cannot silently alter
    parent-completion eligibility.
* **Atomic acceptance criteria:**
  * One authoritative taxonomy names at least these predicates:
    no-longer-blocking, children-terminal (parent-completion), releasable,
    descope, and gate-target. children-terminal is a distinct named predicate
    even where it initially aliases the same status set as no-longer-blocking,
    and descope is distinct from releasable/terminal, so each context can diverge
    intentionally rather than accidentally.
  * Call sites use the named predicates instead of locally repeating divergent
    terminal status slices. The exported mutable `TerminalStatuses` slice is
    replaced with an immutable predicate (or an accessor returning a defensive
    copy) backed by an **unexported** status set; no call site iterates a
    locally-held terminal-status set or ranges an exported slice directly. A test
    asserts the taxonomy cannot be mutated from outside its package, and
    out-of-package references/mutations are grepped before the exported slice is
    removed or deprecated (one authoritative source; the "one seam, no parallel
    path" bar; avoids the exported-mutable-global footgun,
    `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`).
  * Pure-status predicates (no-longer-blocking, children-terminal, releasable)
    are parameter-free; only the config-dependent gate-target predicate accepts
    the configured terminal set as a parameter, so the taxonomy is not coupled to
    `Workspace`/`gateConfig`.
  * Existing queue dependency, shipment releasability, parent/child completion
    cascade, archive composition, and gate-target decisions remain behaviorally
    equivalent unless an intentional difference is called out in tests and docs.
  * Targeted tests fail before implementation and pass after (for the new
    predicates and refactored seams; characterization tests pass on the green
    baseline), followed by the full Ship quality gate when Ship executes the
    task.
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
* High runtime, rollout, or rollback risk: **present for F3, contained.** F3
  touches queue dependency, shipment releasability, parent/child completion,
  archive composition, and gate-target decision surfaces, so it carries a manual
  monitoring checklist, an observation window, and a named rollback trigger (see
  Runtime Verification and Closure). The risk is contained by characterization-
  first testing and a behavior-equivalence acceptance bar, so this remains a
  bounded refactor rather than a hardening-triggering plan. Rollback is removal
  of the taxonomy/predicate wiring before merge. F2 has no runtime surface (an
  internal canonicalizer plus golden tests).

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
* **Release-observability (F3 is runtime-affecting).** F3 changes queue
  dependency, shipment releasability, parent/child completion, archive
  composition, and gate-target decision surfaces, so — even though formal PASS
  admission stays disabled — it carries a monitoring plan, observation window,
  and named rollback trigger. This repository has no live metrics system, so the
  plan is a manual observation checklist, per the release-observability contract:
  * **Signals / SLIs (manual):** across a representative corpus, no unintended
    change in (a) queue-ready item counts, (b) shipment releasability decisions,
    (c) parent-completion cascade outcomes, (d) archive/`archived_status`
    composition, and (e) gate-target evaluation.
  * **Baseline:** the characterization golden truth tables captured in F3's red
    phase.
  * **Observation window:** Ship's post-merge closure run — owner: Ship —
    covering the full quality-gate suite plus the representative smoke check.
  * **Rollback trigger (named):** any characterization/golden truth-table
    regression (pass rate below 100%) or any representative queue, shipment,
    parent-completion, archive, or gate-target decision differing from baseline
    triggers reverting the taxonomy wrapper and compatibility-seam edits before
    merge.
* **Closure:** Ship records that F2 is internal-only (no runtime surface) and
  that F3's manual monitoring checklist, observation window, and rollback trigger
  above are satisfied. Formal PASS-only admission is not enabled by this
  shipment.

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

**Capability overlays and task granularity (traceability):**

* **release-observability overlay:** assessed and satisfied for the
  runtime-affecting F3 unit — the Runtime Verification and Closure section
  provides the monitoring plan (manual Signals/SLIs checklist), baseline (F3
  red-phase golden truth tables), observation window (Ship post-merge closure,
  owner: Ship), and named rollback trigger. F2 is internal-only (no runtime
  surface). Recorded here so a reader does not conclude the overlay was
  unassessed.
* **backlogit overlay:** all task tracking uses `.backlogit/` registry-backed
  artifacts (106-F, 106.001-T, 106.002-T, shipment 114-S); no parallel task
  store is introduced.
* **Task Granularity (NON-NEGOTIABLE):** both F2 and F3 stay within the 2-hour /
  width-isolation / atomic-milestone rule via explicit `2-hour boundary` bullets
  and stop-and-restage conditions. The Expected-tests scenario counts (F2 ~7,
  F3 ~5) are consolidated into table-driven subtests so the "fewer than 4 test
  scenarios" heuristic proxies effort rather than capping table rows; effort per
  task remains within ~2 hours.

Constitution Check: pass

## Plan Review

dispatch_mode: multi-agent-dispatch
TOOL_OK: reviewer-subagent-dispatch
decision: ADVISORY
operator_authorization: approved

This section supersedes the initial post-harvest rubber-duck note with a full
multi-agent plan-review. Five selected personas were dispatched as independent
sub-agents and **all returned findings (complete coverage)**. The two
conditional cross-model personas were correctly not triggered: F2/F3 expose no
new MCP tool or agent-facing surface (Agent-Native Parity Reviewer) and touch no
auth/authz, secrets, external integrations, or sensitive data stores (Security
Lens Reviewer).

**Plan hardening:** required (F3 is runtime-affecting) and satisfied — monitoring
plan, observation window, and named rollback trigger are present in the Runtime
Verification and Closure section. **Constitution Check:** verdict present
(`Constitution Check: pass`).

**Gate progression (honest record).** The review of the as-harvested plan
surfaced **3 P1 findings**, which under the gate table is a FAIL. All three were
resolved in-place by revising this plan; the P2/P3 findings were folded into the
plan text and task acceptance criteria (a few carried forward as Ship-time
verification acceptance). After revision, **no unresolved P0/P1/P2 findings
remain**, so the re-gate decision on the revised plan is **ADVISORY**. Shipment
scope remains F2 + F3 only.

### Persona coverage

| Persona | Rubric | Dispatch | Verdict | Findings |
|---|---|---|---|---|
| Scope Boundary Auditor | always-on | sub-agent | ADVISORY | 2 P2 |
| Constitution Reviewer | always-on | sub-agent | ADVISORY | 1 P2, 2 P3 |
| Go Reviewer | always-on | sub-agent | ADVISORY | 2 P2, 5 P3 |
| Learnings Researcher | always-on | sub-agent | ADVISORY | 2 P1, 3 P2, 1 P3 |
| Architecture Strategist | cross-model | sub-agent | ADVISORY | 1 P1, 5 P2, 1 P3 |

### P1 findings (all resolved in this revision)

* **P1 — Architecture Strategist — F2 re-routing `gateReportHash` would mutate
  durable append-only evidence hashes with no scheme-version tag.** Resolution:
  the seam re-routing is **deferred to F1** (which owns the broker report payload
  contract and adds a hash-scheme version field to the evidence event). F2 is
  scoped to the canonical primitive plus a characterization test that documents
  the current raw hash without changing any recorded value. (F2 Goal, Likely
  files, Expected tests, Acceptance criteria.)
* **P1 — Learnings Researcher — F3 must preserve the documented 6-status
  `core.TerminalStatuses` vs 4-status `isTerminalReleaseStatus` divergence and
  the tracked descope predicate (stash `A3C349DD`).** Resolution: F3 now pins
  both sets exactly as they diverge, introduces a **dedicated descope predicate
  distinct from releasable/terminal**, and keeps the five
  `isTerminalReleaseStatus` release-progression call sites behaviorally
  unchanged. (F3 Likely files, Expected tests, Acceptance criteria.)
* **P1 — Learnings Researcher — F2's shared canonicalizer must be a stdlib-only
  leaf (068-S codec-extraction Rule 1).** Resolution: `internal/canonical` is
  now specified as **stdlib-only** (imports zero internal packages) with an
  import-free package-assertion test and differential golden byte-equality
  tests. (F2 Likely files.)

### P2 findings (folded into plan / acceptance)

* Architecture Strategist: home the canonicalizer in a leaf, not
  `internal/core/gate/*`; keep the parent/child cascade a distinct predicate;
  migrate slice-ranging call sites off the exported `TerminalStatuses`;
  parameterize the gate-target predicate on the configured terminal set; treat
  `archived` terminality literally and scope `archived_status` composition to the
  restore path. **All folded in.**
* Scope Boundary Auditor: golden vector (b) is no longer labeled F1's
  manifest-binding payload; F2's completion seam is enumerated and other hash
  paths explicitly deferred. **Folded in.**
* Constitution Reviewer: Constitution Check now maps the release-observability
  and backlogit overlays and the Task Granularity rule. **Folded in.**
* Go Reviewer: exported mutable `TerminalStatuses` slice is replaced by an
  immutable predicate/accessor over an unexported set; the fail-closed number
  path uses exported sentinel/typed errors asserted via `errors.Is`. **Folded
  in.**
* Learnings Researcher: `omitempty` forbidden on hashed collection/optional
  fields (empty vs absent pinned by golden vectors); hashed-payload timestamps
  normalized to UTC `Z` via `models.NowUTC()`; predicates that need
  `archived_status` read the Markdown source (index omits it) and fail closed on
  empty/unrecognized status via a recognized-status allowlist. **Folded in.**

### P3 findings (folded in / acknowledged)

Characterization tests pass on a green baseline while red-before-green applies to
the new predicates and seams; fail-closed non-integer handling plus recorded RFC
8785 (JCS) divergences; pinned Go surface (`Canonicalize(v any) ([]byte, error)`
+ hash helper, `json.Number` handling, no `context.Context`); "no second hash
path" scoped to the new F2 payloads plus a structural grep/lint guard;
children-terminal added to the acceptance predicate list; table-driven `t.Run`
subtests with a `t.Helper()` golden helper; the seam referred to by its source
identifier `gateReportHash`; taxonomy backed by unexported sets with a
no-external-mutation test. **All folded in.**

### Runtime verification & operational closure

Present and adequate. F3 carries a manual monitoring checklist, an observation
window (Ship post-merge closure, owner: Ship), and a named rollback trigger; F2
is internal-only with no runtime surface. No verification or closure gap remains
at the plan level.

Plan Review: ADVISORY (3 P1 surfaced and resolved in-place; no unresolved
P0/P1/P2 remain; P3 polish folded in). operator_authorization: approved.
