---
doc_type: memory
title: "Stage pipeline session — 108-F size estimation promoted to queued shipment 099-S"
date: 2026-07-18
agent: stage
feature: 108-F
shipment: 099-S
status: complete
---

# Session Memory — 108-F Size Estimation → Queued Shipment 099-S

## Outcome

Ran the Stage stash-to-backlog pipeline end to end for feature **108-F**
("Size estimation for feature and shipment implementation"). Produced a
**queued** shipment **099-S** containing 108-F plus 10 bounded, width-isolated
implementation tasks (originally 9; SE-7 later split into SE-7a/SE-7b per Copilot
F1 — see the Copilot PR #259 addendum below). No source/test/config code modified;
no git; no build/test/lint; shipment left in `queued` (not claimed/shipped).

## Pipeline stages completed

1. **impl-plan** — Wrote `docs/exec-plans/2026-07-18-108-F-size-estimation-impl-plan.md`
   grounded in the PROCEED spike decision (D1–D6), Model A (`docline.backlogit.*`
   under the docline map; base top level stays `additionalProperties:false`), and
   the codec reality (generic artifact codec drops top-level `docline` on
   `.backlogit` artifacts → requires an artifact-codec bridge + executable
   round-trip durability test). Includes a `## Constitution Check` section.
2. **plan-harden** — Added Protected invariants, PA-1..PA-4 proposed-action table,
   deepened runtime verification, operational closure, and unresolved operator
   decisions (schema/codec/round-trip/migration risk).
3. **plan-review** — Genuine multi-persona cross-model review. Gate **PASS**
   after 3 review-fix cycles (hit the cycle limit exactly). Journey: initial
   honest FAIL (3 FAIL/2 ADVISORY, 4 converging P1s) → revised → cycle 1 (Security
   PASS, Go ADVISORY, Arch FAIL 3 P1) → cycle 2 (Arch FAIL, 1 blocker) → cycle 3
   (Arch PASS). Standalone artifact:
   `docs/reviews/2026-07-18-108-F-size-estimation-plan-review.md`.
4. **harvest** — Re-scoped 4 placeholder tasks + created 5 new tasks (9 total),
   wired 11 `blocks` edges, moved all to `queued`.
5. **shipment** — Created queued shipment **099-S** with 10 members.

## Task-ID → implementation-unit mapping (system-assigned IDs)

| Task ID | Unit | Scope |
|---|---|---|
| 108.001-T | SE-1 | Define size + provenance schema (config) |
| 108.005-T | SE-2 | Artifact-codec bridge + round-trip durability (core codec) |
| 108.002-T | SE-3a | Persist size provenance + estimate-history event (core) |
| 108.006-T | SE-3b | Crash-safe append+write reconciliation (core persistence) |
| 108.003-T | SE-4 | Computed-on-read size composition rollups (core) |
| 108.007-T | SE-5 | CLI/MCP mutation parity (cli+mcp) |
| 108.008-T | SE-6 | CLI/MCP read projection parity (cli+mcp) |
| 108.009-T | SE-7 | Two-layer workspace containment (core+config) |
| 108.004-T | SE-8 | Document sizing contract (docs-only) |

## Final dependency graph (11 blocks edges; item depends on depends_on)

- 108.005 ← 108.001  (SE-2 ← SE-1)
- 108.003 ← 108.001  (SE-4 ← SE-1)
- 108.002 ← 108.005  (SE-3a ← SE-2)
- 108.002 ← 108.009  (SE-3a ← SE-7)
- 108.006 ← 108.002  (SE-3b ← SE-3a)
- 108.007 ← 108.006  (SE-5 ← SE-3b)
- 108.008 ← 108.002  (SE-6 ← SE-3a)
- 108.008 ← 108.003  (SE-6 ← SE-4)
- 108.004 ← 108.007  (SE-8 ← SE-5)
- 108.004 ← 108.008  (SE-8 ← SE-6)
- 108.004 ← 108.009  (SE-8 ← SE-7)

## Key design decisions carried in the plan

- SE-3 split into SE-3a (persist + event-before-write, fail-closed) and SE-3b
  (crash-safety via op-id + PrevOpID predecessor chain; doctor compare-and-swap;
  NO shared-JSONL truncation).
- SE-2 create+update single-writer enforcement + canonical `ToFrontmatterMap`
  projection (folds the stash-flagged consolidation to reduce write-path drift).
- Transport-aware actor stamping: MCP/agent rejects `size_source:human`; CLI
  stamps human.
- Typed `SizeMutation{Size, Source *string, RulesetVersion *string, Actor
  ActorContext, OpID *string}`.

## Residual / deferred (non-blocking)

- P2 (Go): test-envelope pressure — mitigated by "table-driven t.Run subtests
  count as one scenario".
- P3 (Security): shell-granted agent can bypass MCP stamping via direct CLI —
  noted in SE-8 doc.
- P3 (Arch): exact JSON key names / doctor verbosity are Ship-level detail.
- Deferred SE-6 CLI human-column parity → separate P2 backlog follow-up (cosmetic).

## Gotchas / notes for next agent

- **blocked→queued is not a valid transition** (`internal/hooks/builtin_pre.go`
  `DefaultTransitions()`). Re-scoping the 4 blocked placeholders to `queued`
  required hand-editing the `.backlogit/queue/*.md` `status:` field then
  `backlogit sync` (sanctioned by task instructions). New `backlogit add` tasks
  default to `queued`.
- **Pre-existing placeholder deps**: the original 108.001–004 placeholders
  carried 3 stale edges (108.002→108.001, 108.004→108.002, 108.004→108.003) that
  surfaced after shipment creation. Removed them via `dep remove` so the final
  edge set matches the reviewed graph exactly (11 edges).
- Hook-queue lock contention on parallel mutations is benign (swallowed post-hook
  WARN); used sequential CLI calls with small sleeps.
- `backlogit get-metadata-catalog` is MCP-only (not a CLI command).

## Files for the Orchestrator to commit

- `docs/exec-plans/2026-07-18-108-F-size-estimation-impl-plan.md` (new)
- `docs/reviews/2026-07-18-108-F-size-estimation-plan-review.md` (new)
- `docs/memory/2026-07-18/108-F-size-estimation-stage-pipeline-memory.md` (new, this file)
- `.backlogit/queue/108.001-T.md` … `108.004-T.md` (re-scoped; hand-edited status)
- `.backlogit/queue/108.005-T.md` … `108.009-T.md` (new tasks)
- `.backlogit/` index/artifact files touched by `backlogit sync`, `add`, `dep add/remove`,
  and `shipment create` (incl. shipment 099-S artifact + item_deps).

## Next steps (Ship agent territory — NOT this session)

- Claim shipment 099-S, generate harness (harness-architect), build-feature loop
  from SE-1 outward following the dependency graph, review, PR, closure.

## Reconciliation Addendum (2026-07-18, post-shipment)

Operator flagged an incorrect premise in the original brief: "size requires an
artifact-codec bridge (docline carrier on models.Artifact) proven by a round-trip
test." VERIFIED WRONG against source:

- `docs/decisions/2026-07-18-size-extension-contract-architecture-spike.md`
  §9(d)/(e) + Recommendation 2: SELECTED `custom_fields.size`; REJECTED any
  models.Artifact docline carrier. docline.backlogit.* is for docline DOCUMENTS,
  not .backlogit artifact frontmatter.
- `internal/core/docline_codec_roundtrip_test.go`: 3 already-passing guards prove
  the generic codec drops top-level docline while custom_fields round-trips. NO
  bridge to build.

Actions taken (Stage-role, no code/git):
- SE-2 (108.005-T) COLLAPSED (not removed) → test-only round-trip guard extending
  docline_codec_roundtrip_test.go to feature/shipment size + size_source +
  size_ruleset_version under custom_fields. Retitled. Now a leaf depending only on
  SE-1; nothing depends on it.
- Write-path single-writer integrity (reject/strip on create, merge-not-replace on
  update) MOVED to SE-3a (108.002-T); files now include internal/core/artifacts.go.
- ToFrontmatterMap two-emitter consolidation DESCOPED (optional, not required).
- Dependency rewire: removed 108.002→108.005 (persist→bridge); added 108.002→108.001
  (persist→schema). Also removed 3 stale placeholder edges. Final: 11 blocks edges,
  acyclic.
- 108.001-T description made explicit: custom_fields.size/size_source/size_ruleset_version
  (recognized carrier), NOT docline.backlogit on artifacts.
- Plan doc + review artifact updated with reconciliation notes; both lint clean.

Gate unchanged: PASS (scope narrowed, no P0/P1 reopened). 099-S remains queued.

## Final dependency graph (post-reconciliation, 11 blocks edges)

- 108.002 ← 108.001  (SE-3a ← SE-1)   [NEW, replaces ←108.005]
- 108.002 ← 108.009  (SE-3a ← SE-7)
- 108.003 ← 108.001  (SE-4 ← SE-1)
- 108.005 ← 108.001  (SE-2 guard ← SE-1)  [leaf; no dependents]
- 108.006 ← 108.002  (SE-3b ← SE-3a)
- 108.007 ← 108.006  (SE-5 ← SE-3b)
- 108.008 ← 108.002  (SE-6 ← SE-3a)
- 108.008 ← 108.003  (SE-6 ← SE-4)
- 108.004 ← 108.007  (SE-8 ← SE-5)
- 108.004 ← 108.008  (SE-8 ← SE-6)
- 108.004 ← 108.009  (SE-8 ← SE-7)

## Copilot PR #259 Reconciliation Addendum (2026-07-18)

External Copilot review on staging PR #259 returned **7 valid findings (F1–F7)**
against the 099-S planning artifacts. All were source-verified (engram-indexed) and
accepted; reconciliation stayed within planning/backlog artifacts (no source/test/
config, no git). 099-S remains **queued**; gate **PASS** holds (no P0/P1 reopened).

**Correcting the "all width-isolated" claim above:** the Outcome/task-mapping and
dependency-graph sections earlier in this file are **pre-F1**. After the SE-7 split
the shipment has **10 tasks, all single-domain** — the earlier "9 tasks" count and
the SE-7 "core+config" mapping row are superseded by this addendum.

Finding dispositions:

- **F1 (P1) — Width Isolation.** SE-7 combined config + core containment. **SPLIT:**
  `108.009-T` → **SE-7a (config-load, config)**; new **`108.010-T`** → **SE-7b
  (lookup-time, core)**; wired `SE-7a → SE-7b`. 099-S manifest grew to **11 members**
  (108-F + 108.001-T…108.010-T). Plan task table, Constitution Check, and this memory
  updated. All ten tasks are now width-isolated with no declared deviation.
- **F2 (P1) — Non-buildable intermediate.** SE-3a (`108.002-T`) now adds
  `SetArtifactSizeWithProvenance` and keeps `SetArtifactSize` as a thin compat
  wrapper until SE-5 (`108.007-T`) migrates callers and removes it.
- **F3 (P1) — SE-2 not red.** SE-2 (`108.005-T`) reframed as an **expected-green
  characterization/round-trip guard**; its SE-3a dependency removed (depends only on
  SE-1). Constitution Check II documents SE-2 as characterization, not a skipped red.
- **F4 (P1) — Retry idempotency.** SE-3b (`108.006-T`) adds a pre-append OpID
  orphan-check: reconcile the pending write instead of re-appending a duplicate; new
  retry-from-orphan test.
- **F5 (P1) — Doctor recovery.** SE-3b event payload now pins full presence-aware
  desired state (size/size_source/size_ruleset_version) with SET/CLEAR semantics; new
  CLEAR-a-field recovery test.
- **F6 (P2) — Unowned follow-up.** Filed stash **`D5FA1EE9`** (kind task, priority
  low) for deferred CLI human-column parity; plan §328 and `108.008-T` name it.
- **F7 (P2) — Atomicity overstated.** Plan D4 summary restated to the actual hard
  invariant (append-then-write; crash-safe reconcile/dedup on retry; log is source of
  truth). No two-way atomicity claim.

**Updated task-ID → unit mapping (post-F1):**

| Task ID | Unit | Domain |
|---|---|---|
| 108.001-T | SE-1 | config (schema) |
| 108.002-T | SE-3a | core (persist + event + sole-writer; compat wrapper) |
| 108.003-T | SE-4 | core (composition rollups) |
| 108.004-T | SE-8 | docs |
| 108.005-T | SE-2 | test (expected-green round-trip guard; leaf) |
| 108.006-T | SE-3b | core (crash-safety; orphan dedup; SET/CLEAR payload) |
| 108.007-T | SE-5 | cli+mcp (mutation parity; removes compat wrapper) |
| 108.008-T | SE-6 | cli+mcp (read parity; CLI human cols → stash D5FA1EE9) |
| 108.009-T | SE-7a | config (config-load containment) |
| 108.010-T | SE-7b | core (lookup-time containment) |

**Final dependency graph (post-F1, 12 blocks edges, acyclic; item ← depends_on):**

- 108.002 ← 108.001  (SE-3a ← SE-1)
- 108.002 ← 108.010  (SE-3a ← SE-7b)   [NEW; replaces ←108.009]
- 108.003 ← 108.001  (SE-4 ← SE-1)
- 108.004 ← 108.007  (SE-8 ← SE-5)
- 108.004 ← 108.008  (SE-8 ← SE-6)
- 108.004 ← 108.010  (SE-8 ← SE-7b)    [NEW; replaces ←108.009]
- 108.005 ← 108.001  (SE-2 guard ← SE-1)  [leaf; no dependents]
- 108.006 ← 108.002  (SE-3b ← SE-3a)
- 108.007 ← 108.006  (SE-5 ← SE-3b)
- 108.008 ← 108.002  (SE-6 ← SE-3a)
- 108.008 ← 108.003  (SE-6 ← SE-4)
- 108.010 ← 108.009  (SE-7b ← SE-7a)   [NEW]

Roots: 108.001 (SE-1), 108.009 (SE-7a). Leaf: 108.005 (SE-2). Removed edges:
108.002 ← 108.009 and 108.004 ← 108.009 (repointed to 108.010 / SE-7b).

**Residual (P2, non-blocking):** SE-3b (`108.006-T`) now sits at/over the 2-hour
envelope (orphan-retry + CLEAR-recovery scenarios + full payload); split the doctor
CLEAR-recovery reconcile into a sibling task during Ship if it exceeds one envelope.

**Review posture:** the Copilot-findings pass was a **single-agent inline
re-assessment** of the deltas (no fresh multi-persona dispatch); each finding was
independently source-verified before acceptance. Recorded in the plan-review
addendum.
