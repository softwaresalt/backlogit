---
chunk_strategy: h1-h2-h3
description: "Execution plan for S2: docline report-contract array fix and decode-policy convergence"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s2-docline-contract-decode-convergence-plan.md
title: "S2 Execution Plan — docline Contract & Decode Convergence"
---

# S2 Execution Plan — docline Contract & Decode Convergence

**Covering feature**: docline report-contract (always-an-array) and decode-policy convergence
**Stash members**: EC987334, 1787FD85
**Tier**: reliability + simplifying refactor (shipment sequence S2)

## Problem Frame

Two internal/docline defects: MigrateReport collection fields vanish on a
zero-apply run (breaking the always-an-array JSON contract), and LintTree vs
PlanMigration carry two divergent decode policies over one frontmatter grammar.
Both are reliability/composability fixes staged ahead of feature work.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; errors wrapped |
| II. Test-First (P-002) | declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | n/a (in-package) |
| IV. CLI Containment | n/a |
| V. Observability | Report shape made deterministic |
| VI. Single Responsibility | Converges on one decode policy; U2b adds a write-path safety guard |
| VII. Destructive Approval | apply path already operator-gated; U2b only tightens it (`ErrPlanHasFindings`) |
| VIII. Safety Modes | PlanMigration read-only report-and-continue is safe |
| IX. Git-Friendly | n/a |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Normalize MigrateReport collection fields to non-nil arrays (EC987334)
* Scope: in `internal/docline/report.go`, remove `omitempty` from `MigrateReport.Applied` and `MigrateReport.Skipped` **and** normalize both to non-nil empty slices in `NewMigrateReport`. Removing `omitempty` alone is insufficient: a nil `[]string` marshals as `null`, not `[]` (compound: `2026-07-21-omitempty-defeats-arrays-always-json-contract.md`). Concretely, initialize `report.Applied = []string{}` and `report.Skipped = []string{}`, then overwrite from `res.Applied`/`res.Skipped` only when those are non-nil, so BOTH the dry-run (`res == nil`) and zero-apply (`res != nil`, empty results) cases emit arrays.
* Acceptance: a table test marshals a MigrateReport for BOTH the dry-run and zero-apply cases, then **decodes the JSON** and asserts `applied` and `skipped` are present as **zero-length arrays** (`[]`, never `null` or absent) — asserting those two keys directly, not sibling fields. (The only `MigrateReport` slice fields besides the already-array `Changes` are `Applied`/`Skipped` here and U2a's `Findings`; no other field needs the same treatment.)
* Also update the `MigrateReport` doc comment (currently "Applied/Skipped present only for an apply … nil for a dry-run") to reflect the new always-array contract in the same commit.
* Files: `internal/docline/report.go` (+ test). Single responsibility (report contract).

### U2a — Durable per-file findings channel for the read-only plan path (1787FD85, part 1)
* Scope: add a durable per-file findings channel so a read-only migration plan can report a malformed file instead of aborting. Domain shape: add `Findings []Finding` to `MigrationPlan` (reusing the existing `Finding` type — File/Field/Rule/Severity/Fix — no new domain type). Transport shape: add `Findings []FindingReport` to `MigrateReport` (reusing `FindingReport`), normalized to a non-nil array (same always-an-array discipline as U1), and map `plan.Findings` in `NewMigrateReport`, building `MigrateReport.Findings` via `make([]FindingReport, 0, len(plan.Findings))` (mirroring `NewLintReport`) so an empty findings list marshals `[]` not `null`. This unit adds the channel ONLY; it does not change PlanMigration control flow yet.
* Compatibility: purely additive — new in-package `MigrationPlan.Findings`; a new `findings` JSON key on `MigrateReport` that is always `[]` until U2b populates it; no existing field changes; existing consumers ignore the new key. `ApplyMigration` untouched.
* Ownership: `internal/docline` maintainers own `MigrationPlan`/`MigrateReport`.
* Acceptance: `MigrationPlan` carries `Findings`; a MigrateReport built from a plan with zero findings marshals `"findings": []`; serialization test locks the always-array shape.
* Files: `internal/docline/report.go`, `internal/docline/service.go` (type only) (+ test). Single responsibility (schema/transport).

### U2b — Guard the apply/write path against a findings-bearing plan (1787FD85, part 2)
* Rationale (corrects the earlier isolation error): `PlanMigration` is the SHARED planner for BOTH dry-run and apply — the apply adapters (`internal/cli/docs.go`, `internal/mcp/docs_tools.go`) call `PlanMigration` then `ApplyMigration` and do NOT inspect `plan.Findings`. Once U2c relaxes `PlanMigration` to report-and-continue (dropping a malformed file from `plan.Changes` and returning no error), an unguarded apply would silently migrate the valid subset instead of aborting the corpus as it does today. "`ApplyMigration` body unchanged" is NOT "apply behavior unchanged". To keep apply's corpus-level all-or-nothing IDENTICAL to today, the write boundary must reject a plan that carries findings, and this guard MUST land BEFORE U2c so the write path is never exposed.
* Scope:
  1. `internal/docline/service.go` (`ApplyMigration`): at the top, if `len(plan.Findings) > 0`, return a new exported sentinel `ErrPlanHasFindings` WITHOUT writing anything — preserving corpus all-or-nothing on the mutating path.
  2. `internal/cli/docs.go` and `internal/mcp/docs_tools.go`: render `plan.Findings` wherever a migrate report is surfaced. Extend the shared render (`writeMigrateResult` text and the JSON shape) so malformed files reported via `plan.Findings` are shown, not silently omitted because they are absent from `plan.Changes` (mirror `printLintText`). On an APPLY, abort via the `ErrPlanHasFindings` rejection, and in MCP map `errors.Is(err, ErrPlanHasFindings)` to a structured, non-`InternalError` (`ValidationFailed`-class) result carrying the findings — a structured JSON result payload (e.g. an `IsError` tool result serializing the findings), not a bare error string. The dry-run/plan and lint paths report findings without aborting.
* Compatibility: preserves today's observable apply behavior — a malformed file still causes an apply to write zero files and return an error (now `ErrPlanHasFindings` + the surfaced findings, instead of the raw decode error). `ApplyMigration`'s atomic/preflight/TOCTOU write logic is otherwise unchanged. Additive sentinel error.
* Acceptance: a mixed-corpus test (one malformed file + valid files) asserts an apply performs ZERO writes and returns `ErrPlanHasFindings` with the malformed file surfaced in CLI text AND JSON; the MCP apply rejection is a structured `ValidationFailed`-class result (not `InternalError`) carrying the findings; a dry-run over the same corpus renders the finding in `writeMigrateResult` output (not omitted); a clean corpus still applies normally.
* Files: `internal/docline/service.go`, `internal/cli/docs.go`, `internal/mcp/docs_tools.go` (+ tests). Single responsibility (write-path safety guard). Depends on U2a.

### U2c — Converge PlanMigration on the shared decode policy, report-and-continue (1787FD85, part 3)
* Scope: make `PlanMigration` report-and-continue per file on a frontmatter decode failure instead of aborting the whole corpus, REUSING the single existing shared policy — no second classifier, no second filesystem read:
  1. `internal/docline/normalize.go`: add the `ErrFrontmatterDecode` discriminator to `Normalize`'s decode-error wrap (mirror `decodeDoc`'s two-`%w` pattern: `fmt.Errorf("docline.Normalize: decode %s: %w: %w", relPath, ErrFrontmatterDecode, err)`), so a frontmatter decode failure from EITHER read path is classifiable by the one policy-neutral `classifyDecodeFailure`. Additive: the original YAML cause stays wrapped via the second `%w`; existing `errors.Is` on the cause is unaffected (verified: the other `Normalize` caller, `cmd/gen-docs/main.go`, does not `errors.Is` the decode cause).
  2. `internal/docline/service.go` (`PlanMigration`): on a `Normalize` error, call the EXISTING shared policy wrapper `applyDecodeFailure(err, rel)` (frontmatter → single `decode_error` Finding + continue; containment/read → fatal). Append findings to `plan.Findings` and `continue`; for a fatal (containment/read/IO) result, re-wrap it as `docline.PlanMigration: normalize %s: %w` so all fatal errors share the `PlanMigration` prefix used by the pre-`Normalize` early returns (outcome unchanged — still a fatal abort).
