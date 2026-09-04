---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Two-%w discriminator pattern: adding a sentinel to an existing error wrap so a shared policy can classify errors from multiple producers"
source: docs/compound/2026-09-04-two-percent-w-discriminator-for-shared-error-policy.md
doc_type: learning
description: "When a shared policy function classifies errors with errors.Is(err, discriminator), EVERY producer that generates that class of error must wrap the discriminator sentinel. Go 1.20+ fmt.Errorf supports two %w verbs: fmt.Errorf('prefix: %w: %w', sentinel, cause) wraps both the sentinel and the original cause, making the error detectable by errors.Is(err, sentinel) while preserving the YAML/IO cause for diagnostics. Missing this in even one producer causes silent misclassification: the policy sees an unrecognized error, falls back to its fail-closed branch, and the scan aborts where it should continue. Verified in 136-S (decodeDoc and Normalize both needed the two-%w pattern for classifyDecodeFailure to correctly route frontmatter errors)."
docline:
    date: 2026-09-04T00:00:00Z
    severity: medium
    tags:
        - go
        - errors
        - fmt-errorf
        - two-w-wrap
        - error-classification
        - shared-policy
        - docline
        - decode-error
        - 136-S
---

# Two-`%w` discriminator: adding a sentinel to an existing error wrap for a shared policy

## Context

`classifyDecodeFailure` in `internal/docline/service.go` classifies decode errors
policy-neutrally using `errors.Is(err, ErrFrontmatterDecode)`. It was written
for `decodeDoc`, which already used the two-`%w` pattern:

```go
return nil, fmt.Errorf("docline.decodeDoc: decode %s: %w: %w", rel, ErrFrontmatterDecode, err)
```

When U2c needed `PlanMigration` to reuse the same policy (via
`applyDecodeFailure`), `Normalize` was its error source. But `Normalize`
wrapped decode errors as a single `%w`:

```go
return nil, fmt.Errorf("docline.Normalize: decode %s: %w", relPath, err)
```

Without the `ErrFrontmatterDecode` discriminator, `classifyDecodeFailure`
fell through to `decodeFailureRead` (the fail-closed default) and returned
a fatal error — aborting the scan instead of recording a finding and
continuing.

## The bug

The policy function is policy-neutral, but it has an implicit contract: every
error it classifies must carry the sentinel it tests for. A policy reused across
producers silently misclassifies if any producer omits the discriminator.

`errors.Is` unwraps through the error chain. A single `%w` wraps the cause but
does **not** introduce a sentinel unless the cause **is** the sentinel. The two-
`%w` pattern is required to wrap an additional sentinel alongside the cause:

```go
// Single %w — cause traversable but no sentinel:
fmt.Errorf("prefix: %w", cause)           // errors.Is(err, sentinel) = false

// Two %w (Go >= 1.20) — both the sentinel and the cause are wrapped:
fmt.Errorf("prefix: %w: %w", sentinel, cause)  // errors.Is(err, sentinel) = true
                                                // errors.As(err, &yamlErr) = true
```

## The fix

Add the `ErrFrontmatterDecode` discriminator to `Normalize`'s decode-error wrap:

```go
// Before:
return nil, fmt.Errorf("docline.Normalize: decode %s: %w", relPath, err)

// After:
return nil, fmt.Errorf("docline.Normalize: decode %s: %w: %w", relPath, ErrFrontmatterDecode, err)
```

This is purely additive — the original YAML cause (`err`) is still the second
`%w` and remains reachable via `errors.As`. Go's multi-error unwrap tree
preserves both wrapped values, so callers can still extract the underlying
YAML cause (e.g. for diagnostics) while the new sentinel is now testable with
`errors.Is`. Verified: the gen-docs caller of `Normalize`
does not test on the YAML cause type.

## Generalized rule

> **When a policy function classifies errors from multiple producers using
> `errors.Is(err, discriminator)`, every producer must wrap the discriminator
> with `fmt.Errorf("...: %w: %w", discriminator, cause)` (Go 1.20+ two-`%w`).**

The failure mode is silent: the policy falls back to its default branch (usually
fail-closed or fatal), and the misclassified error surface appears to be a
different kind of failure. Misclassification typically surfaces as "unexpected
fatal abort" rather than "wrong sentinel," making the root cause hard to trace.

## When to use this pattern

1. A policy function classifies errors with `errors.Is`.
2. A second producer generates the same class of error via a different code path.
3. The second producer's existing error wrap is a single `%w` (no sentinel).
4. Adding the sentinel is purely additive — the existing cause chain is preserved.

Do NOT use if the second producer's error wrap is already a sentinel itself (e.g.
`ErrPathEscapesWorkspace` is already the terminal value — no secondary cause to
preserve).

## Test coverage

`TestU2cNormalize_DecodeErrorWrapsErrFrontmatterDecode` in
`internal/docline/154_004_decode_policy_test.go` pins this:

```go
_, err := Normalize("docs/decisions/broken.md", raw, NormalizeOptions{...})
assert.True(t, errors.Is(err, ErrFrontmatterDecode), "...")
```

Adding a new error producer that should use an existing policy class should
always include a `errors.Is(err, discriminator)` test on the producer's own
output — independent of the policy integration test.

## Relation to existing compound

The `classifyDecodeFailure` comment anticipated future consumers
(`"reusable outside LintTree's own policy"`), but did not document the
obligation for each producer to wrap the discriminator. This learning fills that
gap.

Refs: 136-S / 154.004-T (U2c), `internal/docline/normalize.go`,
`internal/docline/service.go`.
