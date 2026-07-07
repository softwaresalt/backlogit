---
chunk_strategy: h1-h2-h3
description: 'A security-preserving pattern graduated from 085-S — an ENFORCED completion gate that must distinguish "real work tree, cannot prove lineage → fail closed" from "genuine no-repo → preserve the legacy skip" CANNOT use the enforcement flag as the discriminator when the test broker fakes the git probe (so ev.Enforced is true even in a no-repo temp dir). The correct discriminator is a bounded, fail-closed repo-presence probe (git rev-parse --is-inside-work-tree, inGitWorktreeBounded) with a message-independent os.Stat(.git) broken-repo boundary. Empty shipment/member head inside a real work tree under enforcement → FAIL CLOSED; genuine no-repo → skip; broken/corrupt repo, probe timeout/cancel/git-missing → FAIL CLOSED. This closes the empty-head fail-open seams that 084-S explicitly deferred.'
doc_type: learning
docline:
    date: 2026-07-07T00:00:00Z
    severity: high
    tags:
        - security
        - gate
        - git
        - fail-closed
        - repo-presence
        - rev-parse
        - empty-head
        - enforcement
        - core
        - windows
schema_version: "1.0"
source: docs/compound/2026-07-07-empty-head-fail-closed-repo-presence-probe.md
title: 'Repo-presence probe as the enforcement discriminator for empty-head fail-closed (085-S)'
---

# Empty-Head Fail-Closed via a Repo-Presence Probe

