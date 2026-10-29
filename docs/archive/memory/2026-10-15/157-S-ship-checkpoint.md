# 157-S Ship checkpoint (2026-10-15, dark mode)

- Shipment 157-S claimed on `feat/157-s-baseline-convergence-rs-w00-u1-line-ending-source-boundary`; topology gate pre_claim (x2) and post_claim exit 0.
- Baseline file digest verified (CRLF->LF canonical): c986c98378d173de79f11017e8c257299d5dc948e4864ee4a731a3330b56b574. Live claim-time set = 564 (528 Go + 36 JSON) = exact baseline membership; extra=0; HEAD/index blobs identical; worktree == index after CRLF canonicalization for all.
- Frozen path list: `175-001-frozen-path-list.txt` (written before any refresh).
- `.gitattributes`: +9 added rule lines only (`*.go eol=lf`, `*.json eol=lf`, `-text` for exe/db/png/jpg/jpeg/gif/ico); parity golden pin retained.
- Refresh: `git restore --worktree --pathspec-from-file` alone was a no-op (git treats stat-clean index-LF files as unchanged once eol=lf attr applies), so the same scoped path list was deleted and restored from the index (scoped, no whole-tree op). 564 paths now `w/lf`, `i/crlf`=0, `w/mixed`=0; staged set = `.gitattributes` only.
- Interpretation recorded: 26 listed paths under `testdata/` were refreshed as part of the operator's digest-pinned approved list (clause (c)); clause (e)(4) "approve by name" read as satisfied by the digest-pinned list naming them. Flagged for operator acknowledgement in PR.
- Harness `TestU175_001_BaselineEol`: red at 0736b919, green after refresh; hardened at 7f985033 after adversarial review.
- Gates: go build, go vet, full tests green (internal/core needed isolated rerun: 10m default timeout under contention; passes at 768s with -timeout=20m). golangci-lint cannot run locally (tool built with go1.24 vs go1.25 deps) - environmental.
- Next: PR, Copilot gate, merge, closure.
