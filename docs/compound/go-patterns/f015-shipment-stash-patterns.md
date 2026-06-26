---
chunk_strategy: h1-h2-h3
description: Hard-won Go, SQLite, stash migration, and regression-test patterns learned while building F015 in backlogit.
doc_type: learning
docline:
    category: best_practice
    component: migrations
    date: 2026-04-06T00:00:00Z
    file_path: internal/stash/jsonl.go
    message: F015 exposed repeatable patterns for tab-indented edits, SQLite JSON decoding, stash migration, rehydration counts, and default-type regressions.
    problem_type: best_practice
    resolution_type: documentation
    resolved: true
    root_cause: schema_mismatch
    severity: medium
    tags:
        - backlogit
        - f015
        - shipment
        - stash
        - sqlite
        - rehydration
        - migration
        - go-patterns
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/go-patterns/f015-shipment-stash-patterns.md
title: F015 shipment and stash patterns from the two-agent workflow refactor
---

## TL;DR summary

* Treat `internal/config/defaults.go` as a script-edit target, not a direct edit target, because tab-indented files break exact-whitespace patching.
* Treat SQLite JSON arrays as lossy on the way back out. Normalize `[]interface{}` into `[]string` every time you read shipment `CustomFields`.
* Treat stash migration as a dual-format problem. Read checkbox markdown and YAML-list markdown, then keep the richer parse result.
* Treat stash rollout as a dual-reader problem. Read `.stash.md` and `stash.jsonl`, merge by ID, and prefer JSONL on collisions.
* Treat `db.Rehydrate` counts as user-visible behavior. If stash indexing adds records, add that count to the returned total.
* Treat new default templates as count regressions waiting to happen. Grep for hardcoded expected counts after every new default type.

## Problem

F015 introduced shipment artifacts, stash migration behavior, and a two-agent
workflow that touched config defaults, SQLite-backed rehydration, and migration
tests. The implementation worked only after several non-obvious issues were
isolated and turned into repeatable patterns.

The key lesson is that these were not isolated bugs. They were interface
mismatches between tools, storage formats, and test assumptions.

## Symptoms

Common failure modes during F015 looked like this:

* edits against `internal/config/defaults.go` failed because patch tooling could
  not match tab-indented lines reliably
* shipment item lists came back from SQLite as `[]interface{}` even though the
  code stored `[]string`
* stash migration tests passed YAML-list stash content that
  `stash.ParseContent()` could not read
* `db.Rehydrate` returned `0` for workspaces with stash content only
* tests that hardcoded template counts failed after the shipment template was
  added
* gradual stash rollout risked losing entries if only one stash format was read

## What did not work

These approaches were unreliable:

* assuming the edit tool would survive mixed indentation in `defaults.go`
* assuming `json.Unmarshal` through SQLite would round-trip back into the same
  concrete Go slice type
* assuming one stash parser could handle both checkbox markdown and YAML-list
  markdown bodies
* assuming stash indexing did not need to affect the `Rehydrate` return value
* updating only the obvious tests after adding a new default artifact type
* reading only `.stash.md` or only `stash.jsonl` during migration rollout

## Solution

### Tab-indented Go files: use scripted replacement

`internal/config/defaults.go` is tab-indented throughout. Exact-match editing is
fragile there. The reliable pattern is:

1. read the file into a Python script
2. replace an exact old block with a new block
3. write the file back
4. inspect the file before and after the change

```python
from pathlib import Path

path = Path(r"D:\Source\GitHub\backlogit\internal\config\defaults.go")
before = path.read_text(encoding="utf-8")

old = "\tif err := writeFileIfNotExists(filepath.Join(queueDir, \".stash.md\"), []byte(stash.DefaultContent())); err != nil {\n\t\treturn fmt.Errorf(\"write .stash.md: %w\", err)\n\t}\n"
new = "\tif err := writeFileIfNotExists(filepath.Join(queueDir, \".stash.md\"), []byte(stash.DefaultContent())); err != nil {\n\t\treturn fmt.Errorf(\"write .stash.md: %w\", err)\n\t}\n"

if old not in before:
    raise SystemExit("target block not found")

path.write_text(before.replace(old, new, 1), encoding="utf-8")
```

Run it with `python script.py`, then verify the file contents. Do not trust the
edit until you inspect the result.

### SQLite JSON round-trip: normalize slice types on read

Shipment items are written into `artifact.CustomFields["items"]`, but SQLite
JSON round-trips do not preserve the original concrete slice type. The safe read
pattern is in `internal/core/shipment.go`.

```go
raw, _ := artifact.CustomFields["items"]
switch v := raw.(type) {
case []string:
	return v
case []interface{}:
	result := make([]string, 0, len(v))
	for _, item := range v {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
```

The repository implementation uses the same idea with `[]any`:

```go
switch items := raw.(type) {
case []string:
	return append([]string(nil), items...)
case []any:
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok {
			normalized = append(normalized, value)
		}
	}
	return normalized
default:
	return []string{}
}
```