Graduated from shipment 085-S (feature 085-F, task 085.001-T + 3 subtasks; feature
PR #185, merge `7c129b0407db9beb943bc737df4bc3b287286b77`). Cleared a 3-model
pre-push adversarial review that first **BLOCKED** (F1) and, after remediation,
returned **RE-REVIEW PASS** (SEC-1 fail-open-closed + SEC-2 legitimate-empty-preserved
both CONFIRMED), plus two Copilot cycles resolved to zero. Closes the empty-head
seams that `2026-07-06-ancestor-aware-shipment-gate-staleness.md` (084-S) explicitly
scoped out.

## Rule — Do not use the enforcement flag to decide repo presence

The enforced completion gate needs to fork on a question the enforcement flag does
not answer:

- **Empty head + real work tree + enforcement** → the lineage is *unprovable* →
  **FAIL CLOSED** (refuse). A shipment whose head — or a member's head_sha — is empty
  cannot demonstrate the gated work is in the release lineage.
- **Empty head + genuine no-repo** → **preserve the legacy skip**. Tests and
  non-autoharness/bare-repo/inside-`.git` callers legitimately run with no work tree;
  refusing them would be a fail-*shut* regression (broken completion).

### Why `ev.Enforced` cannot be the discriminator

The gate's test broker **fakes the git probe**, so `ev.Enforced == true` even inside a
no-repo temp dir. Keying the fail-closed branch off `ev.Enforced` would either refuse
every no-repo test (false shut) or, if softened, re-open the very hole. Enforcement
tracks *policy*, not *repository presence*. You must measure presence directly.

## The discriminator — a bounded, fail-closed repo-presence probe

`inGitWorktreeBounded` runs `git rev-parse --is-inside-work-tree` with the same trust
boundary as the 084-S lineage helper (argv-array `exec.CommandContext`, no shell,
`cmd.Dir = ws.RootPath`, `cmd.Env = withCLocale(gate.MinimalEnv())`, self-derived
bounded deadline `boundedHelperTimeout()`), and resolves a **four-way** result:

```
runCtx.Err() != nil (checked FIRST) -> fail closed (timeout/cancel misreported as exit code)
exit 0, stdout "true"               -> in a work tree      -> (true,  nil)
exit 0, other stdout                -> bare repo / .git    -> (false, nil)  preserve skip
run error IS *exec.ExitError        -> os.Stat(RootPath/.git):
    !os.IsNotExist(statErr)         ->   .git present OR indeterminate (perm/IO) -> FAIL CLOSED
    os.IsNotExist(statErr)          ->   definitively absent -> marker check:
        exit 128 + stderr has "not a git repository (or any of the parent directories)" -> (false,nil) skip
        else                        ->   FAIL CLOSED (unverifiable)
run error NOT ExitError (git missing / spawn failure) -> FAIL CLOSED (no os.Stat reached)
```

Then: under a real work tree (`true`), an empty shipment head or empty member head_sha
→ FAIL CLOSED; under `false`, preserve the legacy skip.

## The load-bearing subtleties (each cost an adversarial finding)

- **Message-independent broken-repo boundary (F1 + N1).** A present-but-broken `.git`
  pointer emits `fatal: not a git repository: (NULL)` (exit 128, **no** parenthetical);
  an empty/garbage `.git` *directory* emits the genuine marker but `.git` is present.
  A loose substring match on `"not a git repository"` misclassifies both as no-repo →
  re-opens the fail-open. The primary discriminator is therefore
  `os.Stat(RootPath/.git)`: **if `.git` exists (or the stat is indeterminate) → fail
  closed, regardless of git's message.** Only a definitive `os.IsNotExist` proceeds to
  the marker check. Genuine no-repo has **no** `.git` entry at the workspace root.
- **`os.Stat` error handling must itself fail closed (Copilot).** A non-`IsNotExist`
  stat error (permission/IO) must fail closed, not fall through to the marker check —
  use `!os.IsNotExist(statErr)`, not `statErr == nil`.
- **Pin the locale (F1/N2).** The English marker is locale-dependent; strip inherited
  `LANG`/`LC_ALL` from `MinimalEnv()` and append `LC_ALL=C`/`LANG=C`
  (`withCLocale`/`hasEnvKey`, exact `=`-boundary key match) so a localized git cannot
  dodge the marker. The `os.Stat` guard is the message-independent backstop; the locale
  pin hardens the residual marker path.
- **`runCtx.Err()` FIRST (Windows gotcha, inherited from 084-S).** A context-killed
  child reports a platform-dependent code (often exit 1 on Windows). Check
  `runCtx.Err()` before inspecting the exit code, or a timeout is silently misread as a
  legitimate "not in a work tree" skip — a fail-*open* on the exact path you meant to
  harden.
- **Unborn branch counts as a real work tree.** `git init` with no commit is still a
  work tree (`--is-inside-work-tree` → `true`), so an empty head there fails closed —
  correct, because you still cannot prove lineage.

## Why this does not weaken the guard

- **Refuses unprovable lineage, not provable lineage.** Under a real work tree an empty
  head means the gate has nothing to check `--is-ancestor` against; refusing is the
  only fail-closed choice. Non-empty heads still flow to the 084-S ancestor-aware path
  unchanged.
- **Legitimate empty preserved (SEC-2 CONFIRMED).** Genuine no-repo empty shipment head
  still ships; no-repo empty member head stays skipped; non-enforcement / bare-repo /
  inside-`.git` / forced break-glass paths are unchanged.
- **Bootstrapping proof.** 085-S closed its own shipment with a binary built from merged
  `main`: its members carry provable (non-empty, ancestor) lineage, so the new
  fail-closed branches stayed dormant and the ancestor-aware path admitted the closure.
  (The feature artifact `085-F` is exempt from lineage validation by artifact type —
  only `task`/`subtask` members are lineage-checked — so its absent gate head_sha is
  irrelevant to the guard.)
  End-to-end proof the hardening closes the fail-open holes without a fail-shut
  regression.

## Related

- `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md` — the 084-S
  companion. That fix explicitly scoped OUT the empty-member-head bypass and the
  empty-shipment-head fail-open as "separate, separately-reasoned seams"; 085-S closes
  exactly those seams.
- `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md` — the bounded-helper
  DoS lesson reused by `boundedHelperTimeout()` on this probe.
- `docs/closure/2026-07-07-085-S-adversarial-review.md` — F1 BLOCK → fix → RE-REVIEW
  PASS, N1/N2 remediation, SEC-1/SEC-2 confirmation (3 models).
- `docs/closure/2026-07-07-085-S-feature-pr-runtime-verification.md` — real-subprocess
  behavioral matrix (refuse-in-repo / ship-no-repo / skip-no-repo-member /
  fail-closed-broken-repo / fail-closed-expired-ctx).
- `docs/closure/2026-07-07-085-S-post-merge-closure.md` — closure + bootstrapping proof.
