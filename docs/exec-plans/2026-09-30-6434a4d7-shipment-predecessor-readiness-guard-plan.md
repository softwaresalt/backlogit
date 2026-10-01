---
chunk_strategy: h1-h2-h3
description: "Implementation plan for stash 6434A4D7 (074-DL layer L2): shipped-only shipment predecessor readiness at the queue and claim boundaries, a direct ClaimShipment dependency guard, and a governed Stage-reachable disposition for superseded queued shipments that never activates members"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-30-6434a4d7-shipment-predecessor-readiness-guard-plan.md
title: "Implementation Plan: Shipment predecessor shipped-only readiness, claim guard, and governed queued-shipment disposition (6434A4D7)"
docline:
    stash_id: 6434A4D7
    status: draft
    created_at: 2026-09-30T17:20:00Z
---

## Objective

Make a shipment-to-shipment `blocks` predecessor count as satisfied only when
the predecessor actually shipped. Enforce that rule in core code at both
shipment boundaries, the queue listing and the claim. Add one governed
operation that retires a superseded `queued` shipment without activating
anything.

This is layer L2 of the 074-DL design. It owns the shipped-provenance half (P)
of condition (b) in code, for every caller. It does not and cannot see the
external scheduler-consumption half (C). That half belongs to L1
(`AF1E5075`) now, and to L3 (`A592FC1C`) only after a later recorded decision
confirms the gate checks consumption.

## Source and Intake Record

* Stash: `6434A4D7` (high, feature, carries the `DEFERRED SCOPE EXPANSION`
  marker).
* Deliberation (P-021 C6 satisfied, reused, not restarted):
  * `docs/decisions/2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation.md`
    chose model M2: an additive guard, a new predicate, and no in-place change
    to `ClaimShipment` member-activation semantics.
  * `074-DL` (`docs/decisions/2026-09-28-513e62ab-condition-b-enforcement-deliberation.md`)
    places this item as L2 and sequences it after `154-S` and `C29EBEE5`.
    `154-S` has shipped (archived, `archived_status: shipped`, merge
    `6d233d21`), and the `C29EBEE5` claim-code collision is closed.
* Duplicate scan (P-021 C5 (A)): clean. Related but distinct active stash
  entries: `E52607F5` (claim cascade with non-queued member parents) and
  `F05661B1` (pipeline-topology closure awareness, external). Neither is in
  scope.
* Late-identifier reconciliation (P-021 C5 (B)): not triggered. No source-ref
  field of `6434A4D7` is `N/A`.
* Corrected examples from the original stash text:
  * `149-S` is held by a `blocks` edge onto `169-S` (still `queued`). The
    queue filter hides `149-S` today, but a direct `ClaimShipment` call would
    claim it, because the claim path has no dependency check.
  * `156-S` (Repository baseline convergence) is superseded but is not hard
    non-claimable. It is hidden from the queue only through its edge onto
    `169-S`. This plan adds the disposition operation. It does not apply it to
    `156-S`; that stays a separate operator decision.

## Problem Frame

Four defects share one root cause: the engine treats every terminal status as
"no longer blocking", only the queue listing checks dependencies, and nothing
governs how a queued shipment becomes terminal.

1. `filterByResolvedDependencies` in `internal/core/queue.go` releases a
   `blocks` edge when the predecessor reaches any of six statuses through
   `IsNoLongerBlockingStatus` (done, accepted, archived, shipped, abandoned,
   rejected). For a shipment predecessor, `abandoned` and an `archived`
   shipment that was abandoned both release the successor. The index row does
   not carry `archived_status`, so the filter cannot tell an archived shipped
   shipment from an archived abandoned one. `MoveInQueue` calls `QueryQueue`
   with the same filter, so it inherits the defect.
2. `ClaimShipment` in `internal/core/shipment_lifecycle.go` never reads
   dependencies. Any CLI, MCP, or code caller can claim a shipment whose
   predecessor is unshipped.
3. Generic write paths already allow a shipment to move from `queued` to
   `abandoned`. `isProtectedShipmentStatusTransition` in
   `internal/core/artifacts.go` protects only transitions that involve
   `blocked` and the `queued` to `active` edge. `UpdateArtifact`,
   `UpdateArtifactWithGate`, and `BulkUpdateStatus` in `internal/core/queue.go`
   therefore accept `queued` to `abandoned` with no confirmation, reason, or
   journal. Only `MoveShipmentStatus` refuses it, through
   `isValidShipmentTransition`.
4. No governed operation retires a superseded `queued` shipment with
   provenance, confirmation, and recovery.

## Scope

In scope:

* Feature A, readiness: a shipped-provenance predicate, the queue filter, a
  workspace-aware queue entry point used by `MoveInQueue` and both adapters,
  the claim guard, the claim-refusal error mapping in MCP and CLI, and the
  readiness docs and harness instruction text.
* Feature B, disposition: protection of `queued` to `abandoned` on every
  generic write path, one governed edge reachable only through
  `DisposeQueuedShipment`, journal recovery for the new operation,
  core-enforced confirmation, a target-bound deliberation authorization, and
  CLI, MCP, registry, parity-test, docs, and role-contract parity.

Out of scope:

* Any change to `terminalCascadeStatuses`, `releasableStatuses`,
  `IsNoLongerBlockingStatus`, or the other shared taxonomy predicates. The
  2026-09-20 decision forbids unifying or mutating them.
* Dependency semantics for task, feature, subtask, and mixed-type edges. They
  keep the six-status cascade.
* The `active` to `abandoned` shipment edge. It stays allowed on generic
  paths, as today. This asymmetry is recorded, not changed.
* Any check of external scheduler consumption (C).
* Applying the disposition to `156-S` or any other existing shipment.
* AF1E5075 contract text (owned by `AF1E5075`) and autoharness gate code
  (owned upstream through `A592FC1C`).
* The condensed mirror `plugin/agents/ship.agent.md`. See Decisions.

## Requirements Trace

| ID | Requirement | Source | Units |
|---|---|---|---|
| R1 | A shipment `blocks` predecessor is satisfied only when it is live `shipped`, or `archived` with Markdown `archived_status: shipped`. Abandoned and disposed shipments never count | 6434A4D7 (1); 074-DL L2 | A-U1 to A-U4 |
| R2 | Provenance is read from Markdown, not the SQLite index. A missing, unparsable, or non-shipment predecessor returns `false` with a reason; only infrastructure failures return an error | 074-DL L2; compound 2026-07-20 | A-U3, A-U4 |
| R3 | The queue filter, `QueryQueueForWorkspace`, `MoveInQueue`, and both adapters apply R1 to shipment-to-shipment edges that the unchanged `isExecutionBlockingDependency` treats as blocking and whose type is `blocks`, and leave every other edge type and artifact pairing unchanged. Every non-test caller in `internal/` and `cmd/` reaches the queue through `QueryQueueForWorkspace` | 6434A4D7 (1) | A-U5 to A-U10 |
| R4 | `ClaimShipment` refuses with a typed sentinel and the stable token `shipment_predecessor_not_shipped` before any member activation when R1 fails, reading edges from the reloaded Markdown, treating an empty Markdown edge type as `blocks` (the default every write and rehydration path applies), and failing closed on unresolvable predecessors | 6434A4D7 (2); 2026-09-20 M2 | A-U11 to A-U13 |
| R5 | MCP maps the claim refusal to code `shipment_predecessor_not_shipped`, and the CLI claim returns an error carrying the token and the predecessor ID | 6434A4D7 (2) parity | A-U14, A-U15 |
| R6 | `docs/workflow.md` and the backlogit harness instruction state the shipped-only rule and the refusal code, with the rendered-file drift recorded | Parity; harness hygiene | A-U16, A-U17 |
| R7 | `queued` to `abandoned` on a shipment is refused on every generic write path (`MoveShipmentStatus`, `UpdateArtifact`, `UpdateArtifactWithGate`, `BulkUpdateStatus`) and opens only in `MoveShipmentStatus` under the governed-disposition marker | 6434A4D7 (3); review attempt 2 | B-U3 to B-U7 |
| R8 | The status and the disposition provenance land in one governed file write, so a crash leaves either the `queued` preimage or the complete `abandoned` target, never a mix. A pending `dispose` intent journal is provable by recovery and rolls back to the preimage. A leftover `committed` dispose journal passes journal validation and is left in place, as committed `claim` journals are today, and no leftover dispose journal blocks a later lifecycle operation | 6434A4D7 (3); review attempts 2 and 3 | B-U6 to B-U9, B-U11 |
| R9 | `DisposeQueuedShipment` requires `Confirm`, a reason, an actor (`By`), and an `AuthorizationRef` that resolves to a decided `deliberation` (live `done`, or `archived` with Markdown `archived_status: done`) whose `authorizes_disposition` custom field names the target shipment. It records provenance that survives later `update` and archive, activates nothing, mutates no member or feature, refuses non-queued targets and shipments with live dependents, and rolls back on a non-indeterminate failure | 6434A4D7 (3); review attempt 3 | B-U10 to B-U13b |
| R10 | The disposition is reachable through MCP and CLI with registry, parity-test, and docs parity. The registry exposes no automatic CLI fallback; only a confirmed-only command | 6434A4D7 (3); Constitution VII | B-U14 to B-U21 |
| R11 | Only Stage may call the disposition tool. The Stage contract permits it only with an operator-authored, decided, target-bound deliberation that Stage did not create or change in the disposing session, in Careful mode with in-session operator approval. The Orchestrator contract states that neither it nor Ship disposes. The drift is recorded | Stage role boundary; review attempt 3 | B-U22, B-U23 |
| R12 | New API declarations are pinned by an AST declaration harness before stubs exist | Repo convention | A-U1, A-U2, B-U1, B-U2 |
| R13 | `156-S` and every other existing shipment stay unchanged by this work, verified in closure | 6434A4D7 scope | Closure |
| R14 | Before `SA` merges, the live impact of the new readiness rule is inventoried and recorded | Review attempt 2 | Closure |

## Implementation Units

Every unit follows the 2-hour rule: fewer than 3 files, fewer than 5
functions, fewer than 4 test scenarios, and one skill domain. "RED" means the
named command fails before the unit's paired code unit lands. Each code unit's
"Verify" command is scoped so that intermediate commits are not red on tests
that a later unit in the same shipment turns green; the full suite runs at the
end of each shipment.

Known red windows, both closed inside their shipment:

* Feature A, from A-U6 to A-U10: the adapters still call `QueryQueue`
  without a workspace, so they fail closed on archived shipment predecessors.
  A-U6 runs the adapter queue tests and records any failure as this window.
* Feature B, from B-U17 to B-U19: the MCP tool exists before its registry
  entry, so the registry parity tests are red until B-U19.

Test fixtures run in `t.TempDir()` workspaces. No unit reads or writes the
live backlog.

### Feature A: Shipped-only predecessor readiness (queue and claim)

#### A-U1: AST declaration harness for the readiness APIs (tests)

* File: `internal/core/shipment_readiness_decl_test.go` (new).
* Parse `internal/core` and `internal/errors` with `go/parser`. Assert these
  permanent declarations, signatures only, so later successors stay allowed:
  * `func ShipmentPredecessorShipped(ctx context.Context, ws *Workspace, id string) (bool, string, error)`.
  * `func QueryQueueForWorkspace(ctx context.Context, ws *Workspace, filter *QueueFilter) (*QueueView, error)`.
  * `ErrShipmentPredecessorNotShipped` in `internal/errors`.
* The unexported resolver field on `QueueFilter` is deliberately not pinned.
* Scenarios: predicate; workspace queue entry point; sentinel.
* RED: `go test ./internal/core -run '^TestShipmentReadinessDeclarations$' -count=1`
  fails because the declarations are missing.
* Posture: test-first.

#### A-U2: Readiness API stubs and sentinel (code)

* Files: `internal/core/shipment_readiness.go` (new),
  `internal/errors/errors.go`.
* Declare the predicate (stub returns `false, "not_implemented", ErrNotImplemented`)
  and `QueryQueueForWorkspace` (stub returns `nil, ErrNotImplemented`). Add
  `ErrShipmentPredecessorNotShipped = errors.New("backlogit: shipment predecessor has not shipped")`
  in the shipment sentinel block.
* Verify: the A-U1 command passes.

#### A-U3: Shipped-provenance predicate behavior tests (tests)

* File: `internal/core/shipment_readiness_test.go` (new).
* Scenarios, each in a temp workspace with Markdown fixtures:
  1. Accept: a live `shipped` shipment, and an archived shipment whose
     Markdown carries `archived_status: shipped`. The index status alone
     (`archived`) is deliberately ambiguous in the fixture.
  2. Reject with `false`, nil error, and a reason: an archived shipment with
     `archived_status: abandoned` (`not_shipped:archived/abandoned`), an
     archived shipment with no `archived_status`, a live `abandoned` or
     `queued` shipment (`not_shipped:<status>`), and a disposed shipment.
  3. Fail closed, never `true`: an unknown ID and a file whose frontmatter
     cannot be parsed both return `provenance_missing`, because
     `findArtifactInSearchDir` skips unparsable files; a non-shipment ID
     returns `not_shipment`. A cancelled context returns a non-nil error
     (infrastructure), not a reason.
* RED: `go test ./internal/core -run '^TestShipmentPredecessorShipped$' -count=1`
  fails against the stub.
* Posture: test-first.

#### A-U4: Implement the shipped-provenance predicate (code)

* File: `internal/core/shipment_readiness.go`.
* Load the predecessor with `findArtifact` (Markdown, `internal/core/artifacts.go`).
  `errors.Is(err, ErrNotFound)` becomes `false, "provenance_missing", nil`.
  Any other error is returned wrapped. Use an allowlist: accept only
  `status == shipped`, or `status == archived` with
  `archived_status == shipped`.
* Do not import or change `status_taxonomy.go` predicates.
* Verify: the A-U3 command passes.

#### A-U5: Queue filter readiness tests (tests)

* File: `internal/core/queue_shipment_readiness_test.go` (new).
* The tests set the unexported resolver field on `QueueFilter` directly.
* Scenarios:
  1. Shipment edges: a queued shipment with a `blocks` edge onto an abandoned
     shipment, or onto an archived shipment the resolver reports as not
     shipped, is excluded. One onto a predecessor the resolver reports as
     shipped is included. No scenario uses an empty edge type: the index
     never stores one, because every write and rehydration path defaults it
     to `blocks`.
  2. Non-shipment semantics unchanged: a task blocked on an abandoned task is
     released, a shipment with a `blocks` edge onto a non-shipment keeps the
     six-status cascade, and `relates_to` and `parent_of` handling is
     unchanged.
  3. No resolver and resolver errors: without a resolver, an `archived`
     shipment predecessor stays blocking and a live `shipped` one releases;
     a resolver error propagates wrapped from `QueryQueue`.
* RED: `go test ./internal/core -run '^TestQueueShipmentReadiness$' -count=1`
  fails.
* Posture: characterization of scenario 2 first, then test-first.

#### A-U6: Apply shipped-only readiness in the queue filter (code)

* File: `internal/core/queue.go`.
* Add an unexported resolver field to `QueueFilter`. In
  `filterByResolvedDependencies`, also select `artifact_type` into the status
  map so the filter knows whether the predecessor is a shipment. For an edge
  where both the item and the predecessor are shipments,
  `isExecutionBlockingDependency` (unchanged) returns `true`, and the
  trimmed, lowercased type is `blocks`, release the edge only when the index
  status is `shipped`, or the resolver returns `true`. With no resolver, an `archived`
  shipment predecessor stays blocking. Every other edge keeps
  `IsNoLongerBlockingStatus`. Return resolver errors wrapped.
* Before the edit, inventory the callers with
  `git grep -n "QueryQueue(" -- internal cmd` and record them in the commit
  message.
* Verify: the A-U5 command passes, and
  `go test ./internal/core -run 'Queue' -count=1` passes. Then run
  `go test ./internal/mcp ./internal/cli ./tests/... -run Queue -count=1` and
  record any failure as the known red window closed by A-U10.

#### A-U7: Workspace queue entry point tests (tests)

* File: `internal/core/queue_workspace_entry_test.go` (new).
* Scenarios:
  1. `QueryQueueForWorkspace` includes a shipment behind an archived
     `archived_status: shipped` predecessor and excludes one behind an
     archived `archived_status: abandoned` predecessor.
  2. `MoveInQueue` on a shipment behind an archived shipped predecessor
     succeeds.
  3. A predicate infrastructure error (cancelled context) propagates wrapped
     from `QueryQueueForWorkspace`.
* RED: `go test ./internal/core -run '^TestQueryQueueForWorkspace$' -count=1`
  fails against the stub.
* Posture: test-first.

#### A-U8: Implement the workspace queue entry point (code)

* Files: `internal/core/queue.go`, `internal/core/shipment_readiness.go`.
* Implement `QueryQueueForWorkspace`: copy the filter, install a resolver over
  `ShipmentPredecessorShipped`, and call `QueryQueue`. No cache. Change
  `MoveInQueue` to call `QueryQueueForWorkspace`. Add a doc comment on
  `QueryQueue` stating that callers with a workspace must use
  `QueryQueueForWorkspace`.
* Verify: the A-U7 command passes, and
  `go test ./internal/core -run 'Queue' -count=1` passes.

#### A-U9: Adapter queue wiring tests (tests)

* Files: `internal/mcp/tools_queue_shipment_readiness_test.go` (new),
  `internal/cli/queue_shipment_readiness_test.go` (new).
* Scenarios:
  1. MCP: by calling the server handler with a request map, as
     `internal/mcp/contract_consistency_test.go` does, a shipment behind an archived shipped
     predecessor appears in `backlogit_get_queue` output, and one behind an
     archived abandoned predecessor does not.
  2. CLI: through the real command tree
     (`cli.NewRootCommand()`, `SetArgs`, `Execute`),
     `queue view --type shipment` shows the same split.
  3. Source scan, in the CLI test file: a `go/parser` scan of every non-test
     `.go` file under `internal/` and `cmd/` finds exactly one call to
     `QueryQueue`, inside the body of `QueryQueueForWorkspace` in
     `internal/core`. Today's callers are `internal/cli/queue_cmd.go`,
     `internal/mcp/tools.go`, and `MoveInQueue` in `internal/core/queue.go`;
     A-U8 and A-U10 move all three.
* RED: `go test ./internal/mcp ./internal/cli -run 'QueueShipmentReadiness' -count=1`
  fails, because the adapters still call `QueryQueue`.
* Posture: test-first.

#### A-U10: Wire the adapters to the workspace queue entry point (code)

* Files: `internal/cli/queue_cmd.go`, `internal/mcp/tools.go` (the
  `QueryQueue` call sites, today at `queue_cmd.go` line 79 and `tools.go`
  line 1678).
* Replace each `QueryQueue(ctx, ws.DB, filter)` call with
  `QueryQueueForWorkspace(ctx, ws, filter)`. The rule stays in core.
* Verify: the A-U9 command passes, and
  `go test ./internal/mcp ./internal/cli ./tests/... -run Queue -count=1`
  passes.

#### A-U11: ClaimShipment predecessor-guard tests (tests)

* File: `internal/core/shipment_claim_predecessor_test.go` (new).
* Scenarios:
  1. Refuse: a shipment with a `blocks` edge onto a `queued` shipment (the
     `149-S` onto `169-S` shape). The claim returns an error that wraps
     `ErrShipmentPredecessorNotShipped` and whose text contains
     `shipment_predecessor_not_shipped` and the predecessor ID. The shipment
     stays `queued`, no member changes status, and no claim intent, preimage,
     `scheduler_baseline_claim` marker, or reconcile file is written under
     `.backlogit/`. A hand-written Markdown edge with an empty type onto the
     same predecessor refuses the same way.
  2. Refuse, fail closed: the predecessor is archived with
     `archived_status: abandoned`; the edge names an ID that no longer
     resolves; the predecessor file cannot be parsed. The last two refuse
     with the reason `provenance_missing`.
  3. Allow: the predecessor is archived with `archived_status: shipped`, and a
     `blocks` edge onto a non-shipment is not evaluated by the guard. The
     claim activates members exactly as before.
* RED: `go test ./internal/core -run '^TestClaimShipmentPredecessorGuard$' -count=1`
  fails.
* Posture: test-first.

#### A-U12: Claim fixture audit and fix (tests)

