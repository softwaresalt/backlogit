---
title: "Deliberation: 195-S follow-up dispatch and closure contract repairs (227A2930, B83081F5)"
doc_type: "decision"
source: "docs/decisions/2026-10-03-195s-dispatch-closure-contract-deliberation.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: "Stage decision for the two 195-S READY_WITH_CONDITIONS follow-ups that block the next autonomous pipeline cycle: the Orchestrator must resolve, validate, and hand both served roots to Ship before claim, and shipment-reconcile pre-close must accept an explicit feature member that governed ShipShipment completes inside its own transaction."
topic: "Orchestrator served-root handoff to Ship and the shipment-reconcile explicit-feature lifecycle contract"
depth: "standard"
decision_status: "decided (operator delegation via the Orchestrator 'Run pipeline' autopilot instruction, 2026-10-03; recorded assumption below)"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md"
  - "docs/closure/195-S-claim-start-proof-post-merge-closure.md"
  - ".backlogit/reconcile/195-S-pre-20261003T051644Z.md"
  - "docs/exec-plans/2026-10-01-2a355f83-claimed-vs-started-bootstrap-plan.md"
stash_ids:
  - "227A2930"
  - "B83081F5"
tags:
  - "orchestrator-handoff"
  - "served-roots"
  - "shipment-reconcile"
  - "ship-closure"
  - "p-021-c6"
---

# Deliberation: 195-S follow-up dispatch and closure contract repairs

## Intake and Triage Record

| Stash | Kind | Priority | Marker | Source refs |
|---|---|---|---|---|
| `227A2930` | bug | high (raised from medium) | `DEFERRED SCOPE EXPANSION` (P-021 C6 forced deliberation) | PR #471; thread `PRRT_kwDORzozKM6ojnF3`; task `195.003-T`; feature `195-F`; shipment `195-S` |
| `B83081F5` | bug (reclassified from task) | high (raised from medium) | none | `docs/closure/195-S-claim-start-proof-post-merge-closure.md`; `.backlogit/reconcile/195-S-pre-20261003T051644Z.md` |

P-021 C5/C6 triage outcomes for `227A2930`:

* **Duplicate scan (A), unconditional:** clean. No other active stash entry describes the
  served-root handoff. Searched all 109 active entries for served-root and authoritative-root
  phrasing.
* **Late-identifier reconciliation (B):** not triggered. No source-ref field is recorded as
  `N/A`. The PR number, thread ID, and task, feature, and shipment IDs are all concrete.

Grouping decision: the Orchestrator proposed `227A2930` with `41FE00A1` (both edit
`_orchestrator.agent.md`) and `B83081F5` alone. Stage instead groups `227A2930` with
`B83081F5`, for these reasons:

* Both are conditions recorded in the 195-S `READY_WITH_CONDITIONS` closure.
* Both block the *next* autonomous Ship cycle. `227A2930` blocks it at the start (wave
  admission) and `B83081F5` at the end (pre-close).
* One shipment carrying both repairs needs a single bootstrap. Its own post-merge closure
  runs on the repaired reconcile contract from `main`.
* `41FE00A1` is deferred. It edits a different Orchestrator section (the Step 1.5 carve-out),
  needs its own secret-safety deliberation, and is medium priority.

Operator delegation (recorded assumption): the operator issued "Run pipeline" with
autopilot active and the instruction to make reasonable decisions. The Orchestrator delegated
the final grouping decision to Stage and asked for reviewed backlog structure. Stage treats
that as delegated confirmation of this deliberation outcome so the work can proceed to
planning. The operator may reverse the decision before Ship claims the resulting shipment.

## Problem Frame

### P1: Ship cannot be dispatched without both served roots (`227A2930`)

PR #471 (`195.003-T`) made Ship Step 4.0 and Step 4.1b require two absolute roots: the
**served storage root** and the **served workspace root**. Ship reads
`<served storage root>\logs\<id>.jsonl` from the storage root and uses the workspace root
as the CLI `--cwd`. Ship must never infer either root from its worktree. When either is
unknown at wave admission, Ship halts with `WAVE_CLAIM_STATE_INDETERMINATE` and reports an
Orchestrator follow-up.

