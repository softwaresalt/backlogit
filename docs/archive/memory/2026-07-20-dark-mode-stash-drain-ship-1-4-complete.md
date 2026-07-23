---
doc_type: memory
docline:
  date: 2026-07-20
  status: active
  tags: [dark-mode, orchestrator, stash-drain, ship, closure, provenance, copilot-review, 112-F, 113-F, 114-F, 115-F]
schema_version: "1.0"
title: DARK_MODE Stash Drain — SHIP-1..4 Shipped + Provenance Canonicalized
---

# DARK_MODE Pipeline Run — SHIP-1..4 Complete

**Outcome:** All 4 declared DARK_MODE shipments **shipped and merged** with
full multi-persona adversarial review, patient multi-round Copilot review
resolution, and canonical stash provenance across every shipment. Operator was
AFK; PR/merge pre-authorized; admin fallback NOT authorized.

## Shipments (all MERGED, merge-commit only per P-009)

| Ship | Feature | PR | Merge commit | Source stash |
|---|---|---|---|---|
| SHIP-1 | 112-F frontmatter serializer consolidation | #267 | `7f557db` | 12B5649E, 7EEADCD3, 80DD65C4 |
| SHIP-2 | 113-F ship-gate descope predicate | #268 | `f88faa3` | A3C349DD |
| SHIP-3 | 114-F size-composition CLI/MCP parity + perf | #269 | `c2b82a7` | 387DE4BF, D5FA1EE9, 47ED88ED |
| SHIP-4 | 115-F impl-plan Constitution Check section | #270 | `4b1bd81` | CA877CD1 |

## SHIP-4 detail (this session)

- Added `#### Constitution Check (REQUIRED)` section to
  `.github/skills/impl-plan/SKILL.md` (Phase 3, before Plan Hardening Signals) +
  a Quality Criteria bullet.
- 3-persona parallel adversarial review (cross-model): Constitution Reviewer
  (gpt-5.6-terra), Template Integrity Reviewer (gemini-3.1-pro), rubber-duck
  (claude-sonnet-4.6). Adjudicated: split III+IV bullet, softened
  "blocks plan-review" to semantic flagging, added N/A→verdict rollup, pure-docs
  coverage. Follow-up stash `0C419DA8` created for out-of-scope structural
  enforcement.
- **Copilot review: 4 rounds**, each surfacing a deeper governance-accuracy issue
  (findings converged 2→2→1→0):
  - r1 (`f6c59fd`): pipe placeholder + "written exactly as shown" contradiction →
    spelled out two explicit verdict lines (`dbdccb7`)
  - r2 (`dbdccb7`): `documented-deviations` must not accept NON-NEGOTIABLE
    deviations; 115-F provenance gap → added NON-NEGOTIABLE bar + `source_stash_id`
    (`1b6f5ad`)
  - r3 (`1b6f5ad`): MUST violations require self-correction, not documented
    pass-through → reserved documented deviations for SHOULD-level only (`3e56403`)
  - r4 (`3e56403`): clean
- Merged #270 (`4b1bd81`, 2 parents `c2b82a7`+`3e56403`).
- Closure: 115.001-T→done, 115-F→archived (cascaded task). Post-merge closure
  required its own PR #271 (`2c5f0c4`) because `main` is protected
  (branch-protection: PR + 3 required checks).

## Provenance canonicalization (constitution-accuracy dogfooding)

Copilot's SHIP-4 r2 exposed that manually-created features (not harvested via
`harvest_stash`) leave an **abbreviated** harvest record. Investigating the
backlog surfaced the same gap in SHIP-2. Fixed both to canonical form:

- **SHIP-2** (PR #272, `da570766`): A3C349DD was still `active` despite 113-F
  merged. Backfilled `custom_fields.source_stash_id/kind/text/priority/path` on
  113-F + archive record `reason: harvested` + `harvested_artifact_id: 113-F`.
  Copilot round 1 flagged missing kind/text and the abbreviated archive record;
  round 2 clean.
- **SHIP-4** (PR #273, `ccfb4ce9`): same canonical backfill for CA877CD1 → 115-F.
- Verified: all 8 shipment source stashes now `harvested` + linked in the DB;
  SHIP-1/SHIP-3 stashes were already canonical (harvested via `harvest_stash`).

## Follow-up stash entries (deferred to operator — out of DARK_MODE scope)

Self-generated during this run; NOT drained (draining self-generated scope would
violate DARK_MODE bounded-scope discipline and risk non-termination):

- `60336CC0` (medium, feature) — size_composition parity for list surfaces
- `A6A1B47E` (medium, task) — batched size-composition rollup API to remove N+1
- `7063A9F4` (low, task) — document computed-on-read staleness window
- `0C419DA8` (medium, task) — structural Constitution Check gate in plan-review +
  constitution-reviewer persona I–IX → add X, XI

## Excluded stash (unsafe / needs operator — correctly NOT implemented)

- `7F0A6E89` (low) — external autoharness repo write → Principle IV violation
- `8CD8F46A` (medium) — plan-review gate enable-vs-waive → governance/waiver
- `131CEAE4` (low) — repo-wide fsync/durability redesign → high blast radius
- `9D5BB492` (low) — crash-window exactly-once → product-requirement-gated

## Key techniques (see compound learning)

- §1.9 readiness GraphQL gate: latest copilot review `commit.oid == headRefOid`
  AND no pending `copilot-pull-request-reviewer` request AND zero unresolved
  copilot threads. GraphQL login = `copilot-pull-request-reviewer` (no `[bot]`);
  REST request POST uses `copilot-pull-request-reviewer[bot]`.
- Copilot cadence: re-request → wait ~185s → poll. One review per HEAD.
- Manual-harvest provenance backfill: write all 5 `source_stash_*` custom_fields
  the canonical path writes (`internal/core/stash.go:344-349`) + set archive
  record `reason: harvested` + `harvested_artifact_id` + `sync`.

## Next steps

- Operator to prioritize the 4 follow-up stash entries in a future cycle.
- Operator to decide the 4 excluded items (external write, governance waiver,
  durability redesign, exactly-once semantics).
