---
description: "Ship session memory for shipment 092-S — item-writer UTC timestamp normalization: build (11 TDD tasks), review, PR #235, merge 4a90bf4, and post-merge closure."
doc_type: memory
docline:
  ms.date: 2026-07-13T00:00:00Z
  ms.topic: memory
source: docs/memory/2026-07-13/092-S-item-writer-utc-ship-memory.md
title: "092-S item-writer UTC timestamp normalization — Ship session memory"
---

## Outcome

`ship next` executed queued shipment `092-S` end-to-end and drove its post-merge
closure. Merged via **PR #235**, merge commit **`4a90bf4`** (true merge commit,
parents `f19cd01` + `fdbd8bd`). Shipment `092-S` shipped/archived; feature `103-F`
and tasks `103.001-T`…`103.011-T` archived recording `4a90bf4`. Every
item-artifact writer now emits `created_at`/`updated_at` in canonical UTC (`Z`);
the read path stays offset-tolerant.

## Task IDs completed

* `103.001-T` — `models.NowUTC()` foundation helper + `ArtifactFromFrontmatter`
  UTC defaults (all other tasks depend on this).
* `103.002-T`…`103.010-T` — swap `time.Now()` → `models.NowUTC()` at each writer
  site (artifacts, queue, shipment, shipment_lifecycle commit/status/cascade,
  gate_transition, artifact_references, migrate_links, templates/service, cli
  update --section).
* `103.011-T` — `shipment_lifecycle.go` `clearParentID` / `AdoptItem` (second
  logical site in that file).
* `103-F` (covering feature) — done/archived.
* `092-S` (shipment) — shipped (`ship_shipment --sha 4a90bf4`).

## Files modified

**Implementation (feat branch `feat/item-writer-utc-timestamp-normalization`,
merged in `4a90bf4`):**

* Production (10 files, 11 sites): `internal/models/frontmatter.go`,
  `internal/core/{artifacts,queue,shipment,shipment_lifecycle,gate_transition,artifact_references,migrate_links}.go`,
  `internal/core/templates/service.go`, `internal/cli/update.go`.
* Tests (12 `*_utc_test.go`): across `models`, `core`, `core/templates`, `cli`,
  plus `internal/core/utc_whitebox_test.go`.

**Closure (branch `post-merge/092-S`):**

* `.backlogit/archive/092-S.md`, `103-F.md`, `103.001-T`…`103.011-T.md` —
  shipped/archived, merge SHA `4a90bf4`; `.backlogit/hooks_queue.jsonl` appends.
* `docs/closure/2026-07-13-092-S-item-writer-utc-closure.md`,
  `docs/closure/2026-07-13-092-S-compound-refresh.md`.
* `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md` (new),
  `docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md` (new),
  `docs/compound/2026-07-13-copilot-review-loop-convergence.md` (reinforced §092-S).
* `docs/memory/2026-07-13/092-S-item-writer-utc-ship-memory.md` (this file).

## Decisions

* **Shared exported helper in the lowest package.** Put `NowUTC()` in
  `internal/models` and **export** it so `core/templates` and `cli` reuse it with
  no import cycle. Single normalization point > per-site `time.Now().UTC()`.
* **Normalize on write, stay liberal on read.** Kept `ArtifactFromFrontmatter`
  accepting any-zone `time.Time` so historical offset artifacts still load —
  backward-compatible, not a corpus migration.
* **Parallel-test-safe RED phase.** For `internal/cli` (`t.Parallel()`), used a
  hermetic `TZ=America/Los_Angeles` subprocess re-exec instead of a process-global
  `time.Local` override (which would be a data race). Serial packages used a
  scoped override. Asserted the exact trailing `Z`, not a zero offset.
* **Closure via a dedicated PR** (`post-merge/092-S` → PR) because direct pushes
  to `main` are ruleset-blocked (PR + required checks + Copilot review-on-push
  with required thread resolution).
* **No formal plan-review gate** was run (Stage did an inline self-assessment
  only). Proceeded; Ship's own `review` gate ran normally. Recorded the absence
  honestly rather than implying a satisfied gate.

## Failed approaches / gotchas

* A process-global `time.Local` override in a `t.Parallel()` package is a **data
  race** (`go test -race` trips it); the hermetic subprocess is the safe variant.
* Asserting a zero *offset* (`+00:00`) is weaker than asserting `Z` — a writer
  that computes UTC but serializes with a numeric offset would slip through.
  Assert `HasSuffix(v, "Z")` **and** `!matches [+-]\d{2}:\d{2}$`.
* `ship_shipment` **overwrites** members' `commit` field with the merge SHA;
  `4a90bf4` is now recorded on all 13 archived members.
* `backlogit get <id>` prints `created_at`/`updated_at` **display-formatted in
  local time** (e.g. `-0700 PDT`); the stored frontmatter is UTC `Z`. Display ≠
  storage.
* There is **no `backlogit reconcile` subcommand** — GI/GR reconcile is a manual
  verification gate; `ShipShipment` runs `VerifyPostShipConsistency` internally.
* `docs/memory/` is **excluded** from the docline scope, so this memory file is
  not gate-checked; `docs/closure/` (→ `closure`) and `docs/compound/`
  (→ `learning`) **are** gate-checked by `make docs-lint` (default profile).
* Backlogit **MCP tools resolve the installed-plugin workspace**, not the repo
  root — use the repo CLI (`.\backlogit.exe … --cwd .`) for repo backlog work.

## Verification

* Constitution gates: `go test ./...`, `go vet ./...`, `golangci-lint run` pass;
  `gofmt -l .` clean. Runtime: binary emits `Z` under non-UTC `TZ`. CI on PR #235:
  all required checks green. Docline gate: 0 findings.
* GI/GR reconcile: pre PROCEED (11 tasks done; feature/shipment active =
  expected), post PROCEED (all archived, merge SHA recorded, 0 orphans).
* Review: local gate no P0/P1 (one P3 fixed); Copilot clean (36/36, 0 threads);
  §1.9 pass; 0/3 review-fix cycles.

## Follow-ups (open)

* **093-S** — next release unit; eligible only after 092-S closure completes
  (P-001).
* Stash `7F0A6E89` (low) — out-of-tree upstream `spike/SKILL.md.tmpl` (Principle
  IV, carried from 091-S).
* MCP-workspace environment finding — MCP tools resolve installed-plugin
  workspace, not repo root.
* Retained scratch file `docs/decisions/2026-07-13-scratch-spike.md` — untracked,
  awaiting a future operator deletion decision (Principle VII).

## Next steps

Closure PR from `post-merge/092-S` is prepared; awaiting operator merge approval
(P-014 / §1.10). Do NOT self-merge; do NOT start 093-S until 092-S closure lands.