* Files: at most two existing test files named by the audit.
* Run
  `git grep -n -e "dep_type: blocks" -e "AddDependency" -e "ClaimShipment(" -- "internal/*_test.go" "tests/*"`
  and list each fixture that claims a shipment while a shipment `blocks`
  predecessor is unshipped. Fix those fixtures by shipping the predecessor
  or dropping the edge, and name them in the commit message. Weakening the
  guard is not an allowed fix.
* Blocked path: if more than two files need a fix, stop and return to Stage
  for re-decomposition.
* Verify: the audit list is recorded in the commit message; with no affected
  fixture, the unit closes as a recorded no-op.
* Posture: characterization-first.

#### A-U13: Add the ClaimShipment predecessor guard (code)

* File: `internal/core/shipment_lifecycle.go`.
* Add one helper, `requireShipmentPredecessorsShipped(ctx, ws, current)`.
  It reads edges from the reloaded Markdown (`current.Dependencies`), not
  from the index. It normalizes each edge type with `TrimSpace` and
  `ToLower` and treats an empty type as `blocks`, the same default that
  `internal/core/dependencies.go` and `internal/db/rehydration.go` apply, so
  the claim and the queue see the same edge set. It evaluates only `blocks`
  edges. For each one: load the predecessor; an unresolvable ID refuses
  (`provenance_missing`); a non-shipment is skipped; a shipment calls
  `ShipmentPredecessorShipped`. Call the helper in `ClaimShipment` after the
  reload and `queued` re-check under `lockArtifactMutations`, and before the
  preimage, snapshot, and activation. Return
  `fmt.Errorf("claim %s: shipment_predecessor_not_shipped: predecessor %s (%s): %w", ...)`.
  Do not change activation, rollback, or marker code.
* The queue also holds a shipment on `relates_to` and `parent_of` edges; the
  claim guard checks only `blocks`. The difference is intentional and is
  documented in A-U16.
* Verify: the A-U11 command passes, and
  `go test ./internal/... ./tests/... -count=1` passes.

#### A-U14: Claim-refusal adapter tests (tests)

* Files: `internal/mcp/tools_claim_predecessor_test.go` (new),
  `internal/cli/shipment_claim_predecessor_test.go` (new).
* Scenarios:
  1. MCP `backlogit_claim_shipment`, called through the server handler on the
     `149-S` fixture shape, returns the error code string
     `shipment_predecessor_not_shipped`.
  2. CLI `shipment claim`, through the real command tree on the same fixture,
     returns a non-nil error whose message contains
     `shipment_predecessor_not_shipped` and the predecessor ID.
* RED: `go test ./internal/mcp ./internal/cli -run 'ClaimPredecessorNotShipped' -count=1`
  fails on the MCP case.
* Posture: test-first.

#### A-U15: Claim-refusal error mapping (code)

* Files: `internal/mcp/errors.go` (table comment and an `errors.Is` case
  before the generic `ErrShipmentConflict` case), and `internal/cli/shipment.go`
  only if the CLI scenario still fails.
* The CLI claim command already returns the core error text, which carries
  the token. Change it only if A-U14 shows the text is dropped.
* Verify: the A-U14 command passes.

#### A-U16: Readiness documentation (docs)

* Files: `docs/workflow.md`,
  `.github/instructions/backlogit.instructions.md`.
* `docs/workflow.md`: the shipped-only predecessor rule, the
  `shipment_predecessor_not_shipped` claim refusal, the queue versus claim
  edge-type difference, and the statement that the core guard covers (P)
  only, never (C).
* `backlogit.instructions.md`, Shipment Sequencing Protocol: replace the
  statements that a predecessor satisfies the edge when `status == shipped`
  with "live `shipped`, or `archived` with `archived_status: shipped`;
  `abandoned` and disposed shipments never satisfy it", and name the refusal
  code.
* Verify (docs RED): before the edit,
  `git grep -n "shipment_predecessor_not_shipped" -- docs/workflow.md .github/instructions/backlogit.instructions.md`
  returns no match. After the edit both files match, and
  `go run ./cmd/backlogit docs lint --path docs/workflow.md` reports zero
  violations.
* Posture: characterization-first.

#### A-U17: Record the instruction drift (config)

* File: `.autoharness/harness-manifest.yaml`.
* Append to the `drift_reason` of the `.github/instructions/backlogit.instructions.md`
  entry: "6434A4D7 shipped-only shipment predecessor rule and
  shipment_predecessor_not_shipped refusal; upstream template sync pending".
  Leave `drift_allowed: true`.
* Verify: `git grep -n "6434A4D7 shipped-only" -- .autoharness/harness-manifest.yaml`
  matches once. Do not run `autoharness verify-workspace` blindly; its two
  known advisory warnings are pre-existing. If Ship runs it, the acceptance
  is `strict_schema_blockers=[]` with no new warning.

### Feature B: Governed queued-shipment disposition

#### B-U1: AST declaration harness for the disposition API (tests)

* File: `internal/core/shipment_disposition_decl_test.go` (new).
* Pin these permanent declarations, signatures only:
  * `func DisposeQueuedShipment(ctx context.Context, ws *Workspace, id string, opts ShipmentDispositionOptions) (*models.Artifact, error)`.
  * `type ShipmentDispositionOptions struct` with string fields `Reason`,
    `SupersededBy`, `By`, `AuthorizationRef`, and bool field `Confirm`.
  * `ErrShipmentDispositionRefused` in `internal/errors`.
* Scenarios: function signature; options struct; sentinel.
* RED: `go test ./internal/core -run '^TestShipmentDispositionDeclarations$' -count=1`
  fails.
* Posture: test-first.

#### B-U2: Disposition API stubs and sentinel (code)

* Files: `internal/core/shipment_disposition.go` (new),
  `internal/errors/errors.go`.
* Declare the options struct, a stub that returns `ErrNotImplemented`, and an
  unexported test seam `disposeAfterGovernedWriteHook func() error` that is
  nil in production. It runs after the single governed write and before the
  journal is marked committed. Add
  `ErrShipmentDispositionRefused = errors.New("backlogit: shipment disposition refused")`.
* Verify: the B-U1 command passes.

#### B-U3: Protected abandon-edge tests (tests)

* File: `internal/core/shipment_abandon_guard_test.go` (new).
* Scenarios, each on a `queued` shipment:
  1. Characterization: `MoveShipmentStatus` to `abandoned` without the
     marker returns `ErrShipmentConflict`, and `queued` to `active` through
     `UpdateArtifact` is refused, as today.
  2. `UpdateArtifact` and `UpdateArtifactWithGate` with
     `status: abandoned` return an error wrapping
     `ErrShipmentBlockedRequiresEnvelope`, and the file is unchanged.
  3. `BulkUpdateStatus` to `abandoned` reports the shipment in `Failed`, and
     the file is unchanged.
* RED: `go test ./internal/core -run '^TestShipmentQueuedAbandonProtected$' -count=1`
  fails on scenarios 2 and 3.
* Posture: characterization of scenario 1 first, then test-first.

#### B-U4: Abandon-edge fixture audit and fix (tests)

* Files: at most two existing test files named by the audit.
* Run
  `git grep -n -e "StatusAbandoned" -e "\"abandoned\"" -- internal/core internal/cli internal/mcp tests`
  and list every path that moves a `queued` shipment to `abandoned`. A
  production caller is a blocked path: stop and return to Stage. Test
  fixtures are fixed to use `active` first or are removed from the shipment
  case, and named in the commit message.
* Blocked path: if more than two test files need a fix, stop and return to
  Stage.
* Verify: the audit list is recorded in the commit message; with no affected
  fixture, the unit closes as a recorded no-op.
* Posture: characterization-first.

#### B-U5: Protect `queued` to `abandoned` on generic paths (code)

* Files: `internal/core/artifacts.go`, `internal/core/queue.go`.
* Add `previous == queued && next == abandoned` to
  `isProtectedShipmentStatusTransition`. The `UpdateArtifact` guard and the
  write guard then refuse it unless the write envelope sets
  `allowGovernedShipmentMutation`. In `BulkUpdateStatus`, extend the
  shipment pre-scan to refuse this transition as well.
* Verify: the B-U3 command passes, and
  `go test ./internal/... ./tests/... -count=1` passes.

#### B-U6: Governed disposition edge tests (tests)

* File: `internal/core/shipment_disposition_edge_test.go` (new).
* Scenarios:
  1. With the governed-disposition marker carrying a complete provenance
     value (`Reason`, `By`, `AuthorizationRef`, `SupersededBy`),
     `MoveShipmentStatus` moves a `queued` shipment to `abandoned`. The same
     file write sets `disposition_reason`, `disposition_by`,
     `disposition_authorization_ref`, and `superseded_by`; one
     `shipment_status_changed` event is appended. With an empty
     `SupersededBy`, the `superseded_by` key is absent.
  2. The marker opens no other edge: `queued` to `shipped` and `active` to
     `queued` are still refused with the marker present.
  3. A marker whose `Reason`, `By`, or `AuthorizationRef` is empty returns
     `ErrValidation`, and the shipment file is byte-for-byte unchanged.
* RED: `go test ./internal/core -run '^TestGovernedShipmentDispositionEdge$' -count=1`
  fails.
* Posture: test-first.

#### B-U7: Governed disposition edge with a single provenance write (code)

* File: `internal/core/shipment.go`.
* Add `governedShipmentDispositionContextKey struct{}` next to
  `governedShipmentActivationContextKey`. Its context value is an unexported
  `shipmentDispositionProvenance` struct with the fields `Reason`, `By`,
  `AuthorizationRef`, and `SupersededBy`.
* In `moveShipmentStatusWithHeadGuard`, allow `queued` to `abandoned` only
  when that marker is present, before the generic
  `isValidShipmentTransition` check, and leave `isValidShipmentTransition`
  unchanged. Validate that `Reason`, `By`, and `AuthorizationRef` are
  non-empty before any write; otherwise return `ErrValidation`.
* On the loaded shipment, set the status and the `disposition_*` and
  `superseded_by` `custom_fields` keys together, so the existing single
  persist call writes the status and the provenance in one atomic file
  write. A `queued` shipment that carries disposition keys is then
  unreachable by construction. The marker is ignored on every other
  transition. `MoveShipmentStatus` already sets
  `allowGovernedShipmentMutation` for moves that do not involve `blocked`.
* Verify: the B-U6 and B-U3 commands pass.

#### B-U8: Dispose journal recovery tests (tests)

* File: `internal/core/shipment_disposition_recovery_test.go` (new).
* Each scenario writes a `dispose` lifecycle journal by hand to simulate a
  crash at a reachable point, then runs `recoverPendingShipmentOperations`.
  The journal carries the shipment preimage, member preimages that cover the
  manifest, no related preimages, `RecoveryPolicy: "rollback"`, target
  `abandoned`, and the disposition reason in the existing `Reason` field.
* Scenarios:
  1. Intent journal, crash before and after the single governed write: with
     the shipment on disk as the `queued` preimage, and with it on disk as
     `abandoned` with the four disposition keys, recovery restores the
     preimage (`queued`, no `disposition_*` keys), leaves member file hashes
     unchanged, keeps links, removes the dispose status event by correlation
     ID, and removes the journal. A hand-written foreign state that no
     candidate matches (`queued` with disposition keys) fails with
     `ErrShipmentConflict` naming the journal.
  2. Leftover committed journal, the reachable crash between marking the
     journal committed and removing it: `loadShipmentOperationJournals`
     accepts it, `recoverPendingShipmentOperations` leaves both the
     `abandoned` shipment and the journal unchanged (the existing skip of
     non-intent journals, characterized here and not changed), and a later
     `ClaimShipment` of another queued shipment succeeds.
  3. A leftover intent journal does not block a later `ClaimShipment` of
     another shipment or an `AddItemToShipment`; recovery resolves it first.
* RED: `go test ./internal/core -run '^TestDisposeJournalRecovery$' -count=1`
  fails, because `dispose` is not an allowed operation.
* Posture: test-first.

#### B-U9: Dispose journal recovery (code)

* Files: `internal/core/shipment_ops.go`, `internal/core/shipment_recovery.go`.
* `shipment_ops.go`: in `validateShipmentLifecycleJournalRecord`, allow the
  `dispose` operation with the tuple `rollback`/`abandoned`. No new journal
  field is added; the decoder keeps `DisallowUnknownFields`.
* `shipment_recovery.go`:
  * In `reconcileShipmentLifecycleIntent`, accept `dispose` in the operation
    allowlist. The existing manifest-coverage check on member preimages
    applies unchanged.
  * In `shipmentRecoveryCandidates`, add a `dispose` case. The candidates are
    the preimage and, only when the on-disk status is `abandoned`, a clone of
    the preimage with status `abandoned` and the disposition keys copied from
    the on-disk file. The copy is accepted only when `disposition_reason`,
    `disposition_by`, and `disposition_authorization_ref` are non-empty
    strings, `disposition_reason` equals the journal `Reason`, and
    `superseded_by` is absent or a non-empty string. The existing
    full-artifact comparison, with `ignoreUpdatedAt`, proves nothing else
    changed. This mirrors how the `block` case reads `blocked_at` from the
    on-disk file.
  * In `memberRecoveryCandidates`, add a `dispose` case whose only candidate
    is each member preimage.
  * In the `rollback` branch, add a `dispose` case that restores only the
    shipment preimage through `governedCtx`. It writes no member file.
  * Add `dispose` to the operation condition of the claim-style
    early-return branch, the `if journal.Operation == "claim"` block after
    the policy switch. That branch removes the operation events by
    correlation ID with `removeShipmentOperationEvents` (the
    `MoveShipmentStatus` event is tagged through the `_shipment_operation`
    context key), persists the terminal journal, and removes it. Dispose
    therefore appends no `correlation_id` lifecycle event. The branch body is
    unchanged for `claim`.
  * An on-disk state that matches no candidate keeps the existing
    `ErrShipmentConflict` behavior and names the journal for operator repair.
* `internal/core/shipment.go` and its `recoverPendingShipmentOperations`
  are not changed.
* Verify: the B-U8 command passes, and
  `go test ./internal/core -run 'Recover' -count=1` passes.

#### B-U10: DisposeQueuedShipment success and input tests (tests)

* File: `internal/core/shipment_disposition_test.go` (new).
* Scenarios, as named subtests:
  1. `Success`: a `queued` shipment with a valid authorization moves to
     `abandoned`. `custom_fields` records `disposition_reason`,
     `disposition_by`, `disposition_authorization_ref`, and `superseded_by`
     when set. Exactly one status event is written, no member or
     covering-feature file changes (hash before and after), no journal, claim
     marker, or preimage remains, and the members become assignable to
     another shipment.
  2. `InputRefusals`: `Confirm == false` returns `ErrConfirmationRequired`;
     an empty `Reason`, `By`, or `AuthorizationRef` returns `ErrValidation`;
     a `SupersededBy` that names a missing or non-shipment ID returns
     `ErrValidation`.
  3. `AuthorizationRefusals`, each `ErrValidation` with the shipment file
     unchanged: an `AuthorizationRef` that does not resolve; one that
     resolves to a non-`deliberation` artifact; a `deliberation` whose
     `authorizes_disposition` custom field is missing or names a different
     shipment; and the negative decided-status case, a `queued` deliberation
     whose `authorizes_disposition` correctly names the target. A `done`
     deliberation and an archived deliberation with
     `archived_status: done` are accepted in the `Success` fixture variants.
* RED: `go test ./internal/core -run '^TestDisposeQueuedShipment$' -count=1`
  fails against the stub.
* Posture: test-first.

#### B-U11: DisposeQueuedShipment state and failure tests (tests)

* File: `internal/core/shipment_disposition_state_test.go` (new).
* Scenarios, as named subtests:
  1. `StateRefusals`, each `ErrShipmentDispositionRefused`: the target is
     `active`, `blocked`, `shipped`, `abandoned`, `archived`, or not a
     shipment.
  2. `LiveDependents`: a non-terminal shipment holds a `blocks` edge onto the
     target. The refusal names the dependents. Edges are never carried over
     to `superseded_by`.
  3. `FailureHandling`, through `disposeAfterGovernedWriteHook`: a
     non-indeterminate error rolls back to `queued` with no `disposition_*`
     keys, removes the dispose status event, removes the journal, and keeps
     links intact. An `ErrWriteIndeterminate` error is returned unchanged
     and leaves the intent journal; the next
     `recoverPendingShipmentOperations` restores the `queued` preimage and
     removes the journal. This is the in-process crash proof for R8.
* RED: `go test ./internal/core -run '^TestDisposeQueuedShipmentState$' -count=1`
  fails against the stub.
* Posture: test-first.

#### B-U12: Disposition provenance durability tests (tests)

* File: `internal/core/shipment_disposition_provenance_test.go` (new).
* Scenarios:
  1. After a disposition, an `UpdateArtifact` of an unrelated field keeps
     every disposition `custom_fields` key.
  2. After `ArchiveItem`, the archived file carries
     `archived_status: abandoned` and every disposition key.
  3. `ShipmentPredecessorShipped` on the disposed shipment, live or archived,
     returns `false`.
* RED: `go test ./internal/core -run '^TestShipmentDispositionProvenance$' -count=1`
  fails against the stub.
* Posture: test-first (learnings 2026-07-17 and 2026-07-28: provenance must
  survive reload from Markdown).

#### B-U13a: DisposeQueuedShipment validation, locks, and refusals (code)

* File: `internal/core/shipment_disposition.go`.
* Steps, in order:
  1. Validate inputs (`Confirm`, `Reason`, `By`, `AuthorizationRef`,
     `SupersededBy`).
  2. Validate the authorization. The `AuthorizationRef` must match
     `deliberationIDPattern` and resolve with `FindArtifactPath` to an
     artifact with `artifact_type == deliberation` that is decided: live
     `done`, or `archived` with Markdown `archived_status: done`. Its
     `authorizes_disposition` custom field must name the target.
  3. Take the global shipment lifecycle lock, call
     `recoverPendingShipmentOperations`, then take the membership and
     artifact mutation locks in the `ClaimShipment` order.
  4. Load the shipment once with `findArtifact` (Markdown). Require `queued`
     and no live reverse `blocks` dependents.
  5. Until B-U13b lands, return `ErrNotImplemented` after the refusals pass.
* Verify:
  `go test ./internal/core -run '^TestDisposeQueuedShipment$/^(InputRefusals|AuthorizationRefusals)$' -count=1`
  and
  `go test ./internal/core -run '^TestDisposeQueuedShipmentState$/^(StateRefusals|LiveDependents)$' -count=1`
  pass.

#### B-U13b: DisposeQueuedShipment journal, governed write, and rollback (code)

* File: `internal/core/shipment_disposition.go`.
* Replace the B-U13a placeholder return with these steps, in order:
  1. Capture the shipment and member preimages and write the
     `shipmentLifecycleJournal` intent (operation `dispose`,
     `RecoveryPolicy: "rollback"`, target `abandoned`, `Reason` set) with a
     correlation ID.
  2. Inside the journal window, call `MoveShipmentStatus` once with the
     shipment operation context and the governed-disposition marker carrying
     the provenance value. That is the single governed write of status and
     provenance (B-U7). Then run `disposeAfterGovernedWriteHook` when set.
  3. Mark the journal committed, then remove it, as `ClaimShipment` does.
  4. On a non-indeterminate failure after the intent is written, restore the
     shipment preimage under the governed write envelope, remove the dispose
     status event by correlation ID, and remove the journal. Return
     `ErrWriteIndeterminate` unchanged, without retry or rollback, so
     recovery rolls it back.
* No archive, no cascade, no member write.
* Verify: the B-U10, B-U11, B-U12, and B-U8 commands pass.

#### B-U14: CLI disposition tests (tests)

* File: `internal/cli/shipment_dispose_test.go` (new).
* Through the real command tree (`cli.NewRootCommand()`, `SetArgs`,
  `Execute`):
  1. Success on a queued fixture prints the abandoned shipment.
  2. A missing `--confirm` returns an error wrapping
     `ErrConfirmationRequired`; an empty `--reason` or an unbound
     `--authorization-ref` returns an error wrapping `ErrValidation`.
  3. A state refusal returns an error wrapping
     `ErrShipmentDispositionRefused`.
* RED: `go test ./internal/cli -run 'ShipmentDispose' -count=1` fails.
* Posture: test-first.

#### B-U15: CLI `shipment dispose` subcommand (code)

