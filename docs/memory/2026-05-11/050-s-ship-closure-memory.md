---
title: "050-S Ship Closure Memory"
date: 2026-05-11
session: ship-050-s
phase: closure
status: complete
---

# 050-S Ship Session Closure Memory

## Session Summary

Closed shipment 050-S (Release Binary Readiness) through the full Ship pipeline.

## Items Completed

| Item | Title | Outcome |
|---|---|---|
| 050-S | Release Binary Readiness (shipment) | Closed/done — v1.2.0 released |
| 051-F | Release Binary Readiness (feature) | Marked done — merge commit 28aa3a9 |
| 051.010-T | Execute the tag-driven release and validate assets | Marked done |
| 051.010.001-ST | Run release workflow and verify published assets | Marked done |

All prior tasks 051.001-T through 051.009-T were already done before this session.

## PR and Branch

- **PR #114**: `chore/autoharness-mergeinstall-v1.4.0` — merged to main before this session
  - Merge commit: `28aa3a9b1a5cca4807bdcaf711d478a6167319c8`
  - All CI checks passed: CI/test (1.23), CI/test (1.24), CLI Reference Drift
  - Merged at: 2026-05-11T06:59:18Z

## Release Execution

- **Tag created**: `v1.2.0` on commit `28aa3a9b1a5cca4807bdcaf711d478a6167319c8`
- **Release workflow run**: ID 25705445696
- **All jobs passed**: Quality Gates (1.23 + 1.24), Build binaries (6 platforms), Package npm, Publish GitHub Release, Publish npm
- **GitHub Release URL**: https://github.com/softwaresalt/backlogit/releases/tag/v1.2.0

## Release Asset Validation

All 6 platform binaries published with SHA256 digests:

| Asset | Size | SHA256 |
|---|---|---|
| backlogit-darwin-amd64 | 15.24 MiB | 173bbe4acc8799cac86d5b58fb90fde0a91df183594e9fff1334ec45ff6b517c |
| backlogit-darwin-arm64 | 14.54 MiB | 29b1785038474b45afdf14efc6e841bedcf44f441138f2a3a04033518ecd8917 |
| backlogit-linux-amd64 | 14.94 MiB | 0f3a0dc055bba40319e4222205305c37149cd26d702a0a24d163507fef81f82b |
| backlogit-linux-arm64 | 14.12 MiB | 85b3ebbfaa6ee3d72dc8773dfe8d80316b7e4be23dc344fe6b6d36dc96991e73 |
| backlogit-windows-amd64.exe | 15.35 MiB | a87c4b2d1641ba0660a0c6d31fefe418188b8580d2e16c3df78fc9581ca64f76 |
| backlogit-windows-arm64.exe | 14.34 MiB | 3182b7086347653e7b9274da13dd4328f909a6cb80c1458316df1710b0d7b1ce |
| SHA256SUMS | 554 B | 1739a6e19082b3b9ed88d15420e3122cda2b3ee5f693c231a31a0490bb5c5928 |

## Version

- Source version: `1.2.0` (in `internal/version/version.go`)
- Prior latest tag: `v1.1.3`
- New tag: `v1.2.0`
- No additional version bump required (was already at 1.2.0 from 051.009-T)

## Decisions and Rationale

1. **No additional version bump**: Version was already 1.2.0 in source; constraint satisfied.
2. **Tag on merge commit**: Tagged `28aa3a9b` (PR #114 merge commit) — this represents the full merged state of all 051-F work.
3. **PR #114 already merged**: The chore/autoharness-mergeinstall-v1.4.0 branch was merged before this session began; Ship confirmed merge and proceeded to release execution.
4. **Local gofmt shows CRLF issues**: Windows `core.autocrlf=true` causes local gofmt to report formatting issues, but CI (Linux) passes because CRLF→LF conversion happens at checkout. Not a real issue.

## Next Steps

None. 050-S fully closed. Follow-up stash items are noted in the Node.js 20 deprecation warning from the release workflow — actions should be updated before September 2026.
