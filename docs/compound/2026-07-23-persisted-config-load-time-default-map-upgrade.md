---
title: "Adding a new default map entry needs a load-time upgrade path for persisted configs"
source: docs/compound/2026-07-23-persisted-config-load-time-default-map-upgrade.md
doc_type: learning
description: "When a validated default map (e.g. lifecycle status transitions) gains a new entry, workspaces whose config OMITS the map inherit the new default via the empty-map fallback, but workspaces that already PERSISTED an explicit map do not. Ship a load-time normalizer that deep-equals the persisted map against a frozen copy of the PRIOR default and upgrades ONLY on exact match, leaving customized maps untouched. Guard the two default-definition sites with a reflect.DeepEqual sync test."
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
    date: 2026-07-23T00:00:00Z
    severity: medium
    tags:
        - config
        - loader
        - normalization
        - defaults
        - lifecycle-transitions
        - backward-compat
        - deep-equal
        - drift-test
---

# Persisted-Config Load-Time Default Map Upgrade

## Context

Graduated from shipment 104-S / feature 124-F / tasks 124.002-T, 124.003-T,
124.004-T (PR #294 fix, merged `96664088`; PR #295 closure, merged `369e862a`).
The BD8DBB85 stash reported that backlogit's state machine only allowed
`blocked → active` with no path to `queued`, contradicting its own doctor doc.
Option A added `blocked → queued` AND `active → queued` to the validated
status-transition map.

## Problem

The status-transition map has TWO definition sites that must stay identical:

* `internal/config/defaults.go` — `DefaultHooksConfig().Lifecycle.Transitions`
  (production-wired via `internal/core/workspace.go`)
* `internal/hooks/builtin_pre.go` — `DefaultTransitions()` (empty-map fallback
  used by `ValidateStatusTransition(nil)`)

Adding the new transitions to both sites fixed *new* and *transitions-absent*
workspaces immediately: a workspace whose persisted hooks config omits the
transitions map gets the fresh default through the empty-map fallback. But a
workspace that had already persisted an EXPLICIT transitions map (the old
generated default) would keep the stale map on load — `LoadHooks` historically
normalized only the `PreTaskCompletionGate` block, never the transitions map.
Those workspaces would silently never gain the new `→ queued` edges.

## Root Cause

New defaults only reach configs that fall through to the default. Any value a
user (or a prior generator) already wrote to disk shadows the new default
forever unless load-time normalization actively upgrades it. There was no
upgrade path for the transitions map, so the fix was invisible to already-
initialized workspaces.

## Resolution

Task 124.004-T added a load-time normalizer in `internal/config/loader.go`:

* `priorGeneratedDefaultTransitions` — a frozen package-level `var` snapshot of
  the exact prior generated default map (Go maps cannot be `const`, so this is a
  never-mutated `var`). It must never change again; it is the fingerprint used to
  recognize an un-customized legacy config.
* `upgradeLegacyTransitions` — called from `LoadHooks`. It `reflect.DeepEqual`s
  the persisted transitions map against the frozen prior default. On an EXACT
  match it replaces the map with the current default (upgrade). On any
  difference it leaves the map untouched (the user customized it — preserve).

This mirrors the established `PreTaskCompletionGate.Normalize()` pattern from
082-F. A `reflect.DeepEqual` sync test
(`internal/config/transitions_sync_test.go`) locks the two default-definition
sites together so they cannot drift.

### Deep-equal ambiguity (accepted)

`reflect.DeepEqual` matches map VALUES, not provenance. It compares decoded map
and slice values, so a persisted map that is merely value-identical to the prior
default (even with different YAML formatting or key ordering) matches. A user who
happened to customize their map to be value-identical to the prior default is
indistinguishable from an un-customized legacy config and will be upgraded. This
was accepted as a low-probability, low-harm ambiguity rather than adding a
provenance marker.

## Prevention

* When adding an entry to any validated default map, ask: "Do already-
  initialized workspaces persist this map explicitly?" If yes, editing the
  default definition is NOT enough — ship a load-time upgrade path.
* Freeze the prior default as an immutable historical snapshot (a package-level
  `var`, since Go maps cannot be `const`) so the upgrade can recognize an
  un-customized legacy config by exact deep-equal match; upgrade only on exact
  match, preserve everything else.
* Guard multi-site default definitions with a `reflect.DeepEqual` sync test so
  the fallback site and the production-wired site cannot silently diverge.
* Absent-map workspaces need no upgrade — they already inherit the new default
  through the empty-map fallback. Only PERSISTED explicit maps need the load-
  time normalizer.
