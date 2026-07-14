---
description: "Compound-refresh report for shipment 092-S post-merge closure — captured the UTC-frontmatter NowUTC() helper pattern and the parallel-test-safe hermetic-TZ-subprocess RED-phase technique, and reinforced the Copilot review-loop convergence learning."
doc_type: closure
docline:
  ms.date: 2026-07-13T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-13-092-S-compound-refresh.md
title: "092-S compound-refresh report"
---

## Scope

Compound-library maintenance triggered by shipment 092-S (item-writer UTC
timestamp normalization), feature 103-F, PR #235, merge `4a90bf4`. Mode: apply.

## Entries evaluated

| Entry | Classification | Action |
|---|---|---|
| `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md` | create | New `doc_type: learning` entry: route every timestamp write site through one exported helper `models.NowUTC()` (= `time.Now().UTC()`) so `created_at`/`updated_at` serialize as canonical UTC with a trailing `Z`; keep the read/parse path offset-tolerant so historical `+/-hh:mm` artifacts still load; export the helper from the lowest package (`models`) so `core/templates` and `cli` reuse it without an import cycle; assert the exact `Z`, not a zero offset (`+00:00`). |
| `docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md` | create | New `doc_type: learning` entry: to prove (RED) that local-offset emission fails even on a UTC CI runner, run the write under a controlled non-UTC zone; a process-global `time.Local` override is a **data race** in any `t.Parallel()` package (notably `internal/cli`), so use a hermetic subprocess — re-exec the test binary at a helper test with `TZ=America/Los_Angeles` in the child env, emit the serialized timestamp on stdout, and assert `HasSuffix(value, "Z")` in the parent. Serial packages may use a scoped `time.Local` override with defer-restore. |
| `docs/compound/2026-07-13-copilot-review-loop-convergence.md` | reinforce | Added "Reinforcement — 092-S" section: the 36-file feature PR #235 hit the same clean first-HEAD fixed point (Copilot `COMMENTED`, "36/36 files, no comments", 0 threads, 0 review-fix cycles); a second, larger data point that the operative variable is upstream hardening (exhaustive plan + explicit parallel-safe RED design), not luck. Contrasted the staging PR's pre-hardening multi-cycle loop. |
| `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md` | create | New `doc_type: learning` entry (severity high) captured from the closure PR #236 incident: the post-merge `ship_shipment` ran a workspace `backlogit.exe` built ~11h **before** the merge, so it stamped `updated_at` in local `-07:00` — re-emitting the very defect 092-S closed. Rule: before any post-merge operation that WRITES artifacts, rebuild the tool from merged HEAD (or verify the binary's embedded VCS revision matches HEAD — never file mtime, which cannot establish provenance), verify the write path, and on a stale-write repair instant-preservingly + record transparently. |

## Stale / low-signal review

No existing compound entries were invalidated or made stale by 092-S. The
091-S follow-up recommendation "normalize backlogit CLI `created_at`/`updated_at`
to UTC `Z`" is now **realized** by this shipment; two of the new learnings
(`utc-frontmatter-timestamp-normalization`, `parallel-test-safe-tz-subprocess-red-phase`)
capture that durable technique, and a **third** new learning
(`post-merge-lifecycle-requires-fresh-binary`) captures the distinct
closure-process lesson from this shipment's stale-binary incident (three created
entries total; see Files touched). The Copilot review-loop learning is
*reinforced*, not superseded. No deletions or archival needed.

## Files touched

* Created: `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md`
* Created: `docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md`
* Created: `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`
* Updated: `docs/compound/2026-07-13-copilot-review-loop-convergence.md`

All pass `backlogit docs lint` (default profile / CI Docline gate — 0 findings).

## Recommendation

PROCEED — compound library is consistent and current for the UTC-timestamp
serialization, timezone-hermetic testing, and PR-automation domains.
