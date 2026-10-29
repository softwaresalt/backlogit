---
schema_version: "1.0"
doc_type: memory
title: Ship 197-S DARK_MODE_HALTED before PR: branch-wide review found open P1s; independent confirming pass not run
description: Ship re-checked the 197-S PR-stage gates under the reviewer-independence steering. The first adversarial round used a claude-haiku-5.5 reviewer (author model family, not counted) and an unspecified review-skill model (not counted). Ship verified the P1 claims. Two were fixed in docs, one needs an operator scope decision, one is Stage-owned, and one is a P2 in practice. No confirming pass, PR, merge, or closure was performed. Resume point recorded.
timestamp: "2026-10-10T23:27:00Z"
---

# Ship 197-S: DARK_MODE_HALTED before PR (confirming review found open P1s)

## Status

**DARK_MODE_HALTED (P-017), scope [197-S].** No PR was opened. No merge occurred. The post-merge closure
did not start. `origin/main` `d9d0237b` is untouched. 197-S remains `active` with all 18 members of M `done`
and 197.016-T `archived` (descoped by operator ruling). Nothing outside 197-S was claimed.

Branch `feat/cx-ship-closure-protocol-and-gate-correctness`. Parent HEAD `de1f8adf` (the commit whose unfiltered
full suite exited 0). This note and the two doc fixes below are committed on top of it. Those commits
change docs only and are not reviewed yet; the next invocation must review them.

## Reviewer independence (operator steering, applied)

| Pass | Reviewed HEAD | Reviewer models actually used | Counted toward consensus | Result |
|---|---|---|---|---|
| Adversarial round (Adversarial Review agent, report-only) | `de1f8adf` | Anchor `gpt-6.1-sol` (reasoning high); Tier 1 `claude-haiku-5.5`; Tier 3 `claude-sonnet-5.5` | Anchor and Tier 3 only (2 non-haiku, 2 families). The Tier 1 haiku reviewer shares the author's model family and does not count. | BLOCKED: P0 0, P1 5, P2 11, P3 7. |
| Review skill (general-purpose subagent, report-only) | `de1f8adf` | Not set explicitly. The runtime default was used, and I did not verify it is not the author model. Engram was unreachable, and no persona subagents were spawned. | No (model not verified independent) | READY_WITH_FOLLOWUPS: P0 0, P1 0, P2 4, P3 4. Its P1 count conflicts with the adversarial round. Ship verified the adversarial P1s directly. |
| Confirming pass (non-haiku set, final HEAD) | not run | not run | not run | Not performed. See "Why no confirming pass." |

Route notes: the alternate `google/gemini-3.8-flash` reviewer was **not honored** (`ROUTING_DEGRADED`).
The reviewer-set note in the first round read "Anchor/Tier 1/Tier 3" from the workspace routing config, so the
Tier 1 slot defaulted to haiku. That slot is not counted.

## Ship verification of the P1 claims

| # | Claim (source) | Ship verification | Status |
|---|---|---|---|
| P1-1 | R7 catalog defect. `internal/core/metadata_catalog.go:127-128` hard-codes `storage/queue` and `storage/archive`, while `QueueRootDir` (`:133`) reports the configured `QueueLayout.RootDir`. R7 requires the attested `workspace.queue_path` and `workspace.archive_path`. Capture 00A9D01C defers it. | Verified by direct read of the three lines. | **OPEN, operator or Stage decision.** Either (a) an in-scope fix so the catalog reports the configured queue and archive roots, with a test, scoped gates, a full unfiltered run, and a confirming review, or (b) an operator ruling that R7 is partial for non-default queue layouts, with 00A9D01C as the named limitation. Ship does not choose between them, because that is a P-021 C1 scope call. |
| P1-2 | `shipment-reconcile/SKILL.md` Safe-Close step 2 ("Reject blocked or other nonterminal shipment state") literally rejects `active`, the state Ship closes from. | Verified by reading the text. | **FIXED in this commit.** The step now reads "Reject blocked, queued, or any other nonterminal shipment state other than `active` (the admissible pre-close state)." The test anchors in `tests/integration/shipment_reconcile_feature_member_contract_test.go` (heading and rule substrings) are unchanged, and the `Reject blocked` phrase is not asserted by any test. |
| P1-3 | `_ship.agent.md` step 6 item 1e builds closure paths from hard-coded `.backlogit/` and interpolates IDs without a grammar check. | Verified at lines 1503-1510. | **FIXED in this commit.** Paths are rooted at the attested `workspace.storage_root` (`.backlog` or `.backlogit`). Each ID is validated against `^\d{3,}(\.\d{3,})*-[A-Z]{1,2}$` before path construction, with halt `CLOSURE_ALLOWLIST_INVALID_ID`. The fix still needs the confirming review. |
| P1-4 | Safe-close report records changed-ID sets, not exact paths. Side-effect and log writes (for example the tracked `.backlogit/hooks_queue.jsonl`) therefore cannot be allowlisted, and the next closure halts at 1e. | Reviewer verified against `shipment-reconcile/SKILL.md` step 9. Ship did not re-verify. Practical severity is P2, because item logs are gitignored. | **OPEN.** Fix: the safe-close report must list exact tracked side-effect paths, with a contract test. This is an allowlist-completeness change, so it is not folded into this pass. |
| P1-5 | Plan R10 Requirements Trace row reads "profiled, fixed, and reports progress" (`docs/exec-plans/2026-10-08-cx-s-ship-closure-gate-correctness-plan.md:63`). Erratum E2 at `:829` corrects it at the tail. | Verified at line 63. | **OPEN, Stage-owned.** Ship may not modify plan artifacts (P-010). Stage should annotate the row: "fix not delivered; see E2; follow-up stash 76553D8D." The PR body will state partial R10 coverage regardless. |

