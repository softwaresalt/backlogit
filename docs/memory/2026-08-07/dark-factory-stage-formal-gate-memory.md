# Stage session memory — dark-factory staging cycle (formal gate + workspace rename + 054 lifecycle)

- **Date:** 2026-08-07
- **Agent:** Stage (depth 1), dark factory mode (P-017)
- **Worktree:** `.copilot/session-state/ecebe820-92d7-4253-852a-0c3c23f8aea9/files/dark-factory-worktree`
- **Branch:** `admin/dark-stage-formal-gate` (Stage planning/admin branch, based on `origin/main` @ `fd8d2c9d`)

## DARK_MODE_ACTIVE

Operator explicitly authorized a full-pipeline dark factory run. Activation record:

| Field | Value |
|---|---|
| `DARK_MODE_ACTIVE` | `true` |
| `activation_trigger` | Operator dark-factory staging directive (explicit, this session) |
| `visibility_mode` | **degraded** — `agent-intercom` MCP surface unavailable |
| `merge_approval_pre_authorized` | `true`, for every in-scope staging / implementation / closure PR |
| `admin_fallback_pre_authorized` | `true`, **only** for confidently classified `REVIEW_REQUIRED_BLOCK` / `CONVERSATION_RESOLUTION_BLOCK` |
| `merge_strategy` | merge commit only (P-009) |
| `worktree_topology` | one linked planning worktree, sequential reuse by Ship (P-016) |

Merge pre-authorization remains **subject to** current-HEAD local review readiness
(§1.9), green required CI, P-009 merge commits, the P-014 Copilot gate, and P-016
topology. Admin fallback is **forbidden** for checks, merge strategy, stale review,
unresolved P0/P1, secrets exposure, scope mismatch, and unknown blocks.

### Stop conditions (armed)

Scope expansion or ambiguity; missing current-HEAD readiness; unresolved P0/P1;
failed / pending / missing required checks; P-016 violation; destructive action
outside the contract; secret exposure; unavailable required tool without a declared
fallback; ambiguous branch-protection or admin state; any P-005.

## DARK_MODE_SCOPE

Strict operator-selected order:

1. `106-F` F-series remainder — **F1**, then **F4**, then **F6**, then **F5**
   (F2 = `106.001-T` and F3 = `106.002-T` already shipped in `114-S`; reconcile only)
2. active stash `9370A18C` — default workspace install directory `.backlogit` → `.backlog`
3. review artifact `054.001-R` — lifecycle disposition only

Explicitly **excluded**: any other queue item, any other stash entry, any
implementation work, any build/test/lint execution, any PR creation or merge.

### Operator priority policy applied

- reliability and security **>** feature **>** documentation
- simplicity **>** complexity
- composability / interoperability **>** feature
- type ordering: bug, review, feature, task, spike — adjusted by product-outcome importance

## Step 0.0 — Tool availability gate (P-012)

| Surface | Status | Note |
|---|---|---|
| `backlogit` MCP (read-only) | `TOOL_OK` | `get_metadata_catalog`, `query_sql`, `get_item`, `list_shipments` all responded |
| `backlogit` MCP (mutations) | `TOOL_DEGRADED` | MCP `workspace.root_path` resolves to the **primary dirty worktree**. All mutations routed through the CLI inside the linked worktree instead, per the workspace-isolation contract. |
| `backlogit` CLI | `TOOL_OK` | `C:\Tools\backlogit.exe` v1.8.0 (pinned release binary, at/after the 133-F P-015 fix) |
| `agent-intercom` MCP | `TOOL_UNAVAILABLE` | Operator-declared. **Remote operator visibility is degraded** — all dark-mode events recorded here and in the session summary instead of broadcast. |
| `agent-engram` MCP | `TOOL_UNAVAILABLE` | Operator-declared. Declared fallback used: `grep` / `glob` / `view` + `backlogit_query_sql`. |

Overall: `DEGRADED_MODE: agent-intercom, agent-engram, backlogit-mcp-mutations`.

