---
type: dark-mode-halt
timestamp: 2026-08-07T22:24:09-07:00
agent: Stage
skill: direct
breaker_type: policy-halt
policy: P-017 (dark factory mode) — scope expansion
operation: dark-factory staging cycle (formal gate + workspace rename + 054 lifecycle)
---

# DARK_MODE_HALTED — scope expansion

## Halt record

| Field | Value |
|---|---|
| `event` | `DARK_MODE_HALTED` |
| `halt_reason` | **Mid-cycle scope expansion.** The operator added active stash entry `FDEDE39A` to prioritized work while this dark Stage cycle was running. |
| `violated_policy` | P-017 — dark mode must stay bounded to the declared `DARK_MODE_SCOPE` |
| `affected_scope_item` | `FDEDE39A` (not in the recorded `DARK_MODE_SCOPE`); `B5D7E401` also appeared mid-cycle and is likewise out of scope |
| `halt_boundary` | Safe boundary — the Stage cycle had already completed and committed; **no atomic mutation was in flight** at halt time |
| `atomic_mutations_completed_before_halt` | All. Nothing was left partially applied. |
| `required_operator_action` | Reactivate an amended `DARK_MODE_SCOPE` in a new Stage turn via the orchestrator |
| `visibility_mode` | degraded — `agent-intercom` unavailable, so this record replaces the broadcast |

## Explicitly NOT done (scope containment)

`FDEDE39A` and `B5D7E401` were **not** consumed, harvested, deliberated,
planned, archived, linked, or silently folded into any existing shipment. They
remain **active** in the primary worktree's stash exactly as the operator left
them. Only read-only inspection was performed, in the primary worktree, with no
write of any kind.

No new work item, dependency edge, shipment, or shipment membership was created
after the halt directive was received.

## New out-of-scope items (read-only capture)

### `FDEDE39A` — kind `feature`, priority `high`, created 2026-08-08T02:41:12Z

> Add exact-target administrative checkpoint disposition:
> `checkpoint abandon FILE --reason REASON` for valid checkpoints intentionally
> not resumed, and `checkpoint quarantine FILE --reason REASON` for malformed
> checkpoints that cannot pass normal validation. Both CLI and MCP operations
> must be atomic, auditable, filename-scoped, preserve original evidence, record
> operator and reason metadata, and avoid broad retention cleanup.

Stage's read-only assessment, offered as input to the amended scope only — **no
decision was made and no artifact was created**:

* Feature-shaped. Touches `internal/cli/checkpoint.go`, the MCP checkpoint tool
  surface, and `.autoharness/backlog-registry.yaml`.
* Naturally pairs with `B5D7E401` (same checkpoint subsystem), which would make a
  coherent covering feature if the operator chooses to group them.
* Interacts with the already-queued `119-S` (governed-operation CLI/MCP parity):
  "both CLI and MCP operations must be atomic, auditable" is precisely the
  governed-operation contract `119-S` establishes. Sequencing `FDEDE39A` **after**
  `119-S` would let it consume that contract instead of duplicating it.
* Also brushes `120-S` (F5 idempotent multi-mutation envelope): "atomic,
  auditable, preserve original evidence" is the same recovery contract.

### `B5D7E401` — kind `bug`, priority `medium`, created 2026-08-08T03:34:59Z

> Fix checkpoint JSON readability: `encoding/json` HTML-escapes `>`, `<`, `&` as
> `\u003e` / `\u003c` / `\u0026`. Apply `Encoder.SetEscapeHTML(false)` to both V1
> checkpoint persistence (`json.Marshal` path) and checkpoint CLI JSON output
> (`json.Encoder` path) via a shared readable-JSON encoding helper. Add regression
> tests verifying `>`, `<`, and `&` are emitted literally in checkpoint JSON output.

Read-only assessment only: bug-shaped, small, self-contained, and under the
operator's type ordering (bug before feature) it would outrank `FDEDE39A` if both
enter an amended scope.

### `9370A18C` — still present in the primary worktree

`9370A18C` was in scope, was harvested into `135-F` / `121-S`, and was archived in
the **linked** worktree. It still shows as active in the **primary** worktree
because that archival lives on the unmerged branch. This is expected, not a defect.

## Completed Stage work at halt (all committed, nothing in flight)

**Worktree:** `.copilot/session-state/ecebe820-92d7-4253-852a-0c3c23f8aea9/files/dark-factory-worktree`
**Branch:** `admin/dark-stage-formal-gate` — **clean**
**Tip:** `9b3495dca9248cfb4e871eaee05f74926ec00cab`

| Commit | Summary |
|---|---|
| `9bff354e` | three deliberations + five reviewed implementation plans |
| `e6ba6ffe` | 38 harvested items, 38 dependency edges, five queued shipments |
| `f572e19c` | stash `9370A18C` archived; `054.001-R` accepted and archived |
| `34b09b7d` | dark-factory Stage session memory |
| `9b3495dc` | concurrent-operator-activity and stash merge-hazard record |

### Queued shipments (none active or claimed)

| Order | ID | Title | Priority |
|---|---|---|---|
| 1 | `117-S` | Formal gate F1 — evidence authenticity and manifest binding | high |
| 2 | `118-S` | Formal gate F4 — durable dependency type persistence | high |
| 3 | `119-S` | Formal gate F6 — governed-operation CLI and MCP parity | medium |
| 4 | `120-S` | Formal gate F5 — idempotent multi-mutation envelope (final F-series) | high |
| 5 | `121-S` | Default workspace directory rename to `.backlog` | medium |

Ordering is enforced by 38 explicit `blocks` edges, not prose.

### Gate outcomes

* Plan review: `dispatch_mode: multi-agent-dispatch`, cycle 1 **FAIL**
  (0 P0 / 18 P1 / 12 P2), cycle 2 **PASS** on all five plans (0 P0 / 0 P1).
* P-006 hardening: five of five plans declare `Requires plan hardening: yes` and
  carry a `## Plan Hardening` section.
* `054.001-R`: minimal lifecycle disposition only — `review` → `accepted` →
  archived. No feature work created.

## State of the halt

Stage's original in-scope work reached a complete, coherent, committed state
**before** the halt directive. The halt therefore costs nothing: no partial
hierarchy, no half-assembled shipment, no orphaned artifact, no dangling
dependency edge, and no uncommitted change in the linked worktree.

## Resumption

The orchestrator will reactivate an amended `DARK_MODE_SCOPE` in a **new** Stage
turn. On resumption:

1. Re-read this checkpoint and
   `docs/memory/2026-08-07/dark-factory-stage-formal-gate-memory.md`.
2. Do not re-harvest anything already in `117-S`…`121-S`.
3. Treat `FDEDE39A` and `B5D7E401` as fresh intake requiring their own triage,
   grouping, deliberation, planning, and review — none of those gates were run for
   them in this cycle.
4. Re-run the Step 0.0 tool gate and Step 0.1 index sync; intercom and engram were
   unavailable in this cycle and their state must be re-probed.
5. Reuse this same linked worktree to preserve P-016; do not create a second one.
