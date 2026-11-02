---
schema_version: "1.0"
doc_type: plan
title: "Decided plan: Azure DevOps work item synchronization (v1, push-only)"
description: "Decided-plan consolidation of 2026-08-20-azure-devops-sync-plan.md: final decisions, implementation units, risks, and review verdict."
chunk_strategy: h1-h2-h3
source: docs/exec-plans/2026-10-18-azure-devops-sync-decided-plan.md
---

# Decided plan: Azure DevOps work item synchronization (v1, push-only)

Consolidated from `docs/archive/plans/2026-08-20-azure-devops-sync-plan.md` (P-020 compaction, 2026-10-18). The owning release units are archived. Full deliberation history stays in the archived original.

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
   ...

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
| Drift detection at plan time | Batch read `System.Rev`, compare to stored rev | ...

## Implementation units

* **Implementation Units**: Granularity rule, stated precisely because the reviewed draft claimed a bound it broke: every unit touches **at most 2 production files** and **at most 3 test scenarios**, targets a single domain, and produces a verifiable milestone. Posture is test-first ...

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

## Risks and Caveats

| Risk | Impact | Mitigation |
|---|---|---|
| Six Azure DevOps behaviors unverified against a live tenant | The client misclassifies an error, or a drift is treated as a hard failure | Conservative fail-closed defaults; U50 verifies each and U51 encodes the result |
| `[System.Tags] CONTAINS` semantics may be substring rather than exact-tag | The correlation index over- or under-matches | WIQL narrows the candidate set; exact tag matching is always done client-side after the batch read |
| WIQL chunk truncation on a very large project | Correlation coverage incomplete | Fail closed: `correlation_truncated` blocks the plan rather than proposing a create |
| Correlation tag edited or removed by a human | Adoption misses | `adopt` is never automatic; it appears in the plan for operator review, and `--force-relink` is the gated manual repair |
| Frontmatter link record conflicts on a Git merge | Two branches disagree on `rev` | `external.key` and `work_item_id` are immutable after first link, so identity survives; `verify` re-derives `rev` and `content_hash` semantically |
| Link-record writes add Git churn to artifact files | Noisier diffs and more merge contention on synced artifacts | `noop` never rewrites; U49 documents committing link-record writes as a separate synchronization commit |
| Two workspaces cloned from the same repo apply concurrently | The lease does not serialize them | Documented limitation; correlation preflight plus fail-closed truncation reduces but does ...

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: ADVISORY

TOOL_OK: reviewer-subagent-dispatch

## Traceability

| Original path | Archive path | Disposition |
|---|---|---|
| `docs/exec-plans/2026-08-20-azure-devops-sync-plan.md` | `docs/archive/plans/2026-08-20-azure-devops-sync-plan.md` | archived (decided-plan) |
