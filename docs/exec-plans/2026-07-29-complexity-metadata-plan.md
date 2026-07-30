---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for adding optional task-level complexity metadata alongside size and priority, with body-preserving writes, WIT discovery, SQLite projection, CLI/MCP update and filter surfaces, and explicit operator-facing semantics.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-29-complexity-metadata-plan.md
title: 'Add optional task complexity metadata'
---

## Source

* Stash entry: `D46F3B0C` (kind `feature`, priority `medium`)
* Deliberation:
  `docs/decisions/2026-07-29-complexity-metadata-deliberation.md`
* Existing size precedent:
  * `internal/core/artifact_size.go`
  * `internal/config/defaults.go`
  * `.backlogit/header-def.yaml`
  * `internal/cli/update.go`
  * `internal/mcp/tools.go`
  * `internal/db/schema_gen.go`
  * `internal/db/queries.go`
  * `internal/core/size_composition.go`

## Problem Frame

Backlogit has task-level `size` metadata for work volume and `priority` for
importance. It does not have a dedicated field for implementation difficulty,
uncertainty, or cross-cutting risk. Operators and agents can approximate that
signal with labels or priority, but those surfaces are not validated, not
discoverable through WIT metadata, and not reliable for queue filters or SQL.

The planned change adds optional task-level `complexity` metadata that mirrors
the size pattern without changing default queue ordering. Complexity is a
planning signal:

* `size`: how much implementation volume a task has
* `complexity`: how hard, uncertain, or risky the implementation is
* `priority`: how important or time-sensitive the work is

## Requirements Trace

| Source requirement | Implementation action |
|---|---|
| Frontmatter field | Store logical task complexity in `custom_fields.complexity`, matching size's storage pattern. |
| SetArtifactSize-style setter | Add a body-preserving `SetArtifactComplexity` seam that validates against WIT metadata before writing. |
| CLI surface | Add `backlogit update --complexity` and list/filter support. |
| MCP surface | Add `complexity` to `backlogit_update_item` and `backlogit_list_items`, with mutual exclusion matching size. |
| Discovery via `get_wit_metadata` | Add task WIT metadata and live `.backlogit/header-def.yaml` values. |
| WIT allowed-values | Use `trivial`, `low`, `medium`, `high`; optional and no default. |
| SQLite index column | Add and populate `items.complexity` from `custom_fields.complexity` through an idempotent schema-extension path and shared upsert projection. |
| Query support | Add `QueryFilters.Complexity`, CLI/MCP filters, and direct SQL queryability. |
| Relationship to size and priority | Add CLI/MCP descriptions and documentation explaining the distinct semantics. |

## Implementation Units

### Unit A — Define the complexity WIT contract

* **Posture:** test-first.
* **Files:** `internal/config/defaults.go`,
  `internal/config/defaults_complexity_test.go`, `.backlogit/header-def.yaml`.
* **Changes:**
  * Add a task-only `complexity` field to default header-def generation.
  * Set `type: enum`, `values: [trivial, low, medium, high]`,
    `optional: true`, and no default.
  * Update the live workspace header-def so `get_wit_metadata` discovers the
    same contract.
* **Tests:**
  * Task schema exposes optional complexity with the exact allowed values.
  * Feature and shipment schemas do not define stored complexity.
  * A task without complexity remains valid.
* **Acceptance criteria:**
  * WIT metadata is task-only and absent on aggregate types.
  * The live and generated header-def contracts agree.

### Unit B — Implement the body-preserving core complexity seam

* **Posture:** test-first.
* **Files:** `internal/core/artifact_complexity.go`,
  `internal/core/artifact_complexity_test.go`.
* **Changes:**
  * Add `SetArtifactComplexity(ctx, ws, id, complexity)` patterned after
    `SetArtifactSize`.
  * Validate non-empty values against the task WIT enum before any write.
  * Treat an explicit empty value as a clear/unset operation: remove
    `custom_fields.complexity` and allow the index projection to write
    `items.complexity = NULL`.
  * Acquire the same artifact file lock pattern, decode with `mdfront`, set only
    `custom_fields.complexity`, encode, atomically write, and re-upsert a fully
    populated artifact.
  * Do **not** append an `estimate_history` event in this first slice.
    Complexity has no provenance fields yet; adding an audit event would broaden
    size-specific history semantics and belongs with a future complexity
    provenance feature if needed.
  * Add reserved-key preservation so generic create/update paths cannot author
    or drop `complexity` off-seam.
