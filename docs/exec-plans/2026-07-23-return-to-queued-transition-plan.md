---
chunk_strategy: h1-h2-h3
description: 'Add blocked->queued and active->queued to the validated status-transition map (both definition sites) so operators can manually requeue a blocked or active item into the ready pool, matching the gate-broker requeue and the backlogit doctor documented resume. TDD, additive, low-risk. Resolves stash BD8DBB85 via Option A.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-23-return-to-queued-transition-plan.md
title: 'Return-to-queued transition: validated-map alignment (Option A)'
stash_id: BD8DBB85
tags:
  - task-lifecycle
  - state-machine
  - transition-validation
---

Source decision: `docs/decisions/2026-07-23-return-to-queued-transition-deliberation.md`
(Option A, decided; recommendation surfaced for operator confirm at the staging PR).
Related prior decision: `docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md`.

## Problem Frame

The validated status-transition map has no path back to `queued` from any later
state. `blocked` allows only `-> active`; `active` has no `-> queued`. The
user-facing validator `hooks.ValidateStatusTransition`
(`internal/hooks/builtin_pre.go:36`, wired at `internal/core/workspace.go:118`,
gated by `Lifecycle.ValidateTransition`) therefore rejects `backlogit move <id>
--status queued`. Yet:

1. The gate broker already moves items `active -> queued` on repeated failure by
   deliberately bypassing this validator
   (`internal/core/gate_transition.go:264-283`), and prior decision D23DFA0B
   blessed `queued` as the requeue landing state.
2. The `backlogit doctor` long-help (`internal/cli/doctor.go:64-69`, source of
   the generated `docs/cli-reference/backlogit_doctor.md`) tells operators to
   resume with `--status queued`.

The fix (Option A) adds `blocked -> queued` and `active -> queued` to the
validated map so the user-facing path matches both the internal gate behavior and
the documentation. The change is additive — no previously-valid transition becomes
invalid — and no test or invariant depends on `queued` being unreachable.

## Requirements Trace

| Requirement (from decision) | Implementation action |
|---|---|
| Allow manual `blocked -> queued` | Add `queued` to the `blocked` entry in both transition-map definitions |
| Allow manual `active -> queued` (gate parity) | Add `queued` to the `active` entry in both transition-map definitions |
| Keep both map copies in sync | Edit `internal/config/defaults.go:508-514` AND `internal/hooks/builtin_pre.go:16-24`; add a **mandatory** deep-equality (`reflect.DeepEqual`) test asserting `hooks.DefaultTransitions()` equals `DefaultHooksConfig().Lifecycle.Transitions`. No import cycle exists (`hooks` imports only stdlib + `internal/errors`; `config` does not import `hooks`), so an external `_test` package can compare both directly. |
| Verify behavior on the PRODUCTION-wired map (TDD, red->green) | The production validator is wired at `internal/core/workspace.go:114-118` with `hooksCfg.Lifecycle.Transitions` (the **config** map); `DefaultTransitions()` is only the empty-map fallback. So besides the `ValidateStatusTransition(nil)` cases, add a positive test that feeds `DefaultHooksConfig().Lifecycle.Transitions` into `ValidateStatusTransition` and asserts `blocked->queued`/`active->queued` pass, giving the `defaults.go` edit its own observed red->green. |
| Confirm existing workspaces are reached | Verify whether `backlogit init` materializes `hooks.yaml` with a serialized `lifecycle.transitions` block. If it does, a pre-existing workspace's persisted map would shadow the default-map fix — record a migration note. If init does NOT serialize transitions (relies on the in-code default), the fix reaches all workspaces; record that verification. |
| No regression to forbidden transitions | Keep `queued -> done`, `blocked -> done`, etc. rejected; assert in the invalid-transition test |
| Doc stays accurate | Confirm `internal/cli/doctor.go` long-help and generated `docs/cli-reference/backlogit_doctor.md` need NO change (code now honors them); Ship regenerates CLI-reference docs during build if the drift gate requires it |
| Fix stale test comment | Update the comment at `internal/core/shipment_state_integrity_test.go:139` ("blocked->active is the only hook-allowed exit") |
| Additive / backward-compatible | New transitions only widen the allowed set; existing valid moves and existing rejections are preserved |

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Pure map edits + wrapped errors already in place; `go vet`, `golangci-lint`, `gofmt` gates apply. No `unsafe`. |
| II. Test-First (NON-NEGOTIABLE) | TDD: extend unit tests first (red), then edit the two maps (green). See Test Plan. |
| III. Workspace Isolation | No filesystem/path changes. |
| VI. Single Responsibility | No new dependencies; edits confined to existing lifecycle code. |
| IX. Git-Friendly Persistence | No serialization-format change; transition map is in-code config. |
| X. Context Efficiency | N/A (behavioral change only). |
| IV. CLI Containment / VII. Destructive Approval / VIII. Safety Modes | N/A — no file I/O outside cwd, no destructive operation, no elevated-risk action. |
| XI. Merge Commit Preservation | N/A at plan level — merge strategy enforced by the Ship agent's merge gate. |
| Scope / 2-Hour Rule | 2 source files + paired tests, < 5 functions, < 4 new test scenarios. |

