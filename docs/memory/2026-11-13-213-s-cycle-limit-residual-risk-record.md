---
title: "213-S P-021 C1 (Worked Discrimination Case b & Fail-Safe Default) Out-of-Scope Classification and P-021 C3(ii) Residual-Risk Record"
doc_type: reference
source: docs/memory/2026-11-13-213-s-cycle-limit-residual-risk-record.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-13"
status: resolved
---

## P-021 C1 (Worked Discrimination Case `b` & C1 Fail-Safe Default) Out-of-Scope Classification

* Both `C446F206` and `5B8497D4` **fail P-021 C1** and are genuinely **out of scope for `212.002-T` (Unit U9) and `213-S`** under P-021 C1 worked discrimination case `(b)` (`.github/policies/workflow-policies.md:1957`) and the mandatory C1 fail-safe default (`"Genuine ambiguity resolves OUT of scope (fail-safe default)"`), with **zero code or test edits permitted to `internal/toolchain/verify.go` or `internal/toolchain/verify_test.go` in `213-S`**.

## Technical Proof of P-021 C1 Out-of-Scope Classification & P-021 C3(ii) Residual-Risk Record

* **Item (a) — `hostPrerequisite.required: null` YAML Scalar-Tag Validation (`C446F206`, `DISCOVERY-STATUS: AMBIGUOUS 036B0CB9`) Fails P-021 C1 (Worked Discrimination Case `b`)**:
  * In **U8 (`212.001-T`, `internal/toolchain/types.go:34`, locked by `TestU8_ToolchainDeclarationsSourceShape` in `internal/toolchain/toolchain_shape_test.go`)**, `HostPrerequisiteEntry.Required` is declared as a value type **`bool`** (`Required bool`), NOT `*bool`.
  * In **U9 (`212.002-T`, `internal/toolchain/verify.go:199-203`)**, `LoadToolchainManifest` delegates all YAML scalar value decoding to `gopkg.in/yaml.v3`'s `decoder.Decode(&m)`. In `gopkg.in/yaml.v3`, a YAML `!!null` scalar (`required: null`) unmarshals into the Go zero value `false` on a `bool` struct field — indistinguishable from `required: false` after `Decode(&m)` unless either U8's locked `HostPrerequisiteEntry.Required bool` type is changed to `*bool` (which would break U8's locked AST harness `TestU8_ToolchainDeclarationsSourceShape`) OR `validateManifestFields` (`internal/toolchain/verify.go:246-288`) is extended from a **mapping-key presence check** into a **YAML `yaml.Node.Tag` scalar-type validator**.
  * In Cycle 2 (`commit a4ec6ead`, `TestU9_ReviewFixManifestStorageAndYAML/missing-prerequisite-required` at `internal/toolchain/verify_test.go:635-646`), the authorized change in `validateManifestFields` was strictly **mapping-key presence** (`!found[key]` when `required:` is omitted from the YAML map).
  * Under **P-021 C1 worked discrimination case `(b)`** (`.github/policies/workflow-policies.md:1957`: `"the regex does not handle an object-separated form -> different surface -> defer, EVEN THOUGH same function, same file, same PR, and in scope for an earlier authorized cycle"`), extending `validateManifestFields` to inspect `yaml.Node.Tag` (`!!bool` vs `!!null`) when the `required:` key IS present in the map is a **different contract surface** (YAML AST scalar-tag type validation vs. mapping-key presence + `yaml.v3` `bool` struct decoding), and any remaining ambiguity between JSON Schema `type: boolean` and `yaml.v3` `bool` decoding resolves **out of scope** under P-021 C1's mandatory fail-safe default (`"Genuine ambiguity resolves OUT of scope (fail-safe default)"`).
* **Item (b) — Dedicated `server.Type` / `server.URL` Subtest Permutations for `f078fc3f` (`5B8497D4`, `DISCOVERY-STATUS: AMBIGUOUS 48F1E692 6BF65C9D 5F7A81D7`) Fails P-021 C1 & C4**:
  * The production transport checks in `validateMCPConfigPinning` (`internal/toolchain/verify.go:695-725`) are **already implemented and shipping** (commits `f078fc3f` and `1a185d7a`, with `server.URL` exercised by `TestU9_ReviewFixMCPArgumentsAndBindings/duplicate-trimmed-server-names` at `verify_test.go:442`), and Stage formally ratified `f078fc3f` in commits `98faa3ef` and `f843e3ca` against the U9 Step 2 RED harness (`ca7d0e21`, `279c7aa7`) after `build-feature` refused `verify_test.go` edits on a `harness-ready` task (`fdcc2cf4`, `8db00cbb`, tracked in `6BF65C9D`).
  * Expanding `verify_test.go` with additional standalone `server.Type` / `server.URL` test-matrix permutations (or expanding `verify.go` into `48F1E692`'s deferred transport whitespace/case surface) is a **different contract surface** under P-021 C1 (test-matrix permutation expansion beyond the locked 12-test `TestU9_*` suite, analogous to `5F7A81D7`, `FA754084`, and `406AA1E0`) and is barred in-flight by P-021 C4 (`48F1E692`) and P-004 (adding tests for already-implemented `f078fc3f` checks without new production behavior is false-green).

## Follow-Up Stash Inventory

* Prior U9 / `213-S` P-021 C1/C2 deferred stash entries: `00FD5168, 8841FD73, 1406FAE2, CF190A0C, 58AA0592, 5F7A81D7, 2C71DFA2, D1455316, 3083FC1A`.
* Step 5 pre-PR P-021 C1/C2 deferred stash entries: `01D3F26B, CBD2D21C, B7E3A649, 8AD5625B, 5688AB99, 600835CA, 0AF43C97, F12B587F, C446F206, 5B8497D4`.
* Complete follow-up stash ID list (`19` total): `00FD5168, 8841FD73, 1406FAE2, CF190A0C, 58AA0592, 5F7A81D7, 2C71DFA2, D1455316, 3083FC1A, 01D3F26B, CBD2D21C, B7E3A649, 8AD5625B, 5688AB99, 600835CA, 0AF43C97, F12B587F, C446F206, 5B8497D4`.

## Gate Outcome

* Shipment `213-S` (`212-F`, tasks `212.001-T`, `212.002-T`, `212.003-T`) is **`READY_WITH_FOLLOWUPS` (`P0=0, P1=0`)** for Step 5 PR creation, merge, and Step 6 closure with zero further changes to `internal/toolchain/verify.go` or `internal/toolchain/verify_test.go`.
