---
title: "Docline doc_type taxonomy, field mapping, and profile split"
doc_type: decision
source: docs/decisions/2026-06-22-docline-taxonomy-and-field-mapping.md
description: "Closed doc_type vocabulary, per-directory path mapping, legacy-field routing, authoring/ingestion/pipeline ownership tiers, and scope globs for the docline frontmatter contract. Explanatory rationale; the executable copy lives in internal/docline/policy.go."
---

## Purpose

This document is the human-readable rationale for the docline frontmatter
contract that `backlogit` applies to its durable documentation surface so the
external `graphtor-docs` ingestion pipeline can vector- and graph-index the
docs. It captures, **as explanatory rationale only (not machine-consumed)**:

1. the closed `doc_type` vocabulary,
2. the per-subdirectory path → `doc_type` map,
3. how every existing/legacy frontmatter field is routed (top-level contract
   field vs `docline` namespace),
4. the three field-ownership tiers (authored / repo-derived / pipeline-enriched)
   and the authoring-vs-ingestion validation profiles, and
5. the explicit in-scope / out-of-scope path globs.

> The **authoritative, machine-readable** copy of this policy lives in code
> (`internal/docline/policy.go`, task 065.004-T). This document explains the
> *why*; `policy.go` is the executable *what*. If the two disagree, code wins
> and this document is corrected.

The contract surface itself is fixed by
`schemas/docline/base-frontmatter-v1.schema.json` (authoritative; consumed,
never modified by this work).

## 1. Closed `doc_type` vocabulary

The `doc_type` field is a **closed** controlled vocabulary. A document whose
derived or declared `doc_type` is outside this set fails validation
(`ErrUnknownDocType`).

| `doc_type`  | Meaning                                                        |
|-------------|----------------------------------------------------------------|
| `reference` | CLI/API reference and stable factual reference material        |
| `decision`  | Decision records / deliberations (this document is one)        |
| `spike`     | Time-boxed investigation / exploration write-ups               |
| `plan`      | Implementation / execution plans                               |
| `closure`   | Post-merge operational closure records                         |
| `research`  | Background research and external-source synthesis              |
| `review`    | Multi-persona review-gate results                              |
| `learning`  | Compounded learnings / reusable pattern captures               |
| `spec`      | Product specifications / requirements                          |
| `design`    | Design documents (structural/architectural design)            |
| `guide`     | How-to guides, authoring guides, onboarding/knowledge surface  |

Total: **11** types. The set is intentionally small and stable; adding a type
is a contract change requiring a new decision record and a `policy.go` update.

## 2. Path → `doc_type` map

Classification is **directory-based and deterministic** (longest-prefix match),
with a small set of explicit root-file overrides. The classifier
(`internal/docline/classify.go`, 065.005-T) reads this map from `policy.go`.

| Path glob                  | `doc_type`  | Live count (2026-06-22) |
|----------------------------|-------------|-------------------------|
| `docs/cli-reference/**`    | `reference` | 59                      |
| `docs/decisions/**`        | `decision`  | 6                       |
| `docs/exec-plans/**`       | `plan`      | 43                      |
| `docs/closure/**`          | `closure`   | 40                      |
| `docs/research/**`         | `research`  | 1                       |
| `docs/reviews/**`          | `review`    | 5                       |
| `docs/compound/**`         | `learning`  | 40                      |
| `docs/design-docs/**`      | `design`    | 0 (reserved)            |
| `docs/product-specs/**`    | `spec`      | 0 (reserved)            |
| `docs/spikes/**`           | `spike`     | 0 (reserved)            |
| `docs/ARCHITECTURE.md`     | `reference` | 1 (root override)       |
| `docs/*.md` (direct child) | `guide`     | 8                       |
| `README.md` (repo root)    | `guide`     | 1 (root override)       |
| `AGENTS.md` (repo root)    | `guide`     | 1 (root override)       |

Notes:

* Longest-prefix wins, then explicit file override, then the `docs/*.md`
  direct-child default (`guide`).
* `design`, `spec`, and `spike` have no live directory yet; they are reserved so
  the vocabulary stays stable when those trees appear.
* Every in-scope subdirectory present on disk is covered.

## 3. Legacy field → docline routing

The contract surface (top level) holds **only** the docline contract fields
defined by the schema:
`title, source, ingested_at, doc_type, description, content_sha256,
source_path, chunk_strategy, schema_version, docline`.

Every **other** key found in existing heterogeneous frontmatter is **moved,
never dropped**, into the `docline` namespace (a nested map). The normalizer
(065.005-T) folds them; it never silently loses a key. The table below routes
every field named in the acceptance criteria plus the additional keys observed
in the live tree.

