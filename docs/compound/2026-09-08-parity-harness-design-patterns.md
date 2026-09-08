---
title: "Cross-surface parity harness design patterns (138-S / 156-F)"
date: "2026-09-08"
source_shipment: "138-S"
source_feature: "156-F"
category: "go-patterns"
tags: ["parity", "evidence-contract", "json-parsing", "testing", "wave-scheduler"]
---

# Cross-surface parity harness design patterns (138-S / 156-F)

Key learnings from implementing the fault-line evidence contract and three-surface parity harness.

## 1. JSON null ≠ absent key: separate presence checks from type-validity

**Problem**: Go's `json.Unmarshal` maps a JSON `null` value to a `nil` interface{}.
A check of `if !ok || v == nil { return false, false }` treats `null` as absent-and-valid,
masking malformed output where a surface emits `"retryable": null` instead of omitting the key.

**Pattern**: Three-value result `(value T, present bool, typeValid bool)`:
```go
func getBoolPresent(m map[string]any, key string) (bool, bool, bool) {
    v, ok := m[key]
    if !ok { return false, false, true }   // absent: ok
    if v == nil { return false, true, false } // null: present but wrong type
    b, isBool := v.(bool)
    if !isBool { return false, true, false } // wrong type
    return b, true, true
}
```
A type mismatch should fail the dimension closed, never produce a pass.

## 2. parseBody must handle both JSON objects AND arrays

**Problem**: backlogit's `list` command returns a top-level JSON array.
A comparator that `json.Unmarshal`s only into `map[string]any` silently treats
all successful `list` responses as absent, producing "no applicable surfaces to attest."

**Pattern**:
```go
func parseBody(raw []byte) (map[string]any, []any, bool) {
    if len(raw) == 0 { return nil, nil, false }
    var m map[string]any
    if err := json.Unmarshal(raw, &m); err == nil && m != nil { return m, nil, true }
    var a []any
    if err := json.Unmarshal(raw, &a); err == nil { return nil, a, true }
    return nil, nil, false
}
```
Body parse failure should be treated as a surface error (fail closed), not absent.

## 3. TrackedDefect IDs must reference current NON-TERMINAL backlog items

**Problem**: In parity test fixtures, `TrackedDefect` was initially set to `156.007-T` (an archived task).
A harness that validates TrackedDefect against the live backlog (U3 requirement) rejects archived IDs.

**Pattern**: The leaf package (`internal/faultline`) validates only the SYNTAX of the backlog ID
(regex `^\d{3,}(\.\d{3,})*-[A-Z]{1,2}$`). The harness (`internal/faultline/parity`) validates
the ID resolves to a CURRENT NON-TERMINAL item via a read-only filesystem walk of `.backlogit/`.
Use a queued, standalone feature outside the shipment hierarchy as the tracked defect owner.
Archive-aware: check both queue and archive directories; reject items with `archived`, `done`,
`shipped`, or `abandoned` status.

## 4. Timestamp bump comparison requires time.Time parsing

**Problem**: Comparing RFC3339 timestamps as strings is fragile. Equal instants with different
timezone representations (e.g., `2026-09-08T10:00:00Z` vs `2026-09-08T03:00:00-07:00`) compare
as different strings but represent the same moment.

**Pattern**: Always parse timestamps via `time.Parse(time.RFC3339, ...)` or `time.RFC3339Nano`
before comparison. Require `updated.After(seed)` strictly. Fail closed on unparseable timestamps.

## 5. Wave-scheduler fixture maintenance for shipped shipments

**Problem**: After a shipment ships (tasks archived), the wave-scheduler sim's `queue_drift` check
fails because the fixture expects `queued` but the live queue has `archived`.

**Fix**: In `Compare-LiveShipmentProjection`, skip `MEMBER_STATUS` drift when the LIVE member
is in `terminal_success` state. Post-shipment archival is expected and not a contract defect.
Use case-sensitive `-cnotin` (not `-notin`) for consistency with outer `-cne` guard:
```powershell
$terminalSuccessTokens = @((ConvertTo-List $Fx.status_model.terminal_success))
if ($liveMember.status -cnotin $terminalSuccessTokens) {
    Add-Drift $drift 'MEMBER_STATUS' "..."
}
```

## 6. SanitizeDiagnostic: enforce output budget, not input budget

**Problem**: Enforcing a rune budget on the INPUT before escaping allows 200 NUL chars
to produce 800 output chars (`\x00` × 200 = 4 chars/char).

**Pattern**: Check remaining capacity BEFORE writing each escape sequence:
```go
if outputRunes+len([]rune(escaped)) > MaxDiagnosticRunes {
    sb.WriteString("...[truncated]")
    return sb.String()
}
sb.WriteString(escaped)
outputRunes += len([]rune(escaped))
```

## 7. Dynamic producingCommit via runtime/debug.ReadBuildInfo

**Problem**: Hardcoded SHA constants in evidence producers become stale immediately after any new commit.

**Pattern**:
```go
var producingCommit = func() string {
    if info, ok := debug.ReadBuildInfo(); ok {
        for _, s := range info.Settings {
            if s.Key == "vcs.revision" && s.Value != "" {
                return s.Value
            }
        }
    }
    return "dev"
}()
```
Add `SetProducingCommitForTest` for deterministic test overrides.
Falls back to `"dev"` in non-VCS environments (satisfies U4a non-empty invariant).

## 8. AST source-shape harnesses: name-only check is insufficient

**Problem**: An AST harness that only checks `funcDecl.Name.Name` passes even if the receiver,
parameter types, or return types change, failing to guard the public contract.

**Pattern**: For each required function, also inspect:
- `funcDecl.Recv` for method receivers
- `funcDecl.Type.Params.List` for parameter types
- `funcDecl.Type.Results.List` for return types

Use a simple AST type-renderer (not a full type-checker) to extract basic type names.
This is still a source-shape check (go/ast only, no symbol imports) so it compiles before
the implementation exists.
