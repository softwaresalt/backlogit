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

Discharge the two 195-S `READY_WITH_CONDITIONS` follow-up conditions (`227A2930` and
`B83081F5`), without adding lifecycle authority, production Go changes, or a new MCP
surface. The only exception is the one-time, operator-ruled lifecycle exception that
Amendment 3 (A3.4, Option A) may use; it adds no standing authority. P-001 does not block
this shipment (Amendment 3): the operator ratified it as resolved on evidence. The 195-S
closure PR #472 merged at 2026-10-03T06:46:16Z, and its merge commit
`1ccecd946de3577c31f4018166d1ea05a7faf848` is an ancestor of the 196-S branch. The
closure's `compaction_status: degraded` is non-blocking under P-020, and its "closure PR
readiness open" text is stale. The conditions are:

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
* Harness-manifest drift records for the three edited installed artifacts (U6), guarded by
  the `TestUSR6_` harness that harness-architect scaffolds at wave 4 (Amendment 2).

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

Amended by Amendment 3 (A3.2.1, A3.3, A3.5); Amendment 3 text governs.

* **Domain:** tests. **Posture:** test-first. RED is the expected deliverable before U2.
* **File:** `tests/integration/orchestrator_served_root_handoff_contract_test.go` (new).
  * Package `integration_test`, no build tag.
  * Use the shared `testRepoRoot`, `testify/require`, and table-driven `t.Run` subtests.
  * Keep helpers as local closures, because package-level helper names may collide. Follow
    the pattern of `tests/integration/claim_start_admission_contract_test.go`.
* **Change:** add `TestUSR1_OrchestratorServedRootHandoffContract` (renamed by Amendment 2).
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

Amended by Amendment 3 (A3.2.1 green wording); Amendment 3 text governs.

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

Amended by Amendment 3 (A3.2.2, A3.3, A3.5); Amendment 3 text governs.

* **Domain:** tests. **Posture:** test-first. RED is the expected deliverable before U5.
* **File:** `tests/integration/shipment_reconcile_feature_member_contract_test.go` (new).
  Same package, helper, and testify conventions as U1.
* **Change:** add `TestUSR3_ShipmentReconcileExplicitFeatureMemberContract` (renamed by
  Amendment 2).
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
* **Change:** add `TestUSR4_ShipShipmentActiveExplicitFeatureWithPreArchivedTasksIsReleased`
  (renamed by Amendment 2)
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

Amended by Amendment 3 (A3.2.2 Safe-Close step 2 text); Amendment 3 text governs.

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

* **Domain:** config. **Posture:** test-first, harness-required (Amendment 2); metadata-only
  delta.
* **File:** `.autoharness/harness-manifest.yaml`. Harness (scaffolded by harness-architect at
  wave-4 admission, Amendment 2): `tests/integration/harness_manifest_196_drift_records_test.go`.
* **Change:** for the `_orchestrator.agent.md`, `shipment-reconcile/SKILL.md`, and
  `_ship.agent.md` entries:
  * Set `checksum` to the SHA-256 of the installed file content with CRLF normalized to LF,
    the manifest's existing convention (corrected by Amendment 2).
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
* **Exit:** each of the three checksums equals its LF-normalized installed hash, and the
  recipe output for each file is recorded in the task evidence beside the manifest value. The
  YAML parses, the manifest diff is limited to the three entries, and
  `go test -count=1 -v -run '^TestUSR6_' ./tests/integration` passes (Amendment 2). U6 runs
  after U2, U5, and U9, so the hashes reflect the final text.
* **Size:** 1 manifest file, 3 entries, plus the harness file that harness-architect
  scaffolds.

### U7: MCP served-root self-report characterization test

Added by Amendment 1.

* **Domain:** tests (Go, MCP package). **Posture:** characterization-first. It is expected
  GREEN on arrival, because it pins existing behavior and adds no production code.
* **File:** `internal/mcp/served_root_self_report_test.go` (new).
  * Package `mcp`, the same package as `metadata_parity_test.go`.
  * Reuse `setupCatalogServer`, `testify/require`, and table-driven `t.Run` subtests.
* **Change:** add `TestUSR7_ServedRootSelfReportCharacterization` (renamed by Amendment 2)
  with one arranged server over a
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
  * GREEN: `go test -count=1 -v -run '^TestUSR7_' ./internal/mcp` (Amendment 2 selector).
    U7 is `harness-exempt` class `verification-only`; see Amendment 2.
  * If either scenario fails, record a P-021 `DEFERRED SCOPE EXPANSION` with the failing
    assertion, halt the wave, and return to Stage. Do not change production Go.
  * `gofmt`, `go vet`, and `golangci-lint` are clean.
* **Size:** 1 file, 1 test function, 2 scenarios.

### U8: Ship served-root attestation contract test (RED)

Amended by Amendment 3 (A3.2.3, A3.3, A3.5); Amendment 3 text governs.

Added by Amendment 1.

* **Domain:** tests. **Posture:** test-first. RED is the expected deliverable before U9.
* **File:** `tests/integration/ship_served_root_attestation_contract_test.go` (new).
  * Same package, helper, and testify conventions as U1.
  * Slice by substring index on whitespace-normalized text, with explicit unique start and
    end anchors.
* **Change:** add `TestUSR8_ShipServedRootAttestationContract` (renamed by Amendment 2), which
  reads
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

Amended by Amendment 3 (A3.2.3 green wording); Amendment 3 text governs.

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
| Manifest checksum mismatch from line endings | low | low | Hash LF-normalized content, the manifest's convention (Amendment 2) |
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
| VIII. Explicit Safety Modes | pass | Careful mode for U2, U5, U9, and the bootstrap dispatch. Freeze-scope covers the nine files named in U1–U9, plus the U6 harness file added by Amendment 2. The declared runtime write targets are `docs/memory/` (outcome records) and the derived index |
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
| U6 | none | The checksums equal the LF-normalized installed hashes, and `TestUSR6_` passes (Amendment 2) | PR diff |

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

## Amendment 2: Harness Lifecycle Contract (196-S wave-1 admission halt)

**Trigger.** Ship governed-BLOCKED 196-S at wave-1 admission with
`P-002.6_TASK_SCOPED_COMMAND_CONFLICT`
(`docs/memory/2026-10-04/ship-196-S-p0026-task-command-contract-halt.md`). There were two
contract defects:

1. **P-002 harness satisfaction.** 196.004-T (U4) and 196.007-T (U7) carried neither
   `harness-ready` nor a P-002.1-valid `harness-exempt` contract.
2. **P-002.6 rule 4.** Every task-scoped selector was a full anchored function name, for
   example `^TestShipServedRootAttestationContract$`. The rule requires the task's own
   `^TestU<unit>_` prefix.

**Authority.** P-017 dark mode, scope `[196-S]`, relayed by the Orchestrator on 2026-10-04.
Amendment 2 changes only the harness lifecycle and command contract.

**What does not change:**

* requirements R1–R15
* any unit's Change, Scenarios, or file, except the U6 checksum hashing rule (corrected to
  LF-normalized content) and the test-function renames
* the dependency graph, the waves, and `green_maker_tasks` / `green_maker_closes_wave`

**One exception.** U6 gains one harness test file, scaffolded by harness-architect, as
described under "Why U6 stays harness-required" below.

### Test unit tokens (collision-free)

The literal `^TestU<n>_` selectors are not task-scoped in two of the packages:

* `^TestU4_` in `./internal/core` already matches `TestU4_RefusalLeavesAuditJSONLByteUnchanged`
  and `TestU4_NonConformingAlreadyAbandonedReturnsNonConforming`
  (`internal/core/checkpoint_disposition_test.go`).
* `^TestU7_` in `./internal/mcp` already matches three `TestU7_*` functions
  (`internal/mcp/checkpoint_disposition_test.go`).

Either selector could therefore match functions outside the task, or be satisfied by them.
P-002.6 rules 4 and 5 forbid both. Amendment 2 instead uses the unit token `SR<n>` as the
196-S alias of plan unit `U<n>`, so each prefix is `^TestUSR<n>_`. This follows the
`TestUCS<n>_` precedent from 195-S. `git grep TestUSR` was empty when this was written.
Subtest names are fixed, because the `covered-by` and `verification-only` commands assert
them by name.

| Task | Unit | Test function | Subtests (exact) | Package |
|---|---|---|---|---|
| 196.001-T | U1 | `TestUSR1_OrchestratorServedRootHandoffContract` | `ProcedureLiterals`, `CallSites`, `CrossReferenceInvariant` | `./tests/integration` |
| 196.003-T | U3 | `TestUSR3_ShipmentReconcileExplicitFeatureMemberContract` | `SkillLiterals`, `ProceedAndShipStep6`, `SupersededAndPreserved` | `./tests/integration` |
| 196.004-T | U4 | `TestUSR4_ShipShipmentActiveExplicitFeatureWithPreArchivedTasksIsReleased` | `CompletionAndArchival`, `GovernedStatusTransition` | `./internal/core` |
| 196.007-T | U7 | `TestUSR7_ServedRootSelfReportCharacterization` | `MetadataCatalog`, `IndexFile` | `./internal/mcp` |
| 196.008-T | U8 | `TestUSR8_ShipServedRootAttestationContract` | `DefinitionLiterals`, `CallSites`, `PreservedInvariants` | `./tests/integration` |
| 196.006-T | U6 | `TestUSR6_HarnessManifestDriftRecords` (new, see below) | `OrchestratorAgent`, `ShipmentReconcileSkill`, `ShipAgent` | `./tests/integration` |