* **Tests:**
  * Valid complexity persists under `custom_fields.complexity` and preserves
    title, status, priority, and body bytes.
  * Invalid non-empty complexity fails before any write.
  * Empty complexity clears the field and leaves no stale SQLite value.
  * Re-applying the same complexity is idempotent.
  * Generic updates preserve an existing complexity value.
* **Acceptance criteria:**
  * Complexity writes are body-preserving and fail-closed on invalid values.
  * The seam does not permit direct sizing or complexity mutation through generic
    custom fields.

### Unit C — Add SQLite schema evolution for complexity

* **Posture:** test-first.
* **Files:** `internal/db/schema_gen.go`, `internal/db/schema_gen_test.go`.
* **Changes:**
  * Add failing tests that prove an existing database upgraded from the previous
    schema receives an `items.complexity` column when header-def defines it.
  * Keep schema extension idempotent for fresh and existing databases.
  * Reuse the existing `ValidateColumnName` guard for every generated extension
    column name.
* **Tests:**
  * Fresh schema contains extension columns derived from WIT metadata.
  * Existing schema without `complexity` gains the column through
    `ApplySchemaExtensions`.
  * A malformed configured field name is rejected before DDL is generated.
* **Acceptance criteria:**
  * Existing workspaces can be upgraded without manual table edits.
  * Extension-column DDL remains injection-safe.

### Unit D — Populate SQLite extension columns from custom fields

* **Posture:** test-first.
* **Files:** `internal/db/queries.go`, `internal/db/upsert_tx.go`,
  `internal/db/upsert_custom_fields_test.go`.
* **Changes:**
  * Add failing red tests for the intended projection behavior before changing
    upsert code. Characterization of the existing gap is supplemental, not a
    replacement for red/green development.
  * Generalize upsert projection so known header-def custom fields, including
    `complexity`, are written into matching extension columns when those columns
    exist.
  * Implement one shared projection helper used by both `db.UpsertItem` and
    transaction upsert (`upsertItemTx` / rehydration).
  * Validate any dynamic extension column identifier through
    `ValidateColumnName`, quote identifiers, and bind values as parameters.
  * Ignore custom fields that are not represented by existing, validated
    extension columns.
  * Keep the canonical source of truth in Markdown frontmatter; SQLite remains a
    disposable index.
* **Tests:**
  * Upsert writes `custom_fields.complexity` to `items.complexity`.
  * Existing size projection is also populated or remains covered by the same
    generalized projection.
  * Non-modeled custom fields remain only in the JSON blob.
  * Malformed field names cannot be projected into dynamic SQL.
  * Direct `UpsertItem`, transaction/rehydration upsert, and
    `SetArtifactComplexity` re-upsert all populate the same projection.
* **Acceptance criteria:**
  * Direct SQL can query `SELECT complexity FROM items`.
  * Full-row upsert still preserves non-extension columns.

### Unit E — Add complexity query filters

* **Posture:** test-first.
* **Files:** `internal/db/queries.go`, `internal/db/queries_complexity_test.go`.
* **Changes:**
  * Add `Complexity string` to `db.QueryFilters`.
  * Treat empty complexity as absent so blank CLI/MCP values cannot produce
    false-empty queries.
  * Filter on the projected `complexity` column using bound SQL parameters,
    following existing status/type/priority patterns.
* **Tests:**
  * Complexity filter returns matching tasks.
  * Omitted or blank complexity filter does not constrain results.
  * Filtering composes with status/type/priority.
  * A malicious-looking value such as `high' OR '1'='1` is treated as a bound
    value and cannot widen results.
* **Acceptance criteria:**
  * Query support is available through the shared DB layer before CLI/MCP
    surfaces are wired.

### Unit F — Add CLI complexity surfaces

* **Posture:** test-first.
* **Files:** `internal/cli/update.go`, `internal/cli/list.go`,
  `internal/cli/update_complexity_test.go`.
* **Changes:**
  * Add `backlogit update --complexity`.
  * Route it through `core.SetArtifactComplexity`.
  * Make `--complexity` mutually exclusive with generic field and section
    updates, matching `--size`.
  * Make `--complexity` mutually exclusive with `--size`,
    `--size-source`, and `--size-ruleset-version` unless a future combined
    metadata seam is designed.
  * Treat `--complexity ""` as clearing the optional field.
  * Add `backlogit list --complexity` and show a `COMPLEXITY` column for human
    list/group surfaces.
  * Validate non-empty list filters against the same enum before querying;
    blank filters are absent.
  * Describe complexity as implementation difficulty/uncertainty, not size or
    priority.
