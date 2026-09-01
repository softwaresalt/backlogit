---
name: doc-review
description: "Structured documentation review skill that identifies broken cross-references, stale content, missing frontmatter, markdown structure violations, and template variable drift in installed harness docs. Supports an alternate model provider for the review pass."
argument-hint: "[mode:autofix|mode:report-only] [path or glob]"
---

# Documentation Review

Reviews installed harness documentation and workspace docs for quality issues.
Identifies broken cross-references, stale content, missing or malformed
frontmatter, markdown structure violations (MD001/MD025/MD041), and unresolved
template variable drift (double-brace placeholders remaining in installed
output). Produces structured findings with severity tiers and action classes
compatible with the standard review persona routing model.

## When to Use

* After installing or tuning the harness — verify no template variables remain
  unresolved and all cross-references are intact.
* As part of the ship agent's post-merge closure documentation gardening pass.
* On-demand when doc quality has drifted or a structural refactor has been
  applied.
* Before a release when documentation accuracy is a gate condition.

## Alternate Model Support

When `google` and `gemini-3.1-pro-preview` are
configured, the review pass uses those provider and family values instead of
the default Tier 2 routing. This allows the documentation review to use Gemini,
a different Anthropic family, or any provider registered in the workspace model
configuration — independent of the standard tier routing set.

When these variables resolve to empty strings (no alternate model configured),
the skill falls back to Tier 2 defaults.

## Agent-Intercom Communication

**When the `agent-intercom` capability pack is installed**, call `ping` at session
start and broadcast every event in the table below. If the pack is installed but
unreachable, warn the user that operator visibility is degraded.

**When the `agent-intercom` capability pack is NOT installed** (the current state
of this workspace), do not call `ping` and do not attempt a broadcast. Emit the
same events as self-contained entries in the local session output — same prefixes,
same content — so the doc-review trail stays legible without a remote channel. An
intentionally absent optional pack is **not** a degraded state and MUST NOT be
reported as one. Any step that would otherwise wait on an intercom approval or
clarification flow uses the local strict-safety operator-approval path in
`.github/instructions/strict-safety.instructions.md` instead. Missing intercom
never implies approval: if an approval signal is absent or ambiguous, halt.

Throughout this skill, "broadcast" means "broadcast when agent-intercom is
installed, otherwise record to local session output."

| Event | Level | Message prefix |
|---|---|---|
| Doc review start | info | `[DOC-REVIEW] Starting {mode} review of {scope}` |
| Scope analyzed | info | `[DOC-REVIEW] Scope: {file_count} docs, {check_count} checks` |
| Check pass | info | `[DOC-REVIEW] Check passed: {check_name}` |
| Check finding | warning | `[DOC-REVIEW] Finding ({severity}): {summary} in {file}` |
| Autofix applied | info | `[DOC-REVIEW] Applied safe_auto fix: {summary}` |
| Review written | success | `[DOC-REVIEW] Artifact: {file_path}` |
| Review complete | success | `[DOC-REVIEW] Complete: {p0} P0, {p1} P1, {p2} P2, {p3} P3` |

## Subagent Depth Constraint

This skill is a leaf executor. It MUST NOT spawn its own subagents.
Maximum depth: caller → doc-review skill (1 hop).

## Mode Detection

Check arguments for `mode:autofix` or `mode:report-only`.

| Mode | Behavior |
|---|---|
| **Interactive** (default) | Review, present findings, ask for decisions |
| **Autofix** | Apply `safe_auto` fixes only; write artifact; emit residual work |
| **Report-only** | Read-only; return structured findings; no side effects |

## Severity Scale

| Level | Meaning | Gate action |
|---|---|---|
| **P0** | Unresolved double-brace template placeholders in installed output; broken required cross-reference | Block merge/release |
| **P1** | Missing required frontmatter; MD001/MD025/MD041 violation causing structural failure | Block merge |
| **P2** | Stale content (references removed files/sections); soft cross-reference gaps | Backlog follow-up |
| **P3** | Minor style, wording, or organizational improvement | Advisory |

## Action Classes

| Class | Default owner | Meaning |
|---|---|---|
| `safe_auto` | Doc-review skill (autofix mode) | Deterministic local fix |
| `gated_auto` | Operator approval via the local strict-safety decision gate; route through `agent-intercom` only when that pack is installed | Fix changes meaning or structure |
| `manual` | Backlog follow-up item | Requires human judgment |
| `advisory` | Informational | Suggestion; no defect |

