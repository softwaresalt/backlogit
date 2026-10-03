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
   * bind them to the backlogit store the MCP server is serving
   * pass them to Ship

   Any failure, error, or indeterminate result halts with `SERVED_ROOTS_UNRESOLVED`.
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

In scope (six units, one covering feature, one shipment):

* Two RED harness-text contract tests (`tests/integration/`).
* One Go characterization test (`internal/core/`).
* Installed-text edits:
  * `_orchestrator.agent.md`: the procedure plus three call sites.
  * `shipment-reconcile/SKILL.md`.
  * One sentence in `_ship.agent.md` Step 6 item a, the consumer-side wording of the reconcile
    gate.
* Harness-manifest drift records for the three edited installed artifacts.

Out of scope:

* Any production Go change. If U4 fails RED, Ship records a P-021 `DEFERRED SCOPE EXPANSION`
  and returns the shipment to Stage.
* Ship-side changes to served-root intake, which are tracked as a Stage follow-up stash
  entry (see Decisions):
  * operator-supplied roots for direct Ship invocation
  * Ship re-verifying the binding evidence
* MCP served-root self-report (deliberation option P1-B).
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
| R4 | Binding proof against the served store, read-only except at most one derived-index `backlogit_sync_index`. (a) `shipment_id` matches the shipment ID pattern. The shipment manifest exists at exactly one of `<storage>/queue/<id>.md` or `<storage>/archive/<id>.md`. The manifest, its parent directory, the `logs` directory, and `logs/<id>.jsonl` are all direct children with no symlink or reparse-point component. (b) Static: MCP `backlogit_get_shipment` and the on-disk frontmatter agree on `id`, `status`, `updated_at`, and ordered `custom_fields.items`. (c) Dynamic: the newest `item_log_entries` row for the shipment, read through MCP `backlogit_query_sql`, equals the last non-empty complete line of the gitignored `<storage>/logs/<id>.jsonl` by `event_type` and parsed UTC instant | U1, U2 |
| R5 | Any failure, error, or indeterminate result in R1–R4 halts with `SERVED_ROOTS_UNRESOLVED: {reason}`. Indeterminate includes zero index rows, an empty log, an unparseable last line, a zero index timestamp that persists after the one sync, and a sync error. Ship is not invoked, and reasons use workspace-relative or redacted path tokens | U1, U2 |
| R6 | Ship receives `served_workspace_root` and `served_storage_root` as canonical OS-native absolute paths, plus the binding evidence, in the invocation payload | U1, U2 |
| R7 | Every Orchestrator invocation of Ship runs the procedure first: Step 2 item 5, Step 0.0b item 6 `agent: ship` (including a shipment already archived by a crashed closure), blocked-shipment routing, and an operator `ship {id}` naming a non-queued shipment | U1, U2 |
| R8 | Verification outcomes go into tracked `docs/memory/` only, in workspace-relative form. The Orchestrator writes no checkpoint, because a valid checkpoint `agent` is `stage` or `ship` and an Orchestrator-written one would become a recovery candidate | U1, U2 |
| R9 | Pre-close classifies an `active` explicit feature member as `feature-pending-governed-completion`. This requires at least one explicit task member and every explicit task member `matched` or `pre-archived`. Any other explicit feature status remains `status-mismatch` | U3, U5 |
| R10 | Safe-close re-checks R9 on the state it re-reads under lock. Its existing step 5 still requires every non-baseline-archived member to appear in `A`, and post-close still requires a valid archive record | U3, U5 |
| R11 | Ship Step 6 item a describes the gate in terms of the skill's classifications, not "every manifest item `done`" | U3, U5 |
| R12 | The governed lifecycle accepted by R9/R10 is pinned. `ShipShipment` completes (`done`) and archives an `active` explicit feature whose explicit tasks are already `done` and archived | U4 |
| R13 | Manifest drift records name all three local edits | U6 |

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
     * `` `item_log_entries` ``
     * `zero rows, an empty log, or an unparseable last line`
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