## Open P2 and P3 items (not fixed in this pass)

* P2: `.github/policies/workflow-policies.md` (~1296), P-007 violation step 3, still stages `.backlogit/archive/`
  directory-wide, which conflicts with the 1e exact-path allowlist. Policy text change needed.
* P2 (round B, captured earlier): duplicate progress lines (`shipment_lifecycle.go:607` and `:627`, capture
  `5E0CCF91`). Weak CLI progress contract test (capture `2C01BD52`). Allowlist precision (capture `2C8615A5`).
* P2 and P3 not yet captured. Capture before any residual-risk record cites them: the P-007 policy conflict above,
  legacy `.<file>.lock` sidecars not detected by the new scripts (P3), and stale 140-S closure guidance on
  `docs migrate --apply` gate-key folding (P3, `docs/closure/140-S-158-F-post-merge-closure.md:6,40,42`).
* P3, carried from the round-A report: unchecked `Fprintln` (`shipment.go:396`), stale `WAVE_SNAPSHOT_UNRELIABLE`
  taxonomy, and a vacuous `assert.Zero` (`cx_ship_closure_protocol_contract_test.go:59`). Not triaged here.
* **AC3 (U17) refresh is pending.** `.github/agents/_ship.agent.md` (pinned at `.autoharness/harness-manifest.yaml`
  line 407 and by `tests/integration/harness_manifest_197_drift_records_test.go` line 56) changed in this pass.
  The drift test checks only shape and "differs from pre-refresh," so the suite stays green. The recorded
  LF-normalized checksum is nonetheless stale. Re-run the refresh before merge.

## Gates and evidence at this point

* Served roots re-attested: `workspace.root_path` `C:\Source\GitHub\backlogit`, `workspace.storage_root`
  `C:\Source\GitHub\backlogit\.backlogit`, and one `main` row `.backlogit/backlogit.db`.
* Post-claim topology gate: exit 0 (sole active shipment 197-S). The JSON was written under `logs/`, not printed.
* M: all 18 tasks `done`. `197.016-T` `archived`. The manifest is 197-F plus 18 tasks.
* Unfiltered full suite at `de1f8adf`: exit 0, all 40 packages `ok` (`logs/full-suite-197s-orchestrator-diag.txt`,
  gitignored). Two earlier unfiltered runs failed under host load (`internal/core` lock-timing tests and an
  unnamed panic). Both are recorded in `ship-197s-final-gate-halt-memory.md`. Their names and panic line are not
  proven. The clean run is at the same Go state.
* Go changes since `de1f8adf`: none. The two fixes are docs-only. Contract tests that quote the edited files were
  checked by grep for the changed phrases. Only the heading anchor and rule substrings are asserted, and they are
  unchanged. A full unfiltered run is still required before merge if any Go file changes, including the R7 fix.

## Why no confirming pass

The steering asks for one confirming adversarial and review-skill pass on the final HEAD, after in-scope P0/P1
fixes. Two P1s cannot be closed inside Ship's authority: P1-1 needs an operator or Stage scope decision (and, if
fixed, a Go change and a full suite), and P1-5 is Stage-owned. A confirming pass on a HEAD with open P1s would
not clear the gate. The session budget (about 7.3k of 10k AIC at the Orchestrator's last count) cannot also cover
a confirming pass, the PR, the Copilot loop, CI, merge, and closure. Ship stopped here instead of starting a
partial PR.

## Resume point (next invocation, no checkpoint restore)

1. Decision: P1-1 (R7). Operator or Stage chooses (a) an in-scope catalog fix with test and full suite, or (b) a
   partial-coverage ruling with 00A9D01C named as the limitation.
2. Stage: annotate plan R10 row (`:63`) as "fix not delivered; see E2; follow-up stash 76553D8D."
3. Ship: fix P1-4 (exact side-effect paths in the safe-close report, with contract test) if it stays in scope.
   Fix P2 policy conflict (P-007 step 3). Capture the uncaptured P2 and P3 items under P-021 C2.
4. Ship: re-run the AC3 refresh for `_ship.agent.md` (and `shipment-reconcile/SKILL.md` if it is among the eight).
5. Confirming pass on the final HEAD, report-only, scoped to files changed since `becfca4e` plus the 197-S contract
   surface. Reviewers: Anchor `gpt-6.1-sol` (high, openai); Tier 3 `claude-sonnet-5.5` (anthropic); review skill
   on `gpt-6.1-sol` or `claude-sonnet-5.5`. Never `claude-haiku-5.5` (author family). Record `ROUTING_DEGRADED`
   for any route that is not honored, with the actual model used.
6. If any Go file changes, run the full unfiltered `go test -timeout=30m ./...` with no concurrent subagents.
7. Then PR (BOM-less body file under `logs/`, with the `## Local Review Readiness` block, the reviewer model list,
   and the R10 partial-coverage statement), Copilot review loop (P-018, `required`), CI green, and merge commit
   only under the DARK_MODE record. `admin_fallback_pre_authorized` is false. Then post-merge closure, the closure
   PR, and P-020 compaction.

## Notes

* No checkpoint was restored. Ship's halt checkpoint `checkpoint-20261010-223521.json` remains a pointer. A new
  pointer checkpoint is written at the end of this invocation. The stale 153-S checkpoint is untouched.
* No backlog item was triaged, created, or moved in this invocation. No stash entry was edited.
* Scratch files under `logs/` are gitignored and were not committed.