Orchestrator Step 2 ("Route to Ship") passes only the `shipment_id` and the resolved
model-routing directive (step 5). Nothing tells it to obtain, validate, or pass the roots.
Wave 1 of every shipment contains claim-assigned (`active`) members. Without a change, the
next Ship dispatch of any of the 33 queued shipments fails closed, or the Orchestrator
improvises an unverified root.

Who cares: the operator, because unattended (dark-mode) pipeline cycles stall at dispatch,
and Ship, because a guessed root would defeat the raw-log path-safety procedure.

### P2: Pre-close fails on every explicit feature member (`B83081F5`)

The shipment-reconcile skill expects every explicit manifest member to reach `done` in
pre-close (`mode: pre`, `expected_status: done`), and safe-close step 2 requires every
non-pre-archived explicit member to be `done`. Two facts conflict with that:

* Governed `ClaimShipment` activates an explicit feature member directly
  (`TestClaimShipmentFlatScope_ExplicitFeatureMemberActivatesDirectly`).
* Governed `ShipShipment` sets each explicit feature root to `done` ("feature released") and
  archives it inside its own locked transaction (`internal/core/shipment_lifecycle.go`,
  `featureScopeRoots`, then `setArtifactStatus(... StatusDone, "feature released")`).

So an explicit feature member is legitimately `active` at pre-close. For 195-S, all seven
tasks were `done` and pre-archived, but `195-F` was `active`. The result was
`RECONCILE_FAIL`. Ship has no governed operation to move the feature. Closure needed Stage to
move `195-F` to `done` across role boundaries, and only then did a second pre-close pass.
Stage places a covering feature first in every manifest, so every future closure repeats
this unless the contract changes.

### Success criteria

* The Orchestrator resolves both roots read-only, validates them, binds them to the backlogit
  MCP server that serves the shipment, and passes them to Ship together with the
  `shipment_id`. Any failure halts dispatch with a named token and never invokes Ship.
* Pre-close accepts an explicit feature member in `active` state when at least one explicit
  task member exists and every explicit task member is `matched` or `pre-archived`. A `done`
  feature stays `matched`. Every other feature state still halts.
  Safe-close and post-close keep governed closure as the only path, and post-close still
  requires a valid archive record for the feature.
* Both contracts are pinned by tests that fail before the text changes.
* The installed-artifact drift records name the new local drift, so a later tune
  (`731CE551`) cannot silently revert it.

### Scope boundaries (out of scope)

* Any Go change to `ClaimShipment` or `ShipShipment` behavior. The Go lifecycle is already
  correct; this work only pins it with a characterization test. If the characterization test
  shows a defect, Ship records a P-021 deferred expansion and returns to Stage instead of
  widening this shipment.
* A new MCP tool or field that reports served roots (Option P1-B below). This is captured as
  a possible follow-up, not built now. *Amendment 1: no new surface is needed, because
  existing read-only surfaces already self-report the served roots. See Amendment 1.*
* Edits to `_ship.agent.md`. Ship already states the requirement; this work only supplies
  the missing producer. *Superseded by Amendment 1: Ship-side use-time attestation is now in
  scope.*
* The Step 1.5 continuity allowlist (`41FE00A1`), the model-routing re-render (`731CE551`),
  and docs compaction (`359D8F32`).
* The upstream autoharness templates. Local drift is recorded in the manifest. The upstream
  hand-off is bundled with the `731CE551`/`41FE00A1` tune work.

## Research Findings

* **Ship contract text:** `.github/agents/_ship.agent.md` covers this in Step 4.0 item 4
  (served roots required before wave admission, fail closed, "Never infer either root") and
  in Step 4.1b, including the shared raw-log path-safety procedure:
  * canonicalize both roots
  * require the storage/logs root to be contained within the workspace root
  * reject symlink and reparse-point components
  * require the storage directory to be exactly `.backlog` or `.backlogit`
