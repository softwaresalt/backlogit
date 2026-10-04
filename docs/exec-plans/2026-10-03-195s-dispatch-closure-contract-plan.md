---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for the 195-S follow-up repairs: every Orchestrator invocation of Ship first resolves, validates, binds, and passes both served roots (227A2930), and shipment-reconcile accepts an explicit feature member that governed ShipShipment completes inside its own transaction (B83081F5).'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md
title: 'Implementation Plan: 195-S follow-up Orchestrator served-root handoff and explicit-feature reconcile contract'
---

# Implementation Plan: 195-S follow-up dispatch and closure contract repairs

## Objective

Close the two 195-S `READY_WITH_CONDITIONS` conditions that block the next autonomous
pipeline cycle, without adding lifecycle authority, production Go changes, or a new MCP
surface:

1. **Served-root handoff (`227A2930`).** Every Orchestrator invocation of the Ship subagent
   first runs one named, read-only procedure:
   * resolve both absolute served roots
   * validate them
   * attest them against the MCP server's own read-only self-report of the roots it is
     bound to, then bind them to the served shipment record
   * pass them to Ship

   Any failure, error, or indeterminate result halts with `SERVED_ROOTS_UNRESOLVED`. Ship
   then repeats the same Served-Root Attestation at use time: once per wave admission before
   any raw item-log read, its `pragma_database_list` check at each task claim, and
   immediately before the Step 4.1b CLI fallback. A Ship-side failure halts
   with `SERVED_ROOT_ATTESTATION_FAILED` (Amendment 1, PR #474 review).
2. **Explicit-feature reconcile contract (`B83081F5`).** shipment-reconcile pre-close and
   safe-close accept an `active` explicit feature member when at least one explicit task
   member exists and every explicit task member is `matched` or `pre-archived`. Governed
   `ShipShipment` completes and archives that feature itself. Every other feature state
   still halts.

Source decision: `docs/decisions/2026-10-03-195s-dispatch-closure-contract-deliberation.md`
(options P1-A and P2-A). Stash sources: `227A2930`, `B83081F5`.

## Problem Frame

### Served-root handoff

`.github/agents/_ship.agent.md` Step 4.0 item 4 requires both served roots before wave
admission:

* Ship reads raw item logs from `<served storage root>\logs\<id>.jsonl`.
* Ship uses the served workspace root as the CLI `--cwd` fallback (Step 4.1b).
* Ship must never infer either root from its worktree.
* If either root is unknown, Ship halts with `WAVE_CLAIM_STATE_INDETERMINATE` and an
  Orchestrator follow-up.

The Orchestrator invokes Ship from three places, and none of them passes the roots:

* **Step 2 "Route to Ship" item 5.** Normal, `ship next`/`ship {id}`, and dark-mode cursor
  advance, which returns to Step 2.
* **Step 0.0b item 6.** Crash-resumption routing for `agent: ship` checkpoints.
* **The Dark Factory "Route blocked shipments to Ship" rule.** Unblock and normalize
  recovery.

Resumed and recovered shipments already have `active` claim-assigned members, so every one
of these paths fails closed at Ship Step 4.0.

**Amendment 1 (PR #474 review thread `PRRT_kwDORzozKM6olYEw`).** Content comparisons alone
do not bind candidate roots to the MCP server:

* The server stays bound to its startup `RootPath` (`internal/cli/root.go:412-417`,
  `internal/mcp/server.go:60-76`).
* A second workspace copied together with its gitignored `logs` directory reproduces the
  manifest, index, and log-tail content that the original static and dynamic checks
  compared.
* Ship checks path containment but never re-attests the MCP root (`_ship.agent.md` Step 4.0
  item 4 and Step 4.1b). Once the stores diverge, MCP writes could land in one workspace
  while raw reads or the CLI fallback target another.

Two existing read-only MCP surfaces already report the server's bound roots, so no new MCP
surface or production Go change is needed:

* `backlogit_get_metadata_catalog` returns `workspace.root_path` (the server's `ws.RootPath`)
  and `workspace.storage_root` (`internal/core/metadata_catalog.go:125-126`). These are
  computed the same way the server's write paths are (`WorkspaceStorageRoot`,
  `WorkspaceLogsRoot`).
* `backlogit_query_sql` accepts `SELECT name, file FROM pragma_database_list WHERE name =
  'main'` through its read-only gate. It passes because the `\bPRAGMA\b` forbidden pattern
  has no word boundary inside `pragma_database_list`, not because of the `allowedPragmas`
  list in `internal/db/gate.go`. That makes the behavior incidental, so U7 also asserts the
  gate verdict directly. The result is the absolute path of the index file the live server connection actually
  opened (`internal/core/workspace.go:155`, `<storage root>/backlogit.db`). This was checked
  at staging against the served store.

Nothing pins either surface today, so U7 adds a characterization test.

### Explicit-feature reconcile

Two governed Go operations move an explicit feature member through its lifecycle:

* `ClaimShipment` moves an explicit feature member to `active`
  (`internal/core/shipment_claim_flat_scope_test.go:84`).
* `ShipShipment` sets every non-archived explicit member, including an explicit feature, to
  `done` inside its locked transaction, then archives it.
  * Source: `internal/core/shipment_lifecycle.go:711`, `completeReleaseScope`, reason
    `"shipment released"`.
  * The later `featureScopeRoots` → `setArtifactStatus(..., "feature released")` call at
    line 646 is then a no-op, because the status is already `done` (line 980).

Two texts conflict with this lifecycle:

* **shipment-reconcile pre-mode** (`expected_status: done`) classifies the still-`active`
  feature as `status-mismatch`.
* **Safe-close step 2** requires "every non-pre-archived explicit member to be `done`".
  Ship Step 6 item a repeats this expectation.

The result was `RECONCILE_FAIL` for 195-S
(`.backlogit/reconcile/195-S-pre-20261003T051644Z.md`). Closure then needed a cross-role
Stage feature move.

## Scope

In scope (nine units, one covering feature, one shipment):

* Three RED harness-text contract tests (`tests/integration/`): Orchestrator handoff (U1),
  shipment-reconcile (U3), and Ship use-time attestation (U8).
* Two Go characterization tests: the ShipShipment release lifecycle (`internal/core/`, U4)
  and the MCP served-root self-report surfaces (`internal/mcp/`, U7).
* Installed-text edits:
  * `_orchestrator.agent.md`: the procedure, including the Served-Root Attestation, plus
    three call sites (U2).
  * `shipment-reconcile/SKILL.md` (U5).
  * One sentence in `_ship.agent.md` Step 6 item a, the consumer-side wording of the reconcile
    gate (U5).
  * `_ship.agent.md` Step 4.0 item 4 and Step 4.1b: Ship's use-time Served-Root Attestation
    (U9).
* Harness-manifest drift records for the three edited installed artifacts (U6).

Out of scope:

* Any production Go change. If U4 or U7 fails RED, Ship records a P-021
  `DEFERRED SCOPE EXPANSION` and returns the shipment to Stage.
* The remaining Ship-side served-root intake work, tracked in Stage follow-up stash
  `CDBCB258` (see Decisions):
  * operator-supplied roots for direct Ship invocation
  * the Orchestrator's mapping of Ship's served-root halts
  * Ship-side payload key pinning
  * attestation of the Ship backlogit invocations outside Step 4.0 and Step 4.1b, plus the
    P-002.2 and P-002.6 policy mirror for `SERVED_ROOT_ATTESTATION_FAILED`
* A new MCP tool or field that reports served roots (deliberation option P1-B as originally
  framed). Amendment 1 uses the existing `backlogit_get_metadata_catalog` workspace fields and
  `pragma_database_list` instead.
* The Step 1.5 continuity allowlist (`41FE00A1`).
* The model-routing re-render (`731CE551`).
* Docs compaction (`359D8F32`).
* Upstream autoharness template changes.

## Requirements Trace

| Req | Requirement | Units |
|---|---|---|
| R1 | Workspace root is the canonical absolute `git rev-parse --show-toplevel` of the Orchestrator's own checkout. That checkout must be the main worktree (`--git-common-dir` equals `--git-dir`, both `<root>/.git`). Never inferred from a Ship worktree | U1, U2 |
| R2 | Storage root is exactly one of `.backlog` or `.backlogit` under the workspace root. If neither or both exist, fail closed | U1, U2 |
| R3 | Both roots are canonical OS-native absolute paths with no symlink or reparse-point component. The storage root is a direct child of the workspace root | U1, U2 |
| R4 | Binding proof against the served store, read-only except at most one derived-index `backlogit_sync_index`. (a) `shipment_id` matches the shipment ID pattern. (b) **Served-Root Attestation**, run before any raw read: MCP `backlogit_get_metadata_catalog` `workspace.root_path` and `workspace.storage_root` equal the candidate served workspace and storage roots, and MCP `backlogit_query_sql` `SELECT name, file FROM pragma_database_list WHERE name = 'main'` returns exactly one row whose `file` is `backlogit.db` as a direct child of the candidate served storage root. All comparisons use canonical OS-native absolute form. (c) The shipment manifest exists at exactly one of `<storage>/queue/<id>.md` or `<storage>/archive/<id>.md`, and the manifest and its parent directory are direct children with no symlink or reparse-point component. (d) Static: MCP `backlogit_get_shipment` and the on-disk frontmatter agree on `id`, `status`, `updated_at`, and ordered `custom_fields.items` (Amendment 1 replaces the earlier item-log-tail comparison with (b)) | U1, U2, U7 |
| R5 | Any failure, error, or indeterminate result in R1–R4 halts with `SERVED_ROOTS_UNRESOLVED: {reason}`. Indeterminate includes an unavailable MCP surface, a missing, empty, or relative attested value, a `pragma_database_list` result with other than one `main` row, a static mismatch that persists after the one sync, and a sync error. Ship is not invoked, and reasons use workspace-relative or redacted path tokens | U1, U2 |
| R6 | Ship receives `served_workspace_root` and `served_storage_root` as canonical OS-native absolute paths, plus the binding evidence, in the invocation payload. The evidence is informational: Ship never treats it as proof (R14) | U1, U2 |
| R7 | Every Orchestrator invocation of Ship runs the procedure first: Step 2 item 5, Step 0.0b item 6 `agent: ship` (including a shipment already archived by a crashed closure), blocked-shipment routing, and an operator `ship {id}` naming a non-queued shipment | U1, U2 |
| R8 | Verification outcomes go into tracked `docs/memory/` only, in workspace-relative form. The Orchestrator writes no checkpoint, because a valid checkpoint `agent` is `stage` or `ship` and an Orchestrator-written one would become a recovery candidate | U1, U2 |
| R9 | Pre-close classifies an `active` explicit feature member as `feature-pending-governed-completion`. This requires at least one explicit task member and every explicit task member `matched` or `pre-archived`. Any other explicit feature status remains `status-mismatch` | U3, U5 |
| R10 | Safe-close re-checks R9 on the state it re-reads under lock. Its existing step 5 still requires every non-baseline-archived member to appear in `A`, and post-close still requires a valid archive record | U3, U5 |
| R11 | Ship Step 6 item a describes the gate in terms of the skill's classifications, not "every manifest item `done`" | U3, U5 |
| R12 | The governed lifecycle accepted by R9/R10 is pinned. `ShipShipment` completes (`done`) and archives an `active` explicit feature whose explicit tasks are already `done` and archived | U4 |
| R13 | Manifest drift records name all local edits, including the Ship use-time attestation | U6 |
| R14 | Ship independently runs the Served-Root Attestation (R4b) on the supplied roots once per wave admission, before any raw item-log read, and immediately before any CLI fallback that passes the served workspace root. It repeats the `pragma_database_list` check alone at each task claim, before that task's first raw item-log read. It is MCP-only, with no sync or retry. Any failure, error, or indeterminate result halts with `SERVED_ROOT_ATTESTATION_FAILED: {reason}` before any claim, raw read, or CLI fallback. Ship backlogit invocations outside Step 4.0 and Step 4.1b are follow-up scope (`CDBCB258`) | U8, U9 |
| R15 | The two self-report surfaces are pinned by a characterization test: the metadata catalog's `workspace.root_path` and `workspace.storage_root`, and `pragma_database_list` through the `backlogit_query_sql` read-only gate | U7 |

## Implementation Units

### U1: Orchestrator served-root handoff contract test (RED)

* **Domain:** tests. **Posture:** test-first. RED is the expected deliverable before U2.
* **File:** `tests/integration/orchestrator_served_root_handoff_contract_test.go` (new).
  * Package `integration_test`, no build tag.
  * Use the shared `testRepoRoot`, `testify/require`, and table-driven `t.Run` subtests.
  * Keep helpers as local closures, because package-level helper names may collide. Follow
    the pattern of `tests/integration/claim_start_admission_contract_test.go`.
* **Change:** add `TestOrchestratorServedRootHandoffContract`.
  * Read `.github/agents/_orchestrator.agent.md` and normalize whitespace.
  * Slice by substring index on the normalized text, with explicit unique start and end
    anchors. The existing `sliceSection` `^anchor` helper is unusable here, because bullet
    and mid-line anchors do not start a line.
  * The Step 2 slice runs from `### Step 2: Route to Ship` to `### Step 3`.
  * Scenario 1 tokens are stable contract tokens: a future tune must preserve them or change
    this test together with the contract.
* **Scenarios (3):**
  1. **Procedure literals.** The Step 2 slice contains all of these:
     * `#### Served-Root Handoff Procedure`
     * `Every Orchestrator invocation of the Ship subagent runs the Served-Root Handoff Procedure first`
     * `handoff precondition`
     * `` `served_workspace_root` ``
     * `` `served_storage_root` ``
     * `` `git rev-parse --show-toplevel` ``
     * `--git-common-dir`
     * `main worktree`
     * ``exactly one of `.backlog` or `.backlogit` ``
     * `neither or both exist, fail closed`
     * `symlink or reparse-point`
     * `direct child of the served workspace root`
     * ``exactly one of `queue` or `archive` ``
     * `` `backlogit_get_shipment` ``
     * `` `custom_fields.items` ``
     * `Served-Root Attestation`
     * `` `backlogit_get_metadata_catalog` ``
     * `` `workspace.root_path` ``
     * `` `workspace.storage_root` ``
     * `` `pragma_database_list` ``
     * `before any raw read`
     * `exactly one row`
     * ``` `backlogit.db` as a direct child ```
     * `missing, empty, or relative`
     * `No sync or retry applies`
     * `` `SERVED_ROOTS_UNRESOLVED` ``
     * `any failure, error, or indeterminate result`
     * `do not invoke Ship`
     * `read-only except at most one derived-index`
     * `workspace-relative form only`
     * `Never infer either root from a Ship worktree`
  2. **Every call site references the procedure.**
     * In the Step 2 slice, `Run the Served-Root Handoff Procedure` occurs after
       `5. Invoke the **Ship** subagent:` and before
       ``Pass the `shipment_id` as the session scope``.
     * In the Step 2 slice, `` `ship {id}` that names a non-queued shipment `` occurs.
     * The slice from `` `agent: ship` → invoke the **Ship** subagent `` to
       `The Orchestrator MUST NEVER execute` contains `Served-Root Handoff Procedure`.
     * The slice from `Route blocked shipments to Ship` to
       `never performs those lifecycle mutations itself` contains
       `Served-Root Handoff Procedure`.
  3. **Permanent cross-reference invariant.** Labeled in a comment as a stable contract, so
     later tunes may restructure surrounding text. `5. Invoke the **Ship** subagent:` and
     `proceed to step 4/5` remain present.
* **Exit:**
  * RED on current `main` (scenarios 1 and 2), GREEN after U2.
  * `gofmt -l`, `go vet ./...`, and `golangci-lint run` are clean for the file.
* **Size:** 1 file, 1 test function with closures, 3 scenarios.

### U2: Orchestrator Served-Root Handoff Procedure and call sites

* **Domain:** docs (installed agent text). **Posture:** test-first (U1), with U7 pinning the
  self-report surfaces the attestation relies on. Safety mode: careful.
* **File:** `.github/agents/_orchestrator.agent.md`.
* **Change 1, new subsection.** Add `#### Served-Root Handoff Procedure (all Ship invocations)`
  at the end of Step 2, immediately before `### Step 3`. Do not renumber Step 2.
  * State: "Every Orchestrator invocation of the Ship subagent runs the Served-Root Handoff
    Procedure first."
  * State that it is a handoff precondition: it never reads a checkpoint state dump and
    never performs restore, prune, or resolve work.
  * State that the procedure is read-only except at most one derived-index
    `backlogit_sync_index`. It changes no source or backlog store file and writes no
    checkpoint. Its only other write is the R8 outcome record in `docs/memory/`.
  * Then give these steps:
    * a. **Workspace root.** The canonical absolute output of `git rev-parse --show-toplevel`
      in the Orchestrator's own checkout. The checkout must be the main worktree:
      `git rev-parse --path-format=absolute --git-common-dir` equals
      `git rev-parse --path-format=absolute --git-dir`, and both resolve to `<root>/.git`.
      Never infer either root from a Ship worktree.
    * b. **Storage root.** The served storage root is exactly one of `.backlog` or
      `.backlogit`, existing as a directory under the served workspace root. If neither or
      both exist, fail closed. U2 uses U1 literals verbatim, including capitalization.
    * c. **Canonicalize.** Resolve both roots into canonical OS-native absolute form. On
      Windows, git prints forward slashes, so compare drive letters and components
      case-insensitively. Reject any symlink or reparse-point component. Require the
      served storage root to be a direct child of the served workspace root.
    * d. **Bind.**
      * Require `shipment_id` to match `^[0-9]+-S$`.
      * **Served-Root Attestation, before any raw read.** Ask the MCP server which roots it
        is bound to, and compare them with the candidate roots from a–c:
        * Call `backlogit_get_metadata_catalog` and read only its `workspace` object.
          `workspace.root_path` must equal the served workspace root, and
          `workspace.storage_root` must equal the served storage root.
        * Call `backlogit_query_sql` with
          `SELECT name, file FROM pragma_database_list WHERE name = 'main'`. It must return
          exactly one row, and its `file` must be `backlogit.db` as a direct child of the
          served storage root. This is the index file the live server connection actually
          opened, so it also catches a catalog value that was re-resolved after server start.
        * Compare canonical OS-native absolute forms, using the same rules as step c.
          A missing, empty, or relative value, a row count other than one, or any mismatch
          fails the attestation.
        * The attestation is MCP-only. A CLI invocation reports the root it was pointed at,
          so it cannot attest the server. No sync or retry applies, because a sync cannot
          change the root a server is bound to.
      * The shipment manifest must exist in exactly one of `queue` or `archive` under the
        served storage root, so a shipment archived by a crashed closure still binds. If it
        exists in neither or both, fail closed. This assumes the default archive directory;
        a manifest that exists only under a configured non-default archive directory is
        found in neither and fails closed.
      * Require the manifest and its directory to be direct children with no symlink or
        reparse-point component. Apply Ship Step 4.1b's raw-read safety: no-follow open,
        opened-path verification, and a scoped P-012 raw-read declaration.
      * Read only the manifest frontmatter.
      * Static check: `id`, `status`, `updated_at`, and the ordered `custom_fields.items`
        equal the MCP `backlogit_get_shipment` result. On a mismatch, call
        `backlogit_sync_index` once and compare again. A mismatch that persists after the
        sync is a failure, and a sync error is an indeterminate result.
    * e. **Halt.** On any failure, error, or indeterminate result in a–d, halt with
      `SERVED_ROOTS_UNRESOLVED: {reason}` and do not invoke Ship. Indeterminate results
      include detection that is unavailable, an MCP or sync error, a timeout, and a missing
      or unparseable file. Write `{reason}` and the halt trace with workspace-relative or
      redacted path tokens. When only the CLI fallback is reachable, the Served-Root
      Attestation and the static check cannot run, and the procedure intentionally fails
      closed.
    * f. **Pass.** Put `served_workspace_root` (Ship's "served workspace root", Step 4.1b)
      and `served_storage_root` (Ship's "served storage root") in the Ship invocation
      payload as canonical OS-native absolute paths. Include the binding evidence: `id`,
      ordered items, and the attested `workspace.root_path`, `workspace.storage_root`, and
      index `file`. The evidence is informational. Ship re-runs the Served-Root Attestation
      itself at use time (U9) and never treats the evidence as proof. Record the outcome in
      `docs/memory/` in workspace-relative form only.
* **Change 2, Step 2 item 5.** Make the first bullet under
  `5. Invoke the **Ship** subagent:` read "Run the Served-Root Handoff Procedure", placed
  before the existing ``Pass the `shipment_id` as the session scope`` bullet. Amend that
  bullet to also pass both served roots and the binding evidence. Add to the first bullet:
  "An operator `ship {id}` that names a non-queued shipment also runs the procedure before
  Ship is invoked, or halts to the operator."
* **Change 3, Step 0.0b item 6.** In the `agent: ship` branch, run the Served-Root Handoff
  Procedure for the checkpoint's shipment before invoking Ship. Use the checkpoint
  summary's shipment ID only; do not read the state dump. If the summary names no shipment,
  halt to the operator.
* **Change 4, Dark Factory blocked-shipment routing sentence.** Route blocked shipments to
  Ship only after the Served-Root Handoff Procedure passes for that shipment.
* **Exit:**
  * U1 is GREEN and markdownlint is clean.
  * Manual dry-run: the Ship author runs a–d read-only against the main checkout for one
    queued shipment and one archived shipment (for example `195-S`), and records the pass or
    fail of each step in the PR body, in workspace-relative form only. The dry-run also
    records one negative attestation: a deliberately wrong candidate root, such as the
    parent directory of the served workspace root, must fail step d's Served-Root
    Attestation without any write.
* **Size:** 1 file, 4 localized edits.

### U3: shipment-reconcile explicit-feature contract test (RED)

* **Domain:** tests. **Posture:** test-first. RED is the expected deliverable before U5.
* **File:** `tests/integration/shipment_reconcile_feature_member_contract_test.go` (new).
  Same package, helper, and testify conventions as U1.
* **Change:** add `TestShipmentReconcileExplicitFeatureMemberContract`.
  * It reads `.github/skills/shipment-reconcile/SKILL.md` and `.github/agents/_ship.agent.md`
    and normalizes whitespace.
  * It slices by substring index with explicit unique start and end anchors, as in U1.
* **Scenarios (3):**
  1. **Skill literals.** `SKILL.md` contains all of these:
     * `` `feature-pending-governed-completion` ``
     * ``every explicit task member is `matched` or `pre-archived` ``
     * ``applies only to pre-close (`expected_status: done`)``
     * `A manifest with no explicit task member does not qualify`
     * ``Any other explicit feature status remains `status-mismatch` ``
     * `Safe-close re-checks this condition on the state re-read under lock`
     * ``Explicit feature members are completed only by governed ShipShipment (`backlogit_ship_shipment` or its registered CLI fallback)``
  2. **PROCEED and Ship consumer wording.**
     * The `PROCEED` bullet, from `` * `PROCEED`: `` (with the colon, which occurs once) to
       `` * `PAUSED ``, contains
       ``every explicit member is `matched`, `pre-archived`, or `feature-pending-governed-completion` ``.
     * The `_ship.agent.md` Step 6 item a slice, from `Pre-archive reconciliation gate` to
       ``b. Call `backlogit_ship_shipment` with the merge commit SHA``, contains
       `feature-pending-governed-completion`.
  3. **Superseded text and preserved invariants.**
     * The superseded sentence ``require every non-pre-archived explicit member to be `done` ``
       is absent.
     * The post-mode row `Archive record exists with valid provenance` is still present.
     * The safe-close step 5 rule ``require every member/control record that was not already validly archived at baseline to appear in `A` ``
       is still present.
* **Exit:**
  * RED on current `main` (scenarios 1 and 2, plus the absence check in scenario 3), GREEN
    after U5.
  * `gofmt`, `go vet`, and `golangci-lint` are clean.
* **Size:** 1 file, 1 test function, 3 scenarios.

### U4: ShipShipment explicit-feature release characterization test

* **Domain:** tests (Go core). **Posture:** characterization-first. Expected GREEN on current
  code.
* **File:** `internal/core/shipment_explicit_feature_release_test.go` (new, package `core`).
* **Change:** add `TestShipShipment_ActiveExplicitFeatureWithPreArchivedTasksIsReleased`
  using the fixtures of `TestShipShipment_FeatureInclusiveManifestArchivesFeature`
  (`internal/core/shipment_test.go:304`): `setupShipmentWorkspace`, `CreateArtifact`,
  `WithParent`, `bldb.UpsertItem`, `CreateShipment`, `ClaimShipment`, `loadArtifact`.
  * Create a feature and two child tasks, all explicit members, and claim the shipment.
  * For each task, call `UpdateArtifact(ctx, ws, id, map[string]any{"status": "done"})`,
    then `ArchiveItem(ctx, ws.DB, ws, id)`. `setupShipmentWorkspace` already disables the
    exec gate (`shipment_test.go:38`), so no gate-evidence seeding is needed.
  * Use table-driven `t.Run` subtests over a shared arranged fixture, with each scenario as
    one subtest.
* **Preconditions, asserted with `require` before acting:**
  * each task is `archived` with `archived_status: done`
  * the feature is `active`
* **Scenarios (2):**
  1. **Completion and archival.** `ShipShipment` succeeds.
     * `ArchivedIDs` contains the feature ID and the shipment ID, and contains neither task ID.
     * The reloaded feature is `archived` with `archived_status: done`.
     * Both tasks remain `archived`.
  2. **Governed status transition.** Read the durable item log the way
     `flatScopeStatusReasonsSince` does (`internal/core/shipment_flat_scope_harness_helpers_test.go:33`).
     That helper returns reasons only, so read the entries inline (or add a sibling helper
     in the new file) to assert both `Delta["to"]` and the reason.
     * It contains a `status_changed` event to `done` for the feature.
     * The reason is either `shipment released` (`completeReleaseScope`, the current path)
       or `feature released`.
     * No ordering claim is made.
* **Exit:**
  * GREEN on current code.
  * If either the precondition or a scenario cannot be satisfied, record a P-021
    `DEFERRED SCOPE EXPANSION` with the failing assertion, halt the wave, and return to
    Stage. Do not change production Go.
  * `gofmt`, `go vet`, and `golangci-lint` are clean.
* **Size:** 1 file, 1 test function, 2 scenarios.

### U5: shipment-reconcile contract text and Ship Step 6 wording

* **Domain:** docs (installed skill and agent text). **Posture:** test-first (U3), with U4 as
  lifecycle evidence. Safety mode: careful.
* **Files:** `.github/skills/shipment-reconcile/SKILL.md` and `.github/agents/_ship.agent.md`
  (one sentence only).
* **Change.** Use the U3 sentences verbatim, including capitalization.
  * **Classification table.** Add a `` `feature-pending-governed-completion` `` row.
    * Pre-mode: an explicit feature member is `active` and every explicit task member is
      `matched` or `pre-archived`. The row applies only to pre-close
      (`expected_status: done`).
    * Add: "A manifest with no explicit task member does not qualify."
    * Add: "Any other explicit feature status remains `status-mismatch`."
    * Post-mode: N/A.
  * **Recommendations.** `PROCEED` becomes "every explicit member is `matched`,
    `pre-archived`, or `feature-pending-governed-completion` and the shipment record is
    consistent".
  * **Behavioral Constraints, "Explicit feature handling".** Add "Explicit feature members
    are completed only by governed ShipShipment (`backlogit_ship_shipment` or its
    registered CLI fallback)."
  * **Safe-Close step 2.** Replace the superseded sentence with three rules:
    * every non-pre-archived explicit task member must be `done`
    * an explicit feature member must be `done` or meet the
      `feature-pending-governed-completion` condition
    * "Safe-close re-checks this condition on the state re-read under lock."
  * **Ship Step 6 item a.** Replace "verifies that every manifest item is present in queue
    with `status: done`, and scans for orphan items" with "verifies that the skill's
    pre-close result is `PROCEED`, that is, every explicit member is classified `matched`,
    `pre-archived`, or `feature-pending-governed-completion`".
    * The orphan clause is dropped. It removes no behavior: under flat membership the skill
      never classifies a non-member as an orphan (`SKILL.md` lines 15 and 60), and Ship's
      own item b already says reconciliation must not expect unlisted artifacts to appear as
      orphans.
* **Exit:**
  * U3 is GREEN.
  * `tests/integration/shipment_155_harness_contract_test.go` and
    `tests/integration/claim_start_admission_contract_test.go` stay GREEN.
  * markdownlint is clean.
* **Size:** 2 files, 5 localized edits.

### U6: Harness-manifest drift records for the edited installed artifacts

* **Domain:** config. **Posture:** migration-first (metadata only).
* **File:** `.autoharness/harness-manifest.yaml`.
* **Change:** for the `_orchestrator.agent.md`, `shipment-reconcile/SKILL.md`, and
  `_ship.agent.md` entries:
  * Set `checksum` to the SHA-256 of the installed file as checked out.
  * Append a `drift_reason` sentence that ends "Do not auto-revert." The
    `_orchestrator.agent.md` entry cites `227A2930`, the `shipment-reconcile/SKILL.md` entry
    cites `B83081F5`, and the `_ship.agent.md` entry cites both `B83081F5` (Step 6 item a)
    and `227A2930` (the Step 4.0 and Step 4.1b Served-Root Attestation from U9).
  * Keep `drift_allowed: true`.
* **Constraints:**
  * Touch no other entry.
  * Frontmatter model-routing re-render remains `731CE551`'s job.
  * `plugin/` holds no mirror of these three files (checked at staging), so no parity edit
    is needed.
* **Exit:** each of the three checksums equals its installed hash, the YAML parses, and the
  diff is limited to the three entries. U6 runs after U2, U5, and U9, so the hashes reflect
  the final text.
* **Size:** 1 file, 3 entries.

### U7: MCP served-root self-report characterization test

Added by Amendment 1.

* **Domain:** tests (Go, MCP package). **Posture:** characterization-first. It is expected
  GREEN on arrival, because it pins existing behavior and adds no production code.
* **File:** `internal/mcp/served_root_self_report_test.go` (new).
  * Package `mcp`, the same package as `metadata_parity_test.go`.
  * Reuse `setupCatalogServer`, `testify/require`, and table-driven `t.Run` subtests.
* **Change:** add `TestServedRootSelfReportCharacterization` with one arranged server over a
  temporary workspace whose storage root is `.backlogit`.
* **Scenarios (2):**
  1. **Metadata catalog.** `handleGetMetadataCatalog` succeeds. The decoded JSON
     `workspace.root_path` is absolute and equals the workspace root. `workspace.storage_root`
     is absolute and equals `<root>/.backlogit`, a direct child of `root_path`.
  2. **Index file.** `handleQuerySQL` with
     `SELECT name, file FROM pragma_database_list WHERE name = 'main'` succeeds through the
     read-only gate. It returns exactly one row, and that row's `file` equals
     `<storage_root>/backlogit.db`. The same subtest also asserts that
     `db.ValidateQuery` reports `Allowed` for that exact query, so a future tightening of the
     gate fails here first.
  * Compare paths after `filepath.EvalSymlinks` on both sides, because temporary
    directories can sit behind a symlink on some platforms.
* **Exit:**
  * GREEN: `go test -count=1 -run '^TestServedRootSelfReportCharacterization$' ./internal/mcp`.
  * If either scenario fails, record a P-021 `DEFERRED SCOPE EXPANSION` with the failing
    assertion, halt the wave, and return to Stage. Do not change production Go.
  * `gofmt`, `go vet`, and `golangci-lint` are clean.
* **Size:** 1 file, 1 test function, 2 scenarios.

### U8: Ship served-root attestation contract test (RED)

Added by Amendment 1.

* **Domain:** tests. **Posture:** test-first. RED is the expected deliverable before U9.
* **File:** `tests/integration/ship_served_root_attestation_contract_test.go` (new).
  * Same package, helper, and testify conventions as U1.
  * Slice by substring index on whitespace-normalized text, with explicit unique start and
    end anchors.
* **Change:** add `TestShipServedRootAttestationContract`, which reads
  `.github/agents/_ship.agent.md`. Scenario 1 tokens are stable contract tokens.
* **Scenarios (3):**
  1. **Definition literals.** The slice from `#### Step 4.1b: Claim Task` to
     `#### Step 4.1c` contains all of these, and the first one occurs before
     `**Raw-log path safety (shared read-only procedure):**`:
     * `**Served-Root Attestation (shared read-only procedure):**`
     * `never treats roots or binding evidence passed by the Orchestrator as proof`
     * `` `backlogit_get_metadata_catalog` ``
     * `` `workspace.root_path` ``
     * `` `workspace.storage_root` ``
     * `` `pragma_database_list` ``
     * `exactly one row`
     * ``` `backlogit.db` as a direct child ```
     * `missing, empty, or relative`
     * `No sync or retry applies`
     * `there is no CLI attestation`
     * `once per wave admission, before any raw item-log read`
     * `at each task claim, before that task's first raw item-log read`
     * `immediately before any CLI fallback that passes the served workspace root`
     * `` `SERVED_ROOT_ATTESTATION_FAILED` ``
     * `before any claim, raw item-log read, or CLI fallback`
  2. **Call sites.**
     * The slice from `4. **Classify active members before any other active check.**` to
       `5. **Check for completion.**` contains `Served-Root Attestation` and
       `SERVED_ROOT_ATTESTATION_FAILED`.
     * The slice from `4. If the MCP append errors` to `5. Re-read the item log` contains
       `Served-Root Attestation`, and that occurrence comes before
       `re-reads the item log before any CLI fallback` and before `` `backlogit comment add ``.
  3. **Preserved invariants.** Labeled in a comment as a stable contract. These remain
     present: `**Raw-log path safety (shared read-only procedure):**`,
     `WAVE_CLAIM_STATE_INDETERMINATE`, `Never infer either root`, and
     `re-reads the item log before any CLI fallback`.
* **Exit:**
  * RED on current `main` (scenarios 1 and 2), GREEN after U9.
  * `gofmt -l`, `go vet ./...`, and `golangci-lint run` are clean for the file.
* **Size:** 1 file, 1 test function with closures, 3 scenarios.

### U9: Ship use-time Served-Root Attestation

Added by Amendment 1.

* **Domain:** docs (installed agent text). **Posture:** test-first (U8), with U7 pinning the
  surfaces. Safety mode: careful.
* **File:** `.github/agents/_ship.agent.md` (Step 4.0 item 4 and Step 4.1b only). U9 runs
  after U5 so that the two `_ship.agent.md` edits never run in parallel.
* **Change 1, Step 4.1b definition.** Insert a
  `**Served-Root Attestation (shared read-only procedure):**` paragraph immediately before
  `**Raw-log path safety (shared read-only procedure):**`, using the U8 literals verbatim.
  It says:
  * Ship never treats roots or binding evidence passed by the Orchestrator as proof.
  * Ship calls `backlogit_get_metadata_catalog` and reads only its `workspace` object.
    `workspace.root_path` must equal the canonical served workspace root, and
    `workspace.storage_root` must equal the canonical served storage root.
  * Ship calls `backlogit_query_sql` with
    `SELECT name, file FROM pragma_database_list WHERE name = 'main'`. It must return
    exactly one row whose `file` is `backlogit.db` as a direct child of the canonical served
    storage root.
  * Comparisons use canonical OS-native absolute forms. On Windows, drive letters and
    components compare case-insensitively.
  * The attestation is MCP-only, so there is no CLI attestation: a CLI invocation reports
    the root it was pointed at. No sync or retry applies, because a sync cannot change the
    root a server is bound to.
  * Timing: the full attestation runs once per wave admission, before any raw item-log read
    (Step 4.0 item 4), and immediately before any CLI fallback that passes the served
    workspace root. In addition, Ship repeats the `pragma_database_list` check alone at each
    task claim, before that task's first raw item-log read. This narrows an MCP server
    restart in the middle of a wave to at most one task.
  * Any error, timeout, unavailable tool, missing, empty, or relative value, row count other
    than one, or mismatch halts with `SERVED_ROOT_ATTESTATION_FAILED: {reason}` before any
    claim, raw item-log read, or CLI fallback. Record the halt through P-005 with
    workspace-relative or redacted path tokens, and report an Orchestrator follow-up.
* **Change 2, Step 4.0 item 4.** After "Require both served roots before wave admission.",
  add: once both are known, run the Served-Root Attestation (Step 4.1b) once per wave
  admission, before any raw item-log read. A failure halts with
  `SERVED_ROOT_ATTESTATION_FAILED`. An unknown root still halts with
  `WAVE_CLAIM_STATE_INDETERMINATE`, as today.
* **Change 3, Step 4.1b item 4.** Rewrite the opening of item 4 so that the attestation runs
  right after the MCP error and before the re-read: "4. If the MCP append errors, first run
  the Served-Root Attestation again. If it fails, halt with `SERVED_ROOT_ATTESTATION_FAILED`
  and do not use the CLI fallback. Only then Ship re-reads the item log before any CLI
  fallback." The rest of item 4 stays verbatim. A failed MCP append is the likeliest moment
  for a server to have rebound, so the re-read decision must come after the check.
* **Constraints:**
  * Every phrase pinned by `tests/integration/claim_start_admission_contract_test.go` stays
    verbatim, including the raw-log path-safety contract and
    "re-reads the item log before any CLI fallback".
  * The P-002.6 policy text is not edited. The attestation is Ship's intake control, not a
    wave-scheduling rule.
* **Exit:**
  * U8 is GREEN.
  * `claim_start_admission_contract_test.go`, `shipment_155_harness_contract_test.go`, and
    U3's test (after U5) stay GREEN.
  * markdownlint is clean.
* **Size:** 1 file, 3 localized edits.

## Dependency Graph

```text
Wave 1: U1 (tests)  U7 (tests)  U8 (tests)  U3 (tests)  U4 (tests)
Wave 2: U2 (docs) <- U1, U7                 U5 (docs) <- U3, U4
Wave 3: U9 (docs) <- U8, U7, U5
Wave 4: U6 (config) <- U2, U5, U9
```

* U2 blocks-on U1 and U7.
* U5 blocks-on U3 and U4.
* U9 blocks-on U8, U7, and U5. The U5 edge only serializes edits to `_ship.agent.md`.
* U6 blocks-on U2, U5, and U9.
* The graph has no cycles.

## Decisions and Rationale

* **A named procedure with three call sites.** Ship has three Orchestrator entry points.
  Defining the procedure once in Step 2 and referencing it elsewhere avoids drift, and
  placing it at the end of Step 2 keeps every "step 4/5" cross-reference valid.
* **Authoritative attestation instead of content comparison (Amendment 1, replaces "static
  plus dynamic binding").**
  * The PR #474 review showed that content checks, including the gitignored log tail, can
    be reproduced by a workspace copied together with its `logs` directory. They never ask
    the server which root it is bound to.
  * The Served-Root Attestation asks the server directly through two existing read-only
    surfaces, the metadata catalog's `workspace` fields and the live connection's
    `pragma_database_list`, so a copy cannot pass.
  * It is read-only and needs no new MCP surface or production Go change. U7 pins both
    surfaces, so a refactor that removes them fails CI instead of silently halting dispatch.
  * The static manifest check stays. It proves that the named shipment is present and
    consistent in the attested store.
  * Alternatives considered for the amendment:
    * A freshness-nonce comment written through MCP and read back raw. Rejected: it adds a
      write to every dispatch and wave admission, adds attestation noise to the shipment
      log, and proves the same thing less directly.
    * A new Go MCP field. Rejected: unnecessary, because the self-report already exists.
* **Ship re-attests at use time (Amendment 1).** The Orchestrator's attestation is a
  dispatch-time check. Ship re-runs the full attestation on the supplied roots once per wave
  admission and immediately before the Step 4.1b CLI fallback, and repeats the
  `pragma_database_list` check at each task claim. That bounds, rather than closes, the
  window in which an MCP server restarted against another root would split MCP writes from
  raw reads or CLI writes: at most one task's writes can land before the next check. Other
  Ship backlogit invocations are not covered yet and are listed in follow-up stash
  `CDBCB258`. Ship's Step 4.1b raw-read path safety remains the per-read containment
  control.
* **Stricter than Ship on `.backlog` and `.backlogit`.** Ship's intake tolerates both
  directories existing (`BACKLOGIT_WORKSPACE_DIR` selection). The Orchestrator fails closed
  instead, which is an intentional narrowing that a later tune must keep.
* **A new classification instead of widening `matched`.** The report stays auditable, and
  the meaning of `matched` stays the same for intake and resume.
* **Known gap between the skill and the raw tool.** Raw `backlogit shipment ship` completes
  active explicit tasks
  (`TestShipShipment_FeatureInclusiveManifestArchivesFeature`). The skill guard is
  intentionally stricter. This is documented, not changed.
* **Follow-up stash, created by Stage at harvest (Stage's harvest exit verifies it
  exists): `CDBCB258`.** Served-root intake for direct Ship invocation and the Orchestrator's
  matching Ship-halt mapping. It spans both agents and should:
  * accept operator-supplied roots on direct Ship invocation, checked against R1–R4 and
    attested with the Served-Root Attestation
  * add terminal mappings for `SERVED_ROOTS_UNRESOLVED` and `SERVED_ROOT_ATTESTATION_FAILED`
    to the Orchestrator's Ship-halt handling, covering Ship's "Orchestrator follow-up" halts
  * pin the payload key names on the Ship side
  * extend the use-time attestation to Ship backlogit invocations outside Step 4.0 and
    Step 4.1b: the Step 4.0 item 1 CLI `get` path, the Step 6 governed ShipShipment CLI
    fallback, shipment-reconcile raw reads, and index-sync CLI fallbacks
  * add the P-002.2 halt-taxonomy row and the P-002.6 mirror text for
    `SERVED_ROOT_ATTESTATION_FAILED`

  Ship-side use-time attestation at wave admission, task claim, and the Step 4.1b CLI
  fallback moved into this shipment (U8, U9) under Amendment 1. The rest stays out of scope
  because it edits Ship's direct-invocation intake contract, other Ship steps, or policy.
* **One derived-index sync.** A stale index is the common cause of a static manifest
  mismatch. One sync is the smallest remedy, and a sync error is indeterminate. The sync
  never applies to the Served-Root Attestation, because it cannot change the root a server
  is bound to.
* **Characterization, not a fix, for Go.** A RED U4 or U7 stops the shipment (P-021 C1).

## Exception and Dispatch Preconditions (bootstrap)

* **Dispatch bootstrap.** This shipment is dispatched before U2 lands.
  * The dispatching Orchestrator applies R1–R8 from this reviewed plan only after it records
    explicit operator authorization. A P-017 dark-mode bounded scope naming this shipment
    counts as authorization.
  * The authorization record cites this plan path, the commit that carries Amendment 1 (or a
    later commit of this plan), and the pass result of each step a–f, in workspace-relative
    form only.
  * If authorization or root resolution is missing, the Orchestrator halts with
    `SERVED_ROOTS_UNRESOLVED` and asks the operator.
  * Until U9 is merged to `main` and the served agent text that contains it is the text Ship
    has loaded, Ship applies R14 from this reviewed plan throughout this shipment, under the
    same authorization. The Orchestrator's Ship payload must cite this plan path and R14
    explicitly, because Ship's installed text does not yet tell it to read the plan. A
    failure halts with `SERVED_ROOT_ATTESTATION_FAILED`.
* **Closure (this shipment only).** Before Ship Step 6 runs pre-close for this shipment, Ship
  confirms three things:
  * the local checkout contains the PR merge commit
  * `.github/skills/shipment-reconcile/SKILL.md` contains `feature-pending-governed-completion`
  * the binary that will actually run ShipShipment descends from `47dfcc93` (133-F)
    * Check the MCP binary with `backlogit_get_version`, or the CLI fallback with
      `backlogit version`.
    * Test ancestry with `git merge-base --is-ancestor 47dfcc93 <commit>`.
    * At staging the served binary was `7c805f9`, a descendant.

  If any check fails, Ship halts with the shipment-local token
  `RECONCILE_CONTRACT_NOT_IN_FORCE` instead of asking Stage to move the feature. Failures
  include a missing or dirty commit stamp, a timeout, and any exit code other than 0. Only
  exit 0 passes; exit 1 means "not an ancestor".

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| U4 or U7 is RED, or its precondition cannot be reached | low | high | P-021 halt. U5 depends on U4. U2 and U9 depend on U7 |
| Static manifest mismatch from a stale index | medium | low | One `backlogit_sync_index`, then compare again. If it still differs, fail closed |
| Harness-text tests are skipped by the CI paths filter (`B3701713`) | medium | low | Ship runs `go test -timeout=30m ./...` locally. U7 is a Go package test under `internal/mcp` |
| Manifest checksum mismatch from line endings | low | low | Hash the bytes as checked out |
| A later tune reverts the edits | medium | medium | U6 drift records. Ordering of `731CE551` after this shipment is advisory only |
| Operator-direct Ship invocation still has no root source | medium | low | Follow-up stash `CDBCB258`. The Ship halt remains fail-closed |
| A copied workspace, including gitignored logs, passes content checks while MCP stays bound to another root (PR #474 review) | low | high | Resolved by Amendment 1. The Served-Root Attestation compares the candidate roots with the server's own self-report, at dispatch (U2) and at Ship use time (U9) |
| A future change removes the catalog `workspace` fields, or the read-only gate stops allowing `pragma_database_list` | low | medium | Dispatch fails closed with `SERVED_ROOTS_UNRESOLVED` or `SERVED_ROOT_ATTESTATION_FAILED`. U7 fails CI first |
| The catalog re-resolves its storage root at call time, which could differ from the store the server opened at start | low | medium | The `pragma_database_list` check binds to the index file the live connection opened. Both checks must agree |
| Ship's CLI fallback narrows: when MCP is fully unreachable, the attestation cannot run, so Ship halts instead of falling back | medium | low | Accepted fail-closed trade-off. Without attestation, a CLI write could target a different store than earlier MCP writes. Recovery is an operator decision |
| The catalog response is large (about 65 KB here) | medium | low | Read only the `workspace` object. The attestation runs once per dispatch, once per wave admission, and before a CLI fallback |
| A filesystem alias that canonicalization cannot see, such as a Linux bind mount, makes two paths name one directory | low | low | Accepted residual. Both names then address the same store, so no divergence follows |
| An MCP server restarts against another root in the middle of a task, after that task's claim check | low | medium | Accepted residual, bounded to one task. The next task claim's `pragma_database_list` check, or the next wave admission, halts with `SERVED_ROOT_ATTESTATION_FAILED` |
| Ship backlogit invocations outside Step 4.0 and Step 4.1b are not attested: the Step 4.0 item 1 CLI `get` path, the Step 6 governed ShipShipment CLI fallback, shipment-reconcile raw reads, and index-sync CLI fallbacks | medium | medium | Follow-up stash `CDBCB258`, together with the P-002.2 halt-taxonomy row and the P-002.6 mirror for `SERVED_ROOT_ATTESTATION_FAILED`. Within this shipment, the wave-admission and per-claim checks bound the exposure |

## Constitution Check

| Principle | Verdict | Note |
|---|---|---|
| I. Safety-First Go | pass | Only `_test.go` files are added. Each Go test file must pass `gofmt`, `go vet`, and `golangci-lint` |
| II. Test-First Development (NON-NEGOTIABLE) | pass | U1, U3, and U8 are RED before U2, U5, and U9. U4 provides the evidence for U5, and U7 pins the surfaces U2 and U9 rely on. U4 and U7 are expected GREEN on arrival because they characterize existing behavior and add no production code. Tests use testify and table-driven `t.Run` |
| III. Workspace Isolation and Security Boundaries | pass | The procedure is read-only except one derived-index sync. The Served-Root Attestation binds the roots to the server's own self-report before any raw read, at dispatch and at Ship use time. It checks canonical paths, symlinks and reparse points on every component it reads, the ID pattern, and the main worktree. Tracked records use workspace-relative form only |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | pass | No step writes outside the working tree |
| V. Structured Observability | pass | `SERVED_ROOTS_UNRESOLVED` and `SERVED_ROOT_ATTESTATION_FAILED` are durable and get a redacted halt trace. `RECONCILE_CONTRACT_NOT_IN_FORCE` is local to this shipment's closure |
| VI. Single Responsibility | pass | One procedure, one attestation shared by the Orchestrator and Ship, one classification. Each unit has a single domain |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | pass | Nothing destructive. Rollback is `git revert -m 1` on a branch, merged through a PR, never a force push |
| VIII. Explicit Safety Modes | pass | Careful mode for U2, U5, U9, and the bootstrap dispatch. Freeze-scope covers the nine files named in U1–U9. The declared runtime write targets are `docs/memory/` (outcome records) and the derived index |
| Capability overlay: backlogit | pass | Query-first through MCP. The index is refreshed once before trusting a static mismatch. No task state is kept outside the backlog |
| IX. Git-Friendly Persistence | pass | Markdown and YAML only |
| X. Agent Context Efficiency | pass | The procedure reads manifest frontmatter and only the `workspace` object of the metadata catalog |
| XI. Merge Commit History Preservation (NON-NEGOTIABLE) | pass | Ships through a merge-commit PR |

Constitution Check: pass

## Plan Hardening Signals

* **Public API, schema, or contract change: present.** The agent-to-agent handoff and the
  reconcile classification both change.
* **Security, auth, permission, or compliance-sensitive behavior: present.** Path trust
  boundary and store binding.
* **Migration, backfill, destructive, or irreversible step: absent.**
* **External integration or operator checkpoint: present.** The bootstrap authorization.
* **High runtime, rollout, or rollback risk: absent.** A PR revert restores prior behavior.

Requires plan hardening: yes

## Runtime Verification and Closure

| Unit | Runtime surface | Verification | Closure artifact |
|---|---|---|---|
| U1, U3, U4, U7, U8 | none (tests) | Targeted `go test`, then the full suite | PR test results |
| U2 | Orchestrator dispatch | The first post-merge dispatch records workspace-relative verification and binding evidence, including a passing Served-Root Attestation. Ship passes Step 4.0 without a missing-root halt | Dispatch memory entry |
| U9 | Ship Step 4.0 and Step 4.1b | The first post-merge wave admission records a passing Served-Root Attestation before its first raw item-log read | Ship run report |
| U5 | Ship Step 6 | This shipment's pre-close classifies its feature `feature-pending-governed-completion` and proceeds. Safe-close archives it. Post-close is `matched` | `.backlogit/reconcile/{shipment}-pre/post-*.md` and the closure record |
| U6 | none | The checksums equal the installed hashes | PR diff |

Rollback trigger: a first-use verification fails. Rollback: `git revert -m 1 <merge>` on a
branch, merged through a PR. The 195-S manual workaround remains available under operator
authorization.

## Plan Hardening

**Hardening required: yes.** The triggers are a contract change and a path trust-boundary
validation. There is no destructive or data-migrating action.

### Context consulted

* Learnings Researcher (Step 1.8, and the plan-review persona):
  * `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`:
    explicit features archive only as members. This is the basis for R9, R10, and R12.
  * `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`: gate
    evidence for archived members (U4 fixture).
  * `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`
    and `2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`: the binary check at
    closure.
  * `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md`:
    U1 scenario 3 is labeled as an invariant.
* `_ship.agent.md` Step 4.0 item 4, Step 4.1b, and Step 6 item a.
* `_orchestrator.agent.md`:
  * Step 0.0b item 6
  * Dark Factory blocked routing
  * Step 2, including the dark-mode cursor advance back into Step 2
* 195-S closure and reconcile evidence.
* Amendment 1 (PR #474 review): `internal/mcp/server.go` (`newServer`), `internal/cli/root.go`
  (`openMCPServer`), `internal/core/metadata_catalog.go`, `internal/core/workspace.go`
  (`newWorkspace`, `WorkspaceStorageRoot`), `internal/db/gate.go` (`allowedPragmas`),
  `internal/core/commits.go` (`AppendComment`, for the rejected nonce design), and live
  read-only probes of the catalog `workspace` object and `pragma_database_list` against the
  served store.
* `.github/instructions/constitution.instructions.md`.

### Protected invariants

1. Ship never infers a served root. The Orchestrator supplies both roots, or Ship is not
   invoked. Supplied roots are used only after the MCP server's own self-report attests
   them, at dispatch and again at Ship use time.
2. Root resolution and attestation are read-only, except for at most one index sync, which
   reconciles derived state only, and the R8 outcome record in `docs/memory/`. They write no
   checkpoint and no backlog item log.
3. Shipment membership is flat and explicit. Only explicit members (the explicit feature
   member and explicit task members) are considered. Children are never inferred.
4. Only governed ShipShipment completes or archives an explicit feature member.
5. Post-close requires a valid archive record for every explicit member.
6. No production Go change.

### Risky actions

| ProposedAction | ActionRisk | Approval | Expected ActionResult / rollback state |
|---|---|---|---|
| PA-1: Orchestrator procedure and three call sites (U2) | medium | PR review, after U1 and U7 | Ship is invoked with attested, bound roots, or the run halts with `SERVED_ROOTS_UNRESOLVED`. Revert the PR |
| PA-2: Reconcile acceptance and Ship Step 6 wording (U5) | medium | PR review, after U3 and U4 are GREEN | Pre-close proceeds only for a governed-pending feature. Revert the PR |
| PA-3: Manifest drift records (U6) | low | none | Three entries change. Revert the PR |
| PA-4: Bootstrap dispatch applying the plan | medium | Explicit operator authorization, or a dark-mode scope naming the shipment | Ship receives attested roots, or the dispatch halts |
| PA-5: Ship use-time Served-Root Attestation (U9) | medium | PR review, after U8 and U7 are GREEN | Ship proceeds only on attested roots, or halts with `SERVED_ROOT_ATTESTATION_FAILED` before any claim, raw read, or CLI fallback. Revert the PR |

### Added verification

* U2 manual dry-run in the PR body, in workspace-relative form, including one negative
  attestation.
* U4 preconditions are asserted with `require`.
* U7 pins both self-report surfaces in the Go suite.
* `go test -timeout=30m ./...` runs locally because of `B3701713`.
* Closure checks for this shipment: merge commit present, skill literal present, binary
  ancestry.

### Closure, monitoring, and rollback

* **Signals:**
  * the first post-merge dispatch's verification record
  * this shipment's own pre-close, safe-close, and post-close reports
* **Rollback triggers:**
  * a missing-root `WAVE_CLAIM_STATE_INDETERMINATE` after merge
  * a `SERVED_ROOTS_UNRESOLVED` or `SERVED_ROOT_ATTESTATION_FAILED` halt on a correctly
    bound workspace (a false positive)
  * a feature `status-mismatch` at pre-close
  * a missing feature archive at post-close
* **Owners:** Ship owns first use at this shipment's closure. The Orchestrator owns the
  next dispatch.

### Review-gate capability risks

* plan-review must emit literal `dispatch_mode:` and `decision:` markers in the final
  `## Plan Review` section.
* Degraded dispatch must be declared as `single-agent-declared-degradation`.
* `ADVISORY` requires `operator_authorization: approved`.

### Unresolved operator decisions

* The deliberation outcome was confirmed by autopilot delegation ("Run pipeline") and is
  recorded in the deliberation artifact.
* Bootstrap dispatch requires the explicit operator authorization described above.

## Plan Review

dispatch_mode: multi-agent-dispatch

decision: FAIL

Attempt 1. The gate failed on 5 P1 findings across 7 personas. The plan body above is the
revision that addresses them.

**Personas dispatched:** Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer, and Security
Lens Reviewer.

**Severity totals:** 0 P0, 5 P1, 21 P2, 18 P3.

**P1 findings and dispositions:**

* **GO-1 and LR-2:** a `done` move does not pre-archive a task.
  * U4 now uses `UpdateArtifact` and then `ArchiveItem`.
  * The preconditions are asserted.
  * Gate-evidence seeding is named.
* **LR-1:** closure binary skew.
  * The closure precondition now checks binary ancestry against `47dfcc93`.
  * The served binary at staging was `7c805f9`, a descendant.
* **A1 and ANP-1:** Ship has three Orchestrator entry points.
  * There is now a named procedure with three call sites (R7).
  * U1 scenario 2 pins each site.
* **SEC-1:** the static binding could not tell apart a clone at the same commit.
  * A dynamic `item_log_entries` versus gitignored log-tail binding was added (R4c).
  * `status` and `updated_at` were added to the static comparison.

**P2 findings and dispositions:**

* **CR-1:** added a full constitution table.
* **CR-2:** declared safety modes.
* **CR-3:** added quality gates to the exit criteria.
* **CR-4 and SEC-3:** added the ID pattern and queue-file containment.
* **CR-5, ANP-3, SEC-4, and SB-1:**
  * added payload-key, worktree, read-only, and workspace-relative literals
  * added R8
* **GO-2 and GO-3:** strengthened the U4 assertions.
* **GO-4:** named `ListItemLogEntries` and dropped the ordering claim.
* **GO-5:** U5 uses the U3 sentences verbatim.
* **SB-2:** U3 pins safe-close step 5.
* **SB-3:** named the exact function, and a precondition failure is treated as RED.
* **A2:** Ship Step 6 item a wording added to U5 (R11), and U3 pins it.
* **A3 and ANP-5:** closure detection token `RECONCILE_CONTRACT_NOT_IN_FORCE`.
* **A4:** bootstrap requires explicit operator authorization.
* **ANP-2 and ANP-4:** Ship-side intake follow-up stash. Binding evidence is passed in the
  payload.
* **SEC-2:** "any failure, error, or indeterminate result".

**P3 findings and dispositions:**

* **CR-6:** testify and `t.Run`.
* **CR-7:** redacted halt trace.
* **CR-8:** the PR body uses workspace-relative paths.
* **CR-9:** revert through a PR.
* **CR-10:** frontmatter-only read.
* **GO-6:** package, closures, and prefix anchors.
* **SB-4:** dropped the duplicate flat-membership pin.
* **SB-5:** moved the dry-run into U2's exit.
* **SB-6:** the statement is now pinned.
* **SB-7:** `731CE551` ordering is advisory.
* **A5:** re-check under lock, and a feature-only manifest does not qualify.
* **A6:** recorded the narrowing.
* **A7:** mapped keys to Ship's prose.
* **ANP-6:** CLI fallback wording.
* **ANP-7:** documented the known gap between the skill and the raw tool.
* **LR-3:** U1 scenario 3 is labeled as an invariant.
* **LR-4:** cited.
* **LR-5:** covered by the canonical absolute-path requirement.
* **LR-6:** the mirror was checked.
* **SEC-5:** the runtime row wording is now workspace-relative.
* **SEC-6:** the bootstrap records each step.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch

decision: FAIL

Attempt 2, a re-review by 7 freshly dispatched personas with full coverage. The gate failed
on 2 P1 findings. The plan body above has been revised again for attempt 3.

**Severity totals:** 0 P0, 2 P1, 11 P2, 13 P3.

**P1 findings and dispositions:**

* **GO2-NEW-1:** `completeReleaseScope` sets the explicit feature `done` with reason
  `shipment released`, so the `feature released` call is a no-op (`shipment_lifecycle.go:711`
  and `:980`).
  * U4 scenario 2 now accepts either reason and reads the durable log.
  * The Problem Frame and R12 are corrected.
* **ANP2-F1, with A2-F2 (P2):** crash-resume after ShipShipment found the shipment archived,
  so a queue-only binding would block recovery.
  * R4 and U2-d now bind against exactly one of `queue` or `archive`.
  * R7 covers the archived case.
  * U1 pins the literal.

**P2 findings and dispositions:**

* **CR2-1, SB2-1, A2-F1, LR-8, and SEC2-P3:** the read-only contradiction is resolved with
  "read-only except at most one derived-index `backlogit_sync_index`". A sync error is
  indeterminate.
* **LR-1 (partial):** the closure check now covers whichever binary runs ShipShipment (MCP or
  CLI), uses `git merge-base --is-ancestor`, and fails closed on a missing or dirty stamp.
* **LR-7:** the dynamic binding compares `event_type` and the parsed UTC instant. A zero
  index timestamp triggers the single sync.
* **SEC2-F1 and LR-9:** zero rows, an empty log, or an unparseable last line is
  indeterminate. "Last line" means the last non-empty complete line.
* **SEC2-F2:** the reparse-point check now covers the manifest, its directory, `logs`, and
  the log file.
* **ANP2-F3:** applies Ship Step 4.1b raw-read safety and a scoped P-012 declaration.
* **ANP2-F2:** an operator `ship {id}` naming a non-queued shipment runs the procedure or
  halts.
* **GO2-P2:** U1 and U3 slice by substring index with unique start and end anchors.

**P3 findings and dispositions:**

* **CR2-2 and SB2-3:** seven files, plus the declared runtime write targets.
* **CR2-3:** U4 uses `t.Run`, and the Check II row explains why it is GREEN on arrival.
* **CR2-4:** added a backlogit overlay row.
* **SB2-2:** scenario 1 tokens are labeled as stable contract tokens.
* **SB2-4:** `RECONCILE_CONTRACT_NOT_IN_FORCE` is local to this shipment.
* **GO2-P3a:** removed the moot gate-evidence fallback.
* **GO2-P3b:** U4 reads the durable log.
* **A2-F3:** Ship Step 6 item a defers to the skill's `PROCEED` and drops the orphan clause.
* **A2-F4:** the procedure is stated as a handoff precondition with no state-dump read.
* **ANP2-F4:** the follow-up stash adds the terminal halt mapping and the key names.
* **SEC2-P3a:** the full-copy residual is accepted in Risks.

<!-- plan-review-attempt: 2 -->

## Plan Review

dispatch_mode: multi-agent-dispatch

decision: ADVISORY

Attempt 3, the final re-review allowed. 7 freshly dispatched background personas gave full
coverage: Constitution, Go, Scope Boundary, Learnings, Architecture, Agent-Native Parity,
and Security Lens. Every attempt-2 finding was confirmed resolved, except partial residue
on SB2-1 and A2-F1, which is fixed below. No P0 or P1 findings remain. The plan body above
now carries the dispositions for the P2 and P3 findings below. Per the plan-review skill, an
ADVISORY needs explicit operator confirmation (`operator_authorization: approved`) before
harvest. Stage ran under autopilot and did not self-authorize.

**Severity totals:** 0 P0, 0 P1, 3 P2, 13 P3.

**P2 findings and dispositions (applied to the body):**

* **A3-N1:** R8 and U2-f wrote outcome records to `.backlogit/checkpoints/`, but the
  Orchestrator has no valid checkpoint `agent` value.
  * R8 and U2-f now write to `docs/memory/` only.
  * U2 states that the procedure writes no checkpoint.
  * Protected invariant 2 and the Check VIII row match.
  * This also resolves CR3-1, SB2-1 (residue), and SEC3-P3b.
* **GO3-1:** the U3 start anchor `` * `PROCEED` `` is not unique (SKILL.md lines 79 and
  146). It is now `` * `PROCEED`: ``, which occurs once.
* **ANP3-N1:** the `ship {id}` non-queued sentence was not pinned. It moved into Step 2
  item 5 (Change 2), which also resolves SB3-1, and U1 scenario 2 now asserts it.

**P3 findings and dispositions (applied to the body):**

* **GO3-2:** U4 scenario 2 reads the entries inline, or through a sibling helper, to assert
  `Delta["to"]`.
* **SB3-2:** the follow-up stash is relabeled to cover both agents, and Stage's harvest
  exit verifies that it exists.
* **SB3-3:** the orphan-clause drop is justified as removing no behavior (SKILL.md lines 15
  and 60).
* **LR3-P3-1:** the newest row is taken in insertion order (`ORDER BY rowid DESC LIMIT 1`).
* **LR3-P3-2:** only exit 0 passes the ancestry check.
* **A3-P3a:** the U2 dry-run includes an archived shipment.
* **A3-P3b:** the default-archive-directory assumption is stated, and a non-default
  directory fails closed.
* **ANP3-N2:** the row's `log_path` must resolve to the log file that was read.
* **ANP3-N3:** CLI-only operation fails closed intentionally, and the plan now says so.
* **SEC3-P3a:** a zero timestamp that persists after the sync is listed in U2-d.

**Residual (not counted):** U4's halt-on-failure exit covers the possibility that
`gateShipmentCompletion` and `VerifyPostShipConsistency` reject explicit members that were
pre-archived (GO3 residual).

operator_authorization: approved

* **Provenance:** the operator replied "Approved" at 2026-10-03T08:00:44Z
  (2026-10-03T01:00:44-07:00), relayed by the Orchestrator. The reply was given after the
  operator was shown this ADVISORY result (0 P0, 0 P1, 3 P2, 13 P3) together with Stage
  checkpoint `checkpoint-20261003-075017.json` as the restart point.
* **Scope:** this authorizes the ADVISORY outcome of attempt 3 for this plan only. Stage
  resumed at Step 5 with `skip_review: true`. The `skip_review` gate validated this final
  section: `dispatch_mode: multi-agent-dispatch`, `decision: ADVISORY`, and
  `operator_authorization: approved`.

## Plan Review

dispatch_mode: multi-agent-dispatch

decision: ADVISORY

Amendment 1 (PR #474 review thread `PRRT_kwDORzozKM6olYEw`). This is a focused delta review
of the amendment only. It supersedes the attempt 3 record above for gate purposes. Two
freshly dispatched personas covered the changed surface: Security Lens and Agent-Native
Parity. Both returned ADVISORY with no P0 or P1 findings. Both confirmed that the
Served-Root Attestation closes the copied-workspace finding. A copy at another path cannot
match the server's own `workspace.root_path`, or the index file named by
`pragma_database_list`, and the server fixes its logs directory from the same storage root
at start.

**Severity totals:** 0 P0, 0 P1, 6 P2, 5 P3, after merging three findings raised by both personas.

**P2 findings and dispositions (applied to the body):**

* **SEC-A1 and ANP-A6 (gap between check and use within a wave):** Ship now repeats the
  `pragma_database_list` check at each task claim. The Decisions bullet says the window is
  bounded, not closed, and a Risks row records the residual, which is limited to one task.
* **SEC-A2 (Ship backlogit invocations outside the attested sites):** R14 names them as
  follow-up scope. A Risks row lists them, and `CDBCB258` now carries them.
* **SEC-A3 (bootstrap condition could switch the check off early):** the bootstrap now
  applies R14 until U9 is merged to `main` and loaded as Ship's served agent text.
* **SEC-A4 and ANP-A3 (stale authorization and superseded dispositions):** this section
  records the amendment's own decision and authorization. The bootstrap record must cite the
  commit that carries Amendment 1.
* **ANP-A1:** U2 step b now carries the lowercase U1 literal, and U2 must use U1 literals
  verbatim, including capitalization.
* **ANP-A2:** U1 and U8 now pin the comparison rules as well as the names: `exactly one row`,
  `` `backlogit.db` as a direct child ``, `missing, empty, or relative`, and
  `No sync or retry applies`.

**P3 findings and dispositions (applied to the body):**

* **SEC-A5 (wrong control cited):** `pragma_database_list` passes the gate because
  `\bPRAGMA\b` does not match inside it, not because of `allowedPragmas`. The Problem Frame
  is corrected, and U7 also asserts `db.ValidateQuery` directly.
* **SEC-A6 and ANP-A4 (attestation after the re-read):** U9 Change 3 now runs the
  attestation right after the MCP error and before the re-read. U8 pins that order.
* **ANP-A5:** U9 Change 1 states `No sync or retry applies`, and U8 pins it.
* **ANP-A7:** the P-002.2 halt-taxonomy row and the P-002.6 mirror are added to `CDBCB258`.
* **ANP-A8:** the bootstrap Ship payload must cite this plan path and R14.

**Superseded earlier dispositions.** These dispositions in the attempt 1–3 records above
described the item-log-tail binding that Amendment 1 removed, and are superseded by R4b and
U2 step d: SEC-1 (R4c), LR-7, SEC2-F1, LR3-P3-1, ANP3-N2, and SEC3-P3a. The historical
records are kept unchanged for audit.

operator_authorization: approved

* **Provenance:** the operator directed "PR 474: Additional Copilot review comments to fix"
  at 2026-10-03T21:07-07:00, relayed by the Orchestrator to Stage. The Orchestrator's
  delegation states that this directive authorizes amending the approved plan and the
  harvested items to address the review finding.
* **Scope:** this authorizes Amendment 1 and its ADVISORY outcome for this plan only. It
  does not authorize the bootstrap dispatch. That still needs the explicit operator
  authorization described under Exception and Dispatch Preconditions.
