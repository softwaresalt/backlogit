---
title: "Plan Review: Workspace Governance and Integrity"
date: 2026-04-10
plan: "docs/exec-plans/2026-04-10-workspace-governance-integrity-plan.md"
gate: fail-then-revised
reviewers: [constitution-reviewer, go-quality-reviewer, architecture-strategist, scope-boundary-auditor]
revision: "Plan revised 2026-04-10 addressing all 20 findings. Re-review recommended."
---

## Gate Decision: FAIL → REVISED

3 P0 findings require plan revision before harvest.

## Summary

| Severity | Count | Action          |
|----------|-------|-----------------|
| P0       | 3     | Must fix        |
| P1       | 9     | Should fix      |
| P2       | 8     | User discretion |
| P3       | 0     | Advisory        |
| **Total**| **20**| (17 after dedup)|

## Findings

### P0: Critical (must fix before proceeding)

**F-001 (from CR-001): Test-first discipline missing from code units**
Units 2, 3, 4, 5, 6 do not reserve explicit test-first subunits. The plan
only makes tests explicit in Unit 1. Constitution Principle III (Test-First
Development) is non-negotiable.

Recommendation: Add red-test subunits preceding each implementation unit:
hierarchy validation tests for Units 2/3, doctor core tests for Unit 4, MCP
contract tests for Unit 5, and ship/archive invariant tests for Unit 6.

**F-002 (from CR-002): Write-only enforcement relies solely on instructions**
Decision D5 uses instructions rather than hard enforcement for .backlogit
write-only discipline. Constitution Principle IV (Workspace Containment)
requires path validation at the file-operation boundary, not just
documentation.

Recommendation: Clarify that existing SafeResolve containment covers
tool-level enforcement. Unit 7 instructions address agent-level discipline as
a complementary layer. If no tool-level gap exists, downgrade to P2 advisory.
If a gap exists, add enforcement code.

**F-003 (from GQ-001): HarvestStashEntry data loss on new validation failure**
`HarvestStashEntry` removes and rewrites the stash file before
`CreateArtifact` runs. When Unit 2 adds hierarchy validation, a failed harvest
will return an error after the stash entry has already been deleted.

Recommendation: Unit 3 must validate hierarchy constraints before removing the
stash entry, or restore the entry on CreateArtifact failure. Add a test that a
failed harvest leaves the stash entry intact.

### P1: High (should fix before proceeding)

**F-004 (from CR-003): Post-ship warn-only too weak for source-of-truth consistency**
Decision D3 allows archive/markdown inconsistencies to survive a ship
operation. Principle VII (CQRS) treats markdown as authoritative.

Recommendation: Make source-of-truth consistency failures block or roll back
shipping. Reserve warnings for non-authoritative diagnostics only.

**F-005 (from CR-004): Doctor/verification must read markdown, not SQLite**
The plan does not specify whether integrity checks operate on markdown/JSONL
artifacts or the ephemeral SQLite cache. Principle VII requires the cache to be
disposable.

Recommendation: Specify that integrity checks read .backlogit/ markdown/JSONL
artifacts directly. SQLite may assist performance as a rebuildable index only.

**F-006 (from CR-005): MCP tool underspecified for protocol fidelity**
Unit 5 does not require unconditional tool visibility, descriptive pre-init
errors, or schema generation from typed structs. Principle II requires all of
these.

Recommendation: Add acceptance criteria: tool always visible, returns
descriptive errors before init, derives JSON Schema from Go structs.

**F-007 (from GQ-002): LevelForType nil guard missing**
`LevelForType(layout, artifactType)` dereferences `layout.Levels` with no nil
guard. New hierarchy validators or doctor checks calling it when QueueLayout is
unset will panic.

Recommendation: Guard `ws.Config.QueueLayout == nil` or harden LevelForType to
return an error. Add test for no-queue-layout workspaces.

**F-008 (from GQ-003): Hierarchy errors must wrap ErrValidation**
If the new "missing parent for level-2+" rejection uses plain `fmt.Errorf`,
MCP handlers classify it as `internal` instead of `validation_failed`.

Recommendation: Wrap hierarchy rejections with `internal/errors.ErrValidation`
using `%w`. Test MCP surface for `validation_failed` error code.

**F-009 (from GQ-004): Intentional orphans cause false positives**
`ShipShipment` intentionally clears `parent_id` on unreleased descendants.
`IsOrphan`/`AdoptItem` treat those as valid orphan backlog items. A doctor
rule that flags every level-2+ item with nil parent_id will produce false
positives.

Recommendation: Scope Unit 2 to creation-time validation only. Unit 4 should
distinguish intentional orphans (shipped/returned) from corruption. Add tests
covering shipped-orphan items.

**F-010 (from AS-001): Duplicate check coupled to filenames**
The duplicate check derives IDs from filenames, but filenames are configurable
(`file_name_format`) and already differ from IDs for some types.

Recommendation: Walk files, parse frontmatter for `id`, or compare indexed
artifact IDs. Do not derive identity from basenames.

**F-011 (from SB-001 + AS-005, merged): Policy references tooling not in scope**
Unit 8 hybrid archival policy references "doctor check for stale done items"
but that capability is not in any implementation unit. Track D claims
independence but depends on Track C tooling that does not exist.

Recommendation: Remove stale-done detection from the policy, or add it as an
explicit requirement/unit. Keep Unit 8 strictly policy-only if auto/time-based
behavior is deferred.

