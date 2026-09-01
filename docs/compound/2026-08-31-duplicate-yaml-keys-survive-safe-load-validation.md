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
schema validation, because `safe_load` cannot surface the problem.

The critical detail: override **`construct_mapping` only**, and check the
literal keys **before** merge expansion. Registering a plain function for
`DEFAULT_MAPPING_TAG` would replace PyYAML's generator-based
`construct_yaml_map` (breaking recursive anchors) and bypass
`SafeConstructor.construct_mapping` (breaking `<<` merge keys).

```python
import collections.abc
import yaml
from yaml.constructor import ConstructorError

MERGE_TAG = 'tag:yaml.org,2002:merge'


class StrictLoader(yaml.SafeLoader):
    """SafeLoader that refuses duplicate mapping keys.

    Overrides construct_mapping only, so construct_yaml_map stays
    registered (recursive anchors resolve) and super() still runs
    flatten_mapping (merge keys expand normally).
    """

    def construct_mapping(self, node, deep=False):
        seen = set()
        merge_seen = False
        for key_node, _value_node in node.value:
            if key_node.tag == MERGE_TAG:
                if merge_seen:
                    raise ConstructorError(
                        'while constructing a mapping', node.start_mark,
                        "duplicate merge key: '<<'", key_node.start_mark)
                merge_seen = True
                continue          # '<<' is not a data key
            key = self.construct_object(key_node, deep=deep)
            if not isinstance(key, collections.abc.Hashable):
                continue          # let super() raise its own error
            if key in seen:
                raise ConstructorError(
                    'while constructing a mapping', node.start_mark,
                    f'duplicate key: {key!r}', key_node.start_mark)
            seen.add(key)
        return super().construct_mapping(node, deep=deep)


data = yaml.load(text, Loader=StrictLoader)
```

### Why the check must precede merge expansion

`flatten_mapping` **prepends** merged pairs to `node.value` and leaves the
explicit pairs after them. That ordering is exactly how `<<` override works —
the later explicit key wins. So a mapping using a merge key legitimately
contains repeated entries for the same key *after* flattening.

Checking post-flatten therefore rejects valid YAML:

```yaml
base: &b {x: 1, y: 2}
derived:
  <<: *b
  y: 3        # legitimate override, NOT a duplicate
```

Checking the authored keys before expansion, and skipping the merge tag,
flags genuine duplicates while leaving overrides alone.

### Test coverage this requires

Any duplicate-rejecting loader should be pinned by tests before it is trusted:

| Case | Expected |
|---|---|
| Duplicate key, top level | raises |
| Duplicate key, nested mapping | raises |
| `<<` merge key with override | loads; parity with `safe_load` |
| `<<: [*a, *b]` multi-merge | loads, keys from both |
| Two authored `<<` keys in one mapping | raises |
| Recursive anchor (`self: *r`) | loads, self-reference preserved |
| Duplicate alongside a merge key | raises |
| Unhashable key | still raises PyYAML's own error |
| Real target document | loads; parity with `safe_load` |

The two merge-key rows are easy to conflate and pull in opposite directions.
`<<: [*a, *b]` is a *single* merge key whose value is a sequence — valid, and
it must still load. Two separate `<<:` entries in the same mapping are a
duplicate key and must be rejected. A loop that simply skips every merge-tagged
node satisfies the first case but silently accepts the second, so track whether
a merge key has already been seen rather than unconditionally continuing.

`ruamel.yaml` in round-trip mode raises `DuplicateKeyError` by default and is a
drop-in alternative when it is already a dependency — it needs no custom
subclass and carries none of the above pitfalls.

## Rule

* Treat `safe_load` success as evidence of *parseability*, never of
  *integrity*.
* Any script that writes YAML keys must guard each key it writes, and must
  distinguish "insert" from "amend".
* Validate machine-edited YAML with a duplicate-rejecting loader, and pin it
  with merge-key and recursive-anchor tests before trusting it — a naive
  implementation silently breaks valid YAML. Reach for a targeted regex scan
  only as a narrow, explicitly-labelled supplement when a loader is
  unavailable: a regex over an allowlist of keys cannot model nested mapping
  scope and will miss every key outside the list.

## Related

Checksums in `harness-manifest.yaml` are LF-normalized:
`sha256(raw.replace(b"\r\n", b"\n"))`. On a CRLF working copy, read with
`read_text` and write back with `newline=""` to avoid spurious drift.
