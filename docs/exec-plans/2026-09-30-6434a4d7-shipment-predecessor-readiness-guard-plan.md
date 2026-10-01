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
| R3 | The queue filter, `QueryQueueForWorkspace`, `MoveInQueue`, and both adapters apply R1 to shipment-to-shipment `blocks` edges, including edges with an empty type, and leave every other edge type and artifact pairing unchanged | 6434A4D7 (1) | A-U5 to A-U10 |
| R4 | `ClaimShipment` refuses with a typed sentinel and the stable token `shipment_predecessor_not_shipped` before any member activation when R1 fails, reading edges from the reloaded Markdown and failing closed on unresolvable predecessors | 6434A4D7 (2); 2026-09-20 M2 | A-U11 to A-U13 |
| R5 | MCP maps the claim refusal to code `shipment_predecessor_not_shipped`, and the CLI claim returns an error carrying the token and the predecessor ID | 6434A4D7 (2) parity | A-U14, A-U15 |
| R6 | `docs/workflow.md` and the backlogit harness instruction state the shipped-only rule and the refusal code, with the rendered-file drift recorded | Parity; harness hygiene | A-U16, A-U17 |
| R7 | `queued` to `abandoned` on a shipment is refused on every generic write path (`MoveShipmentStatus`, `UpdateArtifact`, `UpdateArtifactWithGate`, `BulkUpdateStatus`) and opens only in `MoveShipmentStatus` under the governed-disposition marker | 6434A4D7 (3); review attempt 2 | B-U3 to B-U7 |
| R8 | A pending `dispose` lifecycle journal is provable by recovery: an intent journal rolls back to the preimage, a committed journal keeps the on-disk target, and a leftover journal never blocks later lifecycle operations | 6434A4D7 (3); review attempt 2 | B-U8, B-U9 |
| R9 | `DisposeQueuedShipment` requires `Confirm`, a reason, an actor (`By`), and an `AuthorizationRef` that resolves to an existing `deliberation` artifact whose `authorizes_disposition` custom field names the target shipment. It records provenance that survives later `update` and archive, activates nothing, mutates no member or feature, refuses non-queued targets and shipments with live dependents, and rolls back on a non-indeterminate failure | 6434A4D7 (3) | B-U10 to B-U13 |
| R10 | The disposition is reachable through MCP and CLI with registry, parity-test, and docs parity. The registry exposes no automatic CLI fallback; only a confirmed-only command | 6434A4D7 (3); Constitution VII | B-U14 to B-U21 |
| R11 | The Stage contract permits disposition only with an operator-authored, target-bound deliberation, in Careful mode with in-session operator approval; the Orchestrator contract states it never disposes; the drift is recorded | Stage role boundary | B-U22, B-U23 |
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
  1. Shipment edges: a queued shipment with a `blocks` edge, or an edge with
     an empty type, onto an abandoned shipment, or onto an archived shipment
     the resolver reports as not shipped, is excluded. One onto a predecessor
     the resolver reports as shipped is included.
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
  where both the item and the predecessor are shipments and the type is
  `blocks` or empty, release the edge only when the index status is
  `shipped`, or the resolver returns `true`. With no resolver, an `archived`
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
  3. Source scan, in the CLI test file: a `go/parser` scan of the non-test
     files in `internal/cli` and `internal/mcp` finds no call to
     `core.QueryQueue`; only `core.QueryQueueForWorkspace` is used.
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
     `.backlogit/`.
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
  It reads `blocks` edges, including edges with an empty type, from the
  reloaded Markdown (`current.Dependencies`), not from the index. For each
  edge: load the predecessor; an unresolvable ID refuses
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
  unexported test seam `disposeAfterProvenanceHook func() error` that is nil
  in production. Add
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
  1. With the governed-disposition marker in the context,
     `MoveShipmentStatus` moves a `queued` shipment to `abandoned` and
     appends one `shipment_status_changed` event.
  2. The marker opens no other edge: `queued` to `shipped` and `active` to
     `queued` are still refused with the marker present.
* RED: `go test ./internal/core -run '^TestGovernedShipmentDispositionEdge$' -count=1`
  fails.
* Posture: test-first.

#### B-U7: Governed disposition edge (code)

* File: `internal/core/shipment.go`.
* Add `governedShipmentDispositionContextKey struct{}` next to
  `governedShipmentActivationContextKey`. In `MoveShipmentStatus`, allow
  `queued` to `abandoned` only when that marker is present, before the
  generic `isValidShipmentTransition` check, and leave
  `isValidShipmentTransition` unchanged. `MoveShipmentStatus` already sets
  `allowGovernedShipmentMutation` for moves that do not involve `blocked`.
