# Stage session — 2026-07-01 — doctor --target nil-HeaderDef hardening (072-S)

## Intake
- Stash `C16DBBEB` (task, medium/P2) — "[071-S PR#156 Copilot follow-up K]".
  `internal/core/doctor_target.go` `ValidateDoctorTargetResolved` returns `kind=pass` when
  `ws.HeaderDef == nil` (validation silently skipped → fail-open defect). Defensive-only,
  non-regression; HeaderDef is always loaded via `config.WriteDefaults` in a normal workspace.
- Single entry explicitly targeted → Step 1.5 single-entry fallback (no grouping); solo group
  with a synthesized covering feature.
- Deferred (left untouched, still active): `21E17BFC`, `D070FD3C`, `9140F65C`, `6B2C2E53`.

## Pipeline decisions
- **Deliberation gate (Step 2): NO separate `deliberate` artifact.** The kind/exit-code choice
  resolves cleanly against the shipped 071-S contract + Ship's thread acknowledgment; per the
  lean-plan directive the A-vs-B rationale was folded into the plan's Decisions section.
- **Core decision — Option B: nil HeaderDef → `kind=DoctorTargetIO` (exit 3)** with a distinct
  "header definition not loaded" diagnostic. Rationale: a nil workspace schema is a system/config
  precondition fault, NOT user-correctable field validation. Exit 1 (`validation`) would falsely
  blame the target artifact. The 071-S contract already routes system faults that block completing
  the check to `io`/exit 3. Reuses the existing kind → no new kind, no exit-code table change, no
  `DoctorTargetResult` schema-version bump. Rejected: new `config`/`internal` kind (would expand
  the versioned 0–4 contract; disproportionate for an unreachable edge).
- **CLI + MCP consistency is structural:** both surfaces route through the single shared
  `core.ValidateDoctorTargetResolved` (CLI via `runDoctorTargetMode`; MCP via
  `handleDoctor → core.DoctorTarget`). Fix one function → both inherit. No per-surface tests
  needed (kept task to 2 files / 2-hour rule).

## Artifacts produced
- Plan: `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md`
  (`Requires plan hardening: no` — all 5 signals absent w/ justification → P-006 satisfied,
  no plan-harden needed).
- Plan review: 6 personas (Constitution, Go, Scope, Learnings, Architecture, Agent-Native Parity).
  No P0/P1. One **P2** (verification narrowed `go test`) → gate ADVISORY → RESOLVED inline
  (full quality-gate set now mandated) → final **gate = PASS**. P3s folded (test via
  `DoctorTarget` wrapper; commit scenario 2; cite compound prior; `DoctorTargetIO` doc-comment;
  intentional field carry-over) or deferred (structured `io_reason` field + MCP tool-desc doc —
  YAGNI/scope). Recorded as `## Plan Review` in the plan.
- Learnings prior applied: `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`
  ("nil zero value must fall to the safe path") — canonical match, high confidence.

## Backlog output
- Feature: **`072-F`** — "doctor --target: fail closed on nil header-def".
- Task: **`072.001-T`** (parent 072-F, test-first, 2 files, AC1–AC5).
- Shipment (queued): **`072-S`** — items `[072-F, 072.001-T]` (parent-first; scope guard applied:
  only the harvest IDs, no scavenging of deferred stash entries).
- Stash `C16DBBEB`: **archived** with forward-link text → `072-F` / `072.001-T` / `072-S` / plan.

## Handoff to Ship
- **`shipment_id = 072-S`** (queued). Ship claims it, implements 072.001-T test-first, runs the
  full quality-gate set, opens/merges the PR. Stage did NOT build, branch, or open PRs (role
  boundary honored).

## Observation (continuous-learning, light)
- Recurring: "071-S PR#156 Copilot follow-up" stash items are small defensive hardening of the
  doctor-target exit-code contract. The nil→fail-closed pattern already has a durable compound
  learning that applied directly — no new compound entry warranted (prior already exists). Sibling
  follow-up `6B2C2E53` (follow-up J, P3, same file, error-text preservation) remains deferred.
