---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for one-way, plan-then-apply Azure DevOps work item synchronization of backlogit artifacts selected by shipment, with durable frontmatter identity and idempotent apply.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-20-azure-devops-sync-plan.md
title: 'Azure DevOps work item synchronization (v1, push-only)'
---

## Purpose

Implement `backlogit ado` — a one-way, operator-initiated, plan-then-apply
synchronization capability that projects backlogit artifacts into Azure DevOps
Boards work items using a declarative target-mapping configuration, while
backlogit's Markdown files remain authoritative.

Source spike: `docs/decisions/2026-08-20-azure-devops-sync-spike.md`
(conclusion `proceed`, confidence `medium`). Every decision recorded there is
settled input to this plan; this document adds the code map, schemas, call
sequences, surfaces, and the atomic work breakdown.

This revision incorporates the seven-persona `plan-review` gate. Findings and
their disposition are recorded in `## Plan Review` at the end; the body below is
the remediated plan, not the reviewed draft.

## The v1 Contract

| Dimension | v1 |
|---|---|
| Direction | Push only: backlogit to Azure DevOps |
| Authority | Markdown is authoritative for every mapped field |
| Remote reads | Identity, revision, and drift only; never write a backlogit field |
| Mapped unit | One artifact maps to one work item |
| Selection scope | Shipment or explicit item list |
| Trigger | Explicit `plan` then `apply`; never a lifecycle hook |
| Providers | Azure DevOps only |
| Deletion | Never; `plan` and `apply` emit no deletion action. `status` and `verify` report `orphan_remote` for ineligible locals that still carry a live link |
| Credentials | PAT from a prefix-restricted environment variable only |
| Target hosts | Allowlisted Azure DevOps hosts only |
| Text format | HTML only; raw inline HTML in artifact bodies is escaped |
| API version | Pinned `7.1` on every request |

## Non-Goals

* Import, pull, or bidirectional merge from Azure DevOps into backlogit
* Any Jira or GitHub Issues/Projects implementation
* Projecting a shipment itself as a work item, an Epic, or an Iteration
* Automatic synchronization on status transition, ship, webhook, or timer
* Remote deletion, recycle-bin operations, or destroy
* Attachments, `System.History` comments, templates, boards, or saved queries
* Organization-level process introspection (`_apis/work/processes`)
* An arbitrary SQL selection mode (`--query`), cut during review as unowned
  scope with a non-authoritative data source
* A selectable Markdown passthrough mode, cut during review as an unrequested
  knob that also destabilizes idempotency

## Assumptions

* The target Azure DevOps project exists, and its area and iteration paths
  exist. backlogit never creates classification nodes.
* The operating identity holds a PAT with `vso.work_write`.
* `models.ArtifactStatus` remains the ten-value enum; a new status requires a
  `state_map` entry, which the config validator enforces, subject to the
  load-time upgrade rule in U2.
* Workspaces may be `.backlog/` (default) or legacy `.backlogit/`; every path
  resolves through `core.WorkspaceStorageRoot`, never a literal directory name.

## Problem Frame

backlogit owns a configurable artifact hierarchy, a governed mutation model, and
a Markdown-authoritative persistence contract, but has no way to project that
backlog into the Agile platform a customer plans in. The reference importer
(`azd-backlogloader`) proves the demand and the mapping shape, but it is a
one-shot Python script with no idempotency, no update path, no concurrency
control, no resume, no durable remote identity, and a credential in a config
file.

Three concrete gaps in `main`:

1. **No provider contract.** `config.FieldConfig.ExternalMap` is declared for
   external systems and `core.TranslateExternalMap` (`internal/core/fields.go`)
   implements a translation step, but the function has zero callers.
2. **No durable remote identity.** `models.Artifact` carries a `Commit` scalar
   but nothing identifying a remote work item, and the SQLite index is an
   ephemeral cache excluded by `.gitignore`, so it cannot hold identity.
3. **No outbound write path with a result.** `hooks.WebhookNotifier` is the only
   outbound HTTP in the domain layer. Its results are unobservable to the caller
   — `dispatchToEndpoint` returns before its goroutine completes and
   `DispatchWithEventType` returns `nil` unconditionally — though goroutines are
   drained at workspace shutdown. It cannot report success, order writes, or
   maintain state.

## Current-State Code Map

| Path | Symbol | Role in this plan |
|---|---|---|
| `internal/models/artifact.go` | `Artifact`, `ToFrontmatterMap`, `CustomFields` | Carrier for the durable link record; no typed-model change |
| `internal/models/artifact.go` | `ArtifactStatus` (10 values) | Domain of `state_map` keys |
| `internal/models` | `NowUTC` | Single stamping helper for `synced_at` |
| `internal/config/schema.go` | `WorkspaceConfig.ArtifactTypes`, `Fields`, `FieldConfig.ExternalMap` | Configurable type/status source; `ExternalMap` becomes a fallback |
| `internal/config/schema.go` | `validateGateBinary` | Precedent: repo-controlled config is untrusted input |
| `internal/config/migration.go` | `MigrationConfig`, `LoadMigrationConfig` | Left byte-unchanged; inbound-only |
| `internal/core/fields.go` | `TranslateExternalMap`, `ValidateFields` | Gains its first caller plus a nil guard |
| `internal/core/mutation_envelope.go` | `MutationEnvelope`, `MutationStep` | Wraps the local write triple after each remote write |
| `internal/errors/mutation_errors.go` | `MutationPartialError`, `IsWriteIndeterminate` | Local durability vocabulary, preserved verbatim |
| `internal/mcp/gate_errors.go` | `durabilityOutcomeResult` | Reused for local durable-write outcomes on MCP |
| `internal/core/shipment.go` | `CreateShipment`, `GetShipment`, `custom_fields.items` | Shipment as the selection scope |
| `internal/core/artifacts.go` | `findArtifact`, `UpdateArtifact`, `persistArtifact` | Markdown-first load and write path for the link record |
| `internal/core/events_writer.go` | `appendItemEvent`, `NewWorkspaceEventWriter` | Per-item JSONL audit trail |
| `internal/core/workspace.go` | `Workspace`, `WorkspaceStorageRoot`, `SafeResolve` | Path resolution and containment |
| `internal/core/task_lock.go` | task-lock primitives | Pattern source for the apply lease |
| `internal/db/schema.go` | `EnsureSchema`, `IntrospectSchema` | Home of the `external_links` projection |
| `internal/hooks/webhook.go` | `WebhookNotifier` | Pattern source for rate limiting; not reused as transport |
| `internal/release/release.go` | `Client` | Nearest precedent for an outbound HTTP leaf |
| `internal/mcp/server.go` | `addTool`, `RegisterTools`, `Server.Events`, `InvokeTool` | MCP registration and shared event writer |
| `internal/cli/root.go` | `newRootCommandImpl` | CLI registration seam |
| `internal/cli/registry_parity_test.go` | `TestRegistryParity_*` | Parity assertions extended for the new operations |
| `.autoharness/backlog-registry.yaml` | `operations`, `cli_only_flags` | Declares operations and deliberate CLI-only levers |
| `go.mod` | `russross/blackfriday/v2` (indirect) | Promoted to direct for Markdown to HTML |

## Architecture and Dependency Direction

```text
cmd/backlogit
  -> internal/cli ------------\
                               >-- internal/core -- internal/extsync -- internal/ado
  -> internal/mcp ------------/          |                 |                 |
                                         v                 v                 v
                                    models, db        models, config    net/http, x/time/rate,
                                    events, errors    errors            internal/errors
```

Rules:

* The computation package is named **`internal/extsync`**, not `internal/sync`.
  Naming it `sync` would shadow the standard library in every consumer
  (`internal/core` and `internal/mcp` both import stdlib `sync`) and collide
  with the existing `backlogit sync` verb.
* `internal/extsync` is **core-independent and filesystem-free**. It holds pure
  computation — value types, mapping, hashing, ordering, and action computation
  — plus exactly one **translation-only** provider adapter (U15) that converts a
  `MappedItem` plus `WriteOptions` into `internal/ado` calls and converts the
  responses back into `extsync` value types. The adapter carries no mapping and
  no policy logic, and it performs the network round trip only by delegating to
  `internal/ado`; the package itself opens no files and constructs no paths.
  Plan, snapshot, and ledger persistence live in `internal/core`, which already
  owns `SafeResolve` and `WorkspaceStorageRoot`. That is what makes the declared
  `core -> extsync -> ado` direction cycle-free; the reviewed draft placed
  persistence in that package and would have required `extsync -> core`.
* `internal/ado` is a near-leaf: standard library plus `golang.org/x/time/rate`
  plus `internal/errors`. It is not a strict leaf like `internal/release`, which
  imports nothing internal, and U37 documents the distinction rather than
  claiming a false equivalence.
* `internal/extsync` never imports `internal/core`. `internal/mcp` never imports
  `internal/cli`. A `go list -deps` test asserts both.

New packages and files:

```text
internal/ado/       client.go auth.go errors.go retry.go patch.go
                    workitems.go discovery.go wiql.go
internal/extsync/   provider.go types.go azuredevops.go validate.go
                    hash.go mapping.go text.go tags.go hierarchy.go plan.go
internal/core/      external_link.go external_sync_select.go external_sync_plan.go
                    external_sync_apply.go external_sync_local.go
                    external_sync_store.go external_sync_progress.go
                    external_sync_verify.go external_sync_events.go
                    external_sync_lease.go
internal/db/        external_links.go
internal/cli/       ado.go ado_apply.go
internal/mcp/       ado_tools.go
internal/config/    azuredevops.go azuredevops_map.go
internal/errors/    external_sync_errors.go
```

## Configuration Schema

New file: `{workspace}/integrations/azure-devops.yaml`, loaded by
`config.LoadAzureDevOpsConfig(workspacePath)`. `migration.yaml` is not touched.

```yaml
schema_version: "1.0"
provider: azure_devops

connection:
  organization_url: https://dev.azure.com/contoso
  project: Platform
  api_version: "7.1"

auth:
  method: pat
  pat_env: BACKLOGIT_ADO_PAT

agent_apply:
  enabled: false

defaults:
  area_path: 'Platform\Backlog'
  iteration_path: 'Platform\Backlog'
  correlation_tag_prefix: 'backlogit-key:'
  suppress_notifications: true

selection:
  eligible_statuses: [queued, active, blocked, review, done]
  exclude_types: [deliberation, spike, review, shipment]

type_map:
  feature:
    work_item_type: Feature
    field_map:
      title: System.Title
      description: System.Description
      priority: Microsoft.VSTS.Common.Priority
      assigned_to: System.AssignedTo
    value_map:
      priority: { critical: 1, high: 2, medium: 3, low: 4 }
  task:
    work_item_type: User Story
    field_map:
      title: System.Title
      description: System.Description
      acceptance_criteria: Microsoft.VSTS.Common.AcceptanceCriteria
  subtask:
    work_item_type: Task
    field_map:
      title: System.Title
      description: System.Description

state_map:
  queued: New
  active: Active
  blocked: Active
  review: Active
  done: Closed
  accepted: Closed
  rejected: Removed
  shipped: Closed
  archived: Closed
  abandoned: Removed

hierarchy:
  parent_relation: System.LinkTypes.Hierarchy-Reverse
  require_parent_synced: true

limits:
  batch_read_size: 200
  wiql_chunk_size: 50
  requests_per_second: 5
  request_timeout_ms: 30000
  run_deadline_seconds: 1800
  retry:
    max_attempts: 5
    initial_backoff_ms: 500
    max_backoff_ms: 30000
```

`state_map` is a single top-level block with a built-in default, not a
per-type block written into every workspace file. The reviewed draft duplicated
the full ten-status map under each type, which made every opted-in workspace a
persisted-explicit-map case that hard-fails when a new `ArtifactStatus` is
added. A per-type `state_map` override remains accepted; when absent, the
top-level map applies, and when the top-level map is absent the built-in default
applies. U2 owns the load-time upgrade rule for a persisted map that exactly
equals a frozen prior default.

### Validation rules (`AzureDevOpsConfig.Validate`)

Security-relevant rules first. The config file is repo-controlled and
auto-loaded, so it is **untrusted input** — the same reasoning
`validateGateBinary` already applies to `autoharness_binary`.

* `connection.organization_url` must be absolute `https` **and** its host must
  match the allowlist: exactly `dev.azure.com`, or a `*.visualstudio.com`
  subdomain, or a host present in an operator-managed
  `BACKLOGIT_ADO_ALLOWED_HOSTS` environment allowlist for on-premises Azure
  DevOps Server. A host outside the allowlist is rejected. Without this, a
  pull request can retarget the credential at an attacker-controlled host.
* `auth.pat_env` must match `^BACKLOGIT_ADO_[A-Z0-9_]*$`. Without this, a
  repo-controlled file can name any environment variable — `AWS_SECRET_ACCESS_KEY`,
  `GITHUB_TOKEN` — and the client will place its value in an `Authorization`
  header.
* `auth.method` must be exactly `pat`. Any other value, including
  `entra_service_principal`, is rejected with the ordinary unknown-value error.
  No placeholder sentinel is defined for an unimplemented method.
* A `personal_access_token` key anywhere in the file is rejected with a message
  naming `pat_env`.
* `defaults.correlation_tag_prefix` must match `^[A-Za-z0-9._:-]+$`. Quotes
  would break WIQL literal quoting; a semicolon is the `System.Tags` separator.
* `defaults.area_path` and `defaults.iteration_path` must be non-empty and must
  not contain a quote character.

Remaining rules:

* `connection.project` non-empty; `api_version` matches `^\d+\.\d+$`
* `type_map` non-empty; every key names a configured artifact type in
  `WorkspaceConfig.ArtifactTypes`
* The effective `state_map` covers every `models.ArtifactStatus` reachable under
  `selection.eligible_statuses`
* `batch_read_size` within `[1, 200]`; `wiql_chunk_size` within `[1, 100]`
* `requests_per_second` within `[1, 50]`
* `request_timeout_ms` within `[1000, 120000]`, and `internal/ado` applies
  `min(configured, packageHardCap)` so a config can lower but never raise it
* `run_deadline_seconds` within `[60, 21600]`
* `retry.max_attempts` within `[1, 10]`
* `agent_apply.enabled` defaults to `false` when absent

## Durable Data Schema

### Artifact frontmatter (Git-tracked, authoritative)

```yaml
custom_fields:
  external:
    key: 4b7e29f1a83d6c05e1d8a2f6b90c3a71
    azure_devops:
      org: contoso
      project: Platform
      work_item_id: 4821
      work_item_type: User Story
      rev: 7
      content_hash: 'sha256:9f2c1a4e...'
      synced_at: 2026-08-20T18:04:11Z
      link_state: linked
```

`custom_fields.external.key` is a **stable external correlation key**, minted
once on first link and never rewritten. It is **128 bits of `crypto/rand`
entropy encoded with `encoding/hex`**, so its serialized form is always exactly
**32 lowercase hexadecimal characters** matching `^[0-9a-f]{32}$`. Both packages
are standard library, so the key adds no dependency. It is deliberately not the
artifact ID: the repository's `adopt` and re-parent operations rewrite artifact
IDs (`AdoptItemResult.NewID`, `RewrittenArtifactIDs`), so an ID-derived
correlation tag would silently orphan or duplicate a work item after a
re-parent. The remote correlation tag is `backlogit-key:{key}`. The artifact ID
is written as a **secondary, non-authoritative** human-readable tag
(`backlogit-id:{artifactID}`) that is refreshed on update and never used for
matching.

Collision handling is local and conservative. Before a minted key is persisted,
the minter checks it against the durable keys already present in the workspace —
the frontmatter link records, read through the `external_links` projection when
it is fresh and through a frontmatter scan otherwise — and regenerates on any
match. At 128 bits a collision is not expected; the check exists so the
immutability invariant cannot be violated by an improbable event or by a
restored backup. Duplicate keys *remote* to the workspace are not handled by
minting at all: two remote work items carrying the same correlation tag are the
multiplicity case, and `Resolve` reports them as many-match, which produces a
`conflict` action rather than a silent pick.

