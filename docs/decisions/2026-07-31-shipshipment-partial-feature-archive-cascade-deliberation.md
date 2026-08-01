---
title: "Durable fix for ShipShipment archival cascade over-archiving covering features"
description: "Deliberation on the covering-feature scope and durable-fix direction for the partial-feature shipment archival cascade in core.ShipShipment (stash C0909DB5)"
source: docs/decisions/2026-07-31-shipshipment-partial-feature-archive-cascade-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "core.ShipShipment cascade over-archives ancestor covering features on partial-feature shipments (C0909DB5)"
depth: "standard"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-07-31-shipshipment-partial-feature-archive-cascade-plan.md"
tags:
  - "bug"
  - "shipment"
  - "lifecycle"
  - "archive"
  - "cascade"
  - "stage"
stash_ids:
  - "C0909DB5"
---

## Problem Frame

`core.ShipShipment` (`internal/core/shipment_lifecycle.go:135`) over-archives the
covering feature when a shipment covers only part of that feature's work (a
**partial-feature shipment** — the feature intentionally stays open for later
units). Two functions combine to produce the defect:

* `featureScopeRoots` (`:423`) walks `parent_id` upward from **every** manifest
  item and collects **every** ancestor `feature`.
* `collectArchiveCandidateIDs` (`:292`) then appends each such feature to the
  archive set **unconditionally** — the only guard is
  `feature.Status != models.StatusArchived`. This is unlike the sibling
  descendant-item branch, which **is** gated by `isTerminalReleaseStatus`.

Consequently, shipping the first sub-shipment of a multi-cycle feature **always**
archives the covering feature. On shipment `114-S` (feature `106-F`),
`backlogit shipment ship 114-S` returned
`archived_ids: [106.001-T, 106.002-T, 106-F, 114-S]` even though the manifest
`items` were only the two tasks `[106.001-T, 106.002-T]` — `106-F` was pulled in
solely by the parent-walk. Unshipped siblings escape archival only when they are
**nonterminal** (because `returnUnreleasedFeatureItems` (`:260`) detaches them —
returns them to `queued` and clears `parent_id` — before
`collectArchiveCandidateIDs` runs); planned-but-unharvested units (`106-F`'s
F1/F4/F5/F6) have **no records at all**.

**Who cares and why.** Every dark-factory / multi-cycle feature that ships in
more than one shipment is corrupted the moment its first shipment is shipped: the
covering feature is archived out from under its remaining intended work. Today
this is contained only by the **procedural** P-015 single-artifact safe-close
policy (`.github/policies/workflow-policies.md`), which requires the Ship agent to
*avoid* the cascade entirely and archive manifest items one at a time. C0909DB5
asks for the **durable product fix** so partial-feature safety is correct by
construction rather than dependent on an agent remembering a policy.

**Success criteria.**

1. Shipping a partial-feature shipment never archives the covering feature or any
   sibling not explicitly released by the manifest.
2. Shipping a genuinely complete feature still closes and archives it (no
   regression of the full-feature path).
3. The fix survives the hard case: planned-but-unharvested units (no descendant
   records) do not cause the covering feature to be treated as complete.
4. A regression test reproduces the exact `114-S` partial-feature scenario.
5. The `.ship.agent.md` Step 6.1.b drift (still instructing
   `backlogit_ship_shipment` as the default) is corrected so guidance matches the
   safe behavior.

**Scope boundaries (out of scope).**

* The optional shipment-*sequencing*/ordering enhancements in stash `0B5FA82B`
  (queue_position, shipment→shipment blocking, shipment priority) — a distinct,
  deferred concern.
* The external-autoharness-repo template follow-ups (`7F0A6E89`, `6FA0829B`) —
  forbidden from this workspace by Constitution Principle IV.
* Broad refactors of the release-scope/gate pipeline beyond what the cascade fix
  requires.

## Research Findings

Prior art retrieved from `docs/compound/` and `docs/decisions/` via the
learnings-researcher (confidence: **high**). The most load-bearing findings:

* **`docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md` (114-S)** —
  the canonical statement of this exact bug. It already names the two durable
  directions (archive strictly by explicit membership, or a durable keep-open
  lifecycle signal) and proves that a **descendant-only** `CheckChildrenTerminal`
  gate is insufficient, because planned-but-unharvested units have no records and
  `returnUnreleasedFeatureItems` detaches harvested nonterminal descendants first.
* **`docs/decisions/2026-07-29-ship-sequence-manifest-spike.md`** — establishes
  that `ShipShipment` already derives `explicitScope = NormalizeShipmentItems(shipment)`
  (`shipment.go:568-625`) and the manifest `custom_fields.items` is the **single
  authoritative membership**. On `114-S` that membership was `[106.001-T,
  106.002-T]` — it did **not** include `106-F`. This directly supports the
  membership-based direction and warns that any *new* coordination metadata needs
  a clearly-owned source of truth.