* **Tests:**
  * `update --complexity high` persists and preserves body content.
  * `update --complexity ""` clears the field.
  * Invalid non-empty complexity errors before write and lists valid values.
  * Combining `--complexity` with `--status` fails before write.
  * Combining `--complexity` with `--size` fails before write.
  * `list --complexity high` filters results.
  * `list --complexity bogus` returns a validation error.
* **Acceptance criteria:**
  * CLI update and query behavior is parity-ready for MCP.

### Unit G — Add MCP complexity surfaces

* **Posture:** test-first.
* **Files:** `internal/mcp/tools.go`,
  `internal/mcp/complexity_metadata_test.go`.
* **Changes:**
  * Add `complexity` to `backlogit_update_item`.
  * Route the update through the same body-preserving core seam.
  * Reject mixing complexity with generic fields or sections.
  * Reject mixing complexity with size/provenance fields unless a future
    combined metadata seam is designed.
  * Treat an explicit empty `complexity` argument as a clear/unset operation,
    matching CLI.
  * Add `complexity` to `backlogit_list_items` filters.
  * Validate non-empty complexity filters against the same enum before querying;
    blank filters are absent.
  * Ensure `backlogit_get_wit_metadata` exposes the allowed values through the
    header-def change from Unit A.
  * Update MCP parameter descriptions so agents see the same semantic contract:
    complexity means implementation difficulty and uncertainty, not size or
    priority.
* **Tests:**
  * MCP update persists complexity and preserves unrelated fields.
  * MCP update with empty complexity clears the field and projected column.
  * Invalid complexity maps to `validation_failed` and lists valid values.
  * Mixed complexity plus generic updates is rejected.
  * Mixed complexity plus size/provenance updates is rejected.
  * MCP list filter matches the CLI filter semantics.
  * Invalid MCP list filter maps to `validation_failed`.
* **Acceptance criteria:**
  * MCP and CLI surfaces expose the same mutation and filter contract.

### Unit H — Document semantics and refresh generated references

* **Posture:** docs/config after code surfaces exist.
* **Files:** `docs/cli-reference/backlogit-update.md`,
  `docs/cli-reference/backlogit-list.md`.
* **Changes:**
  * Refresh generated CLI references for `--complexity`.
  * Include the semantic distinction:
    * size = implementation volume
    * complexity = implementation difficulty and uncertainty
    * priority = importance and scheduling urgency
  * Document that default queue ordering does not change in this release.
  * Ensure MCP tool descriptions carry the same semantic language for
    agent-facing discovery.
* **Tests:**
  * Documentation lint or generated-doc validation, if present.
* **Acceptance criteria:**
  * Operator-facing docs match CLI help and WIT metadata.

## Dependency Graph

```text
Unit A
  └─ Unit B
      ├─ Unit C
      │   └─ Unit D
      │       ├─ Unit E
      │       │   ├─ Unit F
      │       │   └─ Unit G
      │       └─ Unit H (after F/G help text exists)
```

Suggested execution order:

1. Unit A
2. Unit B
3. Unit C
4. Unit D
5. Unit E
6. Units F and G
7. Unit H

## Decisions and Rationale

### D1 — Task-only, optional, no default

Complexity follows the size pattern: tasks carry stored values; features and
shipments do not. This avoids false feature-level precision and keeps legacy
tasks valid.

### D2 — Allowed values are lower-case ordinals

The values are `trivial`, `low`, `medium`, and `high`. They are intentionally
not T-shirt sizes. Lower-case words fit the existing priority style and avoid
confusion with size's `XS` through `XL`.

### D3 — No default queue-order change

Priority remains the default scheduling signal. Complexity is exposed for
filters, SQL, and operator-defined ordering but does not alter queue ordering in
this release.

### D4 — No complexity provenance in the first slice

Size provenance exists because automated estimation may need source and ruleset
metadata. Complexity starts as a human/agent-entered planning field. If an
automated complexity estimator appears later, it can add
`complexity_source`, `complexity_ruleset_version`, and a complexity audit event
in a follow-up.

### D5 — SQLite column projection must be explicit

The header-def schema generator can add columns, but queryability requires
write-side projection into those columns. Units C and D make schema evolution
and projection explicit instead of relying on JSON-only `custom_fields`.

