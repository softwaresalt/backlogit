---
title: "Ship closures and dark-run halts, 2026-10-16 to 2026-10-21 (compacted)"
source: docs/memory/compacted/2026-10-30-dark-runs-and-ship-closures-2026-10-16-to-10-21-compacted.md
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Scope

P-020 compaction of thirteen uncited notes for completed shipments 141-S to 145-S, 184-S, 185-S, 187-S to 189-S, 193-S, plus one stash triage session. Originals are archived under `docs/archive/memory/2026-10-16/` through `docs/archive/memory/2026-10-21/` (same relative names).

## Outcomes

* Dark run 141-S to 145-S (2026-10-16) halted at preflight with `STAGING_GATE_FAIL`: `.backlogit/hooks_queue.jsonl` was dirty on `main` and outside the Step 1.5 continuity allowlist. The operator ruled it belongs on the allowlist; it was carried through the staging PR. A stale active 153-S checkpoint was left untouched (no selection without operator choice).
* 141-S / 159-F: PR 512 merged (`f0257080`). Claim MCP timeout recovered by re-reading state, branch renamed for the topology gate, 4 Copilot reviews, deferred stash `291EE81C`, `B1DD8FA4`, `B597D942`, `1C88D6F8`.
* 142-S / 160-F: PR 515 (`a61d409e`) after 6 Copilot cycles; stash `09B597EE`, `F9C8E8C1`, `D399A888`.
* 143-S / 161-F: PR 518 (`6235ab73`), no admin fallback.
* 144-S / 162-F: PR 521 (`4fa4c6e7`), 5 Copilot cycles, 12 threads fixed; `ship_shipment` finished server-side after an MCP timeout.
* 145-S / 163-F: PR 524 (`b1695713`); stash `7679EE0D`, `10D36FF6`, `0527C687`; learning `docs/compound/2026-10-17-policy-engine-review-fix-classes.md`.
* 187-S (187-F): AST declaration harness, `ErrShipmentPredecessorNotShipped`, predicate implementation with a Markdown-only allowlist; adversarial review READY_WITH_FOLLOWUPS, stash `1FE847BC` (shared `findArtifact` duplicate-ID handling).
* Stash triage (2026-10-18): 230 to 220 active entries; ten archived (six outright, four merged duplicates), three restored after PR review; report `docs/decisions/2026-10-18-stash-triage-report.md`.
* 189-S: build complete (tasks 189.001 to 189.007, `45f1e5b1` to `2dcb8646`); claim stalled over MCP with a pending shipment-operation journal and a held lifecycle lock, which cleared on its own at 18:55 without killing processes or editing the journal.
* Dark run 188-S to 185-S (2026-10-19 to 10-21) completed (`DARK_MODE_COMPLETE`, final main `b51c5a29`). 193-S was blocked because the 186-S closure had prose conditions only (`PREDECESSOR_CLOSURE_INCOMPLETE`); fixed with live halt-row Ship sessions and a dry routing pass (PR 558), not by marking conditions satisfied. 184-S merged as `b826f7b5` with the known A592FC1C `pipeline-topology (ambient)` false positive on a non-required check.

## Learnings

* Commit the archive record after `shipment ship`, not before; a closure PR staged earlier carried a stale archive record (184-S).
* MCP claim, move, and ship calls time out under load; poll state for quiescence rather than retrying.
* Never mark closure conditions satisfied without evidence to clear a gate.

## Open items carried forward

* Closure compaction was `degraded` for several 2026-10-19 closures because retained docs referenced `docs/memory` paths.
* Fixture folders under `logs/af1e5075-fixture` and `logs/6434a4d7-fixture` await operator approval for removal (stash `CBB7997C`, `E22DDC43`).
* `close_path`/`close_evidence` are not produced by the installed harness (stash `3FF72EB8`).