Justified deviations: none. This is an additive lifecycle change with no
principle conflict.

Constitution Check: pass

## Implementation Units

### Unit 1: Add return-to-queued transitions to both validated map definitions (code + unit tests)

* **Domain:** Go / core lifecycle (single skill domain).
* **Execution posture:** test-first (TDD).
* **What changes:**
  1. **Test-first (red):** In `internal/hooks/builtin_pre_test.go`:
     - Add `{"blocked", "queued"}` and `{"active", "queued"}` to the
       `validTransitions` table in `TestValidateStatusTransition_AllDefaultTransitions`
       (`:100-116`). Confirm these two cases FAIL against the current map.
     - Add a focused negative assertion (extend `TestValidateStatusTransition_InvalidTransition`
       or add a sibling) confirming a still-forbidden transition such as
       `blocked -> done` or `queued -> done` continues to error, so the widening
       does not over-open the map.
  2. **Green — `internal/hooks/builtin_pre.go:16-24` (`DefaultTransitions`):**
     - `"active":  {"done", "blocked", "review", "shipped", "abandoned", "queued"}`
     - `"blocked": {"active", "queued"}`
  3. **Green — `internal/config/defaults.go:508-514`
     (`DefaultHooksConfig().Lifecycle.Transitions`):** apply the identical two
     edits (add `"queued"` to the `active` and `blocked` entries).
  4. **Sync guard (red->green) — MANDATORY (not optional):** add a deep-equality
     test (external `_test` package to avoid any cycle — verified none exists)
     asserting `reflect.DeepEqual(hooks.DefaultTransitions(),
     config.DefaultHooksConfig().Lifecycle.Transitions)`, so any future one-sided
     edit (including a divergent non-`queued` entry) fails CI. Do NOT use a weaker
     per-package "contains" assertion — it would let the maps drift on other
     entries undetected. (Compound: `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`.)
  5. **Production-map positive coverage (red->green):** add a test that feeds
     `config.DefaultHooksConfig().Lifecycle.Transitions` into
     `hooks.ValidateStatusTransition(...)` and asserts `blocked->queued` and
     `active->queued` pass — the `ValidateStatusTransition(nil)` cases in
     `AllDefaultTransitions` only exercise the hooks fallback map, so without this
     the `defaults.go` edit (the actually-wired map per `workspace.go:114-118`)
     has no direct behavioral proof.
* **Files:** `internal/hooks/builtin_pre.go`, `internal/config/defaults.go`,
  `internal/hooks/builtin_pre_test.go`, and one sync-guard/production-map test file
  (an external `_test` package, e.g. `internal/config/transitions_sync_test.go`).
* **Acceptance:** `go test ./internal/hooks/... ./internal/config/...` passes;
  the two new valid transitions pass and the negative case still errors.

### Unit 2: Align integration-test comment and confirm doc accuracy (tests + doc verification)

* **Domain:** Go tests + docs verification (kept separate from Unit 1's core
  code change per Width Isolation).
