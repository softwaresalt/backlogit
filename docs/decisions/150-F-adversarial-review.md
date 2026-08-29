---
title: "150-F Adversarial Review Consensus"
feature_id: 150-F
review_type: adversarial
reviewers: 3
models: [claude-sonnet-5, gpt-5.6-terra, gemini-3.7-flash]
status: remediated
created_at: 2026-08-29T09:48:00Z
---

# 150-F Adversarial Review — Consensus Report

## Reviewer Dispatch
- **T1** (claude-sonnet-5): Frontier model, 7 findings
- **T2** (gpt-5.6-terra): Standard model, 3 findings  
- **T3** (gemini-3.7-flash): Fast model, 4 findings

## Consensus Findings

### FALSE POSITIVES (Dismissed)

| Finding | Raised By | Confidence | Verdict | Evidence |
|---------|-----------|------------|---------|----------|
| fsutil.go still has pre-Remove | T2 (P1-HIGH), T3 (P2-HIGH) | HIGH | FALSE POSITIVE | `git show origin/main:internal/events/fsutil.go` confirms no `runtime.GOOS` or `os.Remove(path)`. PR #387 commit 02f5a8fc removed it. |
| AST helpers don't exist | T3 (P1-HIGH) | HIGH | FALSE POSITIVE | Helpers exist in `internal/events/checkpoint_astshape_test.go` lines 22, 79. |
| Cross-volume rename risk | T1 (P1-HIGH) | HIGH | FALSE POSITIVE | archiveDir = `filepath.Dir(checkpointDir)/archive/checkpoints` — always same volume (parent directory relationship). |
| hook_events.go exclusion unjustified | T1 (P1-MEDIUM) | MEDIUM | ADDRESSED | Reviewed actual code: `os.Remove(recoveringPath)` where `recoveringPath = lockPath + ".recovering"` — transient atomic-claim marker, not primary data. Original lockPath intact if rename fails. |

### VALID ADVISORIES (Accepted, No Gate Block)

| Finding | Raised By | Consensus | Severity | Action |
|---------|-----------|-----------|----------|--------|
| AST-only test is structural, not behavioral | T1 (P2-MEDIUM), T2 (P2-HIGH), T3 (P1-HIGH) | MEDIUM (majority) | P2 | **Deferred.** Consistent with 149-F prior art (AST-only test accepted). Behavioral test for Go stdlib os.Rename semantics is enhancement, not bug fix requirement. Would test Go stdlib, not application logic. |
| AST test fragile to variable renaming | T1 (P2-HIGH) | LOW (single) | P3 | **Acknowledged.** Same fragility exists in 149-F test. Pattern is established. |
| Rollback guidance insufficiently operational | T2 (P3-MEDIUM) | LOW (single) | P3 | **Acknowledged.** Plan updated: rollback restores prior behavior including data-loss window, which was the known-working state. |
| P-002 RED commit must be CI-verified | T1 (P3-MEDIUM) | LOW (single) | P3 | **Already addressed** in plan FC-2 requirement. |
| runtime import removal file-wide check | T1 (P3-LOW) | LOW (single) | P3 | **Verified:** only one `runtime.GOOS` usage in file. Removal safe. |

## Gate Decision

**PASS** — No valid P0/P1 findings. All P1 findings were false positives (incorrect code analysis by reviewers). P2 behavioral test coverage is a valid advisory deferred as enhancement, consistent with accepted prior art. Plan proceeds as designed.

## P2 Disclosure: Behavioral Test Gap

The AST test proves the literal `os.Remove(dst)` call is absent. It does not behaviorally test that `os.Rename` correctly replaces an existing destination on Windows. This is deferred because:
1. Go 1.24.0 stdlib behavior is well-documented and tested upstream
2. 149-F used the identical AST-only approach and was accepted
3. A behavioral test would test Go stdlib, not application logic
4. Adding it would expand scope beyond the narrow bug fix