* Verify: the B-U6 and B-U3 commands pass.

#### B-U8: Dispose journal recovery tests (tests)

* File: `internal/core/shipment_disposition_recovery_test.go` (new).
* Each scenario writes a `dispose` lifecycle journal by hand to simulate a
  crash, then runs `recoverPendingShipmentOperations`.
* Scenarios:
  1. Intent journal after the provenance write, and after the status write:
     recovery restores the shipment preimage (`queued`, no `disposition_*`
     keys), leaves member files unchanged, keeps links, removes the dispose
     events by correlation ID, and removes the journal.
  2. Committed journal: recovery keeps the on-disk `abandoned` state with
     its provenance and removes the journal.
  3. A leftover intent journal does not block a later `ClaimShipment` of
     another shipment or an `AddItemToShipment`; recovery resolves it first.
* RED: `go test ./internal/core -run '^TestDisposeJournalRecovery$' -count=1`
  fails, because `dispose` is not an allowed operation.
* Posture: test-first.

#### B-U9: Dispose journal recovery (code)

* Files: `internal/core/shipment_ops.go`, `internal/core/shipment_recovery.go`.
* `shipment_ops.go`: in `validateShipmentLifecycleJournalRecord`, allow the
  `dispose` operation with the tuple `rollback`/`abandoned`.
* `shipment_recovery.go`:
  * In `reconcileShipmentLifecycleIntent`, accept `dispose`. The intent
    carries member preimages that cover the whole manifest, as the existing
    check requires.
  * In `shipmentRecoveryCandidates`, add a `dispose` case: the shipment
    candidates are the preimage and the preimage with `abandoned` plus the
    disposition provenance keys.
  * In `memberRecoveryCandidates`, add a `dispose` case whose only candidate
    is each member preimage.
  * Remove the dispose operation events by correlation ID on rollback, as
    the claim branch does with `removeShipmentOperationEvents`.
  * An on-disk state that matches no candidate keeps the existing
    `ErrShipmentConflict` behavior and names the journal for operator repair.
* Verify: the B-U8 command passes, and
  `go test ./internal/core -run 'Recover' -count=1` passes.

#### B-U10: DisposeQueuedShipment success and input tests (tests)

* File: `internal/core/shipment_disposition_test.go` (new).
* Scenarios:
  1. Success: a `queued` shipment with a valid authorization moves to
     `abandoned`. `custom_fields` records `disposition_reason`,
     `disposition_by`, `disposition_authorization_ref`, and `superseded_by`
     when set. Exactly one status event is written, no member or
     covering-feature file changes (hash before and after), no journal, claim
     marker, or preimage remains, and the members become assignable to
     another shipment.
  2. Input refusals: `Confirm == false` returns `ErrConfirmationRequired`; an
     empty `Reason`, `By`, or `AuthorizationRef` returns `ErrValidation`; a
     `SupersededBy` that names a missing or non-shipment ID returns
     `ErrValidation`.
  3. Authorization refusals, each `ErrValidation`: an `AuthorizationRef` that
     does not resolve; one that resolves to a non-`deliberation` artifact; a
     `deliberation` whose `authorizes_disposition` custom field is missing or
     names a different shipment.
* RED: `go test ./internal/core -run '^TestDisposeQueuedShipment$' -count=1`
  fails against the stub.
* Posture: test-first.

#### B-U11: DisposeQueuedShipment state and failure tests (tests)

* File: `internal/core/shipment_disposition_state_test.go` (new).
* Scenarios:
  1. State refusals with `ErrShipmentDispositionRefused`: the target is
     `active`, `blocked`, `shipped`, `abandoned`, `archived`, or not a
     shipment.
  2. Live dependents: a non-terminal shipment holds a `blocks` edge onto the
     target. The refusal names the dependents. Edges are never carried over
     to `superseded_by`.
  3. Failure handling through `disposeAfterProvenanceHook`: a
     non-indeterminate error rolls back to `queued` with no `disposition_*`
     keys, no journal, and links intact; an `ErrWriteIndeterminate` error is
     returned unchanged and leaves the journal for recovery to reconcile to
     the on-disk state.
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

#### B-U13: Implement DisposeQueuedShipment (code)