`synced_at` is stamped with `models.NowUTC()` so bytes are machine-independent
and end in `Z`.

`link_state` is one of `linked`, `pending_verify`, `conflict`.

### Per-item JSONL audit (Git-tracked, append-only)

Appended through the existing `appendItemEvent` seam so events land in the item
log and project into `item_log_entries` with FTS.

| `event_type` | `delta` payload |
|---|---|
| `external_sync_planned` | `plan_id`, `action`, `provider`, `work_item_id?` |
| `external_sync_applied` | `plan_id`, `action`, `work_item_id`, `rev`, `content_hash` |
| `external_sync_conflict` | `plan_id`, `work_item_id`, `expected_rev`, `remote_rev` |
| `external_sync_pending_verify` | `plan_id`, `action`, `external_key`, `reason` |

### SQLite projection (ephemeral, rebuilt)

```sql
CREATE TABLE IF NOT EXISTS external_links (
    item_id        TEXT NOT NULL,
    provider       TEXT NOT NULL,
    external_key   TEXT NOT NULL,
    org            TEXT NOT NULL,
    project        TEXT NOT NULL,
    remote_id      TEXT NOT NULL,
    remote_type    TEXT,
    remote_rev     INTEGER,
    content_hash   TEXT,
    link_state     TEXT NOT NULL,
    synced_at      DATETIME,
    PRIMARY KEY (item_id, provider)
);
CREATE INDEX IF NOT EXISTS idx_external_links_remote
    ON external_links (provider, org, project, remote_id);
CREATE INDEX IF NOT EXISTS idx_external_links_key
    ON external_links (external_key);
CREATE INDEX IF NOT EXISTS idx_external_links_state
    ON external_links (link_state);
```

Populated by `Rehydrate` from frontmatter and by the apply loop's compensable
upsert step. Never a source of truth. The apply loop also re-upserts the
**`items`** row in the same operation, because the link-record write mutates
`custom_fields` and `updated_at`.

### Ephemeral runtime state (gitignored)

```text
{workspace}/runtime/ado-discovery.json                    schema snapshot
{workspace}/runtime/ado-plans/{plan_id}.json              frozen execution spec
{workspace}/runtime/ado-plans/{plan_id}.progress.jsonl    resume ledger (advisory)
{workspace}/runtime/ado-plans/.lease                      apply lease
```

`.backlogit/runtime/` is already gitignored; U3 adds `.backlog/runtime/`.

## Sync State Machine

```text
                        plan
  unlinked ───────────────────────────► create      ──apply──► linked
     │  (no correlation match)
     ├── exactly one tag match ───────► adopt       ──apply──► linked
     └── two or more tag matches ─────► conflict    (halt item, report)
     └── correlation lookup truncated ► blocked     (halt PLAN, fail closed)

  linked, remote_rev == stored rev
     ├── content_hash changed ────────► update      ──apply──► linked
     └── content_hash unchanged ──────► noop

  linked, remote_rev != stored rev ───► conflict    (halt item, report)
       (override at PLAN time only: CLI --allow-remote-drift, approval-gated,
        freezes skip_revision_test into the action ──► update)

  parent required but not yet linked ─► deferred    (child skipped this run)
  linked, remote id absent ───────────► orphan_local (report)

  apply outcome unknown (5xx, timeout, or transport error after the request
  may have been sent, on any action) ──────► pending_verify (no retry)
                                                  └── verify ──► linked | conflict
```

`orphan_remote` is **not** a state this machine reaches. An artifact whose
status is archived, abandoned, or shipped is outside `selection.eligible_statuses`,
so `plan` never selects it and can never report it. Reporting an ineligible
local that still carries a live remote link is the report-only responsibility of
`backlogit ado status` and `backlogit ado verify`, which scan the durable
`external_links` projection and artifact frontmatter independently of push
eligibility. Neither command, and no plan action, ever proposes a deletion.

Fixed vocabularies. These are producer contracts consumed by agents through
literal field match, so they are asserted by test and reproduced verbatim in the
U37 design doc:

| Field | Closed value set |
|---|---|
| `action` (plan entry) | `create`, `update`, `adopt`, `conflict`, `noop`, `deferred`, `blocked` |
| `outcome` (apply entry, progress ledger) | `applied`, `skipped`, `conflict`, `deferred`, `failed`, `pending_verify` |
| `class` (failure entry, `ado` error classification) | `remote_drift`, `validation`, `not_found`, `throttled`, `indeterminate`, `permanent` |
| `link_state` (frontmatter, projection) | `linked`, `pending_verify`, `conflict` |
| `code` (warning entry) | `orphan_remote`, `orphan_local`, `correlation_truncated`, `discovery_snapshot_stale`, `projection_stale` |
| `error` (partial apply payload) | `external_sync_partial` |
| summary keys | `created`, `updated`, `adopted`, `noop`, `deferred`, `conflicts`, `failed`, `pending_verify` |

There is exactly one `class` vocabulary, shared by `internal/ado` and the apply
result. The reviewed draft carried two (`drift`/`permanent` in the client,
`remote_drift`/`validation` in the result); they are now the same tokens.

Two `class` values are terminal by construction and never enter the retry loop:
`validation` and `permanent` are non-retryable 4xx outcomes that a repeat
request cannot change. `throttled` is the only retryable class; `remote_drift`
is a per-item conflict; `indeterminate` is the no-blind-retry class that becomes
`pending_verify`. The `code` vocabulary is shared across surfaces, but
`orphan_remote` is emitted only by `status` and `verify`; `plan` can never
produce it.

## Plan, Apply, Retry, Resume Flow

### `plan`

`plan` performs zero writes to Azure DevOps. Orchestration lives in
`internal/core` (U19a, U19b), not in `internal/cli`.

1. Load workspace, `AzureDevOpsConfig`, and resolve the selection
   (`--shipment` or `--item`), expanding to ancestors. Expanded ancestors are
   **visible planned actions** subject to the same `eligible_statuses` and
   `exclude_types` filters; an ancestor excluded by policy makes its descendants
   `deferred`, never silently orphaned.
2. Load `ado-discovery.json`; refresh when absent, older than
   `--max-discovery-age` (default 24h), or `--refresh` is set.
3. Validate the mapping against the snapshot. Any unknown work item type, state,
   field reference name, area path, or iteration path fails the whole plan with
   a per-entry diagnostic. Fail closed.
4. Batch-read linked artifacts: chunk remote ids at `limits.batch_read_size`,
   `POST /_apis/wit/workitemsbatch` with
   `fields: [System.Id, System.Rev, System.State, System.Tags]` and
   `errorPolicy: Omit`. Omitted ids become `orphan_local`.
5. Resolve correlation for unlinked artifacts **scoped to the selection**, not
   project-wide: chunk the selected external keys at `limits.wiql_chunk_size`
   and issue one WIQL per chunk with an explicit
   `[System.Tags] CONTAINS 'backlogit-key:<key>'` disjunction, then batch-read
   `System.Tags` for the returned ids and match exact tags client-side. If any
   chunk returns a result count equal to its `$top`, the lookup is **truncated**:
   emit `correlation_truncated` and mark every affected artifact `blocked`. A
   `create` is never proposed on uncertain correlation coverage. The reviewed
   draft used one project-wide prefix query whose truncation could silently
   produce duplicate creates.
6. Compute the per-artifact action.
7. Freeze and persist the execution spec (below) and emit `external_sync_planned`
   per artifact.

### The plan file is a frozen execution contract

`apply` executes the persisted specification and synthesizes nothing. The plan
file therefore carries, per action, the **fully resolved** payload rather than
inputs to be re-derived:

* the resolved field set (reference name to value, post mapping, post
  HTML rendering)
* the resolved tag string
* the resolved parent `RemoteRef`, or a reference to the in-plan action that
  produces it
* `expected_rev` for updates
* `skip_revision_test`, the frozen boolean that records whether the `/rev` test
  operation is emitted for this action. It is `false` unless the plan was
  computed with the approval-gated `--allow-remote-drift`, in which case the
  approving operator's recorded reason is frozen alongside it at plan scope
* the artifact's `content_hash` at plan time

and, at plan scope, semantic digests that bind the plan to its inputs:
`config_digest`, `discovery_digest`, `workspace_target` (org plus project), and
`selection_digest`. `apply` refuses when any digest no longer matches. A
changed mapping can no longer alter an already reviewed write.

`plan_id` is `ado-{content digest of the frozen spec}` — content-addressed, so
an identical re-plan yields an identical id. The run timestamp is a separate
`created_at` field, not part of the id.

Because `skip_revision_test` and the approval reason are part of the frozen
spec, a drift-overriding plan is a **different plan** with a different
`plan_id` than the same selection planned normally. `apply` has no lever that
can add, remove, or weaken a revision test; it executes what was reviewed.

### `apply`

`apply --plan <id>` refuses to run when the plan file is missing, when any
digest mismatches, when a local artifact's `content_hash` differs from the
recorded value, or when the plan contains `blocked` entries.

`apply` first acquires a workspace-scoped **apply lease**
(`runtime/ado-plans/.lease`, the existing task-lock pattern with a heartbeat and
a stale TTL). Two concurrent applies in one workspace would otherwise both see
an unlinked artifact and both issue a create; Azure DevOps has no create-if-absent
primitive, so the correlation preflight cannot serialize them. The lease is held
for the run and refreshed across network I/O. The guarantee is explicitly
scoped to **one shared workspace**; independently cloned workspaces are not
serialized, and the plan says so rather than claiming global idempotency.

Per artifact, in parent-before-child order (deterministic: depth ascending, then
artifact ID):

1. Take the frozen patch payload from the plan.
2. Issue exactly one remote write. Relations are included in the create body so
   a new child costs one request and one revision.
3. On success, execute the local write triple inside `core.MutationEnvelope`:
   * `external-link-frontmatter` — write `custom_fields.external`
     (compensable: restore the prior value)
   * `external-link-projection` — upsert `external_links` **and** re-upsert the
     `items` row (compensable: restore both)
   * `external-link-event` — append the JSONL audit record (append-only,
     sequenced last, never compensated)
4. Append the outcome to `{plan_id}.progress.jsonl`. A ledger append failure is
   wrapped with `%w` and surfaced as a per-artifact `failed` outcome; it is
   never discarded.

Failure handling is per artifact and never cross-artifact:

| Remote outcome | `class` | Action |
|---|---|---|
| 4xx indicating rev or concurrency failure | `remote_drift` | Record `conflict`, continue |
| 4xx field or rule validation | `validation` | Record `failed`, not retryable, continue |
| 404 | `not_found` | Record `orphan_local`, continue |
| 429 | `throttled` | Honor `Retry-After`, retry within the budget |
| 5xx, timeout, transport error after send | `indeterminate` | Record `pending_verify`, no retry, continue |
| Attempt budget exhausted with a terminal 429, and no earlier attempt was indeterminate | `throttled` | Record `failed`, retryable, continue |
| Attempt budget exhausted with a terminal 5xx, timeout, or transport failure, on any action | `indeterminate` | Record `pending_verify`, no retry, continue |
| Any attempt in the sequence was indeterminate, whatever the terminal attempt was | `indeterminate` | Record `pending_verify`, no retry, continue |
| Unclassifiable 4xx | `permanent` | Record `failed`, not retryable, continue |

Classification follows the **terminal cause**, with one dominance rule:
indeterminacy is sticky. A 429 is a rejection the server never applied, so an
exhausted throttling sequence stays `throttled` and stays retryable regardless
of the action. A timeout, a 5xx, or a transport failure after the request may
already have been sent stays `indeterminate`, becomes `pending_verify`, and is
never blindly retried — for a `create` because the work item may already exist,
for an `update` because the patch may already have landed. Once any attempt in
a sequence is indeterminate, no later attempt can downgrade the outcome back to
a retryable class. `validation` and `permanent` outcomes never enter the retry
loop at all; they are recorded and the loop moves to the next artifact.

The reviewed draft classified a budget-exhausted `update` as `permanent` while
marking it retryable, which is self-contradictory: `permanent` is by definition
the non-retryable class. It also made a budget-exhausted `create` retryable,
which let `--resume` re-issue a create whose earlier attempt may already have
persisted.

### `retry` and `resume`

There is no separate retry verb. `apply --plan <id> --resume` re-reads the
progress ledger, skips artifacts already applied, and re-attempts `failed`
entries marked retryable. Two rules make resume safe:

* The **frontmatter link record is authoritative** for resume; the ledger is an
  optimization. A deleted or truncated ledger cannot cause a duplicate create.
* Before re-issuing any `create`, resume **re-resolves the correlation key** for
  the affected artifacts and converts a match into `adopt`.

`pending_verify` entries are owned by `verify`, not by `apply`.

### `verify`

`verify` resolves `pending_verify` artifacts and performs **semantic
confirmation**, not mere existence checking. A correlation tag proves a work
item exists; it does not prove an update landed. For each pending artifact:

1. Resolve the external key to remote refs.
2. Zero matches: remain `pending_verify` with a diagnostic.
3. Two or more matches: `conflict`.
4. Exactly one match: read the remote fields named in the frozen plan payload
   and compare. Only when the remote matches the intended result does the
   artifact become `linked` with a re-baselined `rev` and `content_hash`.
   Otherwise the artifact becomes `conflict` and a re-plan is required.

`verify` never re-baselines a hash merely because a remote id was found.

`verify` additionally performs the **report-only orphan scan** it shares with
`status`: it walks every durable link record in the workspace, including
artifacts whose status is archived, abandoned, or shipped and therefore outside
`selection.eligible_statuses`, and emits an `orphan_remote` warning for each one
whose remote work item still exists. `status` and `verify` are the only
surfaces that report `orphan_remote`, because push selection structurally
excludes those artifacts. The warning is informational: `verify` proposes no
action, and no command in this feature deletes or closes a remote work item.

## Azure DevOps REST Call Sequence

All requests pin `?api-version=7.1` and carry
`Authorization: Basic base64(":" + PAT)`.

| Step | Method and path | Notes |
|---|---|---|
| Discover types | `GET /{org}/{project}/_apis/wit/workitemtypes` | Cached to snapshot |
| Discover states | `GET .../workitemtypes/{type}/states` | Per mapped type |
| Discover fields | `GET .../workitemtypes/{type}/fields`, `GET .../wit/fields` | Reference names, `readOnly` |
| Discover areas | `GET .../wit/classificationnodes/areas?$depth=10` | Validates `area_path` |
| Discover iterations | `GET .../wit/classificationnodes/iterations?$depth=10` | Validates `iteration_path` |
| Read linked | `POST .../wit/workitemsbatch` | ids chunked at 200, `errorPolicy: Omit` |
| Correlate unlinked | `POST .../wit/wiql` (chunked) then `POST .../wit/workitemsbatch` | WIQL returns ids only |
| Create | `POST .../wit/workitems/${type}` | `application/json-patch+json`, success is **200** |
| Update | `PATCH .../wit/workitems/{id}` | First op is `{"op":"test","path":"/rev","value":N}` unless the frozen action carries `skip_revision_test` |

Create body, including the parent relation and both tags in one request:

```json
[
  {"op":"add","path":"/fields/System.Title","value":"Durable dependency type"},
  {"op":"add","path":"/fields/System.Description","value":"<p>...</p>"},
  {"op":"add","path":"/fields/System.State","value":"New"},
  {"op":"add","path":"/fields/System.AreaPath","value":"Platform\\Backlog"},
  {"op":"add","path":"/fields/System.IterationPath","value":"Platform\\Backlog"},
  {"op":"add","path":"/fields/System.Tags","value":"backlogit-key:4b7e29f1a83d6c05e1d8a2f6b90c3a71; backlogit-id:144.004-T; harness"},
  {"op":"add","path":"/relations/-","value":{
      "rel":"System.LinkTypes.Hierarchy-Reverse",
      "url":"https://dev.azure.com/contoso/_apis/wit/workItems/4800",
      "attributes":{"comment":"backlogit parent 144-F"}}}
]
```

Update body:

