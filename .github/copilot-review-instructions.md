---
title: Copilot review instructions
description: Repository-specific guidance for GitHub Copilot code review on the backlogit project
author: backlogit contributors
ms.date: 2026-04-04
ms.topic: reference
keywords:
  - github copilot
  - code review
  - backlogit
  - mcp
  - go
estimated_reading_time: 4
---

## Purpose

Use these instructions when GitHub Copilot reviews pull requests for this repository.
Prioritize correctness, data integrity, security, and architectural fit over style-only
feedback.

## Review priorities

Review for defects that could break backlogit behavior or damage workspace state:

* Functional bugs and regressions
* Security issues and workspace-boundary violations
* Data corruption or data-loss risks
* MCP contract breaks and agent workflow regressions
* Concurrency, locking, or append-safety problems
* Migration, routing, queue, stash, dependency, and archive regressions

Do not spend review budget on formatting, naming preferences, or low-value style nits
unless they hide a real bug.

## Recommended review process

When the review system supports it, use multiple adversarial passes and merge them
conservatively:

* Run at least one runtime and data-integrity pass over `internal/core/`,
  `internal/db/`, `internal/mcp/`, and `internal/events/`
* Run a separate control-plane pass over `.github/agents/**`, `.github/skills/**`,
  and `.github/**/*.instructions.md`
* Run a branch-hygiene pass focused on generated files, tracked ephemeral files,
  queue churn, and scope pollution
* Prefer different models or reviewer personas for each pass when available
* Merge findings with a highest-severity-wins rule when reviewers disagree
* Do not suppress a concrete defect just because another pass did not report it

## Project architecture to protect

backlogit is a file-backed agile workspace with a strict CQRS-style split:

* Markdown artifacts with YAML frontmatter are the source of truth
* `.backlogit/backlogit.db` is an ephemeral SQLite cache and must remain rebuildable
  from Markdown artifacts
* `.backlogit/logs/<item-id>.jsonl` and `.backlogit/telemetry.jsonl` are append-only
  event and telemetry streams

Flag any change that weakens these boundaries. In particular:

* Do not allow SQLite or JSONL to become the canonical source of current artifact state
* Do not allow historical logs, comments, or agent traces to be written back into
  artifact Markdown files
* Do not allow rehydration assumptions to diverge from what is stored on disk in the
  Markdown source of truth

## Workspace safety and filesystem review

Treat workspace containment as critical.

Flag changes that:

* Write outside the intended workspace root or `.backlogit` storage boundary
* Build paths with unsafe string concatenation instead of canonical path handling
* Fail to reject path traversal or untrusted relative paths
* Risk partial writes, corrupted files, or unstable serialized output

This repository is intentionally Git-friendly. Artifact files should stay readable,
deterministic, and safe to merge.

## MCP review guidance

This repository implements an MCP server using `github.com/mark3labs/mcp-go`.
Review MCP-related changes with extra care.

Flag changes that:

* Break the fixed, discoverable MCP tool surface
* Hide tools when the workspace is uninitialized instead of returning descriptive
  errors
* Change tool inputs or outputs without corresponding contract awareness
* Return overly broad, noisy, or unstructured responses that work against agent
  context efficiency
* Bypass the established pattern of validating inputs, routing through core/db logic,
  and returning structured results

## Control-plane review guidance

Treat agent, skill, and instruction files as executable policy.

Flag changes that:

* Introduce contradictions between phase descriptions, mode tables, and later steps
* Claim read-only or report-only behavior while still creating artifacts, queue items,
  or other side effects
* Grant write-capable tools to agents that should be read-only researchers or reviewers
* Write malformed ad hoc files into `.backlogit/queue` instead of real backlogit
  artifacts when backlogit storage is used
* Break commit-tracking discipline by linking a commit to an entire feature set when it
  only affects a subset of items

## Database and query safety

SQLite is the query engine, not the durable system of record.

Flag changes that:

* Weaken the read-only gate around `backlogit_query_sql`
* Permit non-`SELECT` behavior through review-time or runtime gaps
* Bypass parameterized query patterns in the database layer
* Break or desynchronize schema, FTS indexes, or maintenance triggers
* Risk inconsistency across items, dependencies, stash entries, queue state, commit
  links, or item-log indexing

## Go-specific review guidance

Prefer comments only when there is a real correctness or maintainability issue.
Pay close attention to these repository conventions:

* No `panic()` in library code
* No ignored errors or silent success-shaped fallbacks
* Use `log/slog` for structured logging instead of `fmt.Println` or ad hoc logging
* Keep strong types at package boundaries; avoid unnecessary `any`
* At `map[string]any` or YAML-decoded boundaries, prefer explicit type assertions or
  conversion helpers over `fmt.Sprintf("%v", value)` coercion
* Pass `context.Context` to I/O or blocking work
* Avoid global mutable state
* Prefer the standard library unless an added dependency is clearly justified

## Artifact, schema, and branch hygiene

Flag changes that leave the branch in a state that should not ship:

* Tracked ephemeral SQLite sidecars such as `.backlogit/backlogit.db-shm` or
  `.backlogit/backlogit.db-wal`
* Generated PR artifacts such as `pr-reference.xml` that should be cleaned up after use
* Unrelated future-scope queue items, archive churn, or other broad diff noise that is
  not required for the PR's purpose
* Durable-memory documents that contain session telemetry, model details, or other
  short-lived operational data instead of long-lived decisions and outcomes

When a PR adds or changes a backlogit artifact type, require consistency across the
whole surface:

* `internal/config/defaults.go`
* checked-in `.backlogit/config.yaml`, `.backlogit/header-def.yaml`, and templates
* queue hierarchy and allowed-child behavior
* archive and unarchive path handling when filenames diverge from stable IDs
* metadata and discovery tests

## Areas that deserve extra scrutiny

Be especially careful when a pull request touches:

* `internal/mcp/`
* `internal/db/`
* `internal/core/`
* `internal/events/`
* Rehydration and migration flows
* Queue, stash, archive, and dependency workflows
* Templates and section-aware tool behavior
* Markdown/frontmatter serialization

These areas carry a high risk of corrupting user workspaces or breaking agent-facing
workflows in subtle ways.

## Tests and verification expectations

Expect meaningful coverage when behavior changes.

Ask for tests or call out missing coverage when a change affects:

* MCP tool schemas, outputs, or error behavior
* Rehydration or index rebuild behavior
* Workspace routing or archive behavior
* SQL query gating
* Event logging or telemetry
* Markdown/frontmatter parsing or serialization
* Queue, stash, dependency, or migration logic

The repository's core validation bar is:

* `go test ./...`
* `go vet ./...`
* `golangci-lint run`

## Good review comment style

Prefer comments that identify a concrete failure mode or invariant violation.

Strong comments usually explain:

1. What breaks
2. When it breaks
3. Why that matters to backlogit's architecture, user data, or MCP contracts

Weak comments focus only on taste, formatting, or hypothetical cleanup unrelated to a
real defect.
