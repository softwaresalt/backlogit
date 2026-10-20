---
chunk_strategy: h1-h2-h3
# gate-required: closure_status and compaction_status MUST remain at the top level
# of this frontmatter (pipeline-topology gate reads them). Do NOT run
# `backlogit docs migrate --apply` against this file.
closure_status: READY
compaction_status: degraded
description: "Post-merge closure for 192-S / 192-F DisposeQueuedShipment core disposition (PR #552)."
doc_type: closure
schema_version: "1.0"
source: docs/closure/192-S-192-F-post-merge-closure.md
shipment_id: 192-S
title: "192-S / 192-F Post-Merge Closure"
---

# 192-S / 192-F Post-Merge Closure

* Shipment: 192-S (192-F, 192.001-T to 192.007-T), shipped via governed `backlogit shipment ship 192-S --sha 13dddf8d4e0f02f71280129ba44249ce954d9854`. Result envelope: `shipment_status: shipped`, `returned_ids: []`, `archived_ids` is exactly the 7 tasks, 192-F and 192-S (a subset of the manifest plus the control record). 192-S archived with `archived_status: shipped`, 192-F with `archived_status: done`. No archive deletions (P-007 clean). Only the governed `hooks_queue.jsonl` append for 192-S was committed besides the members.
* PR: #552, merge commit `13dddf8d4e0f02f71280129ba44249ce954d9854` (merge-commit strategy, P-009; admin fallback not used). Local main fast-forward synced to origin/main at that SHA.
* Behavior: exported `DisposeQueuedShipment` (validation, authorization-deliberation check, lifecycle then membership then artifact locks in `ClaimShipment` order, `dispose` intent journal, single governed `MoveShipmentStatus` write, nil-in-production `disposeAfterGovernedWriteHook`, commit-then-surface per G-D, not-applied rollback) and `ErrShipmentDispositionRefused`. Interim state I-3b: no CLI command or MCP tool reaches it until B5. G-B held: tests are in-process on temp-dir fixtures; no dispose CLI or MCP call was made against any workspace.
* R13 closure facts: the guard covers (P) only. 156-S and every other existing shipment are unchanged. The manual (P)+(C) 074-DL policy stays in force until AF1E5075 lands and the 154-S attestation exists. Binary version: not bumped by this release.
* Local review: READY_WITH_FOLLOWUPS. A first adversarial pass (2 of 3 reviewers) found a real G-D defect (a journal-finalize failure rolled back a committed write); fixed in 86d16c82 with a regression test and confirmed by 3/3 reviewers in a second pass. Both Go Reviewer passes on the later fixes (184063fd, 53982dce) found no P0/P1. Several reviewer passes had no shell and read code only; Ship ran build, vet and the disposition suites itself.
* Acknowledged P1 (plurality): the `ErrWriteIndeterminate` branch finalizes the journal without re-reading canonical state; mandatory condition G-D prescribes it, so it was deferred (stash EA4E7A29).
* CI: all required checks green on final HEAD 53982dce (test, Windows handle/lock tests, CLI Reference Drift, Docline, Markdown lint, line endings, pipeline-topology). Copilot review: two valid threads fixed within P-021 C1, each replied to after the fix was pushed and resolved via GraphQL: (1) DB-only links materialized by the governed write broke dispose recovery (184063fd; the first attempt added a journal field, which G-D forbids, so it was discarded and redone), (2) the journaled preimage was normalized and conflicted with the unchanged Markdown on recovery (53982dce). Copilot did not start on the auto-request, so review was re-requested through GraphQL; the copilot-review gate (required, 900s) reported SATISFIED.
* Local verification caveat: the full `go test ./...` and `-race` suites passed at 1ae49165 (pre-fix). After the fixes only scoped suites ran locally because the host was too slow (the `internal/core` package exceeded the local timeout); CI provided the post-fix full-suite evidence. `golangci-lint` could not run locally (installed binary older than the Go toolchain); CI lint applies.
* Follow-up stash (P-021 C2), all capture-only and provisional: EA4E7A29 (indeterminate branch finalizes without re-reading canonical state; needs a G-D amendment), EEDBDA6F (shared `MoveShipmentStatus` only logs a non-shipped status-event append failure), 69C33C68 (index-derived dependents; no lock on the authorization deliberation or successor). Lower-priority review notes not stashed: test assertion strictness in `shipment_disposition_preimage_test.go`, a DB-error fail-closed test for the dispose recovery merge, and a comment documenting that the merged DB links are not a compare-and-swap guarantee.
* Source artifacts untouched: stash 6434A4D7 and its provenance, 185-F, 185-S.
* Next in the dark-mode order: 193-S.

## Releasability

READY. Internal core code with no runtime surface and no reachable entry point; rollback is a revert of the merge commit; owner: Ship. No runtime verification was required.

## Compaction (P-020)

Degraded. compact-context was not run to completion: `docs/memory/2026-09-21-ship-155s-wave7-early-green-blocker.md`, `2026-09-23-ship-155s-review-circuit-breaker.md` and `2026-09-30-ship-154s-post-merge-closure.md` are still referenced from retained docs, so moving or archiving them would break those references. No files were moved. A later session should run compact-context after repointing those references.