**F-012 (from SB-002): Unit 7 scope gap vs originating task**
Task 025.011-T requires constraints in constitution, skills, and agent files.
Unit 7 only updates two instruction documents, narrowing scope without
acknowledging the omission.

Recommendation: Expand Unit 7 to include agent/skill surfaces, or explicitly
narrow the task scope and move remaining harness changes to a follow-up.

### P2: Moderate (user discretion)

**F-013 (from CR-006 + SB-004, merged): Doctor output over-engineered before proving utility**
The plan hardcodes multiple output modes and check selectors while also
deferring format details. Combined with unbounded response schema, this creates
unnecessary surface area.

Recommendation: Define one canonical structured report (compact JSON schema
with issue type, artifact IDs, counts, severity, remediation hints). Defer
dual-format presentation and richer flag design to a follow-up.

**F-014 (from CR-007): Missing observability requirements**
Units 4, 5, 6 introduce significant integrity operations without explicit
logging/telemetry requirements.

Recommendation: Add `log/slog` instrumentation and telemetry events for scan
start/end, detected issues, ship verification outcomes, and MCP invocation
summaries.

**F-015 (from GQ-005): Queue absence verification method**
Queue-file absence can be implemented incorrectly if it reuses
`FindArtifactPath` or `loadArtifact`, which search across registered
directories and can find the archived file.

Recommendation: Verify queue absence with explicit queue-root path check
(`os.Stat` on expected queue path), not artifact lookup helpers.

**F-016 (from AS-002): Hardcoded paths bypass registry abstraction**
The plan hardcodes `.backlogit/queue/` and `.backlogit/archive/` as integrity
boundaries, but placement is registry-driven.

Recommendation: Resolve active/archive roots through workspace/registry
abstractions, not literal paths.

**F-017 (from AS-003): Doctor should accept Workspace aggregate**
Doctor is planned as `(DB, workspace path) -> DoctorReport`, splitting one
operation across two low-level dependencies instead of the existing Workspace
aggregate.

Recommendation: Have core doctor operate on `*core.Workspace` or a narrow
interface exposing DB, config, and storage roots.

**F-018 (from AS-004): ShipShipmentResult contract change understated**
Unit 6 "include offending IDs in shipment result metadata" changes a shared
result contract consumed by CLI, MCP, and tests.

Recommendation: Keep verification purely log-only, or expand Unit 6 to include
result-schema consumers and contract coverage.

**F-019 (from SB-003): CLI surface unverified**
Unit 4 includes CLI command, flags, and output behavior, but verification only
exercises `core.Doctor()`.

Recommendation: Add CLI-focused verification or reduce Unit 4 to core-only and
move CLI wiring to a separate unit.

**F-020 (from SB-005): Unit 7 should be prerequisite for Units 2/3**
The plan's own risk section says hierarchy enforcement will break existing
agent workflows unless harvest protocol/instructions are updated. But the
dependency graph does not reflect this.

Recommendation: Make Unit 7 a prerequisite for shipping Units 2/3, or split
"code complete" from "safe to enable" acceptance criteria.

## Reviewer Attribution

| Finding | Reviewer               | Model            |
|---------|------------------------|------------------|
| F-001   | Constitution Reviewer  | Claude Opus 4.6  |
| F-002   | Constitution Reviewer  | Claude Opus 4.6  |
| F-003   | Go Quality Reviewer    | Claude Opus 4.6  |
| F-004   | Constitution Reviewer  | Claude Opus 4.6  |
| F-005   | Constitution Reviewer  | Claude Opus 4.6  |
| F-006   | Constitution Reviewer  | Claude Opus 4.6  |
| F-007   | Go Quality Reviewer    | Claude Opus 4.6  |
| F-008   | Go Quality Reviewer    | Claude Opus 4.6  |
| F-009   | Go Quality Reviewer    | Claude Opus 4.6  |
| F-010   | Architecture Strategist| GPT-5.4          |
| F-011   | Scope Boundary Auditor + Architecture Strategist | GPT-5.4 |
| F-012   | Scope Boundary Auditor | GPT-5.4          |
| F-013   | Constitution Reviewer + Scope Boundary Auditor | Mixed |
| F-014   | Constitution Reviewer  | Claude Opus 4.6  |
| F-015   | Go Quality Reviewer    | Claude Opus 4.6  |
| F-016   | Architecture Strategist| GPT-5.4          |
| F-017   | Architecture Strategist| GPT-5.4          |
| F-018   | Architecture Strategist| GPT-5.4          |
| F-019   | Scope Boundary Auditor | GPT-5.4          |
| F-020   | Scope Boundary Auditor | GPT-5.4          |

## Next Steps

The plan must be revised to address the 3 P0 findings before proceeding to
harvest. The 9 P1 findings should also be addressed. Recommended revision
priorities:

1. Add explicit test-first subunits to all code units (F-001)
2. Fix harvest stash entry ordering to prevent data loss (F-003)
3. Clarify write-only enforcement layering (F-002)
4. Address intentional orphan semantics in doctor/validator (F-009)
5. Specify markdown-first integrity checking (F-005)
6. Add MCP protocol compliance criteria (F-006, F-008)
7. Fix dependency graph: Unit 7 prerequisite for Units 2/3 (F-020)
8. Remove out-of-scope tooling references from policy (F-011)

After revision, re-run plan-review gate before proceeding to harvest.
