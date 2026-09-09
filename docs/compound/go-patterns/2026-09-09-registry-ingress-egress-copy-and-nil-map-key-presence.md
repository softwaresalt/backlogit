---
chunk_strategy: h1-h2-h3
description: Two Go safety patterns from fault-line mutation work - thread-safe registry ingress+egress defensive copies, and key-presence check to avoid bytes.Equal(nil,nil) false positives
doc_type: learning
docline:
    ms.date: 2026-09-09T00:00:00Z
    ms.topic: reference
    source_shipment: 139-S
    tags:
        - go
        - concurrency
        - defensive-copy
        - maps
        - bytes
        - registry
schema_version: "1.0"
source: docs/compound/go-patterns/2026-09-09-registry-ingress-egress-copy-and-nil-map-key-presence.md
title: Thread-Safe Registry Defensive Copies and bytes.Equal(nil,nil) Pitfall
---

## Thread-Safe Registry: Copy on Both Ingress and Egress

When a thread-safe registry stores structs containing slice fields (e.g., `map[string]struct{ S []T }`), **copy the slice on both ingress and egress**, not just egress.

### Why Egress Alone Is Not Enough

Most documentation covers egress (outgoing) defensive copies: copy before returning so the caller cannot corrupt the stored value. But if you don't copy on ingress (incoming), a caller can corrupt the stored value **after registration**:

```go
// Caller owns this slice:
reps := []RepresentationKind{Frontmatter, SQLite}
set := RepresentationSet{Op: "CreateItem", Representations: reps}
Register(set)           // stored without copy — reps backing array now shared
reps[0] = ArchiveFile   // ← corrupts the stored registry entry!
```

### Pattern: Copy on Register, Copy on Lookup

```go
var (
    mu  sync.RWMutex
    reg = map[string]RepresentationSet{}
)

func Register(set RepresentationSet) error {
    // ... validation ...
    mu.Lock()
    defer mu.Unlock()
    // Ingress copy: prevent caller mutation after registration.
    cp := make([]RepresentationKind, len(set.Representations))
    copy(cp, set.Representations)
    set.Representations = cp
    reg[set.Op] = set
    return nil
}

func Lookup(op string) (RepresentationSet, bool) {
    mu.RLock()
    defer mu.RUnlock()
    set, ok := reg[op]
    if !ok {
        return RepresentationSet{}, false
    }
    // Egress copy: prevent consumer mutation from corrupting the stored entry.
    cp := make([]RepresentationKind, len(set.Representations))
    copy(cp, set.Representations)
    set.Representations = cp
    return set, true
}
```

### When to Apply

Any time a registry or cache stores a struct with a public or caller-created slice field. The struct itself is value-copied (Go passes structs by value), but the slice header — not the backing array — is what gets copied. Copying the backing array requires explicit `make` + `copy`.

---

## bytes.Equal(nil, nil) Pitfall in Map-Based Snapshot Verifiers

`bytes.Equal(nil, nil)` returns `true`. In a snapshot verifier that compares `Before[k]` and `After[k]` for each declared key, a snapshot where **both** Before and After lack a key (nil map, or empty map without the key) silently passes comparison — a phantom "no change" is reported with no real observation.

### The Bug

```go
// VerifyFailure: checks that all representations are unchanged after a failure.
for _, k := range set.Representations {
    if !bytes.Equal(snap.Before[k], snap.After[k]) {  // nil==nil for missing keys
        drifted = append(drifted, k)
    }
}
// Result: Passed=true when both Before and After are nil maps!
// No state was observed, yet the verifier claims rollback was confirmed.
```

### The Fix: Check Key Presence First

Use the two-value map lookup to distinguish "key present with nil value" from "key absent":

```go
// Completeness guard: require key presence in at least one direction.
for _, k := range set.Representations {
    _, hasB := snap.Before[k]
    _, hasA := snap.After[k]
    if !hasB && !hasA {
        return VerificationResult{Op: snap.Op, IncompleteSnapshot: true}, nil
    }
}
```

This allows `snap.Before[k] = nil` (explicit nil, key present) to proceed to comparison — the caller explicitly populated the key — while rejecting a snapshot that never populated the key at all.

### Note on VerifySuccess vs VerifyFailure

The fix matters differently for each direction:
- **VerifyFailure**: without the guard, nil/nil gives `Passed: true` — a **false positive** (incorrectly claims rollback verified).
- **VerifySuccess**: without the guard, nil/nil adds to `missing` (unchanged), giving `Passed: false` — not a false positive, but a **misleading** failure mode (can't distinguish "op changed nothing" from "op was never observed").

Apply the guard in both directions for consistency and precision.

### Signal: Add IncompleteSnapshot to the Result Type

Rather than returning an error or reusing an existing failure mode, a dedicated `IncompleteSnapshot bool` field in the result type precisely signals the "snapshot was not observable" case, keeping it distinct from "op passed" or "op failed with drift."

---

## Source

Shipment 139-S: `internal/faultline/mutation` (representations.go, registry.go, verify.go, verify_test.go)
