---
chunk_strategy: h1-h2-h3
description: "Compound-refresh after 196-S. Six in-scope entries were reviewed and all kept, and one new learning was captured."
doc_type: closure
schema_version: "1.0"
source: docs/closure/196-S-served-root-handoff-post-merge-closure.md
shipment_id: 196-S
title: "Compound refresh — 196-S"
docline:
  date: 2026-10-06T00:00:00Z
  status: reviewed
  tags:
    - compound-refresh
    - 196-S
---

## Scope

`scope: recent`, `mode: apply`, context: 196-S post-merge closure (PR #478,
merge `651b066b`). 196-S changed contract text and tests only:

* `.github/agents/_orchestrator.agent.md` (served-root handoff)
* `.github/agents/_ship.agent.md` (Step 0.5 item 3a outcome record)
* `.github/skills/shipment-reconcile/SKILL.md` (adds the
  `feature-pending-governed-completion` classification)
* harness-manifest checksums
* integration contract tests

Selection keyed on `served root`, `feature-pending`, `explicit feature`,
`shipment-reconcile`, `ship_shipment`/`ShipShipment`, `timeout`,
`RECONCILE_FAIL`, `173-F`, and `195-S` across `docs/compound/`.

## Classifications

| Entry | Classification | Evidence |
|---|---|---|
| `2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md` | keep | Its post-133-F guidance (native cascade allowed, gated on explicit membership) still matches `collectArchiveCandidateIDs`. 196-S adds a pre-close classification for an `active` explicit feature but does not change archival scope. |
| `2026-07-13-post-merge-lifecycle-requires-fresh-binary.md` | keep | The rule held: closure first verified that the served MCP binary commit `7c805f9b` descends from `47dfcc93` and that the reconcile skill contains the new classification. |
| `2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md` | keep | Same version-skew class as the new CI topology finding `F88FE051` (PyPI `autoharness==1.5.0` lacks the `dag-root` waiver). That one is a separate tool, so it is tracked as a stash entry, not merged into this entry. |
| `2026-08-18-shipment-shipped-prevention-envelope.md` | keep | Unaffected: 196-S did not touch the core-seam refusal for shipped transitions, and the governed `ShipShipment` path reached `shipped` as designed. |
| `2026-07-20-ship-gate-descoped-archived-member-exemption.md` | keep | Unaffected: 196-S had no descoped members, and the completion gate passed on all ten explicit members. |
| `2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | keep | Confirmed by the 196-S close. After `attachCommitToItems` and archival, the 196-S archive still carries its `related_to` link to `195-S` and `archived_status: shipped`, and every member carries merge SHA `651b066b`. |

## New learning

`docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` was
captured through `compound`, not refreshed. The `backlogit_ship_shipment` MCP
client timeout recurred in both the 195-S and 196-S closures. Neither occurrence
was previously captured.

## Follow-up

None. No entry was marked stale.
