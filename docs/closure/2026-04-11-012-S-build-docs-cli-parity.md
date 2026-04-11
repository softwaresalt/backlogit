---
title: "012-S Closure: Build, Docs & CLI Parity"
description: Post-merge closure record for shipment 012-S
ms.date: 2026-04-11
---

## Shipment

**ID**: 012-S  
**Feature**: 028-F — Build, Docs & CLI Parity  
**Branch**: `chore/012-S-build-docs-cli-parity`  
**PR**: [#27](https://github.com/softwaresalt/backlogit/pull/27) — merged to `main` at `c5fc171`

## Delivered

| Task | Title | Status |
|------|-------|--------|
| 028.001-T | Format all Go files and fix Makefile fmt target | done |
| 028.002-T | Fix documentation accuracy (Go version, DB name, CLI syntax) | done |
| 028.003-T | Fix stash documentation (`fetch-stash` → `stash list`) | done |
| 028.004-T | CLI `add` — 8 new flags + duplicate `db.UpsertItem` removal | done |
| 028.005-T | CLI `update` — 6 new flags + relocation fix + duplicate removal | done |

## Quality Gates

| Gate | Result |
|------|--------|
| `gofmt -l .` | ✅ Zero output |
| `go vet ./...` | ✅ Clean |
| `go test ./...` | ✅ All 16 packages pass |
| `golangci-lint run` | ✅ Zero findings |
| CI Go 1.23 | ✅ Pass |
| CI Go 1.24 | ✅ Pass |
| Copilot review | ✅ 6 comments addressed |

## Copilot Review Resolutions

1. `--labels ""` no-op guard added to `update.go` (`&& labels != ""`).
2. `docs/workflow.md` dependency paragraph corrected to `backlogit_add_dependency` + `dep add/remove` CLI.
3. `TestUpdateCommand_StatusAndSection` upgraded to `active→done` transition with section-in-archive assertion.
4. `TestUpdateCommand_EmptyLabels` upgraded: creates labeled artifact, asserts `keep-me` survives empty flag.
5. Memory file `docs/memory/2026-04-11-ship-012-S-pr-ready.md` given required YAML frontmatter.

## Key Technical Decisions

- `splitCSV("")` returns an empty slice; the no-op guard uses `labels != ""` not `len(splitCSV(labels)) > 0` so the intent is clear at the flag level.
- `FindArtifactPath` is re-resolved inside the section branch (post-frontmatter update) to pick up the relocated path when `--status done` moves the file to `archive/`.
- `readArtifactFile` walks the entire `.backlogit/` tree so tests remain valid regardless of which subdirectory the artifact lands in after relocation.
