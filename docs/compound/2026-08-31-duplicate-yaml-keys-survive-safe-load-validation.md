---
chunk_strategy: h1-h2-h3
description: "Duplicate YAML keys are silently resolved last-wins by yaml.safe_load, so schema validation cannot detect them — scripted frontmatter edits need an explicit duplicate-key scan."
doc_type: learning
schema_version: "1.0"
source: docs/compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md
title: "YAML integrity: duplicate keys survive `safe_load` validation silently"
---

## Insight

`yaml.safe_load` resolves duplicate mapping keys **last-wins, without warning**.
Any validation built on top of it — schema checks, round-trip comparisons,
required-field assertions — will pass on a document that has silently discarded
data. Scripted edits to YAML frontmatter therefore need a **separate,
line-level duplicate-key scan**; parsing alone cannot catch this class.

## Context

A script updating `.autoharness/harness-manifest.yaml` injected a fresh
`drift_allowed` / `drift_reason` pair immediately after the `checksum:` line
for each touched artifact. It correctly skipped artifacts that already had
`drift_allowed`, but did **not** check for a pre-existing `drift_reason`.

The `graphtor-docs.instructions.md` entry already carried a `drift_reason`.
The result was two `drift_reason` keys in one mapping. `safe_load` kept the
last one — which was the *older* rationale — and the newly written explanation
was silently thrown away. Manifest validation passed. The defect was only
found by reading the raw file.

## The Broken Pattern

```python
# WRONG — only guards one of the two keys it writes
if not any(re.match(r'^\s*drift_allowed:', b) for b in block):
    insert_after_checksum('drift_allowed: true')
insert_after_checksum(f'drift_reason: "{reason}"')   # unconditional
```

## The Correct Pattern

Guard **every** key the script writes, and decide explicitly between injecting
and amending:

```python
had_allowed = any(re.match(r'^\s*drift_allowed:', b) for b in block)
had_reason  = any(re.match(r'^\s*drift_reason:',  b) for b in block)

if not had_allowed:
    insert_after_checksum('drift_allowed: true')
if not had_reason:
    insert_after_checksum(f'drift_reason: "{reason}"')
else:
    amend_existing_reason(reason)   # merge, never duplicate
```

## Required Validation Step

Add a line-level scan that counts keys per mapping. This must run *in addition
to* schema validation, because `safe_load` cannot surface the problem:

```python
for line in lines:
    if re.match(r'^\s*- path: ', line):
        seen.clear()                       # new artifact mapping
    for key in ('drift_allowed', 'drift_reason', 'checksum',
                'primitive', 'template', 'note'):
        if re.match(rf'^\s*{key}:', line):
            seen[key] += 1
            if seen[key] > 1:
                raise SystemExit(f'duplicate key: {key}')
```

## Rule

* Treat `safe_load` success as evidence of *parseability*, never of
  *integrity*.
* Any script that writes YAML keys must guard each key it writes, and must
  distinguish "insert" from "amend".
* Ship a duplicate-key scan alongside schema validation for any
  machine-edited YAML document.

## Related

Checksums in `harness-manifest.yaml` are LF-normalized:
`sha256(raw.replace(b"\r\n", b"\n"))`. On a CRLF working copy, read with
`read_text` and write back with `newline=""` to avoid spurious drift.
