---
title: "Spike: Azure DevOps synchronization for backlogit artifacts and shipments"
source: docs/decisions/2026-08-20-azure-devops-sync-spike.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
description: "Findings on how backlogit should implement safe, idempotent one-way Azure DevOps work item synchronization for configurable artifacts, selected by shipment, driven by a target-mapping config."
docline:
    type: spike
    date: 2026-08-20
    time_box: "4h"
    conclusion: "proceed"
    confidence: "medium"
    linked_parent_work_item: null
    promoted_to:
        - "plan"
    plan_artifact: docs/exec-plans/2026-08-20-azure-devops-sync-plan.md
    tags:
        - "azure-devops"
        - "integration"
        - "synchronization"
        - "migration"
        - "backlogit"
---

## Goal

How should backlogit implement a safe, idempotent Azure DevOps synchronization
capability for configurable AutoHarness/backlogit artifacts and shipments,
using declarative mapping configuration, while preserving Markdown plus Git as
the local source of truth?

The spike must resolve the v1 contract — direction, authority, unit of sync,
eligibility, identity, concurrency, recovery, configuration home, package
ownership, surface shape, and credentials — with enough precision that an
implementation plan can be decomposed into atomic tasks without further
research.

## Success Criteria

* Every design question is answered with a concrete decision and a stated
  rejected alternative, not a menu of options
* Decisions are grounded in the backlogit code and configuration schema present
  on `main`, cited by file and symbol, not in aspirational documentation
* Azure DevOps behavior is grounded in current official Microsoft REST
  documentation with citable URLs, not in the reference Python loader
* The durable local-to-remote identity model is expressed in terms of
  backlogit's existing ownership rules: Markdown authoritative, SQLite
  disposable, JSONL append-only
* Idempotency, crash-window recovery, drift, and partial failure have named
  mechanisms
* The recommendation is one of proceed, pivot, defer, or abandon with a
  confidence rating

## Scope Constraints

* Documentation and research only. No prototype code, no production code, no Go
  source changes
* Azure DevOps only. Jira and GitHub Issues/Projects are out of implementation
  scope; only clean extension seams are preserved
* No backlog or queue mutation, no shipment creation, no harvest, no PR, no
  commit, no push
* The sibling repositories `azd-backlogbldr` and `azd-backlogloader` are
  read-only reference material and are not modified
* Workspace graph and search activity performed through the engram CLI against
  the backlogit workspace; direct reads used for known exact paths

## Investigation Approach

1. Bind and query the engram workspace index for prior art on migration
   configuration, external field mapping, provider integration, webhook
   delivery, and durable identity; establish whether prior Azure DevOps work
   exists in this repository.
2. Read the schemas and code paths that any synchronization feature must
   integrate with: artifact model and serialization, workspace and migration
   configuration, external field translation, migration import provenance,
   lifecycle hooks and webhook delivery, mutation envelope and partial-failure
   classification, governed CLI/MCP parity, and the SQLite cache boundary.
3. Reconfirm the reference workflow and reference importer behavior from
   `azd-backlogbldr` and `azd-backlogloader` source files.
4. Verify Azure DevOps REST behavior against current official Microsoft
   documentation for create, update, concurrency, batch read, relations,
   discovery, incremental pull, throttling, authentication, text formats,
   deletion, and error shape.
5. Convert the evidence into a decided v1 contract with named rejected
   alternatives and explicitly deferred work.

## Findings

### What Was Discovered

#### backlogit has the domain model but none of the integration machinery

The canonical artifact is `models.Artifact` (`internal/models/artifact.go`). It
carries `ID`, `Title`, `Status`, `ArtifactType`, `ParentID`, `Sprint`,
`Priority`, `Description`, `AssignedTo`, `Owner`, `Labels`, `Dependencies`
(typed `DependencyEdge`), `Links` (typed `ArtifactLink`), `References`,
`Commit`, archive provenance, and an open `CustomFields map[string]any`.
`Artifact.ToFrontmatterMap` is documented as the single source of truth for
which modeled fields are serialized, and it emits `custom_fields` verbatim when
non-nil. That is the only extension point that can carry new durable
per-artifact state without changing the typed model.

Artifact types are fully configurable.
`config.WorkspaceConfig.ArtifactTypes` (`internal/config/schema.go`) is a
`map[string]*ArtifactTypeConfig` with `Prefix`, `Suffix`, `NameFormat`,
`FileNameFormat`, and `AllowedChildren`. This repository's own
`.backlogit/config.yaml` declares nine types (`feature`, `deliberation`,
`task`, `subtask`, `bug`, `spike`, `chore`, `review`, `shipment`) across a
three-level `queue_layout`. A customer workspace can declare an entirely
different set — `epic`, `user_story`, `task` — so no mapping design may assume
the default names. Statuses are configurable through `config.Fields["status"]`,
though `models.ArtifactStatus` pins the ten-value enum in the validator tag.

`config.FieldConfig.ExternalMap` exists and is explicitly documented as
"translation rules for external systems (e.g., Jira, ADO)". Its only consumer,
`core.TranslateExternalMap` (`internal/core/fields.go:42-51`), has **zero
callers**: an engram `map-code` and `impact` query on the symbol returns only
the defining file edge and no incoming call edges. This repository's
`config.yaml` sets `fields.status.external_map: {}`. The hook exists, has never
been wired, and carries no provider dimension.

`migration.yaml` is inbound-only. `config.MigrationConfig`
(`internal/config/migration.go`) has exactly three fields: `document_classes`
(required, `min=1`), `default_layout` (required, one of
`flat|structured|mixed`), and `source_paths`. Its methods are
`ResolveArtifactType(className)` and `MatchClass(filePath)`. This is a
file-classification schema for a one-time ingest of foreign Markdown into
backlogit artifacts. It has no vocabulary for an external system, a project, a
work item type, a state, a field reference name, or an area path.
`.backlogit/migration.yaml` in this repository still holds the
`WriteMigrationDefaults` output for a Backlog.md layout.

