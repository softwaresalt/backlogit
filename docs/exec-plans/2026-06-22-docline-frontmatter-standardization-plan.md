---
title: "Standardize documentation frontmatter on the docline base schema"
description: "Implementation plan: Go docline validator + native backlogit docs lint/migrate (CLI+MCP), batched content migration, CI gate, and authoring guidance"
date: 2026-06-22
origin: "docs/decisions/2026-06-22-docline-frontmatter-standardization-deliberation.md"
status: planned
stash_ids:
  - "29A71E9C"
---

## Problem Frame

backlogit must standardize its documentation frontmatter on the docline
`BaseFrontmatter` v1 contract (`schemas/docline/base-frontmatter-v1.schema.json`)
so the `graphtor-docs` ingestion pipeline can vector- and graph-index the docs.
Today none of the ~345 `docs/` markdown files satisfy the contract, and
frontmatter is heterogeneous across subdirectories.

The chosen approach (deliberation Option B) is a **durable native command**, not a
throwaway script: a tested Go docline-validation library plus `backlogit docs
lint` / `backlogit docs migrate` subcommands exposed over **both CLI and MCP**,
with an **authoring/ingestion profile split**, a closed `doc_type` taxonomy, the
`docline` namespace for backlogit-specific fields, batched idempotent migration,
and a CI enforcement gate.

Technical anchors in the codebase:

* Frontmatter model/serializer: `internal/models/frontmatter.go`,
  `internal/parser/markdown.go` (deterministic, sorted-key output already exists
  and must be reused).
* Field validation patterns: `internal/core/field_validation.go`.
* CLI command pattern: `internal/cli/*.go` cobra commands wired via
  `internal/cli/root.go` (existing siblings: `migrate.go`, `doctor.go`).
* MCP tool surface: `internal/mcp/`.
* New code lives in a new package `internal/docline/` to keep the contract logic
  cohesive and independently testable.

## Requirements Trace

| # | Requirement | Origin | Implementation unit(s) |
|---|---|---|---|
| R1 | Closed `doc_type` taxonomy + per-directory mapping + docline-namespace field placement + authoring/ingestion/pipeline ownership tiers + scope include/exclude | Deliberation Q1, Q2, Q3 | T1 (doc) + T4 (policy-as-code) |
| R2 | Neutral, body-preserving frontmatter codec decoupled from the backlog-artifact write path; nested `docline` map preserved | Q6 | T3 |
| R3 | Go docline `BaseFrontmatter` model + profile validator (authoring & ingestion) | Q2, Q4 | T4 |
| R4 | Path→doc_type classifier + idempotent, body-preserving normalizer | Q3, Q6 | T5 |
| R5 | Shared docline application service (`LintTree`/`PlanMigration`/`ApplyMigration` + result types) with workspace-root containment — single core for both surfaces | Q4 (CLI+MCP) | T6 |
| R6 | `backlogit docs lint`/`migrate`/`scope`/`classify` CLI adapter over the service | Q4, Q5, Q6 | T7 |
| R7 | MCP parity tools (`backlogit_docs_lint`/`_migrate`/`_scope`) with pinned I/O schema, success-with-violations envelope, apply gating | Q4 (CLI+MCP) | T8 |
| R8 | Bulk-migrate in-scope docs frontmatter in reviewable ≤25-file batches | Q1, Q6 | T9 |
| R9 | CI gate enforcing the authoring profile via `docs lint` (Go-native) + negative-path smoke | Q5 | T10 |
| R10 | Document the docline authoring contract (authoring guide + ARCHITECTURE) | Q3 | T11 |
| R11 | Operator policy sign-off: `ingested_at` ownership, `source` convention, taxonomy/`ms.*`/`prompt.md` | Q1, Q2 | T2 |

## Scope Boundaries

### In Scope

* `internal/docline/` validator + classifier + normalizer (new package).
* `backlogit docs lint` and `backlogit docs migrate` CLI subcommands + MCP tools.
* Frontmatter standardization of the **durable knowledge surface**: `docs/**`
  except `docs/memory/**` and `docs/archive/**`, plus root knowledge files
  `README.md`, `AGENTS.md`, `docs/ARCHITECTURE.md` (and `docs/design-docs/**`,
  `docs/product-specs/**` if present).
* CI enforcement gate + authoring documentation.

### Out of Scope

* `.github/**` prompt artifacts (autoharness-generated, conflicting contract).
* `docs/memory/**`, `docs/archive/**`, root `prompt.md`.
* The graphtor-docs ingestion pipeline itself (external) and the pipeline-owned
  fields `content_sha256`, `source_path`, authoritative `ingested_at`.
* Any change to the schema file `schemas/docline/base-frontmatter-v1.schema.json`
  (authoritative; consumed, not modified).

