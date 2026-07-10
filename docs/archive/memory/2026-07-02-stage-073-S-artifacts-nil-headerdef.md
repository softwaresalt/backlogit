# Stage session — 2026-07-02 — create/update write-path nil-HeaderDef hardening (073-S)

## Intake
- Stash `266816CE` (task, medium/P2) — "[072-S follow-up] Harden create/update write paths
  against nil HeaderDef fail-open." `internal/core/artifacts.go` `CreateArtifact` (line 224) and
  `UpdateArtifact` (line 514) gate `ValidateArtifactFields` (and, on create, `ApplyFieldDefaults`)
  behind `if ws.HeaderDef != nil`; with a nil workspace HeaderDef required-field validation is
  silently skipped and the write succeeds (fail-open defect). Direct sibling of the shipped 072-S
  doctor `--target` fix; third recurrence of the nil-precondition fail-open shape.
- Single entry explicitly targeted → Step 1.5 single-entry fallback (no grouping); solo group
  with a synthesized covering feature.
- Deferred (left untouched, still active): `21E17BFC`, `D070FD3C`, `9140F65C`, `6B2C2E53`.
  Pre-existing unrelated orphan `016.001-R` noted, out of scope.

## Pipeline decisions
- **Deliberation gate (Step 2): NO separate `deliberate` artifact.** The hard-fail-vs-warn choice
  and error mapping resolve cleanly against the shipped 072-S precedent + two local sibling
  precedents (`validateSizeValue`, `NewMetadataCatalog`, both fail closed on nil HeaderDef → MCP
  `internal`). Per the lean-plan directive the rationale was folded into the plan's Decisions
  section rather than a standalone deliberation.
- **Core decision — HARD-FAIL (fail closed).** Silently skipping required-field validation on a
  mutation write is an unacceptable fail-open at a safety boundary. Rejected: warn-and-proceed
  (would perpetuate the exact defect class the compound learning warns against).
- **Error mapping — wrap `blerrors.ErrConfig`** (NOT `ErrValidation`) → maps to MCP `internal`
  (500) / non-zero CLI exit. Rationale: a nil workspace HeaderDef is a system/config precondition
  fault, NOT user-correctable field validation. Reconciles 072-S Option B (system/config fault) +
  local siblings + the exported-cache compound learning. `blerrors` already imported at
  `artifacts.go:13` → zero import cost; enables `errors.Is` test seam instead of brittle
  message-substring matching.
- **Ordering invariant (load-bearing):** `requireHeaderDef(ws)` MUST run BEFORE
  `ApplyFieldDefaults`/`ValidateArtifactFields` because `headerDef.ResolveFieldSchema`
  (`headerdef.go:66`) dereferences its receiver with no nil-guard → would nil-panic. Verified.
- **CLI + MCP consistency is structural:** both surfaces route through the single shared
  `core.CreateArtifact` / `core.UpdateArtifact` (CLI via add.go/update.go/move.go; MCP via
  tools.go handleCreateItem:700 / handleUpdateItem:750 / move:550). Fix the two core functions →
  both surfaces inherit. Verified MCP quirk: `handleCreateItem` bypasses `domainError` and
  hard-maps all core errors via `InternalError(...)`; `handleUpdateItem` uses `domainError`. Both
  converge to `internal` for this non-`ErrValidation` fault (parity by outcome).
- Shared `requireHeaderDef(ws)` helper covers both call sites (224, 514) — same shape, one helper.

## Artifacts produced
- Plan: `docs/exec-plans/2026-07-02-artifacts-write-nil-headerdef-hardening-plan.md`
  (`Requires plan hardening: no` — all 5 signals absent w/ justification → P-006 satisfied,
  no plan-harden needed).
- Plan review (Step 4): 5 personas (Constitution, Go, Scope Boundary, Architecture Strategist,
  Agent-Native Parity). No P0/P1. Several **P2** advisories → load-bearing ones folded inline
  (ErrConfig wrap for the `errors.Is` seam; ordering invariant re ResolveFieldSchema nil-panic;
  create-vs-update MCP mapping mechanism corrected) → scope-expanding ones deferred (distinct
  agent-facing `workspace_not_initialized` MCP type; MCP tool-description enrichment; optional
  hoisting of requireHeaderDef above pre-hooks). Final **gate = PASS**
  (`<!-- plan-review-attempt: 1 -->`). Recorded as `## Plan Review` in the plan.
- Learnings prior applied: `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`
  ("a nil zero value at a safety boundary must fall to the fail-closed path") — canonical match,
  high confidence; explicitly names `266816CE` as the third recurrence site.

## Backlog output
- Feature: **`073-F`** — "create/update write paths: fail closed on nil header-def"
  (medium, labels `hardening,core,072-S-followup`).
- Task: **`073.001-T`** (parent 073-F, test-first, single 2-hour task, ~2 files, AC1–AC5,
  labels `hardening,go,core,harness-ready`). One coherent task — the two call sites share one
  helper, so no split.
- Shipment (queued): **`073-S`** — items `[073-F, 073.001-T]` (parent-first; Step 5.5 scope guard
  applied: only the harvest IDs, no scavenging of the 4 deferred stash entries or other queue
  items).
- Stash `266816CE`: **archived** with forward-link text → `073-F` / `073.001-T` / `073-S` / plan.

## Handoff to Ship
- **`shipment_id = 073-S`** (queued). Ship claims it, implements 073.001-T test-first (force
  `ws.HeaderDef = nil` via `setupTestWorkspace`, assert validation no longer silently skipped on
  both create and update), adds the shared `requireHeaderDef` helper wrapping `blerrors.ErrConfig`
  ordered before pre-hooks, runs the full quality-gate set, opens/merges the PR. Stage did NOT
  build, branch, or open PRs (role boundary honored).

## Observation (continuous-learning, light)
- Recurring: nil-precondition fail-open at safety boundaries is now the THIRD instance
  (exported-cache zero-value → 072-S doctor path → 073-S create/update write path), all resolved by
  the same fail-closed rule. The durable compound learning applied directly and already named this
  site — no new compound entry warranted. Sibling defensive follow-ups remain in the deferred set.
- observe/learn/evolve skills and agent-intercom broadcast tools are NOT exposed in this session →
  milestones recorded here + in the report instead of broadcast (honest degraded handling).