The import path already models provenance and resume. `internal/cli/migrate.go`
builds an `existingImportIndex` keyed on source identity and records
`importedArtifactRef{id, archived}` so an interrupted import resumes rather
than duplicating, and it stashes unmapped type information into custom fields
(`fields["backlog_md_target_type"]`). `parser.MigrationItem` carries `SourceID`
and `SourcePath`. This is a working precedent for the identity and resume
problem, scoped to local ingest.

Lifecycle hooks exist and already dispatch outbound HTTP.
`hooks.WebhookNotifier` (`internal/hooks/webhook.go`) registers on every hook
point at priority 80, filters to top-level operations, dispatches
fire-and-forget goroutines behind a shared `rate.Limiter`, logs non-2xx as a
warning, and returns `nil` unconditionally. Its payload is deliberately compact
and explicitly excludes value maps and body content. It is a notification
channel, not a synchronization channel: no retry, no ordering, no result, no
durable record.

Multi-store write coordination is a solved, documented problem here.
`core.MutationEnvelope` (`internal/core/mutation_envelope.go`) executes ordered
named steps with compensation and classifies failures into `not-applied`,
`indeterminate`, and `double-fault`, with a strict precedence invariant: any
`ErrWriteIndeterminate` forces `indeterminate` and forbids compensation.
`docs/design-docs/governed-mutation-recovery-contract.md` defines the surfaces,
the doctor recovery checks, and the MCP structured error payload
(`error: mutation_partial`, `classification`, `completed_steps`, `failed_step`,
`compensation_state`, `retryable`, `recovery`).

CLI and MCP parity is governed, not aspirational.
`docs/design-docs/governed-operation-parity.md` records the F6 contract: one
shared `core` function per governed operation, a `governed: true` marker plus
`governed_name` in `.autoharness/backlog-registry.yaml`, and
`TestRegistryParity_GovernedOperationBehavioralParity` asserting identical
persisted state from both surfaces. It also records the load-bearing safety
rule that decides the MCP surface shape for this feature: *a fallback surface
must never be more dangerous than the surface it mirrors*, with `--force-gates`
and `--gate-base` documented as `human_terminal_only: true`.

The SQLite index is disposable. `docs/ARCHITECTURE.md` states the database at
`.backlogit/backlogit.db` is an ephemeral query cache rebuilt from Markdown,
and `.gitignore` excludes `backlogit.db`, `-shm`, and `-wal`.
`.backlogit/runtime/` is also gitignored. Per-item JSONL logs under the
workspace logs directory are append-only and already projected into
`item_logs` and `item_log_entries` with FTS. Any durable remote identity
therefore cannot live in SQLite.

Dependency direction is enforced and documented: `core -> models, db, mdfront,
atomicfile`; `mcp` must not import `cli`; `internal/release` is the precedent
for a stdlib-only outbound HTTP leaf consumed by both surfaces.

The module graph already contains a Markdown-to-HTML renderer.
`github.com/russross/blackfriday/v2 v2.1.0` is present in `go.mod` as an
indirect dependency via `cobra -> go-md2man`. Promoting it to a direct
dependency is materially cheaper under Principle VI than introducing a new one.

#### The reference workflow and the reference importer

`azd-backlogbldr` has no application runtime. Its tracked content is
`agent.md`, `README.md`, a Copilot instructions file, a product-owner agent
definition, two generated Markdown backlogs, one example YAML backlog, and a
workshop document. The YAML shape is a nested
`features[] -> user_stories[] -> {title, description, acceptance_criteria[]}`
tree. It is a prompt-driven authoring workflow that produces a shareable
backlog document, and it confirms the hierarchy customers expect: Feature, User
Story, Task.

`azd-backlogloader` is a single 30 KB Python script plus two YAML
configuration files. `config.parameters.yaml` carries `organization_url`,
`project`, `area_path`, `iteration_path`, and a `personal_access_token` field
with an inline instruction not to commit it, an opt-in `use_env_for_pat` flag
defaulting to `false`, and an `enable_markdown` formatting toggle.
`config.templates.yaml` is the useful half: it maps friendly field names to
`azure_field_path` reference names (`Microsoft.VSTS.Common.Priority`,
`Microsoft.VSTS.Scheduling.StoryPoints`,
`Microsoft.VSTS.Common.BusinessValue`,
`Microsoft.VSTS.Scheduling.OriginalEstimate`) with a declared `type` and
`required` flag, per work item type. That structure — per-type field maps keyed
on canonical reference names — is the right shape to carry forward. The
default-credentials-in-config shape is the wrong shape and must not be.

#### Azure DevOps REST behavior that constrains the design

The following are verified against current Microsoft documentation. Items that
could not be confirmed from an official page appear under "Remaining Unknowns"
rather than being asserted here.

Create is `POST .../_apis/wit/workitems/${type}?api-version=7.1` with
`Content-Type: application/json-patch+json` and a JSON **array** body of
`{op, path, value}` operations addressing `/fields/{referenceName}`. The
literal `$` is part of the path. Success is HTTP 200, not 201. The
`validateOnly`, `bypassRules`, and `suppressNotifications` query parameters are
first-class and are exactly the levers a sync client needs.

Update is `PATCH .../_apis/wit/workitems/{id}` with the same content type.
Optimistic concurrency is a JSON Patch test operation on `/rev` —
`{"op":"test","path":"/rev","value":N}` — not on `/fields/System.Rev`. This is
confirmed by the official sample requests on the Update reference page. There
is no documented ETag or `If-Match` alternative.

