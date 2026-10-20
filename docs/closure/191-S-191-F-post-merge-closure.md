---
chunk_strategy: h1-h2-h3
# gate-required: closure_status and compaction_status MUST remain at the top level
# of this frontmatter (pipeline-topology gate reads them). Do NOT run
# `backlogit docs migrate --apply` against this file.
closure_status: READY
compaction_status: degraded
description: "Post-merge closure for 191-S / 191-F dispose journal recovery (PR #550)."
doc_type: closure
schema_version: "1.0"
source: docs/closure/191-S-191-F-post-merge-closure.md
shipment_id: 191-S
title: "191-S / 191-F Post-Merge Closure"
---

# 191-S / 191-F Post-Merge Closure

* Shipment: 191-S (191-F, 191.001-T, 191.002-T), shipped via governed `backlogit shipment ship --sha`; every explicit member and the shipment record archived, `returned_ids` empty, no archive deletions (P-007 clean).
* PR: #550, merge commit `c22fea941be8d6387a9e9a6ac33e402cd458b087` (merge-commit strategy, P-009). Main fast-forward synced to origin/main at that SHA.
* Behavior: shipment lifecycle journal recovery accepts the `dispose` operation (`rollback`/`abandoned`). Recovery never restores the queued preimage over a complete validated disposition (G-D): it keeps the disposition and its status event, repairs the SQLite index only (Copilot review fix, no Markdown or event rewrite; journal retained on index failure), and finalizes the journal. An on-disk queued preimage is cleaned up without change; foreign state fails with `ErrShipmentConflict` naming the shipment (G-C). `shipment.go` unchanged; no new journal field. No production dispose writer exists yet (B-U13b).
* Local review: READY_WITH_FOLLOWUPS at e1021353 (P0 0, P1 0, P2 2, P3 about 8). Four passes (correctness, Go, concurrency, shell-capable general-purpose); three had no shell and ran no tests, the shell-capable pass ran the tests, vet and gofmt. Adversarial escalation criteria not met. CI all green on the final HEAD 4a948bf5. Copilot review: one valid thread (stale SQLite row on kept disposition) fixed in 4a948bf5, replied, resolved; copilot-review gate SATISFIED.
* Local note: `golangci-lint` cannot run locally (toolchain export-data mismatch); under machine load a few unrelated `internal/core` timing tests were flaky locally and passed alone and in CI.
* Follow-up stash (P-021 C2): B9566D61 (kept disposition with a missing status event is finalized without appending evidence), 6560E944 (B-U13b writer must not have its own fresh dispose intent journal cleaned by a recovery barrier; add an ordering test).
* Source artifacts untouched: stash 6434A4D7 and its provenance; 185-F and 185-S.
* Next in the dark-mode order: 192-S (its predecessor 191-S is now shipped).

## Releasability

READY. Internal core recovery code with no runtime surface; rollback is a revert of the merge commit; owner: Ship. No runtime verification was required.

## Compaction (P-020)

Degraded. compact-context was not run to completion: its candidate selection would move `docs/memory/` notes that retained decision, plan and checkpoint documents still reference, and moving them would break those references. No files were moved. A later session should run compact-context after repointing those references.
