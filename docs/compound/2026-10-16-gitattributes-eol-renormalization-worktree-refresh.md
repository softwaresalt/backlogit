---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "After a .gitattributes eol=lf renormalization, git restore does not refresh CRLF working files; delete and restore them"
description: "When the index already holds LF content, a plain git restore leaves CRLF working-tree files untouched because git sees no change. Refresh a scoped, digest-verified file set by deleting then restoring it."
doc_type: learning
source: docs/compound/2026-10-16-gitattributes-eol-renormalization-worktree-refresh.md
docline:
    date: 2026-10-16T00:00:00Z
    severity: medium
    tags:
        - gitattributes
        - line-endings
        - renormalization
        - 157-S
---

# eol=lf renormalization: plain git restore is a no-op

## Problem

Shipment `157-S` added `eol=lf` rules for tracked Go and JSON files. Working-tree
files that still had CRLF endings stayed CRLF after `git restore <paths>`, and
`git status` reported them clean. The index already held LF content, so git saw no
difference between index and worktree under the new attributes and rewrote nothing.

## Fix

1. Verify the target set first: compare the live file list against the pinned baseline
   digest (here 564 files, 528 Go and 36 JSON). Never widen beyond that set.
2. Delete only those worktree files, then run `git restore -- <the same paths>`. Checkout
   now writes each file with LF.
3. Confirm no tracked `*.go` or `*.json` file contains CRLF and that `main` still equals
   `origin/main`.

Deleting tracked files is destructive in form, so run it only under explicit operator
approval, scoped to the verified set. Refreshing also rewrote 26 `testdata` paths, which
the operator approved by digest because only line endings changed.

## Related

* `docs/closure/157-S-175-F-post-merge-closure.md`
