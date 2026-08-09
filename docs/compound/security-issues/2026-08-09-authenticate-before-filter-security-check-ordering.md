---
chunk_strategy: h1-h2-h3
description: 'A security-check-ordering pattern discovered during 117-S formal-gate-evidence review (round 5->6 regression): a type-based relevance filter applied BEFORE cryptographic authentication let an attacker hide a tampered, retained log record from detection simply by making it fail the filter, silently erasing it from a security-critical floor/threshold computation. The fix is structural: authenticate every candidate FIRST, then decide relevance -- never the reverse.'
doc_type: learning
docline:
    date: 2026-08-09T00:00:00Z
    severity: high
    tags:
        - security
        - authentication
        - ordering
        - review
        - core
        - anti-replay
schema_version: "1.0"
source: docs/compound/security-issues/2026-08-09-authenticate-before-filter-security-check-ordering.md
title: 'Authenticate before filtering, never after: a relevance filter placed before crypto verification creates an invisible bypass'
---

# Authenticate Before Filtering, Never After

Graduated from shipment 117-S (Formal Gate F1 — evidence authenticity and
manifest binding; feature 106-F, tasks 106.003-T through 106.011-T; PR #333,
merge `23d88904faf917a4f4003042f185de9b4e568530`). Discovered as a Copilot
review-round-6 finding on a fix I had ALREADY shipped in review round 5 for a
different, legitimate concern — i.e., the fix for one bug introduced a new one.

## Rule — When a security check has both a "does this even apply" filter and an "is this authentic" verification, the verification MUST run first

### Problem

`internal/gateevidence/formal.go`'s `maxOtherCounter` computes the anti-replay
counter floor a candidate proof must exceed, scanning every OTHER event in an
item's log. Round 5 fixed a real bug (a later, legitimately-signed
`EventGateForced` was poisoning the floor and making every genuine pass
followed by any force permanently unadmissible) by adding:

```go
for i := range evs {
    if i == excludeIdx { continue }
    if evs[i].EventType != EventGatePassed { continue } // <-- filter FIRST
    ...
    env, macHex, err := envelopeFromEvent(evs[i], ctx)   // authenticate SECOND
    if verifyErr := gateproof.Verify(env, macHex, ctx.Key); verifyErr != nil { ... }
}
```

This reintroduced a WORSE bug: an actor who takes an existing, genuinely-signed,
HIGH-counter `EventGatePassed` record and edits ONLY its top-level `EventType`
field (no re-signing needed, since the filter runs before any MAC check) gets
that event silently `continue`d out of consideration entirely — its counter
never counted toward the floor, exactly as if it had been deleted. A lower-
counter, previously-superseded, but validly-signed candidate can then pass the
now-deflated floor check and be admitted as if it were the newest evidence —
a same-log-tampering bypass of the anti-replay guarantee.

### Fix

Move the type filter to AFTER authentication, so it only ever excludes an
event from the numeric maximum once its MAC has already been verified against
the REAL, tamper-evident envelope (`EventType` is bound inside the envelope,
so relabeling it is caught by `gateproof.Verify` before the filter is ever
consulted):

```go
for i := range evs {
    if i == excludeIdx { continue }
    if _, hasCounter := evs[i].Delta["counter"]; !hasCounter { continue }
    env, macHex, err := envelopeFromEvent(evs[i], ctx)
    if err != nil { return 0, false, fmt.Errorf(...) }          // authenticate FIRST
    if verifyErr := gateproof.Verify(env, macHex, ctx.Key); verifyErr != nil {
        return 0, false, fmt.Errorf(...)                        // fail closed on any tamper
    }
    if evs[i].EventType != EventGatePassed { continue }          // filter SECOND
    ...
}
```

A relabeled event's MAC now fails to verify (since `EventType` is bound inside
the signed envelope), and `maxOtherCounter` returns an error, causing the
WHOLE admission to refuse — fail closed, exactly like the existing in-place
counter-tamper regression test this fix sits beside.

### Why the "cheap check first" instinct is backwards here

Ordinary code review wisdom says "do the cheap check first, skip expensive
work when it doesn't apply" — and that is correct for PERFORMANCE filters.
It is actively dangerous for SECURITY filters whenever the filter itself
reads a field that a signature is supposed to protect: an attacker who wants
to make something "not apply" can simply tamper the field the filter reads,
and if authentication never runs, the tamper is completely invisible. The
general test: **if a filtered-out record could otherwise have influenced a
security decision (a floor, a threshold, a count, an inclusion list), verify
its integrity before you decide it doesn't matter.**

### Test technique that caught it

`TestFormalAdmit_TamperedOtherEventTypeRefused`: construct one genuinely
signed, high-counter event; mutate ONLY its `EventType` field in place
(mirroring exactly how the sibling `TestFormalAdmit_TamperedOtherEventCounterRefused`
mutates only the `counter` Delta field); pair it with a second, validly-signed
but lower-counter (replayed) candidate. Before the fix, the tampered event was
silently dropped and the replayed candidate was wrongly admitted. This is the
general shape for testing "does my filter accidentally trust an unverified
field" — tamper the ONE field the filter reads, leave everything else intact
and validly signed, and confirm the whole check still fails closed.

## Related

- `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md` — a
  sibling fail-closed exit-code discipline for a different verification path
  in the same feature area (gate lineage checks).
- PR #333 review round 5 (the original, legitimate fix this regressed) and
  round 6 (the finding that caught the regression) — both Copilot
  pull-request-review cycles on the same shipment.
