---
title: "Compound refresh proposal: custody, shipment lifecycle, reconcile, handoff (propose)"
chunk_strategy: h1-h2-h3
doc_type: closure
schema_version: "1.0"
source: docs/closure/2026-11-03-custody-lifecycle-compound-refresh.md
description: "compound-refresh propose-mode report, run after the P-020 compact-context pass on 2026-11-03, for compound entries touching custody, shipment lifecycle, shipment reconcile, nonce, or workflow handoff. Records classifications and evidence; no compound file was changed."
mode: propose
scope: "docs/compound entries matching custody, shipment reconcile, shipment lifecycle, nonce, or workflow handoff"
context: "Shipment 211-S (feature 210-F, workflow custody and Stage-to-Ship handoff) post-merge closure on branch post-merge/211-s-closure."
---

## Scope and method

* `mode`: `propose`. No file under `docs/compound/` was edited.
* Candidate set: 20 compound entries whose title, description, or tags name shipment lifecycle, shipment gate, shipment reconcile, post-merge closure, or the Stage-to-Ship handoff.
* No compound entry mentions `nonce`. Nonce and custody material appears only in the 210-F plan and the 211-S records.
* Evidence level: the frontmatter description was read for each entry except `2026-10-21-closure-conditions-list-predicate.md`, which defaults to keep. The carry-forward entry and the 210-F plan's relevant sections were read in full. The remaining entries were not re-verified against current code in this pass. Their "keep" outcomes rest on description-level evidence and the absence of any 210-F or 211-S supersession statement.

## Supersession check for 210-F and 211-S

* No compound entry is superseded by 210-F or 211-S.
* The 210-F plan cites compound entries as inputs (lock ordering, shipment-shipped prevention envelope, fail-closed authorization, pin-the-handle). It builds on them and does not replace them.
* One entry is flagged as partially superseded (see `carry-forward` below). Supersession is not applied yet, because the governed replacement is not operative. The 211-S runtime verification records that a live Profile A accept fails closed without a nonce provider, and that no custody envelope exists for 211-S.

## Classifications

| Entry | Outcome | Evidence |
|---|---|---|
| `2026-07-06-ancestor-aware-shipment-gate-staleness.md` | keep | Gate staleness rule; not touched by 210-F. |
| `2026-07-13-post-merge-lifecycle-requires-fresh-binary.md` | keep | Still the rule for post-merge state-mutating CLI calls. |
| `2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md` | keep | Its UPDATED header records the shipped 133-F membership fix. 210-F does not change cascade membership. |
| `2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md` | keep | The 210-F plan's binary inventory (v1.12.0, lacks 210-S) confirms the pinned-binary gap this entry describes. |
| `2026-08-02-variadic-options-backward-compatible-shipment-creation.md` | keep | Factory-option pattern; unrelated to custody. |
| `2026-08-18-shipment-shipped-prevention-envelope.md` | keep | Cited by the 210-F plan as an input. Not superseded. |
| `2026-07-20-ship-gate-descoped-archived-member-exemption.md` | keep | Gate member exemption; not touched by 210-F. |
| `2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | keep | Re-persist seam guidance; still applies to the custody write path, which uses the Markdown reload. |
| `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` | update | Core rule holds: after an MCP timeout on a governed ship, poll state and do not retry. Later closures confirm it: the 160-S, 162-S, 165-S, 166-S, 167-S, and 168-S records each say the timed-out call was not retried and that completion was verified by polling. Two deviations are recorded. The 169-S closure ran a CLI retry that returned `shipment status conflict`, which confirmed completion. The 210-S closure also retried the governed close and got the same conflict (210-S compound-refresh report). Both retries were harmless but contradict the "never retry" rule. Add a "no retry" step to the closure runbook, and record the conflict response as the expected confirmation signal. |
| `2026-10-14-ship-governed-archive-git-launch-fallback.md` | keep | Recovery guidance for governed archive launch failures. No 210-F conflict. |
| `2026-10-21-commit-archive-record-after-governed-ship.md` | keep | Consistent with the 160-S and 169-S closure evidence (archive records committed after ship returns). |
| `2026-10-21-claim-stall-under-load-poll-state.md` | keep | Consistent with several 2026-10 closures that converged under load. |
| `2026-10-21-closure-conditions-list-predicate.md` | keep | Closure-gate predicate; no 210-F conflict. |
| `workflow-issues/carry-forward-continuity-files-into-staging-2026-09-08.md` | keep now; flag for replacement | The Orchestrator Step 1.5 carry-forward rule is the manual precursor to governed pre-existing adoption. The 210-F plan (R2) states `.gsd/carry-forward.yaml` cannot express adoption of a specific revision, and it defines the `preexisting_adoption` custody kind as the governed path. Replace or consolidate only after custody is operative and verified in a live Profile A or Profile B run. Until then the manual rule still governs. |
| `workflow-issues/ship-agent-post-merge-closure-skipped-on-idle-merge-2026-04-24.md` | stale candidate (manual review) | Tagged shipment-lifecycle. Its six-step closure description predates the current post-merge local-main synchronization rule in the git-merge instructions. Not verified against current Ship text in this pass; recommend manual review. |
| `best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md` | keep | Snapshot and rollback rule for multi-item lifecycle transitions. |
| `best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md` | keep | Test-harness guidance for lifecycle successors. |
| `go-patterns/f015-shipment-stash-patterns.md` | keep | Historical F015 patterns; no 210-F conflict. |
| `workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md` | keep | Historical gate-ordering lesson; still valid. |
| `workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md` | keep | Historical architecture rationale; the Stage-to-Ship handoff feature does not contradict it. |

## Follow-up items for Stage

* Flag `carry-forward-continuity-files-into-staging-2026-09-08.md` for a replacement decision once custody is operative. Record the dependency on a live `preexisting_adoption` run.
* Apply the `update` to `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` in a later `mode=apply` run. This is a proposal only.
* Manually review the 2026-04-24 post-merge closure entry against the current Ship protocol.
* Compound entries outside this scope were not reviewed. The 210-F plan cites `docs/compound/2026-10-19-lock-held-side-effect-ordering.md` and `docs/compound/2026-10-27-pin-the-handle-not-the-path.md` as inputs; both should be reviewed in the next scheduled full refresh.