| Existing field          | Routing                          | Rationale                                                              |
|-------------------------|----------------------------------|------------------------------------------------------------------------|
| `title`                 | top-level `title`                | direct contract field (authored)                                       |
| `description`           | top-level `description`          | direct contract field (authored)                                       |
| `source`                | top-level `source` (verified)    | repo-derived; set to repo-relative POSIX path, OR preserved verbatim when a pre-existing full-URI known online source (see §4, Q2) |
| `type`                  | superseded → `docline.type`      | replaced by path-derived `doc_type`; original preserved                |
| `tags`                  | `docline.tags`                   | not a contract field; preserved as authored metadata                   |
| `severity`              | `docline.severity`               | review/compound metadata; preserved                                    |
| `date`                  | `docline.date`                   | authored doc date; distinct from `ingested_at` (see §4, Q1)            |
| `conclusion`            | `docline.conclusion`             | decision/spike outcome metadata; preserved                             |
| `confidence`            | `docline.confidence`             | decision/research metadata; preserved                                  |
| `linked_parent_work_item` | `docline.linked_parent_work_item` | backlog cross-ref; preserved                                       |
| `ms.date`               | `docline.ms.date`                | Microsoft-Learn authoring field; folded (move, never drop)             |
| `ms.topic`              | `docline.ms.topic`               | Microsoft-Learn authoring field; folded                                |
| `time_box`              | `docline.time_box`               | spike time-box metadata; preserved                                     |
| `promoted_to`          | `docline.promoted_to`            | learning/stash promotion pointer; preserved                            |
| `gate_decision`         | `docline.gate_decision`          | review-gate outcome (observed in `docs/reviews/**`); preserved         |
| `status`                | `docline.status`                 | authoring lifecycle marker; preserved (not a contract field)           |
| *(any other key)*       | `docline.<key>`                  | **catch-all**: unknown non-contract keys fold under `docline`          |

The catch-all rule (last row) is the invariant the T5 tests assert: after
normalization, the top level contains **only** docline contract fields, and
**no** prior key is lost.

### `ms.*` fold-vs-drop (Q3 confirmation)

`ms.date` / `ms.topic` are **folded** under `docline.ms.*`, not dropped. They
carry authoring provenance and cost nothing to preserve; dropping them would be
irreversible information loss in a migration whose explicit invariant is "move,
never drop."

## 4. Ownership tiers and validation profiles

Three ownership tiers resolve the "required-but-volatile" tension between the
schema's required fields and what a human actually hand-maintains.

| Tier                  | Fields                                                             | Who sets them                                  |
|-----------------------|--------------------------------------------------------------------|------------------------------------------------|
| **Authored**          | `title`, `description`, `doc_type` (intent), `docline.*` metadata  | Document author (human)                         |
| **Repo-derived**      | `source` (repo-relative POSIX path), path-derived `doc_type`, seeded `ingested_at` | `backlogit docs migrate`, deterministically     |
| **Pipeline-enriched** | `content_sha256`, `source_path`, authoritative `ingested_at`, `chunk_strategy`, `schema_version` | `graphtor-docs` ingestion pipeline (external)   |

Two validation profiles consume these tiers:

* **Authoring profile** (`backlogit docs lint --profile authoring`, also the CI
  gate): enforces the fields the repo owns — required `title`, `doc_type`,
  `source` (and closed-vocabulary `doc_type` membership). It does **not**
  require the pipeline-enriched fields, because the repo does not own them.
* **Ingestion profile** (`--profile ingestion`): enforces the **full** schema
  required set (`title`, `source`, `ingested_at`, `doc_type`) plus contract
  shape — the surface `graphtor-docs` actually ingests. Used to smoke-check that
  a migrated file satisfies the external contract.

`description`, `content_sha256`, and `source_path` are schema-optional (defaulted
to `""`); `chunk_strategy` defaults to `h1-h2-h3` and `schema_version` to `1.0`.

## 5. Scope globs

### In scope (the durable knowledge surface)

* `docs/**` **except** `docs/memory/**` and `docs/archive/**`.
* Root knowledge files: `README.md`, `AGENTS.md`.
* `docs/ARCHITECTURE.md` (covered by `docs/**`; called out for clarity).
* `docs/design-docs/**`, `docs/product-specs/**` if/when present.

### Out of scope

* `.github/**` — autoharness-generated prompt artifacts with a conflicting
  contract; standardization here would be clobbered on regeneration.
* `docs/memory/**` — high-churn agent memory checkpoints (156 files); not a
  durable ingestion surface.
* `docs/archive/**` — terminal/retired artifacts.
* Root `prompt.md` — generated prompt artifact, not knowledge surface.
* `schemas/docline/base-frontmatter-v1.schema.json` — read-only authoritative
  input.
* The `graphtor-docs` pipeline itself (external) and its pipeline-owned fields.

## Open operator questions — recommendations (PENDING SIGN-OFF, task 065.002-T)

These two questions change validator/normalizer behavior and the migration
contract. They are recorded here as **recommendations only**. The operator must
confirm them in task 065.002-T **before** any bulk migration (065.009-T). They
are **not** self-answered by the implementing agent.

### Q1 — `ingested_at` ownership

**Recommendation: seed-once at migration.**

* `backlogit docs migrate` writes an RFC3339 `ingested_at` timestamp **once**,
  at migration time, and the idempotent normalizer **preserves an existing
  value** on re-run (so re-migration produces zero diff and no timestamp churn).