Subtest numbering follows each unit's scenario numbering (scenario 1 is the first name).

### Harness lifecycle classification (all nine tasks)

| Task | Wave | Lifecycle | Label now | Task-scoped command |
|---|---|---|---|---|
| 196.001-T U1 | 1 | harness-required, red deliverable; green maker 196.002-T, closes wave 2 (unchanged) | none (harness-architect applies `harness-ready` at wave 1) | `go test -count=1 -v -run '^TestUSR1_' ./tests/integration` |
| 196.003-T U3 | 1 | harness-required, red deliverable; green maker 196.005-T, closes wave 2 (unchanged) | none (as above) | `go test -count=1 -v -run '^TestUSR3_' ./tests/integration` |
| 196.004-T U4 | 1 | `harness-exempt`, `verification-only` | `harness-exempt`, `verification-only` | its `exempt_verification_command` (inner `-run "^TestUSR4_" ./internal/core`) |
| 196.007-T U7 | 1 | `harness-exempt`, `verification-only` | `harness-exempt`, `verification-only` | its `exempt_verification_command` (inner `-run "^TestUSR7_" ./internal/mcp`) |
| 196.008-T U8 | 1 | harness-required, red deliverable; green maker 196.009-T, closes wave 3 (unchanged) | none (as above) | `go test -count=1 -v -run '^TestUSR8_' ./tests/integration` |
| 196.002-T U2 | 2 | `harness-exempt`, `covered-by`, owner 196.001-T | `harness-exempt`, `covered-by` | `harness_owner_command` `go test -count=1 -v -run '^TestUSR1_' ./tests/integration`, driven to its `exempt_verification_command` |
| 196.005-T U5 | 2 | `harness-exempt`, `covered-by`, owner 196.003-T | `harness-exempt`, `covered-by` | `harness_owner_command` `go test -count=1 -v -run '^TestUSR3_' ./tests/integration`, as above |
| 196.009-T U9 | 3 | `harness-exempt`, `covered-by`, owner 196.008-T | `harness-exempt`, `covered-by` | `harness_owner_command` `go test -count=1 -v -run '^TestUSR8_' ./tests/integration`, as above |
| 196.006-T U6 | 4 | harness-required, not a red deliverable | none (harness-architect applies `harness-ready` at wave 4) | `go test -count=1 -v -run '^TestUSR6_' ./tests/integration` |

The exact `exempt_verification_command` strings are bound verbatim in each task's
`harness-exemption-contract` block. Each command does the following:

* It is a read-only `pwsh -NoProfile -Command '...'`. It runs only `Test-Path` and `go test`,
  so it passes the P-002.5 screen.
* It runs the task's anchored selector with an explicit package and `-count=1 -v`.
* It fails on a non-zero exit, on `no tests to run` or `no test files`, on any missing named
  top-level or subtest `--- PASS:` line, or on any `--- FAIL:` or `--- SKIP:` line.
* The `verification-only` commands also fail when the new test file is absent.
* `Write-Output "EXEMPT_VERIFY_OK:<task>"; exit 0` is the last statement.

**Validation at authoring.** All five commands were run on 2026-10-04 at
HEAD `82ee69e8`, before any work:

* 196.004-T and 196.007-T exited 1 because the test file was missing.
* 196.002-T, 196.005-T, and 196.009-T exited 1 because the run matched no tests (vacuous).

So the must-fail precondition was observed. The positive and negative paths of the
196.004-T command were also exercised in a throwaway module outside the repository:

* named passing subtests printed `EXEMPT_VERIFY_OK:196.004-T` and exited 0;
* a renamed subtest exited 1.

### Closed harness-exempt set

The closed harness-exempt set for 196-F / 196-S is exactly:

**{196.002-T, 196.004-T, 196.005-T, 196.007-T, 196.009-T}**

| Task | Class | Owner |
|---|---|---|
| 196.002-T | `covered-by` | 196.001-T |
| 196.004-T | `verification-only` | none |
| 196.005-T | `covered-by` | 196.003-T |
| 196.007-T | `verification-only` | none |
| 196.009-T | `covered-by` | 196.008-T |

No other task in this release unit may claim `harness-exempt`. Adding one requires another
reviewed amendment. The 196-F feature contract does not enumerate the harness lifecycle,
so this plan is the governing contract (P-002.1).

### Why U4 and U7 are `verification-only`

U4 and U7 are characterization tests. They pin behavior that has already shipped:

* U4 pins `completeReleaseScope`.
* U7 pins the metadata catalog's workspace roots and the read-only gate's handling of
  `pragma_database_list`.

Both are GREEN on arrival by design, so neither can be a red-first harness. Each commits
one green guard and adds zero non-test Go (P-002.1 `verification-only`; P-002.4 surface:
one new `*_test.go`).

The must-fail precondition is met because the file is absent before the work.

The P-021 halt semantics are preserved. A failing precondition or scenario fails the
command. Ship then records a `DEFERRED SCOPE EXPANSION`, halts the wave, and returns to
Stage. Ship never changes production Go to make it pass.

### Why U2, U5, and U9 are `covered-by` (beyond the halted wave)

The green makers carried no harness label either, so wave 2 and wave 3 would have halted
the same way. P-002.6 rule 4 cannot be met with a green maker's own prefix, because a
docs-only edit has no test of its own.

The established route is the 195-S precedent (195.003/004/005-T): `covered-by` the red
deliverable it turns green. Each owner meets the `covered-by` owner conditions:

* it is in the same release unit;
* it is a declared dependency of the green maker;
* it is not itself exempt.

Each delta touches only installed agent or skill text and no `*_test.go`. These are
behavior-changing harness-text edits, so `covered-by` is the correct P-002.4 class.

### Why U6 stays harness-required

`.autoharness/harness-manifest.yaml` is repository configuration, and no exempt class
admits it:

* `docs-only` admits only markdown, instruction, prompt, and agent artifacts (P-002.4).
* `verification-only` bars repository configuration and commits only guards or evidence.
* `covered-by` needs a red harness owner in the release unit, and none of U1, U3, or U8
  asserts over the manifest. The earlier U6 text said "no new test" and gave a `Get-FileHash`
check, which would have halted wave 4 under P-002.6 rule 4.

Following the 174.076-T precedent (`TestU19R3_`), harness-architect scaffolds
`tests/integration/harness_manifest_196_drift_records_test.go` at wave-4 admission. It
contains `TestUSR6_HarnessManifestDriftRecords` with three subtests. Each subtest selects
one entry by exact `path` and asserts:

* exactly one entry has that path;
* `drift_allowed: true`;
* the checksum is 64 lowercase hex characters and differs from the pinned
  pre-reconciliation value;
* `drift_reason` contains `Do not auto-revert.` and the stash citation(s) from U6 AC2.

The test is RED at staging, because no entry cites `227A2930` or `B83081F5`. It does
**not** recompute file hashes, because a permanent currency pin would turn every later edit
of these files into a failure, which is outside 196-S. Checksum currency stays a one-time
U6 check.

U6 also corrects its hashing rule. It hashes CRLF-to-LF normalized content, which is the
manifest's existing convention: the current `_orchestrator.agent.md` and
`shipment-reconcile/SKILL.md` checksums match LF-normalized content, not CRLF checkout bytes.
Hashing raw checkout bytes on a Windows `core.autocrlf=true` checkout would record a
non-canonical value.

### P-002.6 scoped-command conformance

| Rule | How every command meets it |
|---|---|
| 1. Executable as written | The exact strings are bound in the task contracts and were run at authoring |
| 2. Explicit package | `./tests/integration`, `./internal/core`, or `./internal/mcp`; never `./...` |
| 3. `-count=1` | Present in every selector and every inner `go test` |
| 4. Anchored own prefix | `^TestUSR<n>_` with a collision-free token. A `covered-by` task runs its owner's anchored selector (build-feature `harness_cmd` rule) |
| 5. Fails closed on a vacuous pass | The exempt commands reject `no tests to run`, `no test files`, and missing named `--- PASS:` lines. A red selector is expected to fail until its green maker lands, and its green-closing check is the green maker's non-vacuous `covered-by` command |
| 6. No weakening device | No `-short`, build tag, `t.Skip`, `\|\| true`, or narrowed selector |

### Preserved invariants and resume notes

* 196.001-T, 196.003-T, and 196.008-T stay red-path tasks for harness-architect. No
  `harness-ready` is added now. Their red-deliverable contracts keep the same
  `green_maker_tasks` and `green_maker_closes_wave` values (2, 2, 3); only
  `red_selector_command` changed.
