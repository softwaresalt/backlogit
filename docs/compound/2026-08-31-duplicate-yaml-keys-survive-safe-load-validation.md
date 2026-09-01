---
chunk_strategy: h1-h2-h3
description: "yaml.safe_load resolves duplicate mapping keys last-wins without warning, so schema validation cannot detect them — use a duplicate-rejecting loader for machine-edited YAML."
doc_type: learning
schema_version: "1.0"
source: docs/compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md
title: "YAML integrity: duplicate keys survive `safe_load` validation silently"
---

## Insight

`yaml.safe_load` resolves duplicate mapping keys **last-wins, without warning**.
Any validation built on top of it — schema checks, round-trip comparisons,
required-field assertions — will pass on a document that has silently discarded
data.

The generalizable fix is a **duplicate-rejecting loader**, not a hand-rolled
scan. A loader hooks the mapping constructor, so it catches duplicates of every
key, at every nesting depth, in one pass.

## Context

A script updating `.autoharness/harness-manifest.yaml` injected a fresh
`drift_allowed` / `drift_reason` pair immediately after the `checksum:` line
for each touched artifact. It correctly skipped artifacts that already had
`drift_allowed`, but did **not** check for a pre-existing `drift_reason`.

The `graphtor-docs.instructions.md` entry already carried a `drift_reason`.
The result was two `drift_reason` keys in one mapping. `safe_load` kept the
last one — which was the *older* rationale — and the newly written explanation
was silently thrown away. Manifest validation passed. The defect was only found
by reading the raw file.

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

Validate with a loader that rejects duplicate keys. This runs *in addition to*
schema validation, because `safe_load` cannot surface the problem. Unlike a
regex scan, it covers every key at every nesting level:

```python
import yaml
from yaml.constructor import ConstructorError
from yaml.resolver import BaseResolver


class StrictLoader(yaml.SafeLoader):
    """SafeLoader that refuses duplicate mapping keys."""


def _reject_duplicates(loader, node, deep=False):
    mapping = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if key in mapping:
            raise ConstructorError(
                'while constructing a mapping', node.start_mark,
                f'duplicate key: {key!r}', key_node.start_mark)
        mapping[key] = loader.construct_object(value_node, deep=deep)
    return mapping


StrictLoader.add_constructor(
    BaseResolver.DEFAULT_MAPPING_TAG, _reject_duplicates)

# Raises ConstructorError with line/column on ANY duplicate, at any depth.
data = yaml.load(text, Loader=StrictLoader)
```

`ruamel.yaml` in round-trip mode raises `DuplicateKeyError` by default and is a
drop-in alternative when it is already a dependency.

## Rule

* Treat `safe_load` success as evidence of *parseability*, never of
  *integrity*.
* Any script that writes YAML keys must guard each key it writes, and must
  distinguish "insert" from "amend".
* Validate machine-edited YAML with a duplicate-rejecting loader. Reach for a
  targeted regex scan only as a narrow, explicitly-labelled supplement when a
  loader is unavailable — a regex over an allowlist of keys cannot model nested
  mapping scope and will miss every key outside the list.

## Related

Checksums in `harness-manifest.yaml` are LF-normalized:
`sha256(raw.replace(b"\r\n", b"\n"))`. On a CRLF working copy, read with
`read_text` and write back with `newline=""` to avoid spurious drift.