## Step 0.1 — Index sync

`INDEX_SYNC_OK` — `backlogit sync` in the linked worktree indexed 1006 artifacts.

## Concurrency / isolation

The primary worktree is dirty with unrelated operator changes
(`.backlogit/archive/stash.jsonl`, `.backlogit/stash.jsonl`,
`.github/agents/.ship.agent.md`, `.github/agents/.stage.agent.md`,
`.github/agents/_orchestrator.agent.md`, `docs/migration-guide.md`).
**None of those were modified, staged, stashed, restored, or committed.**

The two stash files were **read** from the primary worktree and copied forward into
the linked worktree so the linked worktree reflects the operator's current stash
reality (`9370A18C` active; `7F0A6E89` / `6FA0829B` already archived by the
operator). That is a read of the primary worktree and a write in the linked
worktree only.

Single-agent, single-branch inside the linked worktree → no per-file locks acquired
(per `concurrency.instructions.md`).

## Step 1 — Triage and classification

| Entry | Shape | Kind / priority | Routing |
|---|---|---|---|
| `106-F` F1 (evidence authenticity + manifest binding) | feature-shaped sub-unit | security / high | deliberate (micro-decision) → impl-plan → plan-harden → plan-review → harvest |
| `106-F` F4 (durable dependency type) | feature-shaped sub-unit | reliability / high | impl-plan → plan-review → harvest |
| `106-F` F6 (governed-op CLI/MCP parity) | feature-shaped sub-unit | interoperability / medium | impl-plan → plan-review → harvest |
| `106-F` F5 (journaled/idempotent multi-mutation) | feature-shaped sub-unit | reliability / high, high blast radius | deliberate (micro-decision) → impl-plan → plan-harden → plan-review → harvest |
| stash `9370A18C` | feature-shaped | feature / medium | deliberate (compatibility) → impl-plan → plan-harden → plan-review → harvest |
| `054.001-R` | review artifact, terminal content | review | lifecycle disposition (archive) — no new feature work |

All six entries were operator-selected; none is ambiguous. No deferred entries this
session (the stash contains exactly one active entry).

## Step 1.5 — Contextual grouping analysis

Two or more task-shaped entries were **not** present (all F-series units and the
stash entry are feature-shaped), so grouping analysis ran in its reduced form:
the question was only whether the F-series units form one release unit or several.

### Grouping decision

| # | Release unit | Members | Coherence rationale | Risk |
|---|---|---|---|---|
| G1 | F1 — evidence authenticity + manifest binding | `internal/core/gate_evidence.go`, `internal/core/gate_transition.go`, `internal/gateevidence/`, `internal/core/shipment_gate.go`, `internal/config/` | Single security seam: authenticity proof + anti-replay + manifest binding + ship-time verification. Security-sensitive → its own release unit. | high |
| G2 | F4 — durable dependency type | `internal/models/{artifact,frontmatter}.go`, `internal/core/{artifacts,dependencies}.go`, `internal/db/{rehydration,dependencies}.go` | Single data-model/persistence seam with its own compatibility + migration story. | moderate |
| G3 | F6 — governed-op CLI/MCP parity | `internal/core/commits.go`, `internal/cli/update.go`, `internal/mcp/tools.go`, `.autoharness/backlog-registry.yaml` | Single parity seam: one shared commit-association core function + behavioral parity assertion. | low |
| G4 | F5 — idempotent multi-mutation envelope | `internal/core/{commits,shipment_lifecycle}.go`, `internal/db/*` tx helpers | Highest blast radius, depends on F1/F4/F6 landing first. Own final formal-gate release unit. | high |
| G5 | `9370A18C` — default workspace dir rename | `internal/core/workspace.go`, `internal/config/`, `internal/cli/{root,migrate,doctor}.go` + ~100 test files + docs | Potentially breaking default change touching workspace discovery and containment. Separate, after formal-gate work. | high |