## Implementation Units

> **Global obligations (apply to every code-bearing unit T3–T8, T10).**
> *Quality gates (Constitution §Quality Gates):* before each unit's commit run, in
> order, `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .` (zero
> output). *Error model (Principle I):* `internal/docline` exports sentinel errors
> (`ErrUnknownDocType`, `ErrMissingRequiredField`, `ErrPathEscapesWorkspace`,
> `ErrBodyMutated`) and wraps at boundaries with
> `fmt.Errorf("docline.<fn>: %w", err)`. *Tests are written first (red→green)* per
> Principle II; test counts below are kept to the 2-hour heuristic using
> table-driven subtests where a unit needs broader coverage.

### T1 — Taxonomy, field-mapping, profile & ownership-tier decision doc
* **Domain**: docs. **Posture**: documentation-first.
* **Changes**: author `docs/decisions/2026-06-22-docline-taxonomy-and-field-mapping.md`
  capturing, **as explanatory rationale only (not machine-consumed)**: (a) closed
  `doc_type` vocabulary (`reference, decision, spike, plan, closure, research,
  review, learning, spec, design, guide`); (b) per-subdirectory path→doc_type map;
  (c) the keys folded under the `docline` namespace per category; (d) the three
  **ownership tiers** — *authored* (`title`, `description`, `doc_type` intent),
  *repo-generated/derived* (`source`, path-derived `doc_type`, seeded
  `ingested_at`), *pipeline-enriched* (`content_sha256`, `source_path`,
  authoritative `ingested_at`); (e) the scope include/exclude globs. The
  executable copy of this policy lives in code (T4), not in this document.
* **Files**: 1 new markdown doc.
* **Verify**: doc present; tables internally consistent and cover every in-scope
  subdirectory; `docs lint` passes on the doc itself once tooling exists.
* **Depends on**: — (root unit).

### T2 — Operator policy sign-off gate
* **Domain**: decision (operator checkpoint). **Posture**: spike/decision.
* **Changes**: resolve and record the operator decisions that determine code
  behavior and the migration contract: **Open Q1** (`ingested_at` seeded-once at
  migration vs omitted/pipeline-owned), **Open Q2** (`source` = repo-relative
  POSIX path vs canonical URL), plus Q3–Q6 confirmations (closed `doc_type` set,
  `ms.*` fold-vs-drop, `prompt.md` exclusion). Append the operator's decisions to
  the T1 doc as a dated "Policy Decisions (operator-confirmed)" section.
* **Files**: edit the T1 decision doc (decision record only — no code).
* **Verify**: each Open Q has a recorded operator decision; T4/T5 reference it.
* **Depends on**: T1. **Blocks**: T9 (migration MUST NOT run before sign-off);
  informs T4/T5 contract.

### T3 — Neutral, body-preserving frontmatter codec
* **Domain**: code (Go). **Posture**: test-first (red→green).
* **Changes**: `internal/docline/codec.go` — a frontmatter codec that operates on
  **raw bytes**: detect the leading `---` fenced block only, preserve the exact
  body byte-slice (offsets), handle CRLF without normalizing the body, and
  parse/serialize the frontmatter map with **deterministic sorted-key** output
  while **preserving nested maps verbatim** (the `docline` namespace and any
  `map[any]any` subtrees). It MUST NOT route through `internal/parser`
  `ParseMarkdownFile` or the `models.Artifact` write path (those require backlog
  fields and drop unknown keys). Add a **contract test that existing
  backlog-artifact serialization is unchanged** (freeze current behavior).
* **Files**: `internal/docline/codec.go`, `internal/docline/codec_test.go`
  (+ a small artifact-serialization contract test).
* **Tests (write first, table-driven)**: CRLF body round-trips byte-identical;
  no-frontmatter file → insert block without disturbing body; body beginning with
  `---` (horizontal rule) not misparsed; frontmatter containing `---` in a block
  scalar handled; nested `docline` map preserved; sorted-key stable output.
* **Verify**: `go test ./internal/docline/...` green; body-byte equality asserted.
* **Depends on**: — (codec is taxonomy-independent; may start early).

### T4 — docline policy (as code) + BaseFrontmatter model + profile validator
* **Domain**: code (Go). **Posture**: test-first (red→green).
* **Changes**: `internal/docline/policy.go` — the **machine-readable source of
  truth**: taxonomy map, scope include/exclude globs, authoring vs ingestion
  profile field lists, ownership tiers (mirrors T1, owned by code). `frontmatter.go`
  — `BaseFrontmatter` struct + defaults (`chunk_strategy=h1-h2-h3`,
  `schema_version=1.0`, optional `docline`). `validate.go` — `Validate(fm,
  profile)` enforcing required `title`/`doc_type`/`source` for the **authoring**
  profile and the full set for the **ingestion** profile, with closed `doc_type`
  membership and sentinel errors.