* **Orchestrator contract text:** `.github/agents/_orchestrator.agent.md` Step 2 steps 1–7.
  Step 5 passes `shipment_id` and the routing directive only.
* **Existing contract tests:**
  * `tests/integration/claim_start_admission_contract_test.go` pins Ship's served-root text.
  * `tests/integration/shipment_155_harness_contract_test.go` pins shipment-reconcile
    flat-membership text and some Orchestrator text.
  * Both read the installed files and normalize whitespace. That pattern is reused here.
* **Go lifecycle:**
  * `internal/core/shipment_test.go` `TestShipShipment_FeatureInclusiveManifestArchivesFeature`
    covers an `active` feature with `active` tasks at ship time.
  * No test pins the exact 195-S shape: explicit feature `active` with explicit tasks already
    `done` and pre-archived.
* **Compound learnings** (via learnings-researcher; scope 1 confidence low, scope 2 medium):
  * `2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md` and
    `2026-07-27-absolutize-filepath-rel-base-for-absolute-walked-paths.md`: absolutize roots
    explicitly. A relative root silently double-joins.
  * `2026-07-07-empty-head-fail-closed-repo-presence-probe.md`: fail closed when state
    cannot be proven.
  * `security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md`:
    every dispatch entry point must pass through the same guard. This covers Step 2 routing
    and the dark-mode cursor advance.
  * `2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md` and
    `2026-08-18-shipment-shipped-prevention-envelope.md`: ShipShipment owns explicit feature
    completion inside its envelope, so the reconcile checks should expect that.
  * `best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md`:
    validators should accept an explicit set of allowed lifecycle states.
* **Registry:** `.autoharness/backlog-registry.yaml` declares `directory: ".backlog"`, while
  this repository still serves `.backlogit`. Both are supported, so root resolution must
  discover the actual storage directory rather than read the registry default.

## Options Evaluated

### P1 (served roots)

#### Option P1-A: Orchestrator resolves roots locally and binds them to MCP (recommended)

The Orchestrator derives the roots read-only from its own main checkout:

1. The served workspace root is the canonical absolute `git rev-parse --show-toplevel`, and
   the checkout must be the main worktree, not a linked worktree.
2. The served storage root is whichever of `.backlog` or `.backlogit` exists under it. If
   neither exists, or both exist, fail closed.

It then applies Ship's validation:

* absolute and canonical
* no symlink or reparse-point component
* the storage root is a direct child of the workspace root

Finally it binds the roots to the served store: the MCP `backlogit_get_shipment` result must
match the on-disk `<storage root>/queue/<shipment_id>.md` record by `id` and manifest
`custom_fields.items`.

* Pros: text-only change to one installed agent file. Reuses Ship's vocabulary and
  validation. No new MCP surface. Fails closed on every ambiguity.
* Cons: the binding proof is content-equality evidence, not a server self-report. A
  byte-identical second checkout could in principle satisfy it. P-016 already forbids parallel
  implementation worktrees, and the main-worktree check excludes linked worktrees.
* Effort: low. Fit: high.

#### Option P1-B: Add a read-only MCP self-report of served roots

Extend `backlogit_get_version` or `backlogit_doctor`, or add a new tool, to return the
server's resolved `workspace_root` and `storage_root`. The Orchestrator would consume that.

* Pros: authoritative server self-report.
* Cons:
  * New public MCP contract, with parity, schema, and docs work.
  * Exposes absolute local paths through a tool surface.
  * Needs a released binary before agents can rely on it, which lengthens the bootstrap.
  * Still needs the Orchestrator-side validation from P1-A.
* Effort: medium. Fit: medium.
* *Amendment 1: the "new public MCP contract" premise was false. The self-report already
  exists, so a new field is unnecessary.*

#### Option P1-C: Operator supplies roots at each dispatch

* Pros: trivial.
* Cons: breaks unattended and dark-mode dispatch, and gives no validation. Rejected.

### P2 (reconcile vs. ShipShipment)