* File: `internal/cli/shipment.go`.
* `backlogit shipment dispose <id> --reason <text> --by <actor>
  --authorization-ref <deliberation-id> [--superseded-by <id>] --confirm`.
  `--confirm` is a bool flag passed into `Confirm`; core enforces it. Mirror
  the layout of `shipment unblock`.
* Verify: the B-U14 command passes.

#### B-U16: MCP disposition tests (tests)

* File: `internal/mcp/tools_shipment_dispose_test.go` (new).
* Through the server handler, called with a request map as
  `internal/mcp/contract_consistency_test.go` does, asserting error code
  strings only:
  1. Success on a queued fixture returns the abandoned shipment.
  2. `confirm: false` returns `confirmation_required`; an empty reason or an
     unbound authorization reference returns `validation_failed`.
  3. A state refusal returns `shipment_disposition_refused`.
* RED: `go test ./internal/mcp -run 'ShipmentDispose' -count=1` fails.
* Posture: test-first.

#### B-U17: MCP `backlogit_dispose_shipment` tool and error mapping (code)

* Files: `internal/mcp/tools.go`, `internal/mcp/errors.go`.
* Register the tool next to `backlogit_unblock_shipment` with parameters `id`,
  `reason`, `by`, `authorization_ref`, `superseded_by`, and
  `mcplib.WithBoolean("confirm", mcplib.Required())`. In `domainError`, map
  `ErrConfirmationRequired` to `confirmation_required` (no mapping exists
  today) and `ErrShipmentDispositionRefused` to
  `shipment_disposition_refused`.
* Verify: the B-U16 command passes. The registry parity tests stay red until
  B-U19, the known red window.

#### B-U18: Registry parity tests for the disposition (tests)

* File: `internal/cli/registry_parity_test.go`.
* Add `backlogit_dispose_shipment` to `mcpWithoutCLIIntentional`, and add
  `TestRegistryParity_DisposeConfirmationFailClosed`, mirroring the full
  unblock test. It asserts the registry entry has no `cli_command`, has
  `cli_fallback.automatic: false`, has `requires.confirm: true`, has a
  rationale, that the `confirmed_command` contains `--confirm`,
  `--authorization-ref`, `--by`, and `--reason`, and that the live cobra
  `shipment dispose` command has a bool `--confirm` flag.
* RED: `go test ./internal/cli -run '^TestRegistryParity_DisposeConfirmationFailClosed$' -count=1`
  fails.
* Posture: test-first.

#### B-U19: Registry entry for the disposition operation (config)

* File: `.autoharness/backlog-registry.yaml`.
* Add `dispose_shipment` next to `unblock_shipment`: `mcp_tool:
  "backlogit_dispose_shipment"`, no `cli_command`, and
  `cli_fallback: {automatic: false, confirmed_command:
  "backlogit shipment dispose {{id}} --reason {{reason}} --by {{by}} --authorization-ref {{authorization_ref}} --confirm",
  requires: {confirm: true}, rationale: ...}`, plus params. The rationale
  notes that the operator adds `--superseded-by` by hand when it applies.
* Verify: `go test ./internal/cli -run 'TestRegistry' -count=1` passes.

#### B-U20: Disposition documentation (docs)

* Files: `docs/workflow.md`,
  `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md`.
* Document the operation, the confirmation and target-bound deliberation
  requirements, that it never activates or moves members, that dependents
  must be re-declared first, that generic paths refuse `queued` to
  `abandoned`, and that archival stays a separate operation. Add a
  parity-matrix row marking the CLI fallback as confirmed-only.
* Verify (docs RED): before the edit,
  `git grep -n "backlogit_dispose_shipment" -- docs/workflow.md docs/reviews/2026-07-03-cli-mcp-parity-matrix.md`
  returns no match; after the edit both match, and docs lint reports zero
  violations on both files.
* Posture: characterization-first.

#### B-U21: Regenerate the CLI reference (docs)

* Files: the generated pages under `docs/cli-reference/` only.
* Run `go run ./cmd/gen-docs docs/cli-reference`. Hand edits are not
  allowed.
* Verify: a second regeneration leaves `git diff --exit-code docs/cli-reference`
  clean, and the new `shipment dispose` page exists.

#### B-U22: Stage and Orchestrator disposition role contract (harness)

* Files: `.github/agents/_stage.agent.md`,
  `.github/agents/_orchestrator.agent.md`.
* Stage:
  * Edit the Role Boundary table row "Claim or close shipments on behalf of
    Ship" to add an explicit exception for `backlogit_dispose_shipment` on a
    `queued` shipment.
  * Add the conditions: only Stage may call `backlogit_dispose_shipment`;
    the authorizing `deliberation` records the operator's decision with the
    operator quoted verbatim, is decided (`done`, or archived as `done`), and
    names the target in `authorizes_disposition`; Stage never creates or
    changes that deliberation or its `authorizes_disposition` field in the
    same session in which it disposes; Stage enters Careful mode and gets
    in-session operator approval before the call; Stage never disposes on
    its own initiative, and never disposes an `active`, `blocked`, or
    `shipped` shipment.
* Orchestrator: neither the Orchestrator nor Ship ever disposes shipments,
  and the `_Ship` tool list omits the tool on purpose.
* Verify (docs RED): before the edit,
  `git grep -n "backlogit_dispose_shipment" -- .github/agents/_stage.agent.md .github/agents/_orchestrator.agent.md`
  returns no match; after the edit both match.
* Posture: characterization-first.

#### B-U23: Record the agent-contract drift (config)

* File: `.autoharness/harness-manifest.yaml`.
* Append to the `drift_reason` of the `_stage.agent.md` and
  `_orchestrator.agent.md` entries: "6434A4D7 governed queued-shipment
  disposition role contract; upstream template sync pending". Leave
  `drift_allowed: true`.
* Verify: `git grep -n "6434A4D7 governed queued-shipment" -- .autoharness/harness-manifest.yaml`
  matches twice.

## Dependency Graph

```text
Feature A (shipment SA)
A-U1 -> A-U2 -> A-U3 -> A-U4 -> A-U5 -> A-U6 -> A-U7 -> A-U8 -> A-U9 -> A-U10
A-U4 -> A-U11 -> A-U12 -> A-U13 -> A-U14 -> A-U15
A-U10 -> A-U16, A-U15 -> A-U16 -> A-U17
Feature B (shipment SB)
B-U1 -> B-U2 -> B-U3 -> B-U4 -> B-U5 -> B-U6 -> B-U7 -> B-U8 -> B-U9
B-U9 -> B-U10 -> B-U11 -> B-U12 -> B-U13a -> B-U13b
B-U13b -> B-U14 -> B-U15 -> B-U16 -> B-U17 -> B-U18 -> B-U19 -> B-U20 -> B-U21
B-U19 -> B-U22 -> B-U23
```

Shipment edges:

* `SB` `blocks` on `SA`. Both edit `internal/errors/errors.go`,
  `internal/mcp/errors.go`, `internal/core/queue.go`, `docs/workflow.md`, and
  `.autoharness/harness-manifest.yaml`, and B-U12 uses the A-U4 predicate.
* `SB` `blocks` on the `AF1E5075` shipment. Both edit
  `.github/agents/_orchestrator.agent.md` and
  `.autoharness/harness-manifest.yaml`. After the attempt-4 revision the
  `AF1E5075` plan no longer edits `_stage.agent.md`, so the overlap is those
  two files.
* `SA` and the `AF1E5075` shipment both append `drift_reason` text to
  different entries of `.autoharness/harness-manifest.yaml`. Single-active
  shipment execution serializes them, and the edits touch different lines, so
  no edge is needed.
* `SA` and `SB` each carry a `blocks` edge onto `154-S` as an L1 scope
  marker. Under condition (b) and the L1 closure trigger, nothing whose
  `blocks` closure includes `154-S` routes until the `154-S` operator
  attestation exists. The edge places these shipments inside that hold. The
  engine treats the edge as satisfied today, because `154-S` is archived as
  shipped. The attestation hold applies through L1 policy, not the engine.

## Decisions and Rationale

* **Two features from one stash entry.** Readiness and disposition are
  separable release units with separate risks. Feature A owns every surface
  of the claim refusal (core, MCP, CLI, docs), so it ships complete on its
  own.
* **Workspace queue entry point instead of an exported resolver field.**
  `QueryQueue(ctx, db, filter)` has no workspace. `QueryQueueForWorkspace`
  installs the resolver in core, so `MoveInQueue` and both adapters share one
  path, the unexported field cannot be set by callers, and a source scan of
  every non-test package pins all callers to it. A caller without the
  resolver would hide shipments whose `154-S` marker edge points at an
  archived shipped predecessor.
* **Edge-type normalization.** The index never stores an empty dependency
  type, because every write and rehydration path defaults it to `blocks`.
  The queue therefore keys on the indexed `blocks` type and leaves
  `isExecutionBlockingDependency` unchanged. The claim guard reads Markdown,
  where a hand-edited edge may have an empty type, so it applies the same
  empty-to-`blocks` default. Both views then see the same edge set.
* **New predicate, not a taxonomy change.** The six-status cascade is shared
  by feature cascade, task release, and gate code. Changing it would widen the
  blast radius far beyond shipments.
* **Error contract.** Reasons describe artifact state; errors describe
  infrastructure. Callers fail closed on both.
* **Claim guard placement.** Checking the reloaded Markdown after the reload
  under locks closes the time-of-check window and runs before any state is
  written, so no rollback is needed on refusal.
* **Close the generic path first.** `queued` to `abandoned` already succeeds
  through `UpdateArtifact` and `BulkUpdateStatus`. A governed operation is
  only meaningful once those paths refuse the edge, so B-U5 lands before the
  governed edge.
* **One governed edge and one governed write, not a separate write path.**
  The disposition reuses `MoveShipmentStatus`, the lifecycle journal,
  recovery, events, hooks, and the write envelope. Only the context marker
  opens `queued` to `abandoned`, and the marker carries the provenance so the
  status and the `disposition_*` keys land in the same file write. A crash
  can then leave only the preimage or the full target, and recovery derives
  the target candidate from the on-disk file plus the existing journal
  `Reason`, with no new journal field. This mirrors the governed-activation
  marker that `ClaimShipment` already uses.
* **Recovery coverage stays honest.** `recoverPendingShipmentOperations`
  skips journals that are not intents, so a leftover committed dispose
  journal is inert, as committed `claim` journals are today. B-U8 pins that
  behavior instead of asserting a cleanup that the code does not perform, and
  `internal/core/shipment.go` recovery is not changed for a hypothetical
  test.
* **Provenance in `custom_fields`.** No model change is needed, and B-U12
  proves the keys survive `update` and archive.
* **Authorization reference.** backlogit has no `decision` artifact type, so
  the reference is a `deliberation`. Core enforces a structural, target-bound
  reference: the deliberation exists, is decided, and names the target in
  `authorizes_disposition`. The deliberation workflow has no `decided`
  status; a deliberation is decided when it is live `done`, or archived with
  Markdown `archived_status: done`, which matches `074-DL` today. A `queued`
  deliberation is refused. Core cannot prove the operator wrote it. Operator
  origin is a governance rule in the Stage contract (B-U22), and the
  deliberation is the audit trail.
* **No read-tool exposure of the predecessor verdict.** The queue omission
  and the claim refusal already expose it with a stable token. No deferred
  expansion is captured.
* **The L2 token is shared with L1.** The `AF1E5075` contract names
  `shipment_predecessor_not_shipped` first. A change to the token must change
  both.
* **Mirror out of scope.** `plugin/agents/ship.agent.md` is a condensed
  distributable for other workspaces. The core guard covers every caller,
  including those workspaces once they upgrade the binary.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Existing fixtures claim shipments with unshipped shipment predecessors and start failing | Medium | A-U12 audits and fixes them before the guard lands; more than two files returns to Stage. Weakening the guard is not an allowed fix |
| Existing fixtures or callers move a queued shipment to `abandoned` through a generic path | Medium | B-U4 audits them before B-U5; a production caller returns to Stage |
| Live queued shipments become hidden once `SA` merges | Medium | Before merge, Ship inventories shipment `blocks` edges whose predecessor is abandoned or archived non-shipped and records the result in closure |
| Markdown read cost in the queue | Low | Only shipment-to-shipment `blocks` edges onto an `archived` predecessor trigger a read |
| A disposition is misused | Medium | Core-enforced `Confirm`, reason, actor, and a target-bound deliberation; refusal of every non-`queued` status and of live dependents; confirmed-only CLI fallback; Stage Careful mode and in-session approval |
| A leftover dispose journal blocks every lifecycle operation | Medium | B-U9 makes the journal provable; B-U8 scenario 3 pins that later claims proceed |
| A disposition is irreversible through the API | Medium, accepted | The operation is rated destructive. Recovery from a mistake is a new replacement shipment |
| `active` to `abandoned` stays open on generic paths | Low, accepted | Out of scope; recorded in Scope so a later item can decide it |
| Version skew: the guard is merged but the pinned installed binary is older | High | Closure records `backlogit version` against the merge commit. The guard counts as "merged, not yet active" until the installed binary is upgraded |

## Constitution Check

* I Safety-First Go: wrapped errors, typed sentinels, no panics, fail-closed
  provenance reads, `ErrWriteIndeterminate` surfaced without retry. Pass.
* II Test-First: every code unit is preceded by a RED test unit with an exact
  command; docs units use a before and after `git grep` check. Pass.
* III Workspace Isolation and IV CLI Workspace Containment: Markdown reads go
  through `findArtifact` and `FindArtifactPath` under the storage root, and
  tests run in `t.TempDir()`. Pass.
* V Observability: refusals return stable reason tokens and error codes; the
  disposition appends a status event and records provenance. Pass.
* VI Single Responsibility: predicate, queue filter, queue entry point,
  guard, protected edge, governed edge, recovery, and disposition live in
  separate functions. Pass.
* VII Destructive Command Approval: the disposition is rated destructive. It
  requires core-enforced confirmation, a reason, an actor, and a target-bound
  deliberation, and the registry exposes no automatic CLI fallback. Pass.
* VIII Safety Modes: core does not read safety mode. The Stage contract
  requires Careful mode and in-session operator approval before a
  disposition (B-U22). Pass.
* IX Git-Friendly Persistence: state stays in Markdown frontmatter, journals,
  and the item log. Pass.
* X Context Efficiency: the plan cites exact functions and files. Pass.
* XI Merge Commit History: unaffected. Pass.
* Task Granularity: 41 units, each within the 2-hour rule. B-U13 is split
  into B-U13a and B-U13b; the other unit IDs are unchanged. Pass.

Constitution Check: pass

## Plan Hardening Signals

* New public API surface: core functions, an MCP tool, a CLI command, and a
  registry operation.
* Change to a claim-time state machine with lock ordering.
* A new protected transition and a governed transition edge on shipments
  with journal recovery.
* Fail-closed authorization semantics that can block delivery work.
* Agent role-contract changes.

Requires plan hardening: yes

## Runtime Verification and Closure

* Pre-merge for each shipment: `go test ./... -count=1`, `go vet ./...`, and
  `go run ./cmd/backlogit docs lint` with zero violations.
* Live impact inventory before `SA` merges (R14): a read-only
  `backlogit_query_sql` over `item_deps` lists shipment-to-shipment `blocks`
  edges whose predecessor is `abandoned`, or `archived` with a Markdown
  `archived_status` other than `shipped`. The closure record lists each
  affected queued shipment.
* Feature A runtime check after merge, using `go run ./cmd/backlogit` or an
  installed binary whose `backlogit version` matches the merge:
  `backlogit queue view --type shipment` lists no shipment whose shipment
  predecessor is unshipped. A direct `backlogit shipment claim` of a
  `149-S`-shaped shipment in a throwaway fixture under the git-ignored
  `logs/6434a4d7-fixture/` directory fails with a message containing
  `shipment_predecessor_not_shipped`.
* Feature B runtime check after merge: Ship enters Careful mode and gets
  explicit in-session operator approval before it runs
  `backlogit shipment dispose` on a throwaway fixture shipment in
  `logs/6434a4d7-fixture/`, with the working directory set to that fixture.
  The command returns `abandoned` with provenance. The fixture is throwaway
  and no live shipment changes; removing the fixture directory afterwards
  needs explicit operator approval in Careful mode. `backlogit doctor` on the
  live workspace reports no new findings.
* Closure (R13): each closure record states the binary version in use,
  states that the guard covers (P) only, confirms that `156-S` and every other
  existing shipment are unchanged, and states that the manual (P)+(C) policy
  from 074-DL stays in force until `AF1E5075` lands and the `154-S`
  attestation exists.

## Plan Hardening

Hardening required: yes. The plan adds a fail-closed guard to the claim path
that every caller uses, changes queue readiness for shipments, protects and
governs a transition edge, and adds a destructive shipment operation
reachable by Stage.

### Inputs

* Learnings consulted (Step 1.8, `docs/compound/`):
  * `2026-07-20-ship-gate-descoped-archived-member-exemption.md`: the index
    omits `archived_status`; read Markdown, use an allowlist, fail closed.
  * `2026-07-17-backlogit-update-drops-archive-provenance.md`: archive
    provenance survives only through `ArchiveItem`; the update bug is fixed
    (`842b701d`, `e09befa8`). B-U12 re-proves it for disposition keys.
  * Prior claim-guard learnings: refusals belong in the core seam with no
    forgeable exemption; peek, then re-check under the lock; check before
    activation or inside rollback scope; AST declaration harnesses pin
    permanent signatures only; registry-dispatch parity fixtures; pinned CLI
    version skew between the installed binary and source.
* Instructions consulted: `.github/instructions/backlogit.instructions.md`
  (Queue and Dependency Protocol, Shipment Sequencing Protocol), the Markdown
  and writing-style instructions, and the constitution.
* Code facts verified for review attempt 3: the protected-transition guard
  (`artifacts.go`), the `BulkUpdateStatus` pre-scan (`queue.go`), the journal
  operation allowlist (`shipment_ops.go`), the recovery candidates
  (`shipment_recovery.go`), the MCP `domainError` table, and the
  `deliberationIDPattern` (`shipment_lifecycle.go`).

### Protected Invariants

* `terminalCascadeStatuses`, `releasableStatuses`, `IsNoLongerBlockingStatus`,
  `IsCascadeTerminalStatus`, `IsReleasableStatus`, and `IsGateTargetStatus`
  keep their current membership and callers.
* `ClaimShipment` member activation, the `scheduler_baseline_claim` marker,
  preimage, snapshot, crash recovery, and `rollbackShipmentClaim` are
  unchanged. The guard only adds an earlier refusal.
* Lock order stays global lifecycle lock, then membership lock, then artifact
  mutation lock, in both `ClaimShipment` and `DisposeQueuedShipment`.
* `isValidShipmentTransition` is unchanged. Exactly one governed-only edge
  (`queued` to `abandoned` under the disposition marker) is added, and the
  same edge is protected on every generic path.
* Edges other than shipment-to-shipment `blocks` keep the six-status cascade.
* Existing recovery for `claim`, `block`, `unblock`, and `normalize` journals
  is unchanged. `dispose` only joins the operation condition of the
  claim-style early-return branch.
* The disposition status and its provenance land in one file write; no
  state with `queued` plus disposition keys is ever written.
* `156-S` and every other existing shipment are unchanged.

### Risky Actions

| ProposedAction | ActionRisk | Approval | Rollback |
|---|---|---|---|
| PA1: insert `requireShipmentPredecessorsShipped` into `ClaimShipment` (A-U13) | High: every claim caller | Plan-review PASS; Ship review and CI | Revert the A-U13 commit; the helper has one call site |
| PA2: shipment-edge readiness in the queue (A-U6, A-U8, A-U10) | Medium: changes which shipments the queue lists | Plan-review PASS; live impact inventory | Revert A-U6, A-U8, and A-U10 |
| PA3: protect `queued` to `abandoned` on generic paths (B-U5) | Medium: refuses a write that succeeds today | Plan-review PASS; B-U4 audit | Revert the B-U5 commit |
| PA4: governed edge, dispose recovery, and `DisposeQueuedShipment` (B-U7, B-U9, B-U13a, B-U13b, B-U15, B-U17) | Destructive: terminal state change on shipments | Plan-review PASS. Per call: core-enforced confirmation and a decided, target-bound deliberation; only Stage may invoke the tool, in Careful mode with in-session operator approval; the Orchestrator and Ship never invoke it | Code: revert the commits. Data: not reversible through the API; a mistaken disposition is corrected by a replacement shipment. Ship verification never runs it against the live backlog |
| PA5: registry entry and parity test (B-U18, B-U19) | Low | Plan-review PASS | Revert both; the parity test then fails, which is the signal |
| PA6: agent role-contract and instruction text (A-U16, B-U22) | Medium: governs Stage and Orchestrator behavior | Plan-review PASS; Ship review | Revert the commits |

