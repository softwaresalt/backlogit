---
chunk_strategy: h1-h2-h3
description: "Ship agent session summary for 136-S execution — 154-F docline S2 completion"
doc_type: closure
schema_version: "1.0"
source: docs/memory/ship-136-s-closure-20260904.md
title: "136-S Ship Session Closure — docline report-contract and decode-policy convergence"
---

# 136-S Ship Session Closure

**Date**: 2026-09-04
**Agent**: Ship
**Session scope**: Shipment 136-S / Feature 154-F

## Tasks Completed

| Task | Title | Commit |
|---|---|---|
| 154.001-T (U1) | Normalize MigrateReport Applied/Skipped to always-array | a383d284 |
| 154.002-T (U2a) | Add always-array findings channel to MigrationPlan/MigrateReport | fb398f3c |
| 154.003-T (U2b) | ApplyMigration ErrPlanHasFindings guard + CLI/MCP render/mapper | ec5dc13d |
| 154.004-T (U2c) | PlanMigration report-and-continue via shared decode policy | f382be9a |

**Review remediation**: 3511719e (P1-1/P1-2/P1-4), eac32cf6 (Copilot Co2/Co3/Co4), 1f31023b (Copilot Co1 follow-up)

**Merge commit**: ee30d77fb029eeed72d58516d61f4d75a3c9bc13 (PR #415)

## Files Modified

**Production**:
- `internal/docline/report.go`: U1 + U2a
- `internal/docline/service.go`: U2a type + U2b guard + U2c report-and-continue
- `internal/docline/policy.go`: U2b ErrPlanHasFindings sentinel
- `internal/docline/normalize.go`: U2c two-%w ErrFrontmatterDecode discriminator
- `internal/cli/docs.go`: U2b SilenceErrors + render-before-return
- `internal/mcp/docs_tools.go`: U2b planHasFindingsResult + handler mapping

**Tests**: 154_001..154_004 harness files across docline/cli/mcp

## Decisions

1. **Wave order enforced**: U1 independent; U2a→U2b→U2c sequential (explicit backlog deps)
2. **Guard at apply boundary not adapters**: single enforcement point
3. **SilenceErrors: true**: consistent with newDocsLintCommand pattern; Cobra error line is redundant noise
4. **dryRun=true on rejection**: rejection ≠ apply; dry_run:false misrepresents outcome
5. **Structural equivalence not byte-equality**: LintTree and PlanMigration Findings are allowed to differ in prefix text

## Review Iterations

- Adversarial review (10 personas, 2026-09-04): READY_WITH_FOLLOWUPS; 2 HIGH P1s fixed in-PR
- Copilot review cycle 1 (3511719e): 4 threads; Co2/Co3/Co4 fixed in eac32cf6; Co1 declined (design choice)
- Copilot review cycle 2 (eac32cf6): 1 thread; Co1 follow-up test added in 1f31023b; resolved
- Copilot review cycle 3 (1f31023b): COMMENTED, 0 unresolved threads → P-018 SATISFIED

## Deferred Scope (P-021 captures)

| Stash ID | Description |
|---|---|
| 854C7DDD | NewFindingReports shared helper (P1-3) |
| 86A0B65B | Rename applyDecodeFailure (P1-5) |
| B4676755 | MigrationPlan.IsExecutable() / doc invariant (P1-6) |
| 0F67B2F9 | ErrConcurrentEdit/ErrBodyMutated MCP types (P2-8) |
| F8E6D5CA | wrapDecodeFailure shared constructor (P2-9) |

## Closure Status

- ✓ All 4 tasks done
- ✓ PR #415 merged (merge commit only, P-009)
- ✓ 136-S shipped (archived: 154.001-T, 154.002-T, 154.003-T, 154.004-T, 154-F, 136-S)
- ✓ Compound learnings written (2 new, 1 updated)
- ✓ Post-merge PR awaiting closure
- ✓ Releasability: READY (no external dependencies, behavioral fix only)

## Compound Learnings Written

- NEW: `2026-09-04-two-percent-w-discriminator-for-shared-error-policy.md`
- NEW: `2026-09-04-plan-channel-apply-guard-sentinel-report-and-continue.md`
- UPDATED: `2026-07-21-omitempty-defeats-arrays-always-json-contract.md` (added MigrateReport instance)

## Runtime Verification (all PASS)

- RV-1: dry-run mixed corpus → findings:[decode_error], applied:[], skipped:[]
- RV-2: zero-apply → applied:[], skipped:[...], findings:[] (always-array)
- RV-3: apply on findings-bearing corpus → exit 1, ErrPlanHasFindings, findings in JSON, zero writes
- RV-4: text render includes findings section (mirrors printLintText)
- RV-5: SilenceErrors fix → no Error: line in output
- RV-6: dry_run:true in rejected-apply JSON

## Next Session

- Restart cursor: `last=136-S, next=137-S`
- 137-S pre-claim eligibility: 136-S shipped ✓; verify 136-S is no longer blocking 137-S dep edge before claiming
