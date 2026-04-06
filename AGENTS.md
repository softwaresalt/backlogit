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
* `.backlogit/index.db` is an ephemeral query cache.
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

## Available agents

| Agent | Purpose |
|---|---|
| `backlog-harvester` | Decompose plans and deliberation artifacts into backlogit work items |
| `build-orchestrator` | Claim ready work under a feature and drive the build loop |
| `deliberator` | Route idea work between deliberate and spike workflows |
| `doc-ops` | Maintain durable docs and reduce documentation drift |
| `go-engineer` | Apply repository-specific Go engineering standards |
| `go-mcp-expert` | Advise on Go MCP server design and implementation |
| `harness-architect` | Create compilable but failing harnesses for selected work |
| `memory` | Persist and restore session context via `docs/memory/` |
| `pr-review` | Manage PR review lifecycle and handoff |
| `prompt-builder` | Create and refine prompts, agents, instructions, and skills |

## Available skills

| Skill | Purpose |
|---|---|
| `deliberate` | Collaborative idea shaping |
| `spike` | Time-boxed technical investigation |
| `impl-plan` | Plan generation |
| `plan-review` | Multi-persona plan gate |
| `build-feature` | Test-driven implementation loop |
| `review` | Structured code review |
| `fix-ci` | CI and review-comment remediation |
| `compound` | Durable learning capture |
| `compact-context` | Tracking and memory compaction |
| `runtime-verification` | Post-build runtime validation |
| `operational-closure` | Closure and rollout capture |
| `safety-modes` | Elevated-risk workflow controls |

## Default workflow pipeline

```text
Stash or idea
	-> deliberate skill or spike skill
	-> impl-plan skill
	-> plan-review skill
	-> backlog-harvester agent
	-> harness-architect agent
	-> build-orchestrator agent
	-> review skill or pr-review agent
	-> fix-ci skill
	-> runtime-verification skill
	-> operational-closure skill
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
| Markdown and prompt authoring | `.github/instructions/markdown.instructions.md`, `.github/instructions/prompt-builder.instructions.md` |
| Architecture context | `docs/research/Backlogit-Architecture-Design.md`, `docs/design-docs/` |
| Plans and reviews | `docs/exec-plans/`, `docs/reviews/` |
| Durable learnings | `docs/compound/` |

## Session completion

Before ending a meaningful work session:

* persist memory to `docs/memory/`
* update backlogit task state through the tool surface
* capture compound learnings when the work uncovered reusable lessons
* leave the branch and working tree in a reviewable state

