---
title: "008-S Post-Merge Closure Complete"
description: "Final session memory for shipment 008-S. Shipped, archived, closure PR #20 open and CI green."
date: 2026-04-10
origin: session-memory
status: shipped
---

## Shipment Status

**Shipment 008-S** (Workspace Governance and Archival Policies) is **shipped**.

- **Branch (feature):** `025-workspace-governance-integrity`, merged to `main` as `a058efe`
- **PR (feature):** [PR #19](https://github.com/softwaresalt/backlogit/pull/19) (merged)
- **PR (closure):** [PR #20](https://github.com/softwaresalt/backlogit/pull/20) (CI green, awaiting merge)
- **Latest commit (closure branch):** `a458dbe`
- **Shipment status:** `shipped` (via `backlogit shipment ship 008-S --sha a058efe`)

## All Units Complete

| Task | Unit | Description | Status |
|---|---|---|---|
| 025.014-T | 4 | Archive lifecycle | ✅ archived |
| 025.013-T | 2 | Hierarchy enforcement | ✅ archived |
| 025.015-T | 3 | Stash concurrency P0 fix | ✅ archived |
| 025.016-T | 1 | Doctor workspace diagnostics | ✅ archived |
| 025.017-T | 7 | Doctor MCP tool | ✅ archived |
| 025.018-T | 6 | Post-ship consistency verification | ✅ archived |
| 025.011-T | 5 | ShipShipment lifecycle | ✅ archived |
| 025.012-T | 8 | Integration harness | ✅ archived |

## Review Comment History

4 rounds, 22 total comments, all replied to and fixed:

- Round 1 (8 comments, commit `f942b2e`): error wrapping, nil checks, test cleanup
- Round 2 (8 comments, commit `9faa03f`): single-lock harvest, json.Decoder, wiring, test renames
- Round 3 (4 comments, commit `5d8891e`): best-effort cleanup, nil guard, registry-aware dirs, allowedParentTypes fallback
- Round 4 (2 comments, commit `9ed13fb`): registry-derived archive dirs, doctor orphan check fallback

## Post-Merge Closure

- ✅ `backlogit shipment ship 008-S`: all 11 items archived
- ✅ `docs/closure/2026-04-10-008-s-workspace-governance-closure.md`: written
- ✅ `README.md`: updated with doctor, hierarchy enforcement, post-ship verification
- ⏳ PR #20: CI green, awaiting user merge approval

## Protocol Improvements This Session

- **fix-ci SKILL.md Step 4c**: hard gate: post replies to ALL review comment
  threads after every push
- **Compound doc:** `docs/compound/workflow-issues/pr-review-comment-reply-protocol-2026-04-10.md`


## Review Comment History

### Round 1 (IDs 3062940..., commit f942b2e)

8 comments fixed and replied to. Fixes: `wrapError` → `fmt.Errorf`, early
`os.Open` error suppression removal, duplicate stale-file check consolidation,
and test cleanup.

### Round 2 (IDs 3065281..., commit 9faa03f)

8 comments fixed and replied to. Fixes: two-lock stash harvest race → single
lock; `bufio.Scanner` → `json.Decoder`; `VerifyPostShipConsistency` wired into
`ShipShipment`; test functions renamed; `description:` frontmatter added;
`.test-out.txt` and `.pr-comments.json` deleted + added to `.gitignore`.

### Round 3 (IDs 3065412..., commit 5d8891e)

4 comments fixed and replied to:
- **stash.go**: best-effort `os.Remove` on `writeStashEntries` failure to
  prevent duplicate harvests on retry
- **shipment_verify.go** (nil guard): `if ws == nil` returns descriptive error
  instead of panicking
- **shipment_verify.go** (routing): replaced `QueueLayout.RootDir`-only scan
  with `artifactSearchDirs(ws)` (registry-aware), excluding archive dir
- **artifacts.go**: hierarchy enforcement now falls back to `allowedParentTypes`
  when `QueueLayout` is nil, so level-2+ types always require `parent_id`

### Round 4 (IDs 3065518..., commit 9ed13fb)

2 comments fixed and replied to:
- **shipment_verify.go** (archive dir): replaced hardcoded `"archive"` exclusion
  with registry-derived set. Loads registry rules, collects dirs whose status
  condition includes `"archived"`, falls back to `"archive"` only if none found
- **doctor.go** (orphan check): removed `QueueLayout != nil` gate; when
  QueueLayout is absent falls back to `allowedParentTypes(ws, artifactType)`,
  mirroring the round-3 fix to `validateArtifactParent` exactly

- **fix-ci SKILL.md Step 4c** added as standalone hard gate: post replies to
  all review comment threads (NON-NEGOTIABLE). Commit `4dba37a`.
- **Compound doc** at
  `docs/compound/workflow-issues/pr-review-comment-reply-protocol-2026-04-10.md`