#### Option P2-A: Phase-aware explicit-feature expectation in shipment-reconcile (recommended)

The pre-close classification rules change as follows:

* An explicit **feature** member whose queue record is `active` is classified
  `feature-pending-governed-completion`. That classification is accepted only when at
  least one explicit **task** member exists and every explicit **task** member is
  `matched` or `pre-archived`. A manifest with no explicit task member does not qualify.
* A feature that is `done` remains `matched`. A feature that is pre-archived remains
  `pre-archived`.
* Any other feature status (`queued`, `blocked`, `review`, and so on) remains
  `status-mismatch`.

Safe-close step 2 accepts an `active` explicit feature on the same condition. The
`ShipShipment` result envelope must then archive it (it must appear in `archived_ids`).
Post-close keeps the feature's valid archive requirement. Intake and resume pre-mode are
unchanged.

A Go characterization test pins that `ShipShipment` completes and archives an `active`
explicit feature whose explicit tasks are already done and archived (the 195-S shape).

* Pros:
  * Aligns the skill with the governed Go lifecycle.
  * No new authority for Ship.
  * The gate stays closed for every state ShipShipment does not handle.
  * Membership stays flat and explicit, because the classification reads only explicit
    members (the explicit feature and the explicit tasks) and never infers children.
* Cons: adds one classification row and touches pre-close plus safe-close text.
* Effort: low. Fit: high.

#### Option P2-B: Give Ship a governed feature-to-`done` move before pre-close

* Pros: the pre-close text stays unchanged.
* Cons:
  * Adds lifecycle authority to Ship.
  * Duplicates what ShipShipment already does inside its locked transaction.
  * Creates a non-transactional window where the feature is `done` before the gate runs.
  * Contradicts "Governed closure only … does not synthesize per-item transitions".
* Effort: medium. Fit: low.

#### Option P2-C: Exclude features from pre-close status checks

* Pros: smallest text change.
* Cons: a `queued`, `blocked`, or otherwise wrong feature would pass silently, which weakens
  the gate. Rejected.

## Trade-off Comparison

| Criterion | P1-A | P1-B | P1-C | P2-A | P2-B | P2-C |
|---|---|---|---|---|---|---|
| Complexity | low | medium | trivial | low | medium | trivial |
| Risk | low (residual: content-equality binding) | medium (new MCP surface) | high (no validation) | low | medium (authority + window) | high (gate weakened) |
| Unattended dispatch | yes | yes, after release | no | n/a | n/a | n/a |
| Authority change | none | none | none | none | Ship gains a move | none |
| Fits constraints | high | medium | low | high | low | low |

## Decision

Adopt **P1-A** and **P2-A** as one covering feature, "195-S follow-up: Orchestrator
served-root handoff and explicit-feature reconcile contract", shipped as one shipment. The
work is test-first:

* Contract tests are written RED first for both text surfaces.
* A characterization test pins the governed Go lifecycle.
* The Orchestrator and skill text edits follow.
* A final configuration task records the manifest drift for both installed artifacts.

Rationale:

* P1-A and P2-A are the only options that close each condition without new authority or
  surfaces, and both keep fail-closed behavior for every unhandled state.
* Combining them means one bootstrap: the Orchestrator dispatching this shipment applies the
  P1-A procedure from the reviewed plan before it lands on `main`. The shipment's own
  post-merge closure then runs on the repaired reconcile contract instead of the 195-S
  cross-role workaround.

## Rejected Alternatives

* **P1-B:** deferred as a hardening follow-up (new public MCP contract, release dependency).
  Not needed to close the condition. *Amendment 1 adopts the self-report P1-B wanted through
  existing surfaces, with no new contract.*
* **P1-C:** incompatible with unattended dispatch.
* **P2-B:** adds Ship lifecycle authority and a non-transactional window. It duplicates
  ShipShipment.
* **P2-C:** weakens the gate for states ShipShipment does not handle.
* **Grouping `227A2930` with `41FE00A1`:** different section and risk class. `41FE00A1` stays
  deferred for its own secret-safety deliberation.