* No second filesystem read: `PlanMigration` keeps its single `os.ReadFile(abs)`; it does NOT call `decodeDoc`. Classification is driven by `Normalize`'s error.
* Path-asymmetry note (truthful, per review): containment (`SafeResolve` → `ErrPathEscapesWorkspace`) and read/IO (`os.ReadFile`) failures are handled by `PlanMigration`'s PRE-`Normalize` early returns and stay fatal, so they do NOT flow through `applyDecodeFailure`; only the frontmatter-decode case does. "One policy" here means OUTCOME-parity with LintTree (a structurally equivalent `decode_error` Finding for a malformed file — same `File`/`Rule`/`Severity`; caller-function prefix may differ), not that every failure class routes through the same function.
* Apply safety: corpus all-or-nothing on apply is preserved by the U2b guard (this unit DEPENDS on U2b), so relaxing `PlanMigration` never silently migrates a partial corpus.
* Acceptance: a table test over a corpus with (a) one frontmatter-undecodable file, (b) a containment-escaping path, (c) a read/IO failure, (d) the nil-error guard asserts: `PlanMigration` emits a `decode_error` Finding (Rule `RuleDecodeError`) for the malformed file and CONTINUES (dry-run); containment and read/IO remain fatal (empty plan); the emitted Finding is STRUCTURALLY EQUIVALENT to LintTree's for the same file — equal `File`, `Rule == RuleDecodeError`, `Severity == SeverityError`, and a `Fix` containing both `ErrFrontmatterDecode.Error()` and the underlying YAML cause — while the caller-function prefix MAY differ (LintTree's `Fix` originates in `decodeDoc`, PlanMigration's in `Normalize`). Byte-equality is NOT required and MUST NOT be pursued (it would force either stripping `Finding.Fix`, regressing LintTree's shipped output, or a prohibited second `decodeDoc` read). Guardrail: a LintTree regression test pins that its existing `Finding.Fix` output MUST NOT change.
* Files: `internal/docline/normalize.go`, `internal/docline/service.go` (+ table tests). Single responsibility (decode-policy convergence). Depends on U2b.