**F4 + F6 were evaluated for grouping and explicitly kept separate.** The operator
authorized grouping them only if a reviewed plan showed the *same seams* and a
bounded release unit. Evidence says otherwise: F4 lives in the models/persistence/
rehydration seam and carries a frontmatter schema-compatibility story; F6 lives in
the commit-association/surface-parity seam and carries a behavioral-parity story.
The only overlap is the file `internal/mcp/tools.go`, and the dependency CLI/MCP
handlers there already route through shared core functions
(`internal/cli/dep.go:80-84,109-113` ↔ `internal/mcp/tools.go:1346-1349,1369-1372`),
so there is no shared seam to amortize. Merging them would produce one release unit
with two independent compatibility stories and >8 changed files — a violation of
`simplicity > complexity` and of artifact-class isolation.

### Ordering

`F1 → F4 → F6 → F5 → 9370A18C`, matching both the operator's declared order and
the spike's recommendation ("F1 next; F4/F6 in parallel; F5 last"). F4 before F6
because reliability/data-model outranks interoperability under the operator policy.
Ordering is encoded as explicit `blocks` dependency edges, not prose.

## Step 1.8 — Learnings retrieval

`learnings-researcher` returned **high** confidence for the four F-series topics and
**low** confidence for the workspace-rename topic (no direct prior art; adjacent
warnings only). Key inputs carried into deliberation and planning:

- `docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md` — the
  charter for F1/F4/F5/F6.
- `docs/compound/2026-07-07-empty-head-fail-closed-repo-presence-probe.md` — the
  valid / definitively-invalid / unverifiable trichotomy that ship-time proof
  verification must reuse.
- `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md` —
  "data must not choose the code"; the honest trust-boundary precedent for key
  material.
- `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` — the
  `WriteArtifactFile` explicit-key-enumeration drop mechanism that a typed
  dependency field must not fall into.