* Wave membership, the DAG, and every task's status are unchanged. Stage did not touch the
  shipment block envelope, Ship's checkpoints, or the operations record.
* At resume, Ship must re-derive its Step 3 mapping and red-deliverable freeze from these
  amended contracts, not from its pre-halt operations record. Unblocking the shipment is
  Ship's or the Orchestrator's action.

## Plan Review

<!-- plan-review: Amendment 2 (harness lifecycle contract), 2026-10-04 -->

dispatch_mode: multi-agent-dispatch

decision: ADVISORY

**Scope of this record.** This record covers Amendment 2: the Harness Lifecycle Contract
section, the edits to units U1, U3, U4, U6, U7, and U8, and the harness edits to tasks
196.001-T through 196.009-T. Under the Step 4 rule, it supersedes the earlier records as the
plan's authoritative gate. The earlier records stay unchanged for audit.

**Reviewers dispatched, all read-only and in parallel:**

| Reviewer | Route | Verdict |
|---|---|---|
| Plan-review persona: correctness and feasibility (Correctness Reviewer) | default | ADVISORY: 0 P0/P1, 1 P2, 2 P3 |
| Plan-review persona: scope boundary and policy compliance (Scope Boundary Auditor) | default | ADVISORY: 0 P0/P1, 3 P2, 3 P3 |
| Adversarial Review (multi-model; anchor gpt-6-sol plus Tier 1, Tier 2, and Tier 3 reviewers) | report-only | ADVISORY: 0 P0/P1, 1 P2, 1 P3 |

All three found the deviations from the minimum directive justified and within 196-S. Those
deviations are:

* the `covered-by` green makers;
* U6 kept harness-required;
* the collision-free `TestUSR<n>_` token;
* the LF-normalized checksum.

No reviewer found a P-002.1, P-002.3, P-002.5, or P-002.6 conformance defect in any
contract block or command. That covers key order, owner rules, marker placement, the
vacuous-pass checks, and quoting under pwsh and bash.

