# Compacted memory: shipped units 060-S through 067-S

This compacted memory replaces 18 verbose checkpoints for already completed or
closed release-unit work. The originals are archived under `docs/archive/memory/`
and listed in the Provenance section.

## 064-S: duplicate branch-level telemetry shipment

### Outcome

`064-S` was stopped as a duplicate of already shipped branch-level telemetry
work from `058-F`. The implementation was already on `main` through PR #111
(`3373ec30`) plus follow-up PR #112, so Ship correctly halted instead of
creating a no-op implementation PR. Stage then confirmed the duplicate
disposition and planned withdrawal/archive of `064-F`, its six tasks, and
`064-S` while preserving traceability to the shipped 058 lineage.

Durable closure pointer: `docs/closure/2026-05-22-058-s-dependency-queue-integrity-closure.md`
for the shipped predecessor; `064-S` itself has no separate closure artifact in
the scoped memories because its final disposition was duplicate reconciliation,
not new delivery.

### Key decisions

* Treat duplicate-shipment disposition as Stage-owned triage, not Ship
  implementation work
* Do not ship or close `064-S` as though it delivered new code
* Preserve traceability through duplicate-of links and comments rather than
  erasing the duplicate lineage

### Key learnings

* A queued shipment can be semantically duplicate even when its IDs differ from
  the shipped predecessor
* Current-code evidence should win over stale backlog intent before Ship claims
  a queued shipment

### Failed or rejected approaches

* Re-running Ship would have produced an empty diff and false delivery history
* Treating shipment/feature ID offsets as corruption was rejected; the
  `B8FF7590` investigation proved offsets such as `060-S -> 061-F` and
  `061-S -> 062-F` are benign when shipment title, feature, and plan agree

## 065-S: docline frontmatter standardization

### Outcome

`065-S` shipped in two implementation runs plus post-merge closure. Run 1
delivered the non-gated tooling stack on `feat/065-docline-frontmatter`; PR #136
merged as `2a5df85b`. Run 2 delivered operator policy sign-off, bulk migration,
born-compliant generated CLI reference docs, and CI enforcement on
`feat/065-docline-migration`; PR #137 merged as `23a8b045`. Post-merge closure
shipped and archived `065-S`, `065-F`, and all 11 tasks, recording backlog commit
`191c3b1c`.

Durable closure pointer: `docs/closure/2026-06-26-065-S-docline-frontmatter-closure.md`.

### Key decisions

* Scope docline lint/migration to `docs/**` plus root docs while excluding
  `docs/memory/**`, `docs/archive/**`, `.github/**`, and prompt artifacts
* Split authoring and ingestion profiles: authors own title, doc type, source,
  and description; ingestion owns content hash, source path, and authoritative
  ingestion metadata
* Close the doc type vocabulary and fold legacy category fields under a
  `docline` namespace
* Operator sign-off chose seed-once `ingested_at` and repo-relative POSIX
  `source`, while preserving full-URI source values when present
* Choose generated-doc Option A: `cmd/gen-docs` emits docline-compliant
  frontmatter so generated CLI reference pages are born compliant
* Enforce the contract with native `backlogit docs lint`, `make docs-lint`, and
  a CI docs-lint job

### Key learnings

* Backlog dependency operations persist to both the SQLite cache and task-file
  `dependencies:` frontmatter
* `backlogit add --section` only honors `description`; acceptance criteria need
  a follow-up `backlogit update --section`
* Body-preserving migration must be idempotent, batched, and checked for zero
  body-byte changes
* Whole-tree apply paths need a shared `ValidateApplyPath` guard across CLI and
  MCP
* Unknown profile validation must be consistent across `Validate` and
  `ValidateFields`
* Windows `core.autocrlf=true` can create local gofmt noise; LF-normalized blob
  checks are the authoritative local format signal

### Failed or rejected approaches

* A decorative `--dry-run` flag was removed because it implied a safety behavior
  that the implementation did not need
* Deferring generated CLI reference compliance as a scope exclusion was rejected
  in run 2; born-compliant generation was the sustainable fix
* Several Copilot findings appeared across many cycles, but each accepted fix was
  a distinct defensive gap rather than retry-loop thrash

## 066-S: root-ID conflict integrity

### Outcome

