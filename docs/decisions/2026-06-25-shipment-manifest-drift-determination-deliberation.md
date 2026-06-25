---
title: "Shipment Manifest Drift (060/061/062): Benign Counter Offset vs. Genuine Defect"
description: "Reviewed determination that the 060-S/061-S queued-shipment off-by-one is a benign, cosmetic ID-numbering artifact, not a data defect requiring manifest mutation"
topic: "Off-by-one drift in queued shipment manifests 060-S/061-S and feature titles 061-F/062-F (stash B8FF7590)"
depth: "standard"
decision_status: "decided"
promoted_to: "none"
linked_artifacts:
  - "docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md"
  - "docs/exec-plans/2026-05-22-shipment-state-integrity-plan.md"
  - "docs/exec-plans/2026-05-22-metadata-section-sync-integrity-plan.md"
  - "docs/exec-plans/2026-05-22-lifecycle-archive-rollback-plan.md"
stash_ids:
  - "B8FF7590"
tags:
  - "shipment-state"
  - "data-integrity"
  - "manifest-drift"
  - "id-allocation"
  - "least-mutation"
  - "no-op-decision"
---

## Problem Frame

Stash `B8FF7590` (kind=bug, priority=high) reports an "off-by-one drift" in the
two queued shipment manifests `060-S` and `061-S`, claiming each shipment's
`custom_fields.items` and `title` are shifted by one feature relative to their
shipment number. The stash asserts the **impact** is that "claiming `060-S` or
`061-S` would ship the wrong feature hierarchy," and proposes a data repair that
re-aligns each shipment's `items`/`title` and "reconciles" the `061-F`/`062-F`
feature titles.

The bug was split out of the now-shipped root-ID code-hardening work (stash
`0F65FBC9`, shipped via `066-S` / merge `80ce5f12`) per the "Manifest Drift
Decision" in `docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md`,
explicitly to be planned and reviewed independently rather than mutated as a
side effect of a code shipment.

**Who cares:** the Ship agent (which claims and closes these queued shipments)
and harness operators relying on shipments to archive and commit-link the
correct feature hierarchy.

**The deciding question this deliberation must answer first:** is the offset a
**benign ID-numbering artifact** (independent shipment/feature counters, each
shipment internally self-consistent) that warrants a documented no-op — or a
**genuine defect** (orphan feature, empty/mis-titled shipment, mis-archival
risk) that warrants a reviewed, dependency-aware repair?

**Constraints:** Stage role boundary (P-010) — backlog/planning/docs plus
*reviewed* manifest data repair only; least-mutation and width-isolation
(Constitution); do not mutate live queued state speculatively; do not touch the
shipped/archived `066` cluster or the unrelated `065-S` shipment.

**Success criteria:** the determination is grounded in the referenced exec-plans
and live state; if benign, no live manifest is mutated and `B8FF7590` is closed
with a clear rationale; if defect, a minimal reviewed repair with a `doctor`
post-check.

## Research Findings

All findings verified live on `main @ 7222e337` (snapshot 2026-06-25), after
`backlogit sync` (621 artifacts indexed).

### Finding 1 — Feature ↔ exec-plan ↔ task-count mapping is internally perfect

| Feature | Title | Exec-plan | Plan units | Tasks | Status |
|---|---|---|---|---|---|
| `060-F` | Archive and Hierarchy Rollback Integrity | `2026-05-22-lifecycle-archive-rollback-plan.md` | 4 (R1–R4) | `060.001-T`..`060.004-T` (4) | archived / done |
| `061-F` | Shipment State Integrity | `2026-05-22-shipment-state-integrity-plan.md` | 2 units | `061.001-T`,`061.002-T` (2) | queued |
| `062-F` | Metadata and Section Sync Integrity | `2026-05-22-metadata-section-sync-integrity-plan.md` | 5 (R1–R5) | `062.001-T`..`062.005-T` (5) | queued |

Each feature's title matches its source plan's title, and each feature's task
count matches that plan's unit count exactly. The features are correctly named
and correctly populated. **The feature titles are not "wrong" or "duplicated by
accident" — they are the canonical titles of their respective exec-plans.**

### Finding 2 — Each queued shipment's title AGREES with its items' feature

Live `shipment list`:

| Shipment | Title | Status | `custom_fields.items` | Carries feature |
|---|---|---|---|---|
| `060-S` | Shipment State Integrity | queued | `["061-F","061.002-T","061.001-T"]` | `061-F` |
| `061-S` | Metadata and Section Sync Integrity | queued | `["062-F","062.001-T".."062.005-T"]` | `062-F` |

