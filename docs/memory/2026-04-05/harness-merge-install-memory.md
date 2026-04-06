---
date: 2026-04-05
type: session
topic: "harness merge-install"
---

# Memory: Harness Merge-Install Session

**Created:** 2026-04-05 | **Session Type:** Harness Integration and Runtime Fixes

## Task Overview

Merge agent harness primitives from autoharness templates into backlogit repository with runtime support fixes. Objectives:
- Fix deliberation artifact support in live workspace configuration
- Install harness agents, skills, and instruction files into backlogit
- Create autoharness configuration and profiles
- Validate no regressions in markdown/YAML structures
- Avoid duplicate language-specific agents where backlogit already has custom expertise

## Current State

### Completed Work

#### Deliberation Runtime Support (Critical Fix)
- Added `deliberation` artifact type to `.backlogit/config.yaml`
- Added `DL` type schema to `.backlogit/header-def.yaml`
- Created `.backlogit/templates/deliberation.md` with required sections
- **Impact:** `backlogit deliberate` CLI command now has full workspace runtime support

#### Backlogit Agent/Instruction/Skill Additions
- `.github/agents/deliberator.agent.md` — Routes idea work into deliberate/spike workflows
- `.github/instructions/backlogit.instructions.md` — Backlogit workflow rules and query-first protocol
- `.github/instructions/architecture-doc.instructions.md` — Architecture documentation maintenance rules
- `.github/instructions/backlog-integration.instructions.md` — Backlog tool abstraction and operational conventions
- `.github/policies/workflow-policies.md` — Cross-agent sequencing and gate conditions (P-001 through P-005)
- `.github/skills/spike/SKILL.md` — Time-boxed technical investigation workflow
- `.github/skills/runtime-verification/SKILL.md` — Post-build runtime validation
- `.github/skills/operational-closure/SKILL.md` — Release readiness and rollout artifacts
- `.github/skills/safety-modes/SKILL.md` — Elevated-risk interactive workflows (careful, freeze-scope, investigate-first)

#### Backlogit Foundation Updates
- Updated `AGENTS.md` with merged foundation guidance including all 10 harness primitives
- Updated `.github/copilot-instructions.md` with integrated harness surface and corrected `docs/memory/` paths (not deprecated `.backlog/`)

#### Autoharness Configuration (Read-Only Reference)
- Created `.autoharness/config.yaml` — Target workspace settings
- Created `.autoharness/workspace-profile.yaml` — Discovered backlogit tech stack and conventions
- Created `.autoharness/backlog-registry.yaml` — Backlog tool abstraction (MCP and CLI surface mappings)
- Refreshed `.autoharness/harness-manifest.yaml` — Installation metadata with second-wave tuning checksum tracking

### Files Modified

**Configuration & Metadata**
- `.backlogit/config.yaml` — Added deliberation type
- `.backlogit/header-def.yaml` — Added DL schema
- `.backlogit/templates/deliberation.md` — Created from scratch
- `.autoharness/config.yaml` — Created
- `.autoharness/workspace-profile.yaml` — Created
- `.autoharness/backlog-registry.yaml` — Created
- `.autoharness/harness-manifest.yaml` — Updated

**Documentation & Agents**
- `AGENTS.md` — Merged harness primitives and agent inventory
- `.github/copilot-instructions.md` — Integrated harness surface, corrected paths
- `.github/agents/deliberator.agent.md` — New
- `.github/instructions/backlogit.instructions.md` — New
- `.github/instructions/architecture-doc.instructions.md` — New
- `.github/instructions/backlog-integration.instructions.md` — New
- `.github/policies/workflow-policies.md` — New
- `.github/skills/spike/SKILL.md` — New
- `.github/skills/runtime-verification/SKILL.md` — New
- `.github/skills/operational-closure/SKILL.md` — New
- `.github/skills/safety-modes/SKILL.md` — New

## Important Discoveries

### Decisions & Rationale

1. **Deliberation Runtime Support Was Missing** — The `backlogit deliberate` CLI command existed but workspace configuration was incomplete. Adding `deliberation` to config.yaml, header-def.yaml, and the template file ensures the full runtime pipeline works end-to-end for deliberation → stash → plan-review → harvest → implementation.

