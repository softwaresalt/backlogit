---
description: "Post-merge operational closure for shipment 092-S — normalized every item-artifact writer's created_at/updated_at frontmatter emission to canonical UTC (trailing Z) via a shared models.NowUTC() helper, with a backward-compatible offset-tolerant read path."
doc_type: closure
docline:
  ms.date: 2026-07-13T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-13-092-S-item-writer-utc-closure.md
title: "092-S item-writer UTC timestamp normalization closure"
---

## Outcome

Shipment `092-S` normalized item-artifact timestamp emission: every writer now
serializes `created_at` / `updated_at` frontmatter in canonical UTC with a
trailing `Z` instead of a machine-local offset, while the read/parse path stays
backward-compatible with historical `+/-hh:mm` timestamps. Merged via **PR #235**,
merge commit **`4a90bf4`** (true merge commit; parents `f19cd01` + `fdbd8bd`;
P-009 satisfied). This realizes the CLI-timestamp-normalization follow-up
recommended out of 091-S.

* Feature `103-F` (covering) — archived, records merge `4a90bf4`.
* Tasks `103.001-T` … `103.011-T` (11) — archived, record merge `4a90bf4`.
* Shipment `092-S` — `shipped` / archived (`ship_shipment --sha 4a90bf4`,
  `archived_ids: [103.001-T … 103.011-T, 092-S, 103-F]`, `returned_ids: []`).

## Change

Added an exported helper `NowUTC()` in `internal/models/frontmatter.go`
(born-compliant doc comment; `func NowUTC() time.Time { return time.Now().UTC() }`)
and routed **every** item-timestamp write site through it. Exporting from the
lowest package (`models`) lets `core/templates` and `cli` reuse it without an
import cycle. The read path (`ArtifactFromFrontmatter`) still accepts any parsed
`time.Time` regardless of zone, so already-emitted offset timestamps load
unchanged (normalize on write, stay liberal on read).

**Writer sites changed — 11 logical sites across 10 production files (inventory
confirmed complete against the merged diff):**

| Task | File | Site |
|---|---|---|
| 103.001-T | `internal/models/frontmatter.go` | `NowUTC()` helper + `ArtifactFromFrontmatter` default `created_at`/`updated_at` |
| 103.002-T | `internal/core/artifacts.go` | artifact write |
| 103.003-T | `internal/core/queue.go` | queue write |
| 103.004-T | `internal/core/shipment.go` | shipment write (kept `time` import for event-log formatting) |
| 103.005-T | `internal/core/shipment_lifecycle.go` | commit/status/cascade fns |
| 103.006-T | `internal/core/gate_transition.go` | gate transition stamp |
| 103.007-T | `internal/core/artifact_references.go` | reference write |
| 103.008-T | `internal/core/migrate_links.go` | link-migration stamp |
| 103.009-T | `internal/core/templates/service.go` | direct serializer |
| 103.010-T | `internal/cli/update.go` | `update --section` path stamp |
| 103.011-T | `internal/core/shipment_lifecycle.go` | `clearParentID` / `AdoptItem` (second logical site in the same file) |

12 TDD RED-phase test files accompanied the change (`*_utc_test.go` across
`models`, `core`, `core/templates`, `cli`, plus `utc_whitebox_test.go`). No
additional stamping sites were found beyond the plan's inventory.

## Verification

* `go test ./...` — pass. `go vet ./...` — pass. `golangci-lint run` — pass.
  `gofmt -l .` — clean. (Full Constitution quality-gate sequence, run untruncated.)
* **Runtime verification (write path, including shipment/archival):** the binary
  built from the merged source, run under a non-UTC zone, emits freshly-written
  frontmatter timestamps ending in `Z` (not a local offset). The merged UTC test
  suite covers every write site — including the shipment/archival lifecycle path
  (`shipment_lifecycle_utc_test.go`, `shipment_lifecycle_adopt_utc_test.go`) — and
  passes, proving `ship_shipment` / `attachCommitToItems` stamp `updated_at` in
  canonical `Z` under a controlled non-UTC zone. Historical timestamps already on
  disk are preserved by the offset-tolerant read path.
* **RED-phase design (parallel-test-safe):** for the `t.Parallel()` package
  `internal/cli` (task 103.010-T), the RED test drives a hermetic subprocess
  (`exec.Command(os.Args[0], "-test.run=^TestHelperUpdateSectionUTCChild$")` with
  `TZ=America/Los_Angeles`) and asserts the emitted timestamp ends with exactly
  `Z` — **no** process-global `time.Local` mutation in a parallel package. Serial
  packages use a scoped override. Assertions check the exact `Z`, not a zero
  offset (`+00:00`). This is the "deferred build-time note" — already implemented
  in `internal/cli/update_utc_test.go`, not outstanding.
* CI on PR #235 — all required checks green (`Detect code changes`, `test`,
  `Docline frontmatter gate`).

### Closure lifecycle repair — stale-binary incident (transparent record)

