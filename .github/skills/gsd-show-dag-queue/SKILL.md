---
name: gsd-show-dag-queue
description: "Render an eligible-only, DAG-tracked, capability-oriented pipeline readiness view across stash and backlog queue items with dependency matrices and narrative operator choices"
argument-hint: "[scope=all|stash|queue|track] [format=table|matrix|full]"
---

# GSD Show DAG Queue

Render an operator-legible, eligible-only, capability-oriented readiness view of
the workspace pipeline. This skill inspects active stash entries, queued and
active backlog items, explicit DAG `blocks` edges, deterministic readiness gates,
and open post-merge closure conditions to answer **what can actually be staged
and shipped next**, grouped around cohesive capabilities and useful operator
outcomes.

This skill is **read-only** (apart from refreshing the disposable backlog query
index): it MUST NOT claim shipments, mutate backlog item statuses, edit stash
priorities, create plans, or execute implementation tasks.

## When to Use

Invoke `/gsd-show-dag-queue` whenever the operator asks:

* What is eligible to stage or ship next in the pipeline
* Which stash entries, deliberations, features, tasks, or shipments belong
  together as a coherent capability or wave
* Which queued shipments are immediate DAG roots vs. downstream sequenced units
  blocked on predecessors or holds
* For an executive narrative and recommended next-target options before invoking
  Stage, Ship, or `/gsd-dark-mode`

## Inputs

* `scope`: (Optional) `all` (default), `stash`, `queue`, or `track`.
* `format`: (Optional) `full` (default — capability tables, DAG matrices, and
  narrative options), `table`, or `matrix`.

## Output

A structured, self-contained session-output report containing:

1. **Adapter & Gate Readiness Summary** (index sync status, hold scan coverage,
   and open closure conditions)
2. **Eligible-Only Capability & Feature Outcome Table** (mapping `-S`, `-DL`,
   `-F`, `-T`, and stash IDs with plain-language explanations)
3. **Shipment & Stash DAG Dependency Matrix** and **Capability Wave Matrix**
4. **Executive Narrative & Recommended Operator Choices** (conforming to the
   Operator Choice Presentation Rule)

## Workflow

### Step 0: Backlog Index Sync & Readiness Gates

Verify tool availability and gather complete, untruncated backlog and gate state
before filtering or classifying any item:

1. **Backlog Adapter Handling**:
   * **When `backlogit` is the active backlog tool**:
     1. Resolve the effective backlog root with the same precedence as the
        deterministic readiness gates (honoring `BACKLOGIT_WORKSPACE_DIR` when
        set, falling back to `.backlogit`).
     2. Refresh the disposable backlog index (`backlogit sync` or
        `backlogit_sync_index`).
     3. Read `<effective_backlog_root>/stash.jsonl` and
        `<effective_backlog_root>/queue/*.md` in full (never rely on truncated
        previews or filtered queries alone for hold detection).
     4. Enumerate `queued` and `active` shipments,
        features, chores, tasks, and deliberations alongside active stash
        entries.
   * **When a stashless adapter is active (including `backlog-md`)**:
     1. State explicitly in the output that no stash scan applies for the
        stashless adapter.
     2. Scan all work items across the adapter's configured item sources
        (`.backlogit/`) in full, preserving the same DAG dependency
        tracking and full scope-wide hold propagation as Orchestrator Step 0
        (`2a-hold`).
2. **Deterministic DAG & Pre-Claim Gates**:
   * Run `gsd gate dag-readiness --json` to capture ready-set, critical-path,
     and downstream-dependent relationships across shipments and items when
     shipment workflows are enabled (`features.shipments: true`;
     for stashless adapters with `features.shipments: false`, evaluate
     task-level dependencies directly from the configured item sources).
   * When `features.shipments: true` is enabled (`true`), report
     `active` shipments as in-flight without probing `--phase pre_claim`
     (which always returns `PRECLAIM_ACTIVE_SHIPMENT_PRESENT` whenever any
     shipment is active). For each candidate `queued` shipment, run
     the read-only probe
     `gsd gate pipeline-topology --mode agent --shipment <id> --phase pre_claim --json`
     to record authoritative `predecessor_provenance` (`explicit`,
     `declared_root`, `genesis`, or `unsequenced`), `hold_eligibility`, and
     `shipment_readiness` verdicts, distinguishing global
     `PRECLAIM_ACTIVE_SHIPMENT_PRESENT` from per-candidate readiness or holds.