2. **Deliberately Excluded Language-Specific Agents** — Did NOT import `plan-review.agent.md`, `review.agent.md`, `adversarial-review.agent.md`, or `language-engineer.agent.md` from autoharness templates because:
   - backlogit already has custom `review/SKILL.md` with multi-persona tiered review
   - backlogit already has custom `go-engineer.agent.md` and `go-mcp-expert.agent.md` for Go-specific expertise
   - Creating duplicate agents would create confusion and break the Go codebase's specialized review and linting flows
   - Importing those agents would degrade, not enhance, the existing backlogit harness

3. **autoharness Remains Read-Only** — All changes applied to backlogit; autoharness `.autoharness/` artifacts are reference-only and will not be committed to backlogit repository. These files support future autoharness tuning workflows but do not affect backlogit operational model.

4. **Docs/Memory Path Corrections** — Fixed `.github/copilot-instructions.md` to correctly reference `docs/memory/` as the persistent session memory location (not deprecated `.backlog/` directory which is no longer used).

### Failed Approaches

- Initially considered importing all autoharness agents directly — abandoned because backlogit's specialized skill-first review and Go expertise would have been lost to generic templates
- Attempted to update autoharness manifest without workspace profile — failed; discovered that workspace-profile.yaml is prerequisite for manifest generation

### Validation Performed

- **Markdown/YAML Structure Validation:** Ran `get_errors` on all modified .md and .yaml files; reported 0 errors
- **Cross-Reference Integrity:** Verified all new instruction files and skills are referenced in AGENTS.md and accessible via correct paths
- **Deliberation Workflow Test:** Earlier focused test run confirmed deliberation artifact creation and template rendering works end-to-end
- **No Regressions:** Build gates (go test, go vet, golangci-lint) passed in previous phases; final phase was metadata/docs only, no code changes

## Next Steps

1. **Commit and Push** — Create PR with harness merge-install changes (deliberation support, agent/skill/instruction additions, foundation updates)
2. **PR Review** — Use review skill to validate harness composition and instruction cross-references
3. **Operational Closure** — After merge, update runtime documentation in docs/ to reflect new deliberator agent availability
4. **Monitor Stale Artifacts** — In future sessions, use `doc-ops.agent.md` to scan for outdated harness references and refresh AGENTS.md accordingly

## Context to Preserve

### Agents Active This Session
- deliberator.agent.md (new) — Routes idea work into deliberate/spike workflows
- memory.agent.md — Persisting this checkpoint

### Workspace Updates
- `.backlogit/config.yaml` now includes deliberation artifact type (required for runtime)
- `.backlogit/header-def.yaml` now defines DL type schema
- `.backlogit/templates/deliberation.md` is the canonical template for new deliberation artifacts
- AGENTS.md serves as the authoritative agent inventory (no stale references to old brainstorm docs)
- `.github/copilot-instructions.md` correctly directs memory and compound learnings to `docs/memory/` and `docs/compound/`, not deprecated `.backlog/`

### Deliberate + Plan → Harvest → Build Pipeline
The full lineage is now complete:
1. User deliberates via `backlogit deliberate` → creates DL artifact in `.backlogit/queue/`
2. Deliberator agent captures findings in stash and creates deliberation artifact
3. `impl-plan` skill / `backlog-harvester.agent.md` converts plan into feature/task backlogit items
4. `build-orchestrator.agent.md` claims ready work and executes build loop
5. `review/SKILL.md` performs multi-persona code review
6. `runtime-verification/SKILL.md` validates runtime surfaces
7. `operational-closure/SKILL.md` captures rollout and monitoring

### Important Context Not to Lose
- autoharness templates are parameterized; `.autoharness/config.yaml` and `workspace-profile.yaml` are artifacts from the installation run, not sources
- The backlogit-specific review, Go engineering, and deliberation workflows are intentionally not derived from generic autoharness templates — they represent hard-won repository expertise
- If future harness tuning is performed, verify that Go-specific and skill-first review expertise is preserved and not overwritten by generic template imports
