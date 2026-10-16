---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Root-confined file access, GraphQL RequestedReviewer union, and live query validation"
description: "Use os.OpenRoot for root-confined file access, remember the GraphQL RequestedReviewer union excludes Bot, and validate gh GraphQL queries live before relying on them."
doc_type: learning
source: docs/compound/2026-10-16-root-confined-access-and-graphql-reviewer-union.md
docline:
    date: 2026-10-16T00:00:00Z
    severity: low
    tags:
        - security
        - graphql
        - gh
        - 141-S
---

# Root-confined access, RequestedReviewer union, live query validation

## Problem

Three review findings on PR #512 (141-S) recurred across Copilot cycles.

## Learnings

* **Root-confined file access.** When a validator reads files named by untrusted content, use `os.OpenRoot` (Go 1.24+) instead of joining and prefix-checking paths. It rejects `..` traversal and symlink escapes by construction.
* **GraphQL `RequestedReviewer` union.** The union covers `User`, `Team`, `Mannequin` and does not include `Bot`. A `... on Bot` fragment inside `requestedReviewer` is invalid and the query fails.
* **Validate queries live.** Run each `gh api graphql` query read-only against a real PR before relying on it, since schema mistakes only appear against the live API.

## Evidence

PR #512 review cycles; the live review query was verified read-only against PR 512.
