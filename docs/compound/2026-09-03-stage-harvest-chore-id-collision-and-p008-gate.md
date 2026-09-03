---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "A chore-rooted release unit can silently fail to parent tasks when an archived same-number chore's child-task IDs collide, and a Stage harvest PR merges only after new .backlogit/queue/*.md plus exec-plans clear the repo-wide P-008 markdown gate"
source: docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md
doc_type: learning
description: "Two Stage-harvest traps hit during a dark-factory run. (1) CHORE ID REUSE: creating a covering chore assigns the next chore number (e.g. 001-C) even when a previously ARCHIVED chore already used 001-C; the new chore's child-task IDs (001.001-T ...) then collide with the archived namespace on the canonical filesystem, so `backlogit add --type task --parent 001-C` fails with 'artifact ID already exists on the canonical filesystem'. Because the batch suppressed stderr (2>$null), the failures were silent and the chore ended up with zero children. Fix: for a covering release unit that will parent tasks, prefer --type feature (feature numbering had no archived collision) or verify the chore's child-ID namespace is free first; a query for parent_id children returning 0 while a probe reports 'already exists' is the signature of an archived-namespace collision, not a create bug. (2) P-008 IS REPO-WIDE OVER TRACKED MARKDOWN: the Markdown lint (P-008: MD001/MD025/MD041) CI gate lints every non-gitignored tracked .md, which now includes the harvested .backlogit/queue/*.md AND the Stage-authored docs/exec-plans and docs/memory files. Two authoring habits break it: a `#### Plan Hardening Signals` heading placed directly under an `##` section is an MD001 heading-increment jump (##->####); demote it to `###`. A memory/checkpoint .md whose first line is an HTML comment or code fence (not `# H1`) is an MD041 first-line-h1 violation; give it an H1 first line. The docline frontmatter gate and P-008 are SEPARATE gates: passing `backlogit docs lint` does NOT imply passing markdownlint. On Windows, scripts/md-lint.sh fails with $'\\r' CRLF errors under bash; run scripts/md-lint.ps1 instead to reproduce CI locally."
docline:
    date: 2026-09-03T00:00:00Z
    severity: medium
    tags:
        - stage
        - harvest
        - backlogit
        - markdownlint
        - ci
---

# Stage harvest: chore-ID collision and the repo-wide P-008 markdown gate

Two independent traps surfaced while harvesting 13 shipments during a dark-factory
Stage run. Both are silent until a late signal (zero children; a failing CI gate).

## Trap 1 — archived chore ID reuse blocks task children

`backlogit add --type chore` assigned `001-C` even though an archived chore had
previously used `001-C` with children `001.001-T`, `001.002-T`, ... still present
under `.backlogit/archive/`. Creating a task under the new `001-C` tries to mint
`001.001-T`, which collides on the canonical filesystem:

```text
Error: create artifact: create artifact "001.001-T": backlogit: artifact ID already exists on the canonical filesystem
```

The batch had `2>$null`, so every S13 task-create failed silently and the chore
had zero children. A `SELECT parent_id, COUNT(*) ... GROUP BY parent_id` that omits
the chore, combined with a probe that reports "already exists", is the fingerprint.

Fix applied: delete the empty `001-C` and re-create the covering unit as a
`feature` (`165-F`) — feature numbering had no archived collision — then add the
tasks. General rule: prefer `feature` covering units, or verify the child-ID
namespace is free before rooting tasks under a low-numbered chore.

## Trap 2 — P-008 lints harvested backlog files and Stage docs

The `Markdown lint (P-008)` gate (MD001 heading-increment, MD025 single-H1,
MD041 first-line-H1) runs repo-wide over every non-gitignored tracked `.md`. A
Stage harvest commit adds `.backlogit/queue/*.md`, `docs/exec-plans/*.md`, and
`docs/memory/*.md` — all in scope.

- `#### Plan Hardening Signals (REQUIRED)` directly under an `##` section is an
  MD001 `##`->`####` jump. Demote to `###`.
- A memory/checkpoint `.md` starting with an HTML comment or code fence violates
  MD041. Start it with an `# H1` line.

`backlogit docs lint` (docline frontmatter) and markdownlint P-008 are SEPARATE
gates — passing docline does not imply passing P-008. Reproduce P-008 locally
with `scripts/md-lint.ps1` on Windows (`scripts/md-lint.sh` dies on CRLF under
bash).
