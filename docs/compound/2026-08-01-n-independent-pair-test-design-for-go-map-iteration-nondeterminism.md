---
chunk_strategy: h1-h2-h3
description: "Go's map iteration order is randomized per-process, so a regression test for an ordering-dependent bug over a map keyed by only ONE independent pair (e.g. one parent/child pair) has only ~50% probability of failing on a buggy implementation on any given run -- it can pass-by-luck and give false confidence, and CI could flake between red and green across unrelated commits. Discovered fixing internal/core/shipment_lifecycle.go's restoreRolledUpNonMemberFeatures (feature 133-F / Thread C, PR #327): the fix restores multiple non-member covering features from a map, and an unordered range over that map let a child's restore silently overwrite an already-restored parent's status when the child happened to be visited after the parent. The reliable regression-test technique is an N-independent-pair design: construct N structurally-independent parent/child (or sibling) pairs in the SAME test's map/slice so that map iteration must, by the pigeonhole principle, visit at least one pair in the wrong relative order with near-certainty (1 - 0.5^N), then assert order-dependent correctness across ALL pairs simultaneously in one test run. With N=8 pairs the pre-fix implementation failed 5/8 pairs in a single run, decisively proving the bug and, after the fix, the same test passed deterministically across repeated runs -- because a correct depth-first/topological sort has no order sensitivity left to expose."
doc_type: learning
docline:
    date: 2026-08-01T00:00:00Z
    severity: medium
    tags:
        - testing
        - go
        - map-iteration
        - non-determinism
        - tdd
        - regression-test-design
        - flaky-tests
        - core
        - shipment
schema_version: "1.0"
source: docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md
title: "N-independent-pair test design reliably exposes Go map-iteration-order bugs that single-pair tests only catch ~50% of the time"
---

# N-Independent-Pair Test Design for Go Map-Iteration-Order Bugs

## Context

Surfaced fixing a Copilot review finding (Thread C) on PR #327
(`internal/core/shipment_lifecycle.go`, feature 133-F, shipment 115-S).
`restoreRolledUpNonMemberFeatures` iterates a
`map[string]featureStatusSnapshot` of non-member covering features to revert
to their pre-ship status. Go deliberately randomizes map iteration order per
process/run. `setArtifactStatus`'s status write unconditionally cascades to the
parent (`cascadePersistedParentStatuses`), so if a child feature's restore is
processed *after* its parent's restore in a given run, the child's cascade can
silently recompute and overwrite the parent's just-restored status —
non-deterministically, only on runs where the map happened to yield that
order.

## The Trap: Single-Pair Tests Under-Detect Ordering Bugs

A regression test asserting "parent status == expected AND child status ==
expected" over a map containing exactly **one** parent/child pair has, for an
unordered-range bug, only about a **50% chance per run** of iterating in the
"bad" order (child-before-parent is fine; parent-before-child exposes the
bug — or vice versa, depending on which direction is unsafe). Such a test:

* can pass on the very run where you are checking whether your fix works,
  even against the **unfixed** code — a false-negative RED-phase check.
* can flake intermittently in CI long after merge, on totally unrelated
  commits, because Go's map seed differs per process — eroding trust in the
  test suite ("it's just flaky, re-run it") rather than surfacing the real
  bug.
* gives no usable signal for *how much* confidence a passing run actually
  provides.

## The Fix: N Structurally-Independent Pairs in One Test

Construct **N** independent parent/child (or sibling) pairs — independent
meaning no pair's restore correctness depends on another pair's order — inside
the **same** map/snapshot that the code under test ranges over, then assert
order-dependent correctness for **all N pairs simultaneously** in a single test
run:

* By the pigeonhole/binomial argument, the probability that **every single
  one** of N independent pairs happens to iterate in the safe order in a given
  run is `0.5^N`. For N=8, that is `~0.4%` — i.e. the test fails on a buggy
  implementation with **~99.6%** probability per run, versus ~50% for a
  single-pair test.
* This repository's test,
  `TestRestoreRolledUpNonMemberFeatures_RestoresDeepestFirstRegardlessOfMapOrder`
  (`internal/core/shipment_test.go`), used 8 independent pairs. Confirmed RED:
  5 of 8 pairs failed on the pre-fix implementation in a single run — well
  above the noise floor, decisively proving the bug rather than relying on a
  lucky/unlucky single sample. After the fix (sort snapshot keys deepest-first
  via `depthSortedIDs` before iterating, instead of raw `range`), the same test
  passed on **3 consecutive runs** — expected, because a correct
  deepest-first/topological order has no iteration-order sensitivity left for
  Go's map randomization to expose.
* N does not need to be large to be effective — even N=5 already yields
  `0.5^5 ≈ 3%` failure-miss probability, likely acceptable for most cases; the
  choice of N should scale with how costly a missed regression would be
  (production data corruption vs. a cosmetic ordering nit).

## Rule

When testing code that ranges over a Go map (or any data structure whose
iteration order is intentionally unspecified) and the correctness of the
operation depends on visiting entries in a particular relative order:

1. Do not rely on a single-pair/single-entry test to catch an
   order-dependence bug — it is a coin flip, not a proof.
2. Construct N ≥ 5–8 structurally-independent order-sensitive pairs inside the
   *same* collection under test, and assert correctness across all of them in
   one run. Prefer higher N for higher-stakes correctness properties.
3. Confirm true RED before the fix (not just "the test fails once" — ideally
   show most/all pairs failing, not a marginal 1-of-N, to rule out an
   unrelated flake) and true GREEN after, across multiple runs, before trusting
   the fix.
4. When the underlying fix is "iterate in a specific order" (e.g.
   depth-first/topological sort before ranging), the test's own reliability
   check is that a correctly-fixed implementation has **zero** iteration-order
   sensitivity left — so once fixed, repeated runs should never flake, by
   construction.

## Applicability

Any Go code (or other language with intentionally randomized/unspecified
iteration order) where a map, set, or similarly unordered collection is
iterated and the correctness of a mutation, cascade, or side-effecting
operation depends on visiting related entries in a specific relative order —
parent/child restoration, dependency resolution, batch processing with
cross-entry effects, or any "last write wins" cascade over a shared ancestor.