`066-S` implementation PR #132 merged as
`80ce5f12ef52a68feaecfb9bfdeb94f6f1f79fd3`. Post-merge closure shipped and
archived `066-S`, `066-F`, and tasks `066.001-T` through `066.005-T`; backlog
archive commit was `35dae96f`. The feature had stale `active` status, but
`ComputeParentStatus` resolved it to `done`, so closure moved it through the
normal lifecycle rather than forcing state.

Durable closure pointer: `docs/closure/2026-06-25-066-s-root-id-conflict-integrity-closure.md`.

### Key decisions

* Allocate new IDs from a canonical filesystem scan that includes the archive,
  not from a PK-collapsed or archive-blind index maximum
* Refuse archive destination overwrites before writing
* Treat root-ID collision and archive destination occupation as explicit guard
  errors

### Key learnings

* Per-type `MAX(ordinal)+1` over a cache can mask duplicate root IDs when the
  source of truth contains archived or conflicting files
* P-007 archive integrity verification must inspect working-tree deletions in
  `.backlogit/archive/` after lifecycle moves
* Runtime verification for CLI/library integrity work can use doctor checks plus
  targeted core and database tests when no service surface exists

### Failed or rejected approaches

* Relying on the SQLite index as the allocation authority was rejected because
  the markdown filesystem is the source of truth
* CRLF-only gofmt output was identified as local noise and not treated as a code
  regression

## 060-S: shipment state integrity

### Outcome

`060-S` carried feature `061-F` and tasks `061.001-T` and `061.002-T`; the
`060-S -> 061-F` ID offset was confirmed benign. Implementation PR #143 merged as
`7a51904bc159d0f16aa5a9d8866e0bd4c324717d`. Post-merge closure shipped
`060-S`, archived `061-F` and both tasks, and committed backlog state as
`8411a0b8`. Adjacent shipment `061-S` was explicitly verified queued and
untouched.

Durable closure pointer: `docs/closure/2026-06-27-060-S-shipment-state-integrity-closure.md`.

### Key decisions

* Implement `ClaimShipment` as an atomic multi-item transition with a pre-claim
  shipment snapshot, activated-ID tracking, and rollback on mid-flight failure
* Clear stale `blocked_reason` at every backlog re-entry choke point:
  `UpdateArtifact`, `setArtifactStatus`, and `cascadePersistedParentStatuses`
* Remove the fallible post-activation `GetShipment` read-back because item
  activation cannot mutate the shipment artifact

### Key learnings

* Lifecycle `blocked -> queued` bypasses the `validate_status_transition` hook,
  while operator move uses `blocked -> active`; both paths need stale metadata
  cleanup
* Parent cascade can recompute a blocked parent out of blocked state and must
  also clear stale `blocked_reason`
* Rollback must record activated IDs before status persistence so a partial
  cascade failure remains reversible

### Failed or rejected approaches

* The 2026-05-22 plan was stale and named the wrong integration surface, so the
  implementation was grounded in current `ClaimShipment` and lifecycle code
* A post-mutation read-back would have left a torn-state window; it was removed
  by construction

## 061-S: metadata and section sync integrity

### Outcome

`061-S` carried feature `062-F` and five bugfix tasks; the `061-S -> 062-F` ID
offset was confirmed benign and left intact. Implementation PR #145 merged as
`006bb854afa6f56c87f4a80f5d3d6668feef0b58`. Post-merge closure shipped
`061-S`, archived `062-F` and all five tasks, authored closure and compound
artifacts, and opened closure PR #146 on
`post-merge/062-metadata-section-sync-integrity`.

Durable closure pointer: `docs/closure/2026-06-27-061-S-metadata-section-sync-integrity-closure.md`.

### Key decisions

* Restore MCP/CLI metadata parity through dependency injection:
  `Server.CLICommandProvider`, not an `mcp -> cli` import
* Resolve exported command maps under the workspace root, not `.backlogit`
* Re-upsert SQLite and FTS after MCP section rewrites
* Replace the CLI blanket-append fallback with typed parser sentinels and shared
  section-name validation
* Enforce MergeSync dry-run purity before any fallback rehydrate path can write

### Key learnings

* Cross-layer parity needs both a dependency-injection seam and a parity test
* Every mutating content write must keep markdown, SQLite, and FTS indexes in
  sync
