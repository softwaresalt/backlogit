# Stage session — stash 17D29DDC (consolidate shipment-items normalization)

- Date: 2026-07-03
- Agent: Stage
- Entry: single task-shaped stash `17D29DDC` (kind=task, priority=low tech-debt), Orchestrator-routed.
- Solo group (Step 1.5 single-entry fallback). Implicit covering feature synthesized.

## Grounding findings (verified against code)

Three related functions exist (stash named only two):

1. `internal/core/shipment.go:525` `shipmentItems(a *models.Artifact) []string` — PURE READER, the
   fundamental `[]any`->`[]string` mapping. Callers: shipment.go (187,229,363,482),
   shipment_covering.go:61, shipment_lifecycle.go (68,161), + core tests.  ← EXPORT target.
2. `internal/core/shipment.go:359` `normalizeShipmentArtifact(a)` — CORE MUTATOR; sets
   `CustomFields["items"]=shipmentItems(a)` + inits nil map. Callers: CreateShipment(64), GetShipment(93).
   NOT named in stash. Gets call-site rename only (out of consolidation scope otherwise).
3. `internal/mcp/tools.go:1543` `normalizeShipmentItems(shipment)` — MCP MUTATOR; duplicates the
   `[]any`->`[]string` switch inline + writeback. Only caller: handleListShipments(1587).  ← DELETE target.

Guard tests:
- `internal/mcp/shipment_response_test.go:145` `TestNormalizeShipmentItems_AllCases` — tests mcp mutator,
  5 sub-cases, uses helper `buildTestShipmentArtifact` (only used here).  ← MOVE to core.
- `internal/mcp/shipment_response_test.go:121` `TestListShipments_EmptyItems_NeverNull` — integration
  through handleListShipments.  ← STAYS in mcp (end-to-end never-null guard).

Data flow: `NewShipmentView(s)`/`NewShipmentViews` embed the artifact unchanged → JSON marshals
`CustomFields["items"]` directly. non-nil `[]string{}`→`[]`, nil `[]string`→`null`. GET path
canonicalizes via `normalizeShipmentArtifact` (GetShipment). LIST path (MCP) canonicalizes via the
mcp mutator. CLI list path (`internal/cli/shipment.go:151`) does NOT normalize — PRE-EXISTING, OUT OF SCOPE.

TRUE DUPLICATES? Mapping logic is byte-equivalent (order-preserving, non-string-filtering, default→empty).
BUT mcp is a mutator vs core `shipmentItems` a pure reader (shape differs). One edge diverges:
empty `[]string{}` input → mcp mutator returns non-nil `[]string{}`; core reader returns nil
(`append([]string(nil))`). So: logically-duplicate normalization + shape diff + one nil-vs-empty edge.

## Design decisions

1. Signature/location: `func NormalizeShipmentItems(artifact *models.Artifact) []string` in
   internal/core/shipment.go (rename shipmentItems; same signature → minimal call-site churn).
2. Never-null invariant lives in the CORE function's return contract: harden the `[]string` branch
   (`make`+`copy`) so empty input → non-nil `[]string{}`; function NEVER returns nil. MCP list handler
   becomes thin adapter: init nil map, `CustomFields["items"]=core.NormalizeShipmentItems(shipment)`.
3. Reconcile to superset (pure read + always-non-nil). Documented convergence, not silent change.
4. Test consolidation: MOVE all-cases unit test to core (add empty-`[]string`→non-nil case);
   KEEP never-null integration test in mcp. Both guard tests stay green.

Covering unit type = `feature` (no `chore` type in this backlog; shipment covering-feature derivation
requires ArtifactType=="feature"). Label tech-debt.

## Pipeline state
- No deliberation artifact (solo trivial refactor; rationale folded into plan). 
- Plan path: docs/exec-plans/2026-07-03-consolidate-shipment-items-normalization-plan.md
- Next: impl-plan → hardening check (expect no) → plan-review → harvest → shipment → archive 17D29DDC.

## FINAL OUTCOME (session complete)

- Deliberation artifact: NONE (solo trivial internal refactor; rationale folded into plan per Stage guidance).
- Plan: docs/exec-plans/2026-07-03-consolidate-shipment-items-normalization-plan.md — docline lint valid, 0 violations (self-lint + post-revision).
- Plan-review gate: FAIL (attempt 1: 1x P1 CLI parity + 1x P2 build-breaker + P2s) -> revised in place -> PASS (attempt 2). Multi-persona: Go, Scope, Constitution, Architecture, Parity.
- Harvest: feature 077-F + task 077.001-T (single atomic test-first task, ~2h, no subtasks - indivisible rename).
- Shipment: 077-S (queued) = [077-F, 077.001-T], parent-first, covering_feature resolves to 077-F. Scope guard honored.
- Consumed stash 17D29DDC: ARCHIVED with forward-link to 077-F/077.001-T/077-S.
- New tracked follow-up: stash 7ECBAC7E (CLI/MCP list parity gap, deferred from review).
- Untouched (per constraint): stash 21E17BFC/9140F65C/EED25928/B55985DD; orphan 016.001-R; operator in-flux files (hooks_queue.jsonl, *.agent.md, .cursor/, .github/copilot/, .gitignore).
- True-duplicate finding: mapping LOGIC is a true duplicate; mcp is a MUTATOR vs core shipmentItems a PURE READER (shape differs); 3rd fn normalizeShipmentArtifact (core mutator) also wraps it. One nil-vs-empty edge reconciled to superset.
- Handoff token to Ship: shipment_id = 077-S.
