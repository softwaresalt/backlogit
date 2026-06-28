---
chunk_strategy: h1-h2-h3
description: 'Post-merge lightweight runtime verification for shipment 068-S — codec extraction proven behavior-preserving via targeted package tests, byte-identical gen-docs (no CLI Reference Drift), body-preserving docs-migrate dry-run (0 body-byte changes), and live ship dogfooding of the refactored archive/doctor path (PR #148, merge 7450271a)'
doc_type: closure
docline:
    ms.date: 2026-06-28T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T19:50:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-28-068-S-codec-extraction-runtime-verification.md
title: 068-S Shared Frontmatter Codec Extraction — Post-Merge Runtime Verification
---

# Runtime Verification — Shipment 068-S (Shared Frontmatter Codec Extraction)

- **Surface**: CLI / library (`internal/mdfront`, `internal/atomicfile`, `internal/docline`, `internal/core` doctor + archive lifecycle, `cmd/gen-docs`). No runtime service, web, or background-job surface.
- **Mode**: automated test suite + manual command verification (no live mutation beyond the 068-S ship itself).
- **Context**: Ship Step 6 post-merge closure for 068-S; merge commit `7450271a3334fc9c780ba757de6cf390a40edf3c` (PR #148), default branch `main`.
- **Verdict**: **PASS**

## Invariants under test

This is a **pure refactor** (Option B). The single overarching invariant is *behavior preservation*: extracting the body-preserving frontmatter codec into `internal/mdfront` and the atomic-write helper into `internal/atomicfile` must not change any observable behavior.

1. The public `internal/docline` API is unchanged — `docline.Markdown` is a true type alias of `mdfront.Markdown`, `Encode` is the inherited method, and `Decode` forwards to `mdfront.Decode`.
2. `cmd/gen-docs` output is byte-identical (CLI Reference Drift stays green).
3. The docline migration remains idempotent and body-preserving (0 body-byte changes).
4. The `doctor --check-archived-from` audit and the archive-stamping path (now built on the leaf packages) are unchanged.
5. The `internal/docline -> internal/core` codec/atomic-write duplication is removed and the import cycle stays broken — `mdfront` and `atomicfile` are stdlib-only leaf packages.

## Environment prechecks

- Binary under test: repo-root `backlogit.exe` (v1.2.0), freshly built from `main` @ `7450271a`, carrying the `docs` and `doctor --check-archived-from` subcommands. Module target is `go 1.24.0` (`go.mod`); CI validates the matrix `["1.23", "1.24"]`. The local build toolchain (`go1.26.4`) is forward-compatible with that target.
- Workspace: `.backlogit/` (638 artifacts indexed). No service/port/credential dependencies — this is a library + CLI change.
- Package layering confirmed: `internal/mdfront/codec.go` imports only `bytes`, `fmt`, `gopkg.in/yaml.v3`; `internal/atomicfile/atomicfile.go` imports only stdlib (`fmt`, `io`, `os`, `path/filepath`, `runtime`). Neither imports any internal package — both are leaves.

## Evidence

### E1 — Targeted package test suites (green)

`go test ./internal/mdfront/... ./internal/atomicfile/... ./internal/docline/... ./internal/core/... -count=1`

```text
ok  github.com/softwaresalt/backlogit/internal/mdfront        1.736s
ok  github.com/softwaresalt/backlogit/internal/atomicfile     1.516s
ok  github.com/softwaresalt/backlogit/internal/docline        5.928s
ok  github.com/softwaresalt/backlogit/internal/core          36.233s
ok  github.com/softwaresalt/backlogit/internal/core/templates 7.314s
```

Proves invariants (1), (4), (5). The docline suite includes the migration idempotency / body-preservation characterization tests; the core suite includes the doctor archived_from audit and `rewriteArchivedFromField` golden differential byte-equality tests.

### E2 — gen-docs byte-identity (no CLI Reference Drift)

```text
go run ./cmd/gen-docs docs/cli-reference
git diff --exit-code --ignore-cr-at-eol -- docs/cli-reference/   # exit 0
```

Regenerating the full CLI reference from the refactored codec path produced **zero content drift** (exit 0). Proves invariant (2): `gen-docs` output is byte-identical post-extraction. (The local working copy showed only LF/CRLF autocrlf noise, which is absent on the Linux CI runner; the regenerated files were restored to keep the closure branch clean.)

### E3 — docs migrate dry-run is body-preserving

```text
backlogit docs migrate --format json   # dry_run: true
```

The plan listed **233** frontmatter-normalization `update` entries and **0** entries with `body_bytes_changed: true`. The codec round-trips every in-scope doc body byte-for-byte. Proves invariant (3). (The 233 frontmatter-only normalizations are a pre-existing repo baseline unrelated to this extraction; none touch body bytes.)

### E4 — docs lint clean (CI gate baseline)

```text
backlogit docs lint   # {"valid": true, "violation_count": 0, "findings": []}
```

The `Docline frontmatter gate` (`make docs-lint` -> `backlogit docs lint`) is green on the shipped state.

### E5 — Live ship dogfooding of the refactored archive/doctor path

This closure's own `shipment ship 068-S` exercised the refactored archive + frontmatter-stamping path end to end. All 6 newly-archived records carry canonical `archived_from` (0 self-referential):

```text
068-F     -> archived_from: .backlogit/queue/068-F.md
068-S     -> archived_from: .backlogit/queue/068-S.md
068.001-T -> archived_from: .backlogit/queue/068.001-T.md
068.002-T -> archived_from: .backlogit/queue/068.002-T.md
068.003-T -> archived_from: .backlogit/queue/068.003-T.md
068.004-T -> archived_from: .backlogit/queue/068.004-T.md
```

`doctor --check-orphans=false --check-duplicates=false --check-archived-from`: **0** `archived_from_self_ref`; **2** `archived_from_malformed` (`038-DL`, `039-DL`, value `done`) — the known flag-only records, unchanged. The re-archive of the 4 pre-archived tasks changed only metadata fields (`archived_from`, `archived_status`, `commit`, `status`, `updated_at`) — the description body bytes were preserved (confirmed via `git diff` of `068.001-T.md`). Proves invariant (4) on live data and re-confirms the body-preserving codec (3) through the migrated `internal/core/doctor.go` archive path.

## Follow-up risks

- The 2 malformed `archived_from: done` records (`038-DL`, `039-DL`) remain flag-only by deliberate operator decision; doctor surfaces them every run. Pre-existing; not introduced by this shipment.

## Handoff to operational-closure

- **Verdict**: PASS
- **Surfaces verified**: frontmatter codec (`mdfront`), atomic write (`atomicfile`), docline public API + migration, core doctor/archive lifecycle, gen-docs output.
- **Evidence**: targeted package suites green; gen-docs byte-identical; docs-migrate 0 body-byte changes; docs lint clean; live ship dogfooding (6 records canonical, doctor 0 self-ref, body preserved).
- **Follow-up**: none introduced by this shipment.
