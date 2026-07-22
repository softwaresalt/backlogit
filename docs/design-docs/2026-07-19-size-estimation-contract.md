---
chunk_strategy: h1-h2-h3
description: 'Durable sizing contract for backlogit (shipped 099-S / feature 108-F) — the canonical custom_fields.size field stored only on tasks (features/shipments/parents expose a computed-on-read rollup composition instead); size and provenance enums and their defaulting and rejection rules; the event-before-write best-effort audit durability policy; the computed-on-read composition contract; CLI/MCP parity and transport-aware actor stamping; and the map-replacement and validated-once caveats'
doc_type: design
docline:
    date: 2026-07-19T00:00:00Z
    status: accepted
    tags:
        - size-estimation
        - provenance
        - custom-fields
        - composition
        - cli-mcp-parity
        - trust-boundary
        - audit-event
ingested_at: "2026-07-19T00:00:00Z"
schema_version: "1.0"
source: docs/design-docs/2026-07-19-size-estimation-contract.md
title: 'Size estimation contract (task-only sizing with rollup composition)'
---

## Scope

This document is the durable contract for size estimation on backlogit work
items. It covers the canonical storage field, the size and provenance value
sets, the audit-event durability policy, the computed-on-read composition
rollup, and the CLI and MCP parity surfaces. It reflects the behavior shipped in
feature 108-F (shipment 099-S).

For the originating analysis and alternatives, see the spike plan in
[2026-07-14-size-estimation-feature-shipment-plan.md](../exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md)
and the implementation plan in
[2026-07-18-108-F-size-estimation-impl-plan.md](../exec-plans/2026-07-18-108-F-size-estimation-impl-plan.md).

## Canonical size field

Size is stored as `custom_fields.size` and is **task-only**: the stored,
estimatable size lives exclusively on `task` artifacts. Features, shipments, and
any other parent-of-tasks never carry a stored size — their size is delivered as
a computed-on-read rollup composition (see below). There is no dedicated
`models.Artifact` carrier and no migration — the value rides in the existing
`custom_fields` map, which already round-trips through the frontmatter codec.

Task is the single decomposition unit for estimable, composable work. Keeping
the stored estimate on tasks alone keeps estimation measurable and consistent
for agent hand-off, while feature and shipment rollups reveal how well work is
composed.

The size value set is a fixed ordered enum:

| Rank | Value |
|---|---|
| 1 | XS |
| 2 | S |
| 3 | M |
| 4 | L |
| 5 | XL |

A size mutation validates the value against this enum before any write. An
out-of-range value is rejected with a validation error and the file is left
unchanged.

## Provenance fields

Two provenance fields accompany a size:

* `size_source` — one of `human`, `agent`, or `derived`.
* `size_ruleset_version` — an opaque version string identifying the ruleset that
  produced a sized estimate.

Rules:

* An absent `size_source` means unknown or legacy. It is never rewritten as
  `human`; a plain size set does not stamp a source.
* An explicit `size_source` outside `{human, agent, derived}` is rejected.
* Provenance completeness (Rule R): an explicit `size_source` requires an
  accompanying `size_ruleset_version`. A source without a ruleset is rejected and
  the file is left unchanged. This presence rule is what the crash-audit harness
  pins; the ruleset value itself is not enum-validated.
* Creating an artifact through the generic create path that carries any reserved
  sizing key (`size`, `size_source`, or `size_ruleset_version`) is refused,
  provenanced or not, so an initial size is never recorded off-seam, unvalidated,
  and eventless. All initial sizing must route through the audited size seam. This
  is fail-closed: no production path is affected because the backlog.md migration
  adapter prefixes every imported frontmatter key with `backlog_md_`, so a migrated
  `size` never arrives as the reserved key.

## Audit event durability

Size mutations follow an event-before-write audit policy. The audit-event append
is required (fail-closed); only the downstream SQLite index refresh is
best-effort:

* Every size mutation appends an `estimate_history` event to the item log before
  the durable frontmatter write. The append is fail-closed: if it cannot be
  written, the mutation is refused.
* The durable `custom_fields.size` (with its provenance) is the single source of
  truth. The event stream is an advisory audit trail, not the authority.
* Orphan crash-residue events (an event with no matching durable write) are
  ignored on read. Such an orphan can arise not only from a process crash between
  append and write, but also from an ordinary post-append error — for example
  `mdDoc.Encode` or `WriteFileAtomic` returning a disk-full or rename failure. An
  orphan event therefore does not by itself prove a crash occurred.
