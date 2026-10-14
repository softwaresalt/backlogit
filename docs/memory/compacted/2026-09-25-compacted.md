---
title: "Compacted memory: 2026-09-25 155-S wave 3 selector halts, final review halt, wave 20 corrective, and delivery segments"
date_range: "2026-09-25 to 2026-09-26 (group keyed to 2026-09-25; the orchestrator source includes 2026-09-26 segments)"
doc_type: compacted-memory
source_count: 7
---

# Compacted memory: 2026-09-25

Compacts 7 source files covering shipment 155-S (feature 174-F) from wave 3 through delivery, plus the 154-S waiver and PR 449 sync segments. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

- Orchestrator waves: W1 to W3 completed and pushed at 6cd151ca. Tasks 174.073-T, 174.074-T, 174.075-T, and 174.076-T were done.
- Operator Option C: one-time waivers W-174074, W-174074 rev2, and W-174074-ext. They excluded pre-existing dirty paths (hash-verified) and lifecycle bookkeeping (auto-archive, stash appends, checkpoints, telemetry, memory, hook events). Waivers are not a general gate bypass.
- Two malformed Ship checkpoints (054402 and 054451) were repaired in place by removing one stray brace before ,"resume_hint". Originals were backed up under logs/diagnostics/. Both were then resolved as superseded by 054533.
- 174.076-T (W3): Ship's P-002.4 gate rejected docs-only for the YAML manifest and returned the task to Stage (AC6). Stage reclassified it as harness-required (plan rev22.4, D8), then renamed the harness to TestU19R3_ManifestBudgetReconciliation (rev22.5, D9) to meet P-002.6 requirement 4.
- Pinned lint: golangci-lint v1.64.8 installed into the gitignored bin/ with GOTOOLCHAIN=go1.24.0. PATH's v2.13.2 is the wrong major version.
- Tool surface: MCP tools must be named server/tool or server/*. Bare backlogit_* names are silently ignored. Commit 3a240fe0 gave Ship an explicit backlogit/<op> allowlist of 43 operations with no wildcard, and kept backlogit/* for Stage and Orchestrator.
- Final review (3a240fe0): standard review NOT READY with P0=0, P1=0, P2=3. The three P2s were in scope under P-021 C1: generic custom-field updates can invalidate a blocked envelope; unblock audit omits the prior reason and runs after metadata is cleared; compensation can terminalize a journal before its audit event is durable. Out-of-scope JSONL event-history rewrite captured as 388C586D.
- Wave 20 corrective (Stage, plan rev23.5): six units U20C1 to U20C6, assigned 174.077-T to 174.082-T. Attempt-4 review was ADVISORY (P0 0, P1 0). The operator approved it on 2026-09-26, and the six tasks were harvested. 155-S then had 45 members.

## Files modified or created

- Stage plan: docs/exec-plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md (rev22.4, rev22.5, rev23.x, Wave 20 harvest record)
- Ship and operator: .github/agents/_ship.agent.md and .autoharness/harness-manifest.yaml (allowlist, commit 3a240fe0); tests/integration/harness_manifest_budget_reconciliation_test.go (harness, commit edb8d88b)
- Final review record: docs/closure/2026-09-25-155-S-final-review.md (updated in 0b11459f)
- Closure fixes: 24e75400 and 596eadc7 (docline soft keys); 79b0f861 (task archive commit); c7d25cfb (MD001 archive heading)
- Other commits named: dd0e3f52 (build-dev.ps1), f7db30a3, 98367132, b78ce24b, d9a01e6f (wave 20 fixes)
- Local artifacts: .mcp.json backlogit command pointed at ./bin/backlogit.exe (uncommitted at the time); scripts\build-dev.ps1 built a debug binary.

## Key learnings

- P-002.4: a manifest YAML file is not docs-only, so a manifest checksum task needs its own harness.
- P-002.6 requirement 4: each task harness selector must match ^TestU<unit>_. Stage's un-prefixed selector was therefore rejected.
- The installed backlogit binary outside the workspace was built from b07729386a31, which predates the 148-F json.Valid create hardening (0b97483d). It therefore writes malformed checkpoint payloads verbatim. Rebuilding from HEAD is an operator action.
- The working-tree _ship.agent.md tools line (vscode/memory, backlogit/*, engram/*) fails TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery. This is the regression 174.072-T corrected, tracked as 4A0B7BCF.
- Full-suite exit 1 at 3a240fe0 came only from stray gitignored .go files under logs/. No timeout or panic occurred, and internal/core ran in 529.5 s.
- Observation: the operator config set the escalation route equal to the Stage route, which is a same-route ESCALATION_DEGRADED condition for Stage.
- Governed full-suite run at 0b11459f passed for all code packages. The only failure was closure-doc soft keys, since fixed.
- Ship attempt 1 of shipment 155-S was killed at a 5-minute timeout. Diagnosis: per-member itemlog validation at about 10 s per member, not a hang. Attempt 2 succeeded at a 45-minute budget.

## Failed or rejected approaches

- Two malformed checkpoint creates before a valid third (checkpoint 054533).
- Ship's docs-only exemption for the manifest (rejected by P-002.4).
- Un-prefixed selector ^TestHarnessManifestBudgetReconciliation$ (rejected by P-002.6).
- Ship cycle-1 remediation: stopped at compile (unused root variable at internal/core/shipment_blocked_recovery_harness_test.go line 1498). The assertion RED was never reached, counted as one failure.
- Accepting the final review's P2s as residual risk (not done; captured or fixed instead).

## Outcomes and open items

- 155-S was delivered: governed full suite passed; PR 450 merged at 2c8759c3 after CI and lock-order stabilization (three review cycles); shipment ship attempt 2 succeeded; closure PR 451 merged via merge commit c57185c1; local main equals origin/main at c57185c1.
- PR 449 was refreshed with plumbing and merged as merge commit c3f5e2ce. POST_MERGE_SYNC_BLOCKED from operator-dirty stash.jsonl and hooks_queue.jsonl was resolved with a pathspec stash, an ff-only pull, and a record-level merge. Backups are under logs/diagnostics/449-sync-*.bak.
- The operator approved the 154-S P-002.6 bootstrap waiver on 2026-09-26. Label and banner updates were deferred to the next Stage session, and the C29EBEE5 re-validation remained a prerequisite.
- Open stash: 9CA03F5D (red-delta gate contract), 2F7FCA8B (scratch doc-lint), 4A0B7BCF (Ship tools-line regression; allowlist fix landed in 3a240fe0), D116AF58 (perf), 67F17B6B (harness lock versus sidecar naming), plus Copilot and wave-20 captures (0FFBF819, 45BD3B36, AC6B669D, A4B62D86, 8AF55264).
- Operator-owned: rebuild the installed backlogit binary from HEAD, and disposition the superseded Stage checkpoints (005049 kept for E1 to E5).

## Sources

- docs/memory/2026-09-25/orchestrator-155s-w1-w3-resumption-memory.md
- docs/memory/2026-09-25/ship-155s-final-review-halt.md
- docs/memory/2026-09-25/ship-155s-remediation-stage-handoff.md
- docs/memory/2026-09-25/ship-155s-w3-d8-selector-contract-halt.md
- docs/memory/2026-09-25/stage-155s-174076-p0024-reclassification.md
- docs/memory/2026-09-25/stage-155s-174076-p0026-selector-amendment.md
- docs/memory/2026-09-25/stage-155s-wave20-review-remediation.md
