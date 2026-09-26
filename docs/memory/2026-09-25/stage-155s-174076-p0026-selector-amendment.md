---
title: Stage 155-S 174.076-T P-002.6 Selector Amendment (rev22.5)
date: 2026-09-25
status: complete
agent: stage
shipment: 155-S
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head_at_start: edb8d88be551e84d439354aeb685693755994766
---

# Stage: 174.076-T P-002.6 Selector Amendment

## Trigger

Ship halted before implementation. The rev22.4 AC3 selector
`^TestHarnessManifestBudgetReconciliation$` has no unit prefix, which violates P-002.6
task-scoped command requirement 4 (`^TestU<unit>_`). The harness commit `edb8d88b` was
assertion-RED and correct in every other respect.

## Decision (D9, plan rev22.5)

* **Unit token.** The token is `U19R3` (Wave 19R work unit 3 of 3), and the harness is renamed
  to `TestU19R3_ManifestBudgetReconciliation`. No function in the repository already uses the
  `TestU19R` prefix.
* **Harness command.** `harness_cmd` is
  `go test -count=1 -timeout=5m -v -run '^TestU19R3_' ./tests/integration`. A run passes only
  with exactly one top-level PASS for the renamed function.
* **Regression.** The `npx ` regression selector moves into a canonical P-002.6
  `green-regression-contract` block.
* **Checksum probe.** It stays an acceptance-evidence probe. It is neither `harness_cmd` nor a
  green-regression entry.
* **Status.** 174.076-T returns to `queued` and keeps `harness-ready` until the rename's RED is
  re-verified.
* **Scope base.** The scope checks are measured from the Stage plan commit made in this session,
  whose SHA is recorded in the 174.076-T AC7. The task-file commit that follows is bookkeeping.

## Other conflicts checked

* **Compile check.** Handled per task by D2 (`go vet ./tests/integration`).
* **Lint and format.** Scoped to the test file per D3.
* **Contract blocks.** There is no exempt block and no red-deliverable block, so Step 4.3 item 4
  and Step 0.5 do not apply.
* **Wave scope.** W3 has a single member, so the sibling-red set is empty. The 174.074-T red
  deliverable was closed by 174.075-T.
* **Re-freeze.** At resume, Ship must re-freeze the green-regression array (Step 3 item 4)
  because the block is new.

## Not touched

The test file, the manifest, `_ship.agent.md`, other dirty files, and the Ship checkpoint.
