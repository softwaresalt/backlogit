---
title: "Standardize documentation frontmatter on the docline base schema"
description: "Deliberation on refactoring backlogit documentation frontmatter to satisfy the docline BaseFrontmatter v1 contract for graphtor-docs vector + graph indexing"
topic: "Documentation frontmatter standardization on docline base-frontmatter-v1"
depth: "deep"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "schemas/docline/base-frontmatter-v1.schema.json"
  - "docs/exec-plans/2026-06-22-docline-frontmatter-standardization-plan.md"
stash_ids:
  - "29A71E9C"
tags:
  - "documentation"
  - "frontmatter"
  - "docline"
  - "graphtor-docs"
  - "schema"
  - "migration"
---

## Problem Frame

The operator added an authoritative JSON Schema —
`schemas/docline/base-frontmatter-v1.schema.json` (`$id`
`https://docline.softwaresalt.dev/schema/base-frontmatter/v1.json`, JSON Schema
draft 2020-12) — defining a common frontmatter contract (`BaseFrontmatter`) for
all markdown documentation. Standardizing backlogit docs on this contract enables
**vector and graph indexing** by the `graphtor-docs` ingestion pipeline and
interoperability across tools that consume the same contract.

The contract:

* **Required**: `title` (non-empty), `source` (origin URI/path, non-empty),
  `ingested_at` (date-time), `doc_type` (non-empty).