Batch read is capped at **200 ids** for both
`GET .../_apis/wit/workitems?ids=...` and
`POST .../_apis/wit/workitemsbatch`. Neither supports continuation. The
`errorPolicy=Omit` parameter tolerates ids that were deleted remotely, which is
exactly the drift case.

Hierarchy is a `relations` array entry. `System.LinkTypes.Hierarchy-Reverse`
points to the parent; `System.LinkTypes.Hierarchy-Forward` points to a child.
The relation topology is a tree with `singleTarget` and `acyclic` attributes,
so a work item has at most one parent and the server enforces it. Relations are
added with `{"op":"add","path":"/relations/-","value":{...}}` and removed by
array index, which shifts after each removal — a client must remove
highest-index-first or issue separate calls.

Incremental pull is `GET .../_apis/wit/reporting/workitemrevisions` with an
opaque `continuationToken` and an `isLastBatch` terminator. Microsoft's
integration best practices page states that using queries plus individual
get-work-item calls is the top cause of rate limiting, and directs integrations
to the reporting endpoints. WIQL returns ids and urls only, has no continuation
token, and must be followed by a batch read.

Throttling is measured in Azure DevOps Throughput Units with a global 200-TSTU
sliding five-minute window. Clients must honor `Retry-After` and may observe
`X-RateLimit-Remaining` and `X-RateLimit-Limit` when present. Work items are
capped at 10,000 revisions each, which penalizes chatty per-field updates and
rewards one consolidated PATCH per artifact per sync cycle.

Authentication: a PAT over HTTP Basic with an empty username
(`base64(":" + PAT)`) and the `vso.work_write` scope is the documented minimum
for a writing client. Azure DevOps OAuth apps are deprecated — no new
registrations since April 2025, with full deprecation in 2026 — and Microsoft
Entra ID is the recommended direction, with service principals and managed
identities fully supported and requiring the identity to be added to the
organization.

Text fields (`System.Description`, `Microsoft.VSTS.Common.AcceptanceCriteria`,
`System.History`) are typed `html` in the REST 7.1 schema. Markdown authoring
for work item text fields has been rolling out in the web UI, but the REST
surface still reads and writes HTML, and no per-field format companion could be
confirmed on a 7.1 reference page. Sending HTML is the safe universal choice.

Soft delete is `DELETE .../_apis/wit/workitems/{id}` into the recycle bin, with
`destroy=true` and `DELETE .../_apis/wit/recyclebin/{id}` as the irreversible
variants.

The GA documented version for the `wit` area is `7.1`. The docs site default
moniker has moved to `7.2`, but every WIT operation page consulted documents
`7.1`, and Microsoft's guidance is that every request must pin an
`api-version`.

### Decided v1 contract

Each decision states the decision, the reason, and the alternative rejected.

#### Direction and authority

**Decision.** v1 is one-way push (export) from backlogit to Azure DevOps.
backlogit's Markdown files are authoritative for every mapped field. Remote
reads are performed, but only to establish identity, revision, and drift — they
never write a backlogit artifact field. Import and bidirectional merge are
explicitly deferred.

**Reason.** Backlogit's ownership model already declares Markdown
source-of-truth and SQLite disposable. Bidirectional merge requires a per-field
authority policy and a conflict resolution surface that do not exist, and both
would have to survive Git merges. One-way push is the smallest contract that is
still honest, and it is precisely the capability `azd-backlogloader`
demonstrates demand for.

**Rejected.** Bidirectional sync in v1; remote-authoritative pull.

#### Unit of synchronization

**Decision.** The **artifact is the mapped unit**; the **shipment is the
selection scope**. `backlogit ado plan --shipment 128-S` resolves the
shipment's member artifacts plus their ancestors, and maps each artifact to one
Azure DevOps work item. A shipment itself is not projected into a work item in
v1. Explicit `--item` and `--query` selectors are also supported for artifacts
outside a shipment.

**Reason.** `core.CreateShipment` stores membership as `custom_fields.items`
and documents the shipment as "the aggregate root" over an item list — it is
already a selection construct, not a work item. Mapping shipments onto Azure
DevOps would force a choice between Epic, Iteration, and Tag, none of which is
correct for every process template.

**Rejected.** Projecting a shipment as an Epic (process-dependent, and wrong
for the Basic process); projecting a shipment as an Iteration (iterations are
dated schedule containers, shipments are not).

#### Eligibility and trigger

**Decision.** Synchronization is an **explicit two-phase command** — `plan`
then `apply` — and is never triggered by a lifecycle hook, status transition,
or webhook. No backlogit mutation path acquires a hidden external write.

**Reason.** `hooks.WebhookNotifier` establishes the repository's existing
stance: outbound HTTP from a lifecycle hook is fire-and-forget, returns `nil`
unconditionally, and deliberately carries no result. Promoting that channel to
a durable external write would make every local `move_item` a distributed
transaction. Explicit observability, destructive-command approval, and safety
modes all argue for an operator-initiated command over a reviewable plan.

**Rejected.** Auto-sync on `move_item` to `done`; auto-sync on
`shipment ship`; a background daemon.

#### Configuration home

**Decision.** Do **not** extend `migration.yaml`. Introduce a sibling
target-mapping schema at `{workspace}/integrations/azure-devops.yaml`, loaded by a
new `config.AzureDevOpsConfig`. `migration.yaml` is left byte-unchanged.

**Reason.** The evidence is structural, not stylistic.
`config.MigrationConfig` requires `document_classes` with `min=1` and its
entire vocabulary (`glob_patterns`, `keywords`, `source_paths`,
`default_layout`) describes classifying local files during one-time ingest.
Folding an outbound target map into it would force every workspace that wants
Azure DevOps sync to also declare inbound document classes it will never use,
and would force every migrating workspace to carry connection configuration.
The two schemas also have different lifetimes: `migration.yaml` is consumed
once and then inert; the target map is read on every sync.