**Findings and dispositions (fixed in the body, P-021 C1 on 196-S's own surface):**

* **AR-1, COR-1, SCO-1 (P2):** the 196.006-T description still said to hash the file "as
  checked out" (raw CRLF bytes), which contradicted AC1.
  * Fixed: it now says LF-normalized content via the AC1 recipe.
* **COR-2 and SCO-4 (P3):** the U6 posture, Exit, Size, and Scope did not mention the
  `TestUSR6_` harness.
  * Fixed in the U6 section and the Scope list. The "What does not change" list now names
    the U6 hashing correction and the renames.
* **COR-3 (P3):** checksum currency had no recorded evidence.
  * Fixed: U6 AC1 and its Exit now require the recipe output for each file in the
    completion evidence. The harness still never re-hashes.
* **SCO-5 (P3):** the rationale overstated what P-002.4 says.
  * Fixed: it is reworded class by class.
* **SCO-6 (P3):** the `TestUSR6_` citations need stable-contract marking.
  * Fixed: U6 AC4 requires a stable-contract comment and names `731CE551`.
* **SCO-2 (P2):** the task files were edited directly, so the index might be stale.
  * Resolved: `backlogit sync` and `backlogit_sync_index` both indexed 1886 artifacts with 0
    parse failures. `backlogit_get_item 196.004-T` returns the new contract block.
  * Item statuses and the shipment block envelope were not touched.
* **SCO-3 (P2):** Amendment 2 had no Plan Review record of its own.
  * Resolved by this record.

**Residuals, accepted and outside 196-S:**

* **AR-2 (P3):** the P-002.4 completion gate in build-feature computes its changed-file set
  with `git diff --name-only`, which does not list an untracked new `*_test.go`. Ship must
  stage the new U4 and U7 test files before running that gate. Fixing the policy or skill is
  outside 196-S and is reported to the Orchestrator.
* **COR-note:** task titles keep the plan unit labels (`U4:`, `U7:`), while the test token is
  `SR<n>`. The alias is bound in this plan's token table and in every task's notes and
  commands. Ship must take the selector from the task contract, never from the title.

operator_authorization: approved

* **Provenance:** P-017 dark mode, `DARK_MODE_ACTIVE: scope=[196-S]`, relayed by the
  Orchestrator to Stage on 2026-10-04, after Ship's governed block. The Orchestrator directed
  Stage to author Amendment 2, run plan-review and adversarial review, decide without the
  operator, and halt only if a step was unsafe.
* **Scope:** this authorizes Amendment 2 and its ADVISORY outcome for 196-S only. It does not
  unblock the shipment, change any task status, or authorize any other release unit.

## Amendment 3: Owner-Harness Completion for U8 Review Findings (reopen route)

**Trigger.** Ship's report-only U8 review returned twelve findings, `U8-R01` through
`U8-R12` (`docs/scratch/2026-10-05-196-S-u8-review-findings.md`). Ship halted on P-001
(`docs/memory/2026-10-05/ship-196-S-u8-p001-review-halt.md`), and the Orchestrator halted
at the checkpoint gate
(`docs/memory/2026-10-05/orchestrator-196-S-dark-mode-halted-checkpoint-gate.md`).

**Authority.** The operator's binding decisions of 2026-10-04T23:32-07:00, under P-017
dark mode (`scope=[196-S]`, `merge_approval_pre_authorized=true`,
`admin_fallback_pre_authorized=false`):

1. `checkpoint-20261005-052350.json` is selected and confirmed for the Ship resume. Ship
   restores it; Stage does not.
2. Amendment 3 takes the "reopen route". The new assertions go into the OWNER harness of
   each finding: R06 into U1 (196.001-T), R08 into U3 (196.003-T), and R04 and R07 into
   U8 (196.008-T). U1 and U3 are reopened through a governed transition, and U8 returns
   to harness-architect. Fresh red baselines are captured before the green tasks 002,
   005, and 009 run. Shipment membership `M` is unchanged: no new tasks and no P-002.6
   re-freeze.
3. P-001 is ratified as resolved on evidence (see the Objective). `U8-R01` is resolved.
4. The P-021 C2 deferral records are fixed (see the per-finding table).

No governed operation can reopen a `done` task (A3.4). Decision 2's "via governed
transition" therefore cannot be met as written, and Phase 2 (A3.5) is gated on an
operator ruling. Everything else in decision 2 proceeds.

**Precedence.** Where Amendment 3 conflicts with the text of U1, U2, U3, U5, U8, or U9,
Amendment 3 governs. For 196.001-T, 196.003-T, and 196.008-T only, it also governs the
timing and provenance of `red_baseline_sha`: Ship Step 4.1a item 5 and build-feature's
"captured before this task was claimed" rule are replaced by A3.3 item 4, and Ship's
dispatch payload states that the baseline is supplied under A3.3 item 4. Ship passes sections A3.1 through A3.6 to harness-architect and to
build-feature together with the unit text.

**What does not change:**

* requirements R1–R15, the dependency graph, and shipment membership `M`
* the Step 3 red-deliverable mapping: every task's `red_selector_command`,
  `green_maker_tasks`, and `green_maker_closes_wave`
* every `covered-by` command of 196.002-T, 196.005-T, and 196.009-T
* every test function name, subtest name, and each harness's red/pass profile:
  * U1: `ProcedureLiterals` and `CallSites` RED; `CrossReferenceInvariant` PASS
  * U3: all three subtests RED
  * U8: `DefinitionLiterals` and `CallSites` RED; `PreservedInvariants` PASS
* AC3 of 196.001-T, 196.003-T, and 196.008-T ("Exactly 1 new file, 1 test function,
  3 t.Run scenarios"). Every addition goes inside an existing subtest.

Only the wave partition changes, and only under ruling A: the reopened tasks form their own
wave, and the effective closing waves shift by one as an accepted, recorded drift (A3.5).

### A3.1 Per-finding P-021 C1 disposition

| Finding | C1 result | Disposition |
|---|---|---|
| U8-R01 | Resolved | P-001 ratified on evidence: PR #472 merged, `1ccecd94` is an ancestor of HEAD |
| U8-R02 | N/A | Stage recovery-metadata correction under decision 1 (A3.6) |
| U8-R03 | Out-of-scope | Deferred to stash `CDBCB258`, normalized to C2 under decision 4 |
| U8-R04 | Same-surface | Fixed per C3 in the U8 harness (A3.2.3) |
| U8-R05 | Out-of-scope | Deferred to new C2 stash `F6F3AA0E` (requires deliberation) |
| U8-R06 | Same-surface | Fixed per C3 in the U1 harness (A3.2.1); green text bound in U2 |
| U8-R07 | Same-surface | Fixed per C3 in the U8 harness (A3.2.3); green text bound in U9 |
| U8-R08 | Same-surface | Fixed per C3 in the U3 harness (A3.2.2); green text bound in U5 |
| U8-R09 | Out-of-scope | Deferred to new C2 stash `147BD825` (requires deliberation) |
| U8-R10 | Out-of-scope | Deferred to stash `CDBCB258`, normalized to C2 under decision 4 |
| U8-R11 | Covered | Dropped: already pinned by `TestUCS1_ClaimStartContract` |
| U8-R12 | Out-of-scope | Already deferred to compliant C2 stash `3B25D37F` |

**C1 reasoning, same-surface (R04, R06, R07, R08).** Each finding strengthens a contract
this shipment already authorizes, on the same owner harness, against text that this
shipment's green tasks already write:

* R06: the 227A2930 handoff contract (U1 and U2)
* R08: the B83081F5 safe-close rule (U3 and U5)
* R04 and R07: the U8 and U9 use-time attestation contract

The C3 symmetric guard applies. The pins are added only where the green text is already in
scope, and every literal they require is bound verbatim to a green unit below. No new
surface, file, task, or production code is added.

**C1 reasoning, out-of-scope (R03, R05, R09, R10):**

* **R03 and R10.** These edit Ship steps outside Step 4.0 item 4 and Step 4.1b, Ship's
  direct-invocation intake contract, and policy text. 196-F excludes all of these (Risks;
  Amendment 1 boundary). Interim limitation: route direct Ship requests through the
  Orchestrator. 196-S claims no end-to-end served-root parity.
* **R05.** The control-record status assertion is not a U4 acceptance criterion.
  Removing or re-authorizing it is a design decision about a `done` task's deliverable,
  not a same-surface fix.
* **R09.** Formalizing the read-only gate or adding an MCP tool is production Go or a new
  surface. Residual risk: if the gate is tightened later, Ship's attestation fails closed
  with `SERVED_ROOT_ATTESTATION_FAILED`; it does not fail open.

**C2 deferral records:**

* `CDBCB258` was edited in place with `backlogit_stash_edit` rather than superseded.
  Stage's anti-duplication rule reconciles into the earliest-captured entry, so a
  replacement entry would be a duplicate. Decision 4 allowed either route; Stage chose
  this one and flags it for the operator. The entry now carries:
  * the `DEFERRED SCOPE EXPANSION` first line
  * the C1 rationale for R03 and R10
  * per-field source refs: PR and review-thread `N/A` (pre-PR local review), tasks
    196.009-T and 196.002-T, feature 196-F, shipment 196-S
  * `requires deliberation: yes`
  * its original scope text, preserved
  * the Step 4.0 items 1–3 ordering residual (A3.6)
* `F6F3AA0E` (R05) and `147BD825` (R09) are new compliant C2 entries with
  `requires deliberation: yes`. The duplicate scan of all 109 stash entries was clean:
  only the distinct `69B0B3F0` (lifecycle state machine) and `6BF65C9D` (exempt-task
  re-entry) touch nearby topics.
* `3B25D37F` (R12) is verified compliant: all six C2 fields are present. Its
  `requires deliberation: no` does not bypass C6, because Stage's precedence rule forces
  deliberation for every `DEFERRED SCOPE EXPANSION` entry.
* **R11 is covered.** `tests/integration/claim_start_admission_contract_test.go`
  lines 80–86 (`rawLogPathSafetyContract` in `TestUCS1_ClaimStartContract`) already pin
  the raw-log path-safety requirements. The U9 constraint keeps them verbatim.

### A3.2 Assertion specifications (owner harnesses; additive only)

Rules for every addition:

* It is appended at the end of the named existing subtest closure, after its last
  statement. No existing line is changed or deleted.
* It uses the file's existing local helpers (`sliceBetweenUniqueAnchors`,
  `assertContainsAll`) and the existing normalized-text variables.
* New local variables use only the names given below. They must not reuse or shadow an
  existing variable. `ok` is reused only through `name, ok :=` with a new `name`.
* Every Go string literal and every assertion message sits on a single source line.
* Literals in fenced blocks are copied byte-for-byte into Go double-quoted strings.
* A new slice guard uses `require.True` with the unique message given. Guards pass at the
  current HEAD and are not part of the expected red. `require` is used only for slice
  guards. Every order check is `assert.True(t, <condition>, "<order message>")`, and the
  U3 negative check is `assert.NotContains(t, closeReady, "<literal>", "<message>")`.
* The harness compares whitespace-normalized text, so green wording may wrap across
  lines in the Markdown as long as each literal's words stay in order.
* `assertContainsAll` prints `<surface> is missing <kind> contract literals`. Red evidence
  is matched on that full string, or on the full order message, never on the surface name
  alone. Each full red string is unique in its file's output; none contains another.
* Line endings stay as committed, and `gofmt -l <file>` prints nothing.

#### A3.2.1 U1 (196.001-T), `TestUSR1_OrchestratorServedRootHandoffContract`, subtest `CallSites`

Declare one local variable that holds the shared payload literal:

```go
payload := "only after the Served-Root Handoff Procedure passes, with `served_workspace_root`, `served_storage_root`, and the binding evidence in the Ship invocation payload"
```

1. **Item-5 payload (R06).**
   * Slice and guard (message `Step 2 item 5 must have unique start and end anchors`),
     then assert the payload (surface name `Step 2 item 5 Ship payload`) and its order
     (message `item 5 must run the procedure before invoking Ship with the payload`):

     ```go
     item5, ok := sliceBetweenUniqueAnchors(step2, "5. Invoke the **Ship** subagent:", "6. Receive Ship's output:")
     item5Run := strings.Index(item5, "Run the Served-Root Handoff Procedure")
     item5Payload := strings.Index(item5, payload)
     ```

   * The order assertion is `item5Run >= 0 && item5Payload > item5Run`.
2. **Binding evidence (R06).** `assertContainsAll` on `step2` with the literal below and
   surface name `Step 2 binding evidence`.

   ```text
   Include the binding evidence: `id`, ordered items, and the attested `workspace.root_path`, `workspace.storage_root`, and index `file`
   ```

3. **Recovery route (R06).** `assertContainsAll` on the existing `recovery` slice with
   `payload` plus the three literals below, surface name `Ship recovery payload`.

   ```text
   Use the checkpoint summary's shipment ID only
   do not read the state dump
   If the summary names no shipment, halt to the operator
   ```

4. **Blocked route (R06).** `assertContainsAll(t, blockedRoute, []string{payload},
   "blocked-shipment payload")`.

* **Expected red at the current HEAD** (each full string appears in `-v` output):
  * `Step 2 item 5 Ship payload is missing handoff contract literals`
  * `item 5 must run the procedure before invoking Ship with the payload`
  * `Step 2 binding evidence is missing handoff contract literals`
  * `Ship recovery payload is missing handoff contract literals`
  * `blocked-shipment payload is missing handoff contract literals`

  Stage verified that `payload` and "checkpoint summary's shipment ID" each have count 0 in
  `_orchestrator.agent.md`, and both item-5 anchors have count 1.
* **Green (196.002-T).** U2's Change 1 item f already contains literal 2 verbatim.
  Amendment 3 binds this Change 2–4 wording verbatim:
  * **Change 2.** Bullet 1 under item 5 reads: "Run the Served-Root Handoff Procedure.
    Invoke Ship only after the Served-Root Handoff Procedure passes, with
    `served_workspace_root`, `served_storage_root`, and the binding evidence in the Ship
    invocation payload; otherwise halt with `SERVED_ROOTS_UNRESOLVED` and do not invoke
    Ship. An operator `ship {id}` that names a non-queued shipment also runs the procedure
    before Ship is invoked, or halts to the operator."
  * **Change 3.** Keep the existing bullet text "`agent: ship` → invoke the **Ship**
    subagent likewise, under its own Crash-Resumption / Startup Recovery Protocol (see the
    Ship agent definition)." verbatim, so the slice anchor stays unique. Append in the same
    bullet: "Before invoking Ship, run the Served-Root Handoff Procedure for the
    checkpoint's shipment. Use the checkpoint summary's shipment ID only; do not read the
    state dump. If the summary names no shipment, halt to the operator. Invoke Ship only
    after the Served-Root Handoff Procedure passes, with `served_workspace_root`,
    `served_storage_root`, and the binding evidence in the Ship invocation payload."
  * **Change 4.** The sentence begins: "Route blocked shipments to Ship only after the
    Served-Root Handoff Procedure passes, with `served_workspace_root`,
    `served_storage_root`, and the binding evidence in the Ship invocation payload, for
    `backlogit_unblock_shipment` …". Add after it: "The procedure binds that blocked
    shipment's `shipment_id`." The anchor `Route blocked shipments to Ship` stays unique.

#### A3.2.2 U3 (196.003-T), `TestUSR3_ShipmentReconcileExplicitFeatureMemberContract`, subtest `SupersededAndPreserved`

1. **Safe-Close step 2 (R08).**
   * Slice and guard with message
     `Safe-Close step 2 must have unique start and end anchors`:

     ```go
     closeReady, ok := sliceBetweenUniqueAnchors(skill, "2. **Require close-ready state.**", "3. **Capture the baseline.**")
     ```

   * `assertContainsAll` on `closeReady` with the literals below and surface name
     `Safe-Close step 2 acceptance rule`.

     ```text
     every non-pre-archived explicit task member must be `done`
     a non-pre-archived explicit feature member must be `done` or meet the `feature-pending-governed-completion` condition
     any other non-pre-archived explicit member must be `done`
     any other status of a non-pre-archived explicit feature member halts before mutation
     Safe-close re-checks this condition on the state re-read under lock
     ```

   * Assert `closeReady` does NOT contain `every non-pre-archived explicit member`, with
     message `Safe-Close step 2 must not require every explicit member to be done`.

* **Expected red at the current HEAD:**
  * `Safe-Close step 2 acceptance rule is missing explicit-feature contract literals`
  * `Safe-Close step 2 must not require every explicit member to be done`

  Both anchors have count 1. The superseded generic sentence is present, and none of the
  five literals is.
* **Green (196.005-T).** Amendment 3 replaces U5's three-bullet Safe-Close step 2 with
  this verbatim text. The `feature-pending-governed-completion` definition already
  requires at least one explicit task member, so step 2 does not restate it. A pre-archived
  feature member is exempt, like any pre-archived member.

  > 2. **Require close-ready state.** Reject blocked or other nonterminal shipment
  > state. Then apply these rules: every non-pre-archived explicit task member must be
  > `done`; a non-pre-archived explicit feature member must be `done` or meet the
  > `feature-pending-governed-completion` condition; any other non-pre-archived
  > explicit member must be `done`; any other status of a non-pre-archived explicit
  > feature member halts before mutation. Safe-close re-checks this condition on the
  > state re-read under lock.

#### A3.2.3 U8 (196.008-T), `TestUSR8_ShipServedRootAttestationContract`

1. **Root equality (R07), in `DefinitionLiterals`.** `assertContainsAll` on the existing
   `step41b` slice with the literals below and surface name
   `Ship Step 4.1b root equality`.

   ```text
   `workspace.root_path` must equal the canonical served workspace root
   `workspace.storage_root` must equal the canonical served storage root
   SELECT name, file FROM pragma_database_list WHERE name = 'main'
   exactly one row whose `file` is `backlogit.db` as a direct child of the canonical served storage root
   or mismatch halts with `SERVED_ROOT_ATTESTATION_FAILED: {reason}` before any claim, raw item-log read, or CLI fallback
   ```

2. **Step 4.0 order (R07), in `CallSites`.** On the existing `step40Item4` slice:
   * Compute:

     ```go
     rootReq40 := strings.Index(step40Item4, "Require both served roots before wave admission.")
     attest40 := strings.Index(step40Item4, "Served-Root Attestation")
     rawRead40 := strings.Index(step40Item4, "and `logs/<id>.jsonl` under the served storage root")
     ```

   * Assert `rootReq40 >= 0 && attest40 > rootReq40 && rawRead40 > attest40` with message
     `Step 4.0 attestation must follow the root requirement and precede the first raw item-log read`.
   * `assertContainsAll` on `step40Item4` with the literal below and surface name
     `Ship Step 4.0 item 4 failure scope`.

     ```text
     A failure halts with `SERVED_ROOT_ATTESTATION_FAILED` before any claim or raw item-log read
     ```

3. **Fallback halt (R07), in `CallSites`.** `assertContainsAll` on the existing
   `appendFallback` slice with the literal below and surface name
   `Ship Step 4.1b fallback halt`.

   ```text
   If it fails, halt with `SERVED_ROOT_ATTESTATION_FAILED` and do not use the CLI fallback
   ```

4. **Fallback order (R04), in `CallSites`.** A NEW assertion; the existing one is not
   edited. Reuse the closure's existing `attestation`, `reRead`, and `cliFallback`
   variables and assert `attestation >= 0 && reRead > attestation && cliFallback > reRead`
   with message `fallback item-log re-read must precede the CLI fallback`.
   * The `reRead`-before-`cliFallback` order already holds today. The assertion is red
     only because the attestation conjunct fails.
   * It turns green only when U9 places the attestation first and keeps the re-read before
     `` `backlogit comment add ``.
   * Limitation: this pins textual order only, not runtime behavior.

* **Expected red at the current HEAD:**
  * `Ship Step 4.1b root equality is missing served-root attestation literals`
  * `Step 4.0 attestation must follow the root requirement and precede the first raw item-log read`
  * `Ship Step 4.0 item 4 failure scope is missing served-root attestation literals`
  * `Ship Step 4.1b fallback halt is missing served-root attestation literals`
  * `fallback item-log re-read must precede the CLI fallback`

  `Served-Root Attestation` has count 0 in `_ship.agent.md`. In Step 4.0 item 4, the
  root-requirement and raw-read anchors each have count 1 and are correctly ordered.
* **Green (196.009-T):**
  * The literals in item 1 are bound verbatim as substrings of U9 Change 1.
  * U9 Change 3 already contains the item 3 literal verbatim.
  * Amendment 3 binds U9 Change 2 verbatim. Insert immediately after "Require both served
    roots before wave admission." and before "In shipment mode": "Once both are known,
    run the Served-Root Attestation (Step 4.1b) once per wave admission, before any raw
    item-log read. A failure halts with `SERVED_ROOT_ATTESTATION_FAILED` before any claim
    or raw item-log read. An unknown root still halts with
    `WAVE_CLAIM_STATE_INDETERMINATE`."

### A3.3 TDD conditions (binding on Ship)

1. **Additive only.** The harness commit for each owner touches only that owner's test
   file. `git diff --numstat <parent> <harness commit> -- <owner test file>` shows 0
   deleted lines. Backlog, label, implementation-note, and harness-manifest changes go in a
   separate traceability commit (item 4).
2. **No green yet, per owner.** Immediately before each owner's harness edit, prove that
   `git --no-pager log --oneline origin/main..HEAD -- <green file>` is empty, where the
   green file is `.github/agents/_ship.agent.md` for U8,
   `.github/agents/_orchestrator.agent.md` for U1, and
   `.github/skills/shipment-reconcile/SKILL.md` for U3. Stage verified this at HEAD
   `e380ff3b`: the branch diff against `origin/main` contains only the five new test
   files. If any green commit exists, halt, because a red baseline can no longer be
   observed.
3. **Observed red.** Run the owner's unchanged `red_selector_command` with `-v`:
   * Every expected red string from A3.2 for that owner appears in the output.
   * No new guard message appears.
   * The subtest profile matches "What does not change".
   * `go test -run=^$ -count=1 ./tests/integration` (compile) and
     `go vet ./tests/integration` pass, and `gofmt -l` prints nothing for the file.
   * Lint uses the repository's CI pin: golangci-lint v1.64.8 exits 0 for the package. If
     the session also runs v2.13.2, it reports no new finding against the merge-base.
4. **Clean tree, then a fresh red baseline, per task.** Build-feature Step 0.5b compares
   the baseline against unstaged, staged, and untracked changes. Every bookkeeping change
   must therefore be committed before the baseline is taken. **Clean tree** means, here and
   everywhere in Amendment 3, that all three Step 0.5b commands print nothing:
   `git diff --name-only HEAD`, `git diff --cached --name-only HEAD`, and
   `git ls-files --others --exclude-standard`. There is no path exception. The one
   uncommittable file, `checkpoint-20261005-052350.json`, is in `.git/info/exclude` (Stage
   added it to the 196-S continuity block). Every other checkpoint and memory note Ship
   writes follows the repository norm and is committed. For each owner task, in order:
   * the harness commit (item 1)
   * a `chore(backlog)` traceability commit with the label changes, harness-architect's
     implementation note and harness-manifest record, any memory or checkpoint files
     written since the last commit, and, in Phase 2, the task's Step 4.1b claim move
   * clean tree
   * `red_baseline_sha` := HEAD, then re-dispatch build-feature Step 0.5 for the owner
     task: zero delta against `red_baseline_sha`, and the selector observed RED with the
     A3.2 strings
   * Step 4.3 gates, Step 4.4 review, and red-path `done` (Step 4.5), then a
     `chore(backlog)` commit of that completion's bookkeeping (archive move, memory
     note, checkpoint), so the next task starts from a clean tree

   Baselines are never shared between tasks: `git diff <sha>` includes later commits, so
   one task's completion would appear in a sibling's delta. This baseline supersedes
   `e380ff3b` for U8 and the wave-1 baselines for U1 and U3. Record the output in the
   task's completion evidence.
5. **Baseline before green.** 196.009-T does not start until 196.008-T has its fresh
   baseline committed and is `done` again; this holds under every ruling. Under ruling A,
   the same applies to 196.002-T (owner 196.001-T) and 196.005-T (owner 196.003-T).
   Under ruling C, 002 and 005 run against their owners' wave-1 baselines. Each green
   task's `covered-by` command must then pass in full, which includes the Amendment 3
   assertions that exist under the ruling in force.
6. **Label routing.** Ship Step 2 item 2 and harness-architect Step 1 item 4 skip
   `harness-ready` tasks. Before harness-architect runs, Ship removes `harness-ready` with
   `backlogit_update_item` (full label list minus `harness-ready`). Harness-architect
   re-applies it at its Step 6, and Ship verifies the label is back. P-002.4's
   untracked-file caveat (AR-2) applies: stage the harness file before the completion
   gate runs.
7. **Fresh evidence.** Completion gates, Step 4.3 gates, and the Step 4.4 review for each
   re-entered task run fresh at the new HEAD. Earlier evidence is not reused. For 001 and
   003, every completion or shipment gate record must be timestamped after the Option A
   window ends; their logs still hold `gate_passed` events from wave 1.
8. **Harness-architect invocation contract.** Every Amendment 3 call passes
   `feature=196-F` and an explicit task list: `tasks=196.008-T` in Phase 1, and one call
   with `tasks=196.001-T,196.003-T` in Phase 2. Each call states:
   * The task is in scope by this amendment. For 196.008-T, which is `active`, cite the
     Phase 1 P-005 record; Step 1 item 3's non-ready exclusion is waived for this task
     only. If any named task would still be excluded, halt; never drop it silently.
   * The edit is append-only per A3.2: no new file, no stub, no `// TODO: implement`
     marker, and no edit to an existing line.
   * Step 5.2 is satisfied when the task's test function fails under its own selector
     with the A3.2 strings. The PASS subtests named in "What does not change" stay
     passing and are not "fixed".
   * The Step 6 note and manifest record go in the traceability commit (item 4), never in
     the harness commit.

### A3.4 Governed reopen: BLOCKED (operator ruling required)

The approved route requires reopening 196.001-T and 196.003-T "via governed transition".
No governed operation can perform that reopen:

* The lifecycle pre-hook (`internal/hooks/builtin_pre.go:16-24`, defaults in
  `internal/config/defaults.go:503-520`) allows `done -> {archived}` only. No
  `.backlogit/hooks.yaml` override exists, so the defaults apply.
* `backlogit_move_item` and `backlogit_update_item` both run that pre-hook.
* `backlogit_reconcile_archived_lifecycle` targets terminal states only.
* `backlogit_return_blocked` removes the item from the shipment, which would change `M`.
* No skill or agent defines a reopen procedure, and no item log shows a precedent.

Stage does not invent a substitute (P-017 halt rule). This section is the HALT, and
**Phase 2 in A3.5 is gated on the operator's ruling.**

**Target status and start epoch.** Even with an exception, the target must be `queued`,
never `active`. Ship's start epoch begins at the latest `status_changed` event to `active`
with reason `shipment claimed`. The existing `WORK_STARTED: 196-S` record of each task is
therefore in the task's current epoch. A task moved to `active` would be an active
residual at Step 4.0. A `queued` task re-enters through Step 4.1b instead. There, Step
4.1b appends a start record only when none is valid in the current epoch: if the claim
move does not open a new epoch, the existing record satisfies it and nothing is appended;
if it does, exactly one record is appended. Either way the re-read must show exactly one.

**Ruling record.** The operator records the ruling in
`docs/memory/<date>/operator-196-S-a3-ruling.md`. Its first line is exactly
`A3_RULING: A`, `A3_RULING: B`, or `A3_RULING: C`. The file also carries:

* `A3_RESUME_CHECKPOINT: <filename>`: the G-A3 checkpoint Ship recorded in its halt
  note. This is the operator's explicit selection and restore confirmation for Phase 0′.
  It replaces any further `resume_checkpoint_ref` hand edit: the manifest ref stays at
  `checkpoint-20261005-052350.json` until a governed path refreshes it.
* `A3_R02_RATIFIED: yes` or `no`: the operator's ratification of Stage's one R02 edit
  (A3.6).
* under ruling A, the Option A record (step 9).

The operator commits the ruling file on its own, as `docs(memory): 196-S A3 ruling`. Under
ruling A, that commit comes after the reopen commit (step 10). Under ruling B or C, it is
made before Ship is re-invoked. The Orchestrator passes the ruling-file path in every Ship
payload once that commit exists.

**Options for the operator:**

* **A (recommended; keeps `M` unchanged; closest to the approved intent).** A one-time
  lifecycle exception, executed by the operator only. Ship's Role Boundary does not cover
  writing lifecycle configuration, so Ship never runs it and only verifies the result.
  Ruling A replaces decision 2's "via governed transition" with this exception for
  196.001-T and 196.003-T only. It runs only after Phase 1 completes and while Ship is
  halted at G-A3 (the wave-1 Step 4.6 record and Ship's committed G-A3 halt note both
  exist). No Ship, Stage, or Orchestrator activity and no MCP mutation may take place
  during the window.
  1. **Pre-checks.** If `<served storage root>/hooks.yaml` exists, halt. If either task
     file carries `archived_from` or `archived_status`, halt; at staging, neither does.
     The tree must be clean (A3.3 item 4).
  2. **Attest and record versions.** Run the Served-Root Attestation recipe this plan
     defines (U2 step d and U9 Change 1, applied under R14) against the MCP server.
     Record `backlogit_get_version` (MCP; `7c805f9` at staging) and `backlogit version`
     (CLI; `131577c`, v1.11.0, at staging). `131577c` is an ancestor of `7c805f9`, and the
     two differ in neither `internal/config` nor `internal/hooks`. They do differ in
     `internal/core` (claim-marker files). The step 7 three-way read is the accepted
     backstop for that skew.
  3. **Snapshot.** For both tasks, record the full file content and SHA-256 under
     `archive/`, the frontmatter key set (including
     `custom_fields.scheduler_baseline_claim`, `labels`, `references`, `parent_id`, and
     `commit`), and the body section markers.
  4. **Build the hooks file.** Run `backlogit init` with the same CLI in a new empty
     directory under the system temp folder, outside the repository and the served root,
     with `BACKLOGIT_WORKSPACE_DIR` unset. Copy its `hooks.yaml` to
     `<served storage root>/hooks.yaml`. Change only the `done` transition list to
     `[archived, queued]`. A diff of the scratch file against the copy must show exactly
     that row. A partial file would replace the defaults wholesale, so never hand-write
     it. Delete the scratch directory. Record the file's SHA-256 and the window start
     time.
  5. **Move, then always clean up.** Inside a try/finally block, run
     `backlogit move 196.001-T --status queued` and then
     `backlogit move 196.003-T --status queued`. Run each as an argv array with
     `--cwd <served workspace root>` and child `BACKLOGIT_WORKSPACE_DIR=.backlogit`. Stop
     at the first failure. The `finally` block deletes `hooks.yaml` only if its SHA-256
     still equals the recorded value; otherwise it leaves the file and halts. Record the
     window end time.
  6. **Verify removal.** `Test-Path` on the hooks file is false, and
     `git status --short -- .backlogit/hooks.yaml` is empty. Do not test the exception by
     attempting any other move.
  7. **Sync and verify three ways.** Run CLI `backlogit sync` (A3.6 records that the MCP
     sync missed a manual edit). For both tasks, the CLI item read shows `queued`, the raw
     file sits under `queue/` with no copy under `archive/`, and MCP `backlogit_get_item`
     shows `queued`. A partial result (one task moved, a failed move, or any mismatch)
     halts: record each task's status and do not retry inside this window. A later window
     may move only a task that is still `done`.
  8. **Contract integrity.** The raw Markdown is the authority, because the index does
     not project every frontmatter key. First, `git status --porcelain` must list exactly
     the two archive paths (deleted) and the two queue paths (untracked); any other path
     halts, and its diff goes in the ruling file. Then run `git add -A -- <both archive
     paths> <both queue paths>`; `git diff --cached -M --name-status` must show an `R` row
     for each task. Against the snapshot, only `status`, `updated_at`, and the location
     may change. Any other change or loss halts with the diff.
  9. **Commit and record.** Commit the two renames alone as
     `chore(backlog): reopen 196.001-T and 196.003-T (196-S Amendment 3)`. Write the
     ruling file with the window start and end times, the hooks file SHA-256, both
     versions, the two moved items, the commit SHA, and the verification results. Append
     a comment on each task with actor `operator`, never `ship`:
     `REOPENED: 196-S Amendment 3 A3.4 Option A`. That comment is never a start record.
     Comments go to the git-ignored `logs/`, so they leave the tree clean.
  10. **Commit the ruling file** alone as `docs(memory): 196-S A3 ruling`, then confirm
      the tree is clean.
* **B.** Add new harness-required tasks for the U1 and U3 additions under an explicit
  P-002.6 `M` re-freeze ruling. This contradicts decision 2's "`M` unchanged". It needs
  new collision-free `TestUSR1H_` and `TestUSR3H_` functions, and the `covered-by`
  commands of 196.002-T and 196.005-T must change. Stage must write Amendment 4 first, and
  wave 2 is released only after the re-freeze. Not recommended.
* **C.** Accept R06 and R08 as documented residual risk (P-021 C3) and ship U8's
  additions only. U1 and U3 stay as they are, and Ship captures R06 and R08 under P-021
  C2. This weakens the 227A2930 and B83081F5 contracts. Not recommended.

A governed reopen operation in backlogit is production Go and is out of 196-S scope. Stash
`69B0B3F0` (lifecycle state machine) is its natural home.

### A3.5 Ordered Ship execution on resume

The wave schedule is computed live by Step 4.0, so the reopen changes waves without a
re-freeze. Ship carries the wave index forward across sessions: wave 1 is in progress
now, and the first wave after G-A3 is wave 2.

* **Ruling A:** 1 = {001, 003, 004, 007, 008}; 2 = {001, 003} (re-entered);
  3 = {002, 005}; 4 = {009}; 5 = {006}. That is five waves, within the budget of
  `count(M) = 9`.
* **Ruling C:** waves 2–4 are {002, 005}, {009}, and {006}, as originally planned.

**Closing-wave drift under ruling A.** The declared `green_maker_closes_wave` values stay
2 (001), 2 (003), and 3 (008), but the green-makers now land in waves 3, 3, and 4. Step 3
item 3's equality check was validated at freeze and is not re-run; ruling A accepts this
drift. Step 4.6 item 4 is evaluated against the set recomputed at item 2, and it passes:
at wave 2 the open entries are at their declared wave, not past it; at wave 3, 001 and 003
close; at wave 4, 008 closes. Completion comments and `FULL_SUITE_DEFERRED` records cite
both the declared and the effective closing wave. U6's "wave 4" references (Scope, the
Dependency Graph, and the Amendment 2 table) read "wave 5" under ruling A.

