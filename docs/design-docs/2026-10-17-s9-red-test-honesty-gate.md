---
chunk_strategy: h1-h2-h3
description: 'Design note for the S9 red-test honesty gate (143-S / 161-F): how new tests are identified, run against an immutable baseline, classified, and recorded as shared-contract red_green evidence for the S10/S11 consumers'
doc_type: design
docline:
    date: 2026-10-17T00:00:00Z
    status: accepted
    tags:
        - fault-line
        - red-test-honesty
        - evidence
        - faultline
---

# S9 Red-Test Honesty Gate

`internal/redhonesty` verifies that a claimed regression ("red") test is honest:
it must fail against the pre-fix baseline and pass at head. It is a library gate;
wiring it into shipment assembly or the S11 enforcement engine is a follow-up.

## Flow

1. **Resolve.** `BaseRef` and `HeadRef` are resolved to full commit SHAs. The base
   must be an ancestor of head and head must be the checked-out commit with no
   uncommitted or untracked Go inputs.
2. **Identify (U1).** Newly added top-level `Test*` functions are found by
   comparing base and head with `go/parser`; a test moved between files of one
   package is not new. `ChangedSymbols` diffs production declarations per package.
3. **Baseline (U1).** The base tree is exported with `git archive` into
   `.backlogit/runtime/redhonesty/base-*` (verified git-ignored and symlink-free
   before anything is created; no worktree is used). Only the head test files and
   `testdata` are overlaid; head deletions of tests are mirrored; production
   changes are never applied. `go test -json` runs the anchored `-run` selector.
4. **Classify (U2).** `Evaluate` rejects: `baseline_unverifiable` (build failure,
   missing test, skip), `already_passes_at_baseline`, `does_not_exercise_changed_code`,
   `asserts_only_sibling_fields`, `expected_output_via_changed_path`,
   `head_not_green`, `test_source_unanalyzable`. The first and last behavioural
   rules are exact; the structural rules are name-based heuristics (no type
   resolution) and are skipped when no production symbol changed.
5. **Record (U3).** `BuildArtifact` emits a `red_green` `EvidenceArtifact`
   (schema v2, node family `red_green`, applicability `internal`), validated by
   `faultline.Validate`. `Store` persists it at `<root>/<task>/<commit>.json`;
   `Load` re-validates and refuses an artifact not bound to the requested task and
   commit. Recorded evidence cannot be silently replaced.

## Schema (owned by `internal/faultline`)

- `EvidenceSchemaVersionV2 = 2` has the closed family set `{parity, red_green}`;
  v1 stays `{parity}` and is unchanged. `EvidenceView` now also carries
  `ProducingTask` and `ProducingCommit` so a family can bind its payload to the
  envelope.
- Payload: `base_sha`, `head_sha`, `tests[]` of `{package, test, baseline_result,
  head_result, honest, rejection_classes}`.
- Invariants: `head_sha == producing_commit`; `honest` implies
  `baseline=fail`, `head=pass`, no rejection classes; status `pass` requires every
  test honest, `fail` requires at least one dishonest test; `report_only` is not
  valid. Tests are unique per `(package, test)` and canonicalized in sorted order.

## Consumer obligations (S10 / S11)

Shape validity is not authenticity. Consumers must follow the S4 accept path:
authenticate the `Canonical()` bytes, verify producer identity against an
injected trust source, apply anti-replay keyed by the canonical digest, filter by
applicability, then apply the status policy (`pass` satisfies, `fail` blocks).

## Limits

- Only new top-level `Test*` functions are gated; subtests and table rows added
  to an existing test are not.
- A test that cannot build at the baseline is rejected as unverifiable; work that
  adds declarations is covered by source-shape harnesses (P-002.1).
- The gate runs repository test code as CI does; it scrubs likely credentials and
  pins the local toolchain but is not a sandbox. On timeout the run aborts with an
  error; orphaned grandchildren are bounded by `WaitDelay` but not tree-killed.
