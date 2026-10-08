# Stage S4 / 138-S Re-plan Closure

**Date:** 2026-09-05
**Agent:** Stage
**Scope:** Bounded re-plan of shipment 138-S (S4 — cross-surface golden parity harness / fault-line evidence contract), FAIL → truthful PASS.

## Outcome

- **PR #422 MERGED** via merge commit `63167559875fc457066e61bc657731011a8b32e5` (merge commit only, no admin bypass).
- S4 plan `docs/exec-plans/2026-09-03-s4-seq1-parity-harness-plan.md` final `## Plan Review` decision: **PASS** (attempt 3; prior PASS explicitly retained as `## Prior Plan Review (invalidated)`).
- **138-S pre_claim topology gate: exit 0 / "topology gate pass"** — Ship-ready.
- Checkpoint anomalies: **0**. `start.ps1` preserved (modified, never staged/committed).
- PR #404 / #405 / #422: all MERGED, **0 unresolved** Copilot threads.

## Copilot shadow-review loop (P-018)

17 review rounds on PR #422 driven to a clean **SATISFIED: PASS** gate on HEAD `94224a24`. Every thread fixed → commit → push → reply(SHA) → GraphQL resolve. Fix commits: `7cc15421, a5159970, c758a406, 6e70fbbd, 8f0220c8, 82c1107b, 93f49e5f, 4bcf1686, 96d85bd8, 12606419, cc1d2de5, aa740bb5, 05d349aa, ed993725, 1acd3b52, 15d715cf, 94224a24`.

## Contract design (final state)

- `EvidenceArtifact` = shared family-agnostic ENVELOPE; `VerifiedEvidence json.RawMessage` decoded via `FamilyPayload` interface + version-aware `(SchemaVersion,NodeFamily)` registry; v1 family set = `{parity}` (closed, foundation-owned manifest; adding a family = version bump).
- Family payload types + registration live in the shared foundation pkg (binary-independent; freeze fails closed on missing/extra).
- `Validate(a)` derives payload from `a.VerifiedEvidence` (no separate-payload mismatch); family enforces cross-envelope rules via `p.Validate(view EvidenceView)` incl. `Surfaces(set)==view.Applicability` and the Status cross-field matrix.
- Bounded decode (SEC-01), immutable frozen registry (SEC-02), consumer accept-path w/ canonical-digest anti-replay + status-policy step (SEC-03), safe diagnostics (SEC-04). Security Lens RUN (gpt-5.6-sol) — FAIL → remediated → PASS.
- Typed decode errors (`ErrMalformed/ErrUnknownVersion/ErrUnknownFamily/ErrValidation`) → `Classify` four outcomes. `Classify(present=false)` requires empty raw.
- Set-normalized canonical digest; deterministic timestamp projection (normalize, preserve mutation semantics); coarse-equivalence-class exit↔category; CLI/MCP error field mapping.
- 156.004-T = compile-safe declarations + RED AST signature harness; 156.006-T = behavior/bodies; 156.005-T = Classify + four-outcome test + report-only obligations.

## Backlog (138-S manifest = 7 items)

156-F + 156.001-T (U1) + 156.002-T (U2) + 156.003-T (U3) + 156.004-T (U4a-decl) + 156.005-T (U4b) + 156.006-T (U4a-behavior).

Deps: 156.002→{156.001,156.006}; 156.003→156.002; 156.005→156.006; 156.006→156.004. Producer edges 157-161-F→156.006-T; consumer edges 162-163-F→156.005-T.

## Next

138-S is Ship-claim-eligible (depends on 137-S = shipped). Follow-up tracked defect **166-F** (CLI `--json` gate-payload parity drift) queued OUTSIDE the shipment hierarchy — report-only owner, not in 138-S.