The Step 3 item 9 scheduler replay is pinned to the fixture shipment `130-S`
(`tests/simulation/wave-scheduler-contract.json`) and does not model 196-S, so the reopen
cannot change its result. No re-run is required.

#### Phase 0: first resume (do NOT run Step 4.0)

1. **Orchestrator payload.** Before invoking Ship, the Orchestrator re-supplies the
   Amendment 1 bootstrap payload: both absolute served roots, this plan path, explicit
   citations of R14 and Amendment 3, and decision 1 as the operator's confirmation of the
   checkpoint restore. A missing item halts with `SERVED_ROOTS_UNRESOLVED`.
2. **Restore.** Ship restores `checkpoint-20261005-052350.json` under its own recovery
   protocol. Anomaly-first enumeration still runs. Decision 1 is the operator
   confirmation, so Ship does not wait for another.
3. **Preconditions.** Halt and report if any of these fails:
   * MCP `backlogit_get_shipment` for `196-S` shows
     `resume_checkpoint_ref: checkpoint-20261005-052350.json`, and its items are 196-F
     plus the checkpoint's nine `task_ids`.
   * The R14 Served-Root Attestation passes.
   * `<served storage root>/hooks.yaml` does not exist.
   * The tree is clean (A3.3 item 4). Stage's Amendment 3 commit already holds the plan,
     manifest, stash, and 2026-10-05 memory files, and
     `checkpoint-20261005-052350.json` is excluded locally. Any file Ship writes during
     the restore is committed as `chore(backlog)` before this check.
   * HEAD descends from `e380ff3b`, and A3.3 item 2 holds for all three green files.
   * Live statuses: 001, 003, 004, and 007 are `done`. 008 is `active` with exactly one
     current-epoch `WORK_STARTED: 196-S`. 002, 005, 006, and 009 are `active` and
     claim-assigned, with no start record.