## Dependency Graph

U1 is independent (report contract). U2a adds the findings channel; U2b guards the
apply/write path against a findings-bearing plan (must precede any relaxation);
U2c relaxes `PlanMigration` to report-and-continue. Order: U1 and U2a may proceed
in parallel, then U2b, then U2c. Backlog mapping: `154.001-T` (U1), `154.002-T`
(U2a), `154.003-T` (U2b, depends `154.002-T`), `154.004-T` (U2c, depends `154.003-T`).

## Runtime Verification and Closure

U1 changes JSON output of migrate report; U2 changes PlanMigration behavior on
malformed input. Verification via table tests. Closure: regression tests.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Restore the migrate-report JSON contract so `applied` and `skipped` are always arrays | Medium — downstream agents and scripts may depend on the exact report shape | Normalize `MigrateReport.Applied` and `MigrateReport.Skipped` to non-nil empty slices before marshaling; tests must decode the JSON and assert both keys are zero-length arrays, not merely present sibling fields |
| Reuse ONE docline decode classifier and ONE policy across LintTree and PlanMigration | Medium — a second classifier/policy could diverge from LintTree and create false report-and-continue behavior | `classifyDecodeFailure` already exists and is policy-neutral (its doc comment anticipates a PlanMigration consumer). PlanMigration reuses BOTH it and LintTree's `applyDecodeFailure` wrapper; make the single classifier reachable from the PlanMigration path by adding the `ErrFrontmatterDecode` discriminator to `Normalize`'s decode-error wrap (additive, second-`%w`). A test asserts no second classifier/switch over the decode kinds exists |
| Add report-and-continue for per-file decode failures via a durable findings channel | Medium — PlanMigration has no durable findings channel and must not perform a second filesystem read | U2a adds `MigrationPlan.Findings` / `MigrateReport.Findings` (additive, always-array); U2c classifies `Normalize`'s error from PlanMigration's single existing read (no `decodeDoc` re-read) and appends findings |
| Preserve apply corpus all-or-nothing when PlanMigration reports-and-continues | **High** — `PlanMigration` is the SHARED planner for dry-run AND apply (`internal/cli/docs.go`, `internal/mcp/docs_tools.go` do NOT inspect `plan.Findings`); relaxing it would make apply silently migrate the valid subset instead of aborting the corpus | U2b adds an `ApplyMigration` guard rejecting a plan with non-empty `plan.Findings` (`ErrPlanHasFindings`, zero writes), landing BEFORE U2c's relaxation (U2c depends U2b); mixed-corpus zero-write regression test; apply's observable abort-on-malformed behavior preserved |

