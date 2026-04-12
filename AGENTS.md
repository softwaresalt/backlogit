---
title: backlogit Agent Instructions
description: Authoritative rules for all agents operating in the backlogit repository
---

<!-- BACKLOGIT MCP GUIDELINES START -->

<CRITICAL_INSTRUCTION>

## BACKLOGIT WORKFLOW INSTRUCTIONS

This project uses backlogit MCP for all task and project management activities.

**CRITICAL GUIDANCE**

- Call `backlogit_get_metadata_catalog` to load the current backlogit workspace model, supported artifact types, status values, queue and stash conventions, and MCP tool inventory.
- Call `backlogit_export_command_map` when you need a cached command reference in the workspace, for example `.github/instructions/backlogit-command-map.md`.
- Use `backlogit_list_types`, `backlogit_list_templates`, and `backlogit_get_wit_metadata` when you need type-specific field, section, or hierarchy details before creating or updating items.

- **First time working here?** Call `backlogit_get_metadata_catalog` IMMEDIATELY to learn the active workflow surface.
- **Already familiar?** Refresh the catalog before creating items if you are unsure whether config or templates changed.
- **When to read it**: BEFORE creating work items, harvesting stash entries, or when you are unsure how to track work.

These tools cover:
- Search-first workflow support through queue, item, and SQL discovery
- The configured feature, task, and subtask hierarchy
- Template sections and type-specific metadata
- The current backlogit CLI and MCP command surface

You MUST read the metadata catalog or the exported command map before relying on stale workflow assumptions.

</CRITICAL_INSTRUCTION>

<!-- BACKLOGIT MCP GUIDELINES END -->

## Repository operating model

backlogit is both the product and the workflow system used to manage work in
this repository. Agents should treat the repository as a CQRS environment:

* Markdown artifacts in `.backlogit/` are the source of truth for current state.
* `.backlogit/backlogit.db` is an ephemeral query cache.
* JSONL streams capture append-only history and telemetry.
* Durable knowledge belongs in `docs/`, not in backlog artifacts.

For normal workflow operations, prefer backlogit MCP or CLI commands over manual
editing of `.backlogit/` files. Direct edits in `.backlogit/` are reserved for
tool bootstrapping, workspace repair, or repository configuration work that the
tool surface does not yet support.

## Core rules

### Constitution first

The repository constitution in
`.github/instructions/constitution.instructions.md` is authoritative. When a
casual local habit conflicts with the constitution, the constitution wins.

### Go quality gates are mandatory

Production code is Go 1.22+ and must pass the normal gates:

* `go test ./...`
* `go vet ./...`
* `golangci-lint run`
* `gofmt -l .`

All exported packages, types, and functions require GoDoc comments. Prefer
typed structs, explicit error returns, and `errors.Is`/`errors.As` friendly
wrapping over loosely typed plumbing.

### Backlogit is the system of record

Use backlogit-native operations for task state, queue selection, dependency
wiring, comments, checkpoints, and commit traceability. Do not invent sidecar
task trackers or treat free-form markdown notes as equivalent to queue artifacts.

### Durable knowledge belongs in docs

Use the documentation tree intentionally:

* `docs/compound/` for reusable hard-won learnings
* `docs/exec-plans/` for implementation plans
* `docs/decisions/` for durable decisions and investigation outcomes
* `docs/memory/` for session memory and checkpoints
* `docs/closure/` for review, verification, and closure artifacts
* `docs/design-docs/` for graduated architecture and design rationale
* `docs/product-specs/` for product-facing requirements

### Session continuity is explicit

Do not rely on hidden assistant memory. Persist session state through the
`memory` agent into `docs/memory/`, and compact stale context when the tracking
surface grows noisy.

## Repository map

| Topic | Location |
|---|---|
| CLI entrypoint | `cmd/backlogit/main.go` |
| Core business logic | `internal/core/` |
| Workspace config and defaults | `internal/config/` |
| SQLite layer and query gate | `internal/db/` |
| MCP server and tools | `internal/mcp/` |
| Models and frontmatter | `internal/models/` |
| Markdown and migration parsers | `internal/parser/` |
| Contract and integration tests | `tests/` |
| Backlog workspace | `.backlogit/` |
| Durable docs and knowledge | `docs/` |

## Primary workflow

The repository now uses a two-agent path:

* `Stage` owns `STASH -> BACKLOG`. It triages stash entries, routes
  deliberation and plan review, and harvests shipment-aware backlog.
* `Ship` owns `SHIPMENT -> SHIPPED`. It claims shipments and drives harness,
  build, review, CI remediation, and pull request flow until the user approves
  merge.