4. **Mapping.** Compare `M` and the five red-deliverable contract keys of every member
   with the frozen values (red deliverables 001, 003, and 008 with their declared
   green-makers and closing waves). They must be equal. Do not recompute the Step 4.0
   item 7 partition, do not re-run Step 3 item 3's post-partition validation (A3.5,
   closing-wave drift), and do not re-freeze. The same rule applies at Phase 0′. Once
   these checks pass, resolve `checkpoint-20261005-052350.json` under Ship's own
   protocol.
5. **No Step 4.0 now.** Wave 1 is in progress, and 008 is its only non-terminal member.
   Running Step 4.0 now would classify 008 as an active residual and halt with
   `WAVE_NO_PROGRESS`. Resume 196.008-T at its in-task cursor, Step 4.4, and disposition
   the findings by A3.1. **Never append another `WORK_STARTED` to 008.**

#### Phase 1: U8 in-task C3 remediation (not gated)

6. Emit a P-005 event: "Amendment 3 harness-architect re-entry for 196.008-T, in-task
   P-021 C3 remediation authorized by decision 2; not a wave admission." This one
   single-task harness-architect call stands in for Step 4.0 item 10 for 008 only.
7. Remove `harness-ready` from 008 (A3.3 item 6). Invoke harness-architect under the
   A3.3 item 8 contract with `tasks=196.008-T` and the A3.2.3 additions only.