## Risks and Caveats

| Risk | Likelihood | Mitigation |
|---|---|---|
| Complexity is confused with size | Medium | Use lower-case difficulty labels and repeat the size/complexity/priority distinction in CLI/MCP descriptions and docs. |
| Generic writes drop or overwrite complexity | Medium | Copy the reserved-key preservation pattern from size and test generic updates. |
| SQLite projection breaks full-row upsert | Medium | Add failing projection tests first, use one shared helper for direct and transactional upserts, and assert non-extension columns survive. |
| CLI and MCP drift | Medium | Add parity tests and wire both surfaces in adjacent units. |
| Queue users expect automatic ordering changes | Low | Document no default ordering change; expose filters first. |

## Constitution Check

| Principle | Result | Notes |
|---|---|---|
| Safety-First Go | pass | Production code remains Go; no `unsafe`; new errors wrap context with `%w`. |
| Test-First Development | pass | Each code unit begins with failing red tests before production code; characterization tests may supplement but not replace red/green evidence. |
| Workspace Isolation and Security Boundaries | pass | All work stays inside the workspace root and writes only tracked project files. |
| CLI Workspace Containment | pass | No out-of-tree file operations are required. |
| Destructive Command Approval | pass | Additive code/config/docs changes need no approval; any rollback/backfill step that deletes, force-rebuilds, or overwrites the SQLite index beyond normal tested sync must use careful mode and operator approval. |
| Merge Commit History Preservation | N/A | Ship/PR lifecycle must merge via merge commit; Stage does not merge. |
| Single Responsibility | pass | No external dependency is added; complexity uses existing config, core, DB, CLI, and MCP layers. |
| Git-Friendly Persistence | pass | Source-of-truth metadata remains human-readable Markdown/YAML; the complexity seam must be body-preserving and SQLite remains rebuildable cache state. |
| Agent Context Efficiency | pass | Query support uses the SQLite index instead of requiring broad file scans. |

Constitution Check: pass

## Plan Hardening Signals

| Signal | Present | Justification |
|---|---|---|
| Public API, schema, or contract change | yes | Adds WIT metadata, CLI flags, MCP tool parameters, and a SQLite query column. |
| Security, auth, permission, or compliance-sensitive behavior | no | No auth, permission, secret, or external trust boundary changes. |
| Migration, backfill, destructive data/config action, or irreversible step | yes | Existing workspaces need a schema extension column and sync/backfill behavior, though the source of truth remains Markdown. |
| External integration, operator checkpoint, or external dependency | no | No new external service or dependency. |
| High runtime, rollout, or rollback risk | no | Runtime surface is local CLI/MCP metadata handling; rollback is removing the additive field and re-syncing the index. |

Requires plan hardening: yes

## Runtime Verification and Closure

| Unit | Runtime surface | Verification | Closure evidence |
|---|---|---|---|
| A | WIT metadata discovery | `backlogit metadata wit task` shows complexity values after sync. | Note WIT contract in closure. |
| B | Core mutation path | Unit tests prove body preservation, validation, and index upsert. | Record setter contract and rollback path. |
| C | SQLite schema | Fresh and upgraded DB tests prove `items.complexity` exists when header-def defines it. | Record schema-extension evidence. |
| D | SQLite projection | SQL query confirms `items.complexity` is populated from frontmatter across direct and transactional upserts. | Record sync/backfill evidence. |
| E | Query layer | DB tests prove complexity filters compose with existing filters and use bound parameters. | Record no default queue ordering change. |
| F | CLI | Manual/automated CLI update, clear, invalid-filter, and list filter checks. | Record CLI help/docs updated. |
| G | MCP | MCP handler tests prove update, clear, invalid-filter, and list filter semantics. | Record CLI/MCP parity. |
| H | Documentation | Docs lint or generated-reference validation. | Record operator-facing semantics. |

Final quality gates for Ship, in order:

```text
go test ./...
go vet ./...
golangci-lint run
gofmt -l .
goimports -l .
go build ./cmd/...
```

Closure evidence must include changed-file summary, command outcomes, runtime
verification results, and any failures or retries.

Rollback trigger: if complexity updates can drop body content, corrupt existing
frontmatter, or make `backlogit list`/`backlogit_update_item` reject unrelated
valid updates, revert the feature changes and run `backlogit sync` to rebuild
the disposable index from Markdown.

