---
title: "201-S Ship Session Memory"
source: docs/memory/201-S-ship-session-memory.md
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

# 201-S Session Memory (Ship, Dark Factory)

* shipment: 201-S (feature 201-F), branch feat/201-s-cy-c-harden-filesystem-containment-and-snapshot-parsing
* items completed: 201.001-T .. 201.010-T (all done, archived by the governed move)
* implementation: handle-contained manifest member read (POSIX descriptor-relative Openat walk; Windows handle + final-path check), Windows NtCreateFile pinned-directory writers (spike: docs/decisions/2026-10-08-cy-c-windows-relative-create-spike.md), no-follow blocked-snapshot read, shared fold-aware duplicate JSON member scanner (json_member_scan.go, depth limit 10000)
* review: adversarial round 1 BLOCKED (fold mismatch, POSIX non-ancestor-safe read, Windows pin re-resolution) -> remediated; round 2 READY_WITH_FOLLOWUPS (U1 append logs-dir containment, U3 depth limit fixed; U2 deferred); final code-review READY
* deferred (P-021 C2): stash D7558EE1 (pre-existing read-side containment items, snapshot_unix hardening, Windows read-path anchoring, events strict walker)
* gates: build, vet (windows/linux/darwin for internal/core), gofmt clean, go test ./... green with *LintFix and TestU172_ItemLogBarrierSetterUnreachableFromProduction skipped (golangci-lint v1.64.8 built with go1.26 cannot read go1.27 export data; TestU172 walks the ~3GB gitignored logs\ dir and could not be parked because rename was denied) 
* Windows swap tests ran (not skipped) using mklink /J junctions
* next: push, PR, Copilot review loop, merge (merge commit), post-merge closure