* **Execution posture:** characterization-first.
* **What changes:**
  1. Update the stale comment at
     `internal/core/shipment_state_integrity_test.go:139` from
     "blocked->active is the only hook-allowed exit" to note that
     `blocked -> queued` is now also allowed (the test's own move stays
     `blocked -> active` and remains valid — no assertion change needed).
  2. **Sweep for other stale invariant comments.** Grep `internal/core`,
     `internal/mcp`, `internal/cli` for comments/docstrings asserting
     `blocked`/`active` cannot reach `queued` (e.g. "only hook-allowed exit",
     "no path back to queued", "queued is unreachable") and correct any found, so
     no stale invariant prose survives the widening. `go test ./...` catches
     failing assertions but not stale comments.
  3. **Align the gate-broker bypass comment.** Update the rationale comment at
     `internal/core/gate_transition.go:266-267`. After Option A, `active->queued`
     is a validated transition, so "bypasses the user-facing transition validator
     because the gate is the completion authority" should be reworded: the direct
     write exists for authoritative gate-evidence recording, `GateBlockedError`
     semantics, and hook-reentry avoidance — and the validated map now also permits
     the transition, so the two paths are consistent rather than contradictory.
     This prevents a future maintainer from concluding the bypass is dead code.
  4. Verify `internal/cli/doctor.go:64-69` long-help and the generated
     `docs/cli-reference/backlogit_doctor.md` require NO textual change (Option A
     makes the existing `--status queued` resume text correct). Record this
     verification in the task; if the repo's "CLI Reference Drift" gate flags the
     generated doc, Ship regenerates it via the standard doc-generation command
     (no manual edit of generated content).
  5. Full-suite sanity: `go test ./...` green (especially `internal/core/...`,
     `internal/mcp/...`, `internal/cli/...` which reference transition validity in
     comments/tests) to catch any test that implicitly assumed `blocked`/`active`
     could not reach `queued`.
* **Files:** `internal/core/shipment_state_integrity_test.go` (comment only),
  `internal/core/gate_transition.go` (comment only), plus any stale-comment
  corrections surfaced by the sweep; no production doc edit expected.
* **Acceptance:** `go test ./...` passes; no CLI-reference drift or docline gate
  failure remains; doctor doc verified accurate.

## Test Plan (TDD, red -> green)

| Test | Location | Red (before) | Green (after) |
|---|---|---|---|
| `blocked -> queued` valid (hooks fallback map) | `builtin_pre_test.go` `AllDefaultTransitions` table | errors (not in map) | passes |
| `active -> queued` valid (hooks fallback map) | `builtin_pre_test.go` `AllDefaultTransitions` table | errors (not in map) | passes |
| `blocked -> queued` / `active -> queued` valid on the **production-wired config map** | new test feeding `DefaultHooksConfig().Lifecycle.Transitions` into `ValidateStatusTransition` | errors | passes |
| Still-forbidden negative (e.g. `blocked -> done`) | `builtin_pre_test.go` invalid-transition test | passes (already forbidden) | still passes |
| Map sync guard: `reflect.DeepEqual(hooks map, config map)` | new external `_test` package | fails/absent | passes |
| Existing lifecycle suites unaffected | `go test ./...` | green | green |

Execution order: (1) add the two failing valid cases and confirm red, (2) edit
both maps, (3) confirm the new cases + sync guard green, (4) run `go test ./...`,
(5) run `go vet ./...`, `golangci-lint run`, `gofmt -l .`.

## Risks & Mitigations

- **Two map copies drift.** Mitigation: edit both in the same change and add the
  MANDATORY deep-equality sync-guard test (Unit 1 step 4).
- **Config-map edit unverified.** The `defaults.go` map is the production-wired
  one (`workspace.go:114-118`); the `nil`-fed positive tests only exercise the
  hooks fallback. Mitigation: add the production-map positive test (Unit 1 step 5).
- **Persisted `hooks.yaml` shadows the fix.** An existing workspace whose
  `hooks.yaml` serializes an explicit `lifecycle.transitions` block would not pick
  up the in-code default change. Mitigation: verify `backlogit init` behavior
  (Unit 2 / Requirements Trace); record a migration note if init serializes the
  map.
- **A hidden test assumed `queued` unreachable.** Mitigation: Unit 2 runs the
  full suite and sweeps for stale invariant comments; verified during staging that
  no test asserts `blocked->queued` or `active->queued` is forbidden (only stale
  comment at `shipment_state_integrity_test.go:139`).
