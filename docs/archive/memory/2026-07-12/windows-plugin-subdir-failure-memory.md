---
chunk_strategy: h1-h2-h3
description: Session memory for documenting Windows failure of the plugin subdirectory install form.
doc_type: memory
docline:
    ms.date: 2026-07-12T00:00:00Z
    ms.topic: memory
schema_version: "1.0"
source: docs/memory/2026-07-12/windows-plugin-subdir-failure-memory.md
title: Windows plugin subdirectory failure memory
---

## Outcome

Feature `098-F` documented that
`copilot plugin install softwaresalt/backlogit:plugin` fails on Windows with
`The directory name is invalid. (os error 267)`, because the colon becomes part
of Copilot CLI's installed-plugin directory name. PR #220 merged with merge
commit `bd8331e4f4a386b25dd12d2485992ea6f979fe75`.

## Files changed

* `README.md`
* `docs/plugin-guide.md`
* `docs/installation.md`
* `docs/closure/2026-07-12-plugin-install-path-fix-closure.md`
* `tests/integration/plugin_manifest_test.go`
* `.backlogit/queue/098-F.md`

## Verification

* TDD red and green for plugin install docs drift guards
* `go run ./cmd/backlogit docs lint`
* `go build ./cmd/backlogit`
* `go test ./...`
* `go vet ./...`
* `golangci-lint run`
* changed Go file formatting check was empty

## Decisions

* The plain `copilot plugin install softwaresalt/backlogit` command remains the
  canonical cross-platform install path
* Active docs must not recommend the colon subdirectory install form
* Closure records should preserve the Windows `os error 267` field result

## Compact-context result

Compaction was assessed at batch completion. `docs/memory/` contained 24 files
and 84.43 KB, below the 40-file and 500 KB thresholds, with no old memory
candidates found. No files were archived.