* **Files**: `internal/docline/policy.go`, `internal/docline/frontmatter.go`,
  `internal/docline/validate.go` (+ `validate_test.go`).
* **Tests (write first, table-driven)**: authoring-profile valid doc passes;
  missing required field fails actionably; defaults applied on parse; unknown
  `doc_type` rejected; ingestion profile fails on an authored-only file and passes
  on a fully-populated file.
* **Verify**: `go test ./internal/docline/...` green after red.
* **Depends on**: T1, T2, T3.

### T5 — Classifier + idempotent normalizer
* **Domain**: code (Go). **Posture**: test-first (red→green).
* **Changes**: `internal/docline/classify.go` (path→`doc_type` from `policy.go`)
  + `internal/docline/normalize.go` (heterogeneous frontmatter + repo-relative
  path → canonical docline frontmatter: map `title`/`description`, set `doc_type`
  + derived `source`, seed `ingested_at` **per the T2 decision**, fold
  category-specific keys under `docline` (move, never drop), deterministic output
  via the T3 codec, body bytes preserved, idempotent).
* **Files**: `internal/docline/classify.go`, `internal/docline/normalize.go`
  (+ `normalize_test.go`).
* **Tests (write first, table-driven)**: classification per representative subdir;
  idempotency `normalize(normalize(x)) == normalize(x)`; body bytes unchanged;
  every prior non-contract key preserved under `docline` (no silent loss);
  contract surface holds only docline fields.
* **Verify**: `go test ./internal/docline/...` green; idempotency asserted.
* **Depends on**: T3, T4.

### T6 — docline application service (shared core)
* **Domain**: code (Go). **Posture**: test-first (red→green).
* **Changes**: `internal/docline/service.go` — the **single application service
  both surfaces call**: `LintTree(opts) ([]Finding, error)`, `PlanMigration(opts)
  (MigrationPlan, error)`, `ApplyMigration(plan) (Result, error)`, plus shared
  typed results: `Finding{File,Field,Rule,Severity,Fix}`,
  `Change{File,Action(insert|update|noop),Before,After,BodyBytesChanged bool}`.
  Owns directory walking (honoring scope globs), structured diff computation,
  atomic temp+rename write orchestration, and **workspace-root containment**:
  every resolved path goes through `core.SafeResolve` (or equivalent); absolute
  paths and `..` escapes are rejected with `ErrPathEscapesWorkspace`. CLI and MCP
  are thin transport adapters over this service; no tree-walk/diff/write logic
  lives in the CLI or MCP packages.
* **Files**: `internal/docline/service.go`, `internal/docline/service_test.go`.
* **Tests (write first, table-driven)**: `LintTree` reports findings without
  mutation; `PlanMigration` yields structured `Change[]` with
  `BodyBytesChanged=false`; `ApplyMigration` writes atomically and is idempotent;
  a `--path`/opts value escaping the workspace root is rejected with zero writes.
* **Verify**: `go test ./internal/docline/...` green.
* **Depends on**: T4, T5.

### T7 — `backlogit docs` CLI adapter
* **Domain**: code (Go). **Posture**: test-first (red→green).
* **Changes**: `internal/cli/docs.go` (+ wire into `internal/cli/root.go`): parent
  `docs` cobra command with thin subcommands over the T6 service — `lint`
  (`--profile authoring|ingestion` default `authoring`, `--format text|json`
  default text on TTY / json on non-TTY, non-zero exit on violations); `migrate`
  (`--dry-run` is the **default**; `--apply` requires both `--yes` **and** an
  explicit `--path`, and refuses a whole-tree apply in one invocation; atomic
  writes; structured `Change[]` output); read-only `scope` (prints active
  globs/profile/taxonomy) and `classify <path>` (prints derived `doc_type`).
  Path inputs are containment-checked by the service.
* **Files**: `internal/cli/docs.go`, `internal/cli/docs_test.go`, edit
  `internal/cli/root.go`.
* **Tests (write first, table-driven)**: lint passes on a compliant fixture and
  fails (non-zero) on a missing-required fixture; exclude globs honored;
  `--format json` shape; `migrate --dry-run` writes nothing; `--apply` without
  `--yes`/`--path` refuses; `migrate --path ../outside` rejected with zero writes.
* **Verify**: `go test ./internal/cli/...` green; manual smoke on a fixture.
* **Depends on**: T6.