## Check Suite

### Check 1 — Template Variable Drift (P0)

Scan all Markdown and YAML files in scope for unresolved template variable
patterns matching `\{\{[A-Z_][A-Z0-9_]*\}\}`. Any match in installed output
files (not `.tmpl` source files) is a P0 finding.

```text
Pattern: \{\{[A-Z_][A-Z0-9_]*\}\}
Scope:    installed harness files (exclude *.tmpl, exclude .backlogit/)
Severity: P0
Action:   manual — the installer must re-run or the variable must be resolved
```

### Check 2 — YAML Frontmatter Validity (P1)

For each file with a `---` frontmatter block:

1. Extract the YAML block between the first and second `---` delimiters.
2. Parse the YAML. Any parse error is a P1 finding.
3. Verify required keys are present per file type:
   * Agent files: `name`, `description`
   * Skill files: `description` (required). `name` is **optional** — this
     workspace's skill schema is description-only, and 14 of the 25 installed
     skills (`brainstorm`, `build-feature`, `operational-closure`,
     `compact-context`, `compound`, `compound-refresh`, `deliberate`,
     `file-lock`, `fix-ci`, `impl-plan`, `plan-harden`, `runtime-verification`,
     `safety-modes`, `skill-search`) deliberately omit it. Do **not** raise a
     finding for a missing skill `name`; requiring it would flag every one of
     those valid skills as P1.
   * Instruction files: `description`
   * Template files: at minimum `description` or `name`

```text
Action class: gated_auto (reformatting) or manual (missing keys)
```

### Check 3 — Markdown Heading Hierarchy (P1)

Apply markdownlint-compatible rules:

* **MD001** — Heading levels increment by one (no skips from H1 → H3).
* **MD025** — Only one top-level H1 heading per document.
* **MD041** — First line of a Markdown file must be a top-level heading
  (unless the file begins with YAML frontmatter, in which case the first
  heading after the frontmatter close must be H1 for content files).

```text
Action class: safe_auto for level-skip corrections; gated_auto for
multi-H1 (structural ambiguity); manual when context is unclear
```

### Check 4 — Cross-Reference Integrity (P1/P2)

Scan all Markdown links, `applyTo` glob values, and explicit file path
references in documentation:

**Disabled-capability-pack exemption (evaluate first).** Before raising any
missing-target finding, check whether the reference is guarded by an explicit
capability-pack condition — for example "When the `agent-intercom` capability
pack is installed, see `.github/instructions/agent-intercom.instructions.md`".
Read the enabled set from `capability_packs` in `.autoharness/config.yaml`. If
the guarding pack is **not** in that set, the target is *expected* to be absent
and the reference is **not** broken: emit nothing, or at most a P3 advisory.
Raising P1 here would report a correctly-configured workspace as broken on
every run.

This is not hypothetical in this workspace: `agent-intercom` is not an enabled
pack, so `.github/instructions/agent-intercom.instructions.md` is intentionally
not installed, while `AGENTS.md` and the `brainstorm`, `build-feature`,
`deliberate`, `fix-ci`, `impl-plan`, `operational-closure`, `plan-harden`, and
`runtime-verification` skills plus `_ship` and `_stage` all retain conditional
references to it by design. Only an **unguarded** reference to a missing file is
a real finding.

1. For each internal link `[text](path)` or `[text](path#anchor)`:
   * Verify the target file exists relative to the workspace root or the
     referring file.
   * If the file exists but the anchor is missing: P2 finding.
   * If the file is missing entirely **and the reference is not covered by the
     disabled-capability-pack exemption above**: P1 finding.
2. For agent definition references in skill tables or AGENTS.md:
   * Verify the referenced agent file exists in `.github/agents/`.
3. For skill references in agent protocols or instruction files:
   * Verify the referenced skill directory and `SKILL.md` exist in
     `.github/skills/`.

```text
Missing file: P1 — manual
Missing anchor only: P2 — advisory
Missing target guarded by a disabled capability pack: exempt (P3 advisory at most)
```

### Check 5 — Stale Content Detection (P2)

Identify content that references:

* Files or paths that no longer exist in the workspace. The Check 4
  disabled-capability-pack exemption applies here too: a path that is absent
  because its capability pack is not enabled is not stale content.