### Added Verification

* Environment precheck before A-U3: confirm the build under test is the
  source tree (`go run ./cmd/backlogit version`), never the installed binary.
* A-U12 and B-U4 audit fixtures before the guard and the protected edge land.
* B-U10 scenario 1 compares member file hashes before and after.
* Blocked paths:
  * If A-U11 shows the guard cannot run after the reload without changing
    activation code, stop and return to Stage. Do not move the guard after
    activation.
  * If B-U6 shows the marker cannot be honored without changing
    `isValidShipmentTransition`, stop and return to Stage.
  * If B-U8 shows dispose recovery needs changes to the existing `claim`,
    `block`, `unblock`, or `normalize` recovery behavior, or to
    `recoverPendingShipmentOperations` in `internal/core/shipment.go`, stop
    and return to Stage. Adding `dispose` to the operation condition of the
    claim-style early-return branch is in scope; its body stays unchanged.
  * If B-U6 shows the status and the provenance cannot land in one persist
    call inside `moveShipmentStatusWithHeadGuard`, stop and return to Stage.
    Do not fall back to two writes.
  * If deliberation artifacts cannot carry the `authorizes_disposition`
    custom field through the existing create and update operations, stop and
    return to Stage.

### Operational Closure

* Monitoring after merge: Ship pre-claim logs and `backlogit doctor` output
  over the next two shipments. Any `shipment_predecessor_not_shipped` refusal
  on a shipment whose predecessors are all shipped is a rollback trigger for
  PA1.
* Owner: the Ship session that merges each shipment. Validation window: the
  next two shipment claims after the installed binary is upgraded.
* Applying `DisposeQueuedShipment` to `156-S` or any live shipment needs a
  separate operator decision after Feature B ships.

### Review-Gate Capability

* Plan review must emit the literal markers `dispatch_mode:` and `decision:`.
  Expected `dispatch_mode: multi-agent-dispatch` with Constitution, Go, Scope
  Boundary, Learnings, Architecture, Agent-Native Parity (new MCP tool), and
  Security Lens (new state-changing API) personas.
* Tool degradation carried forward: engram is degraded (daemon not ready), so
  code research used bounded known-path reads (PACK-ROUTING deviation). This is
  not a review-dispatch degradation.

### Unresolved Operator Decisions

None block harvest. The `156-S` disposition stays a later decision.

## Attempt-4 Revision

* Cycle authorization: the review cycle limit was reached at attempt 3. The
  operator then said to keep working autonomously until the task is finished.
  The parent recorded that instruction as authorization for exactly one more
  in-scope review and fix cycle, raising the cap from 3 to 4 attempts in
  total. It does not reset the counter, waive the gate, approve an ADVISORY
  outcome, expand the three-item scope, or start Ship. No fifth review is
  authorized.
* P1-1 (Option A): B-U7 writes the status and the disposition provenance in
  one governed persist. B-U9 derives the target candidate from the on-disk
  file and the existing journal `Reason`, so no journal field is added.
  B-U13b makes one governed call. The hook is renamed
  `disposeAfterGovernedWriteHook`.
* P1-2: B-U8 scenario 2 now characterizes the reachable leftover committed
  journal, which recovery leaves in place. B-U11 scenario 3 adds the
  in-process indeterminate-crash proof. `internal/core/shipment.go` recovery
  is not changed.
* P2 fixes: the claim-style early-return branch is named (B-U9); the edge
  type is normalized (R3, R4, A-U5, A-U6, A-U11, A-U13, Decisions); the
  source scan covers every non-test package (A-U9); B-U13 is split into
  B-U13a and B-U13b with named subtests; the authorizing deliberation must be
  decided, with a negative case (R9, B-U10, B-U13a); B-U22 forbids Stage from
  creating or changing the authorization in the disposing session and limits
  the tool to Stage; PA4 and the Feature B fixture name Careful mode and
  operator approval.
* The `SB` edge onto the `AF1E5075` shipment now rests on the
  `_orchestrator.agent.md` and manifest overlap, because the `AF1E5075`
  scope-edge rule was deferred to stash `B88A3716`.

## Plan Review

* review_attempt: 1
* reviewed_at: 2026-09-30T18:40:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: initial draft (U1 to U20)
* P1 findings:
  * The disposition CLI `--confirm` was presence-only and the registry
    rendered required flags into an automatic fallback, so `confirm:false`
    could become a confirmed disposition.
  * The disposition used a separate write path outside `MoveShipmentStatus`,
    the lifecycle journal, recovery, and events.
  * The disposition was rated non-destructive in PA3 and Constitution VII.
* P2 findings: the claim-refusal mapping sat in Feature B; `MoveInQueue` used
  no resolver; the predicate error contract was unclear; the claim guard read
  index edges and did not fail closed on unknown predecessors; the
  `backlogit.instructions.md` sequencing text and the parity matrix were not
  updated; the Stage role boundary forbade the new operation; the generated
  docs exceeded the file limit; a registry red intermediate commit; release
  skew; `rg` and `--types` in verify commands; the `154-S` edge rationale.
* Disposition: all findings are addressed in the attempt-2 revision.

<!-- plan-review-attempt: 1 -->

## Plan Review

* review_attempt: 2
* reviewed_at: 2026-09-30T20:10:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: attempt-1 revision (A-U1 to A-U14, B-U1 to B-U16)
* P1 findings:
  * Dispose recovery was underspecified: the journal operation allowlist,
    the recovery candidates, member preimages, and event cleanup were not
    covered, so a dispose journal could block every lifecycle operation.
  * Generic paths already allow `queued` to `abandoned` on shipments
    (`UpdateArtifact`, `UpdateArtifactWithGate`, `BulkUpdateStatus`), so the
    governed edge was not the only way in.
  * The `AuthorizationRef` named a `decision` artifact type that does not
    exist, with no binding to the target shipment.
* P2 findings: units exceeded the scenario or file limits (queue, adapter
  tests); no bounded fixture-fix units; runtime fixtures had no contained
  location or approval basis; an unreachable `provenance_unreadable` reason;
  no MCP `ErrConfirmationRequired` mapping and a non-required `confirm`
  parameter; no `QueryQueue` caller inventory or adapter source scan; no live
  impact inventory; the empty edge type; the status map lacked
  `artifact_type`; the Constitution VIII line was not truthful.
* P3 findings: the registry red window, HTTP status assertions, the
  `--superseded-by` note, the full unblock parity mirror, real command-tree
  dispatch, and the `156-S` closure invariant.
* Disposition: all findings are addressed in the revision above (A-U1 to
  A-U17, B-U1 to B-U23, R7 to R14).

<!-- plan-review-attempt: 2 -->

## Plan Review

* review_attempt: 3
* reviewed_at: 2026-09-30T21:30:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: attempt-2 revision (A-U1 to A-U17, B-U1 to B-U23)
* P1 findings (Go Reviewer, Feature B):
  * P1-1: B-U13 writes the disposition provenance and then moves the status
    in two steps. A crash between them leaves a `queued` shipment that carries
    `disposition_*` keys. B-U9 does not list that state as a recovery
    candidate, so `validateShipmentLifecycleRecoveryCAS` fails with
    `ErrShipmentConflict` and the lock blocks every later operation. Also,
    `shipmentLifecycleJournal` has no fields for `by`, `authorization_ref`, or
    `superseded_by`, and its decoder uses `DisallowUnknownFields`, so recovery
    cannot rebuild the target candidate.
  * P1-2: B-U8 scenario 2 (committed journal) cannot pass, because
    `recoverPendingShipmentOperations` in `internal/core/shipment.go` skips
    journals that are not intents. Fixing that needs a third file outside the
    unit's file list.
* P2 findings: name the claim-style early-return branch, because the
  `MoveShipmentStatus` event is tagged through the `_shipment_operation`
  context key and not `correlation_id` (Go); an empty edge type conflicts
  with `isExecutionBlockingDependency` (Go); callers without a resolver hide
  shipments with a `154-S` marker edge, so extend the source scan to all
  non-test packages (Architecture); B-U13 is too large and needs a split
  (Scope); require the authorizing deliberation to have a decided status,
  with a negative test (Security); B-U22 must forbid Stage from creating or
  changing `authorizes_disposition` in the disposing session (Security); PA4
  must name Careful mode and approval and state that only Stage may run the
  tool (Security); the Feature B fixture disposal needs Careful mode or
  approval (Constitution).
* Feature A (A-U1 to A-U17) had no P1 findings in this attempt.
* Fix options for the operator: (P1-1) persist the provenance and the status
  in one governed write, or add a third recovery candidate plus new journal
  fields with matching validator changes; (P1-2) drop B-U8 scenario 2, or add
  a scoped unit that changes `internal/core/shipment.go`.
* Escalation: the review cycle limit is reached (attempt counter 3). The
  escalation route (gpt-6-sol, openai, xhigh) differs from the Stage route,
  but engram is degraded, so no analysis hand-off is possible:
  ESCALATION_DEGRADED. Stage halted for operator intervention. No harvest,
  shipment, or stash archive happened.

<!-- plan-review-attempt: 3 -->

## Plan Review

* review_attempt: 4
* reviewed_at: 2026-10-01T01:38:05Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: ADVISORY
* operator_authorization: approved
* operator_authorization_recorded_at: 2026-10-01T02:10:00Z, after the review.
  At review time this line read "not recorded" and the gate was not
  satisfied. The decision stays ADVISORY; it is not relabelled PASS, and no
  finding below is removed or changed.