```json
[
  {"op":"test","path":"/rev","value":7},
  {"op":"add","path":"/fields/System.Title","value":"Durable dependency type"},
  {"op":"add","path":"/fields/System.State","value":"Active"},
  {"op":"add","path":"/fields/System.Tags","value":"backlogit-key:4b7e29f1a83d6c05e1d8a2f6b90c3a71; backlogit-id:144.004-T; harness; triage"}
]
```

Client rules encoded in `internal/ado`:

* The literal `$` in the create path is preserved; only the type name is
  percent-encoded, so `User Story` becomes `$User%20Story`.
* Success on create is HTTP 200. A client checking 201 is wrong.
* `op: add` on `/fields/{ref}` behaves as an upsert and is used uniformly.
* **Redirects are not followed across hosts.** `http.Client.CheckRedirect`
  rejects any redirect whose host leaves the allowlisted origin, and strips the
  `Authorization` header on any redirect it does follow. Go's default client
  would otherwise forward the PAT to a redirect target.
* Every request carries `limits.request_timeout_ms`, capped by a package-level
  hard cap that a config can lower but never raise. Discovery uses a shorter
  probe timeout because it is the first call and gates everything after it. The
  whole run is additionally bounded by `limits.run_deadline_seconds`.
* The rate limiter is a **field on `ado.Client`**, never a package global, and
  every wait is `limiter.Wait(ctx)` with the request context. Backoff and
  `Retry-After` sleeps use `select { case <-ctx.Done(): ...; case <-timer.C: }`,
  never `time.Sleep`. The `WebhookNotifier` pattern is the shape, not the
  context handling: it deliberately waits on `context.Background()`, which would
  make a synchronous apply loop uninterruptible.
* **WIQL literals are quoted, never interpolated.** A single
  `quoteWIQLLiteral` helper doubles embedded quotes; `correlation_tag_prefix` is
  validated against `^[A-Za-z0-9._:-]+$` and every external key against
  `^[0-9a-f]{32}$` before it reaches a query. WIQL has no parameter binding, so
  this is the only defense.
* `System.Tags` is a single semicolon-separated string. Every write composes the
  **union** of the remote tag set read during `plan` and the mapped tag set,
  minus any stale `backlogit-id:` tag, so a human-added tag is never dropped and
  a renamed artifact does not accumulate duplicate id tags.
* Relations are only appended with `/relations/-`. v1 never removes a relation,
  which sidesteps the index-shifting hazard entirely.
* `suppressNotifications=true` by default. `bypassRules` only under the
  approval-gated `--bypass-rules`.
* `validateOnly=true` backs `--validate-remote`, available on both surfaces.
* No `DELETE` method and no `Destroy`-named symbol exists in the package.

## Idempotency and Concurrency Behavior

* **Create is idempotent through adoption**, keyed on the immutable
  `custom_fields.external.key`, and only when correlation coverage is complete.
  A truncated lookup blocks rather than creating.
* **Update is idempotent through the content hash.** The hash is computed over
  the `MappedItem` value — post-mapping, pre-serialization, canonically ordered
  — never over raw frontmatter or file bytes, and `custom_fields.external.*` is
  structurally excluded. Otherwise every apply would change its own hash input
  and no re-plan could ever report `noop`.
* **Concurrency is optimistic** via the `/rev` test op, and **exclusive** within
  a workspace via the apply lease.
* **Ordering is deterministic**: depth ascending, then artifact ID, computed
  from a sorted key list so Go map iteration order cannot affect it.

## CLI and MCP Surface

### Operations

| Operation | CLI | MCP tool | Writes to Azure DevOps |
|---|---|---|---|
| Discovery refresh | `backlogit ado discover` | `backlogit_ado_discover` | No |
| Compute plan | `backlogit ado plan` | `backlogit_ado_plan` | No |
| List and read plans | `backlogit ado plans` | `backlogit_ado_plans` | No |
| Apply plan | `backlogit ado apply --plan <id>` | `backlogit_ado_apply` | Yes |
| Link, drift, and orphan report | `backlogit ado status` | `backlogit_ado_status` | No |
| Reconcile pending, report orphans | `backlogit ado verify` | `backlogit_ado_verify` | No |

`backlogit ado plans` exists because the reviewed draft left computed plans,
progress ledgers, and the discovery snapshot readable only from a gitignored
directory. A human can list that directory; an agent could not, so an agent that
computed a plan could never rediscover its `plan_id` after a session boundary.
`plans` lists plan ids with `created_at`, digest match state, per-artifact
progress, and staleness, and reads a single plan by id.

Shared parameters on both surfaces: `shipment`, `item`, `plan`, `resume`,
`refresh`, `max_discovery_age`, `validate_remote`, `format`.

### Agent apply gate

`backlogit_ado_apply` is refused unless `agent_apply.enabled: true` in the
workspace integration config. Default is `false`.

This closes a real hole: the plan-then-apply split does not by itself constrain
an autonomous agent, because the same agent can call `backlogit_ado_plan` and
then immediately call `backlogit_ado_apply` with the returned id, reviewing
nothing. Gating on workspace **configuration** rather than on surface preserves
operation parity — the operation exists on both surfaces with identical
semantics — while requiring a human to opt the workspace into unattended agent
writes. The refusal returns
`error: agent_apply_disabled` with `remediation` naming the CLI command and the
config key.

### CLI-only levers and the approval gate

| Flag | Owning operation | Rationale |
|---|---|---|
| `--bypass-rules` | `ado apply` | Sends `bypassRules=true`, skipping server validation |
| `--allow-remote-drift` | `ado plan` only | Freezes `skip_revision_test: true` plus the approval reason into the plan, so the resulting `plan_id` differs and the reviewed artifact states the overwrite. `apply` has no such flag |
| `--force-relink` | `ado verify` | Rewrites a link record without remote confirmation |

Registry-level exclusion from MCP is a **surface** restriction, not an approval
mechanism. Constitution VII (NON-NEGOTIABLE) and the strict-safety contract both
require operator approval before a destructive action, so U43 adds a real gate:

* Each lever requires an interactive confirmation on a TTY.
* On a non-TTY invocation the command refuses unless the operator supplies
  `--i-understand` **and** `--reason <text>`; the reason is recorded in the
  result of the gated operation — the plan for `--allow-remote-drift`, the apply
  or verify result for the other two — and required in the closure artifact.
* If the `agent-intercom` capability pack is enabled, approval routes through
  its approval workflow before execution.
* Bypassing the gate emits a P-005 event and halts.
* `--allow-remote-drift` passes the identical gate at **plan** creation, not at
  apply. Its recorded reason is frozen into the plan and echoed in the apply
  result of any run that executes that plan, so the approval travels with the
  reviewed artifact instead of being re-asserted at write time.

`apply` therefore owns no lever that changes what will be written. It executes
the frozen plan unchanged: it cannot drop, add, or weaken a `/rev` test, and no
option on `core.ApplyExternalSyncPlan` can do so either.

### Result shapes

Every collection field is always serialized. No collection carries `omitempty`,
every collection is initialized non-nil, and every per-entry collection appears
on every entry as `[]` when empty. The examples below are the specification an
implementer will encode, so they show that invariant rather than eliding it.

`plan` result:

```json
{
  "plan_id": "ado-9f2c1a4e7b30",
  "created_at": "2026-08-20T18:04:11Z",
  "provider": "azure_devops",
  "organization": "contoso",
  "project": "Platform",
  "selection": {"shipment": "128-S", "artifact_count": 14},
  "discovery_snapshot_age_seconds": 3120,
  "actions": [
    {"artifact_id": "144-F", "action": "create", "work_item_type": "Feature",
     "changed_fields": ["System.Title", "System.Description", "System.State"]},
    {"artifact_id": "144.001-T", "action": "update", "work_item_id": 4821,
     "expected_rev": 7, "changed_fields": ["System.Title", "System.State"]},
    {"artifact_id": "144.002-T", "action": "adopt", "work_item_id": 4990,
     "changed_fields": []},
    {"artifact_id": "144.003-T", "action": "conflict", "work_item_id": 5001,
     "expected_rev": 7, "remote_rev": 11, "changed_fields": [],
     "resolution": "human_terminal_required",
     "remediation": "backlogit ado plan --shipment 128-S --allow-remote-drift --i-understand --reason <text>"},
    {"artifact_id": "144.004-T", "action": "noop", "changed_fields": []}
  ],
  "warnings": [
    {"code": "orphan_local", "artifact_id": "140-F", "work_item_id": 4700}
  ],
  "blocking": []
}
```

`apply` result:

```json
{
  "plan_id": "ado-9f2c1a4e7b30",
  "summary": {"created": 3, "updated": 8, "adopted": 1, "noop": 1,
              "deferred": 0, "conflicts": 1, "failed": 0, "pending_verify": 0},
  "applied": [{"artifact_id": "144-F", "work_item_id": 5210, "rev": 1}],
  "conflicts": [{"artifact_id": "144.003-T", "expected_rev": 7, "remote_rev": 11,
                 "resolution": "human_terminal_required",
                 "remediation": "backlogit ado plan --shipment 128-S --allow-remote-drift --i-understand --reason <text>"}],
  "deferred": [],
  "failed": [],
  "pending_verify": [],
  "warnings": []
}
```

Every `conflict` entry carries `resolution` and `remediation`. Without them an
agent stalls permanently on any concurrent remote edit, because all three
recovery levers are CLI-only. The F6 rule forbids a *more dangerous* agent
surface; it does not sanction a dead-ended one, so the agent gets a
machine-readable escalation contract mirroring `gateBlockedResult`'s
`allowed_next_actions`.

The `remediation` string names `ado plan`, never `ado apply`: the drift override
is a plan-time decision that produces a new, separately reviewable `plan_id`.

`status` and `verify` results follow the same shape: a `summary` object plus
`links`, `conflicts`, `pending_verify`, and `warnings` arrays, always present.
Their `warnings` array is where `orphan_remote` appears, for ineligible locals
that `plan` never sees:

```json
{"code": "orphan_remote", "artifact_id": "140-F", "status": "shipped",
 "work_item_id": 4700, "action": "report_only"}
```

### Error contract

| Sentinel (`internal/errors`) | Meaning |
|---|---|
| `ErrIntegrationConfigMissing` | No `integrations/azure-devops.yaml` |
| `ErrIntegrationCredentialMissing` | `auth.pat_env` names an unset variable |
| `ErrIntegrationHostNotAllowed` | `organization_url` host outside the allowlist |
| `ErrIntegrationSchemaMismatch` | Mapping references something discovery did not return |
| `ErrExternalPlanNotFound` | `--plan` names an unknown or pruned plan |
| `ErrExternalPlanStale` | A digest or artifact hash changed since the plan was computed |
| `ErrExternalApplyLeaseHeld` | Another apply holds the workspace lease |
| `ErrAgentApplyDisabled` | MCP `apply` without `agent_apply.enabled` |

Remote outcomes are classified with a typed error defined in the stdlib-only
leaf so `ado`, `extsync`, `core`, `cli`, and `mcp` can all branch on it:

```go
// ExternalWriteError classifies a remote write outcome.
type ExternalWriteError struct {
    Class   ExternalWriteClass // remote_drift|validation|not_found|throttled|indeterminate|permanent
    Status  int
    TypeKey string
    Err     error
}
func (e *ExternalWriteError) Error() string { ... }
func (e *ExternalWriteError) Unwrap() error { return e.Err }
```

Every classification site uses `errors.As`, never status-string or message
matching. This type is deliberately **distinct from**
`blerrors.ErrWriteIndeterminate`, which belongs to the local durable-write
contract and has special envelope and MCP behavior. A remote outcome-unknown is
`ExternalWriteClass("indeterminate")`; it maps to the same no-blind-retry policy
without being presented as local write indeterminacy.

A partially applied run returns a structured payload modeled on the existing
`mutation_partial` shape:

```json
{
  "error": "external_sync_partial",
  "plan_id": "ado-9f2c1a4e7b30",
  "applied": [],
  "conflicts": [],
  "failed": [{"artifact_id": "144.005-T", "class": "validation",
              "retryable": false, "message": "..."}],
  "pending_verify": [{"artifact_id": "144.006-T",
                      "external_key": "4b7e29f1a83d6c05e1d8a2f6b90c3a71"}],
  "local_partial": {"classification": "indeterminate",
                    "completed_steps": ["external-link-frontmatter"],
                    "continuation_applied": ["external-link-projection"],
                    "failed_step": "external-link-frontmatter",
                    "compensation_state": "not-compensated"},
  "retryable": true,
  "recovery": "re-run with --resume; resolve pending_verify with backlogit ado verify"
}
```

`local_partial` carries the exact nested `MutationPartialError` payload so the
governed recovery vocabulary is preserved rather than flattened. On the MCP
surface, a local durable-write failure additionally routes through the existing
`durabilityOutcomeResult` helper so `write_not_applied` (`retryable: true`) and
`write_indeterminate` (`retryable: false`) match the rest of the MCP surface.

## Security and Secrets Model

The integration config is **repo-controlled and auto-loaded**, therefore
untrusted. The threat model treats a malicious pull request or a compromised
agent write to that file as in scope.

| Control | Mechanism |
|---|---|
| Credential source | Environment variable only, name restricted to `^BACKLOGIT_ADO_[A-Z0-9_]*$`. No `--pat` flag, no config token field, no credential file. A `personal_access_token` key is a validation error |
| Credential exfiltration via config | Host allowlist on `organization_url` plus the `pat_env` prefix restriction. Together these prevent retargeting an arbitrary secret at an arbitrary host |
| Credential exfiltration via redirect | `CheckRedirect` rejects cross-host redirects and strips `Authorization` on any followed redirect |
| Credential in output | Every error from `internal/ado` passes `redactSecrets`, which strips `Authorization` values and any substring equal to the live token. U40 plants a token and asserts it appears in no error, plan file, ledger, snapshot, JSONL event, or telemetry record |
| Least privilege | `vso.work_write` documented as the required scope; U38 requires an organization-scoped PAT with an expiry no longer than the verification window |
| Transport | Absolute `https` enforced; HTTP rejected so a PAT cannot be sent in the clear |
| Query injection | `quoteWIQLLiteral` plus charset validation: `correlation_tag_prefix` against `^[A-Za-z0-9._:-]+$` and every external key against `^[0-9a-f]{32}$`. WIQL has no parameter binding |
| SQL injection | All `external_links` helpers use bound `?` parameters; no identifier or value is interpolated |
| Stored XSS in the target system | The Markdown renderer is configured to **escape raw inline HTML** rather than pass it through. A body containing `<script>` or `<img onerror=...>` renders inert |
| Agent-initiated external writes | `agent_apply.enabled` defaults to `false`; destructive levers are CLI-only and approval-gated |
| Data exposure | `selection.exclude_types` defaults exclude `deliberation`, `spike`, `review`, and `shipment`, so internal deliberations and spike findings are not pushed. `plan` prints the artifact types it will push so an operator sees the scope before applying |
| Path containment | All runtime paths are built once in `internal/core` from `WorkspaceStorageRoot`, normalized with `filepath.Abs`, and are never re-passed through a helper that re-joins non-absolute input onto the root |

On the HTML trust boundary: the reviewed draft argued repo Markdown is trusted
and declined sanitization. That argument does not hold — backlogit's own
`migrate` path ingests externally authored documents into artifact bodies, and
stash entries can carry third-party text. Rather than adding a sanitizer
dependency, U17 configures the renderer so raw inline HTML is escaped at the
source. The protected invariant is testable: rendered output contains no raw
passthrough HTML.

## Requirements Trace

Every requirement maps to at least one unit, and every unit maps to at least one
requirement. The reviewed draft had six orphan units and a misattributed
telemetry row.

