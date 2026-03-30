---
title: "Plan Review: Queue Features — CLI Commands, Header Definitions, Templates, and Dynamic Tools"
date: 2026-03-30
plan: ".backlog/plans/2026-03-30-queue-features-plan.md"
gate: advisory
reviewers: [constitution-reviewer, go-quality-reviewer, architecture-strategist, scope-boundary-auditor]
---

# Plan Review: Queue Features

## Gate Decision: ADVISORY

The plan is architecturally sound, follows the constitution, and addresses the queue requirements. P2 findings below are advisory observations that the implementer should consider but do not block task decomposition.

## Summary

- **P0 (Critical)**: 0 findings
- **P1 (High)**: 0 findings
- **P2 (Moderate)**: 4 findings
- **P3 (Low)**: 3 findings
- **Total**: 7 findings across 4 reviewer perspectives

## Findings

### P0: Critical (must fix before proceeding)

None.

### P1: High (should fix before proceeding)

None.

### P2: Moderate (user discretion)

#### F1: Config Source-of-Truth Overlap

- **Severity**: P2
- **Category**: architecture / cohesion
- **Unit(s)**: Unit 6, Unit 7
- **Reviewer**: Architecture Strategist
- **Issue**: The plan introduces `header-def.yaml` as a new config file for per-type field schemas, but `config.yaml` already has `artifact_types` (with prefix, name_format, allowed_children) and `fields` sections that overlap. Two config files defining type behavior creates ambiguity about which is authoritative for type definitions. The `ArtifactTypeConfig` struct already has `Prefix` and `NameFormat`; `header-def.yaml` proposes its own `prefix` and `id_format`.
- **Recommendation**: Extend `config.yaml` with the header-def fields rather than introducing a separate file, or clearly document the boundary: `config.yaml` owns workspace-level behavior (routing, naming) while `header-def.yaml` owns per-type field schemas (immutable defaults, field types, validation rules). The plan should specify this boundary explicitly.

#### F2: Dynamic Tool Generation Sizing

- **Severity**: P2
- **Category**: sizing / granularity
- **Unit(s)**: Unit 20
- **Reviewer**: Scope Boundary Auditor
- **Issue**: Unit 20 (Dynamic MCP Tool Generation) involves: parsing template configs, generating tool schemas at runtime, creating handler closures that delegate to core CRUD with section-awareness, preventing naming collisions, and registering dynamically. This is a complex unit that likely exceeds 2 hours.
- **Recommendation**: Split Unit 20 into two units: (20a) Template-to-tool-schema generation and collision detection, (20b) Dynamic handler creation and registration with section-aware delegation.

#### F3: Backward Compatibility with Existing Artifacts

- **Severity**: P2
- **Category**: missing-concern
- **Unit(s)**: Unit 6, Unit 7, entire plan
- **Reviewer**: Constitution Reviewer
- **Issue**: The plan proposes changing the ID format to `OP{NNN}` per queue requirements, but existing TASK-001 artifacts use `{prefix}{NNN}-{title_slug}` format (e.g., `T001-implement-jwt`). The plan doesn't address migration or dual-format support for existing workspaces. Principle VIII (Git-Friendly Persistence) requires minimizing merge conflicts and maintaining stability.
- **Recommendation**: Add a compatibility note in Unit 6 specifying that `header-def.yaml` supports configurable ID formats, and the default `OP{NNN}` applies only to new workspaces. Existing workspaces retain their `config.yaml` naming patterns until explicitly migrated.

#### F4: Missing slog Instrumentation Plan for CLI Commands

- **Severity**: P2
- **Category**: observability
- **Unit(s)**: Units 11-17
- **Reviewer**: Constitution Reviewer
- **Issue**: Principle V (Structured Observability) requires all significant operations to emit slog entries. The CLI command units (11-17) don't explicitly call out slog instrumentation in their approach sections. The verification criteria focus on functional correctness but not observability.
- **Recommendation**: Add a cross-cutting note that all CLI commands must include `slog.Info` for command entry/exit, `slog.Debug` for intermediate steps, and `slog.Error` for failures. This can be a single note rather than per-unit repetition.

### P3: Low (advisory)

#### F5: Combined Units for Distinct Concerns

- **Severity**: P3
- **Category**: granularity
- **Unit(s)**: Unit 16, Unit 17
- **Reviewer**: Scope Boundary Auditor
- **Issue**: Unit 16 combines `delete` and `search` commands; Unit 17 combines `query` and `status` commands. These are functionally distinct. While each pair is small enough to fit in 2 hours together, separating them would produce cleaner task isolation for parallel development.
- **Recommendation**: Acceptable as-is for efficiency, but note that the harvester may split these into separate tasks if the combined units feel overloaded during decomposition.

#### F6: Template Inheritance Not Addressed

- **Severity**: P3
- **Category**: scope / deferred
- **Unit(s)**: Unit 8
- **Reviewer**: Architecture Strategist
- **Issue**: The plan lists template inheritance chain depth as "Deferred to Implementation" but the template schema in Unit 8 doesn't include any extension mechanism. If a Sub-Task template wants to inherit sections from the Task template, there's no path for that.
- **Recommendation**: Acceptable deferral. Document in the template schema that `extends: task` is a reserved future field.

#### F7: SQLite ALTER TABLE Assumption

- **Severity**: P3
- **Category**: database
- **Unit(s)**: Unit 2
- **Reviewer**: Go Quality Reviewer
- **Issue**: The Risks section correctly notes that SQLite's `ALTER TABLE ADD COLUMN` has limitations, and that the ephemeral cache makes this safe. However, Unit 2's approach doesn't explicitly state whether it will use `ALTER TABLE` or recreate the schema. Since `EnsureSchema` uses `CREATE TABLE IF NOT EXISTS`, new columns won't appear on existing databases without a drop.
- **Recommendation**: Unit 2 should explicitly call `EnsureSchema` with a version check or simply document that `backlogit sync` rebuilds the schema from scratch (which is the existing behavior via rehydration).

## Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F1 | Architecture Strategist | GPT-5.4 |
| F2 | Scope Boundary Auditor | GPT-5.4 |
| F3 | Constitution Reviewer | Claude Sonnet |
| F4 | Constitution Reviewer | Claude Sonnet |
| F5 | Scope Boundary Auditor | GPT-5.4 |
| F6 | Architecture Strategist | GPT-5.4 |
| F7 | Go Quality Reviewer | Claude Sonnet |

## Next Steps

Gate decision is **ADVISORY** (P2 findings only). Recommended actions:

1. **Proceed to harvester** with these advisory notes carried into the epic description (Recommended)
2. **Revise the plan** to address F1-F4 before decomposition
3. **Discuss specific findings** before deciding
