---
chunk_strategy: h1-h2-h3
description: 'Go pattern: own a resource lock in the synchronous caller frame, never inside a timeout goroutine, so a deadline abandon cannot strand the lock. Split prepare (locked) from validate (lock-free, timed).'
doc_type: learning
docline:
    category: best_practice
    component: doctor_target
    date: 2026-07-01T00:00:00Z
    file_path: internal/core/doctor_target.go
    message: A defer-unlock inside a timeout goroutine runs only when the goroutine finishes; on deadline the parent returns and the advisory lock is stranded.
    problem_type: best_practice
    resolution_type: code_fix
    resolved: true
    root_cause: concurrency
    severity: high
    tags:
        - go
        - concurrency
        - timeout
        - advisory-lock
        - goroutine
        - context-deadline
        - resource-ownership
        - doctor
ingested_at: "2026-07-01T16:45:00Z"
schema_version: "1.0"
source: docs/compound/best-practices/timeout-goroutine-lock-ownership-go-2026-07-01.md
title: 'Own Resource Locks in the Synchronous Frame, Not Inside a Timeout Goroutine'
---

# Lock Ownership Must Live Outside the Timeout Goroutine

## Context

Shipment 071-S added `backlogit doctor --target {file}` — single-file validation
with a real 5s deadline and a per-task advisory file lock (`.<name>.lock`). The
first implementation acquired the lock inside the goroutine that also ran the
timed validation. It took three Copilot review-fix cycles to converge on the
correct ownership model (threads D/E in cycle 2, F in cycle 3).

## Problem

A common Go timeout shape runs the cancellable work in a goroutine and races it
against a deadline:

```go
done := make(chan error, 1)
go func() {
    unlock, err := acquireLock(path) // WRONG: lock owned inside the goroutine
    if err != nil {
        done <- err
        return
    }
    defer unlock() // runs only when THIS goroutine returns
    done <- validate(path)
}()

select {
case err := <-done:
    return err
case <-time.After(5 * time.Second):
    return ErrTimeout // parent returns; goroutine keeps running (or is abandoned)
}
```

When the deadline fires, the parent returns `ErrTimeout`, but the goroutine is
still live. The `defer unlock()` has **not** run yet — it runs whenever the
goroutine eventually finishes, which may be long after the caller believed the
operation was over. The advisory sidecar lock is therefore **stranded**: a
subsequent caller sees a held lock and maps it to busy (exit 4) even though no
one is really working. Worse, if the goroutine is truly abandoned, the lock is
released at a non-deterministic time, defeating the whole point of a bounded
operation.

Two secondary defects fall out of the same mistake:

* The timeout budget is consumed by lock acquisition, so a slow/contended lock
  can eat the deadline and surface as a timeout (exit 2) instead of busy (exit 4).
* Distinguishing an IO fault on the lock (exit 3) from contention (exit 4)
  becomes tangled with the timeout path.

## Solution

Split the operation into a **synchronous, locked prepare** and a **lock-free,
timed validate**. Own the lock in the caller's frame with a plain `defer`; run
only lock-free work under the deadline goroutine.

```go
// PrepareDoctorTarget: synchronous. Acquires the lock; returns an unlock the
// caller defers. Classifies acquisition failures deterministically:
//   busy contention -> exit 4 ; IO/stat fault -> exit 3.
func runDoctorTargetMode(path string) int {
    resolved, unlock, err := core.PrepareDoctorTarget(path)
    if err != nil {
        return doctorTargetExitCode(err) // 3 or 4, never 2
    }
    defer unlock() // bound to THIS synchronous frame — always runs

    // Only the lock-free validation runs under the deadline.
    done := make(chan validateResult, 1)
    go func() { done <- core.ValidateDoctorTargetResolved(resolved) }()

    select {
    case r := <-done:
        return doctorTargetExitCode(r.err) // 0 pass / 1 validation
    case <-time.After(5 * time.Second):
        return 2 // timeout; unlock still runs via the caller's defer
    }
}
```

Key properties:

* **Deterministic release.** The lock lifetime equals the synchronous frame.
  On timeout, pass, or validation failure, the caller's `defer unlock()` runs
  before the function returns. No stranding.
* **Clean exit-code contract.** Lock acquisition maps to 3/4 *before* the timed
  region; the timed region can only yield 0/1/2. Busy is never misreported as
  timeout.
* **No shared mutable state crosses the goroutine boundary.** `ValidateDoctorTargetResolved`
  takes an already-resolved, read-only target and touches no lock — safe to run
  under an abandoned goroutine because it owns nothing that must be released.

## Why This Works

A `defer` only fires when its enclosing function returns. If that function is a
goroutine racing a deadline, "returns" is decoupled from the caller's control
flow — so any resource whose release you `defer` inside the goroutine is
released on the goroutine's schedule, not the caller's. Moving ownership up to
the synchronous frame re-couples release to the caller's return, which is the
event the timeout actually bounds.

## Prevention

* Never acquire a lock (or any must-release resource) inside a goroutine that a
  parent may abandon on timeout/cancel. Acquire it in the synchronous caller and
  `defer` the release there.
* Give the goroutine only lock-free, side-effect-free-on-abandon work.
* Acquire/classify resource-contention errors *before* entering the timed
  region so busy/IO faults never masquerade as timeouts.
* Add a test that holds the sidecar lock and asserts busy (not timeout), plus a
  test that forces a timeout and asserts the lock is released afterward.

## Related Solutions

* `best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md` — the sidecar
  lock + stale-TTL mechanism this ownership model builds on.
* `docs/closure/2026-06-30-071-S-deterministic-gates-slice-closure.md` — the
  frozen `doctor --target` exit-code contract (0/1/2/3/4) this pattern protects.
