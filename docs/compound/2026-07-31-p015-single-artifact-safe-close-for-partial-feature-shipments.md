---
chunk_strategy: h1-h2-h3
description: 'UPDATED 2026-08-01 (shipment 115-S / feature 133-F, PR #327, merge 47dfcc93): the root-cause product fix this entry called for has SHIPPED — core.ShipShipment now gates covering-feature archival and its terminal-sibling sweep on explicit shipment-manifest membership, and snapshots/restores any non-member feature incidentally rolled up by the generic parent-status cascade. P-015 policy text now states the Ship agent MAY call the native cascade for closure, including partial-feature shipments; the manual procedure below is retained as a defense-in-depth fallback, not the mandatory-only path, for repository states containing the fix. Caveat: this repository dogfoods a separately pinned release CLI (C:\Tools\backlogit.exe) for real backlog operations — a merge to main does not retroactively change that pinned binary; verify its embedded commit is at/after the fix before trusting the cascade on a future shipment (see docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md). Original 114-S incident record preserved verbatim below. One durable rule graduated from 114-S — for a partial-feature shipment (a shipment whose manifest covers only some of a covering feature''s work, leaving other units intended for later), pre-fix closure MUST use P-015 single-artifact safe-close and MUST NOT call the cascade backlogit_ship_shipment / `backlogit shipment ship`. The cascade computes a feature scope by walking parent_id up from every manifest item (featureScopeRoots) and archives every ancestor feature unconditionally (collectArchiveCandidateIDs) with no terminal-children gate, so it always archives the covering feature (plus any terminal unshipped siblings; nonterminal siblings are detached by returnUnreleasedFeatureItems and excluded) — corrupting the backlog. Safe-close: archive only the manifest item IDs one artifact at a time (pre-archived items excluded), then the shipment record as its own single artifact, verifying after each that the protected set (covering feature + unshipped siblings) stays in queue. A detected cascade requires git-revert + HALT, not restore-and-continue. A descendant-only CheckChildrenTerminal gate does NOT fix the root cause because planned-but-unharvested units have no records and returnUnreleasedFeatureItems detaches harvested nonterminal descendants first; the durable product fix needs an explicit shipment-membership or durable keep-open lifecycle signal.'
doc_type: learning
docline:
    date: 2026-07-31T00:00:00Z
    severity: high
    tags:
        - core
        - shipment
        - lifecycle
        - archive
        - cascade
        - ship_shipment
        - p-015
        - safe-close
        - closure
        - updated-2026-08-01
        - 133-f
schema_version: "1.0"
source: docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md
title: 'Partial-feature shipments close via P-015 single-artifact safe-close — never the cascade ship_shipment (114-S)'
---

# Partial-Feature Shipments: Use P-015 Safe-Close, Not Cascade Ship

One durable rule graduated from shipment 114-S (feature 106-F,
"Formal-gate foundations" F2+F3, PR #324, merge
`f8870f864d596a1f3593405e54396d8129aa8871`). This entry records both a policy
that is easy to miss and a concrete violation-and-recovery walkthrough.

## Update (2026-08-01, shipment 115-S / feature 133-F): root cause now fixed in code