### T8 — MCP parity tools
* **Domain**: code (Go/MCP). **Posture**: test-first (red→green).
* **Changes**: `internal/mcp/docs_tools.go` — register `backlogit_docs_lint`,
  `backlogit_docs_migrate`, `backlogit_docs_scope` as **fixed, unconditional
  tools via `s.addTool`** (discoverable through `ListTools()` /
  `get_metadata_catalog.mcp_tools`; do **not** extend `backlogit_list_types`).
  Thin adapters over the T6 service. **Pinned I/O schema** identical to the CLI
  `--format json` shape (`findings[]`, `changes[]`); inputs `{path?, profile?,
  apply=false}`. `docs_lint` returns a **success envelope even when violations
  exist** (`{valid, violation_count, findings}`); MCP error results are reserved
  for invalid params / missing workspace / parse-IO failures. `docs_migrate`
  defaults to dry-run; `apply=true` is **gated server-side** (rejected with a
  structured `apply_not_permitted` error unless an explicit allow flag/env is
  set), **requires an explicit scoped `path` and refuses a whole-tree apply**
  (mirroring T7), and is path-containment-checked.
* **Files**: `internal/mcp/docs_tools.go`, `internal/mcp/docs_tools_test.go`,
  edit MCP registration.
* **Tests (write first, table-driven)**: `docs_lint` returns success-with-findings
  envelope; `docs_migrate apply=true` rejected under default config with zero
  writes; tools appear via `ListTools`/metadata catalog; CLI↔MCP parity test
  asserts identical JSON payloads for the same scenario.
* **Verify**: `go test ./internal/mcp/...` green.
* **Depends on**: T6, T7.

### T9 — Bulk-migrate in-scope docs frontmatter in reviewable ≤25-file batches
* **Domain**: docs/content. **Posture**: migration-first (tool-driven).
* **Changes**: run `backlogit docs migrate --apply --yes --path <batch>` over the
  in-scope tree in **batches of ≤25 files**, each a separate commit. Large
  subdirectories are split by alpha range: `cli-reference` (59→3 batches),
  `exec-plans` (41→2), `compound` (39→2), `closure` (37→2); small ones in one
  batch each (`decisions`+`research`, `reviews`, root knowledge +
  design-docs/product-specs if present). Conventional commit per batch:
  `docs(docline): migrate <batch> frontmatter to v1 [N files]`. *(2-Hour Rule
  note: each batch is a mechanical, tool-driven frontmatter rewrite reviewed on
  the aggregate diff; the per-file effort is sub-minute, so the file-count
  heuristic is satisfied by the ≤25-file batch boundary — see Constitution Check.)*
* **Files**: frontmatter-only edits across the in-scope tree (batched subtasks).
* **Verify (per batch)**: `docs lint` passes for that subtree; `git diff` shows
  frontmatter-only changes (`BodyBytesChanged=false`); re-running migrate yields
  zero diff; structured summary (changed/skipped/errored) logged.
* **Depends on**: T2 (operator sign-off — **blocking**), T7.

### T10 — CI gate + Makefile target + negative-path smoke
* **Domain**: config/CI. **Posture**: config-first.
* **Changes**: add a Makefile `docs-lint` target running `backlogit docs lint
  --profile authoring` over the in-scope tree, a CI workflow step invoking it
  (fail on violations), and a `docs-lint-negative` smoke that asserts a broken
  fixture under `tests/fixtures/docline-broken/` fails the gate. Go-native (no
  Node/Python JSON-schema dependency).
* **Files**: `.github/workflows/*.yml`, `Makefile`, `tests/fixtures/docline-broken/`.
* **Verify**: CI green on the migrated tree; negative fixture fails the gate.
* **Depends on**: T7, T9.

### T11 — Document the docline authoring contract
* **Domain**: docs. **Posture**: documentation-first.
* **Changes**: author a docline authoring guide (`docs/` knowledge surface)
  describing the authored fields, the `docline` namespace, the `doc_type`
  taxonomy, the ownership tiers, and the `docs lint`/`migrate`/`scope`/`classify`
  workflow; update `docs/ARCHITECTURE.md` and add an `AGENTS.md` pointer.
* **Files**: 1 new authoring guide + edits to `docs/ARCHITECTURE.md`, `AGENTS.md`.
* **Verify**: guide self-consistent with T1; `docs lint` passes on the new guide.
* **Depends on**: T1, T7.

## Dependency Graph

```text
T1 ─┬─> T2 ─┬───────────────> T9 ─> T10
    │       └─> T4 ─> T5 ─> T6 ─> T7 ─┬─> T8
    │   T3 ─────────^      ^          ├─> T9
    └─────────────> T11 <──┘ (T7) ────┘
```

* **Critical path**: T1 → T2 → T4 → T5 → T6 → T7 → T9 → T10.
* **Early-start**: T3 (codec) is taxonomy-independent and can begin immediately,
  in parallel with T1/T2.
