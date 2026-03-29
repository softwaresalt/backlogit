---
name: review
description: "Structured code review using tiered persona agents, confidence-gated findings, and a merge/dedup pipeline. Use when reviewing code changes before creating a PR, as a build gate, or for standalone review."
argument-hint: "[mode:autofix|mode:report-only] [branch name or file paths]"
---

# Code Review

Reviews code changes using dynamically selected reviewer personas. Spawns persona subagents that return structured JSON findings, then merges and deduplicates into a unified report.

## Agent-Intercom Communication (NON-NEGOTIABLE)

Call `ping` at session start. If agent-intercom is reachable, broadcast at every step. If unreachable, warn the user that operator visibility is degraded.

| Event | Level | Message prefix |
|---|---|---|
| Review start | info | `[REVIEW] Starting {mode} review of {scope}` |
| Diff analyzed | info | `[REVIEW] Analyzed diff: {file_count} files, {line_count} lines changed` |
| Persona routing | info | `[REVIEW] Routing: {always_on_count} always-on + {conditional_count} conditional personas` |
| Persona spawned | info | `[SPAWN] {persona_name} for code review` |
| Persona returned | info | `[RETURN] {persona_name}: {finding_count} findings` |
| Merge complete | info | `[REVIEW] Merged: {total} findings ({p0} P0, {p1} P1, {p2} P2, {p3} P3)` |
| Autofix applied | info | `[REVIEW] Applied safe_auto fix: {finding_summary}` |
| Review written | success | `[REVIEW] Review artifact: {file_path}` |
| Waiting for input | warning | `[WAIT] Blocked on user decision` |
| Review complete | success | `[REVIEW] Complete: {summary}` |

## Subagent Depth Constraint

This skill spawns reviewer subagents. Those subagents are leaf executors and MUST NOT spawn their own subagents. Maximum depth: review skill -> persona subagent (1 hop).

## Mode Detection

Check arguments for `mode:autofix` or `mode:report-only`. Strip the mode token before interpreting remaining arguments.

| Mode | When | Behavior |
|---|---|---|
| **Interactive** (default) | No mode token | Review, present findings, ask for decisions |
| **Autofix** | `mode:autofix` | No user interaction. Apply `safe_auto` fixes only, write artifact, emit residual work |
| **Report-only** | `mode:report-only` | Read-only. Report findings, no edits, no artifacts beyond review doc |

### Autofix mode rules

- Skip all user questions
- Apply only `safe_auto` findings
- Leave `gated_auto`, `manual`, and `advisory` findings unresolved
- Write a review artifact to `.backlog/reviews/`
- Create backlog tasks for unresolved actionable findings
- Never commit, push, or create a PR

### Report-only mode rules

- Skip all user questions
- Never edit files
- Return structured findings to caller
- Safe for the build orchestrator to invoke during the build loop

## Severity Scale

| Level | Meaning | Build gate action |
|---|---|---|
| **P0** | Critical breakage, exploitable vulnerability, data corruption | Block commit |
| **P1** | High-impact defect in normal usage, breaking contract | Block commit |
| **P2** | Moderate issue (edge case, perf, maintainability) | Record as backlog task |
| **P3** | Low-impact, minor improvement | User's discretion |

## Action Routing

| Class | Default owner | Meaning |
|---|---|---|
| `safe_auto` | Review skill (autofix mode) | Deterministic local fix |
| `gated_auto` | agent-intercom approval | Fix exists but changes behavior/contracts |
| `manual` | Backlog task | Actionable work requiring human judgment |
| `advisory` | Informational | Learnings, rollout notes, residual risk |

Routing rules:

- Choose the more conservative route on disagreement between personas
- Only `safe_auto` findings enter the autofix queue
- `requires_verification: true` means a fix needs tests or re-review

## Reviewer Personas

### Always-On (every review)

| Agent | Focus |
|---|---|
| **Python Quality Reviewer** | Type safety, error handling, `raise` patterns, import hygiene, PEP 8/257 compliance |
| **Constitution Reviewer** | Project coding standards compliance |
| **Learnings Researcher** | Search `.backlog/compound/` for related past issues |

### Conditional (based on changed files)

Use a different model from the caller when available to force genuine diversity
of critique. Cross-model is preferred but not blocking; if unavailable, use the
caller's model.

| Agent | Select when diff touches | Suggested Model |
|---|---|---|
| **MCP Protocol Reviewer** | `src/backlogit/mcp/`, MCP-related code | GPT-5.4 or Gemini |
| **SQLite Reviewer** | `src/backlogit/db/`, database queries, schema files | GPT-5.4 or Gemini |
| **Markdown/YAML Reviewer** | `src/backlogit/parser/`, frontmatter handling, YAML schemas | GPT-5.4 or Gemini |

## Workflow

### Step 1: Determine Review Scope

1. Identify changed files from git diff, explicit file list, or caller-provided scope
2. For branch-based review: `git diff --stat origin/main..HEAD`
3. Broadcast the diff analysis

### Step 2: Route Personas

1. Always-on: spawn Python Quality Reviewer, Constitution Reviewer, Learnings Researcher
2. Conditional: analyze changed file paths and content patterns to select additional personas
3. Broadcast the routing decision with persona count

### Step 3: Spawn Persona Subagents

Spawn all selected personas. Each receives:

- The list of changed files with line ranges
- The diff content relevant to their domain
- Instructions to return structured JSON findings
- Codebase search directive (use grep/glob for context)

Broadcast each spawn.

### Step 4: Collect and Merge Findings

As each persona returns:

1. Broadcast the return with finding count
2. Collect all findings
3. Deduplicate: merge findings that identify the same issue
4. Assign final severity (more conservative on disagreement)
5. Assign final action routing

### Step 5: Apply Actions (mode-dependent)

**Interactive mode:**

1. Present findings by severity (P0 first)
2. For each finding, present recommendation and ask for decision
3. Apply approved fixes

**Autofix mode:**

1. Apply all `safe_auto` findings automatically
2. Create backlog tasks for `manual` findings
3. Write review artifact

**Report-only mode:**

1. Return structured findings to caller
2. No edits, no side effects beyond the review artifact

### Step 6: Write Review Artifact

Write to `.backlog/reviews/{YYYY-MM-DD}-{slug}-review.md`

```markdown
---
title: "Code Review: {scope_description}"
date: YYYY-MM-DD
mode: interactive|autofix|report-only
gate: pass|fail
reviewers: [{persona_list}]
---

# Code Review: {scope_description}

## Summary

| Severity | Count | Action |
|---|---|---|
| P0 | {n} | {blocked/fixed/deferred} |
| P1 | {n} | {blocked/fixed/deferred} |
| P2 | {n} | {backlog tasks created} |
| P3 | {n} | {advisory} |

## Findings

{Grouped by file, ordered by severity}

## Learnings Applied

{Relevant solutions from .backlog/compound/ that informed this review}

## Residual Work

{Findings not resolved in this review session}
```

Broadcast the file path when written.