## Unresolved Questions

* Should P1-B (MCP self-report of served roots) be stashed as a follow-up? Stage recommends
  stashing it only if the content-equality binding proves insufficient in practice. It is not
  captured now, to avoid speculative backlog. *Resolved by Amendment 1: the PR #474 review
  showed the content-equality binding was insufficient.*
* Bootstrap for this shipment: the dispatching Orchestrator must apply the plan's served-root
  procedure (recorded in the plan's Dispatch Preconditions) before the text lands on `main`.
  If the Orchestrator cannot resolve and bind the roots, it halts and asks the operator. It
  never guesses.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| The content-equality binding is satisfied by a stale second checkout | Main-worktree requirement, P-016 no parallel worktrees, and comparison of the `id` plus the exact ordered `custom_fields.items`. *Amendment 1 replaces content equality with the Served-Root Attestation* |
| The reconcile change accepts a feature that ShipShipment then fails to archive | The safe-close result envelope must list the feature in `archived_ids`, post-close still requires a valid feature archive, and the Go characterization test pins the behavior |
| A later tune or re-render (`731CE551`) reverts the local edits | Manifest drift records (`drift_allowed`, `drift_reason`) for both installed files, and `731CE551` is sequenced after this shipment |
| The CI paths-filter gap for harness-text tests (`B3701713`) hides the RED/GREEN contract tests in CI | Ship runs the governed full suite locally (`go test -timeout=30m ./...`), and the gap stays tracked under `B3701713` |
| The edit renumbers Step 2 sub-steps and breaks cross-references ("proceed to step 4/5") | The plan requires the handoff as a sub-step of the existing step 5, with no renumbering |

## Amendment 1: Served-Root Attestation (PR #474 review)

* **Trigger:** PR #474 review thread `PRRT_kwDORzozKM6olYEw` on `196.002-T`. The P1-A
  binding compares content that a workspace copied together with its gitignored `logs`
  directory can reproduce. The MCP server stays bound to its startup root
  (`internal/cli/root.go`, `internal/mcp/server.go` `newServer`), and Ship checks path
  containment but never re-attests the server root. If the stores diverge, MCP writes and raw
  reads or CLI writes can target different workspaces.
* **Authorization:** the operator directed "PR 474: Additional Copilot review comments to
  fix" at 2026-10-03T21:07-07:00, relayed by the Orchestrator as authorization to amend the
  approved plan and the harvested items.
* **Finding during the amendment:** the P1-B premise ("new public MCP contract") was false.
  Two existing read-only surfaces already self-report the served roots:
  * `backlogit_get_metadata_catalog` returns `workspace.root_path` and
    `workspace.storage_root`.
  * `backlogit_query_sql` accepts `SELECT name, file FROM pragma_database_list WHERE name =
    'main'`, which names the index file the live server connection opened, under the served
    storage root.
* **Decision:** P1-A is kept for root resolution. The content-equality binding is replaced
  by a read-only **Served-Root Attestation** against those two surfaces, run before any raw
  read:
  * by the Orchestrator at dispatch (plan U2)
  * by Ship at use time (plan U8 and U9): at each wave admission, a `pragma_database_list`
    check at each task claim, and before the Step 4.1b CLI fallback. A failure halts with
    `SERVED_ROOT_ATTESTATION_FAILED`.
  * A characterization test (plan U7) pins both surfaces. The static manifest check stays,
    proving the named shipment is present and consistent in the attested store.
* **Alternatives rejected for the amendment:**
  * A freshness nonce written through MCP as a comment and read back raw. It adds a write
    and log noise to every dispatch and wave admission. The Orchestrator would also need a
    backlog write it otherwise lacks. It proves the same binding less directly.
  * A new Go MCP field. Unnecessary, because the self-report already exists.
* **Scope change:** edits to `_ship.agent.md` are now in scope for the use-time attestation.
  Direct Ship invocation intake, halt mappings, payload key pinning, and attestation of
  Ship's other backlogit invocations stay in follow-up stash `CDBCB258`.
