---
chunk_strategy: h1-h2-h3
description: 'A reliability rule graduated from 084-S — a git helper that is "bounded" by a caller-configured timeout can still be a DoS if that timeout is sized for the wrong workload. The gate broker''s GateBroker.TimeoutSeconds (default 600s) is sized for build/test gate COMMANDS; adopting it verbatim for near-instant local git metadata reads (merge-base --is-ancestor, rev-parse HEAD) on the lock-holding shipment-ship path lets a hung git pin the workspace lock for ten minutes. Fast metadata helpers must derive their OWN hard-cap deadline (5s), honoring a smaller configured value but never a larger one.'
doc_type: learning
docline:
    date: 2026-07-06T00:00:00Z
    severity: high
    tags:
        - reliability
        - dos
        - timeout
        - exec
        - context
        - locking
        - gate
        - git
        - core
schema_version: "1.0"
source: docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md
title: 'A bounded helper still needs a HARD CAP: do not adopt a command-sized timeout for near-instant metadata reads on a lock-holding path (084-S)'
---

# Bounded Is Not Enough — Cap the Helper Timeout

Graduated from shipment 084-S (feature 084-F, PR #182, merge
`f49ce3c37b460afce81591ca6e354b8de3a14a17`). Surfaced by Copilot review on the
feature PR (3 threads) and remediated before merge in commit `c29b189`. Extends the
082-S "timeout the probe" lesson with a second, subtler failure mode.

## Rule — Size the deadline to the workload, not to a convenient existing config knob

### Problem

The ancestor-aware fix added two git helpers on the `shipment ship` path —
`isAncestor` (`git merge-base --is-ancestor`) and `headSHABounded`
(`git rev-parse HEAD`). Both were correctly wrapped in `context.WithTimeout`, so
they looked "bounded". But the first draft derived the deadline from
`GateBroker.TimeoutSeconds`, whose **default is 600s** because it is sized for
build/test **gate commands**. `shipment ship` holds the workspace lock across the
entire completion and is itself unbounded. So a hung, stuck, or maliciously slow
`git` child could hold the lock for **ten minutes** — a bounded call that is
nonetheless a denial of service. "Bounded" told us the wait was finite; it did not
tell us the bound was *appropriate*.

### Fix

Introduce a dedicated hard cap and a helper that treats it as a ceiling:

```go
const ancestryCheckTimeout = 5 * time.Second // HARD CAP for git metadata reads

func (ws *Workspace) boundedHelperTimeout() time.Duration {
    d := ancestryCheckTimeout
    if ws.GateBroker != nil && ws.GateBroker.TimeoutSeconds > 0 {
        if configured := time.Duration(ws.GateBroker.TimeoutSeconds) * time.Second; configured < d {
            d = configured // honor a SMALLER configured value...
        }
    }
    return d // ...but never a larger one.
}
```

`merge-base`/`rev-parse` are near-instant **local** reads, so 5s is generous. Both
helpers derive their OWN deadline via `boundedHelperTimeout()` and never rely on the
caller imposing one. The cap is one-directional: a stricter operator setting is
respected; a command-sized 600s is clamped away.

## Generalization

- A timeout is two decisions: *finite* and *how long*. Wrapping in
  `context.WithTimeout` only settles the first. A finite-but-wrong bound on a
  lock-holding path is still an availability bug.
- Do not reuse a timeout config across workloads with different latency profiles.
  A "gate command" budget (minutes) and a "local metadata read" budget (seconds)
  are different knobs even though both spawn a child process.
- Prefer a per-class hard cap that clamps `min(configured, cap)`, so operators can
  tighten but never accidentally loosen a security/availability-critical read.
- Never let a lock-holding critical path inherit a deadline sized for a slower,
  non-lock-holding operation.

## Related

- `docs/compound/2026-07-06-external-process-timeout-before-probe.md` — the 082-S
  parent lesson (bound the first lock-holding external call). This entry adds: even
  when you *have* bounded it, check that the bound fits the workload.
- `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md` — the fix
  these helpers serve.
- `docs/closure/2026-07-06-084-S-feature-pr-operational-closure.md` — Copilot
  remediation record.