Lifecycle summary: `STASH -> BACKLOG -> SHIPMENT -> SHIPPED`

Treat the linked agent files and `docs/workflow.md` as the durable source of
detail. Keep this file as the brief map.

## Available agents

| Agent | Purpose |
|---|---|
| `stage` | Primary stash-to-backlog orchestrator |
| `ship` | Primary backlog-to-shipped orchestrator |

### Supporting agents

| Agent | Purpose |
|---|---|
| `go-engineer` | Apply repository-specific Go engineering standards |
| `go-mcp-expert` | Advise on Go MCP server design and implementation |
| `prompt-builder` | Create and refine prompts, agents, instructions, and skills |

### Deprecated agents

Deprecated agents live in `.github/agents/deprecated/` and are excluded from
the default Copilot agent picker. They remain available for reference or
targeted invocation when needed.

| Agent | Purpose | Superseded by |
|---|---|---|
| `backlog-harvester` | Decompose plans into backlogit work items | `stage` + `harvest` skill |
| `build-orchestrator` | Claim and execute ready feature work | `ship` + `build-feature` skill |
| `deliberator` | Route idea work into deliberation | `stage` + `deliberate` / `spike` skills |
| `doc-ops` | Documentation quality assurance | `ship` post-merge closure protocol |
| `harness-architect` | Create failing test harnesses | `ship` + `harness-architect` skill |
| `memory` | Session context persistence | Stage and Ship session continuity protocols |
| `pr-review` | Manage PR review lifecycle | `ship` + `pr-lifecycle` skill |

## Available skills

| Skill | Purpose |
|---|---|
| `deliberate` | Collaborative idea shaping |
| `spike` | Time-boxed technical investigation |
| `impl-plan` | Plan generation |
| `plan-harden` | Risk-triggered plan reinforcement before review |
| `plan-review` | Multi-persona plan gate |
| `harvest` | Decompose reviewed plans into backlogit work items |
| `harness-architect` | Scaffold compilable but failing test harnesses |
| `build-feature` | Test-driven implementation loop |
| `review` | Structured code review |
| `fix-ci` | CI and review-comment remediation |
| `pr-lifecycle` | PR creation, Copilot comment handling, and merge readiness |
| `compound` | Durable learning capture |
| `compound-refresh` | Institutional knowledge maintenance |
| `compact-context` | Tracking and memory compaction |
| `runtime-verification` | Post-build runtime validation |
| `operational-closure` | Closure and rollout capture |
| `safety-modes` | Elevated-risk workflow controls |

## Default workflow pipeline

```text
Stash or idea
	-> Stage
	   -> deliberate or spike
	   -> impl-plan
	   -> plan-harden (when needed)
	   -> plan-review
	   -> harvest
	-> ready backlog
	-> shipment assembly
	-> Ship
	   -> harness-architect
	   -> build-feature
	   -> review
	   -> fix-ci
	   -> pr-lifecycle
	-> user-approved merge
	-> SHIPPED
```

## Quality gates

Run the gates in this order when code changes:

```text
# Gate 1 — tests
go test ./...

# Gate 2 — vet
go vet ./...

# Gate 3 — lint
golangci-lint run

# Gate 4 — format check
gofmt -l .
```

## Where to look next

| Need | Read this |
|---|---|
| Workflow rules | `.github/instructions/backlogit.instructions.md` |
| Generic backlog abstraction | `.github/instructions/backlog-integration.instructions.md` |
| Go coding conventions | `.github/instructions/go.instructions.md` |
| Go MCP conventions | `.github/instructions/go-mcp-server.instructions.md` |
| Strict-safety action contract | `.github/instructions/strict-safety.instructions.md` |
| CI security conventions | `.github/instructions/ci-security.instructions.md` |
| Remote operator integration | `.github/instructions/agent-intercom.instructions.md` |
| Engram-first search | `.github/instructions/agent-engram.instructions.md` |
| Markdown and prompt authoring | `.github/instructions/markdown.instructions.md`, `.github/instructions/prompt-builder.instructions.md` |
| Architecture context | `docs/research/Backlogit-Architecture-Design.md`, `docs/design-docs/` |
| Plans and reviews | `docs/exec-plans/`, `docs/reviews/` |
| Durable learnings | `docs/compound/` |

## Session completion

Before ending a meaningful work session:

* follow the Session Continuity protocol defined in your agent file (stage or
  ship) to persist memory, capture learnings, and compact tracking context
* update backlogit task state through the tool surface
* leave the branch and working tree in a reviewable state