| Requirement | Implementation action | Units |
|---|---|---|
| One-way push, Markdown authoritative | Provider exposes no local-write path; remote reads feed only plan computation | U14, U15, U22 |
| Ineligible locals are reported, never pushed or deleted | `status` and `verify` scan durable link records independently of push eligibility and emit `orphan_remote`; `plan` and `apply` exclude those statuses and generate no deletion action | U30, U38, U40 |
| Shipment selects, artifact maps | Selection resolver expands shipment membership plus ancestors | U30, U41 |
| Explicit command, never a hook | No hook registration is added anywhere | U40, U41, U42 |
| Configurable WIT and status mapping | `type_map`, `state_map`, `field_map`, `value_map` | U1, U3, U18 |
| Separate integration schema, `migration.yaml` untouched | New `config.AzureDevOpsConfig` loader | U1, U4 |
| `ExternalMap` becomes a working fallback | `mapValue` consults `value_map` then `TranslateExternalMap` with a nil guard | U18, U49 |
| Durable identity survives ID rewrite | Stable 32-hex-character `external.key` minted from `crypto/rand`, plus an adopt/re-parent migration contract | U24, U39 |
| Disposable SQLite projection | `external_links` table plus rehydrate plus `items` re-upsert | U26, U27 |
| Idempotent create through adoption | Correlation key, chunked WIQL, fail-closed truncation | U12, U20, U22 |
| Optimistic concurrency | `test` on `/rev` as first patch op unless the frozen action carries the plan-time `skip_revision_test` | U10, U11, U22, U23 |
| Drift detection at plan time | Batch read `System.Rev`, compare to stored rev | U12, U22 |
| Plan is a frozen execution contract | Resolved payloads plus binding digests, content-addressed id | U23, U29, U31 |
| Partial failure is artifact-atomic | Per-artifact loop; envelope only for the local triple | U33, U34, U35 |
| Indeterminate never retried | Typed remote classification by terminal cause, indeterminacy dominant, plus `pending_verify` | U8, U9, U36 |
| Resume is safe | Frontmatter-authoritative resume, correlation re-resolution | U37 |
| Pending work is semantically confirmed | `verify` compares remote fields to the frozen payload | U38 |
| No duplicate creates under concurrency | Workspace apply lease | U32 |
| Process discovery, no hardcoding | Discovery snapshot plus fail-closed validation | U13, U16, U29 |
| CLI and MCP parity, dangerous levers CLI-only and approval-gated | Shared core entry points, registry markers, approval gate, drift tests | U40, U41, U42, U43, U44, U45 |
| Agent surface is safe and not dead-ended | `agent_apply` gate, `resolution`/`remediation`, `ado plans` read path | U41, U43, U44 |
| PAT from a restricted environment variable only | `Authenticator`, config validation, redaction | U2, U6, U48 |
| No credential exfiltration via config or redirect | Host allowlist, `pat_env` prefix, `CheckRedirect` | U2, U7, U48 |
| No stored XSS in the target system | Renderer escapes raw inline HTML | U19 |
| No query injection | `quoteWIQLLiteral` plus charset validation of the tag prefix and the hex external key | U2, U12, U24 |
| Rate limiting, retry, and bounded runtime | Client limiter, `Retry-After`, capped backoff, request timeout, run deadline | U7, U9 |
| Provider seam only, no Jira or GitHub | One interface, one implementation, asserted by dependency test | U14, U47 |
| Invariants are enforced, not asserted in prose | Negative-space and dependency-direction tests | U47 |
| Telemetry, monitoring, rollback | Counters, JSONL events, `verify` recovery | U28, U46, U50 |
| Documentation and discoverability stay accurate | ARCHITECTURE, design doc, fallback guide, CLI reference | U49 |
| External unknowns are resolved before closure | Scratch-project runtime verification | U50, U51 |

## Implementation Units

Granularity rule, stated precisely because the reviewed draft claimed a bound it
broke: every unit touches **at most 2 production files** and **at most 3 test
scenarios**, targets a single domain, and produces a verifiable milestone.
Posture is test-first unless stated: the test file is written and observed
failing before the production file exists.

### Phase A — Configuration (domain: config)

**U1 — `AzureDevOpsConfig` load and structural validation.**
Files: `internal/config/azuredevops.go`, `azuredevops_test.go`.
Tests: a valid file loads; a missing `connection.project` is rejected; a
`batch_read_size` of 500 is rejected.
Acceptance: `LoadAzureDevOpsConfig` returns a validated struct or a wrapped
error. No file is created implicitly on any path.

**U2 — Security validation rules.**
Files: `internal/config/azuredevops.go` (extend), `azuredevops_security_test.go`.
Tests: an `organization_url` host outside the allowlist is rejected with
`ErrIntegrationHostNotAllowed`; a `pat_env` not matching
`^BACKLOGIT_ADO_[A-Z0-9_]*$` is rejected; a `personal_access_token` key or an
out-of-charset `correlation_tag_prefix` is rejected.
Acceptance: a repo-controlled config cannot name an arbitrary environment
variable or an arbitrary host. The `correlation_tag_prefix` charset rule is the
config half of the WIQL literal defense; the other half is the minted external
key, whose `^[0-9a-f]{32}$` shape is enforced in U24.
Depends on: U1.

**U3 — Mapping accessors and state-map resolution.**
Files: `internal/config/azuredevops_map.go`, `azuredevops_map_test.go`.
Tests: per-type override wins over the top-level `state_map`, which wins over
the built-in default; an unmapped artifact type returns a typed error; a
persisted map exactly equal to a frozen prior default is upgraded at load time.
Acceptance: accessors never panic on partial config and always return a typed
error rather than a zero value; adding a new `ArtifactStatus` does not hard-fail
an existing workspace whose map is an unmodified generated default.
Depends on: U1.

**U4 — Runtime directory gitignore.**
Files: `.gitignore`.
Tests: none (repo config). Acceptance: `.backlog/runtime/` is ignored alongside
`.backlogit/runtime/`, verified by `git check-ignore`.

### Phase B — Azure DevOps REST leaf (domain: code, `internal/ado`)

**U5 — Client construction and URL building.**
Files: `internal/ado/client.go`, `client_test.go`.
Tests: the create URL for `User Story` yields `.../workitems/$User%20Story` with
the `$` intact; `api-version=7.1` is present on every built URL; a trailing
slash in `organization_url` is normalized.
Acceptance: no call site constructs a URL by string concatenation.

**U6 — Authentication and redaction.**
Files: `internal/ado/auth.go`, `auth_test.go`.
Tests: the header equals `Basic ` plus `base64(":"+pat)`; an unset `pat_env`
returns `ErrIntegrationCredentialMissing`; a planted token never appears in a
redacted error string.
Acceptance: the `Authenticator` interface is the only producer of the
`Authorization` header.
Depends on: U5.

**U7 — Redirect policy and timeout caps.**
Files: `internal/ado/client.go` (extend), `client_redirect_test.go`.
Tests: a cross-host redirect is rejected; a same-host redirect is followed with
`Authorization` stripped; a configured `request_timeout_ms` above the package
hard cap is clamped down, and below it is honored.
Acceptance: `http.Client.CheckRedirect` is always set; no code path uses the
default redirect behavior.
Depends on: U5.

**U8 — Error decode and classification.**
Files: `internal/ado/errors.go`, `internal/errors/external_sync_errors.go`.
Tests: an Azure DevOps error body decodes into `typeKey` and `message`;
`errors.As` recovers `*ExternalWriteError` through two `%w` wraps; a 429
classifies `throttled`, a 5xx `indeterminate`, and an unclassifiable 4xx
`permanent`.
Acceptance: classification uses typed errors only, never status-string or
message matching. `validation` and `permanent` are non-retryable by
construction, `throttled` is the only retryable class, and no classification
ever pairs `permanent` with a retryable result.
Depends on: U5.

**U9 — Retry, backoff, and rate limiting.**
Files: `internal/ado/retry.go`, `retry_test.go`.
Tests: `Retry-After: 2` is honored before the next attempt and the backoff is
capped; cancelling the context during a limiter wait returns
`context.Canceled`; cancelling during a `Retry-After` sleep does the same.
Acceptance: the limiter is a client field, every wait takes the request context,
and no sleep uses `time.Sleep`. Only `throttled` outcomes are retried; a
`validation`, `permanent`, or `indeterminate` outcome exits the loop on the
attempt that produced it, and the exhausted-budget error preserves the terminal
cause plus a sticky indeterminate marker for U36 to classify.
Depends on: U8.

**U10 — JSON Patch encoding.**
Files: `internal/ado/patch.go`, `patch_test.go`.
Tests: `TestRev(7)` marshals to `{"op":"test","path":"/rev","value":7}`;
`SetField` produces `add` on `/fields/{ref}`; `AddRelation` appends to
`/relations/-` and the document marshals as a JSON array.
Acceptance: `Content-Type: application/json-patch+json` is set by the encoder,
not by call sites.

**U11 — Work item writes.**
Files: `internal/ado/workitems.go`, `workitems_write_test.go` (`httptest`).
Tests: `Create` accepts HTTP 200 and returns id plus rev; `Update` places the
`test` op first; `validateOnly` and `bypassRules` are forwarded only when the
options say so.
Acceptance: `WriteOptions` is an explicit parameter; no per-call flag is carried
as client state.
Depends on: U6, U7, U9, U10.

**U12 — Batch read and WIQL.**
Files: `internal/ado/wiql.go`, `wiql_test.go`.
Tests: `GetBatch` chunks a 450-id request into three calls of at most 200 and
returns a `ReadResult` naming the missing refs; `quoteWIQLLiteral` doubles an
embedded quote so an injected `' OR 1=1 --` payload is escaped; a chunk whose
result count equals `$top` is reported as truncated.
Acceptance: no WIQL string is built by interpolation; a correlation key that
does not match `^[0-9a-f]{32}$` is rejected before it reaches a query;
truncation is a first-class return value, not a log line.
Depends on: U6, U9.

**U13 — Discovery operations.**
Files: `internal/ado/discovery.go`, `discovery_test.go`.
Tests: types, states, fields, and classification nodes decode from recorded
fixtures; `$depth` is forwarded; discovery uses the shorter probe timeout.
Depends on: U6, U9.

### Phase C — Provider seam (domain: code, `internal/extsync`)

**U14 — `Provider` interface and value types.**
Files: `internal/extsync/provider.go`, `internal/extsync/types.go`.
Tests: an in-package hand-written fake implements `Provider` and is exercised by
a table test; `RemoteRef`, `MappedItem`, and `Schema` round-trip through JSON
with the exact serialized key names from the producer-contract vocabulary; a
malformed `RemoteRef.Version` is rejected rather than silently dropping the
revision test.
Acceptance: the interface is

```go
type Provider interface {
    Name() string
    Discover(ctx context.Context) (Schema, error)
    Resolve(ctx context.Context, keys []CorrelationKey) (ResolveResult, error)
    Read(ctx context.Context, refs []RemoteRef) (ReadResult, error)
    Create(ctx context.Context, item MappedItem, opts WriteOptions) (RemoteRef, error)
    Update(ctx context.Context, ref RemoteRef, item MappedItem, opts WriteOptions) (RemoteRef, error)
}
```

with `ReadResult{Found map[string]RemoteItem; Missing []RemoteRef}`,
`ResolveResult{Matches map[CorrelationKey][]RemoteRef; Truncated []CorrelationKey}`,
and `WriteOptions{ValidateOnly, BypassRules, SuppressNotifications, SkipRevisionTest bool}`.
`Relate` is **not** on the interface — the parent link is a `MappedItem` field
consumed by the patch builder, and v1 never removes a relation, so a `Relate`
method would be dead API. The reviewed draft's flat `[]RemoteItem` return could
not express which refs were omitted, which is exactly the `orphan_local` signal,
and its flat `Resolve` return could not express zero-, one-, or many-match
multiplicity, which drives three different actions.

**U15 — Azure DevOps provider adapter.**
Files: `internal/extsync/azuredevops.go`, `azuredevops_test.go`.
Tests: `Create` translates a `MappedItem` plus `WriteOptions` into the expected
patch document; `Resolve` returns zero-, one-, and many-match results correctly;
`Read` reports missing refs.
Acceptance: `var _ Provider = (*azureDevOpsProvider)(nil)` lives here, not in
U14. The adapter holds **no mapping or policy logic** — translation only — so it
cannot grow into a framework. It is the single I/O-adjacent type in
`internal/extsync`: it issues no HTTP itself and touches no file, delegating
every network call to an injected `internal/ado` client, so the package stays
core-independent and filesystem-free even though this file participates in a
round trip. Its tests drive a fake `ado` client, never a socket.
Depends on: U11, U12, U14.

**U16 — Fail-closed mapping validation.**
Files: `internal/extsync/validate.go`, `validate_test.go`.
Tests: an unknown work item type is rejected naming the value; an unknown state
is rejected naming the value; an unknown area path is rejected naming the value.
Acceptance: validation returns every offending entry, not the first.
Depends on: U3, U14.

### Phase D — Mapping, hashing, and plan computation (domain: code, `internal/extsync`)

**U17 — Content hash.**
Files: `internal/extsync/hash.go`, `hash_test.go`.
Tests: the hash is stable across Go map iteration order using N independent
pairs; it changes when a mapped field changes; hashing an artifact, writing a
link record into it, and re-hashing yields the same value.
Acceptance: the hash is computed over the `MappedItem` value with a documented
canonical ordering, never over frontmatter or file bytes, and
`custom_fields.external.*` is structurally excluded. Without this the
idempotency proof — an immediate re-plan reporting all `noop` — can never hold.

**U18 — Field and value mapping.**
Files: `internal/extsync/mapping.go`, `internal/core/fields.go` (nil guard).
Tests: status maps through the resolved `state_map`; a value absent from
`value_map` falls back to `FieldConfig.ExternalMap`; a nil `*FieldConfig` or a
nil `ExternalMap` returns the input value instead of panicking.
Acceptance: `TranslateExternalMap` gains
`if fieldConfig == nil { return value, nil }`. `WorkspaceConfig.Fields` is
optional, so the ordinary case of a mapped field with no `fields:` entry would
otherwise panic inside library code.
Depends on: U3, U14.

**U19 — Text rendering.**
Files: `internal/extsync/text.go`, `go.mod` plus `go.sum` (promote
`blackfriday/v2` to direct).
Tests: Markdown renders to HTML; a body containing `<script>alert(1)</script>`
and `<img src=x onerror=alert(1)>` renders inert with the tags escaped; an empty
body yields an omitted field rather than an empty paragraph.
Acceptance: the renderer is configured so raw inline HTML is escaped, never
passed through. The invariant "rendered HTML contains no raw passthrough HTML"
is testable and is listed in the protected invariants.

**U20 — Tag composition.**
Files: `internal/extsync/tags.go`, `tags_test.go`.
Tests: the `backlogit-key:` correlation tag is always present exactly once;
remote-only tags survive a round trip; a stale `backlogit-id:` tag from a prior
artifact ID is replaced rather than accumulated.
Acceptance: output is a semicolon-separated string with deterministic ordering.

**U21 — Hierarchy and ordering.**
Files: `internal/extsync/hierarchy.go`, `hierarchy_test.go`.
Tests: N independent parent/child pairs (N at least 8) in one collection all
order parent-before-child in a single run; an unsynced parent under
`require_parent_synced` yields `deferred` for the child; a cycle is rejected.
Acceptance: ordering is computed from a sorted key list (depth ascending, then
artifact ID) so map iteration order cannot affect it. A single-pair test would
pass on an unfixed implementation and flake later, and a missed ordering bug
here creates a real orphaned or duplicated remote work item.

**U22 — Action computation (state machine).**
Files: `internal/extsync/plan.go`, `plan_test.go`.
Tests: unlinked with no correlation match yields `create` and with one match
yields `adopt`; a truncated correlation lookup yields `blocked`; linked with a
rev mismatch yields `conflict` and with an unchanged hash yields `noop`.
Acceptance: every produced `action` value is in the closed vocabulary; a
`create` is never produced when correlation coverage is uncertain; no code path
produces `orphan_remote`, which is a `status` and `verify` warning only; and a
drift-override input turns a `conflict` into an `update` carrying
`skip_revision_test: true`, so the override is visible in the computed action
rather than applied later.
Depends on: U17, U18, U20, U21.

