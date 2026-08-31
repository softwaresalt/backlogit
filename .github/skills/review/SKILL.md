---
name: review
description: "Structured code review using tiered persona subagents, confidence-gated findings, and a merge/dedup pipeline. Use when reviewing code changes before creating a PR, as a build gate, or for standalone review."
argument-hint: "[mode:autofix|mode:report-only] [branch name or file paths]"
---

# Code Review

Reviews code changes using dynamically selected reviewer personas. Spawns persona subagents that return structured findings, then merges and deduplicates into a unified report.

## Operator Communication (NON-NEGOTIABLE)

Operator visibility is mandatory; the transport is conditional.

**When the `agent-intercom` capability pack is installed**, call `ping` at session
start and broadcast every event in the table below. If the pack is installed but
unreachable, warn the user that operator visibility is degraded.

**When the `agent-intercom` capability pack is NOT installed** (the current state
of this workspace), do not call `ping` and do not attempt a broadcast. Emit the
same events as self-contained entries in the local session output — same prefixes,
same content — so the review trail stays legible without a remote channel. Any
step that would otherwise wait on an intercom approval or clarification flow uses
the local strict-safety operator-approval path in
`.github/instructions/strict-safety.instructions.md` instead. Missing intercom
never implies approval: if an approval signal is absent or ambiguous, halt.

Throughout this skill, "broadcast" means "broadcast when agent-intercom is
installed, otherwise record to local session output."

When the `strict-safety` capability pack is installed, also follow
`.github/instructions/strict-safety.instructions.md`: for high-risk diffs, call
out the `ProposedAction`, `ActionRisk`, approval, and rollback gaps that should
be visible before merge or deployment.

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

This skill spawns reviewer subagents. Those subagents are leaf executors and MUST NOT spawn their own subagents. Maximum depth: review skill → persona subagent (1 hop).

## Mode Detection

Check arguments for `mode:autofix` or `mode:report-only`. Strip the mode token before interpreting remaining arguments.

| Mode | When | Behavior |
|---|---|---|
| **Interactive** (default) | No mode token | Review, present findings, ask for decisions |
| **Autofix** | `mode:autofix` | No user interaction. Apply `safe_auto` fixes only, write artifact, emit residual work |
| **Report-only** | `mode:report-only` | Read-only. Report findings with no edits, no artifacts, no follow-up item creation |

### Autofix mode rules

- Skip all user questions
- Apply only `safe_auto` findings
- Leave `gated_auto`, `manual`, and `advisory` findings unresolved
- Write a review artifact to `docs/closure/`
- Create backlog follow-up items for unresolved actionable findings
- Never commit, push, or create a PR

### Report-only mode rules

- Skip all user questions
- Never edit files
- Return structured findings to caller
- Do not write a review artifact
- Do not create backlog follow-up items
- Safe for the ship agent to invoke during the build loop

## Severity Scale

| Level | Meaning | Build gate action |
|---|---|---|
| **P0** | Critical breakage, exploitable vulnerability, data corruption | Block commit |
| **P1** | High-impact defect in normal usage, breaking contract | Block commit |
| **P2** | Moderate issue (edge case, perf, maintainability) | Record as backlog follow-up item |
| **P3** | Low-impact, minor improvement | User's discretion |

## Action Routing

| Class | Default owner | Meaning |
|---|---|---|
| `safe_auto` | Review skill (autofix mode) | Deterministic local fix |
| `gated_auto` | Operator approval — agent-intercom approval flow when that pack is installed, otherwise the local strict-safety operator-approval path | Fix exists but changes behavior/contracts |
| `manual` | Backlog follow-up item | Actionable work requiring human judgment |
| `advisory` | Informational | Learnings, rollout notes, residual risk |

Routing rules:

- Choose the more conservative route on disagreement between personas
- Only `safe_auto` findings enter the autofix queue
- `requires_verification: true` means a fix needs tests or re-review

## Local Review Readiness

The review produces a readiness outcome that Ship and pr-lifecycle consume before
PR presentation (P-014):

| Outcome | Meaning | Gate |
|---|---|---|
| `READY` | Zero unresolved P0/P1 findings and no required follow-up items | PR may be prepared |
| `READY_WITH_FOLLOWUPS` | Zero unresolved P0/P1 findings, but one or more P2/P3 findings need explicit follow-up tracking or residual-risk notes | PR may be prepared only with follow-up handling recorded |
| `BLOCKED` | One or more unresolved P0/P1 findings remain | Do not create or present a PR |