* **`docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`** —
  two cautions the fix must respect: (a) "manifest exclusion is **not** scope
  exclusion" because `releaseScopeItemIDs` re-expands each manifest item to all
  descendants (`IncludeArchived:true`); and (b) `isTerminalReleaseStatus`
  (`{done,accepted,rejected,archived}`) is **imprecise** versus the canonical
  `core.TerminalStatuses` (`blocking_cascade.go:14`, which also treats
  shipped/abandoned as terminal). A deliberate predicate choice is required, and
  the five other `isTerminalReleaseStatus` callsites (release progression) must
  stay unchanged.
* **`docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`**
  and **`docs/decisions/2026-07-28-archive-repersist-projection-drop-deliberation.md`** —
  any change touching `collectArchiveCandidateIDs`/`attachCommitToItems` inherits
  the re-persist seam: the DB fast-path (`loadArtifact`) omits `item_links` and
  `archived_status`; authoritative state must be read from Markdown
  (`findArtifact`), and archived items must be skipped. Regression tests must
  distinguish "skip happened" from "reload masked the abort" (false-green).
* **`docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`** and
  **`docs/compound/go-patterns/f015-shipment-stash-patterns.md`** — critical for
  the keep-open-flag direction: the typed `Artifact` codec + `WriteArtifactFile`
  **silently drop unmodeled top-level frontmatter keys**, and `custom_fields`
  round-trips through SQLite as `[]interface{}` and must be normalized at every
  read edge. A keep-open flag must therefore either be modeled end-to-end
  (struct + writer + DB projection) or live under `custom_fields` with a
  defensive read-edge normalizer — a non-trivial persistence cost.
* **`docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md`**
  and **`docs/decisions/2026-06-26-archive-archived-from-self-reference-deliberation.md`** —
  the delivery-vehicle template for a durable archival correctness fix: derive
  archive-safety invariants from where a record **belongs** (manifest/queue), not
  where it currently is; ship a `doctor --check/--fix` audit for the invariant;
  and add the regression test for the exact path that previously had none
  (here: partial-feature ship).

**Codebase grounding (read-only).** Confirmed the unconditional feature-append in
`collectArchiveCandidateIDs` (`:292`), the parent-walk in `featureScopeRoots`
(`:423`), the detach-before-collect ordering via `returnUnreleasedFeatureItems`
(`:260`) called from `ShipShipment` (`:135`), and the imprecise
`isTerminalReleaseStatus` predicate (`:955`) versus `core.TerminalStatuses`
(`blocking_cascade.go:14`).

## Options Evaluated

### Option A: Archive strictly by explicit shipment membership + terminal gate

Change `collectArchiveCandidateIDs` so a `featureScopeRoots` ancestor feature is
added to the archive set **only** when it is safe to close — that is, when the
feature is a genuine release: gate the feature-archival branch (using a
deliberately chosen, precise terminal predicate and Markdown-authoritative reads)
so the feature is archived only if it has no remaining non-released work. On a
partial shipment the covering feature is left in place; on a full-feature ship it
still closes.

* **Pros:** Uses the manifest — already the canonical membership authority — as
  the source of truth. Correct-by-construction for the common case. Localized to
  the archival branch. No new persisted schema. Aligns with the "derive from where
  a record belongs" precedent.
* **Cons:** Must precisely define "safe to close" so full-feature shipments still
  archive the feature. The pure descendant-terminal gate is proven insufficient
  for planned-but-unharvested units, so this option must combine membership with a
  terminal-children check and accept that a feature with only body-planned (no
  record) remaining units will **stay open** until those units are harvested and
  shipped — a conservative, safe default.
* **Effort:** medium.
* **Fit:** strong — meets all five success criteria for the recorded hard case.

### Option B: Durable "keep-open" lifecycle flag on the covering feature

Introduce an explicit lifecycle signal (e.g. `keep_open` under `custom_fields`,
or a modeled status) that the archival branch honors: never archive a feature
carrying the flag.

* **Pros:** Explicit operator/agent intent; unambiguous for the
  planned-but-unharvested case; survives even when no child records exist.
* **Cons:** Highest persistence cost and risk. The typed codec drops unmodeled
  top-level keys (`WriteArtifactFile`), and `custom_fields` is lossy through
  SQLite — the flag needs a defensive normalizer and a rehydration-count review,
  or full struct+writer+DB modeling. Introduces a new authority that every
  lifecycle op must maintain (split-brain risk). Requires someone to set/clear the
  flag correctly, re-introducing the human/agent-memory failure mode P-015 already
  suffers from.