* **Leaves**: T8 (MCP parity), T11 (docs) once their deps land.
* No cycles.

Edges (blocked-by):
* T2 ← T1
* T3 ← — (independent)
* T4 ← T1, T2, T3
* T5 ← T3, T4
* T6 ← T4, T5
* T7 ← T6
* T8 ← T6, T7
* T9 ← T2, T7   *(T2 = operator sign-off gate)*
* T10 ← T7, T9
* T11 ← T1, T7

## Constitution Check

| Principle | Status | Evidence |
|---|---|---|
| I — Safety-First Go | Pass | Sentinel errors + `%w` wrapping mandated (global obligations); errors actionable |
| II — Test-First (NON-NEGOTIABLE) | Pass | T3–T8, T10 specify tests written first (red→green), table-driven |
| III — Workspace Isolation | Pass | Scope globs exclude out-of-tree; service rejects path escapes |
| IV — CLI Containment (NON-NEGOTIABLE) | Pass | T6 routes all paths through `core.SafeResolve`; T6/T7/T8 reject `..`/absolute with `ErrPathEscapesWorkspace`; tested |
| V — Structured Observability | Pass | Structured `Finding[]`/`Change[]`; T9 logs per-batch summary; conventional commits |
| VI — Single Responsibility | Pass | Cohesive `internal/docline` package; no new external deps |
| VII — Destructive Approval (NON-NEGOTIABLE) | Pass | `migrate --apply` requires `--yes`+`--path`, refuses whole-tree; MCP apply gated server-side (`apply_not_permitted`) |
| VIII — Safety Modes | Pass | Plan Hardening with ProposedAction/ActionRisk/ActionResult; T2 operator gate |
| IX — Git-Friendly Persistence | Pass | Sorted-key deterministic codec, atomic temp+rename, idempotency tests |
| X — Context Efficiency | Pass | Structured JSON output for agent consumption |
| XI — Merge Commit History | Pass | T9 per-batch commits preserve granularity; no squash |
| Task Granularity (NON-NEGOTIABLE) | Pass | Each unit ≤3 files / single domain / atomic milestone; T9 batches ≤25 files with documented mechanical-rewrite justification |

## Decisions and Rationale

* **Dedicated `internal/docline/` package with a shared service layer** — keeps
  contract logic cohesive and unit-testable; CLI (T7) and MCP (T8) are thin
  adapters over one service (T6), so behavior cannot drift between surfaces.
* **Neutral body-preserving codec, NOT the artifact write path** — reusing
  `internal/parser.ParseMarkdownFile`/`models.Artifact` would require backlog
  fields, drop unknown keys, and risk altering existing artifact serialization;
  a dedicated raw-byte codec (T3) preserves body bytes and nested `docline` maps
  and is frozen against the artifact codec via a contract test.
* **Policy as code (T4), doc as rationale (T1)** — the executable taxonomy/scope/
  profile lives in `policy.go` so behavior is testable and not parsed from
  markdown; the decision doc explains the *why*.
* **Three ownership tiers** — authored / repo-generated-derived / pipeline-enriched
  — resolves the required-but-volatile tension; lint validates *computed* derived
  values (`source`, path-derived `doc_type`, seeded `ingested_at`) rather than
  treating them as hand-maintained.
* **`source` = repo-relative POSIX path** (pending T2 confirmation) — stable,
  derivable, verifiable; satisfies the schema's `source` requirement.
* **Category fields under `docline`** — keeps the shared contract surface clean;
  fields are *moved, never dropped* (asserted by T5 tests).
* **Operator sign-off gate (T2) before migration (T9)** — the `ingested_at` and
  `source` decisions change validator/normalizer behavior, so they are resolved
  before code freezes the contract and before any bulk write.

## Risks and Caveats

| Risk | Mitigation |
|---|---|
| 345-file diff is unreviewable | ≤25-file batches (T9 subtasks), one commit each, `--dry-run` default + structured `Change[]` preview |
| Migration corrupts document bodies | T3 raw-byte codec preserves body slice; round-trip + `BodyBytesChanged=false` assertions (T3/T5/T6) |
| Codec reuse breaks existing artifact serialization | T3 is a *separate* codec; contract test freezes `models` artifact serialization |
| Non-idempotent rewrites churn diffs | Deterministic sorted-key codec; idempotency tests assert zero second diff (T5/T6/T9) |
| CLI/MCP behavior drift | Single T6 service; CLI/MCP are adapters; T8 CLI↔MCP parity test |
| Path-escape on a write command | `core.SafeResolve` containment in T6; reject `..`/absolute; tested (T6/T7/T8) |
| Operator policy (Q1/Q2) not confirmed | T2 gate blocks T9; T4/T5 consume the recorded decision |
| `.github` regen clobbers any standardization | `.github/**` excluded from scope globs (T1/T4) |
| CI gate flips red before tree is compliant | T10 depends on T9; gate added only after the tree is migrated |

