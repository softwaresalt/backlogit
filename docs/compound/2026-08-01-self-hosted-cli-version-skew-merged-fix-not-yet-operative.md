---
chunk_strategy: h1-h2-h3
description: "When a repository self-hosts its own backlog tool (dogfooding: a stable, pinned release binary like C:\\Tools\\backlogit.exe manages this repo's own .backlogit/ artifacts, separate from the in-progress source tree), a fix merged to that tool's own source code does NOT retroactively protect real backlog operations performed via the currently-installed release binary. The fix only becomes operative once (a) a new release is cut from a commit at or after the fix, and (b) the operator upgrades the installed binary past that release. Discovered during shipment 115-S closure: core.ShipShipment's over-archiving-covering-feature fix (feature 133-F, PR #327, merge 47dfcc93) was merged to main, but the installed C:\\Tools\\backlogit.exe v1.7.0 (embedded commit 7daf8c3, built 2026-07-23) predates it by 9 days. Every real `backlogit` CLI invocation this session -- including the actual `backlogit shipment ship 115-S` closure call -- ran under the pre-fix binary. 115-S itself was unaffected only because its own feature (133-F) has no parent feature, so the specific bug's precondition (a non-member covering-feature ancestor) never existed for this shipment; a future partial-feature shipment with a real covering-feature hierarchy, closed via the same pinned CLI, remains exposed to the original bug until the CLI is upgraded."
doc_type: learning
docline:
    date: 2026-08-01T00:00:00Z
    severity: high
    tags:
        - ship
        - post-merge
        - closure
        - self-hosting
        - dogfooding
        - build-artifact
        - stale-binary
        - version-skew
        - cli
        - shipment
schema_version: "1.0"
source: docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md
title: "Self-hosted backlog CLI version skew — a merged source fix does not protect real backlog operations until the pinned release binary is upgraded"
---

# Self-Hosted Backlog CLI Version Skew: A Merged Fix Is Not Yet Operative

## Context

Surfaced during shipment **115-S** post-merge closure (feature **133-F**, PR
**#327**, merge `47dfcc93698a6b0b2c5420c701c365a538895580`). 133-F fixed
`core.ShipShipment` so it no longer over-archives a covering feature (or its
terminal siblings) that is not itself an explicit shipment manifest member.
The fix was source-complete, tested (unit + a compiled-binary runtime
verification dogfood test), reviewed, and merged to `main`.

The Ship pipeline for this repository, however, is directed to operate its own
`.backlogit/` backlog through a **separately pinned, globally-installed release
binary** (`C:\Tools\backlogit.exe`), not a binary rebuilt from the in-progress
repository checkout on every operation. `backlogit version` reported:

```
version    1.7.0
commit     7daf8c3
build date 2026-07-23T22:32:43Z
```

`7daf8c3` is confirmed (`git merge-base --is-ancestor`) to be an ancestor of
`47dfcc93` — i.e. **9 days older** than the merged fix. Every real
`backlogit` CLI call performed against this repository's own backlog during
115-S's closure — including the actual `backlogit shipment ship 115-S` archival
call — therefore ran the **pre-fix** `ShipShipment` implementation, not the
code that had just been merged moments earlier in the same session.

## Why This Did Not Corrupt 115-S

`133-F` (this shipment's own feature) has **no parent feature** — it is
top-level. The bug 133-F fixes requires a non-member covering-feature ancestor
to exist in the release scope; with no ancestor at all, the precondition for
the bug never arises, so the pre-fix binary's behavior and the post-fix
binary's behavior are indistinguishable for this specific shipment (confirmed:
`archived_ids` exactly matched the 6 manifest items + the shipment record,
`returned_ids` was empty). The absence of corruption here is a property of
115-S's shape, not evidence that the installed CLI is safe for the general
case.

## The General Risk

A repository that dogfoods its own tooling this way has an inherent
bootstrapping gap: **fixing a bug in the tool's source does not fix the tool
your workflow actually runs**, because the workflow is pinned to a release
artifact, not to `HEAD`. Any future **partial-feature shipment** — a shipment
whose covering feature has a parent and/or unshipped sibling tasks outside the
manifest — closed via the still-pinned pre-fix `C:\Tools\backlogit.exe`,
remains fully exposed to the original over-archiving cascade bug, **even
though `main` already contains the fix and even though every closure report
after this point could easily (and wrongly) assume the fix is "live."**

This is a distinct failure mode from the one already documented in
`docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`. That
entry concerns a **local development build** going stale relative to
uncommitted or just-merged changes in the same working copy — solved by
"just rebuild before you dogfood." Here, the operational convention is to use
a **separately versioned, deliberately pinned release binary** for day-to-day
backlog operations; "just rebuild" does not apply because there is no local
rebuild step in that operational path at all. The fix does not become
operative until a **new backlogit release is cut** from a commit at or after
the fix, and the operator/environment **upgrades the installed binary** to
that release.

## Rule

When a shipment's own change modifies the backlog tool's own lifecycle,
archival, or mutation logic (i.e. the repository is shipping a fix to the very
tool used to manage its own `.backlogit/` state):

1. **Check the installed CLI's embedded commit** (`backlogit version`) against
   the merge commit before treating the fix as protecting real backlog
   operations performed through that CLI.
2. If the installed CLI **predates** the fix, explicitly record in the
   closure artifact that the fix is **merged but not yet operative** for real
   backlog operations, and name the exact residual exposure (which future
   shipment shapes remain at risk) rather than implying the fix is
   universally active once merged.
3. Do not attempt to unilaterally replace or upgrade a pinned system-wide
   tool install as a side effect of closing a single shipment — that is a
   separate release/deployment decision outside a single release unit's
   scope. Report the gap; do not paper over it by silently rebuilding a
   system path binary.
4. A `doctor`-style structural audit that can detect a regression **after the
   fact** (e.g. this repository's `check-over-archived-features` check) is a
   useful safety net precisely because the fix cannot be assumed live from the
   merge alone — the audit should be run against the CLI actually used for
   production-like operations, not just against a freshly built dev binary.

## Applicability

Any project that dogfoods its own CLI/tool to manage its own operational state
(backlog trackers, migration tools, config linters, code generators) where the
tool is invoked as a pinned, separately versioned release binary rather than
rebuilt from source on every invocation. The generalized reflex: **"the fix
is merged" and "the fix is live in every environment that uses this tool"
are different claims** — verify the second one explicitly whenever a shipment
changes the tool's own operational behavior, instead of assuming merge implies
deployment.
