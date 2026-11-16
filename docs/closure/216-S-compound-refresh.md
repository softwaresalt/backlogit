---
title: "216-S compound refresh and knowledge evaluation"
doc_type: closure
source: docs/closure/216-S-compound-refresh.md
ingested_at: "2026-11-16T17:40:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 216-S
  feature_id: 215-F
---

## Invocation and evidence

Invoked `compound-refresh` (`scope: recent`, `mode: propose`) for shipment
`216-S` (`215-F`) and evaluated durable documentation under Ship Step 6 items 3
and 4. Evidence sources: merged PR `#654` (`04944c1ed866666ec6b22f948ba2822ebbc1612c`),
`internal/atomicfile/*`, `internal/toolchain/*`, `internal/mcp/*`, `internal/cli/*`,
`internal/core/gate/baseref.go`, `internal/core/id_history*`,
`internal/core/shipment_lifecycle.go`, `internal/core/shipment_reconstitute_exec.go`,
and `docs/closure/evidence/216-S-215-F-close-evidence.json`.

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Pin the handle, not the path](../compound/2026-10-27-pin-the-handle-not-the-path.md) | `keep` | Accurate and reinforced by `atomicfile.ReplaceTempFileInRoot`, Windows `verifyExpectedDirMatchesHandle`, POSIX `verifyLocksDirHandle`/`os.SameFile`, and `self_install.go` `*os.Root` handle pinning |
| [Nested os.Root Name is not an absolute validation path](../compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md) | `keep` | Accurate; `ReplaceTempFileInRoot` (`U1`) explicitly accepts caller-verified `dirAbs` separately from `*os.Root` |
| [Protect reconstitution preliminary execution reads](../compound/concurrency-issues/reconstitution-preliminary-reads-under-canonical-execution-locks-2026-11-06.md) | `keep` | Accurate; `U5b` keeps `id_allocator.lock` strictly as a leaf lock (`BeforeRename` journals without `B`/`C`, and `adoptItem` releases `id_allocator.lock` before `C` and uses `unreserveIDSequences` after `C` release) |
| [Unregister completed request cancellation bridges](../compound/concurrency-issues/unregister-completed-request-session-cancellation-2026-11-09.md) | `keep` | Accurate; `U3a`/`U3b` builds on session teardown without breaking request-context bridges (`Shutdown` stops session + coordinator admission before draining) |

Reviewed: 4 entries (`keep: 4`, `update: 0`, `consolidate: 0`, `replace: 0`, `delete: 0`).

## Documentation graduation assessment

`docs/ARCHITECTURE.md` and `AGENTS.md` require no structural updates for `216-S`
(no new packages or agent boundaries were introduced, and `CLI Reference Drift`
CI verified `docs/cli-reference.md` has zero drift). Four out-of-scope P-021 C2
`DEFERRED SCOPE EXPANSION` entries (`39F868B1`, `E0A6FDAF`, `7C26BE01`, `DE5D1EA5`)
and one closure follow-up (`18579EF1`) remain active in `.backlogit/stash.jsonl`
for Stage triage.