* The policy is process-crash-safe only. OS-level crash reconciliation,
  exactly-once semantics, operation IDs, and doctor reconciliation are out of
  scope.

## Computed-on-read composition

Feature and shipment size composition is computed on read and never persisted.
`SizeComposition` returns an XS–XL count histogram, an `unsized` count, a
de-duplicated canonical-members array, and a `skipped` list. `ruleset_version` in
the result is always null until a canonical ruleset is owned.

Membership resolution:

* Feature membership is its direct **task** children by `parent_id`. Non-task
  children (for example reviews or subtasks) are excluded from the rollup.
* Shipment membership is the explicit `custom_fields.items` manifest, resolved to
  **tasks only**. A manifest member that is a feature is expanded into its child
  tasks; the feature itself is never counted as a member.
* Members are de-duplicated, so a manifest listing a feature and its explicit
  child tasks counts each task once.
* An existing member with no size increments `unsized`.
* An unresolved manifest id (or any other resolution error) is warn-skipped: it
  is recorded in `skipped` and counted in neither the histogram nor `unsized`.

The `Histogram` is an unordered count map keyed by size value. The size enum has
a documented rank (XS < S < M < L < XL, see the table above), but no runtime
comparator is implemented — a consumer that needs ordered presentation sorts by
that documented rank itself.

### Read-time freshness (best-effort)

Composition is derived at read time from the SQLite index, which is a derived
cache of the canonical Markdown files. A composition result is therefore a
best-effort multi-read view: it is assembled from several index reads (chunked at
~900 IDs per query, without a cross-chunk transaction), so under concurrent writes
different parts of a single rollup can reflect different index snapshots rather
than one point-in-time snapshot. If member sizes or membership changed after
the index was last synced (for example an out-of-band edit not yet rehydrated),
the rollup can reflect slightly stale sizes or membership. Reads never fail on
this account and never persist a correction — the canonical files remain the
source of truth, and a subsequent sync reconciles the rollup. A consumer that
needs strict freshness syncs the index before reading.

Flat read surfaces (`list` / `list_items` and `queue view` / `get_queue`) compute
the rollup for every aggregate via chunked batch lookups (`SizeCompositions`, each
resolver chunking at ~900 IDs per query) rather than per row, eliminating the
per-aggregate query fan-out. The query count grows with the number of members
rather than staying constant, but no longer scales with the number of aggregates.
The batched result is identical to the per-artifact `SizeComposition`. When the
batched rollup fails, these surfaces degrade to unprojected rows (a warning is
logged) rather than aborting the listing, and both the CLI and MCP transports
share the one core shaper so they degrade identically.

## CLI and MCP parity

Both transports construct the same typed `SizeMutation` and route through the
single core seam.

| Concern | CLI | MCP |
|---|---|---|
| Size input | `--size` | `size` |
| Source input | `--size-source` | `size_source` |
| Ruleset input | `--size-ruleset-version` | `size_ruleset_version` |
| Default actor | `human` | `agent` |
| Mutual exclusivity | `--size` cannot combine with other field-mutating flags | `size*` cannot combine with other field updates or sections |

Transport-aware actor stamping is a trust boundary: the MCP (agent) transport
must not claim human provenance. An explicit `size_source: human` submitted over
MCP is rejected with a validation error, regardless of ruleset. The CLI surface
stamps `human` because it represents a trusted human-authored mutation.

## Caveats

* Whole-map replacement: a generic `custom_fields` update replaces the entire
  map. Reserved sizing keys (`size`, `size_source`, `size_ruleset_version`) are
  preserved across such an update through an explicit merge so a generic field
  edit does not silently drop sizing provenance.
* Validated-once: the size seam validates the value, source, and provenance
  completeness at mutation time. Downstream readers trust the persisted value and
  do not re-validate.
* Best-effort freshness: composition is a best-effort multi-read view derived from
  the index cache — assembled from several reads without a cross-chunk transaction,
  so it may reflect slightly stale or mixed-snapshot sizes/membership until the next
  sync; it never fails the read and never persists a correction (see
  Read-time freshness above).
* Two-layer path containment guards artifact lookup: config-load rejects `..` and
  absolute paths in the root and search directories, and a realpath
  re-containment check runs at lookup so a symlink cannot escape the storage root.