The operator's stated intent — that migration configuration defines the mapping
from source and backlogit structures to target structures — is honored by *what
the new file contains*, not by which file it lives in. A consolidation seam is
preserved: because the loader is a single function over a self-contained
struct, a future minor version can accept the same block under a
`migration.yaml` `targets:` key without changing the internal model.

**Rejected.** Adding an optional `targets:` block to `MigrationConfig` in v1
(couples two unrelated lifetimes and puts connection settings behind a
`document_classes`-required validator).

#### Compatibility with FieldConfig.ExternalMap

**Decision.** Give `ExternalMap` and `TranslateExternalMap` their first real
caller as a **documented fallback**, and mark `ExternalMap` `// Deprecated:` in
favor of the provider-scoped `value_map` in the integration config. Resolution
order is integration config `value_map`, then
`WorkspaceConfig.Fields[name].ExternalMap`, then the literal local value.
`backlogit doctor` reports an advisory finding when both are set and disagree.

**Reason.** `ExternalMap` is `map[string]any` with no provider dimension, so it
cannot express two targets. Deleting it would break workspaces that populated
it in anticipation. Making it a fallback converts dead configuration into
working configuration without a breaking change, and mirrors the repository's
`core.LinkCommit` deprecation pattern.

**Rejected.** Deleting `ExternalMap`; making it authoritative over the
provider-scoped map.

#### Durable local-to-remote identity

**Decision.** Identity and last-synced state live in **artifact frontmatter**
under `custom_fields.external.azure_devops`. The audit trail uses the
**existing per-item append-only JSONL log**. A new SQLite `external_links`
table is a **disposable projection** rebuilt by `Rehydrate`. No new durable
file format is introduced.

The frontmatter record is deliberately small:

```yaml
custom_fields:
  external:
    azure_devops:
      org: contoso
      project: Platform
      work_item_id: 4821
      work_item_type: User Story
      rev: 7
      content_hash: "sha256:9f2c..."
      synced_at: 2026-08-20T18:04:11Z
      state: linked
```

**Reason.** Markdown is the only Git-tracked, merge-visible, source-of-truth
surface. `Artifact.ToFrontmatterMap` already emits `custom_fields` verbatim, so
no typed-model change is required and the `archived_from` round-trip regression
class (`docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`)
is avoided by construction. The `commit:` scalar is direct precedent for
storing a remote identifier in frontmatter. Git merge behavior is the ordinary
YAML-block conflict on the same artifact touched by two branches, which the
repository already accepts for `status`, `commit`, and `dependencies`; the
values are last-write-wins and are self-healing because `verify` re-derives
`rev` and `content_hash` from the remote and the local body.

**Rejected.** A `{workspace}/integrations/azure-devops/state.jsonl` ledger
(introduces a second durable store, a second merge semantic, and unbounded
growth for state that is naturally one record per artifact); storing identity
in SQLite (gitignored and rebuilt, so identity would be lost on every clone).

#### Idempotency, crash window, and orphan recovery

**Decision.** Three layered mechanisms.

1. **Local link check.** Absent `work_item_id` means create; present means
   update. This handles the ordinary case.
2. **Remote correlation tag.** Every created work item carries a reserved
   `backlogit-key:{key}` tag in `System.Tags`, where `{key}` is the stable
   `custom_fields.external.key` — 32 lowercase hexadecimal characters,
   `^[0-9a-f]{32}$`. This is the recovery key for the crash window where the
   remote create succeeded but the local frontmatter write did not. The
   artifact ID is written only as a secondary, non-authoritative
   `backlogit-id:{artifactID}` display tag.
3. **Adopt before create.** `plan` resolves every unlinked artifact against the
   remote by correlation tag — one WIQL `CONTAINS` query for the batch,
   followed by a `workitemsbatch` read — before proposing any create. A match
   produces an `adopt` action, not a `create` action. Multiple remote matches
   produce a `conflict` action and halt that artifact.

**Reason.** Azure DevOps offers no client-supplied idempotency key on create,
so a correlation value written inside the created item is the only way to make
the create step recoverable. `System.Tags` is present on every process template
and is WIQL-queryable, which `relations` are not. The `existingImportIndex` in
`internal/cli/migrate.go` is the same pattern already used for local import
resume.

**Caveat carried into the plan.** Tags are user-editable, so `adopt` is
advisory: it appears as a proposed action in the plan and requires the operator
to apply that plan, exactly like every other action.

**Rejected.** A hidden custom field (may not exist in the target process);
embedding the artifact ID in `System.Description` (mutable, user-visible,
breaks on edit); trusting a local-only ledger to survive a crash.

#### Concurrency, drift, and conflict

**Decision.** Every update carries
`{"op":"test","path":"/rev","value":<last_synced_rev>}` as its first patch
operation. A rejected test is classified as `remote_drift`, not as a transport
error: the artifact is marked `conflict`, skipped, and reported. Drift is also
detected proactively during `plan` by batch-reading `System.Rev` for all linked
artifacts and comparing against the stored `rev`. The CLI-only
`--allow-remote-drift` flag authorizes an overwrite, and it is a **`plan`-time**
flag: it freezes `skip_revision_test: true` plus the approving operator's
recorded reason into the plan action, so the omission of the test operation is
visible in the reviewed artifact and changes the `plan_id`. `apply` owns no such
flag and cannot alter the revision test. The lever is never available on the MCP
surface.

**Reason.** `/rev` is the documented concurrency mechanism and there is no ETag
alternative. Detecting drift at plan time means the operator reviews the
overwrite before it happens instead of discovering it in an error log.
Restricting the override to the CLI applies the F6 rule that a fallback surface
must never be more dangerous than the surface it mirrors.

**Rejected.** Last-write-wins by default; `bypassRules=true` by default.

#### Partial failure and resume