- **CLI-reference drift gate.** Mitigation: no doc text change is expected under
  Option A; if the gate still flags generated content, Ship regenerates via the
  standard command rather than hand-editing.
- **Parity claim over-scoped.** The CLI and MCP move handlers are NOT byte-identical
  (the MCP handler adds a `CheckChildrenTerminal` guard for terminal moves); the
  parity that matters here is that both share the `UpdateArtifactWithGate` ->
  `ValidateStatusTransition` path. `queued` is non-terminal, so the `CheckChildrenTerminal`
  asymmetry does not affect this change. Mitigation: scope the parity statement to
  the shared transition validator only.

## Requires plan hardening

no

Rationale: additive, low-blast-radius lifecycle change in two well-understood
definition sites with a bounded TDD test surface, no data migration, no external
integration, no destructive operation, and no security/auth impact.

## Out of Scope

- Changing the gate broker's validator-bypass design (it remains correct).
- Adding any other transitions (e.g. `review -> queued`, `done -> queued`) — not
  requested and not blessed by any prior decision.
- Config-file / schema surface changes beyond the in-code default map.
- Option B (docs-only) — rejected in the decision; retained only as the
  operator's course-correction path at PR review.


## Plan Review

<!-- plan-review-attempt: 1 -->

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved

**Gate rationale.** Six reviewer personas were dispatched as independent
sub-agents (`multi-agent-dispatch`): Constitution Reviewer, Go Reviewer, Scope
Boundary Auditor, Architecture Strategist, Learnings Researcher, and Agent-Native
Parity Reviewer. Every selected persona completed and returned findings (full
coverage; no partial-gate degradation). No P0 or P1 findings were raised. Several
converging P2 findings were surfaced and have been folded into the plan
(Requirements Trace, Implementation Units, Test Plan, Risks) before harvest, so
only P3 advisory items remain outstanding. Decision: ADVISORY; operator proceed
authorized per the standing staging instruction (course-correct at the staging
PR). Plan hardening was NOT required (`Requires plan hardening: no`) and this is
consistent with the additive, low-blast-radius nature of the change.

**Plan hardening:** not required; not applicable.

### P0 / P1 findings

None.

### P2 findings (surfaced, now RESOLVED in the plan)

* **Config-map test coverage (Constitution + Go, consensus).** The primary
  positive tests use `ValidateStatusTransition(nil)`, which exercises only the
  hooks fallback map; the production path (`workspace.go:114-118`) wires the
  config map (`defaults.go`). Resolved: added a production-map positive test to
  the Test Plan (Unit 1 step 5) so the `defaults.go` edit has its own red->green.
* **Persisted `hooks.yaml` shadowing (Go).** An existing workspace with a
  serialized `lifecycle.transitions` block could bypass the in-code default fix.
  Resolved: added a Requirements-Trace / Unit-2 verification of `backlogit init`
  behavior plus a migration-note contingency.
* **Two-map duplication / sync guard (Architecture + Learnings).** Resolved:
  made the deep-equality sync guard MANDATORY (dropped the weak per-package
  containment fallback), confirmed no import cycle exists, and cited the drift-test
  compound learning.

### P3 findings (advisory — tracked, non-blocking)

* Align the `redirectGate` bypass rationale comment
  (`gate_transition.go:266-267`) — folded into Unit 2.
* Sweep `internal/core`, `internal/mcp`, `internal/cli` for other stale invariant
  comments — folded into Unit 2.
* Add a targeted claimed-shipment manual-requeue consistency test (Architecture) —
  noted as optional hardening in the plan's Risks.
* Agent-native discoverability follow-ups (not in scope): surface the lifecycle
  transition map in the metadata catalog, and add a requeue hint to the MCP
  `backlogit_doctor` tool description at parity with the CLI long-help. Recorded as
  future follow-ups, not blockers for this change.
* Constitution Check now explicitly marks NON-NEGOTIABLE principles IV/VII/VIII/XI
  as N/A for auditability.

### Runtime verification / operational closure

No runtime surface, migration, or rollout risk beyond the additive transition
widening. Ship's normal quality gates (`go test ./...`, `go vet ./...`,
`golangci-lint run`, `gofmt -l .`) plus the TDD test plan constitute sufficient
verification. No monitoring/rollback artifacts required for an in-code additive
lifecycle change.
