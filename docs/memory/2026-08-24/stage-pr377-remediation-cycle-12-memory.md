---
doc_type: memory
schema_version: "1.0"
title: PR #377 Copilot Review Remediation — Cycle 12
---

## Cycle 12 — Exact-Duplicate Unmodeled Key Safety

**PR**: #377 (`chore/stage-130-s`)
**HEAD at cycle start**: `7d6fe805c7ea7f2fdb7aa725918b119f061860ab`
**Operator authorization**: extension past three-cycle limit (continuation of established pattern)

### Root Cause

Copilot review raised two duplicate comments:

* `PRRT_kwDORzozKM6b7A4p` / comment 3849215481 on `.backlogit/queue/147.018-T.md`
* `PRRT_kwDORzozKM6b7A40` / comment 3849215502 on the authoritative plan

Both flagged the same root: the repair table's all-unmodeled duplicate row (`duplicate:<key>` where both members are unmodeled) treated every pair as safely round-trippable by moving both into `context.legacy_top_level`. Exact-duplicate raw key names (e.g., `{"foo":1,"foo":2}`) are lossy: `CheckpointContext.UnmarshalJSON` decodes `context` into `map[string]json.RawMessage`, so two entries with the same raw key collapse to one via Go's last-wins map insertion.

### Changes Applied

1. **Split the all-unmodeled duplicate row** into two rows in both `147.018-T.md` and the authoritative plan:
   * **Distinct raw spellings** (including case variants like `foo`/`Foo`): safe to move — each occupies a unique map key in `Extra`
   * **Exact same raw spelling**: NOT auto-repairable — requires explicit operator choice performed with a duplicate-preserving raw/token-aware method, or quarantine; never implicitly last-wins; even equal values must not be silently collapsed (structural duplication is information)

2. **Generalized exact-duplicate safety invariant** added after the table in both files: the no-implicit-survivor rule applies to exact duplicates regardless of modeled/unmodeled status, while preserving separate modeled-key conflict semantics and distinct-spelling unmodeled-key move semantics

3. **Plan cycle-1 remediation table** updated: the `PRRT_kwDORzozKM6b18IM` row now says "five-row" instead of "four-row" and names the new exact-duplicate-unmodeled class and the generalized invariant

4. **Task acceptance criteria** updated to reflect the split and the invariant

### Files Modified

* `.backlogit/queue/147.018-T.md` — repair table split, acceptance criteria updated, `updated_at` bumped
* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — repair table split, invariant added, cycle-1 remediation row updated
* `docs/memory/2026-08-24/stage-pr377-remediation-cycle-12-memory.md` — this file
* `.backlogit/checkpoints/checkpoint-20260824-191617.json` — review_remediation and memory_path updated

### Topology

Unchanged: 26 queued tasks / 42 queued-to-queued edges / 27 shipment members / sole ready 147.001-T.

### Memory Footprint

After adding this file: 37 files in `docs/memory/`. Below mandatory compaction thresholds (>40 files or >500 KB). No compaction required.

### Decisions

* Exact-duplicate unmodeled keys share the same underlying constraint as exact-duplicate modeled keys (map decode is lossy) — the no-implicit-survivor invariant is universal
* Distinct-spelling unmodeled duplicates remain safely movable (unique map keys)
* The generalized invariant does not change modeled-key semantics or distinct-spelling move semantics — it only makes the shared constraint explicit

### Residual Concerns

* GitHub PR #377 is authoritative for the current head, CI, reviews, and push state — query live PR before any merge-readiness claim
* Stage Role Boundary: PR push, reply, resolve, and merge remain forbidden
* Two new Copilot review threads need reply + resolution by Ship or operator after push
