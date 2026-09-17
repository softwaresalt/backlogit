---
doc_type: memory
schema_version: "1.0"
shipment_id: 149-S
feature_id: 168-F
title: "Governed handback of shipment 149-S"
---

## Decision

The operator explicitly authorized pausing shipment `149-S` so a separate
repository-baseline release unit can remediate the mandatory global quality
gates. No further `149-S` implementation was performed.

The handback used backlogit's governed status transition path:

* `backlogit move 168-F --status queued`
* `backlogit move 149-S --status queued`

No force flag, status-file edit, shipment reclaim, task reset, or policy
exception was used.

## Preserved execution state

* Shipment `149-S`: `queued`
* Covering feature `168-F`: `queued`
* Task `168.001-T`: `done`
* Task `168.001-T` implementation commit:
  `c976315fa6d97e5b9db60f936fa9f5cbc7d12b74`
* Remaining frozen shipment tasks: 18 `queued`
* Existing branch:
  `feat/149-s-trust-anchor-verification-key-lifecycle`
* Preserved branch history through:
  `6ce09a968639bf2f4d848db7c0f53d98a0298af0`

All prior checkpoint, reconciliation, memory, stash, and bug-report evidence
remains preserved. Engram CLI use for this shipment continues to require
clearing the process-local override before each invocation:
`$env:ENGRAM_DIRECT=$null; engram ...`. `.env.local` was not modified.

## Reason for handback

Wave 1 passed its task-scoped harness, package tests, build, targeted lint, and
targeted format checks. The required repository-wide convergence gate remains
blocked by inherited baseline conditions outside shipment `149-S`:

* `go test ./...` fails
  `internal/faultline.TestU4aBehaviorCanonicalByteStable` because a checked
  fixture contains CRLF while canonical output uses LF.
* `golangci-lint run` reports 56 inherited findings: 50 `errcheck` and 6
  `staticcheck`.
* `gofmt -l .` reports broad existing Windows line-ending/format drift.

Those baseline files were not modified because they are outside the frozen
shipment scope.

## Verification

After the transition, the exact frozen task census was:

* `done`: 1
* `queued`: 18
* all other statuses: 0

`autoharness gate pipeline-topology --mode agent --shipment 149-S --phase
pre_claim --json` passed unforced. It reported no active shipments, confirmed
the current branch owns `149-S`, confirmed a single implementation worktree,
and accepted predecessor `148-S`.

## Strict-safety action record

* **ProposedAction:** Return the covering feature and shipment control record
  from `active` to `queued` through backlogit's validated transition path.
* **ActionRisk:** Moderate. The action releases the single active shipment slot
  while retaining completed work and all resumption evidence.
* **Approval:** Explicit operator authorization in the current session.
* **ActionResult:** Applied. Shipment `149-S` and feature `168-F` are queued;
  completed task `168.001-T` remains done; no task was reclaimed or reset.

## Resumption boundary

Ownership is deliberately paused. Do not resume or reclaim `149-S` until the
repository-wide test, lint, and format baseline is green through a separately
authorized release unit. When resumed, re-run Step 4.6 wave convergence before
admitting the next dependency-correct frontier, `168.011-T`.

## Checkpoint disposition

After the queued state, exact task census, pushed handback commit, and unforced
topology result were verified, Ship resolved
`checkpoint-20260917-215709.json` at
`2026-09-17T23:10:58.5719794Z`. No replacement Ship checkpoint was created
because shipment ownership is deliberately paused for baseline remediation.
