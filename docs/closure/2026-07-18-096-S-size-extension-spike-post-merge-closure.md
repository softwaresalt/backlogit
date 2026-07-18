---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 096-S — the size-extension contract architecture spike (feature 109-F + investigation tasks 109.001-007-T; PR #252, merge 86aa6ec). Read-only investigation: no production code changed, deliverable is the findings artifact plus a committed test-tier codec regression guard. Records the confirmed merge-commit (P-009), the shipment ship result (096-S shipped; 109-F and 109.001-007-T archived; pre/post shipment-reconcile both PROCEED), the DARK_MODE authorized merge, the eight Copilot review cycles and the two-model adversarial re-review that flipped the conclusion to proceed and hardened the exactly-once durability and lexical-containment findings, release-readiness SHIPPED with no monitoring and git-revert rollback for the zero-blast-radius spike, and the downstream proceed authorization that permits the 108-F blocked->active restaging in a later stage cycle.'
doc_type: closure
docline:
    ms.date: 2026-07-18T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-18T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-18-096-S-size-extension-spike-post-merge-closure.md
title: 096-S size-extension contract architecture spike — Post-Merge Operational Closure
---

# Operational Closure — 096-S size-extension contract architecture spike (post-merge)

- **Date**: 2026-07-18
- **Mode**: `post-merge`
- **Shipment**: `096-S` · Feature `109-F` · Tasks `109.001-T`–`109.007-T`
- **PR**: #252 (`feat/096-S-size-extension-spike` → `main`)
- **Merge commit**: `86aa6ec3d921bfa9dc2a0bb06a90c901857c2d6a`
- **Deliverable**: `docs/decisions/2026-07-18-size-extension-contract-architecture-spike.md`
- **Regression guard**: `internal/core/docline_codec_roundtrip_test.go` (3 tests)
- **Readiness**: **SHIPPED**
- **Autonomy**: `DARK_MODE_ACTIVE` (P-017), operator AFK, merge pre-authorized.

## Merge confirmation

- PR #252 `state: MERGED`, merged 2026-07-18T05:36:36Z.
- Merge method: **merge commit** (P-009 preserved). Repository settings confirm
  `allow_merge_commit: true`, `allow_squash_merge: false`, `allow_rebase_merge: false`.
- Pre-merge §1.9 readiness gate passed for the current HEAD `e845145`
  (no pending Copilot request; latest Copilot review covered `e845145`; zero
  unresolved Copilot threads); CI green (`test`, `Docline frontmatter gate`,
  `CLI Reference Drift`, `Detect code changes`).

## Shipment ship result

- `backlogit shipment ship 096-S` → `shipment_status: shipped`.
- Archived: `096-S`, `109-F`, `109.001-T`–`109.007-T` (nine IDs); `returned_ids: []`.
- Shipment-reconcile **PRE**: all eight manifest members `status: done` and present
  in `.backlogit/archive/`; shipment `096-S` active in queue → **PROCEED**.
- Shipment-reconcile **POST**: `096-S` in `.backlogit/archive/096-S.md` with
  `status: archived` → **PROCEED**.

## Review provenance

Eight Copilot review cycles plus a two-model adversarial re-review, all resolved:

- **Cycles 1–4 (Copilot)**: task-completion contracts recorded (read/mutation
  parity matrices, ratified composition contract, durability-policy selection,
  provenance flag/field selection — sections 7–9); `commit_links` durability
  wording corrected to a disposable index projection; the regression guard was
  strengthened to drive the real `UpdateArtifact` path; the containment
  recommendation was moved to lookup time; test inventory corrected.
- **Cycle 5 (two-model adversarial re-review on the current HEAD)**: Gemini 3.1 Pro
  returned clean; GPT-5.6-sol surfaced five charter-cross-checked findings, all
  accepted and applied in `ed016e3`:
  - **P0**: exit conclusion is **`proceed`, not `pivot`** — the charter
    (`exec-plan:652-664`) makes `proceed` the sole authorization for a later size
    implementation once all three proceed-gates resolve, and all three are resolved.
  - **P1**: durability must **guarantee exactly-once** history events per persisted
    provenance change (`exec-plan:183-193`); the lenient event-after-write policy
    was replaced with **event-before-write, fail-closed**.
  - **P1**: `QueueLayout.RootDir` admits a **lexical `..` escape** independent of
    symlinks; section 6 now mandates a two-layer lexical + realpath containment fix.
  - **P1**: read-parity matrix corrections (queue/list default divergence,
    `get_queue` already projecting `custom_fields`, `ExitError{Code:4}`
    mis-citation for a validation rejection).
  - **P2**: provenance-scope coherence (Scope now states provenance is SELECTED at
    medium confidence).
- **Cycles 6–8 (Copilot)**: PR-description/decision conclusion agreement confirmed
  (both `proceed`); spike `time_box` synced to the charter's 14h
  (`exec-plan:11,18,47,244`) in `e845145`. Final cycle-8 review on `e845145`
  returned zero unresolved threads.

## Decision outcome

- **Conclusion: `proceed`** (confidence: high on placement / medium on provenance).
- Canonical artifact-size home selected: **`custom_fields.size`** (the Model-A
  delegated option); the `models.Artifact` docline-carrier bridge rejected for now.
- All three proceed-gates resolved: canonical size location, inheritance-bridge
  selection, and the containment boundary (two-layer lexical + realpath fix).

## Release readiness

- **Blast radius**: none. Read-only spike; the only committed source is a test-tier
  regression guard. No runtime surface, schema, CLI, or MCP behavior changed.
- **Monitoring**: not applicable (no runtime change).
- **Rollback**: `git revert 86aa6ec` (single merge commit) if the deliverable or
  guard ever needs removal; no data or config migration to reverse.
- **Runtime verification**: not required (no runtime surface touched).

## Downstream authorization

The `proceed` conclusion authorizes — in a later **stage** cycle, not here — the
Stage restaging that moves `108-F` `blocked→active` and re-harvests bounded ≤2h
implementation units for the size feature, per the charter
(`docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md:652-664`).
The spike itself remained read-only.

## Follow-ups

- Promote the recorded decisions into the 108-F impl-plan (custom_fields.size for
  feature/shipment, two-layer containment hardening, event-before-write fail-closed
  exactly-once provenance policy, `size_source`/`size_ruleset_version` fields).
- Plan the size-aggregation ruleset as a separate work item.
- Optionally close the cosmetic CLI `list`/`shipment list` human-column read gap.