## Plan Hardening

Hardening required: yes. The plan adds a machine-readable metadata contract,
mutates artifact frontmatter, extends the SQLite index shape, and changes CLI
and MCP tool contracts. The work is additive and local, but it touches
source-of-truth persistence and derived index projection, so the invariants must
be explicit before plan review.

### Reinforcing context consulted

* `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`:
  generic typed updates can drop unmodeled frontmatter; complexity must use a
  body-preserving seam or be fully modeled.
* `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`:
  re-persist paths must reload from Markdown or fully project new fields.
* `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`:
  CLI, MCP, and index writes must stay in parity.
* `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md`:
  new filters need parity coverage and blank-value handling.
* `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`:
  exact field names, allowed values, and semantics must be specified.
* `docs/compound/2026-06-26-docline-frontmatter-contract.md`:
  frontmatter mutations must be idempotent and body-preserving.

### Protected invariants

* Markdown frontmatter remains the source of truth; SQLite is a disposable,
  rebuildable index.
* `complexity` is optional, task-only, and has no default. An absent value means
  unknown, not medium.
* Valid values are exactly `trivial`, `low`, `medium`, and `high`.
* Complexity writes modify only `custom_fields.complexity` and preserve body
  bytes and unrelated frontmatter keys.
* Generic artifact updates cannot author, overwrite, or drop reserved
  complexity metadata off-seam.
* Explicit empty complexity clears the optional field and NULLs the projected
  index column; invalid non-empty values fail closed.
* Dynamic SQLite extension-column names are validated with
  `ValidateColumnName`; filter values use bound parameters.
* Default queue ordering remains unchanged.
* CLI and MCP mutation/filter semantics remain equivalent.

### Risky actions

#### ProposedAction: Add task complexity WIT metadata

* `summary`: Add task-only `complexity` to generated and live header-def
  metadata.
* `targets`: `internal/config/defaults.go`, `.backlogit/header-def.yaml`,
  related config tests.
* `change_kind`: schema/contract change.
* `rollback`: Remove the field from generated/live metadata and re-run normal
  sync. If rollback requires deleting or force-rebuilding the index, enter
  careful mode and request operator approval first.
* `approval_required`: no.
* `ActionRisk`: moderate.
* `ActionResult`: planned.

#### ProposedAction: Add body-preserving complexity mutation seam

* `summary`: Add `SetArtifactComplexity` and reserve the complexity key from
  generic update paths.
* `targets`: `internal/core/artifact_complexity.go`,
  `internal/core/artifacts.go`, core tests.
* `change_kind`: local edit to persistence path.
* `rollback`: Revert seam and reserved-key changes; existing
  `custom_fields.complexity` values remain inert metadata.
* `approval_required`: no.
* `ActionRisk`: moderate.
* `ActionResult`: planned.

#### ProposedAction: Project complexity into the SQLite index

* `summary`: Add schema extension and index projection so `items.complexity` is
  created and populated from `custom_fields.complexity`.
* `targets`: `internal/db/schema_gen.go`, `internal/db/queries.go`,
  `internal/db/upsert_tx.go`, DB tests, generated schema-extension behavior.
* `change_kind`: index schema/projection change.
* `rollback`: Revert projection/filter changes and run normal sync; the
  Markdown source remains authoritative. If rollback requires deleting or
  force-rebuilding the index, enter careful mode and request operator approval.
* `approval_required`: yes for destructive index rebuild/delete steps; no for
  additive schema/projection code changes.
* `ActionRisk`: moderate.
* `ActionResult`: planned.

#### ProposedAction: Extend CLI and MCP contracts

* `summary`: Add `complexity` update and filter parameters to CLI and MCP.
* `targets`: `internal/cli/update.go`, `internal/cli/list.go`,
  `internal/mcp/tools.go`, CLI/MCP tests and generated reference docs.
* `change_kind`: public local contract change.
* `rollback`: Revert the public parameters and docs; existing metadata remains
  in frontmatter but is no longer first-class.
* `approval_required`: no.
* `ActionRisk`: moderate.
* `ActionResult`: planned.

#### ProposedAction: Rebuild or backfill the SQLite index during rollback

* `summary`: Re-run `backlogit sync` or equivalent rehydration after rollback
  when the disposable SQLite index needs to match Markdown.
* `targets`: `.backlogit/backlogit.db` and derived index state.
* `change_kind`: data/cache rebuild.
* `rollback`: The Markdown source remains authoritative; restore prior code and
  re-run sync from source.
