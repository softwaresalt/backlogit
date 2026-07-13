---
chunk_strategy: h1-h2-h3
description: Reconcile the spike skill's findings-artifact frontmatter example with the docline base frontmatter v1 contract, surgically and without a broad plugin content-sync.
doc_type: plan
docline:
    stash_ref: E75605E5
    conclusion: proceed
    confidence: high
ingested_at: "2026-07-13T19:52:00Z"
schema_version: "1.0"
source: docs/exec-plans/2026-07-13-spike-findings-docline-reconciliation-plan.md
title: Reconcile spike skill findings-artifact example with docline frontmatter contract
---

## Reconcile spike skill findings-artifact example with docline frontmatter contract

## Context

Stash entry `E75605E5` (task, medium): *"Follow-up: reconcile plugin spike
skill findings-artifact example with docline frontmatter contract without broad
plugin content-sync."*

The `spike` skill's **Phase 5: Write Findings Artifact** section embeds a fenced
`markdown` code block that shows agents the YAML frontmatter to emit when they
write a findings artifact to `docs/decisions/{YYYY-MM-DD}-{slug}-spike.md`. That
output path is **in docline scope** (`docs/**`), so the example must teach
docline-conformant frontmatter. It currently does not.

### Current (divergent) example frontmatter

```yaml
---
title: "{Goal question — short form}"
type: spike
date: {YYYY-MM-DD}
time_box: "{time_box value}"
conclusion: "{proceed|pivot|defer|abandon}"
confidence: "{high|medium|low}"
linked_parent_work_item: "{feature or chore path/ID, or null}"
promoted_to: ["{plan|queue|learnings|none}"]
tags:
  - "{domain tag}"
  - "{technology tag}"
---
```

### Divergences from the docline base frontmatter v1 contract

Per `docs/docline-frontmatter-authoring-guide.md` and
`schemas/docline/base-frontmatter-v1.schema.json` (authoring profile):

1. Uses `type: spike` at top level instead of the required `doc_type`. For a
   `docs/decisions/**` path the derived `doc_type` is **`decision`** (verified
   with `backlogit docs classify`). The `type: spike` marker belongs under the
   `docline` namespace.
2. Missing the required `source` field (repo-relative POSIX path).
3. Eight non-contract keys (`type`, `date`, `time_box`, `conclusion`,
   `confidence`, `linked_parent_work_item`, `promoted_to`, `tags`) sit at the
   top level. The contract requires every non-contract key to live under the
   `docline:` namespace (move, never drop).

### Established convention (gold standard)

Two existing conformant spike findings artifacts already model the target shape:

* `docs/decisions/2026-05-05-telemetry-gap-analysis-spike.md`
* `docs/decisions/2026-07-09-github-actions-cost-spike.md`

Both use top-level `title` / `source` / `doc_type: decision` / `description`
(plus migrator-seeded `chunk_strategy` / `schema_version` / `ingested_at`) and
nest `type: spike`, `date`, `time_box`, `conclusion`, `confidence`,
`linked_parent_work_item`, `promoted_to`, and `tags` under `docline:`.

## Goal

Update the findings-artifact frontmatter example in the spike skill so an agent
that follows it produces frontmatter that passes
`backlogit docs lint --profile authoring` and matches the established
convention — without triggering a broad plugin content-sync or sweeping edits
across plugin content.

## Scope

### In scope (surgical)

Replace the single fenced-YAML example block in the **Phase 5** section of both
in-repo generated copies of the spike skill:

1. `plugin/skills/spike/SKILL.md` — the explicit stash target (plugin bundle
   copy, referenced by `.github/plugin/plugin.json`).
2. `.github/skills/spike/SKILL.md` — the in-repo twin that contains the
   identical example block and is the skill agents load when operating in this
   repository. Leaving it divergent would perpetuate the non-conformant pattern.

Both edits are the **same** one-block replacement. This is two surgical edits of
one example, not a regeneration or content-sync of the plugin bundle.

### Out of scope

* The upstream `spike/SKILL.md.tmpl` generator source lives in the external
  autoharness repo. Per Principle IV (CLI Workspace Containment) it MUST NOT be
  edited from this workspace. It is recorded as a follow-up stash so a future
  regeneration does not overwrite this in-repo fix.
* No broad plugin content-sync, no regeneration of other skills, no edits to any
  other skill/agent/template file.
* No schema changes (`schemas/`), no CLI changes (`src/`, `internal/`), no
  changes to `docs/decisions/**` existing artifacts.
* No relocation of the findings-artifact output path from `docs/decisions/` to a
  new `docs/spikes/` tree — that would be a larger convention change and is not
  what this stash asks for. The example keeps the `docs/decisions/` path (→
  `doc_type: decision`), matching both existing conformant artifacts.

## Proposed change

Replace the divergent example block with the reconciled block below in both
files (identical replacement):

```yaml
---
title: "{Goal question — short form}"
source: docs/decisions/{YYYY-MM-DD}-{slug}-spike.md
doc_type: decision
description: "{One-line summary of the spike}"
docline:
  type: spike
  date: {YYYY-MM-DD}
  time_box: "{time_box value}"
  conclusion: "{proceed|pivot|defer|abandon}"
  confidence: "{high|medium|low}"
  linked_parent_work_item: "{feature or chore path/ID, or null}"
  promoted_to: ["{plan|queue|learnings|none}"]
  tags:
    - "{domain tag}"
    - "{technology tag}"
---
```

Rationale for field placement:

* `title`, `source`, `doc_type` are the required top-level authoring fields.
* `doc_type: decision` matches the `docs/decisions/**` taxonomy mapping (the
  path the skill writes to), verified via `backlogit docs classify`.