* Dry-run guards must wrap fallback branches as well as primary write branches
* Parser-owned sentinels prevent corruption better than string matching or
  broad fallback appends

### Failed or rejected approaches

* The stale plan mislocated the `062.003` defect on CLI instead of MCP and
  understated the CLI-specific `062.004` corruption path
* Manifest ID realignment was rejected because the title/feature/plan agreement
  showed the offset was benign
* Copilot re-request automation for closure PR #146 exhausted the available CLI,
  REST, and GraphQL paths; the session halted with a stale review and operator
  handoff

## 067-S: archived_from integrity

### Outcome

`067-S` fixed `ArchiveItem` and unarchive integrity for `archived_from`.
Implementation PR #141 merged as
`41f6ff7d309ccb7c388accd85d2c438205370a77`. The implementation repaired 130
self-referential archive records, left two malformed `archived_from: done`
records flag-only, and shipped the migration inside the release unit.
Post-merge closure shipped `067-S`, archived `067-F` and seven tasks, and
committed backlog state as `907bc9c0`.

Durable closure pointer: `docs/closure/2026-06-27-067-S-archived-from-integrity-closure.md`.

### Key decisions

* Canonical `archived_from` values must be repo-root-relative and
  `.backlogit/`-prefixed
* The restore-path resolver is pure over the workspace queue layout, not the WIT
  registry
* `--fix-archived-from` is CLI-only and not exposed through MCP doctor tools
* `UnarchiveItem` self-heals legacy self-references at read time and refuses to
  clobber an existing queue file
* Malformed `archived_from: done` records are reported only; permanent
  disposition was stashed separately

### Key learnings

* Archive/unarchive integrity depends on stamping the original queue path at
  archive time, not attempting to infer it after the fact
* Doctor repair must be idempotent, symlink-safe, and body-preserving
* The first post-merge ship after the fix dogfooded the exact pre-archived case:
  seven fieldless task archives were stamped canonically, with zero
  self-referential records
* `docline` helpers could not be reused inside core repair code because that
  would create an import cycle; extraction to a leaf package was stashed as
  follow-up `8863C6C8`

### Failed or rejected approaches

* Prefix-less `queue/...` `archived_from` values were rejected because F-006 would
  reject them and strand records
* A one-shot migration without doctor audit was rejected in favor of Option B:
  doctor audit plus fix
* A fourth Copilot cycle exceeded the nominal review-cycle count, but it found a
  genuine clobber-risk bug, so the additional fix improved data safety

## Provenance

The compacted summary above preserves the decision and outcome content from
these archived originals:

* `docs/archive/memory/2026-06-23-stage-docline-frontmatter-session.md`
* `docs/archive/memory/2026-06-25-065-run2-migration-checkpoint.md`
* `docs/archive/memory/2026-06-25-065-run2-session-summary.md`
* `docs/archive/memory/2026-06-25-ship-065-docline-run1-checkpoint.md`
* `docs/archive/memory/2026-06-25-ship-065-docline-run1-final.md`
* `docs/archive/memory/2026-06-25-stage-b8ff7590-manifest-drift-checkpoint.md`
* `docs/archive/memory/2026-06-26-ship-065-S-closure.md`
* `docs/archive/memory/2026-06-26-ship-067-S-archived-from-integrity.md`
* `docs/archive/memory/2026-06-26-stage-067-S-archive-archived-from-integrity.md`
* `docs/archive/memory/2026-06-27-ship-060-S-post-merge-closure.md`
* `docs/archive/memory/2026-06-27-ship-060-S-shipment-state-integrity.md`
* `docs/archive/memory/2026-06-27-ship-061-S-post-merge-closure.md`
* `docs/archive/memory/2026-06-27-ship-067-S-post-merge-closure.md`
* `docs/archive/memory/20260623-172900-ship-064-S-blocked-duplicate-of-058-memory.md`
* `docs/archive/memory/20260623-184000-stage-064-S-reconcile-duplicate-of-058-checkpoint.md`
* `docs/archive/memory/20260625-002500-ship-066-S-post-merge-closure-complete.md`
* `docs/archive/memory/20260627-175500-ship-061-S-harness-checkpoint.md`
* `docs/archive/memory/20260627-183000-ship-061-S-session-summary.md`