* Rationale: the schema makes `ingested_at` **required** (`minLength ≥ 1`), so
  the repo must populate a value to satisfy the authoring → ingestion handoff.
  Seeding once keeps it stable and git-diff-quiet, and the repo does not pretend
  to track live pipeline ingest state.
* Trade-off: the seeded value is a *migration* timestamp, not a true *ingest*
  time. `graphtor-docs` should treat the repo `ingested_at` as a lower bound and
  is free to overwrite it with the authoritative ingest time on ingestion
  (pipeline-enriched tier).
* Alternative (rejected for now): omit `ingested_at` in the repo and let the
  pipeline own it entirely — rejected because it would make the migrated tree
  fail the schema's required-field check under the ingestion profile, breaking
  the smoke-check that a migrated file is ingestible.

### Q2 — `source` convention

**Recommendation: repo-relative POSIX path** (e.g.
`docs/closure/045-S-post-merge-closure-2026-04-26.md`).

* Rationale: stable, deterministic, derivable from the file's location,
  verifiable by lint, and diff-friendly. `source_path` (pipeline-owned) is the
  same repo-relative POSIX path, so the two stay consistent.
* Trade-off / confirmation needed: a repo-relative path is not a
  globally-resolvable URI on its own; `graphtor-docs` must combine it with the
  repo origin to form a full reference. **The operator must confirm that
  graphtor-docs accepts a repo-relative POSIX path for `source`** (vs requiring a
  full origin URL such as
  `https://github.com/softwaresalt/backlogit/blob/main/<path>`).
* Alternative (rejected for now): emit a full origin URL in `source` — rejected
  because it bakes the hosting origin and branch into every doc, churns on
  fork/rename, and duplicates information `graphtor-docs` already knows.

> **Gate status: CLOSED (2026-06-25).** The operator's answers to Q1 and Q2 are
> recorded in the "Policy Decisions (operator-confirmed)" section below; the bulk
> migration (065.009-T) and the CI enforcement gate (065.010-T) are unblocked.
> The recommendations above are retained as historical rationale.

## Policy Decisions (operator-confirmed)

**Sign-off date: 2026-06-25.** The operator confirmed the two open questions for
task 065.002-T. The migration gate is now **CLOSED**; bulk migration (065.009-T)
and the CI enforcement gate (065.010-T) are unblocked.

### Q1 — `ingested_at` ownership: SEED-ONCE AT MIGRATION (confirmed)

`backlogit docs migrate` writes an RFC3339 `ingested_at` timestamp **once**, at
migration time. Re-runs **preserve** the existing value, so re-migration is
idempotent (no timestamp churn, zero diff). `graphtor-docs` MAY overwrite the
value with the authoritative ingest time later (pipeline-enriched tier). This
matches the standing recommendation; no change to the tooling defaults.

### Q2 — `source` convention: REPO-RELATIVE POSIX PATH (confirmed, with a full-URI exception)

The default `source` convention for this workspace is a **repo-relative POSIX
path** (e.g. `docs/closure/045-S-post-merge-closure-2026-04-26.md`). The bulk
migration of in-scope workspace docs seeds `source` = repo-relative POSIX path.

**Operator nuance:** a full origin URI MAY be used for `source` where a
document's source is a known online source. Therefore the validator/codec MUST
**accept** a full URI in `source` where present and MUST **not** reject or
rewrite a pre-existing full-URI `source` that is a legitimate known online
source. Default = repo-relative path; full URI = allowed exception.

* **Implementation:** the normalizer preserves an existing `source` that carries
  a URI scheme (e.g. `https://…`) verbatim and derives the repo-relative POSIX
  path only when `source` is absent or is itself a repo-relative path. The
  authoring and ingestion validators require `source` to be non-empty only, so a
  full-URI source already passes validation unchanged.
  (065.002-T sign-off → `internal/docline/normalize.go` source rule.)
* No in-scope workspace doc currently carries a full-URI `source`, so the bulk
  migration seeds repo-relative paths throughout; the exception is a
  forward-looking contract guarantee for documents whose origin is online.

### Generated CLI reference docs (`docs/cli-reference/**`)

The 64 `docs/cli-reference/**` pages are generated by `cmd/gen-docs` (`make
docs`). Hand-migrating their frontmatter would be clobbered on the next
regeneration, breaking the CI gate (065.010-T). **Decision: Option A (durable).**
`cmd/gen-docs` is updated to emit docline-conformant frontmatter on every
generated page (`title`, `doc_type: reference`, repo-relative `source`,
`description`, seeded/preserved `ingested_at`) via the shared
`docline.Normalize` codec, so generated pages are born-compliant and stable
across regenerations (the CLI-Reference-Drift check stays green). `docs migrate`
is therefore not run over the generated pages; gen-docs owns their frontmatter.

> **Gate status: CLOSED (2026-06-25).** Operator answers to Q1 and Q2 recorded;
> bulk migration (065.009-T) and the CI enforcement gate (065.010-T) may proceed.