Always normalize at the read edge. Do not assume the stored type survived.

### Stash body parsing: support checkbox and YAML-list formats

`internal/stash/stash.go` parses checkbox entries such as:

```text
- [ ] [AB12CD34] [priority:high] task: ship the feature
```

The migration test path also used YAML-list bodies:

```yaml
- id: AB12CD34
  priority: high
  kind: task
  text: ship the feature
```

`ParseContent()` only handled the checkbox form. The fix was to add
`parseStashMDAsYAML()` in `internal/stash/jsonl.go` and try both parsers during
migration. Keep whichever parser yields more entries.

```go
_, regexEntries, regexErr := ParseFile(srcPath)
yamlEntries, yamlErr := parseStashMDAsYAML(srcPath)

var entries []Entry
if regexErr == nil && len(regexEntries) >= len(yamlEntries) {
	entries = regexEntries
} else if yamlErr == nil {
	entries = yamlEntries
} else if regexErr == nil {
	entries = regexEntries
} else {
	return 0, fmt.Errorf("parse stash.md: %w", regexErr)
}
```

This keeps migration tolerant without forcing production parsing to depend on a
single stash body shape.

### Rehydrate count semantics: include stash-only workspaces

`internal/db/rehydration.go` documents `Rehydrate` as returning the number of
artifacts successfully indexed. Once stash indexing was added, that count needed
to include stash records too. Otherwise a workspace with only stash content
looked empty to callers and tests asserting `count > 0` failed.

Use this rule: if rehydration persists stash records into the index, their count
belongs in the returned total.

### Default template regressions: grep for hardcoded counts

Adding the shipment template changed default-type totals. F015 surfaced the
usual failure sites:

* `internal/config/defaults_templates_test.go`
* `internal/cli/migrate_import_test.go`

Examples from the updated tests:

```go
assert.Len(t, templates, 6)
```

```go
assert.Len(t, markdownFiles, 7, "expected config templates plus exactly one imported artifact markdown file")
```

After introducing a new default type, grep for hardcoded counts and update every
assertion that assumed the old baseline.

### Stash rollout: read both formats and deduplicate by ID

Gradual migration from `.stash.md` to `stash.jsonl` needs a dual-reader. The
reader should:

1. load legacy markdown entries
2. load JSONL entries
3. merge by ID
4. prefer JSONL when the same ID exists in both places

That pattern preserves data during rollout and lets older workspaces coexist
with newer ones.

```go
seen := make(map[string]struct{}, len(activeEntries))
for _, e := range activeEntries {
	seen[e.ID] = struct{}{}
}
for _, e := range jsonlEntries {
	if _, ok := seen[e.ID]; !ok {
		activeEntries = append(activeEntries, e)
		seen[e.ID] = struct{}{}
	}
}
```

If you revise this logic, keep the deduplication key stable and document the
winner on collisions. For rollout safety, JSONL should be treated as the newer
source of truth.

## Why this works

These patterns work because each one repairs the boundary where assumptions were
wrong:

* scripted edits bypass fragile whitespace matching
* slice normalization converts storage-decoded data into the shape the domain
  layer expects
* dual stash parsing accepts both the test fixture format and the production
  format
* rehydration counts stay aligned with what the index actually contains
* regression grep catches hidden count assumptions outside the file you changed
* dual-reader stash rollout avoids data loss during format migration

## Gotchas

* Do not assume `[]interface{}` means corruption. It is the normal JSON decode
  shape unless you normalize it.
* Do not change stash parsing in only one place. Migration and runtime readers
  have different responsibilities.
* Do not forget count assertions outside config tests. Import and migration tests
  often encode the same assumptions indirectly.
* Do not trust a scripted replacement without verifying the exact file content
  afterward.
* Do not silently pick an older stash entry when IDs collide. Make the
  precedence rule explicit.

## Prevention

Use these guardrails for future workflow and migration work:

* write Python replacement scripts for tab-indented Go files and verify the
  result after execution
* normalize `CustomFields` values at every read boundary where JSON-decoded data
  re-enters typed Go logic
* keep stash migration tolerant of both checkbox markdown and YAML-list bodies
* treat `Rehydrate` return counts as contract behavior and update them whenever
  new indexed record types are added
* grep for hardcoded expected counts after adding defaults, templates, or other
  generated artifacts
* keep dual-format readers in place until all supported workspaces can be safely
  migrated

## Related solutions

* [`docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md`](../workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md)
  explains the broader two-agent workflow boundary decisions.
* [`internal/stash/jsonl.go`](../../../internal/stash/jsonl.go) captures the
  dual-parser migration logic discussed here.
* [`internal/db/rehydration.go`](../../../internal/db/rehydration.go) contains
  the stash rehydration and merge path.
* [`internal/core/shipment.go`](../../../internal/core/shipment.go) contains the
  shipment item normalization pattern for `CustomFields["items"]`.