* `description` is an optional authored field (kept for a good example).
* `chunk_strategy`, `schema_version`, and `ingested_at` are migrator/pipeline
  seeded and are intentionally omitted from the authored example (the authoring
  profile does not require them; `backlogit docs migrate` fills them).
* All eight former top-level non-contract keys move verbatim under `docline:`,
  preserving `type: spike` as the semantic marker (matching existing artifacts).

No surrounding prose in the Phase 5 section changes; the output path sentence
("Write the findings artifact to `docs/decisions/{YYYY-MM-DD}-{slug}-spike.md`")
stays as-is and is now consistent with `doc_type: decision`.

## Acceptance criteria

1. The Phase 5 example block in `plugin/skills/spike/SKILL.md` and
   `.github/skills/spike/SKILL.md` matches the reconciled block above (identical
   in both files).
2. A `docs/decisions/*-spike.md` file authored from the reconciled example (with
   placeholders filled) passes `backlogit docs lint --profile authoring` with
   zero findings.
3. No unresolved `{{...}}` template variables introduced; YAML is valid; the
   markdown structure of the Phase 5 section is otherwise unchanged.
4. `make verify-plugin` (`go test ./tests/integration/ -run
   'TestPluginBundleStructurallyValid'`) still passes — skill directory
   structure and SKILL.md `name`/`description` frontmatter are untouched.
5. No files outside the two target skill files are modified.

## Verification steps (for Ship)

1. Apply the two identical block replacements.
2. Author a scratch `docs/decisions/2026-07-13-scratch-spike.md` from the
   reconciled example, run `backlogit docs lint --profile authoring --path
   docs/decisions/2026-07-13-scratch-spike.md`, confirm zero findings, then
   delete the scratch file (it is a verification artifact only).
3. Run `make verify-plugin`.
4. Confirm `git diff --stat` shows only the two skill files changed.

## Risks and mitigations

* **Risk**: The `.github` twin is out of the stash's literal "plugin" wording.
  **Mitigation**: Documented as a same-concern, same-block extension; trivially
  reducible to plugin-only if the operator prefers. Not a content-sync.
* **Risk**: Upstream regeneration overwrites the fix.
  **Mitigation**: Follow-up stash records the upstream `.tmpl` reconciliation
  (external repo, out-of-tree — deferred per Principle IV).
* **Risk**: Convention drift toward a `docs/spikes/` tree.
  **Mitigation**: Explicitly out of scope; example keeps `docs/decisions/` +
  `doc_type: decision` to match existing artifacts.

## References

* `docs/docline-frontmatter-authoring-guide.md`
* `schemas/docline/base-frontmatter-v1.schema.json`
* `docs/decisions/2026-05-05-telemetry-gap-analysis-spike.md`
* `docs/decisions/2026-07-09-github-actions-cost-spike.md`
* `plugin/skills/spike/SKILL.md` (Phase 5), `.github/skills/spike/SKILL.md` (Phase 5)
* `.github/plugin/plugin.json`, `tests/integration/plugin_manifest_test.go`
* `docs/compound/2026-06-26-docline-frontmatter-contract.md` (born-compliant authoring pattern, 076-S)

## Plan Review

**Gate decision: PASS** — proceed to harvest.

Reviewed inline (single-agent Stage context) against the plan-review persona
checklist. No P0/P1 findings. No plan-hardening signals (blast radius is limited
to two instructional skill-doc examples; no schema, CLI-distribution, or
multi-template-family surface), so `plan-harden` is correctly not required.

### Persona results

* **Constitution Reviewer** — PASS. Principle IV honored (out-of-tree `.tmpl`
  edit deferred to a follow-up stash, not attempted). Test-First (P-II) applies
  to Go code; this is a documentation/skill-example change with explicit
  verification steps (`backlogit docs lint --profile authoring`,
  `make verify-plugin`). Task granularity: one task, single width (skill
  authoring), atomic milestone. No violations.
* **Go Reviewer** — N/A. No Go code changes.
* **Scope Boundary Auditor** — PASS. Scope is bounded to one example block in two
  files. No `docs/spikes/` tree introduction; no plugin regeneration/content-sync.
  See P3-1 for the `.github` twin judgment.
* **Learnings Researcher** — PASS (confirming). The plan directly applies the
  established pattern in `docs/compound/2026-06-26-docline-frontmatter-contract.md`
  (§ "Reinforcement — 076-S"): teach an agent-authoring surface the
  gate-required frontmatter shape so its output is born-compliant. No prior
  resolution is contradicted; the reconciled `doc_type: decision` +
  `docline`-namespaced keys match the two existing conformant artifacts.
* **Architecture Strategist** — PASS. Isolated to skill documentation; no module
  boundary, coupling, or dependency impact.
* **Agent-Native Parity Reviewer** — PASS. The change improves agent/tooling
  parity (spike outputs become docline-lintable); no parity regression.
* **Security Lens Reviewer** — N/A. No auth/authz, secrets, API surface, or
  external trust boundary touched.

### Findings

* **P3-1 (advisory)** — The stash text says "plugin spike skill," but the
  identical example block also lives in the in-repo twin
  `.github/skills/spike/SKILL.md`. The plan includes both as one atomic,
  same-block reconciliation. This is well-justified and reversible (trivially
  narrowable to plugin-only) and is **not** a broad plugin content-sync.
  Recorded for operator awareness; does not block harvest.
* **P3-2 (advisory)** — Optional enhancement: mirror impl-plan's treatment by
  adding the unquoted-`#` YAML quoting reminder near the example. Deferred to
  keep the reconciliation minimal; not required for docline conformance.

### Verification / closure readiness

Verification steps are defined (docs lint of an author-from-example scratch file;
`make verify-plugin`; `git diff --stat` bounded to two files). Operational
closure is Ship-owned. No runtime code surface, so no runtime-verification gap.
