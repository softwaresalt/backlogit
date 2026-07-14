---
title: "Post-merge lifecycle writes must use a binary built from the merged source, not a stale workspace binary"
source: docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md
doc_type: learning
description: "A Ship agent's post-merge closure step often runs a workspace CLI (e.g. backlogit.exe) to perform state-mutating lifecycle operations like ship_shipment, which WRITE artifacts. If that binary was built before the feature merged, it silently emits pre-fix output — here, item-artifact timestamps in local -07:00 offset instead of the canonical UTC Z that the just-merged change guarantees. The workspace binary is a build artifact whose freshness is not enforced by git. Rule: before any post-merge operation that writes artifacts, rebuild the tool from the merged HEAD (or assert its build is newer than the merge commit) and verify the write path, so closure does not re-emit the very defect it is closing."
docline:
    date: 2026-07-13T00:00:00Z
    severity: high
    tags:
        - ship
        - post-merge
        - closure
        - build-artifact
        - stale-binary
        - lifecycle
        - determinism
        - dogfooding
---

# Post-Merge Lifecycle Writes Must Use a Binary Built From the Merged Source

## Context

Surfaced during shipment 092-S post-merge closure (PR #236). 092-S normalized
every item-artifact writer to emit `created_at`/`updated_at` in canonical UTC
(`Z`). After the feature PR #235 merged (`4a90bf4`, 22:24), the closure step ran
`.\backlogit.exe shipment ship 092-S --sha 4a90bf4` to archive the shipment and
record the merge SHA. `ship_shipment` **writes** frontmatter (`updated_at`,
archival keys) to every member artifact. The closure PR's Copilot review then
flagged that all 13 newly-archived members carried `updated_at: …-07:00` — the
exact local-offset defect 092-S was closing.

## Problem

The workspace `backlogit.exe` had been built at **11:23 the same morning**,
~11 hours *before* the 22:24 merge, so it predated the `models.NowUTC()` change
and still stamped `time.Now()` (local offset). The trap has three properties that
make it easy to miss:

1. **The tool is a build artifact, not source.** `backlogit.exe` is gitignored;
   git cleanliness says nothing about whether the binary reflects `HEAD`. A clean
   tree with a stale binary looks fine.
2. **The defect is self-inflicted and on-topic.** The closure of a fix re-emits
   that very fix's defect, so the artifact "proof of closure" actually disproves
   it — the worst place for it to appear.
3. **Silent.** No error; the write succeeds and looks plausible (`-07:00` is a
   valid timestamp), so only a format-aware reviewer/gate catches it.

## Solution

Before any post-merge (or post-build) operation that **writes** artifacts with a
workspace tool:

1. **Rebuild the tool from the merged HEAD first** — `go build -o backlogit.exe
   ./cmd/backlogit` (or `go run ./cmd/backlogit …`, which always compiles current
   source). Treat "rebuild the CLI" as step 0 of closure, not an afterthought.
2. **Or assert freshness** — compare the binary's build/mtime against the merge
   commit time and refuse to proceed if older
   (`(Get-Item backlogit.exe).LastWriteTime` vs `git show -s --format=%ci HEAD`).
3. **Verify the write path** the operation exercises — run the merged tests that
   assert the emitted format (`go test … -run UTC`), and after the operation,
   grep the written artifacts for the expected canonical form.
4. **Repair, don't hide.** If a stale binary already wrote bad output: rebuild,
   re-verify the write path, and normalize the emitted values to the correct
   canonical form (instant-preserving where the value is a timestamp), then record
   the incident transparently in the closure artifact rather than silently
   overwriting.

## Applicability

Any agent/automation that shells out to a locally-built CLI to perform
state-mutating work right after that CLI's own code changed — backlog tooling,
code generators, migration tools, formatters. The generalized reflex: **a tool
you are shipping changes to is exactly the tool most likely to be stale in your
own workspace** — rebuild-from-HEAD before you dogfood it on the release you just
merged, and verify its output against the guarantee you are closing.