**U23 — Frozen plan specification.**
Files: `internal/extsync/plan.go` (extend), `plan_freeze_test.go`.
Tests: the frozen spec carries resolved fields, tags, parent ref, and
`expected_rev`; `plan_id` is a content digest stable across two identical
computations and different when an action changes; `created_at` is not part of
the id; a plan computed with the drift override carries `skip_revision_test:
true` plus the recorded approval reason and yields a `plan_id` different from
the same selection planned without it.
Acceptance: `Plan` is a pure value type with `MarshalJSON`/`UnmarshalJSON` and
`PlanID()`. It performs no filesystem access; persistence is U29.
`skip_revision_test` and the approval reason are frozen fields, so nothing
downstream of `plan` can change whether a `/rev` test is emitted.
Depends on: U22.

### Phase E — Durable link state (domain: code, `internal/core` and `internal/db`)

**U24 — External key and link record codec.**
Files: `internal/core/external_link.go`, `external_link_test.go`.
Tests: an artifact carrying `archived_from` and `archived_status` retains both
after a link-record write; an artifact carrying `links` entries retains them; a
first link mints a stable `external.key` matching `^[0-9a-f]{32}$` that a second
write does not change, and a seeded minter that first returns an already-used
key regenerates before persisting.
Acceptance: the key is 128 bits from `crypto/rand` encoded with `encoding/hex`
— standard library only — and is validated against `^[0-9a-f]{32}$` on both
mint and read, so a hand-edited or malformed key is a typed error rather than a
WIQL literal. Before persistence the minter compares the candidate against the
durable keys already in the workspace and regenerates on a match; remote
duplicates are out of scope here and are handled by `Resolve` multiplicity. The
codec loads through `findArtifact` (Markdown source of truth),
never through the DB fast path, and uses a single load for both the guard and
the mutate-then-persist step. `synced_at` is stamped with `models.NowUTC()` and
serializes with a trailing `Z`. This is a known regression class: the typed
`UpdateArtifact` round trip drops archive provenance, and a `loadArtifact`-first
re-persist destroys `item_links`, because the `items` table has neither column.

**U25 — Link-record write guard.**
Files: `internal/core/external_link.go` (extend), `external_link_guard_test.go`.
Tests: an artifact whose type is in `exclude_types` is refused before any
type-map resolution; an `archived` artifact is refused; a `type_map` that
deliberately includes an excluded type does not defeat the guard.
Acceptance: the guard sits at the top of the setter, so `--force-relink` and any
future caller cannot bypass an upstream selection filter. The guard blocks
*writes* only; reading an existing link record on an archived, abandoned, or
shipped artifact stays available, because that is exactly what the `status` and
`verify` `orphan_remote` scan needs.
Depends on: U24.

**U26 — SQLite projection.**
Files: `internal/db/external_links.go`, `internal/db/schema.go` (extend).
Tests: `EnsureSchema` creates the table and indexes idempotently; upsert
replaces on `(item_id, provider)`; `IntrospectSchema` reports the new table.
Acceptance: every helper is `func XxxExternalLink(ctx context.Context, db *sql.DB, ...)`
using `QueryContext`/`ExecContext` with bound `?` parameters, `defer rows.Close()`,
and a checked `rows.Err()`. Index retention is backed by one `EXPLAIN QUERY PLAN`
subtest per query shape `status` and `verify` actually issue, matching the index
name with a trailing `(` and scanning plan integer columns as `int64`.

**U27 — Rehydrate the projection.**
Files: `internal/db` rehydrate path (extend), rehydrate test (extend).
Tests: an artifact carrying a link record produces an `external_links` row after
rehydrate; deleting the database and rehydrating restores every row.
Depends on: U24, U26.

**U28 — Sync audit events.**
Files: `internal/core/external_sync_events.go`, `external_sync_events_test.go`.
Tests: each of the four event types appends with the documented delta keys; an
append failure surfaces rather than being swallowed; the helper uses the
caller-supplied writer.
Acceptance: the signature takes `ew *events.EventWriter` with a nil guard — the
CLI passes `nil`, the MCP server passes `s.Events`. Minting a writer inside core
would give each concurrent MCP call its own mutex and can interleave the
per-item JSONL stream.
Depends on: U24.

### Phase F — Core orchestration (domain: code, `internal/core`)

**U29 — Runtime store: snapshot, plan, and ledger persistence.**
Files: `internal/core/external_sync_store.go`, `external_sync_store_test.go`.
Tests: a plan and a snapshot round-trip; a snapshot older than the configured
age reports stale; every path resolves correctly when the workspace root is
**relative**.
Acceptance: paths are built once from `WorkspaceStorageRoot` and normalized with
`filepath.Abs`; a path that already contains the workspace root is never
re-passed through a helper that re-joins non-absolute input onto that root. At
least one test constructs a genuinely relative workspace root, because
`t.TempDir()` is always absolute and structurally cannot exercise this path.
Depends on: U23.

**U30 — Selection resolution and ancestor expansion.**
Files: `internal/core/external_sync_select.go`, `external_sync_select_test.go`.
Tests: a shipment expands to members plus ancestors; an explicit `--item` list
resolves; an ancestor excluded by `exclude_types` or `eligible_statuses` makes
its descendants `deferred` rather than orphaned; an archived, abandoned, or
shipped artifact never enters the selection, whatever its link state.
Acceptance: an unknown shipment or item returns a typed error. Push selection is
the only consumer of `eligible_statuses`; the resolver exposes a separate
`AllLinkedArtifacts` read used by `status` and `verify` (U38, U40) that
deliberately ignores eligibility, because an ineligible local is precisely what
`orphan_remote` reports.

**U31 — Plan orchestration.**
Files: `internal/core/external_sync_plan.go`, `external_sync_plan_test.go`.
Tests: a stale snapshot triggers a refresh; batch-read chunking is invoked with
the configured size; a truncated correlation chunk emits `correlation_truncated`
and blocks the plan.
Acceptance: `AzureDevOpsConfig` is loaded **inside** this shared core entry
point, not in either surface handler, so CLI and MCP cannot drift on
config-driven behavior. `external_sync_planned` is appended per artifact.
Depends on: U16, U22, U29, U30.

**U32 — Apply lease.**
Files: `internal/core/external_sync_lease.go`, `external_sync_lease_test.go`.
Tests: a second apply in the same workspace fails with
`ErrExternalApplyLeaseHeld`; the lease heartbeats across a simulated long call;
a stale lease past its TTL is reclaimable.
Acceptance: the lease follows the existing task-lock pattern and its scope
(one shared workspace) is stated in the error message.

**U33 — RED harness for the apply loop.**
Files: `internal/core/external_sync_apply.go` (signature-only stub),
`internal/core/external_sync_apply_test.go`.
Tests, driven by a fake `extsync.Provider`: a failure on artifact 2 leaves
artifact 1 applied; a `remote_drift` result records `conflict` and continues; a
successful create writes all three local representations.
Acceptance: the stub returns `errors.New("not implemented")` so the harness
**compiles and fails on assertions** rather than failing to build. A test file
referencing an undefined symbol produces a build error, which is a different and
unverifiable signal.
Depends on: U14, U23, U24, U28.

**U34 — Apply loop: ordering and provider invocation.**
Files: `internal/core/external_sync_apply.go`.
Acceptance: U33's ordering and continuation assertions turn green; the loop
consumes only frozen plan payloads and synthesizes no action; the entry point is
`core.ApplyExternalSyncPlan(ctx, ws, planID string, prov extsync.Provider, ew *events.EventWriter, opts ...ExternalSyncOption)`
with `WithResume()` and `WithBypassRules()`, applied before validated required
fields so an option cannot override an invariant. There is deliberately **no**
`WithAllowRemoteDrift()`: `skip_revision_test` is a frozen per-action field
decided and approved at plan time (U23, U43), and the loop reads it rather than
being able to set it. A test asserts that the emitted patch document carries the
`/rev` test whenever the frozen action does not set `skip_revision_test`.
Depends on: U32, U33.

**U35 — Local write triple and envelope semantics.**
Files: `internal/core/external_sync_local.go`, `external_sync_local_test.go`.
Tests: injecting `ErrWriteIndeterminate` into the frontmatter step asserts no
compensation ran, exactly one `external_links` row exists, and the terminal
JSONL event is `external_sync_pending_verify` rather than
`external_sync_applied`; injecting `ErrWriteNotApplied` asserts compensation ran
and the class is `not-applied`; a successful run re-upserts the `items` row.
Acceptance: the envelope's real control flow is respected. On
`ErrWriteIndeterminate` the envelope does **not** halt — it skips compensation
and continues the remaining steps, recording them in `ContinuationApplied`. The
projection upsert is therefore a continuation step the envelope already runs; no
direct `db.UpsertItem` reconciliation call is added, and the JSONL step branches
on `blerrors.IsWriteIndeterminate`. Injection uses a package-level function
variable seam (`externalLinkWriteFn`); tests overriding it must not call
`t.Parallel`.
Depends on: U34.

**U36 — Remote outcome classification and `pending_verify`.**
Files: `internal/core/external_sync_apply.go` (extend),
`external_sync_outcome_test.go`.
Tests: a 5xx after send records `pending_verify` with no retry, for both a
`create` and an `update`; a budget exhausted by repeated 429s records `failed`
with `class: throttled` and `retryable: true`, for both actions; a sequence
containing one indeterminate attempt followed by 429s still records
`pending_verify`; a `validation` or `permanent` outcome is recorded without any
retry attempt.
Acceptance: classification follows the terminal cause with indeterminacy
dominant, and no recorded outcome pairs `class: permanent` with
`retryable: true`. `link_state` becomes `pending_verify` in frontmatter for
every indeterminate outcome; the remote class is `*errors.ExternalWriteError`,
never `blerrors.ErrWriteIndeterminate`.
Depends on: U35.

**U37 — Progress ledger and resume.**
Files: `internal/core/external_sync_progress.go`,
`external_sync_progress_test.go`.
Tests: `--resume` skips applied artifacts; deleting the ledger mid-run still
produces zero duplicate creates; a `create` re-attempt whose work item already
exists resolves to `adopt`.
Acceptance: the frontmatter link record is authoritative for resume and the
ledger is an optimization; a ledger append error is wrapped with `%w` and
surfaced as a per-artifact `failed`, never discarded.
Depends on: U34.

**U38 — Verify with semantic confirmation.**
Files: `internal/core/external_sync_verify.go`, `external_sync_verify_test.go`.
Tests: one correlation match whose remote fields equal the frozen payload
becomes `linked`; one match whose fields differ becomes `conflict`; zero matches
remain `pending_verify` with a diagnostic; an archived, abandoned, or shipped
artifact whose linked work item still exists produces exactly one
`orphan_remote` warning and no action.
Acceptance: `verify` never re-baselines `rev` or `content_hash` merely because a
remote id was found. A correlation tag proves existence, not that an update
landed. The `orphan_remote` scan reads link records through the eligibility-blind
path from U30, so it reports the statuses `plan` structurally cannot see, and it
never proposes a delete, a close, or a state change.
Depends on: U30, U36.

**U39 — Artifact ID rewrite migration contract.**
Files: `internal/core/external_link.go` (extend),
`external_link_idrewrite_test.go`.
Tests: a linked artifact re-parented by `AdoptItem` keeps its `external.key`,
its `external_links` row moves to the new `item_id`, and its
`backlogit-id:` tag is refreshed on the next update; a pending-verify artifact
survives an ID rewrite; an unlinked artifact is unaffected.
Acceptance: no remote work item is orphaned or duplicated by a re-parent.
Depends on: U24, U27.

### Phase G — CLI surface (domain: code, `internal/cli`)

**U40 — `backlogit ado` group with `discover` and `status`.**
Files: `internal/cli/ado.go`, `ado_test.go`.
Tests: `discover` writes the snapshot and prints a summary; `status` reports
link counts by state; `status` reports `projection_stale` when the projection
disagrees with frontmatter rather than reporting a wrong count; `status` emits
an `orphan_remote` warning for a shipped artifact whose work item still exists,
which the same fixture's `plan` run does not and cannot report.
Acceptance: a missing integration config exits with the mapped code for
`ErrIntegrationConfigMissing`. `status` is a report-only surface: it scans link
records independently of `eligible_statuses`, issues no write, and offers no
deletion affordance.
Depends on: U26, U29, U30.

**U41 — `ado plan` and `ado plans`.**
Files: `internal/cli/ado.go` (extend), `ado_plan_test.go`.
Tests: `plan` writes a frozen plan and issues no remote write; `plan
--validate-remote` issues `validateOnly` creates and persists nothing; `plans`
lists plan ids with progress and digest match state.
Acceptance: `plan` owns the drift decision. The `--allow-remote-drift` flag is
registered on this command and gated in U43; its effect is entirely inside the
frozen spec.
Depends on: U31, U40.

**U42 — `ado apply` and `ado verify`.**
Files: `internal/cli/ado_apply.go`, `ado_apply_test.go`.
Tests: `apply` without `--plan` is rejected; a digest mismatch exits with
`ErrExternalPlanStale`; `--resume` forwards to the progress ledger; `apply`
defines no `--allow-remote-drift` flag, so passing it is an unknown-flag error.
Acceptance: `apply` executes the frozen plan unchanged and exposes no lever that
alters the revision test.
Depends on: U37, U38, U41.

**U43 — Destructive lever approval gate.**
Files: `internal/cli/ado.go` (extend), `internal/cli/ado_apply.go` (extend),
`ado_approval_test.go`.
Tests: `--allow-remote-drift` on `ado plan` on a non-TTY without
`--i-understand` and `--reason` is refused; with both, the reason and
`skip_revision_test: true` are frozen into the plan and the resulting `plan_id`
differs from the ungated plan for the same selection; `--bypass-rules` on `ado
apply` and `--force-relink` on `ado verify` pass the same gate.
Acceptance: one shared gate helper serves all three levers, and each lever is
registered on exactly one command — `plan`, `apply`, and `verify` respectively.
The gate is the approval mechanism Constitution VII requires; the registry
marker is documentation, not enforcement. Bypassing emits a P-005 event and
halts.
Depends on: U42.

### Phase H — MCP surface and parity (domain: code, `internal/mcp` plus config)

**U44 — MCP tool registration and handlers.**
Files: `internal/mcp/ado_tools.go`, `ado_tools_test.go`.
Tests: all six tools are registered in `Server.toolNames`; `apply` without
`agent_apply.enabled` returns `error: agent_apply_disabled` with a
`remediation`; a local durable-write failure routes through
`durabilityOutcomeResult`.
Acceptance: handlers pass `s.Events` to the core entry point; every result
serializes all collection keys even when empty, asserted by type-asserting to
`[]any` and checking length zero.
Depends on: U34, U38.

**U45 — Registry entries and parity assertions.**
Files: `.autoharness/backlog-registry.yaml`,
`internal/cli/registry_parity_test.go` (extend).
Registry shape added for each operation. In the block below, `<shipment>` and
`<plan>` stand in for the registry's double-brace placeholder syntax — the same
form every other operation in `.autoharness/backlog-registry.yaml` already uses
— so this document stays conformant with the repository Markdown ingestion
check:

```yaml
  external_sync_plan:
    mcp_tool: "backlogit_ado_plan"
    cli_command: "backlogit ado plan --shipment <shipment>"
    governed: true
    governed_name: external_sync_plan
    params:
      shipment: "shipment"
      item: "item"
      validate_remote: "validate_remote"
      format: "format"
    cli_only_flags:
      allow-remote-drift:
        human_terminal_only: true
        rationale: "freezes skip_revision_test into the plan, authorizing an overwrite of concurrent remote edits; operator-only blast radius"
  external_sync_apply:
    mcp_tool: "backlogit_ado_apply"
    cli_command: "backlogit ado apply --plan <plan>"
    governed: true
    governed_name: external_sync_apply
    params:
      plan: "plan"
      resume: "resume"
      format: "format"
    cli_only_flags:
      bypass-rules:
        human_terminal_only: true
        rationale: "skips server-side work item rules; operator-only blast radius"
```