**Decision.** The **artifact is the atomic unit**. A failure on artifact *N*
never rolls back artifacts 1..*N*-1. `core.MutationEnvelope` compensation is
used only for the *local* write triple (frontmatter, SQLite row, JSONL event)
that follows a successful remote write, never across the remote boundary. A
remote write whose outcome is unknown — transport error after the request was
sent, 5xx, timeout — is recorded as `pending_verify` and is **never blindly
retried**; `backlogit ado verify` reconciles it by correlation tag on the next
run. `apply` writes per-item progress beside the plan so
`apply --plan <id> --resume` skips completed artifacts.

**Reason.** This is the existing `indeterminate` rule from
`docs/design-docs/governed-mutation-recovery-contract.md` applied to a remote
step: when a durable write may already be present, prefer convergence over
rollback. Compensating a remote create means deleting a work item, which is
destructive, may be blocked by permissions, and may already carry human edits.

**Rejected.** Wrapping remote calls in the compensating envelope; automatic
retry of an indeterminate create.

#### Process discovery and validation

**Decision.** Nothing about Agile, Scrum, CMMI, or Basic is hardcoded.
`backlogit ado discover` reads `workitemtypes`, `workitemtypes/{type}/states`,
`workitemtypes/{type}/fields`, `fields`, and `classificationnodes` for areas
and iterations, and caches the snapshot in gitignored
`{workspace}/runtime/ado-discovery.json`. `plan` **fails closed** when the mapping
references a work item type, state, field reference name, area path, or
iteration path that the snapshot does not contain.

**Reason.** A project's process cannot be changed after creation, inherited
processes add types and states freely, and an invalid area or iteration path is
rejected at PATCH time with an opaque 400. Validating at plan time converts a
mid-apply failure into a pre-apply diagnostic.

**Rejected.** Built-in process tables; discovering lazily during apply.

#### CLI and MCP surface

**Decision.** A provider-named command group over an internally
provider-agnostic orchestrator.

| Operation | CLI | MCP |
|---|---|---|
| Discover process metadata | `backlogit ado discover` | `backlogit_ado_discover` |
| Compute a reviewable plan | `backlogit ado plan` | `backlogit_ado_plan` |
| Apply a computed plan | `backlogit ado apply --plan <id>` | `backlogit_ado_apply` |
| Report link and drift state | `backlogit ado status` | `backlogit_ado_status` |
| Reconcile pending or orphaned links | `backlogit ado verify` | `backlogit_ado_verify` |

`apply` on both surfaces requires an explicit `plan_id` produced by a prior
`plan`; there is no single-shot plan-and-apply. The dangerous levers —
`--bypass-rules`, `--allow-remote-drift`, `--force-relink` — are CLI-only and
carry `human_terminal_only: true` in `.autoharness/backlog-registry.yaml`. Each
lever is owned by exactly one command: `--bypass-rules` by `apply`,
`--allow-remote-drift` by `plan`, and `--force-relink` by `verify`.

**Reason.** Requiring a persisted plan is what makes the MCP surface safe
enough to expose: an agent cannot cause an external write that a
human-reviewable artifact did not already describe. Reusing `backlogit sync`
was rejected because that verb already means "rebuild the SQLite index" and
overloading it would be a silent behavioral trap.

**Rejected.** `backlogit sync --target ado`; MCP read-only with CLI-only apply
(that is asymmetry on the *operation*, which the parity contract discourages,
rather than on the *lever*, which it endorses).

#### Package ownership and dependency direction

**Decision.**

| Package | Owns | Imports |
|---|---|---|
| `internal/ado` | REST transport, auth header, JSON Patch encoding, error decode, retry and backoff | stdlib plus `internal/errors` |
| `internal/config` | `AzureDevOpsConfig` load and validate | existing config dependencies |
| `internal/extsync` | Provider interface, mapping, content hashing, plan computation, state machine — core-independent and filesystem-free: pure computation plus one translation-only provider adapter that delegates all I/O to `ado` | `models`, `config`, `ado`, `errors` |
| `internal/core` | Orchestration and persistence: load artifacts, call provider, write frontmatter, append JSONL, upsert projection, persist plan, snapshot, and ledger | existing plus `extsync` |
| `internal/cli`, `internal/mcp` | Surfaces | `core` |

**Reason.** `internal/release` is the established precedent for a stdlib-only
outbound HTTP leaf consumed by both surfaces. Keeping plan computation
core-independent and filesystem-free in `internal/extsync` makes the entire mapping
and diff engine table-testable with no network and no filesystem: the only
I/O-adjacent type in the package is the provider adapter, which translates
values and delegates every request to `ado`, so tests drive a fake client rather
than a socket. Plan, snapshot, and ledger persistence live in `internal/core`,
which already owns `SafeResolve` and `WorkspaceStorageRoot`. `core -> extsync ->
ado` preserves the documented direction; `extsync` never imports `core`, so no
cycle is possible.

**Rejected.** Putting the client in `internal/core` (couples domain logic to
HTTP); putting orchestration in `internal/cli` (breaks CLI/MCP parity by
construction).

#### Credentials

**Decision.** v1 resolves a PAT **only** from an environment variable named in
the config (`auth.pat_env`, default `BACKLOGIT_ADO_PAT`). The token is never a
config value, never a CLI flag, never written to a plan file, never written to
frontmatter, and is redacted from every error and log line. The required scope
is `vso.work_write`. An `Authenticator` interface is the seam for Entra ID
service principal and managed identity, named as the v2 direction because Azure
DevOps OAuth apps are deprecated.

**Reason.** `azd-backlogloader` ships a `personal_access_token` config field
with `use_env_for_pat: false` as the default — the exact anti-pattern this
repository's constitution forbids. `hooks.WebhookNotifier` already establishes
the correct local pattern: `os.ExpandEnv` over configured values, with no
secret in the config file.