* **Effort:** high.
* **Fit:** moderate — solves the hard case but adds durable surface area and a new
  maintenance burden.

### Option C: Code-enforced P-015 (ShipShipment refuses/degrades on partial-feature)

Detect a partial-feature shipment inside `ShipShipment` and refuse the cascade (or
degrade to single-artifact safe-close), turning the P-015 policy into a hard code
guard.

* **Pros:** Directly encodes the existing policy; strong defense-in-depth; hard to
  bypass.
* **Cons:** A guard, not a root-cause structural fix — the unconditional
  parent-walk archival still exists behind the guard. "Partial-feature" detection
  needs the same membership/terminal reasoning as Option A, so it does not avoid
  that work. On its own it can only refuse, leaving closure to the manual
  single-artifact path.
* **Effort:** medium.
* **Fit:** good as a **complement** (defense-in-depth), weak as the sole fix.

## Trade-off Comparison

| Criterion | A: Membership + terminal gate | B: Keep-open flag | C: Code-enforced refuse/degrade |
|---|---|---|---|
| Root-cause structural fix | Yes | Yes | No (guard over the defect) |
| Handles planned-but-unharvested hard case | Yes (stays open, conservative) | Yes (explicit) | Yes (refuses) |
| New persisted schema / codec risk | None | High (codec + SQLite lossy) | None |
| New maintenance authority / split-brain risk | None | Yes | None |
| Preserves full-feature auto-close | Yes (terminal-gated) | Yes | N/A (refuses partial only) |
| Effort | Medium | High | Medium |
| Prior-art alignment | Strong | Cautioned | Strong (as complement) |

## Decision

**Chosen direction: Option A (archive strictly by explicit shipment membership,
with a precise terminal-children gate on the covering-feature archival branch),
plus a `doctor` audit and the `114-S` regression test, and the `.ship.agent.md`
Step 6.1.b drift correction. Add Option C as a lightweight defense-in-depth guard
only if it falls out cheaply; defer Option B (keep-open flag).**

Rationale:

* The manifest is **already** the canonical membership authority
  (`NormalizeShipmentItems`), so Option A reuses an owned source of truth instead
  of inventing one (Option B) — directly following the ship-sequence-manifest
  spike and the "derive from where a record belongs" archival precedent.
* Option A is localized to the archival branch, adds no persisted schema, and
  carries the lowest data-integrity risk — decisive given the documented
  re-persist and typed-codec traps that Option B would have to navigate.
* The conservative default (a feature with remaining, non-released work stays
  open) is exactly the safe behavior the P-015 policy was protecting; encoding it
  in code removes the agent-memory dependence.
* A `doctor --check/--fix` audit plus the exact partial-feature regression test
  mirrors the proven archived-from delivery vehicle and prevents silent
  reintroduction.

This decision is recorded under the operator's `Stage next` authorization; the
staged plan carries the recommended direction into implementation, where Ship
finalizes the precise predicate and gate mechanics under test-first discipline.

## Rejected Alternatives

* **Option B (keep-open flag) as the primary fix** — set aside for now due to the
  high persistence/codec cost (unmodeled-key drop, lossy `custom_fields`
  round-trip) and the new maintenance authority it creates. It remains the
  fallback if Option A's terminal-children gate proves insufficient for a
  product-required "auto-close full feature without listing it in the manifest"
  behavior.
* **A pure descendant-only `CheckChildrenTerminal` gate** — explicitly rejected by
  the 114-S learning: planned-but-unharvested units have no descendant records and
  detached nonterminal siblings are removed before archival, so this gate would
  still over-archive.
* **Widening the DB projection to read archived/link state** — rejected on
  precedent (`archive-repersist-projection-drop`): the `items` table has no such
  columns; Markdown is the source of truth. Reload from Markdown instead.

## Resolved Questions (finalized during plan review)

* **"Safe to close" predicate — RESOLVED to membership alone.** The covering feature
  closes/archives **iff it is an explicit manifest member** (`explicitScope`). No
  descendant-terminal inference is used: it cannot distinguish a full feature from one
  with body-planned (record-invisible) remaining work — the exact gap 114-S proved. The
  full-feature path becomes an explicit feature-inclusive shipment; the three existing
  children-only-manifest tests (`QueuePathAbsentAfterShip`, `CleansReleasedFeatureScope`,
  `SkipsAlreadyArchivedLinkedDeliberation`) are updated to this contract.
* **Terminal predicate — NOT NEEDED.** Because the gate is pure set-membership, the fix
  introduces no new terminal-status predicate and does not touch `isTerminalReleaseStatus`
  / `blocking_cascade.go`. (There is no exported `core.TerminalStatuses`; the cascade set
  is the unexported `terminalCascadeStatuses` via `IsCascadeTerminalStatus` — unused here.)