## Plan Hardening Signals (REQUIRED)

* **public API, schema, or contract change** — **PRESENT**: new CLI subcommands
  (`backlogit docs lint/migrate`) and MCP tools (`backlogit_docs_lint/migrate`)
  are a public, agent-facing contract; the docline frontmatter contract is
  applied repo-wide.
* **security, auth, permission, or compliance-sensitive behavior** — absent: docs
  metadata only; no auth/permission surface.
* **migration, backfill, destructive data/config action, or irreversible step** —
  **PRESENT**: bulk frontmatter backfill across ~345 files (large blast radius).
  Reversible via git, but a migration/backfill signal nonetheless.
* **external integration, operator checkpoint, or external dependency** —
  **PRESENT**: the contract exists to feed the external `graphtor-docs` ingestion
  pipeline; the authoring/ingestion profile boundary requires operator
  confirmation (open Q1).
* **high runtime, rollout, or rollback risk** — moderate: migration is git-
  reversible and batched; rollout is incremental per subdirectory.

**Requires plan hardening: yes**

## Runtime Verification and Closure

* **Changed runtime surfaces**: the `backlogit` CLI (`docs lint`, `docs migrate`)
  and the MCP tool catalog.
* **Runtime verification**: unit tests per package (T3–T8); fixture-tree lint
  pass/fail; `docs migrate --dry-run` on real subdirs; post-migration
  `docs lint` green for each batch (T9); CI gate green (T10); a smoke check that a
  migrated file is parseable by the graphtor-docs ingestion contract (full
  profile) before closing the feature.
* **Operational closure**: rollback = `git revert` of the offending batch commit
  (each T9 batch is an isolated commit); owner = repo maintainer; validation
  window = confirm graphtor-docs ingests the migrated tree without contract
  errors before the CI gate is set to blocking.

## Plan Hardening

**Hardening required: yes.** Triggered by three signals from the impl-plan:
(1) public/agent-facing contract change (new CLI subcommands + MCP tools +
repo-wide frontmatter contract), (2) migration/backfill across ~345 files (large
blast radius), and (3) external-integration coupling to the `graphtor-docs`
ingestion pipeline with an operator-decision boundary (the authoring/ingestion
profile split). Strict-safety is active, so risky actions are classified with the
`ProposedAction` / `ActionRisk` / `ActionResult` vocabulary below.

### Protected Invariants

1. **Body bytes are never altered by migration.** Only the YAML frontmatter block
   may change; document body content must be byte-identical pre/post migrate.
2. **Idempotency.** `docs migrate --apply` run twice produces a zero second diff.
3. **Deterministic, git-friendly output.** Sorted-key serialization, atomic
   temp+rename writes (Principle IX); no reordering churn on re-run.
4. **Contract-surface cleanliness.** Only the docline contract fields appear at
   top level; all backlogit-specific keys live under the `docline` namespace.
5. **Schema file is read-only.** `schemas/docline/base-frontmatter-v1.schema.json`
   is the authoritative input and must not be modified by this work.
6. **Out-of-scope trees are never touched.** `.github/**`, `docs/memory/**`,
   `docs/archive/**`, and root `prompt.md` must be excluded by the scope globs.

### Risky Actions (ProposedAction / ActionRisk)

| ProposedAction | ActionRisk | Approval | Expected ActionResult |
|---|---|---|---|
| **PA-1**: `docs migrate --apply` bulk-rewrites frontmatter across ~345 in-scope files (T9) | **High** (large blast radius, backfill) | Operator confirms profile boundary (T2 gate) + run is batched ≤25 files; each batch a separate commit; `--apply` requires `--yes`+`--path` | Frontmatter-only diff per batch; `docs lint` green; `BodyBytesChanged=false`; re-run = zero diff |
| **PA-2**: seed `ingested_at` once during migration (T5) | **Medium** (writes a schema-required value the pipeline also owns) | Operator confirms seed-once vs pipeline-owns (Open Q1) at the T2 gate before T9 | `ingested_at` present + stable; never re-stamped on subsequent migrate runs |
| **PA-3**: introduce new CLI subcommands + MCP tools (T7, T8) | **Medium** (public/agent-facing contract addition) | Standard review; agent-native parity check; pinned I/O schema | `backlogit docs *` + MCP tools available; identical JSON across CLI and MCP (shared T6 service) |
| **PA-4**: flip CI gate to blocking (T10) | **Medium** (can red-line the pipeline) | Gate added only after T9 makes the tree compliant | CI green on migrated tree; gate fails a deliberately broken fixture |
| **PA-5**: fold heterogeneous category keys under `docline` (T5) | **Low–Medium** (potential data loss if a key is dropped) | Test asserts no authored key is silently lost (moved, not deleted) | Every prior non-contract key preserved under `docline`; contract surface clean |