Tests: all six tools pass the existing live-tree drift assertions
(`EveryMCPToolMappedOrDeferred`, `EveryCLICommandResolves`, `NoOrphanMCPTool`,
`DiscoverabilityConsistency`, `FlagAndPositionalParity`); a generalized
`cli_only_flags` parity test iterates every operation, asserts each declared
flag is absent from that operation's `params` and carries `human_terminal_only`
plus a non-empty rationale, and asserts `allow-remote-drift` is declared on
`external_sync_plan` and on no other operation; the governed behavioral fixture
drives both surfaces
through `Server.InvokeTool` and `NewRootCommand` against one injected fake
provider and asserts identical frontmatter, `external_links`, and JSONL state.
Acceptance: no fallback `cli_command` template embeds a destructive lever, and
the `external_sync_apply` fallback template in particular carries no drift
lever, because `apply` has none to carry. Both
the MCP server and the root command gain a provider-injection seam so the
governed fixture can run without a network.
Depends on: U43, U44.

### Phase I — Cross-cutting

**U46 — Telemetry counters.**
Files: `internal/core/external_sync_apply.go` (extend),
`internal/telemetry/schema_ref.go` (extend).
Tests: counters for created, updated, adopted, conflicted, deferred, failed,
pending-verify, requests, throttled, and retries are emitted once per apply run;
the schema-reference drift test stays green.
Depends on: U34.

**U47 — Invariant assertions.**
Files: `internal/ado/invariants_test.go`,
`internal/extsync/deps_test.go`.
Tests: parsing the `internal/ado` package directory with `go/parser.ParseDir`
finds no exported `*ast.FuncDecl` or `*ast.TypeSpec` whose name contains
`Delete` or `Destroy`, and no `http.MethodDelete` literal; a recording
`http.RoundTripper` observes zero `POST /workitems/$` and zero `PATCH` during a
`plan` run; `go list -deps` shows `internal/extsync` does not import
`internal/core`.
Acceptance: the mechanism is stdlib-only. Reflection cannot enumerate package
symbols, so the invariant would otherwise stay prose.
Depends on: U12, U22, U31.

**U48 — Credential leak assertions.**
Files: `internal/ado/redaction_test.go`,
`internal/core/external_sync_redaction_test.go`.
Tests: a planted distinctive token appears in no error returned from a failing
request; it appears in no plan file, ledger, snapshot, JSONL event, or telemetry
record.
Depends on: U6, U34.

**U49 — Documentation.**
Files: `docs/ARCHITECTURE.md`, `docs/design-docs/azure-devops-sync-contract.md`
(new), `docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md`,
`docs/cli-reference/` (regenerated), `internal/config/schema.go` (the
`// Deprecated:` comment on `ExternalMap`).
Acceptance: `make docs-lint` reports 0 violations. The ARCHITECTURE dependency
block is **corrected**, not merely appended to: it states `ado -> stdlib,
x/time/rate, errors` as a near-leaf distinct from `release`, adds
`extsync -> models, config, ado, errors`, and corrects the existing `core` line
to its actual import set, backed by a `go list -deps` driven test so it cannot
drift again. The design doc reproduces the closed vocabularies verbatim and a
drift test asserts the Go constants and the documented tables agree.
Depends on: U45.

**U50 — Runtime verification against a scratch project.**
Files: `docs/closure/{date}-azure-devops-sync-runtime-verification.md` (new).
Acceptance: every scenario in the runtime verification table below is executed
against a scratch Azure DevOps project and recorded, including the observed HTTP
status and `typeKey` for a failed `/rev` test.
Depends on: U42, U43.

**U51 — Conditional reclassification (only if U50 contradicts a default).**
Files: `internal/ado/errors.go`, `errors_test.go`.
Acceptance: the observed `/rev` test-failure status and `typeKey` are encoded and
the conservative default is replaced. This unit exists so the rework is a
declared conditional follow-up rather than an unrecorded backward edit of a
completed unit.
Depends on: U50.

## Dependency Graph

Edges below are derived from each unit's `Depends on` line; the two
representations cannot disagree.

```text
U1 ─┬─► U2
    ├─► U3 ─┬─► U16 ─┐
    └────────┴─► U18 ─┤
U4  (independent)     │
                      │
U5 ─┬─► U6 ─┬─► U11 ──┼─► U15 ◄── U14
    ├─► U7 ─┤         │      │
    └─► U8 ─┴─► U9 ───┴─► U12│
                  └─► U13 ───┘
U14 ─► U16
U17 ─┐
U18 ─┤
U19 ─┤
U20 ─┼─► U22 ─► U23 ─► U29
U21 ─┘
U24 ─┬─► U25
     ├─► U27 ◄── U26
     ├─► U28
     └─► U39 ◄── U27
U16, U22, U29, U30 ─► U31
U14, U23, U24, U28 ─► U33 ─┐
U32 ───────────────────────┴─► U34 ─┬─► U35 ─► U36 ─► U38
                                    ├─► U37
                                    ├─► U46
                                    └─► U48 ◄── U6
U26, U29, U30 ─► U40 ─► U41 ◄── U31
U30 ─► U38            (eligibility-blind link read for orphan_remote)
U37, U38, U41 ─► U42 ─► U43 ─┬─► U45 ◄── U44 ◄── U34, U38
                             └─► U50 ─► U51
U12, U22, U31 ─► U47
U45 ─► U49
```

No cycles. The U50-to-U6 rework path from the reviewed draft is now the explicit
conditional unit U51, so the acyclicity claim is honest.

Three independent front lines can proceed in parallel: configuration (U1-U4),
the REST leaf (U5-U13), and pure mapping (U17-U21). They converge at U22 and
again at U33.

## Decisions and Rationale

| Decision | Rationale | Rejected alternative |
|---|---|---|
| Separate `integrations/azure-devops.yaml` | `MigrationConfig` requires `document_classes` and describes inbound file classification; the schemas have different lifetimes | A `targets:` block inside `migration.yaml` |
| Package named `extsync`, not `sync` | Shadowing the standard library would force an alias in every consumer and collides with the existing `backlogit sync` verb | `internal/sync` |
| Persistence in `core`; `extsync` core-independent and filesystem-free | `SafeResolve` and `WorkspaceStorageRoot` live in `core`; putting path work in `extsync` would require `extsync -> core` and create an import cycle. `extsync` holds pure computation plus one translation-only adapter that delegates all I/O to `internal/ado` | Filesystem access inside `extsync`; calling it a fully pure package while it hosts the provider adapter |
| Stable `external.key`, not the artifact ID, as the correlation key | `AdoptItem` rewrites artifact IDs, so an ID-derived tag orphans or duplicates a work item after a re-parent | `backlogit-id:{artifactID}` as the matching key |
| Key is 128 random bits as 32 lowercase hex characters | `crypto/rand` plus `encoding/hex` are standard library, the form is trivially validated by `^[0-9a-f]{32}$`, and it is safe in a WIQL literal and a `System.Tags` value | A ULID or UUID library dependency; a shorter key whose collision margin depends on workspace size |
| `--allow-remote-drift` is plan-only | The override changes what will be written, so it belongs in the artifact the operator reviews and in the `plan_id` that identifies it; an apply-time flag would let a reviewed plan be executed with different semantics | An apply-time flag or a `WithAllowRemoteDrift()` option |
| `orphan_remote` reported by `status` and `verify` only | Archived, abandoned, and shipped artifacts are outside `eligible_statuses`, so `plan` cannot select them and therefore cannot warn about them; a report-only scan over durable link records is the only reachable path | Claiming `plan` emits the warning; widening push selection to reach it |
| Correlation scoped to the selection, fail closed on truncation | A project-wide prefix query can be truncated, and a missed match becomes a duplicate work item | One project-wide WIQL |
| Frozen plan payload plus binding digests | Otherwise a changed mapping between `plan` and `apply` silently alters a reviewed write | Rebuilding the patch at apply time |
| Content-addressed `plan_id` | A timestamped id contradicts the stability requirement for an identical re-plan | Timestamp in the id |
| Workspace apply lease | Azure DevOps has no create-if-absent primitive, so a correlation preflight cannot serialize two concurrent applies | Relying on correlation alone |
| Frontmatter authoritative for resume | The ledger is gitignored and can vanish while the durable link record survives | Ledger-authoritative resume |
| Exhausted-budget class follows the terminal cause, indeterminacy dominant | A 429 was never applied, so it stays `throttled` and retryable; a timeout or 5xx may already have persisted, so it stays `indeterminate` and becomes `pending_verify` | A per-action rule that classified an exhausted `update` as `permanent` while marking it retryable |
| `verify` confirms semantically | A correlation tag proves existence, not that an update landed; re-baselining on existence silently loses a push | Re-baseline on tag match |
| Distinct `ExternalWriteError` from `ErrWriteIndeterminate` | The local sentinel has special envelope and MCP behavior; overloading it would misroute recovery | Reusing the durability sentinel for remote outcomes |
| Host allowlist plus `pat_env` prefix | The config file is repo-controlled; without both, a pull request can exfiltrate an arbitrary secret to an arbitrary host | Absolute-HTTPS validation alone |
| Renderer escapes raw inline HTML | `migrate` ingests externally authored bodies, so repo content is not a guaranteed trust boundary | Trusting repo Markdown; adding a sanitizer dependency |
| `agent_apply` config gate, default false | The plan/apply split does not constrain an agent that can call both; gating on config preserves operation parity while requiring a human opt-in | Removing `apply` from MCP entirely |
| `resolution` plus `remediation` on every conflict | All three recovery levers are CLI-only, so without an escalation contract an agent stalls permanently | Leaving conflict handling implicit |
| Interactive approval gate for the three levers | A registry marker documents intent; it does not gate execution, and Constitution VII requires approval | Registry marker alone |
| One consolidated PATCH per artifact per cycle | Work items cap at 10,000 revisions; chatty per-field updates burn that budget and TSTUs | Per-field patches |
| HTML only, no `text_format` knob | No requirement asked for it, it doubles renderer branches, and toggling it churns every work item revision | A selectable format |
| No `--query` selector | Unowned scope over a non-authoritative data source | An arbitrary SQL selection mode |
| No `Relate` on `Provider` | Relations ship in the create body and v1 never removes one, so the method would be dead API | A relation method for symmetry |
| Promote `blackfriday/v2` to direct | Already in the module graph via `cobra -> go-md2man` | Adding `goldmark` or a sanitizer module |
| `backlogit ado`, not `backlogit sync --target` | `sync` already means "rebuild the SQLite index" | Overloading the existing verb |

## Compatibility and Migration Strategy

**`migration.yaml`.** Unchanged, byte for byte. `MigrationConfig`,
`LoadMigrationConfig`, `WriteMigrationDefaults`, `ResolveArtifactType`, and
`MatchClass` keep their signatures and semantics; `backlogit migrate` behavior
is untouched, asserted by the existing migrate tests staying green with no
fixture edits. U49 states explicitly that `migration.yaml` governs one-time
inbound ingest and `integrations/azure-devops.yaml` governs outbound
synchronization. A future consolidation under a `migration.yaml` `targets:` key
remains possible because the loader is a single function over a self-contained
struct.

**`FieldConfig.ExternalMap`.** Retained and functional, with a `// Deprecated:`
comment directing new configuration to the provider-scoped `value_map`.
Resolution order is `value_map`, then `ExternalMap`, then the literal value, so
a workspace that already populated `external_map` starts working without edits.
`core.TranslateExternalMap` gains only a nil guard (U18); its behavior for a
non-nil config is unchanged. A doctor advisory that diffs the two mapping
sources was cut during review as an unrequested feature in an untouched
subsystem; it is recorded as a follow-up.

**Existing workspaces.** No artifact rewrite, no schema migration, no backfill.
An artifact with no link record is `unlinked` and plans as a `create`, or as an
`adopt` when a correlation tag already exists remotely. `EnsureSchema` adds
`external_links` idempotently on the next `sync`, matching the existing additive
pattern. A workspace whose persisted `state_map` exactly equals a frozen prior
generated default is upgraded at load time (U3) rather than hard-failing when a
new `ArtifactStatus` is added.

**Feature absence is inert.** Without `integrations/azure-devops.yaml`, every
`backlogit ado` command returns `ErrIntegrationConfigMissing` and nothing else
in backlogit changes behavior.

## Risks and Caveats

| Risk | Impact | Mitigation |
|---|---|---|
| Six Azure DevOps behaviors unverified against a live tenant | The client misclassifies an error, or a drift is treated as a hard failure | Conservative fail-closed defaults; U50 verifies each and U51 encodes the result |
| `[System.Tags] CONTAINS` semantics may be substring rather than exact-tag | The correlation index over- or under-matches | WIQL narrows the candidate set; exact tag matching is always done client-side after the batch read |
| WIQL chunk truncation on a very large project | Correlation coverage incomplete | Fail closed: `correlation_truncated` blocks the plan rather than proposing a create |
| Correlation tag edited or removed by a human | Adoption misses | `adopt` is never automatic; it appears in the plan for operator review, and `--force-relink` is the gated manual repair |
| Frontmatter link record conflicts on a Git merge | Two branches disagree on `rev` | `external.key` and `work_item_id` are immutable after first link, so identity survives; `verify` re-derives `rev` and `content_hash` semantically |
| Link-record writes add Git churn to artifact files | Noisier diffs and more merge contention on synced artifacts | `noop` never rewrites; U49 documents committing link-record writes as a separate synchronization commit |
| Two workspaces cloned from the same repo apply concurrently | The lease does not serialize them | Documented limitation; correlation preflight plus fail-closed truncation reduces but does not eliminate the window |
| No CI integration test against a real organization | A transport regression escapes unit tests | Recorded-fixture `httptest` harness per endpoint plus U50; both recorded in closure |
| Rate limiting during a large first sync | A bulk apply is throttled mid-run | Client limiter, `Retry-After`, capped backoff, run deadline, `--resume`, and a documented recommendation to sync one shipment at a time |
| Deleted remote work item | A linked artifact points at nothing | `errorPolicy: Omit` surfaces `orphan_local`; a re-plan proposes a `create` |
| `internal/core` grows further | Cohesion pressure on an already broad package | The external-sync files are a named, self-contained file set with a single entry point; a later extraction into an application service is recorded as a follow-up |

## Rollout, Feature Gating, and Operations

**Gating.** By configuration presence, not a build flag. The `backlogit ado`
group is always registered so `--help` is discoverable, but every subcommand
returns `ErrIntegrationConfigMissing` until a workspace opts in. Agent-initiated
`apply` requires the additional `agent_apply.enabled` opt-in.

**Rollout sequence.**

1. `backlogit ado discover` — read-only; proves connectivity, credentials, scope.
2. `backlogit ado plan --shipment <id>` — read-only; proves the mapping.
3. `backlogit ado plan --shipment <id> --validate-remote` — `validateOnly=true`;
   proves creates would be accepted, persisting nothing.
4. `backlogit ado apply --plan <id>` on a scratch project.
5. Repeat 2 and 4 against the real project, one shipment at a time.

**Telemetry (U46).** Per apply run: `created`, `updated`, `adopted`, `noop`,
`deferred`, `conflicts`, `failed`, `pending_verify`, `requests`,
`throttled_429`, `retries`, `duration_ms`, emitted through the existing
`events.TelemetryWriter`.

**Monitoring signals.** Healthy: `conflicts` and `pending_verify` zero,
`throttled_429` under one percent of requests, `noop` dominating a re-run of an
unchanged selection. Degrading: `conflicts` rising across runs;
`throttled_429` above five percent; `pending_verify` surviving two `verify`
runs.

**Rollback.** There is no automated remote rollback, deliberately.

* A bad *plan* is discarded by deleting the gitignored plan file. Nothing was
  written.
* A bad *apply* is contained: local link records revert with an ordinary
  `git revert` of the artifact commit. Remote work items created by the run
  remain, carry their correlation tag, and are enumerated in the apply result so
  an operator can remove them through Azure DevOps.
* A bad *mapping* is corrected in the integration file and re-planned; the next
  apply converges because updates are hash-driven.

**Rollback triggers.**