The readiness summary must include:

* reviewed HEAD SHA or equivalent diff identity
* counts for P0, P1, P2, and P3 findings
* follow-up item IDs or residual-risk notes when outcome is `READY_WITH_FOLLOWUPS`
* whether runtime verification follow-up is required

When `DARK_MODE_ACTIVE` is present under P-017, this local review record is the
authoritative merge-readiness signal:

* unresolved P0/P1 findings always produce `BLOCKED`
* `READY_WITH_FOLLOWUPS` must include concrete follow-up item IDs or explicit
  residual-risk notes
* hosted Copilot/GitHub review cannot replace this local review record
* advisory shadow-review comments are follow-ups by default unless the operator
  or policy explicitly elevates them to blocking status
* the reviewed HEAD SHA or equivalent diff identity must be current when the PR
  readiness block is written

## Reviewer Personas

### Always-On (every review)

| Persona Subagent | Focus |
|---|---|
| **Constitution Reviewer** | Constitutional compliance |
| **Go Reviewer** | Language-specific safety and correctness |
| **Learnings Researcher** | Search compound library for related past issues |

### Conditional (based on changed files)

Use a different model from the caller when available to force genuine diversity of critique. Cross-model is preferred but not blocking.

| Persona Subagent | Select when diff touches | Suggested Model |
|---|---|---|
| **Architecture Strategist** | Module boundaries, new abstractions, dependency changes | Different from caller |
| **Concurrency Reviewer** | Concurrent/async patterns | Different from caller |
| **Scope Boundary Auditor** | Changes spanning multiple domains or exceeding expected scope | Different from caller |
| **Agent-Native Parity Reviewer** | MCP SDKs, tool handlers, agent-exposed actions, or user/agent parity-critical flows | Different from caller |
| **Security Reviewer** | Auth middleware, public endpoints, input handling, permission checks, secret management | Different from caller |
| **Template Integrity Reviewer** | `.tmpl` files, Markdown workflow assets, generated artifact references, or policy/instruction surfaces | Different from caller |
| **Schema-CLI-Docs Coupling Reviewer** | Cross-domain diffs spanning schemas, CLI verification logic, install/tune skills, and operator docs | Different from caller |

## Workflow

### Step 1: Determine Review Scope

1. Identify changed files from git diff, explicit file list, or caller-provided scope
2. Categorize each file by type and domain
3. Identify which instruction files apply (via `applyTo` patterns)
4. Broadcast the diff analysis

### Step 2: Route Personas

1. Always-on: spawn Constitution Reviewer, Go Reviewer, Learnings Researcher
2. Conditional: analyze changed file paths, content patterns, and workspace agent-native signals to select additional personas:
   * Select **Security Reviewer** (`security-reviewer.agent.md`) when the diff touches: authentication or authorization code, public endpoint handlers, user input processing, permission or role checks, secret or credential management, or files matching `- Path traversal and workspace escape attempts
- SQL injection in query parameters
- Unsafe file operations outside workspace root
- Secret or credential exposure in committed files
- Unvalidated MCP tool inputs
- Race conditions in concurrent file access`
   * Select **Template Integrity Reviewer** (`template-integrity-reviewer.agent.md`) when the diff touches template files, Markdown harness artifacts, review/policy/instruction assets, or generated-artifact reference tables
   * Select **Schema-CLI-Docs Coupling Reviewer** (`schema-cli-docs-coupling-reviewer.agent.md`) when the diff spans schema files, `src/` verification logic, install/tune skills, or operator-facing documentation in the same change set
3. Broadcast the routing decision with persona count

### Step 2.5: Coordinator Structural Discovery (REQUIRED before spawning)

Structural discovery is a **coordinator responsibility**, not a leaf-persona one.
Before any persona subagent is spawned, this skill's coordinator MUST perform the
agent-engram structural discovery for the review scope and MUST pass the resulting
context into every persona payload. This keeps the capability-pack-enforcement
classification (see `.github/instructions/capability-pack-enforcement.instructions.md`)
satisfied at the one hop that actually holds `engram/*`, instead of pushing an
unsatisfiable routing obligation onto leaf personas that were never granted those
tools.

