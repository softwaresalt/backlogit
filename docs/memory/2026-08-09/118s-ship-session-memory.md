---
type: session-memory
session: 118-S Ship
timestamp: 2026-08-09T23:40:00Z
---

# Session Memory — 118-S F4 Durable Dependency Type Persistence

## Task IDs Completed

106.012-T (U1), 106.013-T (U2), 106.014-T (U3), 106.015-T (U4),
106.016-T (U5), 106.017-T (U6), 106.018-T (U7)

## Shipment

118-S → shipped; PR #335 merged as 39a3dbaf

## Key Decisions

- DependencyEdge{ID, Type} replaces []string for Artifact.Dependencies
- Serialization: all-blocks → bare strings; any non-default → typed objects
- toDependencyEdges normalizer handles both forms at load edge
- ErrInvalidDependencyType validated at load, not only at persist
- BackwardCompat: queries.go JSON fallback for legacy SQLite string arrays
- rewriteIDSlice in scripts/migrate-ids.go updated to handle typed objects

## Hard-Won Fixes

- gofmt -l shows pre-existing formatting issues across many files; only changed files need formatting
- WriteAllLines vs WriteAllText: ReadAllLines removes CRLF, WriteAllLines preserves them
- dependencyEdgeFromMap: use raw, present := fields["type"] pattern to detect absent vs non-string
- Test patterns: assert.Contains with []DependencyEdge → manual for-loop checks
- frontmatter_map_test.go is package models (not models_test) so no models. prefix needed
- artifact_expansion_test.go struct field is lowercase dependencies []models.DependencyEdge

## Next Steps

- 119-S (Formal gate F5) — not yet claimed
- Follow-up backlog: full CLI cobra command invocation in parity test