**Rejected.** A `--pat` flag; a config-file token with a warning comment; a
credential file under the configured workspace storage root.

#### Extension seam for Jira and GitHub

**Decision.** One interface, one implementation, no speculative abstraction
layer:

```go
// Provider is the external work-tracking system a sync run targets.
type Provider interface {
    Name() string
    Discover(ctx context.Context) (Schema, error)
    Resolve(ctx context.Context, keys []CorrelationKey) (ResolveResult, error)
    Read(ctx context.Context, refs []RemoteRef) (ReadResult, error)
    Create(ctx context.Context, item MappedItem, opts WriteOptions) (RemoteRef, error)
    Update(ctx context.Context, ref RemoteRef, item MappedItem, opts WriteOptions) (RemoteRef, error)
}
```

`ResolveResult{Matches map[CorrelationKey][]RemoteRef; Truncated []CorrelationKey}`
expresses zero-, one-, and many-match multiplicity plus a fail-closed
truncation signal; `ReadResult{Found map[string]RemoteItem; Missing []RemoteRef}`
names the refs the remote did not return, which is the `orphan_local` signal;
`WriteOptions{ValidateOnly, BypassRules, SuppressNotifications, SkipRevisionTest bool}`
is an explicit per-call parameter rather than client state. There is no
`Relate` method: the parent relation is carried on `MappedItem` and emitted by
the patch builder, and v1 never removes a relation, so a `Relate` method would
be dead API.

Plan computation, the state machine, hashing, and the CLI/MCP result shapes are
written against `Provider`. Azure DevOps is the only implementation. No
registry, no plugin loader, no factory, no second config namespace.

**Reason.** Principle VI forbids speculative dependencies and abstraction. One
interface with one implementation costs nothing and is the difference between a
later Jira adapter being a new file and being a refactor.

**Rejected.** A provider registry with dynamic dispatch; a generic
`ExternalItem` schema union; any Jira or GitHub implementation task.

#### Observability, throttling, and rollback

**Decision.** A shared token-bucket limiter in `internal/ado` following the
`hooks.WebhookNotifier` pattern (`golang.org/x/time/rate` is already a direct
dependency); mandatory `Retry-After` honoring; capped exponential backoff with
jitter on 429 and 5xx; a bounded attempt count; one consolidated PATCH per
artifact per cycle to respect the 10,000-revision cap;
`suppressNotifications=true` by default for bulk apply; structured JSONL audit
events per artifact (`external_sync_planned`, `external_sync_applied`,
`external_sync_conflict`, `external_sync_pending_verify`); telemetry counters
through the existing `events.TelemetryWriter`; and a rollback that is
**operator-driven** — `verify` reports, `--force-relink` re-baselines, and
remote deletion is never automated.

**Rejected.** Automatic remote deletion when a local artifact is archived (v1
reports an `orphan_remote` finding instead, and only from `backlogit ado status`
and `backlogit ado verify`, which scan durable link records independently of
push eligibility; `plan` and `apply` exclude archived, abandoned, and shipped
artifacts and emit no deletion action); unbounded retry.

### What Was Tried and Failed

* **Reusing `migration.yaml` for the target map.** Abandoned after reading
  `config.MigrationConfig`: `document_classes` is `required,min=1` and every
  field describes local file classification. Any target block would have been a
  parasitic second schema inside a validator that cannot express it.
* **Reusing `hooks.WebhookNotifier` as the sync transport.** Abandoned after
  reading `dispatchToEndpoint`: it returns before its goroutine completes, logs
  failures at warn level, and returns `nil` unconditionally. It cannot report a
  result, so it cannot maintain state.
* **Wrapping the remote call in `core.MutationEnvelope`.** Abandoned because
  compensation across the remote boundary means deleting a work item that may
  already carry human edits, and because the envelope's own contract states
  that a possibly-applied durable write must not be compensated.
* **A JSONL sync-state ledger under `{workspace}/integrations/`.** Abandoned in
  favor of frontmatter plus the existing per-item log: it would have introduced
  a third durable store, a new Git merge semantic, and unbounded growth for
  state that is naturally one record per artifact.
* **engram `query-memory` for prior decisions and compound learnings.**
  Returned zero results; the engram index for this workspace covers code
  symbols only (`stats` reports 4,046 functions across 623 code files, with no
  document region). Prior-art discovery for `docs/` fell back to direct reads
  of the enumerated `docs/compound`, `docs/decisions`, and `docs/design-docs`
  listings, which contain no prior Azure DevOps or external-provider work.

### Remaining Unknowns

These are carried into the plan as pre-implementation verification tasks, not
as blockers. Each has a safe default.

* **HTTP status and `typeKey` for a failed `/rev` test operation.** Not stated
  on the 7.1 Update reference page. Safe default: treat a 4xx whose body
  indicates a revision or concurrency failure as `remote_drift`, and treat an
  unclassifiable 4xx as a hard per-artifact failure rather than a silent
  overwrite. Confirm against a live project during the client task.
* **`_apis/work/processes` and `_apis/projects/{id}/properties` exact
  templates.** Could not be loaded from a 7.1 reference page. Safe default: v1
  discovery uses the project-scoped `wit/workitemtypes` family, which was
  verified and is sufficient; organization-level process introspection is not
  required for v1.
* **Documented cap on `_apis/wit/$batch` operations per request.** Not stated
  on the archived reference page. Safe default: v1 does not use `$batch` for
  writes — one PATCH per artifact with client-side rate limiting. The batch
  endpoint is recorded as a future throughput optimization.
* **Whether a per-field Markdown format control exists in REST 7.1.** No such
  parameter appears on the pages consulted. Safe default: render Markdown to
  HTML and send HTML, with raw inline HTML in artifact bodies escaped by the
  renderer. HTML is the v1 contract, not a default behind a switch: there is no
  `text_format` knob and Markdown passthrough is not a v1 option.
