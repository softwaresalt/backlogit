---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "UTC-normalized frontmatter timestamps via a shared NowUTC() helper with a backward-compatible parse path"
source: docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md
doc_type: learning
description: "Item-artifact writers that stamp created_at/updated_at with time.Now() emit a machine-local offset, so identical artifacts serialize differently across CI runners and developer machines. Route every write site through one exported helper — models.NowUTC() = time.Now().UTC() — so emission is canonical UTC with a trailing Z, while keeping the read/parse path offset-tolerant so historical +/-hh:mm timestamps still load. Exporting the helper from the lowest package (models) lets templates and cli reuse it without an import cycle."
docline:
    date: 2026-07-13T00:00:00Z
    severity: medium
    tags:
        - timestamps
        - utc
        - frontmatter
        - go
        - serialization
        - backward-compatibility
        - import-cycle
        - determinism
---

# UTC-Normalized Frontmatter Timestamps via a Shared NowUTC() Helper

## Context

Surfaced by shipment 092-S (feature 103-F, PR #235, merge `4a90bf4`). backlogit
item artifacts (tasks, features, shipments) carry `created_at` / `updated_at`
frontmatter. Every writer stamped these with `time.Now()`, which serializes in
the machine's local zone — e.g. `2026-07-13T22:33:20-07:00`. The same logical
write therefore produced different bytes on a `-07:00` developer laptop, a UTC CI
runner, and a `+02:00` machine, and mixed offsets accumulated across the corpus.
(Recommended as a follow-up out of 091-S; realized here.)

## Problem

1. **Non-deterministic emission.** Local-offset timestamps make artifact bytes
   depend on the writer's wall-clock zone, so diffs, fixtures, and golden files
   drift by geography rather than by content.
2. **Many independent write sites.** Timestamp stamping was scattered across
   ~10 files / 11 logical sites (`models`, `core`, `core/templates`, `cli`).
   Fixing them one-off risks missing a site and re-introducing local offsets.
3. **Import-cycle trap.** The natural home for a shared helper is a low-level
   package, but `core/templates` and `cli` both need it — placing it too high
   (e.g. in `core`) would make `templates`/`cli` import `core` where they must
   not, creating a cycle.
4. **Read-path regression risk.** A naive "UTC everywhere" change that also
   tightens parsing would reject the historical `+/-hh:mm` artifacts already on
   disk — a silent backward-incompatibility.

## Solution

1. **One exported helper at the lowest package.** Add to `internal/models`:

   ```go
   // NowUTC returns the current wall-clock time normalized to UTC, so that every
   // item-artifact writer serializes created_at/updated_at with a canonical
   // trailing "Z" instead of a machine-local offset. It is exported so both
   // internal/core/templates and internal/cli can reuse it without importing
   // internal/core (which would form an import cycle).
   func NowUTC() time.Time { return time.Now().UTC() }
   ```

   `models` sits below `core`, `core/templates`, and `cli`, so all of them can
   depend on it with no cycle. Exporting (not a package-private `nowUTC`) is the
   deliberate mechanism that lets `templates` and `cli` share the single
   normalization point.

2. **Route every write site through it.** Replace `time.Now()` with
   `models.NowUTC()` at each timestamp write — 11 sites here: `models`
   (`ArtifactFromFrontmatter` defaults), `core/artifacts.go`, `core/queue.go`,
   `core/shipment.go`, `core/shipment_lifecycle.go` (commit/status/cascade **and**
   `clearParentID`/`AdoptItem` — two logical sites in one file),
   `core/gate_transition.go`, `core/artifact_references.go`,
   `core/migrate_links.go`, `core/templates/service.go`, `cli/update.go`
   (the `update --section` path). Keep the `time` import where a file still uses
   `time` for non-stamping purposes (e.g. event-log formatting in `shipment.go`).

3. **Keep the read/parse path offset-tolerant.** `ArtifactFromFrontmatter` still
   accepts a parsed `time.Time` regardless of its zone
   (`if v, ok := fm["created_at"].(time.Time); ok { ... }`), so historical
   `+/-hh:mm` artifacts continue to load unchanged. Normalize on **write**, stay
   liberal on **read** — this is what makes the change backward-compatible rather
   than a corpus migration.

4. **Assert the exact trailing `Z`, not merely a zero offset.** Tests assert the
   emitted string ends with `Z` and does **not** match `[+-]\d{2}:\d{2}$`. A
   `+00:00`/`-00:00` suffix is a zero *offset* but is not canonical `Z`; asserting
   `Z` catches a writer that computed UTC but serialized it with an explicit
   numeric offset.

## Applicability

Any Go service that serializes timestamps into files/records and wants
byte-deterministic, machine-independent output. Reflexes: (a) inventory **every**
write site (an exhaustive sweep, not a spot fix) before editing; (b) put the
shared clock helper in the lowest common package and **export** it to dodge
import cycles; (c) normalize on write, stay liberal on read to preserve
backward compatibility with already-emitted data; (d) assert the exact canonical
form (`Z`) rather than a semantically-equal variant (`+00:00`). The RED-phase
proof that local emission actually fails on a UTC CI runner needs a controlled
non-UTC zone — see
`docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md` for the
parallel-test-safe way to inject one.