The "Root-Cause Follow-Up" section below called for exactly this: "explicit
shipment membership (archive only what the manifest lists)". That fix has
since shipped: `core.ShipShipment` (`internal/core/shipment_lifecycle.go`,
PR #327, merge `47dfcc93698a6b0b2c5420c701c365a538895580`) now gates both the
covering-feature archival branch **and** its terminal-sibling descendant sweep
in `collectArchiveCandidateIDs` on explicit shipment-manifest membership, and
additionally snapshots/restores any non-member covering feature that the
*separate*, generic `completeReleaseScope` → `cascadePersistedParentStatuses`
parent-status rollup incidentally rolls up (`snapshotNonMemberFeatureStatuses`
/ `restoreRolledUpNonMemberFeatures`, restoring deepest-first to stay correct
under Go's randomized map iteration order). `.github/policies/workflow-policies.md`
P-015 has been updated accordingly ("Code Enforcement (as of release unit
115-S)") to state that **the Ship agent MAY call the native cascade
`backlogit_ship_shipment` directly for closure, including partial-feature
shipments** — the manual safe-close procedure below is retained by policy as a
**defense-in-depth fallback**, not the mandatory-only path, for repository
states containing this fix. It remains the mandatory path against a
`core.ShipShipment` predating the fix, or if the `doctor
--check-over-archived-features` audit (added alongside the fix) ever reports a
regression.

**Caveat — code fix ≠ operative everywhere yet**: this repository dogfoods a
**separately pinned, globally-installed release binary**
(`C:\Tools\backlogit.exe`) to operate its own `.backlogit/` backlog, rather
than rebuilding from `HEAD` on every operation. A merge to `main` does not
retroactively change that pinned binary's behavior. See
`docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`
for the concrete instance discovered while closing 115-S itself (installed
CLI `v1.7.0`, commit `7daf8c3`, predates the fix by 9 days) — before trusting
the cascade path on any *future* partial-feature shipment, verify the
installed CLI's embedded commit is at or after `47dfcc93`, not merely that
`main` contains the fix.

The verification, code diff, and code-comment rationale for this fix are
recorded in
`docs/closure/2026-08-01-133-shipshipment-cascade-fix-runtime-verification.md`
and this shipment's post-merge closure artifact. The remainder of this entry
(below) is preserved verbatim as the historically accurate incident record for
114-S and the pre-fix era.

## The Rule (P-015)

When a shipment is a **partial-feature shipment** — its manifest covers only a
subset of a covering feature's work and the feature intentionally stays open for
later units — closure MUST follow **P-015 single-artifact safe-close**
(`.github/policies/workflow-policies.md`, "P-015: Single-Artifact Shipment
Closure"):

* Archive **only** the shipment manifest's explicit item IDs, one artifact at a
  time (`backlogit archive <id>`), then the **shipment record itself** as its own
  single artifact. Manifest items already archived (`pre-archived`) are excluded
  from the item loop.
* Compute the **protected set** = the covering feature **plus** every unshipped
  sibling task not in the manifest. A **baseline integrity gate** confirms every
  protected-set member is present in `.backlogit/queue/` before any archival.
* **Verify-after-each invariant**: after archiving each single artifact, confirm
  every protected-set member still exists in `.backlogit/queue/`. The pre-archived
  exemption applies to **manifest items only** — the protected set has no
  pre-archived exemption.
* **MUST NOT call the cascade `backlogit_ship_shipment` / `backlogit shipment ship`.**

Note the drift trap: Ship agent Step 6.1.b (`.github/agents/.ship.agent.md`)
still instructs `Call backlogit_ship_shipment with the merge commit SHA`. That is
the **full-feature** default. **P-015 (added later, changelog 1.10.0) overrides it
for partial-feature shipments.** Consult P-015 before closing any shipment whose
covering feature has remaining intended work.

## Why the Cascade Is Unsafe

`core.ShipShipment` does not limit archival to manifest items:

* `featureScopeRoots` walks `currentID = item.ParentID` up from every released
  item, collecting every ancestor `feature`.
* `collectArchiveCandidateIDs` then appends each such feature **unconditionally**
  (`if feature.Status != models.StatusArchived { candidates = append(...) }`) with
  **no terminal-children gate** — unlike the descendant-item branch, which is gated
  by `isTerminalReleaseStatus`.

So shipping the first sub-shipment of a multi-cycle feature **always** archives the
covering feature. Unshipped siblings are archived only when **terminal** —
`returnUnreleasedFeatureItems` detaches **nonterminal** siblings (returns them to
`queued`, clears `parent_id`) before `collectArchiveCandidateIDs`, so they are
excluded from archival. Observed on 114-S: `backlogit shipment ship 114-S` returned
`archived_ids: [106.001-T, 106.002-T, 106-F, 114-S]` — the covering feature `106-F`
was archived though the manifest `items` were only `[106.001-T, 106.002-T]`; there
were no sibling records to consider (F1/F4/F5/F6 are planned-but-unharvested).

## Violation-and-Recovery Walkthrough (what happened on 114-S)

1. The cascade `backlogit shipment ship 114-S` was mistakenly called and its
   side-effects committed on the closure branch (PR #325, not yet merged — main
   stayed clean).
2. P-015 violation action is **git-revert-on-cascade, then HALT** — restoration is
   recovery, not authorization to continue. Because the cascade was committed,
   `git revert <commit>` was the mandated recovery (vs `git restore` for an
   uncommitted cascade). The revert reinstated 114-S in queue, un-stamped the
   manifest items, and returned the tree to the clean baseline.
3. **HALT (mandated).** After recovery, P-015's violation action is to HALT and
   surface the cascade to the operator with a P-005 event — recovery is NOT
   authorization to continue. A same-session self-authorized reclosure labeled
   "compliant" would teach agents to bypass this halt and must be avoided.
4. The reclosure (surfaced for operator authorization, not auto-authorized):
   baseline integrity gate (106-F present) → manifest items pre-archived
   (excluded) → `backlogit update 114-S --commit <merge>` → `backlogit move
   114-S --status done` → `backlogit archive 114-S` (three single-artifact ops;
   hook events `update_artifact` + `archive_item`, never `ship_shipment`) →
   verify-after-each confirmed 106-F still in queue. In this session the
   reclosure was carried in the closure PR and presented to the operator for
   explicit authorization.

## Root-Cause Follow-Up (why a descendant gate is not enough)

A naive fix — gate the `featureIDs` archival branch on `CheckChildrenTerminal` —
does **not** cover the 114-S case:

* F1/F4/F5/F6 are **planned only in the feature body**, so there are no nonterminal
  descendant records to block archival.
* `returnUnreleasedFeatureItems` clears `parent_id` on harvested nonterminal
  descendants **before** `collectArchiveCandidateIDs` runs, so even harvested
  siblings would not be seen as blocking descendants.

The durable product fix therefore needs a **lifecycle signal that survives both
cases** — for example explicit shipment membership (archive only what the manifest
lists) or a durable "keep-open" state on the covering feature — not merely a
descendant-terminal gate. Until such a fix exists, P-015 single-artifact safe-close
is the authoritative closure procedure for partial-feature shipments.