* **`doctor` remediation scope — DEFERRED.** Ship the check-only audit this release;
  destructive `--fix` (unarchive) is a separate operator-approved, CLI-only unit scoped
  to affected records only.

## Risks and Mitigations

* **Regressing the full-feature close path.** Mitigation: characterization-first —
  add a feature-inclusive full-feature archival test. Note the three existing
  children-only-manifest tests (`TestShipShipment_CleansReleasedFeatureScope`,
  `TestShipShipment_QueuePathAbsentAfterShip`,
  `TestShipShipment_SkipsAlreadyArchivedLinkedDeliberation`) are deliberately updated to
  the membership contract — they encoded the 114-S bug and cannot stay green unchanged.
* **False-green regression tests.** Mitigation: assert both "covering feature
  remains in queue after partial ship" **and** "manifest items are archived",
  distinguishing skip-happened from reload-masked-abort per the re-persist
  learning.
* **Predicate change leaking into unrelated callsites.** Mitigation: introduce any
  new predicate as a dedicated function; leave the existing five
  `isTerminalReleaseStatus` release-progression callsites untouched.
* **Re-persist seam data loss.** Mitigation: read authoritative status from
  Markdown (`findArtifact`), skip already-archived items, follow the
  attach-commit-repersist rule.
* **Scope re-expansion undermining membership.** Mitigation: verify the archival
  branch operates on the intended candidate set and that `releaseScopeItemIDs`
  descendant expansion does not re-pull manifest-excluded siblings into archival.

## Post-Review Correction (2026-07-31)

The plan-review gate (multi-agent dispatch) surfaced blocking findings that were
verified against `internal/core/shipment_lifecycle.go` and `status_taxonomy.go` and
are corrected in the implementation plan. This deliberation's Decision direction
(Option A — membership + precise terminal gate, Markdown-authoritative, `doctor`
audit) is unchanged; the mechanism details below supersede the earlier phrasing:

* **Three mutation seams, gated/neutralized on membership.** The covering feature is
  rolled to `StatusDone` transitively by `completeReleaseScope` (`:171`) — which marks
  each shipped child done via `setArtifactStatus` (`:254`), unconditionally cascading
  (`cascadePersistedParentStatuses` `:519` → `ComputeParentStatus`
  `harness_status.go:82`) to roll a covering feature up when all its *recorded* children
  are done — then set `StatusDone` directly at `:191` (the `featureIDs` loop), both
  **before** `collectArchiveCandidateIDs` (`:292`). The fix gates the two **direct**
  seams — and the entire per-feature collector block (feature append +
  `linkedDeliberationIDs` + descendant sweep) — on **explicit manifest membership**
  (`explicitScope`, resolved at `:158` before any mutation), **and neutralizes the
  transitive rollup for non-member features** via a narrow pre-ship status/location
  snapshot+restore (or a cascade-suppression variant). A pre-ship snapshot **is**
  therefore required for the rollup seam (contradicting the earlier "no snapshot"
  phrasing) — but it is a targeted status/location capture, not a descendant-terminal
  predicate. `completeReleaseScope` **is** a covering-feature seam, but only
  transitively (via the child→parent status rollup), not through `releaseScope`
  membership: `releaseScopeItemIDs` expands only downward, so a derived ancestor feature
  never enters `releaseScope`, yet marking its children done still rolls it up.
* **No exported `core.TerminalStatuses`.** References to `core.TerminalStatuses`
  (Research Findings, Contentious Areas, Unresolved Questions) are corrected: the
  canonical cascade set is the unexported `terminalCascadeStatuses`, reached only via
  `IsCascadeTerminalStatus(string)` / `CascadeTerminalStatuses()` in
  `status_taxonomy.go`. `blocking_cascade.go` and the five `isTerminalReleaseStatus`
  callsites stay untouched.
* **`doctor` audit is check-only this release.** The destructive `--fix` remediation
  is deferred to a separate operator-approved, **CLI-only (not MCP)** unit per
  Constitution VII; detection reconstructs membership from the manifest +
  `returned_to_backlog` provenance, not `parent_id`.
* **Membership is the sole close-intent authority.** A feature closes/archives iff it is
  an explicit manifest member; a complete feature whose manifest omits the parent stays
  open by design (pinned by the updated tests). This fixes the planned-but-unharvested
  hard case — once the transitive rollup is suppressed for the non-member feature (not
  "by construction" from membership alone, since the rollup would otherwise close a
  body-planned feature) — and matches P-015 "archive only what is listed."
* **Ship guidance (Unit 4) retires the P-015 manual workaround** for partial
  shipments once the code is safe by construction, restoring user/agent command
  parity, rather than perpetuating a manual single-artifact safe-close path.