8. Apply A3.3 items 1–3 with `go test -count=1 -v -run '^TestUSR8_' ./tests/integration`.
   Expected: `DefinitionLiterals` FAIL, `CallSites` FAIL, `PreservedInvariants` PASS, and
   all five A3.2.3 strings present. Commit with type `test` and scope `harness`.
9. Verify that `harness-ready` is back, then apply A3.3 item 4: traceability commit, clean
   tree, `red_baseline_sha` := HEAD, and build-feature Step 0.5.
10. Re-run the Step 4.3 gates and the Step 4.4 review at the new HEAD. Handle new findings
    under P-021.
11. Complete 008 on its red-path `done` path (Step 4.5), then commit the completion
    bookkeeping (A3.3 item 4).
12. Run the wave-1 Step 4.6 convergence gate as usual. The open-red set is {001, 003,
    008}, and each red selector must be observed RED.

#### Gate G-A3 (HALT before wave 2)

13. After wave 1 converges and before the next Step 4.0, Ship looks for the ruling file
    (A3.4) in its payload. This check is mandatory: without it, Step 4.0 item 6 would
    compute `ready_k = {002, 005}`, because 001 and 003 are `done`. From Phase 1 onward,
    every Ship checkpoint `resume_hint` restates this gate.
    * **No ruling file, or `A3_RULING: A` without a committed Option A record:** write a
      fresh checkpoint under Ship's halt protocol, and record its filename in the P-005
      event and in Ship's halt memory note. Commit the checkpoint and the note as
      `chore(backlog)` so the tree is clean for Option A. Emit `DARK_MODE_HALTED` with
      reason "Amendment 3 reopen pending operator lifecycle exception" and next action
      "operator ruling A/B/C; under A, the operator executes A3.4 Option A". Halt. Ship
      never edits the shipment manifest by hand. The operator names this checkpoint in
      the ruling file's `A3_RESUME_CHECKPOINT` line; no manifest repoint is needed.
    * **`A3_RULING: B` or `C`, or `A` with its Option A record and reopen commit already
      present:** continue at Phase 0′ item 15, skipping the halt.
14. As a second guard, at every later Step 4.0 item 6:
    * If `<served storage root>/hooks.yaml` exists, halt with
      `A3_LIFECYCLE_EXCEPTION_PRESENT` and do not claim.
    * If `ready_k` contains 196.002-T or 196.005-T while Phase 2 is incomplete and the
      ruling is not C, halt with the G-A3 record instead of claiming. Phase 2 is complete
      when the wave-2 Step 4.6 record exists and 001 and 003 are `done` with gate records
      timestamped after the Option A window ends.

#### Phase 0′: resume after G-A3

15. **Payload and preconditions.** The Orchestrator passes both served roots, this plan
    path, the R14 and Amendment 3 citations, and the ruling file path. Ship restores the
    checkpoint named by the ruling file's `A3_RESUME_CHECKPOINT` line under its own
    protocol; that line is the operator's selection and confirmation. Under ruling A,
    the checkpoint records 001 and 003 as `done` while they are now `queued`. That
    divergence is the expected Option A result, confirmed by the ruling file, and is not a
    restore anomaly. Ship halts if any of these fails:
    * The R14 attestation passes.
    * The ruling file is committed (an ancestor of HEAD), and its first line is
      `A3_RULING: A`, `B`, or `C`.
    * `<served storage root>/hooks.yaml` does not exist.
    * 008 is `done`, and the wave-1 Step 4.6 record exists.
    * The tree is clean (A3.3 item 4).
    * **Ruling A only:** the ruling file carries the Option A record, and its reopen
      commit is an ancestor of HEAD. 001 and 003 are `queued` under `queue/`, and each has
      exactly one valid `WORK_STARTED: 196-S` under the Step 4.1b epoch rule.

    Then: ruling A → Phase 2; ruling B → halt for Stage Amendment 4; ruling C → Phase 3.

#### Phase 2: U1 and U3 (ruling A only)

16. While both tasks are `queued`, remove `harness-ready` from each (A3.3 item 6).
17. Run Step 4.0 for wave 2. Expect `ready_k = {001, 003}`; 002, 005, 006, and 009 wait as
    claim-assigned members with unfinished dependencies. Any other result halts with the
    item 7 report.
