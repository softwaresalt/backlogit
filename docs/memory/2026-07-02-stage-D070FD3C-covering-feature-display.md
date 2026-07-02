# Stage checkpoint — D070FD3C shipment covering-feature display

- **Phase**: planning (impl-plan drafted, pre plan-review)
- **Stash**: D070FD3C (feature, low) — surface covering feature ID/title in shipment views
- **Governing decision**: docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md (B8FF7590) — offset is BENIGN; only safe fix is a FORWARD display convention, NEVER a retroactive manifest/title rewrite. This serves as the deliberation basis; no new deliberation artifact created.

## Triage
- Shape: feature-shaped (single coherent capability). Solo entry (operator-targeted). No grouping analysis needed.
- Other stash entries 21E17BFC, 9140F65C: NOT touched this session (Step 5.5 scope guard).

## Grounding (real code)
- CLI: internal/cli/shipment.go — `shipment list` (table/tile via artifactsToRows OR --json raw encode; flags --status,--format) and `shipment get` (JSON-only via core.GetShipment). Shared columns in internal/cli/list.go (artifactColumns/artifactsToRows) — do NOT mutate shared columns; use shipment-specific rendering.
- MCP: internal/mcp/tools.go — handleGetShipment (toolResultJSON(shipment)) + handleListShipments (normalizeShipmentItems then toolResultJSON). Raw Artifact JSON shape; list/get same-shape contract (shipment_response_test.go, contract_consistency_test.go).
- Core: internal/core/shipment.go — GetShipment, shipmentItems() (normalizes []string/[]any at read edge), loadArtifact() -> bldb.GetItem. models.Artifact has ID/Title/ArtifactType/Level/CustomFields.
- Covering feature = shipment item with ArtifactType=="feature" and lowest Level (level 1); parent-first first match on ties. custom_fields.items holds member IDs.

