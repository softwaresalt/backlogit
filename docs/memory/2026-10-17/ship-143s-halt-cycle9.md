---
schema_version: "1.0"
doc_type: memory
title: "Ship 143-S halt at extended Copilot cap (cycle 9)"
description: "PR #518 HEAD 177299e2; cycles 7-9 resolved; cycle-10 P1 on baseline.go applyOverlay pending; halted per operator cap."
chunk_strategy: h1-h2-h3
---

# Ship 143-S halt (2026-10-17 UTC)

* Branch feat/s9-red-test-honesty-gate, PR #518 OPEN, HEAD 177299e2 (full suite, vet, lint, build, LF gofmt green).
* Cycles 7 (empty Results list), 8 (build-constrained new tests), 9 (go-ignored _/. names) fixed, replied, resolved.
* Open thread PRRT_kwDORzozKM6td6IA (baseline.go:117): cycle-9 isTestdata widening also makes applyOverlay overlay hidden production inputs (p/.config). Fix: separate discovery-exclusion predicate from overlay eligibility (overlay only real test files and explicit testdata); add regression that hidden production config stays at baseline.
* Not merged; 143-S still active; 161-F still active; no closure done.