* **Domain:** docs (installed agent text). **Posture:** test-first (U1). Safety mode: careful.
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
    * b. **Storage root.** Exactly one of `.backlog` or `.backlogit` exists as a directory
      under the served workspace root. If neither or both exist, fail closed.
    * c. **Canonicalize.** Resolve both roots into canonical OS-native absolute form. On
      Windows, git prints forward slashes, so compare drive letters and components
      case-insensitively. Reject any symlink or reparse-point component. Require the
      served storage root to be a direct child of the served workspace root.
    * d. **Bind.**
      * Require `shipment_id` to match `^[0-9]+-S$`.
      * The shipment manifest must exist in exactly one of `queue` or `archive` under the
        served storage root, so a shipment archived by a crashed closure still binds. If it
        exists in neither or both, fail closed. This assumes the default archive directory;
        a manifest that exists only under a configured non-default archive directory is
        found in neither and fails closed.
      * Require the manifest, its directory, the `logs` directory, and
        `logs/<shipment_id>.jsonl` to be direct children with no symlink or reparse-point
        component. Apply Ship Step 4.1b's raw-read safety: no-follow open, opened-path
        verification, and a scoped P-012 raw-read declaration.
      * Read only the manifest frontmatter.
      * Static check: `id`, `status`, `updated_at`, and the ordered `custom_fields.items`
        equal the MCP `backlogit_get_shipment` result.
      * Dynamic check: the newest `item_log_entries` row for `shipment_id` in insertion
        order (`ORDER BY rowid DESC LIMIT 1`), read through MCP `backlogit_query_sql`,
        equals the last non-empty complete line of `logs/<shipment_id>.jsonl` by
        `event_type` and parsed UTC instant, and the row's `log_path` resolves to that same
        log file. That log is gitignored, so a stale clone at the same commit cannot match it.
      * On a mismatch or a zero index timestamp, call `backlogit_sync_index` once and compare
        again.
      * zero rows, an empty log, or an unparseable last line is an indeterminate result, as
        is a zero index timestamp that persists after the sync.
    * e. **Halt.** On any failure, error, or indeterminate result in a–d, halt with
      `SERVED_ROOTS_UNRESOLVED: {reason}` and do not invoke Ship. Indeterminate results
      include detection that is unavailable, an MCP or sync error, a timeout, and a missing
      or unparseable file. Write `{reason}` and the halt trace with workspace-relative or
      redacted path tokens. When only the CLI fallback is reachable, the static and dynamic
      checks cannot run, and the procedure intentionally fails closed.
    * f. **Pass.** Put `served_workspace_root` (Ship's "served workspace root", Step 4.1b)
      and `served_storage_root` (Ship's "served storage root") in the Ship invocation
      payload as canonical OS-native absolute paths. Include the binding evidence: `id`,
      ordered items, and the log-tail `timestamp`/`event_type`. Record the outcome in
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
    fail of each step in the PR body, in workspace-relative form only.
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
  * Append a `drift_reason` sentence that cites `227A2930` or `B83081F5` and ends
    "Do not auto-revert."
  * Keep `drift_allowed: true`.
* **Constraints:**
  * Touch no other entry.
  * Frontmatter model-routing re-render remains `731CE551`'s job.
  * `plugin/` holds no mirror of these three files (checked at staging), so no parity edit
    is needed.
* **Exit:** each of the three checksums equals its installed hash, the YAML parses, and the
  diff is limited to the three entries.
* **Size:** 1 file, 3 entries.

## Dependency Graph

```text
Wave 1: U1 (tests)   U3 (tests)   U4 (tests)
           |            |  \________|
Wave 2: U2 (docs)    U5 (docs)
           \____________/
Wave 3:      U6 (config)
```

* U2 blocks-on U1.
* U5 blocks-on U3 and U4.
* U6 blocks-on U2 and U5.
* The graph has no cycles.

## Decisions and Rationale

* **A named procedure with three call sites.** Ship has three Orchestrator entry points.
  Defining the procedure once in Step 2 and referencing it elsewhere avoids drift, and
  placing it at the end of Step 2 keeps every "step 4/5" cross-reference valid.
* **Static plus dynamic binding (P1-A, extended after review).**
  * Static fields alone cannot tell apart a clone at the same commit.
  * The gitignored item log, compared with the served index's `item_log_entries` through an
    existing MCP surface, binds to the live store. It needs no new MCP surface.
  * Ship's Step 4.1b re-validation at use time remains the TOCTOU control.
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
  exists).** Served-root intake for direct Ship invocation and the Orchestrator's matching
  Ship-halt mapping. It spans both agents and should:
  * accept operator-supplied roots, checked against R1–R4, on direct Ship invocation
  * re-verify the passed binding evidence at Step 4.0
  * add a terminal `SERVED_ROOTS_UNRESOLVED` mapping for Ship's "Orchestrator follow-up"
    halt in the Orchestrator's Ship-halt handling
  * pin the payload key names on the Ship side

  This is out of scope because it edits Ship's intake contract.
* **One derived-index sync.** A stale index is the common cause of a dynamic-binding
  mismatch, and comment rows can carry a zero timestamp until a rehydrate. One sync is the
  smallest remedy. A sync error is indeterminate.
* **Characterization, not a fix, for Go.** A RED U4 stops the shipment (P-021 C1).

## Exception and Dispatch Preconditions (bootstrap)

* **Dispatch bootstrap.** This shipment is dispatched before U2 lands.
  * The dispatching Orchestrator applies R1–R8 from this reviewed plan only after it records
    explicit operator authorization. A P-017 dark-mode bounded scope naming this shipment
    counts as authorization.
  * The authorization record cites this plan path and the pass result of each step a–f, in
    workspace-relative form only.
  * If authorization or root resolution is missing, the Orchestrator halts with
    `SERVED_ROOTS_UNRESOLVED` and asks the operator.
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
| U4 is RED or its precondition cannot be reached | low | high | P-021 halt. U5 depends on U4 |
| Dynamic binding mismatch from a stale index | medium | low | One `backlogit_sync_index`, then compare again. If it still differs, fail closed |
| Harness-text tests are skipped by the CI paths filter (`B3701713`) | medium | low | Ship runs `go test -timeout=30m ./...` locally |
| Manifest checksum mismatch from line endings | low | low | Hash the bytes as checked out |
| A later tune reverts the edits | medium | medium | U6 drift records. Ordering of `731CE551` after this shipment is advisory only |
| Operator-direct Ship invocation still has no root source | medium | low | Follow-up stash. The Ship halt remains fail-closed |
| A deliberate full directory copy, including gitignored logs, passes the binding until the stores diverge | low | low | Accepted residual for a local tool. Ship's Step 4.1b use-time checks and the Ship-side re-verification follow-up |

## Constitution Check

| Principle | Verdict | Note |
|---|---|---|
| I. Safety-First Go | pass | Only `_test.go` files are added. Each Go test file must pass `gofmt`, `go vet`, and `golangci-lint` |
| II. Test-First Development (NON-NEGOTIABLE) | pass | U1 and U3 are RED before U2 and U5. U4 provides the evidence for U5. U4 is expected GREEN on arrival because it characterizes existing behavior and adds no production code. Tests use testify and table-driven `t.Run` |
| III. Workspace Isolation and Security Boundaries | pass | The procedure is read-only except one derived-index sync. It checks canonical paths, symlinks and reparse points on every component it reads, the ID pattern, and the main worktree. Tracked records use workspace-relative form only |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | pass | No step writes outside the working tree |
| V. Structured Observability | pass | `SERVED_ROOTS_UNRESOLVED` is durable and gets a redacted halt trace. `RECONCILE_CONTRACT_NOT_IN_FORCE` is local to this shipment's closure |
| VI. Single Responsibility | pass | One procedure, one classification. Each unit has a single domain |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | pass | Nothing destructive. Rollback is `git revert -m 1` on a branch, merged through a PR, never a force push |
| VIII. Explicit Safety Modes | pass | Careful mode for U2, U5, and the bootstrap dispatch. Freeze-scope covers the seven files named in U1–U6. The declared runtime write targets are `docs/memory/` (outcome records) and the derived index |
| Capability overlay: backlogit | pass | Query-first through MCP. The index is refreshed once before trusting a mismatch. No task state is kept outside the backlog |
| IX. Git-Friendly Persistence | pass | Markdown and YAML only |
| X. Agent Context Efficiency | pass | The procedure reads frontmatter and the log tail only |
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
| U1, U3, U4 | none (tests) | Targeted `go test`, then the full suite | PR test results |
| U2 | Orchestrator dispatch | The first post-merge dispatch records workspace-relative verification and binding evidence. Ship passes Step 4.0 without a missing-root halt | Dispatch memory entry |
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
* `.github/instructions/constitution.instructions.md`.

### Protected invariants

1. Ship never infers a served root. The Orchestrator supplies both roots, or Ship is not
   invoked.
2. Root resolution is read-only, except for at most one index sync, which reconciles
   derived state only, and the R8 outcome record in `docs/memory/`. It writes no checkpoint.
3. Shipment membership is flat and explicit. Only explicit task members are considered.
4. Only governed ShipShipment completes or archives an explicit feature member.
5. Post-close requires a valid archive record for every explicit member.
6. No production Go change.

### Risky actions

| ProposedAction | ActionRisk | Approval | Expected ActionResult / rollback state |
|---|---|---|---|
| PA-1: Orchestrator procedure and three call sites (U2) | medium | PR review | Ship is invoked with bound roots, or the run halts with `SERVED_ROOTS_UNRESOLVED`. Revert the PR |
| PA-2: Reconcile acceptance and Ship Step 6 wording (U5) | medium | PR review, after U3 and U4 are GREEN | Pre-close proceeds only for a governed-pending feature. Revert the PR |
| PA-3: Manifest drift records (U6) | low | none | Three entries change. Revert the PR |
| PA-4: Bootstrap dispatch applying the plan | medium | Explicit operator authorization, or a dark-mode scope naming the shipment | Ship receives bound roots, or the dispatch halts |

### Added verification

* U2 manual dry-run in the PR body, in workspace-relative form.
* U4 preconditions are asserted with `require`.
* `go test -timeout=30m ./...` runs locally because of `B3701713`.
* Closure checks for this shipment: merge commit present, skill literal present, binary
  ancestry.

### Closure, monitoring, and rollback

* **Signals:**
  * the first post-merge dispatch's verification record
  * this shipment's own pre-close, safe-close, and post-close reports
* **Rollback triggers:**
  * a missing-root `WAVE_CLAIM_STATE_INDETERMINATE` after merge
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