18. At Step 4.0 item 10, harness-architect runs once, under the A3.3 item 8 contract, with
    `tasks=196.001-T,196.003-T`, and adds the A3.2.1 and A3.2.2 assertions only. Apply
    A3.3 items 1–3 per owner:
    * U1 expects `ProcedureLiterals` FAIL, `CallSites` FAIL, `CrossReferenceInvariant`
      PASS, and all five A3.2.1 strings.
    * U3 expects all three subtests FAIL and both A3.2.2 strings.

    Commit each owner file on its own `test(harness)` commit.
19. Claim each task through Step 4.1b and apply its start-record rule exactly (A3.4). The
    re-read must show exactly one valid record.
20. Apply A3.3 item 4 to 196.001-T and then to 196.003-T, each with its own traceability
    commit, clean tree, `red_baseline_sha`, build-feature Step 0.5, fresh gates and review
    (A3.3 item 7), red-path `done`, and completion-bookkeeping commit. The first
    traceability commit also carries both claim moves.
21. Run the wave-2 Step 4.6 gate. The open-red set is again {001, 003, 008}, all RED.

#### Phase 3: green tasks

22. The next wave runs 196.002-T (U2 plus the A3.2.1 green wording) and 196.005-T (U5
    plus the A3.2.2 text). Their `covered-by` selectors `^TestUSR1_` and `^TestUSR3_` must
    pass in full. At Step 4.6, 001 and 003 are newly closed and must be GREEN.
23. The following wave runs 196.009-T (U9 plus the A3.2.3 Change 2 wording). `^TestUSR8_`
    must pass in full, and 008 must be GREEN at Step 4.6.
24. The last wave runs 196.006-T. It computes the U6 checksums after the last
    installed-file edit.

### A3.6 Shipment record, telemetry, and residual risks

* **R02.** Stage changed `resume_checkpoint_ref` in `.backlogit/queue/196-S.md` from
  `checkpoint-20261004-192622.json` to `checkpoint-20261005-052350.json`, then ran CLI
  `backlogit sync`. No governed operation sets the ref without blocking the shipment. The
  MCP `backlogit_sync_index` did not pick up the edit; the CLI sync did, and MCP
  `backlogit_get_shipment` now shows the new ref. The frontmatter has a single
  `resume_checkpoint_ref` key. Stage also added a body note recording the P-001
  ratification.
* **R02 method.** Change only the single `resume_checkpoint_ref` frontmatter key, run CLI
  `backlogit sync`, and confirm the new value through MCP `backlogit_get_shipment`. This
  is a Stage task and was used once. Amendment 3 needs no further repoint: Phase 0′ takes
  its checkpoint from the ruling file's `A3_RESUME_CHECKPOINT` line. Ship never edits the
  shipment manifest by hand. The operator ratifies the one hand edit through the ruling
  file's `A3_R02_RATIFIED` line.
* `member_status_snapshot` is stale: it shows every member as `active`. Stage did not
  hand-edit it. Refresh it only through a governed Ship or Orchestrator path.
* **Stash citations.** Ship cites the deferrals with comments that change no status:
  `CDBCB258` on 196.009-T and 196.002-T, `F6F3AA0E` on 196.004-T, and `147BD825` and
  `3B25D37F` on 196.007-T. It also cites them in the run-level memory. No comment on
  196.008-T may be a `WORK_STARTED` record.
* **P-017 telemetry expected from Ship.** Each event carries the scope item, gate state,
  outcome, and next action:
  * `DARK_MODE_SCOPE` stays `[196-S]`.
  * A P-005 event records the Phase 1 harness-architect re-entry on 196.008-T.
  * `DARK_MODE_HALTED` is emitted at gate G-A3 if no ruling exists.
  * A P-005 event records the Option A lifecycle exception, if ruled. It cites the ruling
    file and the reopen commit.
  * A P-005 event records the closing-wave drift accepted under ruling A (A3.5).
  * The operator's ratification of the R02 hand edit is recorded in the ruling file.
* **Option A residual risk.** For the length of the window, the lifecycle allows
  `done -> queued` for every item in the served root. A PowerShell `finally` block does
  not run if the process is killed, so the hard backstop is the "`hooks.yaml` absent"
  check repeated at every Ship entry point (Phase 0, item 14, Phase 0′). The try/finally
  block and the rule that no Ship, Stage, Orchestrator, or MCP mutation runs during the
  window are additional bounds.
* **P-021 C3 residual risks.** The PR body and the closure summary list:
  * `CDBCB258` (R03, R10), `F6F3AA0E` (R05), `147BD825` (R09), and `3B25D37F` (R12) as
    follow-ups
  * the R10 interim limitation: direct Ship requests must go through the Orchestrator
  * Step 4.0 items 1–3 read through MCP before the item 4 attestation runs (`CDBCB258`)
  * the R04 assertion pins textual order only
  * under ruling C, R06 and R08 as accepted residual risks
  * R11 as covered by `TestUCS1_ClaimStartContract`
  * R01 as resolved under ratified P-001
* **Permanence.** The Amendment 3 pins become permanent contract tests once green. Later
  edits to these surfaces must keep the pinned literals or amend the pins on purpose.

<!-- plan-review-attempt: 1 -->
<!-- plan-review-attempt: 2 -->
<!-- plan-review-attempt: 3 -->

## Plan Review

<!-- plan-review: Amendment 3, attempt 3 of 3 (final record; supersedes earlier records for Amendment 3 only) -->

* scope: Amendment 3 (A3.1 to A3.6) and the rewritten Objective. Earlier Plan Review
  records for the base plan and Amendments 1 and 2 are not reopened.
* dispatch_mode: multi-agent-dispatch
* decision: FAIL
* operator_authorization: none
* reviewed_at: 2026-10-05 (Stage, P-017 dark mode, scope `[196-S]`)

**Cycle history**

| Cycle | Reviewers | Result | Blocking findings |
|---|---|---|---|
| 1 | Correctness, Agent-Native Parity, Architecture, Go, Learnings, Scope | FAIL | 1 P0 and several P1s: target status, start epoch, Step 4.0 sequencing, exact assertion specs |
| 2 | Correctness (ADVISORY), Agent-Native Parity (FAIL), Architecture (ADVISORY), Go (ADVISORY), Learnings (ADVISORY) | FAIL | 3 P1s (Parity): zero-delta bookkeeping conflict; Option A delegated to Ship; no re-entry after G-A3 |
| 3 | Agent-Native Parity (FAIL), Correctness (FAIL), Architecture (ADVISORY) | FAIL | 4 P1s; see below |

**Cycle-3 P1 findings and remediation, applied after cycle 3 and NOT re-reviewed**

| ID | Reviewer | Finding | Remediation in this text |
|---|---|---|---|
| C3-P1-a | Parity, Correctness | The clean-tree rule allowed untracked `.backlogit/checkpoints/`, but build-feature Step 0.5b's `git ls-files --others --exclude-standard` has no such exception | A3.3 item 4 defines clean tree as the exact Step 0.5b three-command union with no exception. The selected checkpoint is excluded in `.git/info/exclude`. Every other checkpoint and memory note is committed. |
| C3-P1-b | Parity, Correctness | No committer for the ruling file, Ship's G-A3 halt note, Stage's edits, or the repoint | The ruling file gets an operator commit (A3.4 ruling record, Option A step 10). Ship commits its halt note and checkpoint (item 13). Stage's files are committed with this amendment. The second manifest repoint is removed (`A3_RESUME_CHECKPOINT`). Phase 0 gains a clean-tree precondition. |
| C3-P1-c | Correctness | Phase 2 shared one `red_baseline_sha` between 001 and 003, so the first completion appears in the second task's delta | Baselines are per task, each followed by a completion-bookkeeping commit (A3.3 item 4, Phase 2 item 20) |

The cycle-3 P2 and P3 findings are also remediated:
* G-A3 skip rule: A without an Option A record halts.
* Phase 0 item 4 compares the contract keys only.
* A3.3 item 5 is scoped to the ruling.
* Precedence covers the `red_baseline_sha` provenance.
* Phase 0′ declares the expected 001/003 divergence.
* The harness-architect call passes `feature=196-F` and a joint task list.
* The scratch directory is in temp, outside the repository.
* The `REOPENED` comment's actor is `operator`.
* Step 8 checks the full status.
* The `internal/core` skew is accepted with the three-way backstop.
* `A3_LIFECYCLE_EXCEPTION_PRESENT` is defined.
* Phase 2 completion is defined.
* The residual-risk backstop is restated.

**Gate consequence.** The final decision is FAIL. Under Stage's plan-review contract, a
FAIL halts regardless of `skip_review`. Ship MUST NOT execute any part of Amendment 3,
Phase 1 included, until a later Plan Review record for Amendment 3 shows `decision: PASS`,
or `decision: ADVISORY` with `operator_authorization: approved`. The cycle cap is
reached: the operator must either authorize a fourth review cycle of this remediated text
or review it and record the authorization directly. Stage escalation status:
`ESCALATION_DEGRADED`. No engram escalation-intake operation is available in this
session, so the fallback is the operator halt.