- `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
  — `ErrWriteNotApplied` vs `ErrWriteIndeterminate`; never roll back an
  indeterminate write.
- `docs/decisions/2026-07-23-crash-window-exactly-once-size-mutation-spike.md` —
  OpID / CAS exactly-once machinery already tried and descoped at the root.
- `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`
  — thread the caller's `*events.EventWriter`; never mint one inside shared core.

## Reconciliation findings (recorded before planning)

1. **F2 shipped.** `internal/canonical` exists with `Canonicalize(any) ([]byte, error)`
   and `Hash(any) (string, error)` plus sentinel errors, tested in
   `internal/canonical/canonical_test.go`. F1 consumes it; it is not re-created.
2. **F3 shipped.** `internal/core/status_taxonomy.go` exports
   `IsCascadeTerminalStatus`, `IsNoLongerBlockingStatus`, `CascadeTerminalStatuses`,
   `IsReleasableStatus`, `IsGateTargetStatus`. F1's formal-admission predicate builds
   on these; they are not re-created.
3. **The spike's Q6 Windows premise is now obsolete.** The spike asserted
   `atomicfile` used a Windows `os.Remove`-then-rename fallback. HEAD does not:
   `internal/atomicfile/atomicfile_windows.go` uses `MoveFileEx` with
   `MOVEFILE_REPLACE_EXISTING` unconditionally (plus `MOVEFILE_WRITE_THROUGH` when
   durable), and `internal/atomicfile/atomicfile_other.go` uses plain `os.Rename`.
   Single-file replacement **is** atomic on both platforms today. This materially
   shrinks F5 and is the primary input to the F5 recovery-contract deliberation.
4. **No cryptographic material exists in the repo today.** No `crypto/hmac`,
   `crypto/ed25519`, keyring, or credential storage under `internal/`. The only
   secret-adjacent precedent is `internal/config/loader.go:188-196`, which
   **rejects literal hook secrets and permits environment expansion only**. F1
   inherits that rule.
5. **Item JSONL logs carry no sequence field**, but `internal/events/hook_events.go:35-54,149`
   already implements a monotonic `Seq`. F1's anti-replay counter reuses that shape.
6. **`114-S` contained only `106.001-T` and `106.002-T` — not `106-F`.** That is the
   P-015-safe partial-feature shipment shape for this repository, and it is why
   `106-F` is still `active`. See "Shipment assembly deviation" below.

## Shipment assembly deviation (recorded, with rationale)

The Stage contract says a covering feature must be added to a shipment before its
child tasks. For the three **partial** formal-gate shipments (F1, F4, F6) the
covering feature `106-F` is deliberately **excluded**, matching the `114-S`
precedent. Post-`133-F`, `core.ShipShipment` gates covering-feature archival on
explicit shipment-manifest membership
(`docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`),
so including `106-F` in a partial shipment would archive the covering feature and
strand the remaining F-units. `106-F` is added to the **final** F-series shipment
(F5) only, where it is the correct release-unit closure and the contract invariant
is satisfied. This is the more conservative behavior under the capability-overlay
conflict rule.

## Steps 2–4 outcome (deliberation → plan → harden → review)

### Artifacts produced

| Artifact | Path |
|---|---|
| F1 micro-decision | `docs/decisions/2026-08-07-f1-evidence-authenticity-mechanism-deliberation.md` |
| F5 micro-decision | `docs/decisions/2026-08-07-f5-multi-mutation-recovery-contract-deliberation.md` |
| Rename compatibility decision | `docs/decisions/2026-08-07-workspace-default-dir-rename-deliberation.md` |
| F1 plan | `docs/exec-plans/2026-08-07-f1-evidence-authenticity-manifest-binding-plan.md` |
| F4 plan | `docs/exec-plans/2026-08-07-f4-durable-dependency-type-plan.md` |
| F6 plan | `docs/exec-plans/2026-08-07-f6-governed-op-cli-mcp-parity-plan.md` |
| F5 plan | `docs/exec-plans/2026-08-07-f5-idempotent-multi-mutation-envelope-plan.md` |
| Rename plan | `docs/exec-plans/2026-08-07-workspace-default-dir-rename-plan.md` |

### `BRAINSTORM_HANDOFF_READY`

All three deliberations reached `decision_status: decided` and were promoted to
plans. Unresolved questions carried forward as advisory follow-ups, none blocking:
external high-water ledger provisioning (F1, operations decision); key rotation
ergonomics beyond a MAC-bound `key_id` (F1); automatic `doctor` repair for partial
mutations (F5); whether the envelope later covers the archive path (F5); whether
and when `.backlogit` support is deprecated (rename); whether this repository's own
backlog directory is ever renamed (rename, explicitly out of scope). Handoff target:
`impl-plan` → `plan-harden` → `plan-review` → `harvest`, all completed this session.

### P-006 plan hardening

Five of five plans declared `Requires plan hardening: yes` and carry a
`## Plan Hardening` section with protected invariants, consulted learnings,
`ProposedAction` / `ActionRisk` / `ActionResult` tables, deepened verification and
rollback, and unresolved operator decisions. F6 initially declared `no` while
answering `yes` to a contract signal; that was corrected during review remediation.

### Plan review gate

`dispatch_mode: multi-agent-dispatch` — seven cross-model personas dispatched
against all five plans: Constitution Reviewer (`claude-opus-4.6`), Scope Boundary
Auditor (`gpt-5.6-terra`), Security Lens Reviewer (`gpt-5.6-sol`), Architecture
Strategist (`gemini-3.1-pro-preview`), Go Reviewer (`claude-sonnet-4.6`),
Agent-Native Parity Reviewer (`gpt-5.6-luna`), Learnings Researcher.

| Cycle | Decision | P0 | P1 | P2 | P3 |
|---|---|---|---|---|---|
| 1 | **FAIL** (F1, F4, F5, rename); ADVISORY (F6) | 0 | 18 | 12 | 9 |
| 2 (post-remediation) | **PASS** (all five) | 0 | 0 | — | — |

