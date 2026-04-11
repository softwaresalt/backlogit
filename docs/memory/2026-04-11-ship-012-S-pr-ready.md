# Ship 012-S: PR Ready — Awaiting Merge Approval

**Date**: 2026-04-11
**Shipment**: 012-S — Build, Docs & CLI Parity
**Branch**: `chore/012-S-build-docs-cli-parity`
**PR**: #27 — https://github.com/softwaresalt/backlogit/pull/27
**Status**: PR open, CI green, awaiting user merge approval

## Completed Items

| Item | Title | Status |
|------|-------|--------|
| 028-F | Build, Docs & CLI Parity | review |
| 028.001-T | Format all Go files and fix Makefile fmt target | done |
| 028.002-T | Fix documentation accuracy (Go version, DB name, CLI syntax) | done |
| 028.003-T | Fix stash documentation (fetch-stash → list) | done |
| 028.004-T | CLI add flag parity and duplicate persistence removal | done |
| 028.005-T | CLI update flag parity, duplicate removal, and relocation fix | done |

## Blocked Items

None.

## Quality Gates Passed

- [x] `gofmt -l .` — zero output
- [x] `go vet ./...` — clean
- [x] `go build ./...` — success
- [x] `go test ./...` — all 16 packages pass
- [x] CI (Go 1.23) — SUCCESS
- [x] CI (Go 1.24) — SUCCESS
- [x] Code review gate — no P0/P1 findings

## Key Decisions

- D1: Comma-separated strings for `--labels`, `--dependencies`, `--references` (matches MCP)
- D2: Removed duplicate persistence calls (core functions handle full persistence)
- D5: `FindArtifactPath` re-resolved after frontmatter update to handle relocation
- D7: `splitCSV` helper for comma-separated flags

## Commits

1. `bf12725` — chore(build): format Go files, fix docs accuracy, CLI add/update flag parity
2. `bdfa5c3` — chore(backlog): update 012-S items to done/review status

## Next Steps

- Awaiting user merge approval for PR #27
- After merge: run post-merge closure protocol (shipment → shipped, documentation updates)