`060-S`'s title ("Shipment State Integrity") is identical to `061-F`'s title, and
its `items` reference `061-F` plus exactly `061-F`'s two tasks. `061-S`'s title
("Metadata and Section Sync Integrity") is identical to `062-F`'s title, and its
`items` reference `062-F` plus exactly `062-F`'s five tasks.

**This mutual title↔items agreement is the decisive signal.** A genuine
mis-assembly (wrong items pasted into a shipment) would produce a shipment whose
title and items *disagree* about which feature they describe. Here they agree
perfectly. The shipment correctly carries the next un-shipped feature, correctly
titled — only the shipment *number* lags the feature *number* by one.

### Finding 3 — The offset is a known, working pattern: precedent `057-S` (done)

`057-S` "Branch-Level Telemetry Metrics" has status **done** and
`items = ["058-F","058.001-T".."058.006-T"]`, with the description stating
verbatim: *"6 tasks under feature 058-F."* A shipment numbered `057` successfully
shipped feature `058` to `done` with the identical `+1` offset and zero ill
effect. The offset is not novel and not failure-inducing — it has already
round-tripped a shipment through the full lifecycle.

### Finding 4 — There is NO shipment-number = feature-number invariant

`065-S` "Standardize documentation frontmatter…" has
`items = ["065-F","065.001-T".."065.011-T"]` — shipment `065` carries feature
`065`, with **no offset at all**. Across the live and recently-shipped
shipments, the relationship between shipment number and feature number is:

* `057-S` → `058-F` (offset +1)
* `060-S` → `061-F` (offset +1)
* `061-S` → `062-F` (offset +1)
* `065-S` → `065-F` (offset 0)

The offset is not constant, therefore it is not an invariant being violated.
Shipment IDs and feature IDs are allocated from **independent monotonic
counters**; whether they happen to align is coincidental. A shipment that "ships
feature N+1" is exactly as correct as one that "ships feature N" — the binding is
the `items` array, not the numeric label.

### Finding 5 — `doctor` is clean, including the new 066 root-ID audit

`backlogit doctor` reports **"No issues found."** The root-ID conflict-integrity
audit shipped in `066-S` (merge `80ce5f12`) — which now scans canonical files for
duplicate IDs, orphans, and archive-collision hazards — surfaces nothing on the
060/061/062 cluster. There is no orphan feature, no duplicate ID, no empty
shipment, and no broken parent linkage.

### Finding 6 — `060-F` is already archived/done; it cannot be the intended target of `060-S`

`060-F` and its tasks `060.001-T`..`060.004-T` are archived/done (shipped earlier).
A shipment cannot ship an already-shipped feature. So the hypothesis "`060-S`
*should* carry `060-F`" is impossible by construction — `060-S` carrying `061-F`
(the next un-shipped feature) is the only coherent intent. Every currently queued
feature (`061-F`, `062-F`) is covered by exactly one shipment (`060-S`, `061-S`);
there is no missing `062-S` because there is no third queued feature to cover.

### Prior knowledge (compound library)

`docs/compound/go-patterns/f015-shipment-stash-patterns.md` documents that
shipment `items` round-trip through SQLite as JSON and are normalized at the read
edge — i.e. `items` membership is the authoritative binding, consistent with
Finding 4. No prior learning describes a shipment/feature counter-offset defect.
Learnings retrieval confidence: low (no directly-applicable prior solution).

## Options Evaluated

### Option 1 — Benign counter offset → documented no-op (recommended)

Record the determination in this decision artifact, mutate no live manifest, and
archive `B8FF7590` as resolved-benign with a forward reference to this doc.

* **Pros:** Honors least-mutation and width-isolation. Preserves the currently
  *correct* title↔items↔plan alignment. Zero blast radius on live queued state.
  Avoids introducing a Git merge-conflict surface on tracked manifests. Matches
  all live evidence (Findings 1–6).
* **Cons:** Leaves a cosmetic shipment#≠feature# numeric offset in place (which
  precedent `057-S` proves is harmless). A future reader may momentarily expect
  numeric alignment; mitigated by this durable decision record.
* **Effort:** low. **Fit:** high.

### Option 2 — Treat as defect → rewrite `060-S`/`061-S` items+titles and feature titles

Apply the stash's proposed repair: re-align shipment `items`/`title` to the
shipment number and "reconcile" `061-F`/`062-F` titles.

