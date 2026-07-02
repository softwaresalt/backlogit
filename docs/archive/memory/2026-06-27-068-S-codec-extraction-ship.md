# Ship session memory — 068-S Shared frontmatter codec extraction

- **Date**: 2026-06-27
- **Shipment**: 068-S "Shared frontmatter codec extraction" (status: active, queued for post-merge closure)
- **Feature**: 068-F (status: active)
- **Branch**: `feat/068-codec-extraction`
- **Type**: pure behavior-preserving refactor (break `internal/docline ↔ internal/core` codec + atomic-write duplication via two stdlib-only leaf packages)

## Items completed (all 4 tasks done + archived)

| Task | Unit | Scope | Commit |
|---|---|---|---|
| 068.001-T | U1 | `internal/mdfront` body-preserving frontmatter codec (leaf) | `dd7b81fe` |
| 068.002-T | U2 | `internal/atomicfile` hardened `WriteFileAtomic` (leaf) | `181f654d` |
| 068.003-T | U3 | Migrate `internal/docline` onto leaf pkgs (alias + Decode re-export, atomicfile delegation) | `4743da5c` |
| 068.004-T | U4 | Migrate `internal/core/doctor.go` onto leaf pkgs (mdfront + F1 guard + atomicfile) | `848c162c` |

Follow-up commits on branch:
- `ea10ced0` docs(core): clarify atomicfile mode-policy comment (P3 review nit)
- `7a23a2db` chore(core): mark tasks 068.001-T..068.004-T done; claim shipment 068-S (backlog state)

## Decisions / rationale

- **TRUE alias `type Markdown = mdfront.Markdown`** in docline/codec.go: `(*Markdown).Encode()` is INHERITED via the alias; Go forbids re-declaring a method on a non-local alias target, so only the package-level `Decode` is forwarded. Preserves byte-identical public docline API for `cmd/gen-docs`.
- **F1 guard** in core `rewriteArchivedFromField`: `mdfront.Decode` returns `HasFrontmatter=false` + nil map + NO error on a fence-less record (vs core's old hard error). Added `if !md.HasFrontmatter { return error }` + non-nil-map guard to preserve error→skip parity and prevent nil-map panic / synthetic-frontmatter corruption.
- **Leaf packages stdlib-only** (mdfront: bytes/fmt/yaml.v3; atomicfile: stdlib) — import cycle broken cleanly; core + docline both import the leaves, leaves import nothing internal.
- **atomicfile hardening**: tmp+rename, io.Writer seam with short-write guard, mode-clamp `perm &^ 0o022`, Windows pre-remove-and-retry rename fallback, deliberately sync-free.
- **Path guards UNTOUCHED**: docline `ValidateApplyPath` + `core.SafeResolve` preflight, doctor Lstat/EvalSymlinks/pathContained checks all preserved before the (path-agnostic) atomic write.

## Verification (behavior preservation)

- `go test ./...` green (22 pkgs incl. mdfront, atomicfile); `go vet` clean; `golangci-lint run` clean; `gofmt -l` clean on all 10 changed files.
- Golden differential byte-equality tests lock mdfront codec + doctor `rewriteArchivedFromField` against captured pre-refactor bytes.
- Runtime: `docs migrate` dry-run byte-identical old-vs-new (0 body-byte changes); single-file `--apply` no-op byte-identical; doctor `--check-archived-from` identical 0 self-ref / 2 malformed (038-DL, 039-DL) on old vs new binary.
- CLI Reference Drift: proven pre-existing Windows CRLF working-tree noise (old `main` produces identical 63-file drift); CI on Linux/LF expected green.

## Review gate

- Go Reviewer: ZERO P0/P1 (4 P3 advisory nits; 1 fixed in `ea10ced0`, others retained intentionally).
- Security Reviewer: ZERO exploitable findings; all 5 path/mode/F1 invariants verified preserved.

## Next steps

1. Push `feat/068-codec-extraction`.
2. Open PR; request Copilot review; run §1.9 GraphQL unresolved-thread loop (reply + RESOLVE) to 0.
3. Drive CI green (incl. docline gate + CLI Reference Drift).
4. HALT at merge gate (P-009 merge-commit) for operator approval. Do NOT merge.
5. Post-merge closure (ship 068-S, archive 068-F) is a SEPARATE run.
