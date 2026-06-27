---
title: Ship memory for npm publish metadata PR
description: Session memory for the branch that adds npm publish metadata to the backlogit npm wrapper and platform packages.
ms.date: 2026-04-28
ms.topic: reference
---

## Session scope

* Branch: `chore/npm-publish-metadata`
* PR: `#81`
* Status: Open and waiting on CI plus review

## Files modified

| File | Change |
| --- | --- |
| `npm/backlogit-mcp/package.json` | Added `files`, `repository`, and `publishConfig.access` metadata |
| `npm/platforms/darwin-arm64/package.json` | Added `repository` and `publishConfig.access` metadata |
| `npm/platforms/darwin-x64/package.json` | Added `repository` and `publishConfig.access` metadata |
| `npm/platforms/linux-arm64/package.json` | Added `repository` and `publishConfig.access` metadata |
| `npm/platforms/linux-x64/package.json` | Added `repository` and `publishConfig.access` metadata |
| `npm/platforms/win32-x64/package.json` | Added `repository` and `publishConfig.access` metadata |

## Decisions and rationale

* Scoped npm packages now declare `publishConfig.access: public` so `@backlogit/*` can publish to the public npm registry without requiring a per-command override.
* The main wrapper package now declares a `files` whitelist so publish output stays limited to the wrapper runtime files.
* Each package now includes repository metadata with the correct monorepo directory for npm discoverability and provenance.
* Source-controlled package versions remain at `0.0.0`. The release workflow stamps the real release version during packaging, and the wrapper's `optionalDependencies` stay aligned with that convention in source.

## Validation

* `node --test test/resolve.test.js`
* `go test ./...`
* `go vet ./...`
* `golangci-lint run`
* `gofmt -l .` reported a pre-existing repo-wide baseline failure on unrelated Go files
* Report-only review found no material issues in the metadata diff

## Failed approaches and corrections

* I first updated the package versions to `1.1.2`. Repo documentation and the release packaging script confirmed that source should stay at `0.0.0`, so I reverted that part and kept only the metadata changes.
* A local `npm pack --dry-run` on platform packages looked inconsistent because the source tree does not mirror the release-packaged artifact layout. The release workflow copies platform binaries into the expected paths before publishing.

## Next steps

1. Wait for CI on PR `#81`.
2. Merge after approval if checks pass.