1. Any apply reporting a non-zero `failed` count with `class: validation` — the
   mapping is wrong. Stop applying, correct the config, re-plan.
2. `conflicts` exceeding ten percent of the selection — the drift assumption is
   wrong. Stop; do not reach for `--allow-remote-drift`.
3. Any credential appearing in a log, plan file, or error — revoke the PAT
   immediately and treat it as a security incident.

**Observation window.** Twenty-four hours after the first apply against a real
project, during which the operator re-runs `backlogit ado plan` on the same
selection and confirms it reports all `noop`. Owner: the operator who ran
`apply`; synchronization is never unattended.

## Runtime Verification and Closure

| Unit range | Runtime surface changed | Verification before absorption |
|---|---|---|
| U1-U4 | None (config parsing) | Unit tests; U2 is the security gate |
| U5-U13 | Outbound HTTP | Recorded-fixture `httptest` per endpoint; U50 live confirmation |
| U14-U23 | None (computation and translation only; the U15 adapter issues no request itself) | Table-driven unit tests, with U15 driven against a fake `ado` client |
| U24-U28 | Local persistence and SQLite schema | Round-trip, provenance-retention, and rehydrate tests; `backlogit sync` on this repository leaves the projection empty and the suite green |
| U29-U39 | External writes and local durable writes | Fake-provider contract tests, seam-injected durability tests, plus U50 |
| U40-U43 | CLI | `backlogit ado --help`, `discover`, `plan`, `plans`, `status` executed against the scratch project, including with a relative `--cwd` |
| U44-U45 | MCP | Registration, agent-gate, drift, and governed behavioral parity tests, plus a live `backlogit_ado_plan` call through the MCP server |
| U46-U49 | Telemetry and docs | `make docs-lint`, schema drift test, regenerated CLI reference |
| U50-U51 | External system | The scenario table below |

**Environment prechecks before any live run.**

1. A **scratch** Azure DevOps project, never production. Organization and
   project name are recorded in the closure artifact.
2. A PAT scoped to `vso.work_write`, bound to that organization only, with an
   expiry no longer than the verification window.
3. `backlogit ado discover` succeeds and the snapshot lists the expected work
   item types. A discovery failure blocks every later step.
4. A clean working tree, so link records written by `apply` are the only diff.

**Target scenarios.**

| Scenario | Expected observable |
|---|---|
| `plan` on a fresh selection | All `create`; zero remote writes confirmed in the project activity feed |
| `apply` that selection | Work items created with both tags and the parent relation |
| Immediate re-`plan` | All `noop` — the idempotency proof |
| Edit one field locally, re-`plan` | Exactly one `update` naming the changed field |
| Edit the same work item in the Azure DevOps UI, re-`plan` | `conflict` with `remote_rev` greater than `expected_rev`, plus `resolution` and `remediation` |
| Delete the local link record, re-`plan` | `adopt`, not `create` — the crash-window proof |
| Re-parent a linked artifact with `adopt`, re-`plan` | Still linked; no duplicate work item |
| Delete the remote work item, re-`plan` | `orphan_local` warning, not a crash |
| Ship a linked artifact, then run `status` and `verify` | `orphan_remote` reported by both, with `action: report_only`; the same selection re-`plan`ned omits the artifact entirely and reports no warning |
| `apply` with a deliberately invalid `area_path` | Blocked at plan validation, before any request |
| Two concurrent `apply` runs | The second fails with `ErrExternalApplyLeaseHeld` |
| CLI invoked with a **relative** `--cwd` | Every path resolves; no double-join failure |
| `ado plan --allow-remote-drift` on a non-TTY without acknowledgement | Refused by the approval gate |
| `ado plan --allow-remote-drift --i-understand --reason <text>` after a real drift | A plan whose action carries `skip_revision_test: true` and the reason, with a `plan_id` distinct from the ungated plan; `ado apply` rejects the same flag as unknown |
| MCP `backlogit_ado_apply` without `agent_apply.enabled` | `agent_apply_disabled` with a remediation; no request issued |
| MCP `backlogit_ado_apply` without `plan_id` | Structured validation error; no request issued |
| Failed `/rev` test op | Observed HTTP status and `typeKey` recorded for U51 |

**Blocked-path handling.** If no scratch organization is available, U50 is
recorded as `blocked`, not skipped. The feature may still merge, but the closure
artifact must state plainly that live verification did not occur, list the six
spike unknowns as unresolved, and leave the conservative defaults in place.
Shipping with a blocked U50 requires explicit operator acknowledgment, because
it means the error-classification table is unproven.

**Closure artifacts required before absorption.**

* `docs/closure/{date}-azure-devops-sync-runtime-verification.md` — the U50
  record, including resolved answers to all six spike unknowns, every use of a
  destructive lever with its recorded reason, and the final `ActionResult` for
  PA-1 through PA-5
* `docs/design-docs/azure-devops-sync-contract.md` — the durable design record
* A corrected `docs/ARCHITECTURE.md` dependency block
* Confirmation that the repository merge strategy is merge-commit and that
  squash and rebase merging are disabled
* A compound learning if live verification contradicts any Azure DevOps
  behavior recorded here

## Constitution Check

| Principle | Verdict | Note |
|---|---|---|
| I. Safety-First Go | pass | Go 1.24; no `unsafe`; every error wraps with `%w`; new sentinels and the typed `ExternalWriteError` live in `internal/errors` |
| II. Test-First Development (NON-NEGOTIABLE) | pass | Every unit is test-first; U33 is an explicit RED harness with a signature-only stub so it compiles and fails on assertions rather than on the build |
| III. Workspace Isolation and Security Boundaries | pass | Paths resolve through `SafeResolve` and `WorkspaceStorageRoot` and are normalized with `filepath.Abs`; the PAT is environment-only, prefix-restricted, and redacted; the target host is allowlisted |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | pass | Every file written is inside the workspace tree; the only out-of-tree effect is the intended Azure DevOps REST call, which is the feature |
| V. Structured Observability | pass | Four JSONL event types, telemetry counters, and structured `plan`, `apply`, `plans`, `status`, and `verify` payloads with closed vocabularies |
| VI. Single Responsibility | pass | One dependency change: promoting the already-present indirect `blackfriday/v2`. One `Provider` interface with one implementation and no `Relate` method. `--query`, `text_format`, and the `entra` placeholder were cut as unrequested |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | pass | U43 implements a real approval gate: TTY confirmation, or `--i-understand` plus a recorded `--reason` on a non-TTY, with intercom routing when available and a P-005 halt on bypass. The registry marker documents intent; U43 enforces it |
| VIII. Explicit Safety Modes | pass | The plan/apply split is freeze-scope made structural; `apply` executes a frozen specification and synthesizes nothing |
| IX. Git-Friendly Persistence | pass | Durable state is YAML frontmatter with deterministic key ordering and atomic temp-plus-rename writes, plus append-only JSONL; SQLite stays disposable and gitignored |
| X. Agent Context Efficiency | pass | MCP tools return structured summaries; `status` answers from the projection and reports `projection_stale` rather than a wrong count; `ado plans` gives agents a read path to plan state |
| XI. Merge Commit History Preservation (NON-NEGOTIABLE) | pass | Verification that squash and rebase merging are disabled is an explicit closure-artifact item, not an assumption |
| Overlay: backlogit | pass | Runtime plan files are run artifacts, not a parallel task store; the projection is refreshed and staleness is surfaced |
| Overlay: agent-intercom | pass | U43 routes destructive-lever approval through the intercom workflow when the pack is enabled, and falls back to local TTY confirmation otherwise |
| Overlay: agent-engram | N/A | The plan adds no indexed-search surface |
| Overlay: strict-safety | pass | Risky actions are classified below with `ProposedAction`, `ActionRisk`, and `ActionResult`, and carried into closure |

Constitution Check: pass

## Plan Hardening Signals

| Signal | Present | Justification |
|---|---|---|
| Public API, schema, or contract change | yes | New config schema, new frontmatter contract, new SQLite table, six CLI commands and six MCP tools |
| Security, auth, permission, or compliance-sensitive behavior | yes | Credential handling, host allowlisting, redirect policy, query and HTML injection surfaces, an authenticated write path into a customer planning system |
| Migration, backfill, destructive data action, or irreversible step | yes | Remote work item creation is not locally revertible; no local migration or backfill is required |
| External integration, operator checkpoint, or external dependency | yes | Azure DevOps REST is the feature; `apply` and the approval gate are operator checkpoints |
| High runtime, rollout, or rollback risk | yes | Throttling, partial application, and drift are live-system risks; remote rollback is manual by design |

Requires plan hardening: yes

## Unresolved Operator Decisions

Configuration-level choices with safe defaults already encoded. None blocks
implementation.

1. **`state_map` for `blocked` and `review`.** Defaults to `Active` because every
   default process has that state. A workspace modelling blocked work distinctly
   may prefer a custom inherited state.
2. **Whether `archived` and `shipped` artifacts stay in the selection.** The
   default `eligible_statuses` excludes them, so a shipped artifact stops
   updating remotely and is reported as `orphan_remote` by `backlogit ado
   status` and `backlogit ado verify` — never by `plan`, which cannot select it.
   Including them in `eligible_statuses` would instead push the mapped closed
   state to the remote when backlogit ships. Neither choice makes any command
   delete a work item.
3. **Whether to write a provenance note to `System.History`.** The plan does
   not, to conserve the revision budget and avoid notification noise.
4. **Whether link-record writes are committed with the authored artifact change
   or as a separate synchronization commit.** U49 documents the separate-commit
   convention as the default.

One item blocks **closure**, not implementation:

* **Availability of a scratch Azure DevOps organization for U50.** Without it,
  the `/rev` test-failure status and `typeKey` remain unverified. Implementation
  proceeds on the conservative default — an unclassifiable 4xx is a hard
  per-artifact failure, never a silent overwrite — so the failure mode is a
  false `failed` rather than an incorrect write. It must be visible in the
  closure record rather than assumed away.

## Future Provider Seams (No Implementation Tasks)

Recorded so a later provider is additive. **No unit implements any of them.**

* **Jira.** Would add `internal/jira` mirroring `internal/ado` and
  `internal/extsync/jira.go` implementing `Provider`. Its correlation key would
  be a label; its concurrency primitive is the issue `version` field rather than
  `/rev`. `MappedItem`, `Plan`, the state machine, hashing, and the result
  shapes are unchanged.
* **GitHub Issues and Projects.** Would add `internal/ghissues` and
  `internal/extsync/github.go`. Its correlation key would be a label or a hidden
  marker; it has no revision counter, which is why `RemoteRef.Version` is an
  opaque string the adapter alone interprets rather than an `int`.
* **Multi-target workspaces.** Would require the config to become a list; the
  frontmatter link record already keys on provider name
  (`custom_fields.external.{provider}`) and the stable `external.key` is
  provider-independent, so no schema change is needed.
* **Pull and bidirectional merge.** Would build on
  `GET .../wit/reporting/workitemrevisions` with a persisted `continuationToken`
  cursor and a per-field authority policy. That cursor is workspace-scoped
  rather than artifact-scoped and is the one piece of state this plan
  deliberately does not model.
* **Extraction of an external-sync application service.** If a second provider
  or a second external integration lands, the `internal/core/external_sync_*`
  file set is the natural extraction boundary; it already has one entry point
  and composes ports for artifact access, link persistence, audit writing, plan
  storage, and remote execution.

## Plan Hardening

Hardening was required: all five signals are present.

### Context consulted

| Source | Applied as |
|---|---|
| `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` | U35 envelope semantics and `durabilityOutcomeResult` reuse in U44 |
| `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md` | The `externalLinkWriteFn` seam and the no-`t.Parallel` rule in U35 |
| `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md` | Arrays-always result contract and empty-case assertions in U44 |
| `docs/compound/2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md` | U29 path construction and the relative-root test; relative `--cwd` in U50 |
| `docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md` | U45 in-process handler dispatch and three-representation assertions |
| `docs/compound/2026-07-23-machine-readable-governance-field-contract.md` | The closed vocabularies in the state machine plus the U49 drift test |
| `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` | U24 Markdown-first load and provenance-retention tests |
| `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | U24 `findArtifact`, never `loadArtifact` |
| `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md` | U28 threaded `*events.EventWriter` |
| `docs/compound/2026-07-06-external-process-timeout-before-probe.md` and `2026-07-06-bounded-helper-timeout-hard-cap.md` | Request timeout, discovery probe timeout, and the one-directional hard cap in U7 |
| `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` | U45 drift tests and the U49 discoverability-guide update |
| `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md` | The generalized `cli_only_flags` parity test in U45 |
| `docs/compound/2026-05-07-mcp-cli-config-parity.md` | Config loaded inside the shared core entry point (U31) |
| `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` | `items` re-upsert in the local write triple (U35) |
| `docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md` | N-pair tests in U17 and U21 |
| `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md` | `models.NowUTC()` for `synced_at` (U24) |
| `docs/compound/2026-07-23-persisted-config-load-time-default-map-upgrade.md` | The `state_map` load-time upgrade rule (U3) |
| `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md` | The setter-level guard in U25 |
| `docs/compound/2026-07-22-composite-index-prefix-does-not-obsolete-narrow-index.md` | `EXPLAIN QUERY PLAN` evidence in U26 |
| `docs/compound/2026-08-02-variadic-options-backward-compatible-shipment-creation.md` | Variadic `ExternalSyncOption` in U34 |
| `docs/design-docs/governed-mutation-recovery-contract.md` | Nested `local_partial` payload and classification precedence |
| `docs/design-docs/governed-operation-parity.md` | `governed`/`governed_name` markers and the asymmetry rule |
| `.github/instructions/constitution.instructions.md`, `strict-safety.instructions.md`, `backlogit.instructions.md` | Constitution Check rows and the risky-action vocabulary |

### Protected invariants

A change that breaks one of these is a stop condition, not a review finding.

1. **Markdown remains authoritative.** No path in `internal/extsync` or
   `internal/ado` writes a backlogit artifact field from remote data. Only the
   link record is written by sync, from locally computed values plus the remote
   id and rev.
2. **`plan` never writes to Azure DevOps.** Asserted by U47's recording
   transport.
3. **`apply` cannot exceed its plan.** Every remote write is a frozen action
   entry; nothing is synthesized at apply time. In particular `apply` cannot
   add, drop, or weaken the `/rev` test: `skip_revision_test` is frozen at plan
   time behind the approval gate, and no apply-side flag or option can set it.
4. **Indeterminate is never compensated and never retried.** Inherited from the
   governed mutation recovery contract, with the envelope's real continuation
   semantics respected in U35.
5. **No credential leaves the process.** No token in config, flags, plan files,
   ledgers, snapshots, JSONL, frontmatter, telemetry, or errors. Asserted by U48.
6. **v1 never deletes remote state.** No `DELETE` method and no `Destroy`-named
   symbol exists in `internal/ado`. Asserted by U47.
7. **Archive provenance and `item_links` survive every link-record write.**
   Asserted by U24.
8. **Rendered HTML contains no raw passthrough HTML.** Asserted by U19.
9. **The correlation key is immutable and well-formed.**
   `custom_fields.external.key` is minted once as 32 lowercase hexadecimal
   characters and never rewritten, including across an artifact ID rewrite.
   Asserted by U24 and U39.
10. **`migration.yaml` behavior is byte-unchanged.** Asserted by the existing
    migrate tests staying green with no fixture edits.
11. **`internal/extsync` never imports `internal/core`.** Asserted by U47.
12. **No command produces a remote deletion action.** `plan` and `apply` emit
    no deletion, and the `orphan_remote` finding produced by `status` and
    `verify` is report-only. Asserted by U38, U40, and U47's negative-space
    scan of `internal/ado`.

### Risky actions

| ID | ProposedAction | Targets | Change kind | ActionRisk | Approval | Rollback | ActionResult |
|---|---|---|---|---|---|---|---|
| PA-1 | Create work items in a customer Azure DevOps project | Remote work items | External write, irreversible from backlogit | high | Operator runs `apply` against a reviewed `plan_id`; agent surface additionally requires `agent_apply.enabled` | Not automated; created items are enumerated in the result and carry the correlation tag for manual removal | planned |
| PA-2 | Update existing work items, including `System.State` | Remote work item fields | External write, contract-affecting | high | Same plan gate; the `/rev` test prevents silent overwrite | Re-plan and re-apply from a corrected mapping; remote revision history retains prior values | planned |
| PA-3 | Authorize overwriting concurrent remote edits by planning with `--allow-remote-drift` | Frozen plan action, then remote work item fields when that plan is applied | Plan-time authorization of an external overwrite | destructive | U43 gate at **plan** creation: TTY confirmation, or `--i-understand` plus a recorded `--reason`; intercom routing when available. The approval and `skip_revision_test: true` are frozen into a plan with its own `plan_id`; `apply` adds no further lever | Discard the plan before applying; after applying, prior values are recoverable only from Azure DevOps revision history | planned |
| PA-4 | Skip work item type rules via `--bypass-rules` | Remote project validation | External write bypassing server validation | destructive | U43 gate | Re-apply with rules enabled after correcting the mapping | planned |
| PA-5 | Rewrite a link record without remote confirmation via `--force-relink` | Local frontmatter | Local state change, recovery-only | high | U43 gate | `git revert` of the artifact commit | planned |
| PA-6 | Resume an interrupted apply | Remote work items | External write replay | high | Inherits the `apply` gate; creates are re-correlated before re-issue | `verify` reconciles; duplicates are detectable by correlation key | planned |
| PA-7 | Add the `external_links` table | Local ephemeral cache | Additive schema change | low | Not required | Delete the database and `backlogit sync` | planned |
| PA-8 | Write the link record into artifact frontmatter | Local Git-tracked Markdown | Durable contract change | moderate | Not required | `git revert`; the record is additive under `custom_fields` | planned |
| PA-9 | Promote `blackfriday/v2` to a direct dependency | Module graph | Dependency change | low | Not required | Revert the `go.mod` and `go.sum` hunk | planned |

`ActionResult` stays `planned` until the corresponding unit lands. PA-1 through
PA-6 must appear in the review artifact and the closure record with their final
`ActionResult`, and every use of PA-3, PA-4, or PA-5 must carry its recorded
reason.

### Human checkpoints

1. Reviewing `plan` output before `apply`. Structural: `apply` requires a
   `plan_id`.
2. Authorizing `--allow-remote-drift`, `--bypass-rules`, or `--force-relink`
   through the U43 gate, with a recorded reason. `--allow-remote-drift` is
   authorized when the plan is computed, so the reviewed plan already states the
   overwrite.
3. Opting a workspace into `agent_apply.enabled`.
4. Approving the first `apply` against a non-scratch project.
5. Acknowledging a blocked U50 if live verification is impossible.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: ADVISORY

TOOL_OK: reviewer-subagent-dispatch

### Gate rationale

Seven personas were dispatched as independent sub-agents across four model
tiers, and all seven returned findings. Coverage was complete, so the
`multi-agent-dispatch` label holds.

The first-pass gate decision was **FAIL**: the reviewed draft carried 4 P0 and
24 P1 findings after deduplication. Every P0 and every P1 has been remediated in
the body above, and the plan was re-derived rather than annotated. The residual
set is P2 and P3 only, which is `ADVISORY` under the gate table. Because this
plan is not being harvested in this session, no `operator_authorization` marker
is recorded; harvest remains blocked pending explicit operator approval.

Plan hardening was required and is satisfied: the `## Plan Hardening` section
carries protected invariants, a nine-row `ProposedAction` table with
`ActionRisk` and `ActionResult`, human checkpoints, deepened runtime
verification, and rollback triggers with named metrics.