The initial post-merge `ship_shipment 092-S --sha 4a90bf4` was run with a
workspace `backlogit.exe` that had been **built ~11h before the merge commit**,
so it predated the `NowUTC()` normalization and stamped the newly-written
archive `updated_at` fields with a local `-07:00` offset — surfaced by the
closure PR #236 Copilot review. Root cause and repair:

* **Root cause fixed:** rebuilt `backlogit.exe` from the merged source
  (`go build -o backlogit.exe ./cmd/backlogit`); the fresh binary carries the
  UTC-emission code path.
* **Ship path re-verified under a non-UTC zone:** the merged UTC test suite
  (`go test ./internal/{core,models,cli}/... -run UTC`) passes, proving the
  shipment/archival lifecycle writers emit `Z` under a controlled zone.
* **Output repaired (instant-preserving):** the 13 newly-archived members'
  `updated_at` values were normalized to their exact UTC `Z` equivalents (same
  instant; e.g. `2026-07-13T22:33:28.7497461-07:00` → `2026-07-14T05:33:28.7497461Z`),
  matching the byte-form the merged serializer produces. Historical `created_at`
  values (and the broader pre-092-S archive corpus) are **intentionally
  preserved** as-is — that is precisely the backward-compatible, offset-tolerant
  read path this shipment guarantees; 092-S normalizes emission on **write**, it
  does not migrate historical data.

## Review

Local `review` gate (report-only; Go + Constitution lenses): **no P0/P1
findings**; one P3 advisory addressed. GitHub Copilot review: clean (`COMMENTED`,
"36/36 files, no comments", 0 threads) on the merged HEAD. §1.9 pre-merge
readiness gate passed all checks. **Review-fix cycles: 0/3** — the PR reached a
clean first-HEAD fixed point, attributable to upstream plan hardening (exhaustive
writer-site inventory + explicit parallel-safe RED design). See
`docs/compound/2026-07-13-copilot-review-loop-convergence.md` (§092-S).

## Plan-review provenance (honest)

The implementation plan's "Plan Review" section is an inline single-agent Stage
self-assessment, **not** the formal multi-persona `plan-review` skill (this
environment could not dispatch reviewer personas). No formal plan-review gate was
satisfied. Ship ran its own `review` gate normally before the PR; the absence of
a formal plan-review is recorded here rather than implied as satisfied.

## GI/GR reconciliation

* **Pre-mode:** all 11 tasks `done`; feature `103-F` `active` and shipment `092-S`
  `active` — the expected pre-ship state (`ShipShipment` itself transitions the
  feature and items to done, so the parent was intentionally not manually
  cascaded). No orphans → PROCEED.
* **Post-mode:** `ship_shipment 092-S --sha 4a90bf4` returned `shipped`,
  `archived_ids` = all 11 tasks + `092-S` + `103-F`, `returned_ids: []`; the
  built-in `VerifyPostShipConsistency` passed. Confirmed each member `archived`
  with `commit: 4a90bf4` recorded and `092-S` removed from the queue → PROCEED.

## Scope note — `.backlogit/` lifecycle mutations

Claiming and shipping 092-S wrote expected `.backlogit/` lifecycle artifacts
(queue→archive moves for the 13 members, `commit`/status updates,
`hooks_queue.jsonl` appends). These are expected and separate from the
implementation diff; "implementation-only" assertions scope to the Go
production/test files.

## Retained scratch file (operator decision)

`docs/decisions/2026-07-13-scratch-spike.md` is **retained, untracked, and
uncommitted**. Operator did NOT approve deletion (Principle VII — Destructive
Command Approval, NON-NEGOTIABLE). Disposition: **retained pending a future
operator decision.** Unrelated to 092-S.

## Compound learnings

* Created `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md`
  (shared `NowUTC()` helper + backward-compatible parse path).
* Created `docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md`
  (hermetic `TZ` subprocess RED phase for `t.Parallel()` packages).
* Reinforced `docs/compound/2026-07-13-copilot-review-loop-convergence.md`
  (§092-S: 36-file clean first-HEAD convergence; upstream hardening beats
  after-the-fact thread resolution).
* Created `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`
  (post-merge lifecycle writes must use a binary built from merged HEAD — captured
  from the closure PR #236 stale-binary incident, see the repair note above).

Refresh report: `docs/closure/2026-07-13-092-S-compound-refresh.md`.

## Remaining backlog / follow-ups (open)

* **093-S** — next release unit; eligible only **after** 092-S closure completes
  (P-001: one release unit at a time). Do not start until closure lands.
* Stash **`7F0A6E89`** (low) — out-of-tree upstream `spike/SKILL.md.tmpl` update
  in the external autoharness repo (Principle IV — deferred, carried from 091-S).
* **MCP-workspace environment finding** — backlogit MCP tools resolve the
  installed-plugin workspace, not the repo root; continue using the repo CLI
  (`.\backlogit.exe … --cwd .`) for repo backlog work.

## Recommendation

**GO / CLOSED.** Shipment 092-S is merged, shipped, archived, and reconciled;
the merge SHA is recorded on all members. Remaining items are tracked follow-ups;
none block closure. This closure itself ships via a dedicated post-merge PR
(direct pushes to `main` are ruleset-blocked), halting at operator merge approval
(P-014).