* **Optional (defaults)**: `description` (`""`), `content_sha256` (`""`,
  "Populated by the assemble pipeline"), `source_path` (`""`, "Populated by the
  assemble pipeline"), `chunk_strategy` (`"h1-h2-h3"`), `schema_version`
  (`"1.0"`), `docline` (object|null, `null`) — an explicit namespace for
  docline-only metadata that "must not leak into the shared contract surface."

**Current state (verified).** `backlogit sync` indexes 598 backlog artifacts;
the documentation tree is heterogeneous and **none** of the ~345 `docs/`
markdown files currently satisfy the docline contract. Survey results:

| Tree | Files | Frontmatter keys observed | Notes |
|---|---:|---|---|
| `docs/` (root files) | 9 | `title, description, ms.date, ms.topic` (ARCHITECTURE) | knowledge surface |
| `docs/cli-reference/` | 59 | `title, description` | reference docs |
| `docs/closure/` | 37 | `title, description, author, ms.date, ms.topic, keywords, estimated_reading_time` | closure reports |
| `docs/compound/` | 39 | `title, tags, date, severity` | learnings/patterns |
| `docs/decisions/` | 3 | `title, description, type, date, time_box, conclusion, confidence, linked_parent_work_item, promoted_to, tags` (+ ADR variants) | decisions + spikes |
| `docs/exec-plans/` | 41 | `title, description, date, origin, status, review, ms.date, source` | plans |
| `docs/memory/` | 150 | mixed; often **no** YAML; `session_phase, shipment, pr, merge_sha` when present | transient checkpoints |
| `docs/research/` | 1 | none in sample | legacy residue |
| `docs/reviews/` | 5 | `title, description, source, gate_decision, ms.date` | generated review reports |
| `docs/archive/` | 1 | `title, description, ms.date, ms.topic` | frozen history |
| `.github/**` | ~79 | `name, description, tools, applyTo, model_*, agent, policy_*, …` | **prompt artifacts — different contract** |

No file currently carries `source`, `ingested_at`, or `doc_type`. `.github/**`
files use an entirely different, autoharness-template-generated schema.

**Who cares / why.** The graphtor-docs ingestion pipeline (and any downstream
vector/graph index) needs a stable, machine-validated frontmatter contract to
chunk, embed, and link documents. backlogit is a Go CLI/MCP tool, so the durable
answer should reuse its existing frontmatter machinery
(`internal/models/frontmatter.go`, `internal/parser/markdown.go`,
`internal/core/field_validation.go`) and command surface
(`internal/cli/*.go` cobra commands wired via `root.go`, MCP tools in
`internal/mcp/`).

**Constraints.** Constitution Principle II (Test-First, NON-NEGOTIABLE),
Principle IX (Git-Friendly Persistence — sorted keys, atomic writes, minimal
merge conflicts), Task Granularity (2-hour rule, width isolation, atomic
milestones). 345 files is a large blast radius requiring idempotent, reviewable,
reversible migration.

**Success criteria.** (1) In-scope docs validate against an agreed authoring
profile of the docline contract; (2) a durable, tested Go command performs and
re-enforces the standardization; (3) a CI gate prevents regression; (4)
migration is idempotent, batched, and git-reviewable; (5) the backlogit-specific
fields are preserved without polluting the shared contract surface.

## Research Findings

* **Schema escape hatch is deliberate.** The `docline` namespace exists
  precisely so tool-specific metadata (e.g. `conclusion`, `severity`,
  `confidence`) does not get mistaken for graphtor-contract fields. This is the
  intended home for backlogit's heterogeneous category fields.
* **Pipeline-owned fields.** `content_sha256` and `source_path` are documented as
  "Populated by the assemble pipeline." A SHA-256 digest over body bytes cannot
  be hand-maintained without immediately going stale on every edit; coupling
  frontmatter to body bytes also fights Principle IX (stable, low-churn diffs).
* **Required-but-volatile tension.** `source` and `ingested_at` are *required* by
  the schema, yet `ingested_at` is intrinsically an ingestion-time timestamp.
  This forces a **profile split**: a repo-side "authoring profile" (what humans
  and agents maintain and CI enforces) vs the full graphtor "ingestion contract"
  (what the assemble pipeline completes at ingestion time).
* **Native tooling fits the architecture.** `internal/cli/` already contains
  `migrate.go` and `doctor.go`; adding a `docs` command (`docs lint`,
  `docs migrate`) follows the established cobra pattern and reuses the existing
  frontmatter serializer (which already emits deterministic output). A throwaway
  bash/python script would duplicate parsing logic, lack tests, and rot.
* **No prior art.** The compound learnings library (`docs/compound/`) contains no
  relevant prior solution for frontmatter migration or schema enforcement
  (learnings-retrieval confidence: low) — this is greenfield for the repo.
* **`.github/` conflict is real.** Those files are autoharness-generated from
  templates and excluded from the `docs` commit type; standardizing them would
  be overwritten on the next harness regeneration and would fight the
  template-generation flow. They are out of scope.

## Options Evaluated

### Option A: One-time bulk migration script (throwaway)

A single migration script (bash/python/Go `cmd/`) rewrites all `docs/`
frontmatter once; every contract field, including `ingested_at` and
`content_sha256`, is hand/script-authored in-repo. No durable command, no CI
gate beyond an ad-hoc check.

* **Pros**: fastest to a first pass; no command-surface design.
* **Cons**: duplicates frontmatter parsing logic outside the tested core;
  violates Principle II (hard to test a throwaway); `content_sha256`/`ingested_at`
  go stale immediately (high-churn diffs, violates Principle IX intent); no
  ongoing enforcement; no reusable authoring aid; the digest/body coupling is a
  maintenance trap.
* **Effort**: low up front, high long-term carrying cost.
* **Fit**: poor — fails the "durable answer for a Go CLI/MCP tool" goal.

### Option B (RECOMMENDED): Durable native `backlogit docs` command + profile split + CI gate

Build a tested Go docline-validation library and a native `backlogit docs`
command with `lint` (validate, read-only) and `migrate` (backfill/normalize)
subcommands, exposed via **both CLI and MCP**. Split the contract into an
**authoring profile** (repo-authored: `title`, `doc_type`, `source`,
`description`, plus seeded `ingested_at`; defaults for `chunk_strategy`,
`schema_version`) and a **pipeline-owned set** (`content_sha256`, `source_path`,
re-stamped `ingested_at`) that graphtor-docs completes at ingestion. Move all
backlogit-specific category fields under the `docline` namespace. The "one-time
migration" is simply the first batched invocation of `docs migrate`. A CI gate
runs `docs lint` (authoring profile) to prevent regression. Scope = the durable
knowledge surface: `docs/**` minus `docs/memory/**` and `docs/archive/**`, plus
root knowledge files (`README.md`, `AGENTS.md`, `docs/ARCHITECTURE.md`).

* **Pros**: reuses tested core frontmatter machinery; idempotent + deterministic
  output (Principle IX); TDD-friendly (Principle II); migration, enforcement, and
  future authoring share one source of truth (no drift); MCP parity for agents;
  pipeline-owned fields stay out of repo churn; `docline` namespace keeps the
  shared contract clean.
* **Cons**: more up-front design (command surface, profile semantics, taxonomy);
  requires the validator library before migration can run.
* **Effort**: medium — but amortized across enforcement and future authoring.
* **Fit**: strong — matches the Go CLI/MCP architecture and every constraint.

### Option C: Maximalist standardization (include `.github`, `memory`, `archive`; all fields in-repo)

Standardize every markdown file in the repo, require the full contract
(including `content_sha256` and live `ingested_at`) in-repo everywhere.

* **Pros**: total uniformity; one rule everywhere.
* **Cons**: fights the autoharness template-generation flow for `.github/**`
  (regenerated files would be clobbered); forces hand-maintenance of digests and
  timestamps (stale diffs, Principle IX violation); indexes 150 transient memory
  files and frozen archives, adding low-value, high-churn entries to the vector
  index; largest, riskiest blast radius.
* **Effort**: high; ongoing friction.
* **Fit**: poor — over-scoped (YAGNI) and conflicts with existing flows.

## Trade-off Comparison

| Criterion | Option A (throwaway) | Option B (native command) | Option C (maximalist) |
|---|---|---|---|
| Reuses tested core | No | Yes | Yes |
| Testability (Principle II) | Poor | Strong | Medium |
| Idempotent / git-friendly (IX) | Weak | Strong | Weak (digest/time churn) |
| Ongoing enforcement | None | CI gate via same tool | CI gate, but brittle |
| Blast radius | docs/ | docs/ minus transient | entire repo |
| Conflicts with autoharness | No | No (excludes `.github`) | Yes |
| Human maintenance burden | High (stale fields) | Low (profile split) | Very high |
| Agent (MCP) parity | No | Yes | Partial |
| Overall fit | Poor | **Strong** | Poor |

## Decision

**Adopt Option B.** Build a durable, tested `backlogit docs` command (CLI + MCP)
backed by a Go docline-validation library, with an explicit authoring/ingestion
**profile split**, a closed `doc_type` taxonomy, the `docline` namespace for
category fields, batched idempotent migration, and a CI enforcement gate.

Resolutions to the six design questions:

**Q1 — Scope boundary.** Standardize the **durable, human/agent-authored
knowledge surface**:

* **In scope**: `docs/**` EXCEPT `docs/memory/**` and `docs/archive/**`; PLUS
  root knowledge files `README.md`, `AGENTS.md`, `docs/ARCHITECTURE.md`; PLUS
  `docs/design-docs/**` and `docs/product-specs/**` if present (referenced in
  AGENTS.md).
* **Out of scope**: `.github/**` (autoharness-generated prompt artifacts with a
  conflicting, regenerated contract), `docs/memory/**` (150 transient
  high-churn session checkpoints, mixed/no frontmatter), `docs/archive/**`
  (frozen history), and root `prompt.md` (a prompt-engineering artifact).
* **Mechanism, not hardcoding**: the migrate/lint command takes an explicit
  include/exclude config so scope is data-driven and `memory`/`archive` can be
  added later as a phase-2 with relaxed `doc_type`s if graphtor-docs decides to
  ingest them.
* *Rationale*: graphtor-docs indexes durable knowledge; transient memory and
  frozen archives add low-value, high-churn index entries, and `.github`
  standardization would be overwritten by harness regeneration.

**Q2 — Authored vs pipeline-populated fields.** Resolve the required-but-volatile
tension with a **two-profile model**:

* **Authoring profile (repo-side, enforced by `docs lint` + CI)** — required:
  `title`, `doc_type`, `source`; expected: `description`; defaulted:
  `chunk_strategy` (`h1-h2-h3`), `schema_version` (`1.0`); optional: `docline`.
  `source` is authored as the **repo-relative POSIX path** of the file (stable,
  meaningful provenance, trivially derivable and verifiable by the tool) — this
  satisfies the schema's `source` requirement with a stable value.
* **Pipeline-owned set (graphtor-docs assemble pipeline)**: `content_sha256`
  (digest over body bytes — never hand-authored), `source_path` (pipeline's
  project-relative path), and the authoritative `ingested_at` (stamped at actual
  ingestion). `docs lint`/CI do **not** require these in-repo.
* **`ingested_at` seeding**: `docs migrate` seeds `ingested_at` **once** at
  migration time (from the file's git first-commit date, falling back to the
  migration date) so each repo file is self-valid against the full schema; it is
  thereafter treated as immutable/pipeline-managed, never continuously
  hand-maintained. (Flagged as the top open question for operator confirmation.)

**Q3 — doc_type taxonomy + field placement.** Closed controlled vocabulary (v1):
`reference`, `decision`, `spike`, `plan`, `closure`, `research`, `review`,
`learning`, `spec`, `design`, `guide`. Mapping:

| Source category | `doc_type` |
|---|---|
| `docs/cli-reference/**` | `reference` |
| `docs/ARCHITECTURE.md`, `docs/design-docs/**` | `reference` / `design` |
| `docs/decisions/**` (`type: spike`) | `spike` |
| `docs/decisions/**` (ADR/other) | `decision` |
| `docs/exec-plans/**` | `plan` |
| `docs/closure/**` | `closure` |
| `docs/research/**` | `research` |
| `docs/reviews/**` | `review` |
| `docs/compound/**` | `learning` |
| `docs/product-specs/**` | `spec` |
| `README.md`, `AGENTS.md` | `guide` |

All non-contract category fields move **under the `docline` namespace** (the
schema's explicit escape hatch): `tags`, `severity`, `conclusion`, `confidence`,
`linked_parent_work_item`, `time_box`, `promoted_to`, `gate_decision`, `status`,
`origin`, `review`, `author`, `keywords`, `session_phase`, and deprecated `ms.*`
keys (`ms.date`, `ms.topic` → `docline.ms_date` / `docline.ms_topic`, or dropped
in favor of git history). `title` and `description` map to the top-level contract
fields. This keeps the shared contract surface exactly the docline fields, so
graphtor consumers never confuse a backlogit field for a contract field.

**Q4 — Migration mechanism.** **Both, realized as one durable native command.**
Build `backlogit docs lint` + `backlogit docs migrate` (CLI + MCP) backed by a
tested docline-validation library; the "one-time bulk migration" is simply the
first batched run of `docs migrate --apply`. No throwaway script. Reuses the
existing frontmatter serializer for deterministic, sorted-key output.

**Q5 — Enforcement.** **Yes — a CI gate** runs `backlogit docs lint` (authoring
profile) over the in-scope tree and fails on violations, using the **Go-native
validator** (single source of truth with `docs migrate`, no extra Node/Python
JSON-schema dependency in CI, integrates with the existing Makefile/CI). The gate
validates the authoring profile, not the full ingestion contract (the repo
legitimately lacks pipeline-injected fields).

**Q6 — Idempotency & safety.** `docs migrate` is idempotent (re-running yields no
diff; compliant files are untouched), writes **only frontmatter** with stable
sorted-key ordering and atomic writes (Principle IX), never computes
body-coupled fields in-repo (`content_sha256` is pipeline-owned), offers
`--dry-run` (print diffs) and `--check`/lint (report only) modes, and runs in
**reviewable per-subdirectory batches** (each batch a separate commit, revertable
via `git revert`).

## Rejected Alternatives

* **Option A (throwaway script)** — rejected: duplicates tested logic outside the
  core, no enforcement, stale digest/timestamp churn, no agent parity.
* **Option C (maximalist)** — rejected: over-scoped (YAGNI), forces
  hand-maintenance of pipeline-owned fields, conflicts with autoharness
  regeneration of `.github`, and pollutes the index with transient/archived
  content.
* **Requiring `content_sha256`/`ingested_at` in-repo** — rejected: body-coupled
  and time-coupled fields cause perpetual churn; delegated to the ingestion
  pipeline via the profile split.
* **Promoting category fields to top-level** — rejected: pollutes the shared
  contract surface; the `docline` namespace exists specifically to prevent this.

## Unresolved Questions

1. **(High) Authoring-profile boundary confirmation.** Confirm repo authors
   `{title, doc_type, source, description}` and the pipeline owns
   `{content_sha256, source_path, ingested_at}`. Specifically: should `docs
   migrate` seed `ingested_at` once at migration, or should repo files omit it
   entirely and rely on `graphtor-docs` to inject it (requires graphtor-docs to
   accept files lacking a schema-required field at the repo layer)?
2. **`source` value convention.** Confirm `source` = repo-relative POSIX path is
   acceptable to graphtor-docs, vs a canonical repo URL or a value graphtor
   assigns.
3. **Phase-2 trees.** Should `docs/memory/**` and/or `docs/archive/**` ever be
   ingested? Default is excluded; the tool's include/exclude config leaves the
   door open.
4. **doc_type vocabulary.** Confirm the closed v1 set, especially `design` vs
   `reference` for design-docs/ARCHITECTURE and `guide` vs `reference` for
   README/AGENTS.
5. **Deprecated `ms.*` keys.** Drop entirely (rely on git history) vs fold under
   `docline.ms_*`? Default: fold under `docline`.
6. **`prompt.md`.** Confirm root `prompt.md` stays out of scope (treated as a
   prompt artifact like `.github/**`).

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| 345-file blast radius creates an unreviewable diff | Per-subdirectory batched migration; one commit per batch; `--dry-run` preview |
| Migration corrupts body content | Tool writes only the frontmatter block; round-trip tests assert body bytes unchanged |
| Non-idempotent rewrites churn diffs | Canonical sorted-key serialization; re-run yields zero diff (asserted by test) |
| Schema/contract drift between migrate and CI | Single Go validator library shared by `docs migrate`, `docs lint`, and the CI gate |
| Required-but-volatile fields force bad hand-maintenance | Authoring/ingestion profile split; pipeline owns volatile fields |
| `.github` standardization clobbered by harness regen | `.github/**` explicitly excluded |
| Taxonomy disputes block migration | Taxonomy fixed in a decision doc task first; migrate consumes a config map, not hardcoded rules |
