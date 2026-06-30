---
chunk_strategy: h1-h2-h3
description: 'Durable gotcha from 070-S (070.001-T, PR #154, merge b4c317e): when you export a cache/memo type that short-circuits a safety scan, a caller in another package can construct its zero value (nil backing map). A nil-backed cache read as "empty" makes the guard treat every key as unseen and silently bypass the very check the cache was meant to optimize. Fix: treat refs == nil as UNSEEDED and lazily run the one-time scan on first use; an empty-but-non-nil map (already seeded) is left untouched so a batch still scans exactly once.'
doc_type: learning
docline:
    category: best_practice
    component: core
    date: 2026-06-29T00:00:00Z
    file_path: internal/core/canonical_cache.go
    message: An exported short-circuit cache must treat its zero value (nil backing map) as unseeded and re-scan on first use, never as an authoritative empty set
    problem_type: best_practice
    resolution_type: code_fix
    resolved: true
    root_cause: nil_vs_empty_conflation
    severity: high
    tags:
        - cache
        - memoization
        - zero-value
        - nil-vs-empty-map
        - exported-type
        - uniqueness-guard
        - lazy-seeding
        - test-seam
        - go
ingested_at: "2026-06-29T21:52:00Z"
schema_version: "1.0"
source: docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md
title: 'Exported short-circuit caches must treat their zero value as "unseeded", not "empty" (070-S)'
---

# An Exported Cache That Short-Circuits a Safety Scan Must Re-Seed Its Zero Value

A durable gotcha graduated from shipment 070-S (feature 070-F, task 070.001-T,
PR #154, merge `b4c317e`), which batched the canonical-uniqueness scan in
`CreateArtifact` so bulk callers stop re-walking the whole backlog on every
create.

## Context

`CreateArtifact` enforces ID uniqueness by scanning every queue/archive `.md`
(`scanCanonicalArtifacts`) and rejecting an ID that already exists. For bulk
callers (the migrate external-import loop, the stash priority harvest) that scan
ran once per create — O(files × creates) → O(N²) on a large backlog. The
optimization: build one `CanonicalCache` (a `refs map[string][]artifactRef`)
before the loop, pass it to every create, and skip the per-create filesystem
scan; each successful create records its new ID back into the cache so
within-batch collisions are still caught.

## The Footgun

`CanonicalCache` is an **exported** type. The original optimization trusted the
provided cache's `refs` map directly:

```go
if o.canonicalCache != nil {
    canonical = o.canonicalCache.refs   // ← trusts whatever was passed
} else {
    canonical, _ = scanCanonicalArtifactsFn(ws)
}
if existing := canonical[artifactID]; len(existing) > 0 {
    return ErrIDCollision
}
```

Because the type is exported, a caller in another package can legally construct
the **zero value** `&core.CanonicalCache{}`, whose `refs` is `nil`. A `nil` map
reads as empty for every key, so `canonical[artifactID]` is always empty and the
uniqueness guard treats **every** ID as unseen — including an ID that already
exists in queue or archive. The cache built to *speed up* the safety check would
*silently disable* it on the first create of the batch, allowing duplicate IDs.

## Root Cause

Conflating "nil backing map" (the cache was never seeded — we don't know what
exists) with "empty set" (we scanned and nothing exists). For a structure whose
whole job is to short-circuit a correctness check, those two states have opposite
safety meanings, and the zero value lands on the dangerous one.

## Resolution

Treat `refs == nil` as **unseeded** and lazily perform the one-time scan on first
use; leave an empty-but-non-nil map (already seeded by `NewCanonicalCache`)
untouched so a batch still scans exactly once:

```go
func (c *CanonicalCache) ensureSeeded(ws *Workspace) error {
    if c == nil || c.refs != nil { // already seeded (incl. empty non-nil map) → no-op
        return nil
    }
    refs, err := scanCanonicalArtifactsFn(ws)
    if err != nil {
        return fmt.Errorf("seed canonical cache: %w", err)
    }
    if refs == nil {
        refs = make(map[string][]artifactRef)
    }
    c.refs = refs
    return nil
}
```

The call site seeds before reading: `o.canonicalCache.ensureSeeded(ws)` then
`canonical = o.canonicalCache.refs`. Defense-in-depth: `record` also guards
against a nil map before assigning, so a zero-value cache that somehow reaches
`record` cannot panic.

## Prevention

- **Nil-vs-empty discipline for short-circuit caches**: any exported cache/memo
  that lets callers skip a correctness check must make its zero value mean
  "unseeded → re-scan", never "authoritative empty". Distinguish *not yet
  scanned* (`nil`) from *scanned, nothing found* (empty non-nil).
- **Prefer an unexported field + constructor, but assume the zero value will be
  built anyway.** Keeping the backing map unexported is necessary but not
  sufficient — `&T{}` is always reachable for an exported struct. Add a lazy
  `ensureSeeded`/`init`-on-first-use guard so correctness does not depend on
  callers using the constructor.
- **Make "scan exactly once" a test invariant, not a comment.** A package-level
  function seam (`scanCanonicalArtifactsFn`) that counts total scans lets a test
  assert: N creates with a shared cache ⇒ exactly 1 scan; N creates without a
  cache ⇒ N scans; and a zero-value cache ⇒ still 1 scan (proves the seeding
  guard fires).
- **Document the concurrency scope.** This cache is scoped to one sequential
  batch and is explicitly NOT concurrency-safe; the seeding guard does not add
  synchronization. Record that constraint at the type so a future concurrent
  bulk-create path builds its own cache or adds a lock.

## Evidence

- Shipped code at merge `b4c317e` (PR #154): `internal/core/canonical_cache.go`
  (`CanonicalCache`, `NewCanonicalCache`, `ensureSeeded`, `record`,
  `WithCanonicalCache`, `scanCanonicalArtifactsFn` seam),
  `internal/core/artifacts.go` (seed-before-lookup at the CreateArtifact guard),
  `internal/core/canonical_cache_test.go` (scan-count + zero-value-seeding tests).
- Wired bulk callers: `importMigrationItems` (migrate), `harvestStashEntryLocked`
  (priority harvest). Single interactive creates pass no cache and scan per call.
- Surfaced by Copilot review on PR #154 (zero-value `&core.CanonicalCache{}`
  bypass), fixed in-cycle and re-reviewed.
- Closure: `docs/closure/2026-06-29-070-S-internal-robustness-cluster-closure.md`.
- Runtime verification: `docs/closure/2026-06-29-070-S-internal-robustness-cluster-runtime-verification.md`.

## Applicability

Reuse for any exported optimization that lets callers skip a safety/correctness
step: validators with a precomputed allow-set, dedup caches, permission/ACL
memos, "already processed" sets. The rule generalizes the nil-vs-empty-map
distinction to a safety boundary — when the cheap path is also the unsafe path,
the zero value must fall back to the safe (re-scan) path.

## Related learnings

- `docs/compound/best-practices/empty-string-vs-sentinel-in-classification-2026-05-09.md`
  — the sibling discipline at the data layer: distinguish *absence* from a real
  value rather than collapsing them. Here the same "absence ≠ empty value"
  principle is applied to a cache's backing map (nil = not scanned, not = nothing
  exists). The 070.003-T empty-string-vs-absent-key parity in docline
  `ValidateFields` (same shipment) is the validation-layer expression of the same
  idea.