## Prior art (compound)
- go-patterns/f015-shipment-stash-patterns.md — normalize CustomFields["items"] at read edge ([]any->[]string). Reuse shipmentItems().
- workflow-issues/cli-reference-drift-check-*.md — any Cobra Short/Long/flags change on shipment cmds => run `make docs` + commit regenerated docs/cli-reference/*.md or CI "CLI Reference Drift Check" fails.

## Key design decisions
1. Display covering feature **ID + title**, both CLI and MCP.
2. Where: CLI `shipment list` (COVERING FEATURE column in table/tile + additive keys in --json) and `shipment get` (additive keys in JSON); MCP list_shipments + get_shipment (additive keys in response JSON).
3. Mechanism: render-time enrichment injecting `covering_feature_id` + `covering_feature_title` into a CLONE of the returned artifact's custom_fields. NEVER persisted. Additive/optional — preserves raw-Artifact response shape and list==get same-shape contract. Aligns Principle X (agent context efficiency).
4. READ-ONLY invariant: no manifest/title/custom_fields storage mutation; derivation at render/response time only. Regression test asserts db/file unchanged.
5. Edge cases: zero features -> omit/empty (ok=false); multiple features -> level-1 covering feature (parent-first first match); unresolvable/archived item -> skip defensively; works regardless of shipment status.

## Decomposition (feature + 3 tasks, all TDD-first)
- T1 core: CoveringFeature(ctx,ws,shipment)->(id,title,ok) in internal/core. FOUNDATION.
- T2 CLI: surface in shipment list + get. depends T1. flags CLI Reference Drift regen.
- T3 MCP: surface in handleListShipments + handleGetShipment. depends T1.
- Deps: T2->T1, T3->T1. T2 & T3 parallel after T1.

## Next steps
- plan-review gate -> harvest (feature + 3 tasks) -> assemble queued shipment (scope guard: only harvested IDs) -> archive stash D070FD3C.

---

## SESSION-END CLOSURE (final, post-review)

**Status:** COMPLETE. Pipeline ran end-to-end; handoff token = queued shipment `075-S`.

### Deliberation
- NO new deliberation artifact created. Governing decision already settled/durable in B8FF7590
  (`docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md`): offset is
  BENIGN; only safe remedy is a forward DISPLAY convention, never a manifest/title rewrite.
  Remaining choices folded into the impl-plan (lean-plan directive honored).

### Plan
- Path: `docs/exec-plans/2026-07-02-shipment-covering-feature-display-plan.md`
- Hardening gate (3.2): `Requires plan hardening: no` -> proceeded to review.

### Plan-review outcome
- Attempt 1 = FAIL: two converging P1s (derived data in custom_fields => agent write-path bypass;
  ok=false shape ambiguity). Claimed P0 (Level==1 rule) VERIFIED false-positive via DB query.
- Revised plan; Attempt 2 = PASS (advisory). `<!-- plan-review-attempt: 2 -->`. Both review
  records appended to the plan.

### FINAL design decisions (supersede mid-session decisions #3/#5 above)
1. Display covering feature **ID + title**, both CLI and MCP, on list + get.
2. Emit a **segregated TOP-LEVEL** `covering_feature: {id,title}` object — NOT keys inside
   custom_fields (revised after P1: an agent echoing custom_fields back through a write tool would
   persist derived data). Shared core envelope embeds `*models.Artifact` + a `*CoveringFeature`
   POINTER (`json:"covering_feature,omitempty"`; pointer required so omitempty actually omits).
3. READ-ONLY structural guarantee: derived data never enters the persisted map; resolution uses
   pure `bldb.GetItem` (NOT `loadArtifact`, which upserts on cache-miss = a DB write).
4. ok=false / zero-feature -> OMIT the object (pointer nil), identically on all four surfaces.
5. Multiple features -> covering = ArtifactType=="feature" AND root (dotless id), corroborated by
   Level==1, parent-first first match.
6. Errors: skip only on errors.Is(ErrNotFound); slog.WarnContext + skip otherwise (never swallow).
7. CLI table: append COVERING FEATURE column to a COPY of shared artifactColumns (never mutate the
   shared slice in list.go). `shipment get` stays JSON-only (no --format split = YAGNI).
8. Single shared shaper for CLI+MCP; dedicated CLI==MCP parity test.

### Harvest (P-003 validated)
- Feature `075-F` "Surface Covering Feature in Shipment Views" (priority low; labels
  shipment/cli/mcp/forward-ux/read-only).
- `075.001-T` core derivation+shaping helper (FOUNDATION, ~2h, test-first).
- `075.002-T` CLI shipment list/get (depends 075.001-T; flags CLI Reference Drift regen in AC).
- `075.003-T` MCP shipment tools (depends 075.001-T).
- Dep edges: 075.002-T -> 075.001-T (blocks); 075.003-T -> 075.001-T (blocks). T2 & T3 parallel after T1.

### Shipment assembly (Step 5.5)
- Queued shipment `075-S`, items `[075-F, 075.001-T, 075.002-T, 075.003-T]` (parent-first,
  dependency-ordered). Scope guard respected: unrelated stash 21E17BFC / 9140F65C NOT scavenged.

### Stash archival (Step 5.6)
- `D070FD3C` forward-linked (feature 075-F / shipment 075-S) then archived (status=archived).
- Deferred debt captured as new stash `17D29DDC` (task/low): consolidate mcp.normalizeShipmentItems
  into a single exported core normalizer (reviewer P3, deferred to avoid scope creep).
- Untouched active stash: 21E17BFC, 9140F65C. Orphan 016.001-R left alone.

### Handoff
- **Ship's input token = shipment `075-S`** (queued). NOT the feature ID.
- CLI Reference Drift regen (`make docs`) is Task 075.002-T's responsibility (Ship executes).

### Constraints honored
- Stage scope only: no code/branches/builds/PRs. Did NOT touch operator in-flux files
  (.backlogit/hooks_queue.jsonl, .github/agents/*, .cursor/, .github/copilot/, .gitignore).
- End-of-session `backlogit sync` -> INDEX_SYNC_OK (669 artifacts).