* **Pros:** Would make shipment number equal feature number (cosmetic tidiness).
* **Cons:** **Actively harmful.** It would break the verified-correct
  title↔feature↔plan alignment (Findings 1–2): e.g. renaming `061-F` away from
  "Shipment State Integrity" would desynchronize it from its own exec-plan and
  tasks. It mutates live queued shipment state the Ship agent may schedule, for
  no functional benefit, expanding blast radius and adding a merge-conflict
  surface (Constitution IX). It is premised on an impact claim (Finding 6) that
  is demonstrably false.
* **Effort:** medium (and risky). **Fit:** low.

### Option 3 — Benign, but add `related_to` shipment→feature semantic links

Option 1 plus explicit `related_to` links (`060-S`→`061-F`, `061-S`→`062-F`).

* **Pros:** Slightly more explicit traceability for a casual reader.
* **Cons:** Redundant — the `items` array *already* encodes shipment→feature
  membership authoritatively. Still mutates live queued shipment metadata,
  weakening the least-mutation posture for marginal value.
* **Effort:** low. **Fit:** medium.

## Trade-off Comparison

| Criterion | Option 1 (no-op) | Option 2 (rewrite) | Option 3 (no-op + links) |
|---|---|---|---|
| Matches live evidence | Yes | No (contradicts Findings 1–6) | Yes |
| Preserves correct title↔plan alignment | Yes | **No — breaks it** | Yes |
| Mutates live queued state | No | Yes (high) | Yes (low) |
| Blast radius | None | Elevated | Low |
| Constitution IX merge risk | None | Elevated | Low |
| Resolves the actual reported risk | Yes (shows it is a non-risk) | No (none existed) | Yes |
| Least-mutation / width-isolation | Strong | Violated | Weakened |

## Decision

**The 060/061/062 shipment-manifest "drift" is a BENIGN, cosmetic ID-numbering
artifact, not a data defect. Adopt Option 1: documented no-op.**

Each queued shipment correctly carries the next un-shipped feature's complete
hierarchy under a title that matches that feature and its exec-plan. The only
artifact is a numeric offset between two independent counters (shipment vs.
feature), which precedent `057-S` (done) proves ships correctly and which `065-S`
(offset 0) proves is not an invariant. `doctor` is clean. No orphan, no empty
shipment, no mis-title, no mis-archival path.

The stash's stated impact — "claiming `060-S` or `061-S` would ship the wrong
feature hierarchy" — is **inaccurate**. Claiming `060-S` ships `061-F`'s real,
coherent, correctly-titled hierarchy; claiming `061-S` ships `062-F`'s. That is
the intended behavior.

**Actions:**

1. No mutation of `060-S`, `061-S`, `061-F`, or `062-F` manifests.
2. Archive stash `B8FF7590` as resolved-benign with a rationale comment
   referencing this decision artifact.
3. No harvest and no shipment assembly — there is no actionable backlog work.

The intra-shipment task ordering quirk in `060-S` (`061.002-T` listed before
`061.001-T`) is cosmetic and parent-first is preserved (`061-F` first); it is not
worth a live mutation and is noted here for completeness.

## Rejected Alternatives

* **Option 2 (rewrite manifests/titles):** rejected — it would *introduce* a
  defect by desynchronizing feature titles from their exec-plans and tasks, and
  mutates live queued state to "fix" a non-problem. The proposed repair is
  premised on a false impact claim.
* **Option 3 (add semantic links):** rejected — `items` already encodes
  shipment→feature traceability; adding links mutates live state for negligible
  benefit and weakens the least-mutation posture. This decision record is the
  durable annotation.
* **Renumbering shipments/features to force alignment:** rejected — root IDs are
  immutable anchors for commit-tracking and archive history; renumbering would be
  the highest-blast-radius option and is explicitly out of scope.

## Unresolved Questions

1. **Cosmetic numbering ergonomics (non-blocking):** if operators find the
   shipment#≠feature# offset confusing in future, the durable fix is a *forward*
   convention/UX change (e.g. surfacing the covering feature ID in shipment
   listings), not a retroactive manifest rewrite. Out of scope here; flagged for
   the operator's awareness only.
2. **Detection going forward:** the root-ID/doctor audit from `066-S` does not —
   and need not — flag this offset, because it is not an integrity violation. No
   change requested.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| A future reader re-files the same "drift" bug | This decision record is durable and cross-referenced from the archived stash entry; the benign rationale and evidence are preserved |
| Ship claims `060-S`/`061-S` and is surprised by the number offset | The shipment `items` are authoritative and correct; closing the shipment archives and commit-links the right feature hierarchy regardless of the numeric label (precedent `057-S`) |
| Operator still prefers cosmetic alignment | Escalated as Unresolved Question 1 — handle as a forward convention, never a retroactive live-manifest rewrite |