3. **Open Closure-Condition & Artifact Context Scan**:
   * Scan `docs/closure/*.md` for any `READY_WITH_CONDITIONS` records
     containing open (`satisfied: false`) post-merge conditions that block
     downstream shipments.
   * Cross-reference `docs/decisions/` (deliberations `-DL`),
     `docs/plans/` (implementation plans), and `docs/compound/`
     (compound learnings) to explain each work unit in plain language.

### Step 1: Eligible-Only Filtering

Exclude work that cannot currently be acted on so the operator sees only
actionable, eligible pipeline candidates (with a brief exclusion count summary):

1. **Exclude**:
   * Shipped, completed, or archived items (`status: shipped`, `done`, or
     `archived`)
   * Operator-held or hold-gated items (`HOLD:` clauses, hold labels, hold-gate
     records, or `must not be claimed` / `do not claim` directives,
     propagated across shipment, feature, and task boundaries)
   * Externally blocked items (unfinished external dependencies, unmet
     predecessor closure conditions, or non-executable external blockers)
   * Obsolete, superseded, or duplicate stash/queue items
2. **Classify Remaining Eligible Candidates**:
   * **Immediate DAG Roots**: Eligible items or shipments whose `pre_claim`
     verdict is `ELIGIBLE` (zero unfinished predecessors, `explicit` satisfied
     or `declared_root` / `genesis` valid, and no active hold).
   * **Downstream Sequenced Units**: Eligible items or shipments whose only
     blockers are explicit in-scope `blocks` predecessors (`PENDING` until the
     upstream shipment in the same track or wave lands).

### Step 2: Capability-Oriented Feature/Outcome Table

Organize all eligible items around **cohesive capabilities or feature sets that
implement a useful operator outcome**, rather than raw chronological ID dumps.

For each capability track, render a structured table linking every related
artifact type:

| Capability Track / Useful Outcome | Shipments (`-S`) | Features / Chores (`-F` / `-C`) | Tasks (`-T`) | Deliberations (`-DL`) | Stash Entries | Plain-Language Explanation & Readiness |
|---|---|---|---|---|---|---|

Guidelines for this table:

* Group related stash entries, deliberations, features, tasks, and shipments
  that together deliver one coherent user-visible or operator-visible outcome.
* Explain *what capability changes for the user or operator* in plain language,
  avoiding opaque internal jargon alone.
* Note when a capability track spans multiple waves or requires Stage intake
  (stash -> deliberation -> plan -> shipment) before Ship execution.

### Step 3: DAG Dependency & Wave Matrices

Render two compact decision matrices so the operator can verify exact ordering
and downstream unlock impact at a glance:

1. **Shipment & Stash DAG Dependency Matrix**:
   * Columns: `Unit / Shipment ID`, `Predecessor(s) (blocks edges)`,
     `Predecessor Provenance`, `pre_claim Verdict` (`ELIGIBLE` | `PENDING` |
     `EXTERNAL_BLOCKER`), `Downstream Unlocked Units`, and `Stage / Ship Posture`.
2. **Capability Wave Matrix**:
   * Columns: `Wave`, `Capability Track`, `Target Units`, `Why This Wave First`,
     and `Verification / Handoff Readiness`.
   * Clearly distinguish immediate Wave 1 roots (`ELIGIBLE` now) from Wave 2+
     sequenced successors (`PENDING` on Wave 1) and Stage-only stash tracks
     needing deliberation/planning.

### Step 4: Executive Narrative & Recommended Operator Choices

Close with a concise narrative that frames the trade-offs across capability
tracks and presents self-contained recommended choices conforming to the
**Operator Choice Presentation Rule**:

1. Explain the recommended ordering across tracks (for example, unblocking
   high-leverage tooling or reliability tracks before downstream consumers).
2. Present numbered, self-contained options (`Option 1`, `Option 2`, ...) where
   each option states:
   * Item / Shipment / Stash IDs, priority, and kind/type
   * One-line plain-language summary of the capability outcome
   * Recommended execution sequence (e.g., `[001-S -> 002-S]` or Stage-first
     stash harvest)
   * The exact choice requested from the operator (such as running `/gsd-dark-mode`
     on a chosen sequence or invoking Stage on selected stash IDs)
3. Explicitly state that operator selection or confirmation is awaited before
   any Stage or Ship mutation runs.

Generated by gsd | Template: gsd-show-dag-queue/SKILL.md.tmpl