* **Presence of `X-RateLimit-Reset` and `X-RateLimit-Resource`.** Not confirmed
  on the rate-limits page. Safe default: rely on `Retry-After` and capped
  exponential backoff; treat the other headers as optional telemetry.
* **`vso.work_full` as a distinct scope.** Only `vso.work` and `vso.work_write`
  were confirmed. Safe default: document `vso.work_write` as the required
  scope.

## Recommendation

**Conclusion**: proceed
**Confidence**: medium

Proceed with a one-way, plan-then-apply, artifact-level Azure DevOps push
capability configured by a dedicated target-mapping schema, with identity in
frontmatter, correlation-tag adoption, `/rev` optimistic concurrency, and an
artifact-atomic apply loop.

Confidence is medium rather than high for two reasons, both about the external
system and neither changing the architecture. First, six documented behaviors
listed above could not be confirmed from an official page and must be validated
against a live organization during implementation; each has a conservative
fail-closed default, so the exposure is rework inside one package rather than a
redesign. Second, no integration test against a real Azure DevOps project is
possible in CI without an organization and a credential, so v1 correctness
rests on a recorded-fixture HTTP test harness plus documented manual runtime
verification against a scratch project. That is stated in the plan's
verification section rather than assumed away.

Architectural confidence is high. Every load-bearing decision reuses an
existing, tested backlogit contract: `custom_fields` serialization, per-item
JSONL audit, the disposable SQLite projection, the `indeterminate`-dominates
rule, the governed CLI/MCP parity registry, and the `internal/release`
stdlib-leaf client shape. The feature promotes one already-present indirect
dependency (`blackfriday/v2`) and adds no new external dependency.

## Next Steps

1. Implementation plan at
   `docs/exec-plans/2026-08-20-azure-devops-sync-plan.md`, written as part of
   this spike's promotion path.
2. Confirm the six external unknowns against a scratch Azure DevOps project
   during the client implementation phase; record the results in the plan's
   verification section.
3. Harvest the plan into backlog work items only on explicit operator
   instruction — this spike deliberately performed no backlog mutation.

## Decisions Superseded by Plan Review

The implementation plan promoted from this spike was put through the
seven-persona `plan-review` gate, which corrected eight of the decisions
recorded above. The plan is authoritative where the two disagree; this section
keeps the spike honest rather than silently stale.

| Spike decision | Superseded by | Reason |
|---|---|---|
| Correlation tag is `backlogit-id:{artifactID}` | A stable, immutable `custom_fields.external.key` minted once as 128 bits of `crypto/rand` entropy encoded with `encoding/hex` — 32 lowercase hexadecimal characters, `^[0-9a-f]{32}$`; the artifact ID becomes a non-authoritative display tag | `AdoptItem` rewrites artifact IDs, so an ID-derived key orphans or duplicates a work item after a re-parent |
| One project-wide WIQL correlation query | Correlation scoped to the selection, chunked, and fail-closed on truncation | A truncated project-wide result can silently produce a duplicate create |
| Pure package named `internal/sync`, owning plan and snapshot persistence | `internal/extsync`, core-independent and filesystem-free: pure value types and computation plus a translation-only provider adapter that delegates I/O to `internal/ado`; persistence moved to `internal/core` | Persistence needs `SafeResolve` and `WorkspaceStorageRoot`, which live in `core`, so the original split was an import cycle. The name also shadowed the standard library |
| CLI-only levers restricted by registry marker | A real approval gate: TTY confirmation, or `--i-understand` plus a recorded reason on a non-TTY | A registry marker documents intent; it does not gate execution, and Constitution VII requires approval |
| MCP `apply` safe because it requires a `plan_id` | Additionally gated by `agent_apply.enabled`, default false | The same agent can call `plan` and then `apply`, reviewing nothing |
| HTML rendering safe because repo Markdown is trusted | The renderer escapes raw inline HTML | `backlogit migrate` ingests externally authored bodies, so repo content is not a guaranteed trust boundary |
| `Provider` with flat `Resolve`/`Read` returns, no per-call write options, and a `Relate` method | `Resolve` returns `ResolveResult`, `Read` returns `ReadResult`, `Create` and `Update` take `WriteOptions`, and `Relate` is removed | A flat `[]RemoteItem` could not express which refs were missing (`orphan_local`) and a flat `Resolve` return could not express match multiplicity, which drives three different actions; the parent relation ships on `MappedItem` in the patch, so `Relate` would be dead API |
| `defaults.text_format` as a knob that can opt into raw Markdown | HTML only, with raw inline HTML escaped; no `text_format` knob | Unrequested scope: no requirement asked for it, it doubles renderer branches, and toggling it churns every work item revision |

The spike's core conclusion is unchanged: proceed with a one-way,
plan-then-apply, artifact-level push keyed on durable frontmatter identity.

### Corrections from a subsequent focused pass

A later focused architecture pass over the remediated plan and this spike — not
part of the seven-persona review above, and run after it — found five internal
inconsistencies. One of the five, the classification of an exhausted retry
budget, did not contradict this spike: the partial-failure decision above
already says an outcome-unknown write is recorded `pending_verify` and never
blindly retried. The plan's final rule sharpens rather than reverses that:
classification follows the **terminal cause** with indeterminacy sticky — an
exhausted 429 sequence stays `throttled` and retryable, any timeout, 5xx, or
transport failure after a possible send stays `indeterminate` and becomes
`pending_verify`, and `validation` and `permanent` never enter the retry loop.
A second, the external key format, is now spelled out in the superseded table
above (32 lowercase hexadecimal characters from `crypto/rand`). The three below
correct spike text directly, and the plan remains authoritative where the two
documents disagree.