* `approval_required`: yes if the operation deletes, force-overwrites, or
  rebuilds index files; normal non-destructive validation sync may proceed under
  Ship's existing quality-gate workflow.
* `ActionRisk`: moderate, destructive if a delete/force-rebuild command is used.
* `ActionResult`: planned.

### Safety mode for index operations

Ship should use freeze-scope mode for schema/index work: edits are limited to
the complexity WIT, core setter, DB projection/query, CLI/MCP, and generated
reference surfaces named in this plan. If an implementation or rollback step
requires deleting, force-overwriting, or rebuilding `.backlogit/backlogit.db`,
switch to careful mode, enumerate the command and rollback, and obtain operator
approval before execution.

### Added verification and closure detail

Before Ship presents this work as complete, it should verify:

* `backlogit metadata wit task` advertises `complexity` with the exact values
  and aggregate types do not.
* `backlogit update <task> --complexity high` writes
  `custom_fields.complexity` without changing body bytes.
* `backlogit list --complexity high` returns only matching items and an omitted
  or blank complexity filter behaves as absent; invalid non-empty filters fail
  validation.
* `backlogit_update_item` and `backlogit_list_items` MCP tests prove parity with
  CLI semantics.
* A direct read-only SQL query can select `complexity` from `items` after sync.
* Existing `size` update and composition tests still pass, proving the new
  metadata path did not regress size.

Operational closure should record:

* final value vocabulary and semantics
* evidence that default queue ordering did not change
* evidence that SQLite can be rebuilt from Markdown with `backlogit sync`
* rollback command: revert the feature commit(s), then run `backlogit sync`
* validation window: one local Ship verification pass before PR review; no
  post-deploy monitoring is required beyond CI and CLI/MCP runtime checks

### Operator decision points

No operator decision blocks Stage harvest. The recommended default is:

* `complexity` values: `trivial`, `low`, `medium`, `high`
* task-only, optional, no default
* no provenance fields in the first slice
* no default queue-ordering change

If the operator later prefers a different vocabulary or wants aggregate
`complexity_composition`, those should be separate follow-up backlog items
rather than changes to this Stage output.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Gate rationale: the first review pass found blocking gaps in schema evolution,
red/green test requirements, SQLite projection coverage, reserved metadata
conflict handling, final quality gates, and safety-mode treatment for index
rebuilds. The plan was revised in place to address those gaps before harvest.
The final plan now has explicit schema-extension coverage, a shared direct and
transactional upsert projection helper, enum/clear semantics, size/complexity
mutual exclusion, MCP/CLI parity, strict test-first posture, final Go gates, and
safety-mode/approval boundaries for destructive index rebuilds.

Plan hardening required: yes.

Plan hardening satisfied: yes. The `## Plan Hardening` section records protected
invariants, `ProposedAction` / `ActionRisk` entries, verification additions,
rollback boundaries, and operator decision points.

Personas covered:

* Constitution Reviewer
* Go Reviewer
* Scope Boundary Auditor
* Learnings Researcher
* Architecture Strategist
* Agent-Native Parity Reviewer
* Security Lens Reviewer

### Findings

#### P0

None.

#### P1

All blocking findings were addressed before this final gate decision:

* Added explicit schema evolution for existing databases before projection work
* Replaced characterization-first Unit C posture with test-first red/green
  requirements
* Added final Go quality gates and structured closure evidence
* Added careful/freeze-scope guidance and approval boundaries for destructive
  index rebuilds
* Removed first-slice `estimate_history` complexity audit event scope
* Added size/complexity mutual-exclusion requirements
* Required one shared projection helper for direct and transactional upserts
* Defined empty complexity clear/unset behavior

#### P2

Addressed before final gate decision:

* Added explicit projection mapping and `ValidateColumnName` identifier safety
* Added invalid non-empty filter validation and bound SQL parameter tests
* Added MCP parameter description parity
* Added no-default-queue-ordering and no first-slice provenance boundaries

#### P3

Advisory items accepted:

* Generated references and MCP descriptions should repeat the
  size/complexity/priority distinction
* Validation errors should list allowed values for agent self-correction
* Ship should keep task slices at or below the 2-hour rule during harvest

Runtime verification and operational closure gaps: none blocking after the
revision. Ship must carry forward the final quality gates and runtime
verification matrix in this plan.
