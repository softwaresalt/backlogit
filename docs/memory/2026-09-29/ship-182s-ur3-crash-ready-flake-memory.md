---
title: "Ship 182-S memory: UR3 crash-ready handoff flake"
doc_type: memory
shipment_id: 182-S
feature_id: 181-F
task_ids: [181.001-T]
branch: feat/182-s-flake-prerequisite-deterministic-ur3-crash-ready-handoff
status: awaiting-merge-approval
---

## Completed

* `182-S` claimed (pre_claim / post_claim topology gates PASS); single-member
  manifest `[181.001-T]` unchanged. `181-F` deliberately NOT added.
* `181.001-T` done (auto-archived to `.backlogit/archive/` with `status: done`).
  * AC1 RED at `af3d13ab`: `TestUR3CrashReady_PartialMarkerIsNotReady` failed on
    the 0-byte marker with `unexpected end of JSON input` (the flake signature).
  * GREEN at `eabfee9d`.
  * AC2: `go test ./internal/core/ -run '^TestUR3' -count=25 -race -timeout 20m -v`
    PASS, 150/150, both named tests 25/25, 0 FAIL, 0 decode errors, 0 races.
  * AC3: `go test -race ./internal/core/...` PASS. Full `go test -timeout=30m ./...` PASS.
  * AC4/AC5 verified by diff review.

## Decisions

* RED and GREEN committed separately so the reproduction is visible in history.
* `golangci-lint run` reports 57 pre-existing issues on main; `--new-from-rev=bfe260a6`
  reports 0, so none are attributable to this change.

## Next steps

* Orchestrator merges with operator approval, then hands post-merge closure back to
  Ship: `ship_shipment` with the merge SHA, then the plan's `## Closure` 181-F step
  (confirm shipped provenance of `181.001-T` → `backlogit update 181-F --commit <sha>`
  → move done → archive), closure artifact, compact-context (P-020).
* `154-S` is untouched and must remain held until `182-S` has shipped provenance.