Every cycle-1 P1 and P2 was remediated in-plan; each plan records the findings and
their resolutions in its `## Plan Review` section with a `<!-- plan-review-attempt: 2 -->`
marker. Adversarial escalation criteria were met (3+ P0/P1 findings **and**
security-sensitive work), and the multi-model dispatch above satisfied that gate
directly rather than as a second pass.

Highest-value cycle-1 corrections:

- **F1** — the anti-replay claim was false because the counter floor derives from
  the attacker-writable log; the guarantee was narrowed honestly and an optional
  external high-water ledger added. Enforcement moved to an environment anchor
  outside the workspace. The key is now stripped from every child-process
  environment (the git helper builds its env from `os.Environ()`). The envelope
  gained `magic` / `purpose` / `schema` / `alg` / `key_id` / `workspace_id`
  **inside** the MAC. The speculative verifier interface was removed and the
  `(Outcome, error)` dual return collapsed to sentinels.
- **F4** — the model type change was implied but never declared; `DependencyEdge`,
  the `{id, type}` YAML shape, and the full call-site list are now explicit, and
  validation moved to the load edge.
- **F6** — `AssociateCommit` was monolithic, which would have forced a rewrite in
  F5; it is now specified as discrete idempotent steps. The registry `governed`
  marker moved **before** the assertion that consumes it, and an empty governed set
  is now a test failure.
- **F5** — the envelope no longer takes `*events.EventWriter`; `MutationPartialError`
  is a typed struct; indeterminate now dominates the joined classification.
- **Rename** — the override became a **closed set**, killing the traversal class;
  symlink/reparse rejection and realpath containment added; the "everything derives
  from `WorkspaceStorageRoot`" premise was proven false and replaced with a
  safety-path inventory plus a literal guard; MCP **production** path resolution
  replaced a discovery-only test.

## Step 5 — harvest

38 work items created, all `queued`, all ≤2 hours, each with acceptance criteria
and a single skill domain.

| Release unit | Items |
|---|---|
| F1 | `106.003-T` … `106.011-T` (9) |
| F4 | `106.012-T` … `106.018-T` (7) |
| F6 | `106.019-T` … `106.024-T` (6) |
| F5 | `106.025-T` … `106.031-T` (7) |
| Rename | `135-F` (new covering feature) + `135.001-T` … `135.009-T` (9) |

38 explicit `blocks` dependency edges wired — internal chains within each unit plus
cross-unit edges enforcing the operator-mandated order
`F1 → F4 → F6 → F5 → 9370A18C`, and one semantic cross-edge `106.027-T ← 106.021-T`
because F5's commit-association wrap needs F6's routed `AssociateCommit`.

## Step 5.5 — shipment assembly

| Order | Shipment | Priority | Members |
|---|---|---|---|
| 1 | `117-S` Formal gate F1 — evidence authenticity and manifest binding | high | 9 tasks |
| 2 | `118-S` Formal gate F4 — durable dependency type persistence | high | 7 tasks |
| 3 | `119-S` Formal gate F6 — governed-operation CLI and MCP parity | medium | 6 tasks |
| 4 | `120-S` Formal gate F5 — idempotent multi-mutation envelope (final F-series) | high | `106-F` + 7 tasks |
| 5 | `121-S` Default workspace directory rename to `.backlog` | medium | `135-F` + 9 tasks |

All five are `queued`; none is active or claimed. Manifests verified by read-back.

## Step 5.6 — consumed stash archival

`9370A18C` archived **after** successful harvest and shipment assembly. No other
stash entry was touched. No entries deferred — the stash is now empty.

## `054.001-R` lifecycle disposition

The artifact records `Gate: PASS`, P0=0, P1=0, one P2 fixed in commit `0474d30`,
five P3 advisories (all false positives or design choices), and
`Residual Work: None blocking`. Its parent `054-F` is `done` and archived, and
shipment `054-S` is archived — the review artifact was simply left behind in the
queue.

