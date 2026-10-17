---
schema_version: "1.0"
doc_type: memory
title: "Compacted memory: 143-S ship checkpoints and PR #518 review-fix history"
description: "Dense summary of the 143-S build checkpoint and cycle-9 halt notes, with the extended Copilot cycles 10-11 and resolution."
chunk_strategy: h1-h2-h3
---

# 143-S ship checkpoints (compacted, 2026-10-17 UTC)

Compacts `ship-143s-checkpoint.md` and `ship-143s-halt-cycle9.md` (originals moved to `docs/archive/memory/2026-10-17/`).

## Decisions and outcomes

* Shipment 143-S (feature 161-F, tasks 161.001-T..161.003-T) built red-first: new package `internal/redhonesty`, faultline schema v2 (`{parity, red_green}`); v1 stays `{parity}`.
* Structural honesty rules are documented heuristics; behavioural rules are exact. Unbuildable-at-baseline tests are rejected as unverifiable (fail closed). The gate is not a sandbox.
* A five-persona adversarial review ran before the PR; findings were batched.
* Cycles 7-9 fixed empty Results lists, build-constrained new tests and Go-ignored `_`/`.` names. The prior session halted at the cap with cycle 10 open.
* The operator extended the cap by up to 3 cycles with stricter triage. Cycle 10 fixed a cycle-9 regression by separating discovery exclusion from overlay eligibility. Cycle 11 made overlay use `isTestFile`. Cycle 12 was not needed.
* PR #518 merged as `6235ab737c18b784a0b33d69a6e13b4076463c45` (merge commit); final reviewed head `29a51277`.
* `ship_shipment` timed out client-side but completed server-side; verify by re-reading state, and a CLI retry fails on the lifecycle lock while the server operation is still running.
* Stash entries: `0ACF635E`, `3A2C710D`, `E6EE4ACD`, `19E87B10`, `95DF7CE9`.