* Commands that no longer appear in the project toolchain
  (`go build ./cmd/backlogit`, `go test ./...`, `golangci-lint run`).
* Backlog item IDs mentioned as "active" but archived.
* Section anchors referenced internally that have been renamed.

```text
Action class: advisory for informational mentions; manual for
content that asserts an incorrect current state
```

### Check 6 — Frontmatter Field Completeness (P2)

Verify optional-but-recommended frontmatter fields:

* Instruction files: `applyTo` glob is present and non-empty.
* Skill files: `argument-hint` is present.

```text
Action class: advisory — missing recommended fields do not block but
degrade runtime behavior or tooling integration
```

Agent files additionally carry one **required** structured tier field:

* Agent files: `max_subagent_tier` is present as an integer. Per P-013.4 an
  agent that omits it is non-conformant. (The base tier is config-resolved via
  `model_routing`; there is no `model_tier` frontmatter field.)

```text
Action class: manual — a missing `max_subagent_tier` is a P-013.4 conformance
finding, not an advisory nit; surface it for correction before the next
verification pass
```

## Workflow

### Step 1: Resolve Scope

1. If arguments specify a path or glob, resolve it to a list of files.
2. Otherwise, default scope:
   * `.github/agents/*.md`
   * `.github/skills/**/SKILL.md`
   * `.github/instructions/*.md`
   * `AGENTS.md`
   * `docs/**/*.md` (excluding `docs/archive/`)
3. Announce scope size. Broadcast.

### Step 2: Select Review Model

1. Read `google` and `gemini-3.1-pro-preview`.
2. If both are non-empty: use the alternate provider/family for this review
   pass. Log `[DOC-REVIEW] Using alternate model: gemini-3.1-pro-preview
   (provider: google)`.
3. If either is empty: use Tier 2 defaults.

### Step 3: Run Check Suite

Execute Checks 1–6 over the resolved scope. For each finding:

1. Record: `{file, line, check_id, severity, message, action_class, fix_hint}`.
2. Broadcast the finding at the appropriate level.

### Step 4: Apply Actions (mode-dependent)

**Autofix mode:**

1. Apply all `safe_auto` findings automatically.
2. Create backlog follow-up items for `manual` findings using
   `backlogit add --type task --title <title>`.
3. Write the review artifact to `docs/closure/`.

**Report-only mode:**

1. Return structured findings to caller.
2. No side effects: no edits, no artifact, no follow-up items.

**Interactive mode:**

1. Present findings grouped by severity (P0 first).
2. For each P0/P1, ask the user to accept, modify, or reject the fix.
3. Apply approved fixes. Write review artifact.

### Step 5: Produce Output

Assemble the findings report:

```markdown
# Doc Review: {scope_summary}

Date: {YYYY-MM-DD}
Mode: {mode}
Model: {model_used}
Files reviewed: {count}

## P0 Findings

{findings}

## P1 Findings

{findings}

## P2 Findings

{findings}

## P3 / Advisory Findings

{findings}

## Summary

| Severity | Count | Auto-fixed | Deferred |
|---|---|---|---|
| P0 | {n} | {n} | {n} |
| P1 | {n} | {n} | {n} |
| P2 | {n} | {n} | {n} |
| P3 | {n} | {n} | {n} |
```

Write to `docs/closure/{YYYY-MM-DD}-doc-review-{slug}.md` (autofix and
interactive modes). Return structured findings to caller in report-only mode.

## Integration with Review Skill

The `review/SKILL.md` persona routing model may invoke `doc-review` as a
conditional reviewer when the diff touches:

* `.github/agents/`, `.github/skills/`, `.github/instructions/`
* `AGENTS.md`, `docs/`
* Any `.tmpl` file

When invoked from `review/SKILL.md` as a persona, this skill operates in
**report-only** mode and returns its structured findings for inclusion in
the parent review's finding merge and dedup pipeline.

## Quality Criteria

* Every file in scope is checked against all applicable checks.
* All P0 findings must be reported; none may be silently dropped.
* The review artifact (autofix and interactive modes) is written even if all
  findings are P3/advisory.
* The alternate model is used when configured; the fallback is Tier 2
  (never Tier 3 without explicit configuration).

## Model Routing

When `google` / `gemini-3.1-pro-preview` are set:
uses the alternate model (e.g., Gemini). Otherwise: **Tier 2 (Standard)**.

Generated by autoharness | Template: doc-review/SKILL.md.tmpl