### Reinforced Verification

* **Pre-migration precheck (T9)**: run `docs migrate --dry-run` on the full
  in-scope tree first; review the aggregate diff stat; confirm zero body-byte
  changes and that excluded trees are absent from the change set.
* **Per-batch gate (T9)**: after each ≤25-file batch, `docs lint` MUST pass
  for that subtree and the structured `Change[]` MUST show `BodyBytesChanged=false`
  before the batch is committed.
* **Idempotency proof**: T5 and T6 carry an explicit "run twice → zero diff"
  test; T9 re-runs migrate post-apply to confirm no residual diff.
* **Body-preservation proof**: the T3 codec round-trip test asserts body bytes are
  unchanged; spot-check a sample of large/edge-case files (tables, code fences,
  embedded `---`, CRLF) for false frontmatter-fence detection.
* **Profile boundary smoke**: before closing the feature, validate that a
  migrated file passes the **full ingestion profile** when the
  pipeline-injected fields are added (simulating graphtor-docs), proving the
  authoring profile is a strict subset.

### Reinforced Rollback & Monitoring

* **Rollback procedure**: each T9 batch is an isolated commit on the feature
  branch; revert the specific batch via `git revert <sha>` without disturbing
  other batches. Because migration is frontmatter-only and idempotent, revert is
  clean and re-applicable.
* **Rollback trigger**: graphtor-docs ingestion reports a contract error on a
  migrated file, OR `docs lint` regresses, OR a body-byte delta is detected in
  review.
* **Monitoring signal**: graphtor-docs ingestion success/error count against the
  migrated tree during the validation window; CI `docs-lint` status.
* **Owner**: repo maintainer. **Validation window**: until one full graphtor-docs
  ingestion cycle succeeds against the migrated tree, after which the CI gate is
  set to blocking (T10).

### Edge cases reinforced for execution

* Files with **no existing frontmatter** (e.g. `docs/research/*`, some in-scope
  files) → the T3 codec must *insert* a frontmatter block without disturbing the
  leading body.
* Files whose body legitimately contains `---` (horizontal rules, YAML examples)
  → the T3 codec must only treat the leading fenced block as frontmatter (it does
  **not** reuse `internal/parser/markdown.go`; a dedicated regression test covers
  this).
* **CRLF** files → body byte sequence preserved exactly (no LF normalization).
* Files with a `title`/`description` already present → map through, do not
  duplicate.

### Unresolved operator decisions blocking safe execution

* **Open Q1 (high)** — confirm the authoring/ingestion profile boundary and the
  `ingested_at` seed-once policy (PA-2) at the **T2 operator gate, before T9
  runs**. Modeled as an explicit dependency: T9 ← T2.
* **Open Q2** — confirm `source` = repo-relative POSIX path is acceptable to
  graphtor-docs. A wrong convention would require a re-migration; also resolved at
  the T2 gate.

These operator decisions are gated as T2 and block the T9 migration action. They
do not block harvest/backlog creation; the backlog encodes T9 ← T2 so the
migration task cannot start before sign-off.

### Learnings & instructions consulted

* `docs/compound/` — no relevant prior frontmatter/migration learning (greenfield).
* `constitution.instructions.md` — Principle II (Test-First), Principle IX
  (Git-Friendly Persistence), Task Granularity (2-hour rule, width isolation).
* `strict-safety.instructions.md` — risky-action classification vocabulary.
* `schemas/docline/base-frontmatter-v1.schema.json` — authoritative contract.

## Plan Review

<!-- plan-review-attempt: 2 -->

Multi-persona review gate (always-on + triggered cross-model personas, run with
mixed models for diversity). Two review cycles were executed.

### Gate Decision: PASS (attempt 2)

Attempt 1 returned **FAIL** (multiple P1 findings). The plan was revised to
resolve every P1 and the material P2s, then re-reviewed. Attempt 2: Go Reviewer
**PASS**, Architecture Strategist **PASS**, MCP Protocol Reviewer **ADVISORY**
(single residual P2, folded into T8). No P0/P1 findings remain. Plan hardening
was required (migration/backfill + contract + external-integration signals) and a
`## Plan Hardening` section with `ProposedAction`/`ActionRisk`/`ActionResult`
classification is present and satisfies the strict-safety gate condition.

### Personas

Always-on: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings
Researcher (no compound prior art — greenfield). Cross-model (triggered):
Architecture Strategist (always), Agent-Native Parity Reviewer (MCP tools +
CLI/MCP parity), MCP Protocol Reviewer (new MCP tools).

