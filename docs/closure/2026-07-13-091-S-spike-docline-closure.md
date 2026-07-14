---
description: "Post-merge operational closure for shipment 091-S — reconciled the spike skill findings-artifact frontmatter example with the docline base-frontmatter v1 contract."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-13T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-13-091-S-spike-docline-closure.md
title: "091-S spike findings-artifact docline reconciliation closure"
---

## Outcome

Shipment `091-S` reconciled the `spike` skill's **Phase 5 "Write Findings
Artifact"** YAML frontmatter example with the docline base-frontmatter v1 contract,
surgically and without a broad plugin content-sync. Merged via **PR #231**, merge
commit **`ec2b859`** (true merge commit; parents `3514def` + `222052a`; P-009
satisfied).

* Feature `102-F` (covering) — archived, records merge `ec2b859`.
* Task `102.001-T` — archived, records merge `ec2b859`. Implementation commit
  `fd0a30b`.
* Shipment `091-S` — `shipped` / archived (`ship_shipment --sha ec2b859`,
  `archived_ids: [102.001-T, 091-S, 102-F]`).

## Change

The Phase 5 fenced ` ```markdown ` example was replaced **identically** in both
in-repo copies (`plugin/skills/spike/SKILL.md`, `.github/skills/spike/SKILL.md`):

* Added required top-level `source` and `doc_type: decision` (+ optional
  `description`).
* Moved the eight non-contract keys (`type`, `date`, `time_box`, `conclusion`,
  `confidence`, `linked_parent_work_item`, `promoted_to`, `tags`) under the
  `docline:` namespace (4-space indent, matching gold-standard artifacts
  `docs/decisions/2026-05-05-telemetry-gap-analysis-spike.md` and
  `docs/decisions/2026-07-09-github-actions-cost-spike.md`).

The output path `docs/decisions/{...}-spike.md` maps to `doc_type: decision`
(verified via `backlogit docs classify`). No surrounding prose changed.

## Verification

* `backlogit docs classify docs/decisions/2026-07-13-scratch-spike.md` → `decision`.
* A scratch artifact authored from the reconciled example passed
  `backlogit docs lint --profile authoring` — **0 findings**.
* Full-repo `backlogit docs lint` (CI Docline gate) — **0 findings**.
* `make verify-plugin` (`TestPluginBundleStructurallyValid`) — pass (SKILL.md
  `name`/`description` frontmatter untouched).
* `go test ./...`, `go vet ./...`, `golangci-lint run` — pass (no Go files changed).
* CI on PR #231 — all required checks green (`Detect code changes`, `test`,
  `Docline frontmatter gate`).

**Runtime verification:** N/A — no runtime surface changed (instructional
skill-example doc). The functional equivalent — an agent authoring from the
example produces docline-conformant output — was verified via the authoring-profile
lint of the scratch artifact (0 findings).

## Review

Local `review` gate (report-only; Template Integrity + Constitution lenses):
**0 P0/P1/P2 findings**, P3 advisories only. GitHub Copilot review: clean
(COMMENTED, "6/6 files, no comments", 0 threads, on the merged HEAD). §1.9
pre-merge readiness gate passed all three checks. Review-fix cycles: 0/3.

## GI/GR reconciliation

* Pre-mode: both members `pre-archived`, no orphans → PROCEED
  (`.backlogit/reconcile/091-S-pre-20260713-153100.md`).
* Post-mode: shipment + both members present in archive, 0 archive deletions
  (P-007 guard clean) → PROCEED
  (`.backlogit/reconcile/091-S-post-20260713-153145.md`).

## Retained scratch file (operator decision)

`docs/decisions/2026-07-13-scratch-spike.md` (the verification artifact authored
from the reconciled example) is **retained, untracked, and uncommitted**. Operator
did NOT approve deletion (Principle VII — Destructive Command Approval,
NON-NEGOTIABLE). Disposition: **retained pending a future operator decision.**

## Accepted follow-ups (from PR #230 + 091-S review)

1. **Memory-doc provenance** — RESOLVED in this closure:
   `docs/memory/2026-07-13/spike-findings-docline-stage-memory.md` corrected to state
   the plan's "Plan Review" is an inline single-agent Stage self-assessment, NOT the
   formal multi-persona `plan-review` skill; no formal gate evidence exists.
2. **Plan wording vs Ship lifecycle** — the plan's acceptance-criterion 5 /
   verification step 4 ("two files only" / `git diff --stat`) apply to the
   *implementation diff*; `.backlogit/` claim/ship lifecycle mutations are expected
   and separate (documented; no further action).
3. **Upstream template drift** — follow-up stash `7F0A6E89` (low): update
   `spike/SKILL.md.tmpl` in the external autoharness repo so regeneration re-applies
   this fix (Principle IV — out-of-tree, deferred).
4. **CLI timestamp normalization** — recommended separate task: normalize backlogit
   `created_at`/`updated_at` emission to UTC `Z` across item writers (currently
   local-offset for items, UTC for stash). Not tackled here.
5. **`.github/skills/spike/SKILL.md` missing `name:` key** (P3, pre-existing) —
   surfaced by the Template Integrity review; the `.github` twin's own top-level
   frontmatter lacks the `name: spike` key present in the plugin copy. NOT introduced
   by 091-S; recommend tracking separately if skill-name parity is required.

## Compound learnings

* Reinforced `docs/compound/2026-06-26-docline-frontmatter-contract.md` (§091-S:
  instructional example blocks are generators too).
* Created `docs/compound/2026-07-13-copilot-review-loop-convergence.md`.

Refresh report: `docs/closure/2026-07-13-091-S-compound-refresh.md`.

## Recommendation

**GO / CLOSED.** Shipment 091-S is merged, shipped, archived, and reconciled.
Remaining items are tracked follow-ups; none block closure.
