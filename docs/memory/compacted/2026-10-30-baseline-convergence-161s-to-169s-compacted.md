---
title: "Baseline convergence shipments 161-S to 169-S, 2026-10-23 to 2026-10-26 (compacted)"
source: docs/memory/compacted/2026-10-30-baseline-convergence-161s-to-169s-compacted.md
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Scope

P-020 compaction of eleven uncited notes for completed shipments 161-S, 162-S, 163-S, 165-S, 166-S, and 169-S (feature 175-F baseline convergence, remediation waves RS-W04 to RS-W12). Originals are archived under `docs/archive/memory/2026-10-23/` through `docs/archive/memory/2026-10-26/`. Notes for 164-S, 167-S, and 168-S are still cited and remain in place.

## Outcomes

* 161-S (RS-W04, tasks 175.031 to 175.039): harness `b1257a9b`, fix `277f4a25`, doc `d334bce4`; 49 owned lint findings removed. The claim timed out client-side but landed (verified by queue state and a `CLAIM_VERIFY_OK` post_claim gate). PR 572 (`9578b255`) later added tests plus four function-variable seams for PR 570 close-error branches (stash `D8391636`) and a verified `conditions:` list on the 161-S closure.
* 162-S (RS-W05, nine tasks): harnesses `e9a74ff8`, P-021 correction `5a8e6769`, hardening `9ac957bf`; adversarial review found one P1 (harness false-pass), fixed. `internal/core` timed out under concurrent load; isolated rerun recorded in the PR.
* 163-S (RS-W06, 175.043 and 175.052 to 175.059): RED harnesses `ba603f10`, errcheck fixes `95607025`; PR 575 merged as `3103b25a`; closure `READY`, follow-ups stash `FB313DEB`.
* 165-S (RS-W08, 175.068 to 175.076) and 166-S (RS-W09, 175.077 to 175.085): errcheck remediation waves, 166-S commits `10cc44f8`, `a9a7369b`, stash `913890C2`. Run golangci-lint harnesses sequentially because concurrent runs flake.
* 169-S (U12 terminal convergence, 175.012-T): Ship halted first because `scripts/verify-terminal-lint.ps1` is fixture-only, tracked-file `gofmt -l` scanned git-ignored `.copilot/` and `logs/`, and `go test` needed `-timeout=30m`. Stage amended the task (live golangci-lint v2.13.2 on both OS targets, `git ls-files` batching for gofmt, serial tests with `-skip` for the LintFix harnesses), re-pinned `task_contract_sha256` on 175-F, and unblocked 169-S; stash `FDA05002` records 23 post-baseline lint findings. Ship then re-claimed, and evidence commit `a0837abf` passed `EXEMPT_VERIFY_OK:175.012-T`; stash `03211AD7` captures an `internal/evidencedag` load flake.

## Learnings

* `backlogit_claim_shipment` and `move_item` time out client-side but complete; poll for quiescence instead of retrying.
* Terminal gate scripts that require fixtures never pass without `-FixtureRoot`; fix the contract, not the gate.
* The 162-S pre_claim gate checks branch ownership first, so it only passes from `main` or the shipment branch.
* `closure-evidence` fails only at `close_path` (known posture, stash `3FF72EB8`).
