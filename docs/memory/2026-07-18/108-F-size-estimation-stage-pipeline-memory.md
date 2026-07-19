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
**queued** shipment **099-S** containing 108-F plus 9 bounded, width-isolated
implementation tasks. No source/test/config code modified; no git; no
build/test/lint; shipment left in `queued` (not claimed/shipped).

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