**Action taken: minimal lifecycle cleanup only.** Moved `review` → `accepted`, then
archived to `.backlogit/archive/`. No feature work was created. The one residual
product observation (`ModelGroup` intentionally omits `avg_tokens_per_session`) is
recorded here as an **advisory follow-up only**; it is not a blocking gap and was
deliberately not harvested, per the operator's no-scope-expansion instruction.

## Pre-existing condition (not in scope, not actioned)

`backlogit doctor` reports one orphan, unchanged from session start:
`[orphaned_artifact] 016.001-R` — a review artifact with no `parent_id` and no
`returned_to_backlog` event. It is outside `DARK_MODE_SCOPE` and was deliberately
left alone.

## Files changed in this session

All changes are confined to the linked planning worktree on
`admin/dark-stage-formal-gate`. The primary worktree was never modified.

Commits on the branch, in order:

| Commit | Scope |
|---|---|
| `9bff354e` | `docs(docs)` — three deliberations and five reviewed plans |
| `e6ba6ffe` | `chore(harness)` — 38 harvested items, 38 dependency edges, five queued shipments (also carries the `054.001-R` removal from `queue/`) |
| `f572e19c` | `chore(harness)` — stash `9370A18C` archived, `054.001-R` archived |
| `<memory>` | `docs(docs)` — this session memory artifact |

Note: the `054.001-R` move is split across `e6ba6ffe` (queue removal, staged with
the queue directory) and `f572e19c` (archive addition). Both commits are internally
coherent and the artifact exists in exactly one place at branch tip.

File-level detail:
- `.backlogit/stash.jsonl`, `.backlogit/archive/stash.jsonl` — operator stash state
  carried forward, then `9370A18C` archived after harvest
- `.backlogit/queue/106-F.md` — dependency edge added by the harvest
- `.backlogit/queue/106.003-T.md` … `106.031-T.md` — 29 new F-series tasks
- `.backlogit/queue/135-F.md`, `135.001-T.md` … `135.009-T.md` — new rename feature
  and 9 tasks
- `.backlogit/queue/117-S.md`, `118-S.md`, `119-S.md`, `120-S.md`, `121-S.md` —
  five queued shipments
- `.backlogit/archive/054.001-R-053-s-model-aware-telemetry-branch-review.md` —
  archived review artifact (moved out of `queue/`)
- `.backlogit/hooks_queue.jsonl` — tool-managed append-only hook event stream
- `docs/decisions/2026-08-07-*.md` — three deliberation artifacts
- `docs/exec-plans/2026-08-07-*.md` — five reviewed implementation plans
- `docs/memory/2026-08-07/dark-factory-stage-formal-gate-memory.md` — this file

## `DARK_MODE_COMPLETE` (Stage half)

Stage's half of the dark-mode scope is complete. Five queued shipments in strict
dependency order are ready for Ship. Stage performed **no** source, test, or config
mutation, ran **no** build, test, or lint, and created, pushed, or merged **no** pull
request. The branch carries only Stage-owned planning, backlog, and memory artifacts
and is ready for the orchestrator's staging PR merge gate.

## Handoff to the orchestrator / Ship

1. The linked worktree is clean and must be **reused sequentially** by Ship to
   preserve P-016 — do not create a second worktree.
2. Ship claims `117-S` first, then `118-S`, `119-S`, `120-S`, `121-S` in that order.
   The dependency graph enforces the order independently of prose.
3. `120-S` includes covering feature `106-F` deliberately; `117-S`, `118-S`, and
   `119-S` deliberately exclude it. Do not "fix" that.
4. Destructive and approval-gated actions are already classified in each plan's
   Risky Actions table. `135.005-T` (workspace directory move) is
   `ActionRisk: destructive` and requires explicit operator approval with a
   reviewed `--dry-run` first.
5. Remote operator visibility is degraded (`agent-intercom` unavailable). All
   dark-mode events are recorded here rather than broadcast.