1. Verify the engram workspace binding once for the review
   (`get_workspace_status`; `sync_workspace` only if the index is stale after
   out-of-band edits). Record the binding/freshness state.
2. For the changed-file set, run the structural queries the personas would
   otherwise need:
   * `list_symbols` — the symbols defined or modified in each changed file
   * `map_code` — callers/callees and local graph context for each changed
     or newly introduced symbol
   * `impact_analysis` — blast radius for each modified exported or
     cross-package symbol
   * `query_graph` / `query_graph_neighborhood` — typed-edge traversal when a
     specific relationship question remains open
3. Assemble a **structural context block** per persona domain containing: the
   symbol inventory, the caller/callee map, the impact/blast-radius set, the
   relevant graph neighborhoods, and the binding/freshness state.
4. If engram is unavailable, unbound, or degraded, the coordinator — not the
   personas — performs the documented fallback (`list_symbols` + `map_code` +
   `impact_analysis` where partially available, then grep/glob) and marks the
   structural context block `degraded: true` with the reason. Personas receive the
   degraded block and treat its coverage as incomplete rather than re-deriving it.
5. Broadcast structural discovery completion, including whether the context is
   complete or degraded.

### Step 3: Spawn Persona Subagents

Spawn all selected personas. Each receives:

- The list of changed files with line ranges
- The diff content relevant to their domain
- The **structural context block** produced by the coordinator in Step 2.5
  (symbol inventory, caller/callee map, impact/blast-radius set, graph
  neighborhoods, and the `degraded` flag with its reason when set)
- Instructions to return structured findings
- Codebase search directive: classify each search per
  `.github/instructions/capability-pack-enforcement.instructions.md` before
  searching. Structural questions — callers/callees, impact analysis, symbol
  lookup, blast radius, inheritance, implementations, implementers, and
  "where/how is this implemented?" — are **answered from the coordinator-supplied
  structural context block**, which is the persona's routed engram result. A
  persona MUST NOT re-issue structural discovery it was already given, and MUST
  NOT treat "I do not hold `engram/*`" as license to silently fall back to
  grep/ripgrep for a structural question: the routing obligation was already
  discharged one hop up. Personas use only the tools they were actually granted.
  Direct grep/glob remains available under the documented direct-tool exemptions
  (literal-text or regex lookups, a known exact file path needing line-level
  confirmation, or a trivial single-file lookup where indexed search adds only
  latency), and as the fallback when the supplied structural context is marked
  `degraded: true` or is demonstrably insufficient for the question at hand — in
  which case the persona states the gap explicitly in its findings rather than
  silently substituting a text search. Coordinators holding `engram/*` (this skill
  and the adversarial-review agent) still route structural questions through the
  agent-engram code-graph tools (`list_symbols`, `map_code`, `impact_analysis`,
  `query_graph`, `query_graph_neighborhood`) before grep/ripgrep or raw file
  reads. See `.github/instructions/agent-engram.instructions.md`.

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

1. Present findings grouped by severity (P0 first)
2. For each P0/P1, ask the user to accept, modify, or reject the recommendation
3. Apply approved fixes

**Autofix mode:**

1. Apply all `safe_auto` findings automatically
2. Create backlog follow-up items for unresolved actionable findings
3. Write review artifact to `docs/closure/`

**Report-only mode:**

1. Return structured findings to caller
2. No side effects: no edits, no review artifact, no follow-up items

When the diff changes runtime surfaces, include an explicit recommendation for whether follow-up runtime verification is required and which mode (`manual`, `api`, `browser`) is appropriate.

When the diff includes destructive potential, contract changes, migrations,
security-sensitive edits, or other high-blast-radius work, include an explicit
recommendation for whether strict-safety action classification or approval
follow-up is required before merge or deployment.

When the `adversarial-review` capability pack is installed and this review surfaces 3 or more
P0/P1 findings, recommend escalation to the `adversarial-review` agent for multi-model consensus
validation before blocking the build.

## Quality Criteria

* Every changed file is reviewed by at least the always-on personas
* All P0 findings are addressed before the review is marked complete
* P1 findings require explicit acknowledgment (fix or defer with rationale)
* The review report accurately reflects all findings and their resolution status


## Model Routing

This skill operates at **Tier 2 (Standard)** — review coordination and finding assembly.

Generated by autoharness | Template: review/SKILL.md.tmpl