* File: `internal/core/shipment_disposition.go`.
* Steps, in order:
  1. Validate inputs and the authorization. The `AuthorizationRef` must
     resolve with `FindArtifactPath` to an artifact with
     `artifact_type == deliberation` whose ID matches `deliberationIDPattern`
     and whose `authorizes_disposition` custom field names the target.
  2. Take the global shipment lifecycle lock, call
     `recoverPendingShipmentOperations`, then take the membership and artifact
     mutation locks in the `ClaimShipment` order.
  3. Load the shipment once with `findArtifact` (Markdown). Require `queued`
     and no live reverse `blocks` dependents.
  4. Capture the shipment and member preimages and write the
     `shipmentLifecycleJournal` intent (operation `dispose`,
     `RecoveryPolicy: "rollback"`, target `abandoned`) with a correlation ID.
  5. Inside the journal window, write the provenance into `custom_fields`
     and move the status through `MoveShipmentStatus` with the
     governed-disposition marker, so the event carries the correlation ID.
  6. Mark the journal committed, then remove it, as `ClaimShipment` does.
  7. On a non-indeterminate failure, roll back to the preimage and remove the
     journal. Return `ErrWriteIndeterminate` unchanged, without retry or
     rollback, so recovery reconciles it.
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
  * Add the conditions: the authorizing `deliberation` records the operator's
    decision with the operator quoted verbatim and names the target in
    `authorizes_disposition`; Stage never creates that deliberation in the
    same session in which it disposes; Stage enters Careful mode and gets
    in-session operator approval before the call; Stage never disposes on
    its own initiative, and never disposes an `active`, `blocked`, or
    `shipped` shipment.
* Orchestrator: it never disposes shipments, and the `_Ship` tool list omits
  the tool on purpose.
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
B-U9 -> B-U10 -> B-U11 -> B-U12 -> B-U13
B-U13 -> B-U14 -> B-U15 -> B-U16 -> B-U17 -> B-U18 -> B-U19 -> B-U20 -> B-U21
B-U19 -> B-U22 -> B-U23
```

Shipment edges:

* `SB` `blocks` on `SA`. Both edit `internal/errors/errors.go`,
  `internal/mcp/errors.go`, `internal/core/queue.go`, `docs/workflow.md`, and
  `.autoharness/harness-manifest.yaml`, and B-U12 uses the A-U4 predicate.
* `SB` `blocks` on the `AF1E5075` shipment. Both edit
  `.github/agents/_orchestrator.agent.md`, `.github/agents/_stage.agent.md`,
  and `.autoharness/harness-manifest.yaml`.
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
  path, the unexported field cannot be set by callers, and a source scan pins
  the adapters to it.
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
* **One governed edge, not a separate write path.** The disposition reuses
  `MoveShipmentStatus`, the lifecycle journal, recovery, events, hooks, and
  the write envelope. Only the context marker opens `queued` to `abandoned`.
  This mirrors the governed-activation marker that `ClaimShipment` already
  uses.
* **Provenance in `custom_fields`.** No model change is needed, and B-U12
  proves the keys survive `update` and archive.
* **Authorization reference.** backlogit has no `decision` artifact type, so
  the reference is a `deliberation`. Core enforces a structural, target-bound
  reference: the deliberation exists and names the target in
  `authorizes_disposition`. Core cannot prove the operator wrote it. Operator
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
* Task Granularity: 40 units, each within the 2-hour rule. Pass.

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
* Feature B runtime check after merge: `backlogit shipment dispose` on a
  throwaway fixture shipment in `logs/6434a4d7-fixture/` returns `abandoned`
  with provenance. The approval basis is that the fixture is throwaway and no
  live shipment changes; removing the fixture directory needs operator
  approval. `backlogit doctor` on the live workspace reports no new findings.
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
  is unchanged.
* `156-S` and every other existing shipment are unchanged.

### Risky Actions

| ProposedAction | ActionRisk | Approval | Rollback |
|---|---|---|---|
| PA1: insert `requireShipmentPredecessorsShipped` into `ClaimShipment` (A-U13) | High: every claim caller | Plan-review PASS; Ship review and CI | Revert the A-U13 commit; the helper has one call site |
| PA2: shipment-edge readiness in the queue (A-U6, A-U8, A-U10) | Medium: changes which shipments the queue lists | Plan-review PASS; live impact inventory | Revert A-U6, A-U8, and A-U10 |
| PA3: protect `queued` to `abandoned` on generic paths (B-U5) | Medium: refuses a write that succeeds today | Plan-review PASS; B-U4 audit | Revert the B-U5 commit |
| PA4: governed edge, dispose recovery, and `DisposeQueuedShipment` (B-U7, B-U9, B-U13, B-U15, B-U17) | Destructive: terminal state change on shipments | Plan-review PASS; per call: core-enforced confirmation and a target-bound deliberation | Code: revert the commits. Data: not reversible through the API; a mistaken disposition is corrected by a replacement shipment. Ship verification never runs it against the live backlog |
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
    `block`, `unblock`, or `normalize` recovery branches, stop and return to
    Stage.
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