### P0 findings (all remediated)

| # | Persona | Finding | Remediation |
|---|---|---|---|
| P0-1 | Security Lens | SSRF and secret exfiltration: a repo-controlled config could point `organization_url` at an attacker host and `pat_env` at any environment variable, sending that secret as a Basic auth header | Host allowlist plus `^BACKLOGIT_ADO_[A-Z0-9_]*$` on `pat_env` (U2); `CheckRedirect` rejects cross-host redirects and strips `Authorization` (U7) |
| P0-2 | Go Reviewer | `internal/sync` could not be pure and own snapshot/plan persistence without importing `internal/core` for `SafeResolve` and `WorkspaceStorageRoot` — a compile-time import cycle, so "no cycle is introducible" was false | Persistence moved to `internal/core/external_sync_store.go` (U29); `extsync` is value types plus codec only; U47 asserts the direction with `go list -deps`. Refined by F-1 below: the package is core-independent and filesystem-free, not fully pure |
| P0-3 | Learnings Researcher | Writing the link record via `UpdateArtifact`/`loadArtifact` reintroduces two solved bugs: the typed round trip drops `archived_from`/`archived_status`, and a `loadArtifact`-first re-persist destroys `item_links` | U24 loads through `findArtifact` with a single load for guard and persist; U25 refuses archived and excluded-type artifacts; provenance and `links` retention are explicit test scenarios and protected invariant 7 |
| P0-4 | Constitution Reviewer | Principle VII marked `pass` while the destructive levers had only a registry label, not an approval mechanism | U43 adds a real gate: TTY confirmation, or `--i-understand` plus a recorded `--reason` on a non-TTY, intercom routing when available, and a P-005 halt on bypass |

### P1 findings (all remediated)

| Persona | Finding | Remediation |
|---|---|---|
| Security Lens | Agent can call `plan` then `apply`, bypassing the intended human review boundary | `agent_apply.enabled` config gate, default `false`, with a structured refusal and remediation |
| Security Lens | Stored XSS: unsanitized `blackfriday` output written into `System.Description` | U19 configures the renderer to escape raw inline HTML; protected invariant 8 |
| Architecture, Learnings | Mutable artifact IDs used as the correlation identity; `AdoptItem` rewrites IDs | Stable `custom_fields.external.key`; `backlogit-id:` demoted to a non-authoritative display tag; U39 adds an ID-rewrite migration contract |
| Architecture | Project-wide WIQL correlation index can truncate and produce duplicate creates | Correlation scoped to the selection, chunked at `wiql_chunk_size`, `correlation_truncated` blocks the plan (U12, U22, U31) |
| Architecture | Concurrent applies can duplicate remote work items; no lock | U32 workspace apply lease, with its single-workspace scope stated |
| Architecture | The plan was not an immutable execution contract; a changed mapping could alter a reviewed write | Frozen resolved payloads plus `config_digest`, `discovery_digest`, `workspace_target`, `selection_digest`; content-addressed `plan_id` (U23) |
| Architecture | Correlation tags cannot confirm an indeterminate update; re-baselining on existence loses a push | U38 semantic confirmation against the frozen payload |
| Architecture | Governed recovery data lost in the bulk result; remote unknown conflated with local durability | Nested `local_partial` `MutationPartialError`; distinct `ExternalWriteError` type (U8, U35) |
| Go Reviewer | `Provider` could not express `WriteOptions`, and `Read`/`Resolve` return shapes lost `orphan_local` and match multiplicity | U14 interface revised: `WriteOptions`, `ReadResult`, `ResolveResult`; `Relate` removed as dead API |
| Go Reviewer | Hardening amendment A contradicted the real `MutationEnvelope` indeterminate semantics (it continues, it does not halt) | U35 rewritten against the actual control flow; the JSONL step branches on `IsWriteIndeterminate` |
| Go Reviewer | `TranslateExternalMap` panics on a nil `*FieldConfig`, which is the ordinary case | U18 adds the nil guard plus test cases |
| Go Reviewer | WIQL built by string interpolation of operator- and artifact-controlled values | `quoteWIQLLiteral` plus `correlation_tag_prefix` charset validation (U2, U12) |
| Go Reviewer | U11's tests were a false green and inverted the dependency order | Interface assertion moved to U15; U14 gains a hand-written fake and JSON key-name round trips |
| Go Reviewer | Progress-ledger append failure unhandled; ledger treated as resume authority | U37: append errors surface as `failed`; frontmatter is authoritative; a deleted-ledger test asserts zero duplicate creates |
| Go Reviewer | Package named `sync` shadows the standard library | Renamed to `internal/extsync` |
| Learnings | Apply loop did not thread the caller's `*events.EventWriter`, breaking MCP append serialization | U28 and U34 take `ew *events.EventWriter`; U44 asserts the handler passes `s.Events` |
| Learnings | No per-request timeout or run deadline on the write path | `request_timeout_ms` with a one-directional hard cap, a shorter discovery probe timeout, and `run_deadline_seconds` (U7, U9) |
| Learnings | Registry work stopped at governed parity, skipping the honest-fallback drift tests and the discoverability guide | U45 covers all four live-tree drift tests; U49 updates the fallback guide |
| Learnings | N-independent-pair design applied only to hashing, not to the hierarchy ordering where the hazard lives | U21 uses N ≥ 8 independent pairs and makes ordering deterministic by sorted key |
| Parity | Agent dead-ends on `conflict` with no forward path or escalation signal | Every conflict entry carries `resolution` and `remediation` |
| Parity | Computed plans and ledgers were an agent-invisible surface | New `backlogit ado plans` / `backlogit_ado_plans` read path |
| Constitution, Scope | `apply --resume` could re-issue a create and duplicate a work item | Budget-exhausted creates classify `pending_verify`; resume re-resolves correlation before re-issuing (U36, U37). Refined by F-3 below: the class now follows the terminal cause, with indeterminacy dominant |
| Constitution | No unit owned `plan` orchestration; it would have landed in `internal/cli` | U30 selection and U31 plan orchestration added in `internal/core` |
| Scope | `--bypass-rules` and `--force-relink` were declared but unowned | Owned by U43 with named test scenarios and registry entries |
| Scope | U27 was two or more units in disguise | Split into U34 (loop) and U35 (local triple) |

### P2 findings (accepted, addressed in-plan)

Granularity breaches (U1, U6, U7, U8, U9, U14, U16, U32 in the draft) are
resolved by the restated rule — at most 2 production files and at most 3 test
scenarios per unit — and by splitting the offenders: draft U9 became U11 and
U12, draft U31 became U40 and U41, draft U32 became U42 and U43.

Also addressed: orphan units and the misattributed telemetry row in the
requirements trace; the reversed U11/U12 dependency arrow; the undeclared
U38-to-U6 feedback edge, now the explicit conditional U51; missing acceptance
criteria on most units; the two inconsistent error-class vocabularies, now one;
outcome and summary vocabularies added to the closed set; `projection_stale`
detection in `status`; `EXPLAIN QUERY PLAN` evidence for the `external_links`
indexes; `context.Context` and bound parameters on every new `db` helper; the
`externalLinkWriteFn` seam for injecting a local indeterminate write;
`models.NowUTC()` for `synced_at`; the `state_map` load-time upgrade path;
`items` re-upsert on the success path; the setter-level eligibility guard;
config loaded inside the shared core entry point; the generalized
`cli_only_flags` parity test; `external_links` reaching agents through the
metadata catalog's `sql_schema`; `validate_remote` exposed on both surfaces;
deterministic key ordering and atomic writes for the link record; and the
corrected `docs/ARCHITECTURE.md` dependency block rather than an append to a
stale one.

Scope reductions accepted: `--query`, the `text_format` knob, the
`entra_service_principal` placeholder and its sentinel, the implicit config
template writer, and the doctor advisory were all cut as unrequested scope. The
doctor advisory is recorded as a follow-up.

### P3 findings (advisory)

Accepted and applied: the `internal/release` "mirror" claim softened to
near-leaf; the `WebhookNotifier` characterization corrected to note the
`WaitGroup` drain; line-number citations replaced with symbol citations;
capability-overlay rows added to the Constitution Check; the merge-strategy
verification added to the closure checklist; the `Provider` adapter constrained
to translation only; and variadic `ExternalSyncOption` adopted for the apply
entry point.

Not adopted, with rationale: extracting a separate external-sync application
service now (recorded as a future seam instead — it is a refactor without a
second consumer, which Principle VI resists), and deferring the `Provider`
interface entirely until a second provider exists (the fake provider in U33 is a
genuine test seam that the interface exists to serve).

### Residual follow-ups

1. `backlogit doctor` advisory when `value_map` and `ExternalMap` disagree.
2. Extraction of `internal/core/external_sync_*` into a dedicated application
   service if a second external integration lands.
3. Serialization across independently cloned workspaces, which the apply lease
   does not cover.
4. `POST .../wit/$batch` as a throughput optimization once its documented
   per-request limit is confirmed.

### Focused post-review consistency pass

This subsection records a **subsequent focused architecture pass**, run after
the seven-persona gate above had already been completed and remediated. It was
not part of that review, no persona sub-agents were dispatched for it, and it
did not re-derive the plan. It read the remediated plan and its source spike for
internal contradictions only, and found five. All five are corrected in the body
above and in `docs/decisions/2026-08-20-azure-devops-sync-spike.md`; the review
history above is left intact.

| # | Severity | Inconsistency | Remediation |
|---|---|---|---|
| F-1 | P2 | `internal/extsync` was called a pure package with no filesystem access, while U15 places the Azure DevOps provider adapter — which performs the remote round trip — inside it, and the dependency map declares `extsync -> ado` | The package is now described as **core-independent and filesystem-free**: pure computation plus one translation-only adapter that delegates every network call to `internal/ado`. Blanket "pure package" claims are removed. The `extsync` must never import `core` invariant and the existing dependency direction are unchanged |
| F-2 | P1 | The external key was specified as "16 hex characters" while every example (`01J8ZC4M7QK3X9V2`) was Crockford-style, not hexadecimal | The key is now 128 bits from `crypto/rand` encoded with `encoding/hex` — **32 lowercase hexadecimal characters**, `^[0-9a-f]{32}$` — standard library only. Every example, tag, payload, validation rule, and U2/U12/U24 acceptance is updated. Local collisions are handled by checking existing durable keys and regenerating before persistence; remote duplicates remain the `Resolve` multiplicity case |
| F-3 | P1 | The failure table classified a budget-exhausted `update` as `class: permanent` while recording it retryable, contradicting the closed vocabulary in which `permanent` is the non-retryable class | Classification now follows the **terminal cause**, with indeterminacy dominant: an exhausted 429 sequence stays `throttled` and retryable; any timeout, 5xx, or transport failure after a possible send stays `indeterminate`, becomes `pending_verify`, and is never blindly retried; `validation` and `permanent` never enter the retry loop. The failure table, U8, U9, U36, the result contracts, and the vocabulary prose are aligned |
| F-4 | P1 | `--allow-remote-drift` was owned by both `ado plan` and `ado apply`, so an apply-time flag could change what a reviewed plan wrote | The flag is **plan-only** and approval-gated at plan creation. It freezes `skip_revision_test: true` plus the recorded approval reason into the action and therefore changes `plan_id`. `WithAllowRemoteDrift()`, apply-side flag ownership, and every claim that `apply` modifies or drops the revision test are removed; `apply` executes the frozen plan unchanged. The lever table, U22, U23, U34, U41 through U43, the registry block and parity assertions, remediation strings, and PA-3 are aligned |
| F-5 | P1 | `orphan_remote` was documented as a `plan` warning for archived, abandoned, and shipped artifacts, which push selection structurally excludes, so the warning was unreachable | `orphan_remote` is now the **report-only** responsibility of `backlogit ado status` and `backlogit ado verify`, which scan durable `external_links` and frontmatter independently of push eligibility. `plan` and `apply` continue to exclude ineligible statuses and still generate no deletion action. The state machine, the v1 contract, U25, U30, U38, U40, the requirements trace, the result examples, the runtime scenarios, and the unresolved-decision wording are aligned |

No new scope was added by this pass. Every change is a correction to an existing
claim, and no unit gained a requirement that the reviewed plan did not already
imply.

One stale cross-reference inside the approval-gate paragraph rewritten for F-4
(`U33` where `U43` is meant) was corrected with it. Other pre-existing unit
cross-reference drift elsewhere in the document was left untouched, because it
is outside these five findings.