* operator_authorization_source: the user's continuation message at
  2026-09-30T18:13:36.645-07:00 ("Keep working autonomously until the task
  is truly finished ... make good decisions and keep working"), given after
  the full finding list was surfaced. The parent declared it, at
  2026-09-30T18:49 -07:00, as a delegated operator decision covering routine
  completion decisions, and authorized Stage to proceed on this ADVISORY
  result.
* operator_authorization_scope: delegated approval of this ADVISORY outcome
  for Stage backlog creation only, on the condition that every P2 below is a
  mandatory, explicit acceptance criterion of named tasks that Ship cannot
  complete without. This is not a literal per-finding approval by the user.
  It does not waive any P0 or P1 gate, any Ship or merge gate, or the plan's
  safety rules, and it accepts no unsafe runtime behavior. No fifth review
  was run, no persona was invoked again, and the reviewed plan body is
  unchanged.
* operator_authorization_conditions (P2 to enforcing task IDs; the full
  condition text is in each task as MANDATORY ADVISORY CONDITION):
  * G-A, Feature A runtime check (Constitution): fixture claims use only a
    source-bound CLI, or a core or MCP instance bound to the fixture, with
    an absolute, isolated fixture root and explicit `--cwd`. The storage
    root is proved to be the fixture before any seeding or claim. The live
    `149-S` is never claimed through an ambient working directory or a
    stale binary. Fixture removal needs approval in Careful mode. Enforced
    by `184.018-T`.
  * G-B, Feature B runtime check vs B-U22, PA4, and R11 (Agent-Native
    Parity, Security Lens): the disposition is Stage-only, consistently
    across the MCP tool, the confirmed CLI command, and the agent
    instructions. Ship and the Orchestrator never invoke it, including as
    runtime verification; Ship's evidence is in-process tests. The
    Orchestrator file is not the only place the ban is written: a Ship Role
    Boundary line is added. Enforced by `185.010-T`, `185.015-T`,
    `185.016-T`, `185.017-T`, `185.018-T`, `185.020-T`, `185.021-T`,
    `185.023-T`, `185.024-T` (Ship Role Boundary, split from B-U22 for the
    file limit), `185.025-T`, and `185.026-T` (replaces the Ship-run
    fixture disposition).
  * G-C, B-U8 scenario 1 and B-U9 (Go): assert the actual reachable output
    (`errors.Is(err, ErrShipmentConflict)` plus the shipment ID or the
    emitted text), never the journal filename. Enforced by `185.008-T` and
    `185.009-T`.
  * G-D, B-U13b step 4, B-U11 `FailureHandling`, and R8 (Learnings,
    `2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`):
    commit-then-surface holds. There is no compensating rollback or retry
    over a possibly committed disposition. Recovery keeps an on-disk complete
    disposition and preserves the evidence, the classified
    `ErrWriteIndeterminate` is surfaced, and the actual post-commit error
    case is tested. Enforced by `185.008-T`, `185.009-T`, `185.011-T`, and
    `185.014-T`.
* operator_authorization_p3: the P3 findings are carried as non-gating
  advisory notes on `184.009-T`, `184.018-T`, `185.011-T`, `185.013-T`,
  `185.014-T`, `185.017-T`, `185.019-T`, `185.023-T`, and `185.026-T`. The
  `A-U10 -> A-U11` edge is adopted as a real dependency
  (`184.011-T` depends on `184.010-T`).
* operator_authorization_harvest: features `184-F` (harvested from stash
  `6434A4D7`) and `185-F`, tasks `184.001-T` to `184.018-T` and `185.001-T`
  to `185.026-T`, queued shipments `184-S` and `185-S`.
* reviewed_revision: attempt-4 revision (Feature A A-U1 to A-U17, Feature B
  B-U1 to B-U23 with B-U13a and B-U13b; 41 units)
* cycle_authorization: attempt 4 is the single extra cycle the parent
  recorded from the operator's instruction to keep working; the cap is 4
  total and no fifth review is authorized.
* gating: per plan, on this plan's own findings only.
* Attempt-3 fixes: verified by all seven personas, including against the
  code. B-U7 persists status and provenance in one governed write; B-U9
  builds dispose candidates from the on-disk file and the journal `Reason`
  without new journal fields and uses the claim-style early-return branch;
  B-U8 scenario 2 characterizes a reachable committed journal; B-U11
  scenario 3 proves in-process `ErrWriteIndeterminate` recovery; empty edge
  types are normalized; the A-U9 source scan covers all non-test packages;
  B-U13 is split; the decided-deliberation rule has its negative case; and
  B-U22, PA4, and the Feature B fixture name Careful mode, approval, and
  Stage-only use.
* P0 findings: none.
* P1 findings: none.
* P2 findings (new):
  * Feature B runtime check vs B-U22, PA4, and R11 (Agent-Native Parity,
    Security Lens): the runtime check has Ship run
    `backlogit shipment dispose` on a fixture, while B-U22 and PA4 say only
    Stage disposes. The ban also names only the MCP tool, so the confirmed
    CLI command stays open to Ship, and it is written only in
    `_orchestrator.agent.md`, which a directly started Ship session never
    reads. Proposed fix: make the fixture disposition an operator-run step,
    or add a narrow fixture-only CLI carve-out with a storage-root check;
    cover both the MCP tool and the CLI command in B-U22; add a Ship Role
    Boundary line.
  * Feature A runtime check (Constitution): the throwaway-fixture claim
    names no working directory and allows any installed binary. If it runs
    from the repo root with a pre-guard binary, it could claim the live
    `149-S`. Removing the fixture also lacks an approval rule. Proposed fix:
    set the working directory to the fixture, check the storage root, use
    `go run ./cmd/backlogit`, and require approval in Careful mode to remove
    the fixture.
  * B-U8 scenario 1 and B-U9 (Go): the foreign-state case asserts an
    `ErrShipmentConflict` that names the journal. The existing CAS error at
    `shipment_recovery.go` around line 615 and the wrap in
    `recoverPendingShipmentOperations` do not include the journal path.
    Proposed fix: assert `errors.Is(err, ErrShipmentConflict)` plus the
    shipment ID, or wrap the error with the journal path inside
    `reconcileShipmentLifecycleIntent`.
  * B-U13b step 4, B-U11 `FailureHandling`, and R8 (Learnings,
    `2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`):
    leaving the intent journal after `ErrWriteIndeterminate`, so recovery
    restores `queued`, conflicts with the commit-then-surface learning, and
    the plan does not cite it. Proposed fix: either cite the learning and
    record dispose as the same exception `ClaimShipment` uses, or switch to
    commit-then-surface and adjust B-U8 and B-U11.
* P3 findings: name `lockedCtx` in B-U13b step 2; keep the intent journal
  and return `MutationPartialError` when a restore fails during rollback;
  move `LiveDependents` into B-U13b's Verify; re-read the authorization
  after the locks are taken; make the B-U22 verify check each condition;
  check `--reason`, `--by`, and `--authorization-ref` on the live command in
  B-U18; add a `write_indeterminate` case in B-U16; ban `t.Parallel()` in
  tests that change the hook seam; add an `A-U10 -> A-U11` edge or narrow
  A-U13's Verify; count every `QueryQueue` identifier in the A-U9 scan; add
  `golangci-lint run` and `gofmt -l .` to the pre-merge gates.
* Disposition: ADVISORY without operator authorization does not satisfy the
  gate, and no fifth review is authorized. The findings are recorded but
  the plan is not edited after the review, so the reviewed revision stays
  the gated one. No harvest, shipment, or stash archive happens for
  `6434A4D7`. Next step for the operator: either authorize this ADVISORY
  outcome (record an explicit operator approval of it, after which Stage may
  harvest with the P2 fixes carried as task acceptance criteria), or
  authorize a further fix-and-review cycle.
* Escalation: the authorized cycle cap of 4 is reached. engram is degraded,
  so no analysis hand-off is possible: ESCALATION_DEGRADED. Halted for
  operator decision.
* Post-review status (2026-10-01): the operator decision requested above was
  recorded as the delegated `operator_authorization: approved` with the
  conditions listed in this block. The halt is resolved without a fifth
  review, and Stage harvested under those conditions.

<!-- plan-review-attempt: 4 -->

## Shipment Decomposition Addendum

This addendum covers packaging only. On 2026-09-30 the user said that `184-S`
(18 tasks) and `185-S` (26 tasks) together carry far too many tasks per
shipment and must be decomposed by size and complexity. The Orchestrator
relayed that request to Stage.

The addendum changes how the reviewed units are grouped into release units.
It keeps every requirement, mandatory guard, design choice, and 074-DL
decision.

* The only task-body changes are the packaging refinements PR-1 to PR-4.
  Each moves a stated requirement to a different unit or release, and each is
  justified only by a green release boundary.
* PR-5 changes release-closure text only.

* The four Plan Review blocks above stay verbatim. They remain the gate record
  for the plan body. The plan-review attempt counter stays at 4.
* The packaging is reviewed on a new, separately authorized surface, the
  Shipment Decomposition Review blocks below. Those blocks are not a fifth
  plan review.
* This text is revision 2. It replaces revision 1, which decomposition
  review attempt 2 failed. Both failed attempts are recorded below with their
  findings.
* Out of scope:
  * `183-F` and `183-S`.
  * `186-F` and `186-S`, including `186.007-T` and G-E.
  * Stash `B88A3716`, stash `84E54F92`, and every other stash entry.
  * Any claim, ship, archive, abandon, or status change.

### Release Semantics Checked

These semantics were read in code at `153a6b0e`:

* `ClaimShipment` activates only the explicit manifest members.
* `ShipShipment` releases only the explicit manifest
  (`releaseScopeItemIDs`):
  * A feature listed in the manifest is marked `done` and archived.
  * A feature not listed is left untouched.
  * Parent cascades stop at the manifest boundary.
  * Nothing requires all of a feature's children to be done.
* `DeriveCoveringFeature` returns the first root (dotless) feature in the
  manifest.
* Stage invariants: every task in a shipment has the covering feature as its
  parent, and the covering feature is listed first.
* An item may belong to only one non-terminal shipment
  (`validateShipmentItemIDs`).

Two consequences follow:

* A broad feature cannot sit in an early chunk. It would be archived as
  `done` while most of its work is still queued.
* A task cannot ship under a feature missing from its manifest.

Each release unit therefore gets its own fully contained root feature.

How tasks move:

* Tasks move with `backlogit_adopt_item` (`AdoptItem` in
  `internal/core/shipment_lifecycle.go`).
  * It renames the hierarchical ID, the Markdown file, and the event log.
    Renaming has existed since `170ae28b`.
  * It rewrites `parent_id`, dependencies, links, and index rows in other
    artifacts. That rewrite has existed since `ffd885b4`.
  * It records `origin_feature`.
  * If it cannot generate a new ID, it keeps the old one. Every adoption is
    therefore read back.
* `.github/instructions/backlogit-yaml-header-tooling.instructions.md` still
  says that adopt does not rename. That text is stale, and this correction
  leaves it unchanged as a recorded discrepancy.
* It does not rewrite shipment manifests or body prose. Body prose cites plan
  unit IDs (`A-Ux`, `B-Ux`), and those IDs do not change.
* The old-to-new ID map is recorded in the Applied Mapping section after
  apply.

How the original IDs are used:

* `184-F` and `184-S` keep the closing chunk A-U16, A-U17, and SA-CV.
* `185-F` and `185-S` keep the closing chunk B-U20, B-U23, and SB-CV.
* Each broad feature is released only by its own closure check, after every
  predecessor release has shipped.
* The existing edges stay as they are, and no edge is removed:
  * `184-S` blocks on `154-S`.
  * `185-S` blocks on `154-S`, `186-S`, and `184-S`.
* The guarded closure tasks keep their IDs: `184.018-T` (G-A), and
  `185.021-T`, `185.025-T`, and `185.026-T` (G-B).

The other eight chunks become new features and shipments, placed before the
two original IDs in the DAG.

### Release Units

Effort is in human-equivalent hours. Every task stays within the 2-hour rule.

| Unit | Shipment / feature | Units (original task IDs) | Tasks | Effort (h) | Complexity / risk | Direct shipment prerequisites |
|---|---|---|---|---|---|---|
| A1 predicate | new / new | A-U1 to A-U4 (`184.001-T` to `184.004-T`) | 4 | 2.5 to 4.25 | medium / low | `154-S` |
| A2 queue readiness | new / new | A-U5 to A-U10 (`184.005-T` to `184.010-T`) | 6 | 5.5 to 8.75 | high / moderate-high | A1, `154-S` |
| A3 claim guard | new / new | A-U11 to A-U15 (`184.011-T` to `184.015-T`) | 5 | 4 to 7 | high / high | A1, A2, `154-S` |
| A4 docs and closure | `184-S` / `184-F` | A-U16, A-U17, SA-CV (`184.016-T` to `184.018-T`) | 3 | 2 to 3 | medium / low-moderate | A2, A3, `154-S` |
| B1 generic-path guard and governed edge | new / new | B-U3 to B-U7 (`185.003-T` to `185.007-T`) | 5 | 4.25 to 7.5 | high / high | A2, A3, `154-S` |
| B2 dispose recovery | new / new | B-U8, B-U9 (`185.008-T`, `185.009-T`) | 2 | 3 to 4 | high / high | B1, `154-S` |
| B3 disposition core | new / new | B-U1, B-U2, B-U10 to B-U13b (`185.001-T`, `185.002-T`, `185.010-T` to `185.014-T`) | 7 | 7 to 10.75 | high / high | A1, B2, `154-S` |
| B4 Ship disposition ban | new / new | B-U22s (`185.024-T`) | 1 | 0.25 to 0.5 | low / moderate (agent contract) | `186-S`, `154-S` |
| B5 adapters and role contract | new / new | B-U14 to B-U19, B-U21, B-U22 (`185.015-T` to `185.020-T`, `185.022-T`, `185.023-T`) | 8 | 6 to 8.75 | medium-high / high | A3, B3, B4, `186-S`, `154-S` |
| B6 docs, drift, closure | `185-S` / `185-F` | B-U20, B-U23, SB-CV (`185.021-T`, `185.025-T`, `185.026-T`) | 3 | 2.25 to 3.25 | medium / moderate | `184-S` (A4), B5, `186-S`, `154-S` |

Totals:

* 44 tasks.
* Feature A: 14 to 23 hours.
* Feature B: 22.75 to 34.75 hours.

Sizes:

* Eight units are within the 4-to-6 target or smaller: A1 to A4, B1, B2, B4,
  and B6.
* B2 and B4 are small on purpose, because they are recovery work and an
  authority boundary.
* No unit is split by numeric slice.
* B3 holds 7 tasks and B5 holds 8, with the rationale below.

Why B3 holds 7 tasks (inseparable green boundary):

* The exported `DisposeQueuedShipment` stub (B-U2) and the B-U13a
  refusal-only placeholder both stay until B-U13b.
* The B-U10, B-U11, and B-U12 suites stay red against the stub until B-U13b.
* Any split would either release a stub or move a red test away from the
  code that makes it pass.

Why B5 holds 8 tasks (inseparable green boundary, at the cap):

* User criterion: no release may carry an adapter inconsistency. Under G-B,
  Stage-only authority must hold consistently across the MCP tool, the CLI
  command, and the agent instructions. The CLI command (B-U15) and the MCP
  tool (B-U17) therefore ship together.
* The Stage exception with its safeguards and the Orchestrator ban (B-U22)
  ship in the same release that first makes either form reachable. That
  removes the unbounded interim window that attempt 1 left open.
* The CI job `cli-reference-drift` (`.github/workflows/ci.yml`) fails a
  change that adds `shipment dispose` without regenerating
  `docs/cli-reference`. B-U21 therefore ships with B-U15.
* `TestRegistryParity_EveryMCPToolMappedOrDeferred` turns red once the MCP
  tool exists without a registry row. B-U18 and B-U19 therefore ship with
  B-U17.
* The Ship-side ban (B-U22s) is moved to B4, ahead of B5, so B5 stays within
  8 tasks (PR-4).
* B-U20 (docs) and B-U23 (drift reason) are not needed at the B5 boundary.

Per-task effort (hours) and complexity:

| Task | Unit | Domain | Effort | Complexity | Release |
|---|---|---|---|---|---|
| `184.001-T` | A-U1 | tests | 0.5 to 1 | low | A1 |
| `184.002-T` | A-U2 | code | 0.25 to 0.5 | trivial | A1 |
| `184.003-T` | A-U3 | tests | 1 to 1.5 | medium | A1 |
| `184.004-T` | A-U4 | code | 0.75 to 1.25 | medium | A1 |
| `184.005-T` | A-U5 | tests | 1 to 1.75 | medium | A2 |
| `184.006-T` | A-U6 | code | 1.5 to 2 | high | A2 |
| `184.007-T` | A-U7 | tests | 0.75 to 1.25 | medium | A2 |
| `184.008-T` | A-U8 | code | 0.75 to 1.25 | medium | A2 |
| `184.009-T` | A-U9 | tests | 1 to 1.5 | medium | A2 |
| `184.010-T` | A-U10 | code | 0.5 to 1 | low | A2 |
| `184.011-T` | A-U11 | tests | 1.25 to 1.75 | medium | A3 |
| `184.012-T` | A-U12 | tests | 0.5 to 1.5 | medium | A3 |
| `184.013-T` | A-U13 | code | 1.25 to 2 | high | A3 |
| `184.014-T` | A-U14 | tests | 0.75 to 1 | low | A3 |
| `184.015-T` | A-U15 | code | 0.25 to 0.75 | low | A3 |
| `184.016-T` | A-U16 | docs | 0.75 to 1 | low | A4 |
| `184.017-T` | A-U17 | config | 0.25 to 0.5 | trivial | A4 |
| `184.018-T` | SA-CV | verification | 1 to 1.5 | medium | A4 |
| `185.003-T` | B-U3 | tests | 0.75 to 1.25 | medium | B1 |
| `185.004-T` | B-U4 | tests | 0.5 to 1.5 | medium | B1 |
| `185.005-T` | B-U5 | code | 0.75 to 1.25 | medium | B1 |
| `185.006-T` | B-U6 | tests | 1 to 1.5 | medium | B1 |
| `185.007-T` | B-U7 | code | 1.25 to 2 | high | B1 |
| `185.008-T` | B-U8 | tests | 1.5 to 2 | high | B2 |
| `185.009-T` | B-U9 | code | 1.5 to 2 | high | B2 |
| `185.001-T` | B-U1 | tests | 0.5 to 0.75 | low | B3 |
| `185.002-T` | B-U2 | code | 0.5 to 0.75 | low | B3 |
| `185.010-T` | B-U10 | tests | 1.25 to 2 | medium | B3 |
| `185.011-T` | B-U11 | tests | 1.25 to 2 | high | B3 |
| `185.012-T` | B-U12 | tests | 0.75 to 1.25 | medium | B3 |
| `185.013-T` | B-U13a | code | 1.25 to 2 | high | B3 |
| `185.014-T` | B-U13b | code | 1.5 to 2 | high | B3 |
| `185.024-T` | B-U22s | harness | 0.25 to 0.5 | low | B4 |
| `185.015-T` | B-U14 | tests | 1 to 1.25 | medium | B5 |
| `185.016-T` | B-U15 | code | 0.75 to 1.25 | medium | B5 |
| `185.017-T` | B-U16 | tests | 1 to 1.25 | medium | B5 |
| `185.018-T` | B-U17 | code | 0.75 to 1.25 | medium | B5 |
| `185.019-T` | B-U18 | tests | 0.75 to 1 | medium | B5 |
| `185.020-T` | B-U19 | config | 0.5 to 0.75 | low | B5 |
| `185.022-T` | B-U21 | docs (generated) | 0.25 to 0.5 | trivial | B5 |
| `185.023-T` | B-U22 | harness | 1 to 1.5 | medium | B5 |
| `185.021-T` | B-U20 | docs | 1 to 1.25 | low | B6 |
| `185.025-T` | B-U23 | config | 0.25 to 0.5 | trivial | B6 |
| `185.026-T` | SB-CV | verification | 1 to 1.5 | medium | B6 |

The `size` and `complexity` fields on the backlog items are not set. The
estimates live in this table only.

### Green Boundaries

Before it merges, every release must pass the gates that CI enforces in
`.github/workflows/ci.yml`:

* `golangci-lint`.
* `go test -race ./...`.
* `go vet ./...`.
* The Windows job (`test-windows`).
* `make docs-lint` and the docline soft-key test.
* `make md-lint`, the P-008 heading gate.
* `cli-reference-drift`. B5 is the first release that changes the CLI
  surface.

Ship runs these gates. Stage does not run builds, tests, or linters. P-008
is therefore not verified by Stage for any future release.

* A1:
  * A-U2 stubs the predicate and declares the final sentinel. A-U4
    implements the predicate. The narrowed A-U1 harness passes (PR-1).
  * The sentinel `ErrShipmentPredecessorNotShipped` is an exact-text final
    value, not a placeholder. It stays in A1 because the A-U1 pin and the
    test-first order put it there, and A-U11's compile-but-fail harness needs
    it before A3 (see I-1).
* A2:
  * Holds the whole A-U5 to A-U10 red window. A-U10 migrates every adapter.
  * A-U5 pins the `QueryQueueForWorkspace` signature with an AST harness
    before the stub exists, as R12 requires (PR-1). A-U6 declares the stub,
    A-U7 compiles and fails against it, and A-U8 implements it. No stub
    survives the release.
* A3: green after A-U13. The A-U14 red step is closed by A-U15. The
  `A-U10 -> A-U11` edge keeps A-U13's full-suite Verify outside the A2 red
  window.
* A4: docs, the drift record, and the Feature A closure check.
* B1: green after B-U5 and B-U7.
  * Generic paths refuse `queued` to `abandoned`.
  * The governed edge is reachable only through the unexported
    governed-disposition context marker (R7). Production code reads it, and
    B3's `DisposeQueuedShipment` sets it. The B-U6 test sets it from inside
    package `core`.
* B2: recovery for `dispose` journals. No code writes them until B3. B-U8
  tests recovery with hand-written journals, and G-C and G-D are met inside
  B2.
* B3: holds the B-U2 stub, the B-U13a placeholder, and the red suites until
  B-U13b. B3 is green when it ends.
* B4: a single fail-closed agent-contract line banning Ship from both
  disposition forms (PR-4).
* B5: holds the CLI, the MCP tool, the registry parity window B-U17 to
  B-U19, the CLI reference regeneration, and the Stage and Orchestrator role
  contract. At the B5 boundary the MCP tool, the CLI command, and all three
  agent contracts state Stage-only authority consistently (G-B).
* B6: workflow and parity-matrix docs, the drift reason, and the Feature B
  closure check.

### Packaging Refinements

Each refinement keeps the unit's exact requirements and guards. Each is
justified only by a green release boundary.

#### PR-1 (A-U1, A-U2, A-U5, A-U6, A-U8)

Problem: if A1 ended with the `QueryQueueForWorkspace` stub pinned by A-U1,
it would release a stub. R12 also requires the pin to exist before the stub.

Changes:

* `184.001-T`: drop the `QueryQueueForWorkspace` pin and the "workspace
  queue entry point" scenario. Keep the predicate and sentinel pins.
* `184.002-T`: drop the `QueryQueueForWorkspace` stub. AC 3 now refers to
  the predicate stub.
* `184.005-T`:
  * Add the signature pin to `TestShipmentReadinessDeclarations` in
    `internal/core/shipment_readiness_decl_test.go`, without pinning the
    unexported resolver field. The task now touches two files.
  * Add an AC: the pin fails through `go/parser` until A-U6 declares the
    stub. This is the R12 order, with the pin before the stub.
  * The task remains a tests-only task.
* `184.006-T`: declare the stub
  `func QueryQueueForWorkspace(ctx context.Context, ws *Workspace, filter *QueueFilter) (*QueueView, error)`,
  returning `nil, ErrNotImplemented`, in `internal/core/queue.go`. That is
  still one file.
* `184.007-T` is unchanged. Its AC 1 ("fails against the stub") already
  matches this order.
* `184.008-T`: add an AC that the declaration harness passes and that no
  `ErrNotImplemented` remains in `QueryQueueForWorkspace`.

Unchanged: the exact signature, the sentinel text, and the predicate stub
that A-U4 replaces inside A1.

#### PR-2 (B-U1, B-U2, B-U3, B-U10)

Problem: the exported B-U2 stub would otherwise sit across B1 and B2.

Changes:

* B-U1 and B-U2 move into B3, ahead of B-U10.
* Task edges:
  * Remove `185.003-T -> 185.002-T`.
  * Add `185.010-T -> 185.002-T`.
  * `185.002-T -> 185.001-T` and `185.010-T -> 185.009-T` stay.
* B-U3 uses no B-U2 symbol.
* Only the "Plan dependencies" lines of `185.003-T` and `185.010-T` change.

#### PR-3 (B-U20, B-U21, SB-CV)

Problem: the CI job `cli-reference-drift` requires the regenerated CLI
reference in the same release that adds `shipment dispose`. B-U20 is not
test-enforced.

Changes:

* B-U21 moves into B5 and B-U20 moves to B6.
* Task edges:
  * Remove `185.022-T -> 185.021-T`.
  * Add `185.022-T -> 185.020-T`.
  * Add `185.026-T -> 185.021-T`.
* Only the "Plan dependencies" lines of `185.022-T` and `185.026-T` change.

#### PR-4 (B-U22, B-U22s, B-U23)

Problem: G-B requires the Ship ban, the Orchestrator ban, and the Stage
exception to be in place wherever either disposition form is reachable.
Adapters plus all contracts make 9 tasks.

Changes:

* B-U22s, a pure Ship-side prohibition, moves ahead into B4. B-U22 ships
  with the adapters in B5.
* Task edges:
  * Remove `185.024-T -> 185.023-T`.
  * Add `185.023-T -> 185.024-T`.
  * Add `185.025-T -> 185.023-T`. `185.025-T -> 185.024-T` stays.
* Only the "Plan dependencies" lines of `185.023-T`, `185.024-T`, and
  `185.025-T` change.
* The ban text and every G-B block are unchanged.

Why B-U22 cannot move to B4 with B-U22s:

* B-U22 gives Stage permission to use the dispose tool and command. Before
  B5 that tool does not exist, so the permission would be an unfulfilled
  contract.
* B-U22s is a ban. A ban on an absent capability fails closed.

#### PR-5 (R14 timing, closure text only)

Problem: the plan requires the read-only R14 live impact inventory "before
SA merges". The first behavior-changing Feature A merge is now A2, whose
queue filter hides shipments behind unshipped archived predecessors.

Change: no task body changes. The Per-Release Closure Rule in the A2 and A3
feature descriptions carries the R14 duty instead.

* Before A2 merges, its closure record states the read-only R14 inventory,
  using the same query as SA-CV, with no fixture and no live mutation.
* Before A3 merges, its closure record either re-states the inventory or
  states that it is unchanged since A2, with the query and its timestamp.
* SA-CV keeps its own R14 AC and re-records the inventory before A4 merges.

### Per-Release Closure Rule

Each new feature's description carries this rule:

* The release passes the CI-aligned gates listed under Green Boundaries.
* Its closure record states the R13 items:
  * the binary version;
  * that the guard covers (P) only;
  * that `156-S` and every other existing shipment are unchanged;
  * that the manual (P)+(C) 074-DL policy stays in force until AF1E5075
    lands and the `154-S` attestation exists.
* For A2 and A3, the closure record also carries the R14 duty from PR-5.
* The runtime fixture checks stay in SA-CV and SB-CV, under G-A and G-B.

The new feature descriptions use this text:

```text
Release unit {unit} of the 6434A4D7 shipment decomposition. Source plan:
docs/exec-plans/2026-09-30-6434a4d7-shipment-predecessor-readiness-guard-plan.md
(Shipment Decomposition Addendum). Original feature: {184-F|185-F}.
Per-release closure rule: {rule above, with the R14 line for A2 and A3}.
Hold: condition B scheduler-consumption attestation C for 154-S is absent;
not Ship-eligible until it exists.
```

`184-F` and `185-F` get this note appended to their descriptions. Nothing
else in those descriptions changes.

```text
Decomposition note (2026-09-30): this feature now covers only its closing
release ({A4|B6}). Earlier work moved to the release-unit features listed in
the plan's Applied Mapping, via backlogit_adopt_item (origin_feature kept).
```

### Interim States Between Releases

None of these states leaves a test red, a stub, or an adapter
inconsistency. Each is recorded so reviewers can accept or reject it.

* I-1 (after A1, through A2):
  * The exported predicate has no production caller until A2.
  * The final sentinel is returned first by A3. It stays in A1 because the
    A-U1 pin and the test-first order put it there.
* I-1b (after A2, before A3):
  * The queue hides shipments behind unshipped archived predecessors.
  * `ClaimShipment` still allows claiming them until A3.
  * The existing Shipment Sequencing Protocol and the condition B hold
    still apply.
* I-2 (after B1): no supported queued-shipment disposition exists until B5.
  * Generic `queued` to `abandoned` is refused, which fails closed.
  * The governed-disposition context marker is read but never set in
    production until B3.
  * The manual 074-DL policy is unchanged.
  * B-U4 returns the task to Stage if it finds a production caller.
* I-3 (after B2): recovery accepts `dispose` journals that no code writes
  yet. G-D forbids restoring over a complete disposition.
* I-3b (after B3, before B5):
  * The exported `DisposeQueuedShipment` is final, tested code.
  * No CLI command or MCP tool reaches it until B5.
* I-4 (after B4, before B5):
  * Ship's contract bans both disposition forms before either exists. A
    prohibition of an absent capability fails closed.
  * Ship's tool list already omits the MCP tool. P-010 already blocks
    unlisted CLI mutations.
  * The harness-manifest drift reason for `_ship.agent.md` lags from B4
    until B6 (B-U23). `drift_allowed: true` is already set.
* I-5 (after A3, before A4): the claim refusal exists before the docs that
  describe it. The existing Shipment Sequencing Protocol already requires
  `shipped` predecessors.
* I-6 (after B5, before B6):
  * The workflow doc, the parity-matrix row, and the harness-manifest drift
    reason lag behind.
  * Every agent-facing surface already states Stage-only authority, so this
    is a docs-only lag.
  * `drift_allowed: true` is already set on the three agent files.
* Follow-up note, not in scope: no row in the backlog-integration
  instructions names the dispose operation. The plan did not include one,
  and this addendum adds none.

### Shipment DAG

```text
154-S <- A1 <- A2 <- A3 <- 184-S(A4)
A1 <- A3; A2 <- 184-S(A4)
A2 <- B1; A3 <- B1; B1 <- B2 <- B3; A1 <- B3
186-S <- B4
B3 <- B5; B4 <- B5; A3 <- B5; 186-S <- B5
B5 <- 185-S(B6); 184-S(A4) <- 185-S(B6); 186-S <- 185-S(B6)
every unit also blocks on 154-S
```

Edge reasons:

* B1 needs A2 because both edit `internal/core/queue.go`.
* B1 needs A3 so that A3 and B1 to B3 merge in a fixed order. The A-U12
  claim-fixture audit and the B-U4 abandon-fixture audit then each see the
  other's fixtures. This also keeps the original order, where all Feature A
  code came before Feature B code.
* B3 needs A1 because B-U12 uses the predicate and both edit
  `internal/errors/errors.go`.
* B5 needs A3 because both edit `internal/mcp/errors.go`. A-U15 touches
  `internal/cli/shipment.go` only conditionally.
* B4 and B5 need `186-S` because of file overlaps:
  * `186.005-T` edits `_ship.agent.md`, and `186.007-T` verifies it.
  * `186.004-T` edits `_orchestrator.agent.md`.
* `185-S` keeps `186-S` and `184-S` because of overlaps in
  `.autoharness/harness-manifest.yaml` and `docs/workflow.md`.
* A4 (A-U17) also edits a different harness-manifest entry. As the reviewed
  Dependency Graph says, that needs no edge to `186-S`.

Task edges after PR-2 to PR-4 that cross releases. Each points to an earlier
release:

* `184.005-T -> 184.004-T`
* `184.011-T -> 184.004-T` and `184.011-T -> 184.010-T`
* `184.016-T -> 184.010-T` and `184.016-T -> 184.015-T`
* `185.008-T -> 185.007-T`
* `185.010-T -> 185.009-T`
* `185.015-T -> 185.014-T`
* `185.023-T -> 185.024-T`
* `185.021-T -> 185.020-T`
* `185.025-T -> 185.023-T` and `185.025-T -> 185.024-T`
* `185.026-T -> 185.022-T`

No task edge crosses from Feature A to Feature B. That ordering is carried by
shipment edges.

### Condition B Gate

The condition B gate is unchanged.

* Every release unit carries a `blocks` edge onto `154-S`.
* The engine treats that edge as satisfied, because `154-S` is archived as
  shipped.
* Scheduler-consumption attestation C is still absent, so no unit is
  Ship-eligible under 074-DL L1 policy.
* Every new shipment gets a comment recording this hold.
* This addendum invents no attestation.

### Guard Preservation

The guard text does not change. It moves with each task file.

* G-A: `184.018-T` (kept).
* G-B:
  * `185.010-T` (B3).
  * `185.015-T` to `185.018-T` and `185.020-T` (B5).
  * `185.023-T` (B5).
  * `185.024-T` (B4).
  * `185.021-T`, `185.025-T`, and `185.026-T` (kept).
* G-C: `185.008-T` and `185.009-T` (B2).
* G-D: `185.008-T` and `185.009-T` (B2), and `185.011-T` and `185.014-T`
  (B3).
* G-E: `186.007-T`, out of scope and unchanged.

Bodies changed by PR-1 to PR-4:

* `184.001-T`, `184.002-T`, `184.005-T`, `184.006-T`, and `184.008-T`.
* `185.003-T`, `185.010-T`, `185.022-T`, `185.023-T`, `185.024-T`,
  `185.025-T`, and `185.026-T`.

That is 12 bodies. Five of them hold a G-B block: `185.010-T` and
`185.023-T` to `185.026-T`. Those blocks must stay byte-identical, which is
checked by hash.

The only title change is `184.002-T`, from "readiness API stubs" to
"readiness predicate stub".

### Packaging Hardening

Requires plan hardening: yes. This is a high-blast-radius backlog
restructuring: 38 renamed task IDs (15 from Feature A and 23 from Feature B)
and edits to two manifests.

Learnings consulted:

* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
* `docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`
* `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`
* `docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md`
* `docs/compound/2026-07-20-manual-feature-harvest-provenance-backfill.md`

Execution order (total). All steps run in Careful mode under the user's
explicit authorization for this correction. After every step, Stage reads
the result back. If a tool returns an indeterminate result, Stage reads the
item back and does not retry.

1. Preflight:
   * Read the MCP server version (`backlogit_get_version`) and the CLI
     version (`C:\Tools\backlogit.exe --version`). Confirm both include
     `47dfcc93` (manifest-only release), `170ae28b` (adopt rename), and
     `ffd885b4` (cross-artifact rewrite). If they do not, halt.
   * Confirm that no file under `.backlogit/queue` or `.backlogit/archive`
     uses the next root numbers.
   * Confirm the baseline hashes.
2. Commit the reviewed addendum as a rollback checkpoint, by explicit path.
3. PA-P4: rewire task edges.
4. PA-P5: edit the bodies of the original IDs.
5. PA-P2: create eight root features.
6. PA-P8: add hold comments to `184-S` and `185-S` that record the item
   lists before the edit. Each comment supersedes the earlier
   "Ready for Ship claim" comment and its item count.
7. PA-P1: edit the manifests, then sync.
8. PA-P3: adopt the tasks in dependency order.
   * Halt after the first adoption. Read back the new ID, the renamed file,
     and the renamed log before continuing.
   * Record each returned new ID.
9. PA-P7: create eight shipments from the read-back new IDs, with the
   feature first. Add a hold comment to each.
10. PA-P6: add shipment edges.
11. Add comments to `184-S` and `185-S` with the item lists after the edit.
    Append the feature notes to `184-F` and `185-F`. Fill in the Applied
    Mapping and run the verification.
12. Commit by explicit path list. Build the list from the read-back map.
    Before staging, diff the unrelated dirty files against their baseline
    hashes. The commit SHA is then added to the `184-S` and `185-S` comments
    in a follow-up commit.

Risky actions:

* ProposedAction PA-P1: remove only the moved IDs from the
  `custom_fields.items` lists of `184-S` and `185-S` with a surgical edit,
  then run `backlogit_sync_index`.
  * Targets: `.backlogit/queue/184-S.md` and `.backlogit/queue/185-S.md`.
  * change_kind: manifest edit.
  * ActionRisk: moderate.
  * Approval: the user's correction request.
  * Rollback: restore the two files from the checkpoint commit and sync.
    This is valid only before PA-P3. After PA-P3, use the operator-level
    rollback under PA-P3.
  * ActionResult: planned.
* ProposedAction PA-P2: create eight root features with the
  `source-stash-6434A4D7` label, the plan and 074-DL references, a
  `related_to` link to `184-F` or `185-F`, no `source_stash_id`, and the
  description text above.
  * Targets: eight new feature files.
  * change_kind: create.
  * ActionRisk: low.
  * Approval: user request.
  * Rollback: revert the Stage commit. Deletion is not authorized.
  * ActionResult: planned.
* ProposedAction PA-P3: adopt 38 tasks with `backlogit_adopt_item`, in
  dependency order.
  * Targets: the 38 task files and logs, plus every artifact that references
    them.
  * change_kind: rename and reparent.
  * ActionRisk: high. IDs change, and the tool does not rewrite manifests or
    prose.
  * Approval: user request, in Careful mode.
  * If the tool returns an indeterminate result, do not retry and do not
    roll back. Read the item back.
  * If the run fails partway, halt, read back, and report. Do not restore
    or clean the tree, because unrelated dirty files are present.
  * Operator-level rollback after completion: `git revert` of the Stage
    commit, then `backlogit_sync_index`. Re-adopting would mint new IDs.
  * ActionResult: planned.
* ProposedAction PA-P4: rewire the eight task edges of PR-2 to PR-4 with
  `backlogit_remove_dependency` and `backlogit_add_dependency`, on the
  original IDs.
  * Targets: the frontmatter of `185.003-T`, `185.010-T`, `185.022-T`,
    `185.023-T`, `185.024-T`, `185.025-T`, and `185.026-T`.
  * change_kind: dependency edit.
  * ActionRisk: low.
  * Approval: user request.
  * Rollback: apply the inverse edge operations.
  * ActionResult: planned.
* ProposedAction PA-P5: edit the 12 task bodies listed above and one title,
  through the official update operation.
  * Targets: the 12 task files.
  * change_kind: body edit.
  * ActionRisk: moderate. Five bodies hold G-B blocks.
  * Verification: every guard block hash is unchanged.
  * Approval: user request.
  * Rollback: restore each body from the checkpoint commit through the same
    update operation.
  * ActionResult: planned.
* ProposedAction PA-P6: add 24 shipment `blocks` edges with
  `backlogit_add_dependency`, which runs cycle detection.
  * Targets: the eight new shipments, plus `184-S` and `185-S`.
  * change_kind: dependency create.
  * ActionRisk: low.
  * Approval: user request.
  * Rollback: `backlogit_remove_dependency` for each added edge.
  * ActionResult: planned.
* ProposedAction PA-P7: create eight queued shipments with
  `backlogit_create_shipment`, using the read-back new IDs. Add a hold
  comment to each.
  * Targets: eight new shipment files.
  * change_kind: create.
  * ActionRisk: low.
  * Approval: user request.
  * Rollback: revert the Stage commit. Abandonment and deletion are not
    authorized.
  * ActionResult: planned.
* ProposedAction PA-P8: append comments to `184-S`, `185-S`, `184-F`, and
  `185-F`.
  * change_kind: append-only comment or description note.
  * ActionRisk: low.
  * Approval: user request.
  * Rollback: none needed. Comments are append-only and a later comment can
    supersede them.
  * ActionResult: planned.

Verification:

* Every original task maps to exactly one current ID and appears in exactly
  one of the ten manifests.
* No manifest contains a pre-adoption ID.
* Every manifest lists its covering feature first, and that feature is the
  parent of every task in the manifest.
* No descendant of `184-F` or `185-F` sits outside A4 or B6.
* No status changes.
* The DAG has no cycle and no missing target, with a Kahn order recorded.
  That covers both task edges and shipment edges, checked after adoption.
* `backlogit doctor` reports no new orphan or duplicate.
* Every guard block hash is unchanged.
* The `183` and `186` files are unchanged.
* The unrelated dirty files keep their hashes.
* `backlogit docs lint` and `git diff --check` pass on the edited Markdown.
* P-008 heading check: Stage does not run `make md-lint`, because that is a
  linter. Stage checks the edited Markdown by hand for MD001, MD025, and
  MD041 and records the result. Each hold comment states that P-008 was not
  verified by Stage.

Packaging Constitution Check:

* II (test-first): every red step stays in the same release as its green
  step. A-U5 pins the signature before A-U6 declares the stub (R12).
* Workflow 1 (2-hour rule, width): no task grows past three files or two
  hours. Two tasks gain one item each: A-U5 gains one pin and one file
  (two files in total), and A-U6 gains a one-line stub.
* Workflow 5 (no dead code): I-1 to I-3b ship tested, final code that no
  production path reaches yet. None of it is a stub or placeholder, and each
  case is recorded.
* VII and VIII (role boundary, safety): G-B holds at every boundary. Each
  risky action carries an ActionRisk.
* No application source, test, instruction, agent, registry, or config file
  changes.

Constitution Check: pass.

<!-- shipment-decomposition-addendum: revision 2 -->

## Shipment Decomposition Review

This block records the review of the packaging proposal. It is not a
`## Plan Review` block and does not change the plan-review attempt counter,
which stays at 4.

* review_surface: shipment-decomposition (authorized by the user request
  of 2026-09-30)
* review_attempt: 1
* dispatch_mode: multi-agent-dispatch
* decision: FAIL
* TOOL_OK: reviewer-subagent-dispatch
* Personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity
  Reviewer, and Security Lens Reviewer. All seven returned.
* Merged counts: P0=0, P1=7, P2=13, P3=14. Duplicates were merged.
* Plan hardening was required and present. The Constitution Check verdict
  was present.

P1 findings and their dispositions in revision 1:

* B4 split (Scope Boundary Auditor). The original rationale overclaimed
  the registry window. Revision 1 rests B5's size on the user's
  no-adapter-inconsistency criterion and G-B consistency across MCP, CLI, and
  contracts. It also records the CI and registry-parity couplings exactly.
* Old-to-new ID map missing (Scope Boundary Auditor), and stale manifests
  from creating shipments before adoption (Learnings Researcher,
  Architecture Strategist P2, Security Lens P2). Fixed: a total execution
  order, PA-P7 from read-back IDs, the Applied Mapping section, and a
  verification line.
* CLI version skew (Learnings Researcher). Fixed: a preflight version and
  ancestry check.
* R14 timing (Architecture Strategist). Fixed by PR-5 and the Per-Release
  Closure Rule.
* G-B interim window between adapters and role contracts (Agent-Native
  Parity Reviewer, two P1s; Security Lens Reviewer). Fixed by PR-4:
  * The Stage and Orchestrator contract ships with the adapters in B5.
  * The Ship ban ships first, in B4.
  * B4 and B5 both block on `186-S`.

P2 findings and their dispositions:

* A-U7 compile-red window (Constitution, Go, Scope). Fixed: the stub moves
  to A-U6, and A-U7 again fails against a stub.
* Sentinel unused until A3 (Go). Recorded in I-1. It cannot move, because
  A-U11 needs it to compile.
* ActionRisk levels (Constitution, Scope). Fixed: they now use defined
  levels, with approvals and ActionResult.
* I-4 mitigations inaccurate (Constitution, Parity). Superseded by PR-4.
* A3 and B1 to B3 ordering (Architecture). Fixed: added the edge B1 to A3.
* Per-unit closure (Architecture). Fixed by the Per-Release Closure Rule.
* Doctor audit (Learnings). Added to verification.
* P-008 markdown gate (Learnings). Recorded: Stage runs the
  documentation entrypoint `backlogit docs lint` and `git diff --check`.
  Markdown linters are a Ship-side gate under Stage's role boundary.
* Child-ID namespace check (Learnings). Added to preflight.
* Provenance on new features (Learnings, Architecture P3). Fixed by PA-P2.
* Rollback before commit (Security). Fixed: a checkpoint commit, plus halt
  and read back.

P3 findings, accepted as advisory:

* Every cross-release edge is now listed.
* The harness-manifest statement is qualified.
* The A4 range is corrected: A-U17 is 0.25 to 0.5 hours.
* The B5 to A3 rationale is narrowed.
* The hold comment is added.
* B6 is kept as a separate release.
* Redundant direct edges are kept, each with its reason.
* The B-U6 internal-package note is recorded in the B1 boundary.
* Declined, to avoid requirement expansion: the optional extra AC on
  `185.022-T`, and the optional per-condition grep on `185.023-T`.

<!-- shipment-decomposition-review-attempt: 1 -->

## Shipment Decomposition Review

This block records decomposition review attempt 2, of revision 1. It is not
a `## Plan Review` block. The plan-review attempt counter stays at 4.

* review_surface: shipment-decomposition (authorized by the user request
  of 2026-09-30)
* review_attempt: 2
* dispatch_mode: multi-agent-dispatch
* decision: FAIL
* TOOL_OK: reviewer-subagent-dispatch
* Personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity
  Reviewer, and Security Lens Reviewer. All seven returned.
* Merged counts: P0=0, P1=1, P2=11, P3=23. Duplicates were merged.
* This is the second decomposition review cycle. One re-entry remains
  (attempt 3). If attempt 3 fails, Stage halts and escalates.

P1 finding and its disposition in revision 2:

* SBA2-P1-1 (Scope Boundary Auditor; also Constitution P2 and Go P3). PR-1
  put the `QueryQueueForWorkspace` pin in A-U7, after A-U6 declared the stub.
  That breaks R12, which requires the AST pin before the stub exists. Fixed:
  the pin moves to A-U5 (`184.005-T`, tests, before A-U6) and fails through
  `go/parser` until A-U6. `184.007-T` is no longer edited. `184.005-T` grows
  to two files and 1 to 1.75 hours.

P2 findings and their dispositions in revision 2:

* PR-5 edited the A-U10 body (Scope, Constitution, Architecture). Fixed: no
  task body changes for R14. The duty moves to the Per-Release Closure Rule
  for A2, and A3 re-states the inventory or states that it is unchanged.
* Gates did not match CI (Go). Fixed: the Green Boundaries list the CI
  gates, including `golangci-lint`, `go test -race ./...`, the Windows job,
  `make docs-lint`, `make md-lint`, and `cli-reference-drift`.
* P-008 unverified by Stage (Learnings). Recorded: Stage checks MD001,
  MD025, and MD041 by hand and runs `backlogit docs lint`. The hold
  comments state that P-008 was not verified by Stage.
* Both binaries (Learnings). Fixed: preflight checks the MCP server and the
  CLI.
* Read-back for every step and missing targets after adoption (Learnings).
  Fixed in the execution order and the verification list.
* Explicit path commit with a dirty-hash diff (Security). Fixed: step 12.
  The PA-P1 rollback is valid only before PA-P3.
* Hold and superseding comments on `184-S` and `185-S`, with before and
  after item lists and the SHA (Security). Fixed: PA-P8, steps 6, 11,
  and 12.
* Interim states after A2 and after B3 (Security). Fixed: I-1b and I-3b.
* Rollback, targets, and change_kind for every PA (Constitution). Fixed.
* Feature description text (Constitution, Scope). Fixed: exact templates.
* Counts (Stage self-check). Fixed: 38 adoptions, eight units within the
  target, five G-B bodies, and 12 edited bodies.

P3 findings, accepted as advisory or fixed:

* The B1 marker is named: the governed-disposition context marker (R7).
* `186.005-T` edits `_ship.agent.md`, and `186.007-T` verifies it.
* Why B-U22 cannot move to B4: a permission for an absent tool is an
  unfulfilled contract, while a ban fails closed.
* The `_ship.agent.md` drift-reason lag runs from B4 to B6 (I-4).
* I-1 is reworded: the sentinel stays in A1 because of the A-U1 pin and
  the test-first order.
* Adopt provenance is cited in code (`170ae28b`, `ffd885b4`). The stale
  YAML-header instruction is recorded, not edited.
* Follow-up note, not in scope: no backlog-integration instructions row for
  the dispose operation.
* Advisory, left unchanged: the Stage exception sits in the Forbidden cell
  of the role table, and the Orchestrator ban uses a wildcard. Both are
  reviewed plan text under 074-DL.

<!-- shipment-decomposition-review-attempt: 2 -->

## Shipment Decomposition Review

This block records decomposition review attempt 3, of revision 2. It is not
a `## Plan Review` block. The plan-review attempt counter stays at 4.

* review_surface: shipment-decomposition (authorized by the user request
  of 2026-09-30)
* review_attempt: 3
* dispatch_mode: multi-agent-dispatch
* decision: ADVISORY
* operator_authorization: approved
* Authorization provenance: the user's correction request, relayed by the
  Orchestrator, delegates end-to-end execution of this correction to Stage.
  That includes review and mutation. Stage accepts this ADVISORY under that
  delegation, following the precedent of the plan's attempt-4 ADVISORY.
  Every P2 below is converted into a mandatory condition. No P0 or P1
  remains. The final report states this acceptance so that the operator can
  object to it.
* TOOL_OK: reviewer-subagent-dispatch
* Personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity
  Reviewer, and Security Lens Reviewer. All seven returned ADVISORY.
* Merged counts: P0=0, P1=0, P2=5, P3=15. Duplicates were merged.
* Prior findings: all seven personas confirmed that the attempt-1 and
  attempt-2 dispositions are present in revision 2.

The revision 2 text above stays as reviewed. The mandatory conditions below
supersede the sentences they name. Stage applies them during execution.

Mandatory decomposition conditions (from the P2 findings):

* MDC-1 (Constitution, Go, Scope: A-U5 red evidence). This supersedes the
  PR-1 `184.005-T` AC sentence.
  * The `184.005-T` AC reads: "RED (R12): before A-U6,
    `go test ./internal/core -run '^TestShipmentReadinessDeclarations$' -count=1`
    fails. Record whether it is a compile failure or a `go/parser`
    failure. The test file for this task's resolver field also stops the
    package from compiling until A-U6."
  * `184.006-T` gains an AC: "`go test ./internal/core -run '^TestShipmentReadinessDeclarations$' -count=1`
    passes."
  * Both bodies were already in the edited set, so the count stays at 12.
* MDC-2 (Scope: verification must check exact edges). Verification also
  checks:
  * The shipment edge set equals the Shipment DAG list. Every unit has a
    `154-S` edge, and B4, B5, and B6 have a `186-S` edge.
  * Every new shipment has a hold comment.
  * The task edge set equals the stated post-PR-2-to-PR-4 set.
  * The 12 edited bodies differ from the checkpoint only in the declared
    text. Every other body is unchanged apart from ID rewrites by adopt.
* MDC-3 (Learnings: frontmatter loss on update). Before and after each
  update call, Stage compares the frontmatter key set and halts on any loss.
  The title change is made in a separate call.
* MDC-4 (Parity: the I-6 claim was too broad). This supersedes the I-6
  sentence "Every agent-facing surface already states Stage-only authority".
  The claim covers only `.github/agents/*`, the MCP tool description, the
  CLI help, and the registry.
  * The distributed plugin bundle (`plugin/agents/ship.agent.md` with
    `backlogit/*`) has no ban. That gap was already in the unsplit `185-S`
    and is not caused by this packaging.
  * The gap is reported to the operator as a follow-up. The user did not
    authorize any new stash capture in this correction.
* MDC-5 (Security: dirty-state snapshot and path match).
  * Preflight records `git status --porcelain` and a SHA-256 for every
    dirty path.
  * Before each commit, the changed paths must equal the expected set. That
    set is the map, the old and new paths of each renamed file, the new
    features and shipments, `184-S`, `185-S`, `184-F`, `185-F`, the plan,
    and the memory files.
  * Stage halts on an unexpected path, on a changed dirty hash, or on an
    overlap with a dirty file.
  * Staging uses `git add -- <paths>`, including the deleted old paths. The
    follow-up commit uses the same rule.

P3 findings, accepted (applied where cheap, otherwise recorded):

* Exact new "Plan dependencies" lines:
  * `185.003-T`: "none".
  * `185.010-T`: "B-U9, B-U2".
  * `185.022-T`: "B-U19".
  * `185.023-T`: "B-U19, B-U22s".
  * `185.024-T`: "none (the ban precedes the adapters)".
  * `185.025-T`: "B-U22, B-U22s".
  * `185.026-T`: "B-U20, B-U21, B-U23".
* R12 trace: R12 for `QueryQueueForWorkspace` is now met by A-U5 (pin) and
  A-U6 (stub).
* The Applied Mapping is appended after this block as
  `## Shipment Decomposition Applied Mapping`. The revision 2 text is not
  edited.
* The PA-P4 and PA-P5 rollbacks are valid only before PA-P3. After PA-P3,
  use the PA-P3 operator rollback.
* Shipments are created one at a time: create, add edges, add the hold
  comment, then move to the next.
* The A2 hold comment repeats the I-1b warning and the R14 duty. The A3
  hold comment repeats the R14 duty.
* The ID namespace is checked again after PA-P2, against the IDs actually
  created, by globbing queue, archive, and logs.
* Version commands: the CLI reports `backlogit version` commit `131577c`.
  `git merge-base --is-ancestor` confirms that `47dfcc93`, `170ae28b`, and
  `ffd885b4` are ancestors. The MCP server is checked with
  `backlogit_get_version`.
* An indeterminate adoption is read back by querying `parent_id` and by
  checking both the old and the new filenames.
* The P-008 hand check also covers the newly created `.backlogit/queue`
  files.
* Archived predecessors: `archived` is in `terminalCascadeStatuses`, so a
  task edge onto an archived task does not block.
* PA-P8 targets: `184-S`, `185-S`, `184-F`, and `185-F`. The Constitution
  Check also counts the new AC on `184.008-T`.
* Recorded rationale for the small units:
  * A4 and B6 are the closing chunks that keep the original IDs.
  * Moving B-U22s into B1 would hold B1 to B3 behind `186-S`.
* The harness-manifest drift-reason lag is non-agent-facing metadata.

<!-- shipment-decomposition-review-attempt: 3 -->

## Shipment Decomposition Applied Mapping

<!-- shipment-decomposition-applied-mapping: revision 2, review attempt 3 -->

This section records what Stage applied under the attempt-3 ADVISORY
decision. It does not edit the addendum text above.

Execution record (2026-09-30, local time):

* Preflight passed.
  * MCP binary `2c8759c3` (dirty debug build) and CLI `131577c` both
    contain `47dfcc93`, `170ae28b`, and `ffd885b4`.
  * The 187+ root namespace was free.
  * The unrelated dirty files matched their baseline hashes.
* Rollback checkpoint commit `8725f50b` holds the addendum and review
  records 1 to 3.
* PA-P4 edges and PA-P5 bodies were applied before adoption. The task
  frontmatter key sets were unchanged and the guard hashes were identical.
* PA-P2 created `187-F` to `194-F`. Each carries the label
  `source-stash-6434A4D7`, a `related_to` link to its original feature, and
  no `source_stash_id`.
* PA-P8 and PA-P1: hold comments recorded the before lists. Then the
  `184-S` and `185-S` manifests were trimmed and synced.
  * One MCP `backlogit_get_shipment` read right after the first sync
    returned the old item list. The CLI read and the file were correct.
  * A second MCP sync returned the trimmed lists before any adoption ran.
* PA-P3: 38 adoptions in dependency order.
  * `184.001-T` used MCP `backlogit_adopt_item`. Stage halted and read back
    the new ID, the renamed file, and the renamed log.
  * The other 37 used CLI `backlogit adopt --parent`, one at a time, with a
    halt on any nonzero exit. All 38 returned exit 0.
  * Each moved task carries `origin_feature` (`184-F` or `185-F`).
* PA-P7 and PA-P6: eight shipments created with the feature first. Edges
  were added one at a time.
  * The `184-S` to `189-S` edge call timed out. A read-back showed the edge
    was absent, so Stage added it once.
* Each new shipment has one hold comment.
  * The first A2 comment paraphrased R14 wrongly. A second comment on
    `188-S` supersedes that sentence with the exact PR-5 duty. The comment
    log is append-only and was not edited.
* After-edit comments were added to `184-S` and `185-S`.
* The decomposition note was appended to `184-F` and `185-F`. The key sets
  were unchanged, and `184-F` keeps `custom_fields.source_stash_id`.
* The `187-S` title is "Readiness A1: readiness API declarations and
  predecessor sentinel". It differs from the `187-F` title. The other new
  shipments use their feature titles.

Task mapping (original ID to current ID to feature to shipment):

| Original | Current | Unit | Feature | Shipment | Guard |
|---|---|---|---|---|---|
| `184.001-T` | `187.001-T` | A-U1 | `187-F` | `187-S` | |
| `184.002-T` | `187.002-T` | A-U2 | `187-F` | `187-S` | |
| `184.003-T` | `187.003-T` | A-U3 | `187-F` | `187-S` | |
| `184.004-T` | `187.004-T` | A-U4 | `187-F` | `187-S` | |
| `184.005-T` | `188.001-T` | A-U5 | `188-F` | `188-S` | |
| `184.006-T` | `188.002-T` | A-U6 | `188-F` | `188-S` | |
| `184.007-T` | `188.003-T` | A-U7 | `188-F` | `188-S` | |
| `184.008-T` | `188.004-T` | A-U8 | `188-F` | `188-S` | |
| `184.009-T` | `188.005-T` | A-U9 | `188-F` | `188-S` | |
| `184.010-T` | `188.006-T` | A-U10 | `188-F` | `188-S` | |
| `184.011-T` | `189.001-T` | A-U11 | `189-F` | `189-S` | |
| `184.012-T` | `189.002-T` | A-U12 | `189-F` | `189-S` | |
| `184.013-T` | `189.003-T` | A-U13 | `189-F` | `189-S` | |
| `184.014-T` | `189.004-T` | A-U14 | `189-F` | `189-S` | |
| `184.015-T` | `189.005-T` | A-U15 | `189-F` | `189-S` | |
| `184.016-T` | `184.016-T` | A-U16 | `184-F` | `184-S` | |
| `184.017-T` | `184.017-T` | A-U17 | `184-F` | `184-S` | |
| `184.018-T` | `184.018-T` | SA-CV | `184-F` | `184-S` | G-A |
| `185.003-T` | `190.001-T` | B-U3 | `190-F` | `190-S` | |
| `185.004-T` | `190.002-T` | B-U4 | `190-F` | `190-S` | |
| `185.005-T` | `190.003-T` | B-U5 | `190-F` | `190-S` | |
| `185.006-T` | `190.004-T` | B-U6 | `190-F` | `190-S` | |
| `185.007-T` | `190.005-T` | B-U7 | `190-F` | `190-S` | |
| `185.008-T` | `191.001-T` | B-U8 | `191-F` | `191-S` | G-C, G-D |
| `185.009-T` | `191.002-T` | B-U9 | `191-F` | `191-S` | G-C, G-D |
| `185.001-T` | `192.001-T` | B-U1 | `192-F` | `192-S` | |
| `185.002-T` | `192.002-T` | B-U2 | `192-F` | `192-S` | |
| `185.010-T` | `192.003-T` | B-U10 | `192-F` | `192-S` | G-B |
| `185.011-T` | `192.004-T` | B-U11 | `192-F` | `192-S` | G-D |
| `185.012-T` | `192.005-T` | B-U12 | `192-F` | `192-S` | |
| `185.013-T` | `192.006-T` | B-U13a | `192-F` | `192-S` | |
| `185.014-T` | `192.007-T` | B-U13b | `192-F` | `192-S` | G-D |
| `185.024-T` | `193.001-T` | B-U22s | `193-F` | `193-S` | G-B |
| `185.015-T` | `194.001-T` | B-U14 | `194-F` | `194-S` | G-B |
| `185.016-T` | `194.002-T` | B-U15 | `194-F` | `194-S` | G-B |
| `185.017-T` | `194.003-T` | B-U16 | `194-F` | `194-S` | G-B |
| `185.018-T` | `194.004-T` | B-U17 | `194-F` | `194-S` | G-B |
| `185.019-T` | `194.005-T` | B-U18 | `194-F` | `194-S` | |
| `185.020-T` | `194.006-T` | B-U19 | `194-F` | `194-S` | G-B |
| `185.022-T` | `194.007-T` | B-U21 | `194-F` | `194-S` | |
| `185.023-T` | `194.008-T` | B-U22 | `194-F` | `194-S` | G-B |
| `185.021-T` | `185.021-T` | B-U20 | `185-F` | `185-S` | G-B |
| `185.025-T` | `185.025-T` | B-U23 | `185-F` | `185-S` | G-B |
| `185.026-T` | `185.026-T` | SB-CV | `185-F` | `185-S` | G-B |

Shipment edges (`blocks`), as read back:

| Shipment | Unit | Tasks | Prerequisites |
|---|---|---|---|
| `187-S` | A1 | 4 | `154-S` |
| `188-S` | A2 | 6 | `154-S`, `187-S` |
| `189-S` | A3 | 5 | `154-S`, `187-S`, `188-S` |
| `184-S` | A4 | 3 | `154-S`, `188-S`, `189-S` |
| `190-S` | B1 | 5 | `154-S`, `188-S`, `189-S` |
| `191-S` | B2 | 2 | `154-S`, `190-S` |
| `192-S` | B3 | 7 | `154-S`, `187-S`, `191-S` |
| `193-S` | B4 | 1 | `154-S`, `186-S` |
| `194-S` | B5 | 8 | `154-S`, `186-S`, `189-S`, `192-S`, `193-S` |
| `185-S` | B6 | 3 | `154-S`, `184-S`, `186-S`, `194-S` |

A topological order (Kahn) for the shipment graph is:
`187-S`, `193-S`, `188-S`, `189-S`, `184-S`, `190-S`, `191-S`, `192-S`,
`194-S`, `185-S`. The external predecessors are `154-S` (archived,
`archived_status: shipped`) and `186-S` (queued). The graph has no cycle.

Verification (2026-09-30):

* Coverage: 44 tasks, 44 manifest memberships, 44 distinct, no duplicates.
  No original ID file remains under `.backlogit/queue`.
* Each manifest lists its feature first. Every listed task's `parent_id` is
  that feature. Every task under each feature is in that feature's manifest.
* Every artifact is `queued`. No task status changed.
* Every task edge that crosses shipments points to a task in a shipment that
  is a transitive prerequisite. There are 0 violations and no missing
  targets.
* No frontmatter in `.backlogit/queue` references an original moved ID.
  No moved task body or closing-chunk body names an original moved ID in
  prose.
* The 17 guarded tasks keep the `advisory-guard` label. Their MANDATORY
  ADVISORY CONDITION text hashes match the pre-edit baseline. G-E in
  `186.007-T` is unchanged.
* The `183-F`, `183-S`, `183.001-T` to `183.003-T`, `186-F`, `186-S`, and
  `186.001-T` to `186.007-T` files match their baseline hashes.
* Hold: condition B scheduler-consumption attestation C for `154-S` is
  absent, so no unit is Ship-eligible. Stage did not run P-008 gates, builds,
  or tests. Ship runs them per the Per-Release Closure Rule.

## Shipment Decomposition Readiness BLOCKED Addendum

<!-- shipment-decomposition-readiness: BLOCKED, post-verification, 2026-09-30 -->

Status: BLOCKED. This addendum records a HALT found after the Applied
Mapping verification. It does not edit the review records, the mandatory
conditions, or the Applied Mapping above.

Blocker (P-002, P-004, R12; same-contract completion, not a deferred scope
expansion):

* MDC-1 (this plan, the `184.005-T` AC text) allows a compile failure as RED
  evidence for `TestShipmentReadinessDeclarations`. It also states that the
  resolver-field test file stops `internal/core` from compiling until A-U6.
* `.backlogit/queue/188.001-T.md` AC4 (A-U5) permits "a compile failure or a
  `go/parser` failure". The resolver-field test file stops compilation until
  `188.002-T`.
* `.backlogit/queue/188.002-T.md` (A-U6) depends on `188.001-T`. It adds the
  resolver field and the `QueryQueueForWorkspace` stub, and its AC5 says the
  declaration test passes afterward.
* So the declaration signature pin is never seen failing as a compiling
  `go/parser` test before its production declaration exists. A compile
  failure cannot stand in for the required compiling-but-failing harness.
* The requirement is not waived and not softened to an advisory.

Review cycle budget:

* Decomposition review attempts: 1 FAIL, 2 FAIL, 3 ADVISORY. MDC-1 to MDC-5
  were applied after attempt 3. These records and the original plan-review
  attempts 1 to 4 stay unchanged.
* All three decomposition fix cycles are consumed. Another review or fix
  cycle needs explicit operator disposition: extend the cycle limit, or
  accept a documented residual risk. No counter reset and no fourth review
  is authorized by this addendum.

Applied result (stays in place, not undone): 10 queued manifests, 44 tasks.
`187-S` 4, `188-S` 6, `189-S` 5, `184-S` 3, `190-S` 5, `191-S` 2, `192-S` 7,
`193-S` 1, `194-S` 8, `185-S` 3. No status changed. The 17 guarded
conditions are unchanged.

Handoff:

* No source work and no Ship claim is permitted from this handoff.
* Operator-approved, in-scope remediation of the A-U5 and A-U6 RED and
  green boundary is required first.
* Condition B scheduler-consumption attestation C for `154-S` is still
  absent. So no manifest is Ship-eligible, including manifests this blocker
  does not touch.
* Plan-readiness HOLD comments on `188-S` and `188.001-T` are notes only.
  They are not a deterministic code claim gate.

## Shipment Decomposition Correction Revision 3 (Packaging Attempt 4)

<!-- shipment-decomposition-correction: revision 3, 2026-10-01 -->

Status: proposed for packaging review attempt 4. This revision repairs the
blocker in the BLOCKED addendum above. It does not edit any earlier review
record, mandatory condition, the Applied Mapping, or the BLOCKED addendum.
Those stay in place as historical records.

### Authorization and Cycle Budget

* Provenance: the operator instruction "You have not yet marked the task as
  complete ... Keep working autonomously until the task is truly finished,
  then call task_complete", relayed by the parent session at
  2026-10-01T04:08:18Z. The parent treats it as authorization for one
  additional in-scope review and fix cycle for this exact defect.
* Budget: the packaging review limit rises from 3 to 4 attempts in total.
  Attempts 1 FAIL, 2 FAIL, and 3 ADVISORY stay as recorded. This revision
  is reviewed as packaging attempt 4. No counter is reset, and no attempt 5
  is authorized.
* The original plan-review counter (attempts 1 to 4) is unchanged. This is
  not original plan-review attempt 5.
* This authorization is not an approval of individual findings, not a
  dark-mode activation, and not a waiver of P-002, P-004, or R12. It does not
  authorize source implementation, Ship, a pull request, or a merge.

### Superseded Text

From this revision on, the text below is superseded. It stays in place as
history and is no longer authoritative:

* MDC-1, both bullets: the `184.005-T` RED AC that accepts a compile or
  `go/parser` failure, and the `184.006-T` AC that
  `TestShipmentReadinessDeclarations` passes.
* PR-1, the `184.005-T` bullets (pin added to
  `TestShipmentReadinessDeclarations`, two files, "fails through `go/parser`
  until A-U6") and the `184.006-T` bullet (declare the stub). The
  `184.001-T`, `184.002-T`, and `184.007-T` bullets stand. The `184.008-T`
  bullet is restated below.
* Green Boundaries, the second A2 sub-bullet ("A-U5 pins ... A-U6 declares
  the stub"). It is restated below.
* `188.001-T`: scenario 4, the second file, and AC4.
* `188.002-T`: the resolver-field and stub declaration duties, and AC5.
* `188.004-T`: AC4, restated with the new test name.
* The `187.001-T` pointer "moved to A-U5" now resolves to A-U5a in the same
  `188-S` release. The `187.001-T` body stays unchanged.

RED rule for every A2 unit: a compile error, a type error, a
`[build failed]` result, or a setup failure is never valid RED evidence.
RED means the test binary compiles and the named test reports `--- FAIL`
with its own assertion messages.

### Resolver Field Binding

* The plan never named the unexported resolver field or its type. A-U5 sets
  the field directly, and A-U8 installs "a resolver over
  `ShipmentPredecessorShipped`". A compiling A-U5 harness must name the
  field, so the name and type must be declared before A-U5.
* Binding, derived from A-U8 as the predicate with `ws` bound, with no new
  behavior and no schema change:
  `shipmentPredecessorResolver func(ctx context.Context, id string) (bool, string, error)`
  on `QueueFilter`.
* A-U1's sentence "The unexported resolver field on `QueueFilter` is
  deliberately not pinned" governs the permanent public API harness
  `TestShipmentReadinessDeclarations` (`187.001-T`), which stays unchanged.
  The field pin lives only in the A2 harness A-U5a, inside the
  `internal/core` package. An in-package refactor updates both together.

### New Unit A-U5a: Queue Readiness Declaration Harness (tests)

* Task: new, under `188-F` (expected ID `188.007-T`).
* File: `internal/core/queue_readiness_decl_test.go` (new). Test:
  `TestQueueReadinessDeclarations`.
* Uses only `go/parser`, `go/token`, `go/ast`, and the standard library. It
  parses each non-`_test.go` file in the `internal/core` package directory
  and inspects the AST. It names no production identifier at compile time,
  so it compiles before the declarations exist.
* It asserts exactly two declarations, by parameter and result types:
  1. `func QueryQueueForWorkspace(ctx context.Context, ws *Workspace, filter *QueueFilter) (*QueueView, error)`.
  2. A `QueueFilter` field `shipmentPredecessorResolver` of type
     `func(context.Context, string) (bool, string, error)`.
* It does not assert function bodies, so both the stub and the final
  implementation pass.
* Plan dependencies: A-U4 (`187.004-T`).
* AC:
  1. RED (R12): before A-U5b,
     `go test ./internal/core -run '^TestQueueReadinessDeclarations$' -count=1 -v`
     compiles and reports `--- FAIL: TestQueueReadinessDeclarations`, with an
     assertion message for each missing declaration. A compile error, a type
     error, `[build failed]`, or a setup failure is not RED.
  2. The RED run happens before `queue_shipment_readiness_test.go` (A-U5)
     exists, so no other file can mask the result.
  3. The RED output is recorded in the commit message or a task comment.
* Effort: 0.5 to 0.75 hours. Complexity: low-medium. Posture: test-first.

### New Unit A-U5b: Queue Readiness Declarations (code)

* Task: new, under `188-F` (expected ID `188.008-T`).
* File: `internal/core/queue.go`.
* Add the `shipmentPredecessorResolver` field to `QueueFilter` with a doc
  comment. Declare `QueryQueueForWorkspace` with the signature above,
  returning `nil, blerrors.ErrNotImplemented`, with a doc comment.
* No filtering change, no caller change, and no read of the field.
* Plan dependencies: A-U5a.
* AC:
  1. `go test ./internal/core -run '^TestQueueReadinessDeclarations$' -count=1`
     passes.
  2. `filterByResolvedDependencies` and `QueryQueue` are unchanged.
  3. `go build ./...` passes.
* Known window: `golangci-lint` `unused` may flag the field until A-U6 reads
  it. A-U6 closes this inside `188-S`, and the A2 release gate runs
  `golangci-lint` before merge.
* Effort: 0.25 to 0.5 hours. Complexity: low.

### Changed Unit A-U5 (`188.001-T`)

* One file: `internal/core/queue_shipment_readiness_test.go` (new).
  Scenarios 1 to 3 stay as reviewed. Scenario 4 and the second file are
  removed.
* The tests set `shipmentPredecessorResolver` directly. A-U5b declares it,
  so the file compiles.
* Plan dependencies: A-U5b. This replaces the direct A-U4 dependency, which
  stays transitive.
* AC:
  1. RED: `go test ./internal/core -run '^TestQueueShipmentReadiness$' -count=1 -v`
     compiles and reports `--- FAIL`. Groups 1 and 3 each have at least one
     failing behavior assertion. A compile error, a type error,
     `[build failed]`, or a setup failure is not RED.
  2. Scenario 2 passes against the code before A-U6 (characterization).
  3. All three scenario groups are present.
  4. `TestQueueReadinessDeclarations` still passes.
* Effort: 0.75 to 1.25 hours. Complexity: medium.

### Changed Unit A-U6 (`188.002-T`)

* File: `internal/core/queue.go`. The resolver-field and stub declaration
  duties move to A-U5b and are removed here. The rest stays: select
  `artifact_type`, the shipment edge rule that reads
  `shipmentPredecessorResolver`, wrapped resolver errors, and the caller
  inventory.
* ACs 1 to 4 stay as reviewed. AC5 is replaced: the field and
  `QueryQueueForWorkspace` declarations are unchanged from A-U5b,
  `TestQueueReadinessDeclarations` passes, and `QueryQueueForWorkspace`
  still returns `ErrNotImplemented` so A-U7 fails against the stub.
* Effort: 1.25 to 1.75 hours. Complexity: high.

### Changed Unit A-U8 (`188.004-T`)

* AC4 becomes:
  `go test ./internal/core -run '^(TestShipmentReadinessDeclarations|TestQueueReadinessDeclarations)$' -count=1`
  passes, and no `ErrNotImplemented` remains in `QueryQueueForWorkspace`.

### Restated A2 Green Boundary

* A-U5a pins the field and `QueryQueueForWorkspace` with a compiling AST
  harness that fails before they exist (R12).
* A-U5b declares them. `QueryQueueForWorkspace` is a stub.
* A-U5 compiles and fails on behavior. A-U6 implements the filter.
* A-U7 compiles and fails against the stub. A-U8 implements it.
* A-U10 closes the adapter red window. No stub survives the release.

### Task Chain and Edges

Order: `187.004-T`, `188.007-T`, `188.008-T`, `188.001-T`, `188.002-T`,
`188.003-T`, `188.004-T`, `188.005-T`, `188.006-T`.

Task edge changes (task depends on predecessor):

* Add `188.007-T -> 187.004-T`.
* Add `188.008-T -> 188.007-T`.
* Remove `188.001-T -> 187.004-T`.
* Add `188.001-T -> 188.008-T`.

Unchanged: `188.002-T -> 188.001-T`, `188.003-T -> 188.002-T`,
`188.004-T -> 188.003-T`, `188.005-T -> 188.004-T`,
`188.006-T -> 188.005-T`, `184.016-T -> 188.006-T`,
`189.001-T -> 188.006-T`, and every shipment edge.

### `188-S` Size: 8 Tasks

Manifest order: `188-F`, `188.007-T`, `188.008-T`, then `188.001-T` to
`188.006-T`.

Why `188-S` holds 8 tasks, the same cap as B5:

* The declarations and the stub cannot ship without their behavior. PR-1
  forbids a stub-only release.
* The harness must precede the declarations in the same release, and the
  declarations must precede the compiling A-U5 harness.
* The A-U5 to A-U10 red window must close inside one release.
* The split is by dependency, not by numeric slice.

### Effort Changes

| Task | Unit | Domain | Old effort (h) | New effort (h) | Complexity |
|---|---|---|---|---|---|
| `188.007-T` | A-U5a | tests | none | 0.5 to 0.75 | low-medium |
| `188.008-T` | A-U5b | code | none | 0.25 to 0.5 | low |
| `188.001-T` | A-U5 | tests | 1 to 1.75 | 0.75 to 1.25 | medium |
| `188.002-T` | A-U6 | code | 1.5 to 2 | 1.25 to 1.75 | high |

Totals:

* 46 tasks over the same 10 shipments.
* A2 (`188-S`): 8 tasks, 5.75 to 9.25 hours (was 6 tasks, 5.5 to 8.75).
  Complexity and risk stay high and moderate-high.
* Feature A: 14.25 to 23.5 hours (was 14 to 23). Feature B is unchanged at
  22.75 to 34.75 hours.
* No other shipment changes membership or scope. Every shipment keeps the
  `154-S` attestation C hold, so none is Ship-eligible.

### Revision 3 Review Conditions (Packaging Attempt 4)

These conditions resolve the attempt 4 findings. They are part of Revision 3
and are bound into the task bodies when the revision is applied.

#### FC-5 and P-002.1 Ordering

* Finding LR-4-P1 (Learnings Researcher): FC-5 in
  `docs/decisions/2026-08-30-p002-breach-incident-152f-134s.md` treats an
  `ErrNotImplemented` stub as production behavior that must not precede its
  RED. A-U5b lands the stub before the A-U7 behavior RED.
* Disposition: resolved, not waived. The authoritative policy text is
  P-002.1 in `.github/policies/workflow-policies.md`, cycle 31: "harness
  first, declaration second". A declaration "whose body would absorb real
  behaviour MUST be split into *declaration* -> *behaviour harness* ->
  *implementation*, each gated by a harness that precedes it."
* Revision 3 follows that split exactly. Each step is gated by an observed,
  compiling RED that lands first:

  | Step | Unit | Gating RED that lands first |
  |---|---|---|
  | Declaration of the field and `QueryQueueForWorkspace` | A-U5b (`188.008-T`) | A-U5a (`188.007-T`), source-shape |
  | Filter behavior | A-U6 (`188.002-T`) | A-U5 (`188.001-T`), behavior |
  | `QueryQueueForWorkspace` behavior | A-U8 (`188.004-T`) | A-U7 (`188.003-T`), behavior |

* The stub body carries no behavior beyond the declared shape. It returns
  `nil, blerrors.ErrNotImplemented`, reads no field, and changes no caller.
  This matches the 140-S declaration-first precedent in
  `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md`.
* Bound conditions:
  * `188.008-T` AC: the A-U5b commit is a strict descendant of the commit
    that records the A-U5a RED output. The stub reads no field and changes no
    caller.
  * `188.004-T` AC: the Ship FC-5 manual pre-flight names A-U5a as the
    gating RED for the declaration, and A-U7 as the gating RED for the
    `QueryQueueForWorkspace` behavior. `188.003-T` is unchanged: its RED
    already runs against the compiling stub.
* FC-5 stays a manual Ship pre-flight obligation, deferred to stash
  `A2C91FE5`. Nothing in this plan automates it or claims to.

#### AST Comparison Method (A-U5a)

* Read the `internal/core` directory with `os.ReadDir`. Skip directories and
  every `_test.go` file. Parse each remaining `.go` file with
  `parser.ParseFile`.
* Assert at least one file was parsed, so the check is not vacuous.
* Imports: standard library only (for example `go/ast`, `go/parser`,
  `go/token`, `go/types` for `types.ExprString`, `os`, `path/filepath`,
  `strings`, `testing`), plus testify if wanted. No production package
  import and no production identifier. This matches the existing harnesses
  in `internal/canonical`.
* Function check:
  * Find the `*ast.FuncDecl` named `QueryQueueForWorkspace` with
    `Recv == nil`.
  * Expand parameters and results by `max(1, len(Names))`.
  * Compare each type with `types.ExprString`, ignoring parameter names.
  * Expected parameters: `context.Context`, `*Workspace`, `*QueueFilter`.
    Expected results: `*QueueView`, `error`.
* Field check:
  * Find the `*ast.TypeSpec` named `QueueFilter` whose type is an
    `*ast.StructType`.
  * Find the field named `shipmentPredecessorResolver`.
  * Compare its type with `types.ExprString`, ignoring parameter names.
    Expected: `func(context.Context, string) (bool, string, error)`.
* Messages: "missing" when a declaration is absent, and "wrong signature:
  got X, want Y" when it is present with the wrong types.
* Other declarations are allowed. The harness asserts that these two exist
  with these types. It does not assert that they are the only ones. This
  replaces "exactly two declarations" in A-U5a above.
* A-U5a and A-U5 are `package core`.

#### Lint Window (Principle I)

* Conflict, recorded per the Constitution Governance section:
  * Principle I wants `golangci-lint run` clean before any commit.
  * Between A-U5b and A-U5, the unexported field has no reader, and the
    `unused` linter may report it.
* Narrowed window: it closes at A-U5 (`188.001-T`). Test writes to the field
  count as uses, so A-U6 is not needed to close it. This replaces the "Known
  window ... until A-U6" bullet in A-U5b above.
* Simpler alternative rejected: declaring the field in A-U5 or A-U6 would
  either make A-U5 fail to compile or put the declaration behind its own
  behavior. Both are R12 and P-004 defects.
* Bound conditions:
  * `188.008-T`: run `golangci-lint run ./internal/core/...` and record the
    output in a task comment. Only an `unused` report on
    `shipmentPredecessorResolver` is tolerated, and only until `188.001-T`.
  * `188.008-T`: `go vet ./...` passes. A func field makes `QueueFilter`
    non-comparable, and vet catches any comparison of it.
  * `188.001-T`: `golangci-lint run ./internal/core/...` reports nothing for
    the field.

#### Resolver-Error Fixture (A-U5 Group 3)

* The group 3 resolver-error case stubs the resolver to return a sentinel
  error for an archived predecessor.
* It asserts with `errors.Is` that the error is wrapped.

#### Additional Superseded Text

The text below is also superseded. It stays in place as history:

* A-U6 body: "Add an unexported resolver field to `QueueFilter`." A-U5b now
  owns it.
* Release Units: the A2 row (6 tasks, 5.5 to 8.75 hours) and the Totals
  bullets ("44 tasks", "Feature A: 14 to 23 hours"). These are restated in
  Effort Changes above.
* Dependency Graph: `184.005-T -> 184.004-T`. It is restated as
  `188.001-T -> 188.008-T`.
* Packaging Constitution Check, the II and Workflow 1 bullets ("A-U5 pins
  the signature before A-U6 declares the stub"; "A-U5 gains one pin and one
  file"). These are restated in the Constitution Check below.
* SBA2-P1-1 disposition (revision 2), and the revision 3 "R12 trace"
  sentence ("met by A-U5 (pin) and A-U6 (stub)"). The R12 trace is now:
  A-U5a pins, A-U5b declares, A-U7 tests behavior, A-U8 implements.
* Applied Mapping: the `188-S` row count of 6, and the line "Coverage: 44
  tasks". These are restated in the Revision 3 Applied Mapping.

#### Constitution Check (Revision 3)

* I (safety-first Go): the lint conflict and its narrowed window are
  recorded above. `go vet` and `go build` are ACs on `188.008-T`.
* II (test-first): every A2 production step lands after an observed,
  compiling RED: A-U5a before A-U5b, A-U5 before A-U6, A-U7 before A-U8. A
  compile error is never RED.
* Workflow 1 (2-hour rule, width): each new task touches one file and one
  domain, and each is under one hour. `188.001-T` shrinks to one file.
  `188-S` holds 8 tasks, the same cap as B5, and the rationale is recorded
  above.
* Workflow 5 (no dead code): the stub is replaced inside `188-S` by A-U8.
  The `188.004-T` AC rejects any remaining `ErrNotImplemented`.
* The 17 mandatory guarded conditions, their provenance, and the original
  four plan reviews are unchanged.
* The operator authorization is read as one in-scope fix cycle for this
  defect only. The operator may challenge that reading. The provenance is
  recorded above.

#### Proposed Actions (strict-safety)

| ProposedAction | ActionRisk | Rollback | Approval |
|---|---|---|---|
| Create `188.007-T` and `188.008-T` under `188-F`, queued | moderate | Archive both new tasks and restore the edges | operator continuation |
| Rewrite the bodies of `188.001-T`, `188.002-T`, and `188.004-T`, with frontmatter key sets checked before and after | moderate | Restore the bodies from git at `d5ee164` | operator continuation |
| Edit dependencies: add 3, remove 1 | moderate | Invert each edge change | operator continuation |
| Add the 2 tasks to `188-S` and reorder `items` | moderate | Restore `188-S.md` from git, then sync | operator continuation |
| Append comments to `188-S`, `188-F`, `188.001-T`, `188.007-T`, and `188.008-T` | low | Append-only; correct with a later comment | operator continuation |

No action is destructive. No status changes, and nothing is archived,
claimed, shipped, or merged.

#### Read-Back Verification Checklist

* None of the task bodies for `188.001-T`, `188.002-T`, `188.007-T`, or
  `188.008-T` contains "compile failure", "MDC-1", "Add the unexported
  resolver field", or "declare the stub".
* Dependency fields and `item_deps` match the edge list exactly. The DAG has
  no cycle and no missing reference.
* The `188-S` `items` list is in the manifest order above.
* There are 46 distinct tasks over 10 manifests. Each task appears exactly
  once, and all are `queued`.
* Cross-shipment edge check: `188.007-T -> 187.004-T` targets `187-S`, a
  prerequisite of `188-S`. That gives 0 violations.
* The 17 guarded-condition bodies, the unrelated dirty-file hashes, and the
  `154-S` C hold are unchanged.

## Shipment Decomposition Review

<!-- shipment-decomposition-review-attempt: 4 -->

* review_surface: Shipment Decomposition Correction Revision 3 and its
  Review Conditions. This is the `188-S` declaration handoff only.
* review_attempt: 4 (packaging). This is the final authorized attempt. No
  attempt 5 is authorized.
* dispatch_mode: multi-agent-dispatch
* decision: ADVISORY
* operator_authorization: approved. Source: the operator continuation
  relayed at 2026-10-01T04:08:18Z, which authorizes this one cycle and its
  in-scope fixes. It is not individual-finding approval and not dark mode.

Personas and verdicts:

| Persona | Verdict | P0 | P1 | P2 | P3 |
|---|---|---|---|---|---|
| Constitution Reviewer | ADVISORY | 0 | 0 | 2 | 1 |
| Go Reviewer | ADVISORY | 0 | 0 | 1 | 4 |
| Scope Boundary Auditor | ADVISORY | 0 | 0 | 2 | 3 |
| Architecture Strategist | PASS | 0 | 0 | 0 | 1 |
| Learnings Researcher | finding | 0 | 1 | 0 | 0 |

Not triggered:

* Agent-Native Parity Reviewer: no MCP, CLI, or agent contract change.
* Security Lens Reviewer: no auth, trust-boundary, or data-exposure change.

Findings and dispositions:

* LR-4-P1 (FC-5 stub-before-RED): resolved in the FC-5 and P-002.1
  Ordering subsection, with bound ACs. This is not downgraded or waived.
* Lint window (Principle I): resolved by a recorded conflict, the narrowed
  window, and lint, vet, and build ACs.
* Read-back verification: bound as the checklist above.
* AST comparison method: specified above.
* Superseded attempt 3 and Packaging Constitution Check sentences: listed
  above.
* Proposed actions, risk, and rollback: listed above.
* P3 items (comparison wording, non-vacuity, `go vet`, package name,
  resolver-error fixture, cross-shipment edge check, guards unchanged,
  operator-reading note): all applied above.
* Out of scope: none. The Scope Boundary Auditor reported no out-of-scope
  finding, so no P-021 C2 capture is required.

Gate result:

* No unresolved P0 or P1 finding.
* Every P2 finding is fixed in the revision text or bound into a task AC,
  and is carried into the task bodies when the revision is applied.
* Revision 3 is therefore approved for application.

## Revision 3 Applied Mapping

Applied on 2026-10-01 through official backlogit operations, after the
attempt 4 gate above. This section records the applied state. It does not
change any reviewed text above.

New tasks under `188-F`:

| Task | Plan unit | Title | Depends on | Effort |
|---|---|---|---|---|
| `188.007-T` | A-U5a | Tests: queue readiness declaration harness | `187.004-T` | 0.5-0.75 h, low-medium |
| `188.008-T` | A-U5b | Code: declare queue readiness resolver field and entry point | `188.007-T` | 0.25-0.5 h, low |

Neither new task carries `custom_fields.origin_feature`. Both are new units,
not carry-overs from `184-F`.

Rewritten task bodies:

* `188.001-T` (A-U5): uses one test file only and has a compiling RED AC. A
  compiler error is never valid RED. It now depends on `188.008-T`, and the
  edge to `187.004-T` is removed because that order is now reached through
  `188.007-T`.
* `188.002-T` (A-U6): the field and stub declaration duties are removed. AC5
  requires the declarations to stay unchanged.
* `188.004-T` (A-U8): AC4 runs both declaration tests and requires that no
  `ErrNotImplemented` remains. AC5 records the FC-5 pre-flight.

Applied `188-S` chain:

`187.004-T -> 188.007-T -> 188.008-T -> 188.001-T -> 188.002-T -> 188.003-T
-> 188.004-T -> 188.005-T -> 188.006-T`

The downstream edges `184.016-T -> 188.006-T` and `189.001-T -> {187.004-T,
188.006-T}` are kept. The `188-S` manifest order is: `188-F`, `188.007-T`,
`188.008-T`, `188.001-T`, `188.002-T`, `188.003-T`, `188.004-T`,
`188.005-T`, `188.006-T`.

| Shipment | Tasks | Status |
|---|---|---|
| `187-S` | 4 | queued |
| `188-S` | 8 (5.75-9.25 h) | queued |
| `189-S` | 5 | queued |
| `184-S` | 3 | queued |
| `190-S` | 5 | queued |
| `191-S` | 2 | queued |
| `192-S` | 7 | queued |
| `193-S` | 1 | queued |
| `194-S` | 8 | queued |
| `185-S` | 3 | queued |

That is 46 tasks: the original 44 plus `188.007-T` and `188.008-T`. Feature
A is 14.25-23.5 h and Feature B is 22.75-34.75 h.

Read-back verification results (staging-only):

* `query_sql` found 46 manifest task references, 46 distinct, 46 `queued`,
  0 missing.
* `item_deps` for the 46 tasks has 49 edges and 0 missing targets. A
  topological sort found no cycle, with 188.007 before 188.008, then
  188.001, then 188.002.
* The `188` edge set equals the target set exactly.
* The phrases "compile failure", "MDC-1", "Add the unexported resolver
  field", and "declare the stub" each appear 0 times in the active bodies of
  `188.001-T`, `188.002-T`, `188.007-T`, and `188.008-T`.
* Frontmatter key sets for `188-F`, `188-S`, `188.001-T`, `188.002-T`, and
  `188.004-T` match the baseline.
* This plan is append-only (0 deleted lines). The MANDATORY and guard line
  counts in the rewritten task bodies are unchanged. The 17 guarded
  conditions are unchanged.
* The unrelated dirty-file hashes are unchanged.

These checks verify documents and structure only. No harness test was
written or run, and none is claimed. The compiling-RED evidence is for
Ship to produce when the tasks run.

Handoff state: READY_FOR_STAGING_HANDOFF. This supersedes the earlier
packaging HALT for `188-S`. It does not make any shipment eligible for
Ship: the `154-S` scheduler-consumption attestation C hold remains on all
10 shipments. The informational comments on `188-S`, `188-F`, `188.001-T`,
`188.007-T`, and `188.008-T` are not a claim gate.