| Spike statement | Corrected to | Reason |
|---|---|---|
| `internal/sync` owns everything "all pure" | Core-independent and filesystem-free: pure computation plus a translation-only provider adapter that delegates every request to `internal/ado` | The adapter that satisfies `Provider` for Azure DevOps lives in this package and participates in a network round trip, so a blanket purity claim was inaccurate. The must-never-import-`core` invariant and the direction are unchanged, now read as `core -> extsync -> ado` after the rename recorded above |
| `--allow-remote-drift` re-baselines by dropping the test operation | A `plan`-time, approval-gated flag that freezes `skip_revision_test: true` plus the recorded reason into the action, changing the `plan_id`; `apply` has no such flag | An apply-time lever would let a reviewed plan be executed with semantics the reviewer never saw |
| v1 "reports an `orphan_remote` finding" for archived locals | `orphan_remote` is report-only and produced solely by `backlogit ado status` and `backlogit ado verify`, which scan durable link records independently of push eligibility | Archived, abandoned, and shipped artifacts are outside the eligible push selection, so `plan` can never reach them and could never emit the warning |

## References

### backlogit code and configuration

* `internal/models/artifact.go` — `Artifact`, `ToFrontmatterMap`,
  `serializeDependencies`, `DependencyEdge`, `ArtifactLink`
* `internal/config/schema.go` — `WorkspaceConfig`, `ArtifactTypeConfig`,
  `FieldConfig.ExternalMap`, `NotificationsConfig`
* `internal/config/migration.go` — `MigrationConfig`, `DocumentClassConfig`,
  `SourcePathConfig`, `LoadMigrationConfig`, `WriteMigrationDefaults`
* `internal/core/fields.go:42-51` — `TranslateExternalMap` (no callers)
* `internal/core/mutation_envelope.go` — `MutationEnvelope`, `MutationStep`
* `internal/core/shipment.go` — `CreateShipment`, `custom_fields.items`
* `internal/cli/migrate.go` — `existingImportIndex`, `importedArtifactRef`,
  `importMigrationItems`
* `internal/parser/adapter.go` — `MigrationAdapter`, `MigrationItem`
* `internal/hooks/webhook.go` — `WebhookNotifier`, `dispatchToEndpoint`
* `internal/mcp/server.go` — `newServer`, `addTool`, `CLICommandProvider`
* `internal/db/schema.go` — `EnsureSchema`, `IntrospectSchema`
* `.backlogit/config.yaml`, `.backlogit/migration.yaml`,
  `.autoharness/backlog-registry.yaml`, `go.mod`, `.gitignore`

### backlogit documentation

* `docs/ARCHITECTURE.md` — domain map, dependency direction, SQLite cache
  boundary
* `docs/design-docs/governed-mutation-recovery-contract.md` — classification,
  compensation, doctor recovery, MCP error payload
* `docs/design-docs/governed-operation-parity.md` — governed markers,
  behavioral parity test, `human_terminal_only` asymmetry rule
* `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`
* `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`
* `.github/instructions/constitution.instructions.md`

### Reference repositories (read-only)

* `azd-backlogbldr` — `backlog-yml/product-backlog-tasklevel.yaml`, `agent.md`,
  `README.md`
* `azd-backlogloader` — `ado.workitem-loader.py`, `config.parameters.yaml`,
  `config.templates.yaml`,
  `samples/sample-backlog-with-custom-fields.yaml`

### Azure DevOps REST documentation

* [Work items — Create](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/create?view=azure-devops-rest-7.1)
* [Work items — Update](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/update?view=azure-devops-rest-7.1)
* [Work items — List](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/list?view=azure-devops-rest-7.1)
* [Work items — Get work items batch](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/get-work-items-batch?view=azure-devops-rest-7.1)
* [Work items — Delete](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/delete?view=azure-devops-rest-7.1)
* [Recycle bin](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/recyclebin?view=azure-devops-rest-7.1)
* [Work item relation types — List](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-item-relation-types/list?view=azure-devops-rest-7.1)
* [Link type reference](https://learn.microsoft.com/en-us/azure/devops/boards/queries/link-type-reference?view=azure-devops)
* [Work item types — List](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-item-types/list?view=azure-devops-rest-7.1)
* [Work item type states — List](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-item-type-states/list?view=azure-devops-rest-7.1)
* [Fields — List](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/fields/list?view=azure-devops-rest-7.1)
* [Classification nodes — Get](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/classification-nodes/get-classification-nodes?view=azure-devops-rest-7.1)
* [WIQL — Query by WIQL](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/wiql/query-by-wiql?view=azure-devops-rest-7.1)
* [WIQL syntax](https://learn.microsoft.com/en-us/azure/devops/boards/queries/wiql-syntax?view=azure-devops)
* [Reporting work item revisions](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/reporting-work-item-revisions/read-reporting-revisions-get?view=azure-devops-rest-7.1)
* [Integration best practices](https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/integration-bestpractices?view=azure-devops)
* [Rate and usage limits](https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rate-limits?view=azure-devops)
* [Authentication guidance](https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/authentication/authentication-guidance?view=azure-devops)
* [Use personal access tokens](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)
* [Azure DevOps OAuth (deprecated)](https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/authentication/oauth?view=azure-devops)
* [Microsoft Entra ID authentication](https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/authentication/entra?view=azure-devops)
* [Service principals and managed identities](https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/authentication/service-principal-managed-identity?view=azure-devops)
* [Agile process workflow](https://learn.microsoft.com/en-us/azure/devops/boards/work-items/guidance/agile-process-workflow?view=azure-devops)
* [Scrum process workflow](https://learn.microsoft.com/en-us/azure/devops/boards/work-items/guidance/scrum-process-workflow?view=azure-devops)
* [CMMI process workflow](https://learn.microsoft.com/en-us/azure/devops/boards/work-items/guidance/cmmi-process-workflow?view=azure-devops)