### Attempt 1 findings (FAIL) and resolution

| # | Sev | Finding | Resolution |
|---|---|---|---|
| 1 | P1 | Reusing `internal/parser` `ParseMarkdownFile`/`models.Artifact` drops unknown keys, requires backlog fields | **Resolved** — T3 is a dedicated raw-byte codec that explicitly forbids that path; artifact-serialization frozen by contract test |
| 2 | P1 | Body-byte preservation unachievable via `models.ParseFrontmatter` (CRLF, separator trim, embedded `---`) | **Resolved** — T3 byte-oriented codec preserves exact body slice; CRLF/embedded-`---`/leading-`---` regression tests |
| 3 | P1 | Sorted-key no-loss serialization overstated; nested `docline` (`map[any]any`) could be dropped | **Resolved** — T3 deterministic codec preserves nested maps verbatim; round-trip tests |
| 4 | P1 | "Single core for CLI+MCP" only partial — tree-walk/diff/write left in CLI; MCP would duplicate | **Resolved** — T6 `service.go` (`LintTree`/`PlanMigration`/`ApplyMigration` + result types) is the shared core; T7/T8 are thin adapters |
| 5 | P1 | Dependency graph missing edges; operator-approval gate not modeled | **Resolved** — edges corrected (T4←T1,T2,T3; lint/classify flow through service); T9←T2 operator gate added |
| 6 | P1 | `--path`/MCP path on a write command lacks workspace-root containment | **Resolved** — T6 routes all paths through `core.SafeResolve`; `ErrPathEscapesWorkspace`; CLI/MCP escape-rejection tests |
| 7 | P1 | MCP discovery verified via `list_types` (wrong surface) | **Resolved** — T8 uses `s.addTool`/`ListTools`/`get_metadata_catalog.mcp_tools`; not `list_types` |
| 8 | P1 | MCP lint violations must be a success envelope, not a protocol error | **Resolved** — T8 `docs_lint` returns `{valid,violation_count,findings}`; errors reserved for bad params/IO |
| 9 | P1 | Missing required "Constitution Check" section | **Resolved** — `## Constitution Check` table added (I–XI + Task Granularity) |
| 10 | P1 | T7 batches (37–59 files) exceed the 2-Hour file-count heuristic without justification | **Resolved** — T9 batches capped at ≤25 files (large dirs split by alpha range) + documented mechanical-rewrite justification |
| 11 | P1 | MCP migrate apply-mode posture/I-O contract unspecified | **Resolved** — T8 pins `apply=false` default, server-side gate, scoped-path + whole-tree refusal, pinned JSON schema + CLI↔MCP parity test |
| 12 | P2 | Policy parsed from a markdown doc is brittle | **Resolved** — T4 `policy.go` is the machine-readable source of truth; T1 is rationale-only |
| 13 | P2 | Profile split leaky (`source`/`ingested_at`/derived `doc_type` not purely authored) | **Resolved** — three ownership tiers (authored / repo-generated-derived / pipeline-enriched); lint validates computed values |
| 14 | P2 | Quality gates (`go vet`/`golangci-lint`/`gofmt`) not enumerated | **Resolved** — global obligations block mandates all four gates per code unit |
| 15 | P2 | Destructive `--apply` approval routing (Principle VII) | **Resolved** — `--apply` requires `--yes`+`--path`, refuses whole-tree; MCP apply gated |
| 16 | P2 | Error model unspecified for new package | **Resolved** — sentinel errors + `%w` wrapping mandated |
| 17 | P2 | Structured dry-run output (assert `body_bytes_changed` programmatically) | **Resolved** — structured `Change[]{BodyBytesChanged}` in T6/T7/T8 |
| 18 | P2 | Test matrix thin; profile/scope/classify not surfaced to agents | **Resolved/Advisory** — matrices expanded; `--profile` exposed; `scope`/`classify` read-only surfaces added (CLI T7 + MCP T8) |

### Acknowledged advisory (P3) items for execution awareness

* Pin CLI `--format` defaults explicitly (text on TTY, json on non-TTY); MCP
  always emits structured JSON.
* If operator selects "seed `ingested_at` at migration date only" (Open Q1), the
  git-first-commit lookup branch in T5 can be dropped.
* Surface a single `apply_not_permitted` error story consistently across plan,
  CLI rejection, and MCP error.

### Open operator decisions (do not block harvest; gate the T9 migration via T2)

* **Open Q1 (high)** — `ingested_at` seed-once-at-migration vs pipeline-owned.
* **Open Q2** — `source` = repo-relative POSIX path acceptable to graphtor-docs?

### Cycle tracking

Attempt 1: FAIL. Attempt 2: PASS. Within the 2-cycle limit. Ready for harvest.