Rollback: revert to the previous report shape and PlanMigration abort behavior if
consumers cannot tolerate the restored contract. Compatibility: this is
contract-restoring and additive for consumers that already expected arrays, but
legacy consumers that distinguished absent from empty must be checked. Ownership:
`internal/docline` maintainers own the shared decode policy and report contract.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — U1 makes promised arrays always present (contract-restoring); U2a ADDS a new always-array `findings` key to the migrate report (additive); U2b adds the `ErrPlanHasFindings` sentinel on the apply path (additive).
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: yes

## Prior Plan Review (invalidated)

dispatch_mode: multi-agent-dispatch
decision: INVALIDATED

The prior PASS record is retained only as invalidated history. It omitted mandatory personas and is superseded by the genuine multi-agent Plan Review below.

## Plan Review

<!-- plan-review-attempt: 3 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

personas:
* Constitution Reviewer (`claude-opus-4.8`)
* Go Reviewer, anchor (`gpt-5.6-sol`, effort high)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Correctness Reviewer (`claude-sonnet-4.6`)
* Architecture Strategist (`grok-4.6`)
* Learnings Researcher over `docs/compound/`

Security Reviewer was not risk-triggered (in-package docline report-contract + decode-policy refactor; no auth/secrets/injection/network surface).

Review history (genuine multi-agent-dispatch, cross-model, 3 cycles):
* Attempt 1 (FAIL): the original 2-unit plan resolved only U1's array claim on paper; controlling P1s remained — nil-slice→`null` insufficiency, no durable per-file findings channel, and no proof the decode reuse avoided a second policy/second filesystem read.
* Attempt 2 (FAIL): after re-authoring (U1 normalize non-nil; U2a findings channel; U2b/U2c decode convergence), a HIGH-confidence cross-model P1 surfaced — `PlanMigration` is the SHARED planner for dry-run AND apply (`cli/docs.go`, `mcp/docs_tools.go` do not inspect `plan.Findings`), so relaxing it would silently migrate a partial corpus on apply. Remediated by splitting out U2b (an `ApplyMigration` `ErrPlanHasFindings` guard) landing before U2c, preserving apply corpus all-or-nothing.
* Attempt 3 (PASS): the apply-path-leak P1 is resolved (guard at the single shared write boundary + enforced `U2a→U2b→U2c` ordering via backlog deps `154.003-T`→`154.004-T`; 6/6 reviewers concur). A cycle-2 P1 (U2c's "byte-equal Finding" acceptance was unachievable and self-contradictory) was corrected to structural-equivalence with a LintTree no-regression guardrail, and findings-surfacing was pinned (CLI text+JSON via `writeMigrateResult`; MCP `ValidationFailed`-class). All three original controlling P1s are resolved in the same contract surface.

Residual (non-blocking P3 advisories, implementation-time): the MCP rejection constructs a structured JSON result (not a bare string); U1 audits all `MigrateReport` collection fields (already factually complete).

Readiness: READY. Same-contract completion of the authorized S2 work; no scope creep. Ship may claim 136-S.
