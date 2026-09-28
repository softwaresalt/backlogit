---
title: "Autoharness upstream fix proposals from the 2026-09-27 tune review"
doc_type: "decision"
source: "docs/decisions/2026-09-27-autoharness-upstream-fix-proposals.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: "Upstream fix proposals for autoharness 1.5.0 surfaced by the backlogit tune review: release_lock.ps1 root, containment, and stderr fixes; the multi-line security-pattern substitution defect; and the release check-then-delete race"
topic: "Autoharness file-lock scripts and security template rendering"
depth: "deep"
decision_status: "proposed"
promoted_to: "upstream"
linked_artifacts:
  - ".autoharness/tuning-reports/2026-09-27-tuning-report.md"
  - "docs/memory/2026-09-27/autoharness-tune-memory.md"
  - "scripts/acquire_lock.ps1"
  - "scripts/release_lock.ps1"
---

# Autoharness Upstream Fix Proposals (2026-09-27 Tune Review)

This document records three defect classes found while reviewing the
autoharness 1.5.0 tune output in backlogit. Each section states the defect, the
evidence, the local backlogit disposition, and the proposed upstream fix.
Template paths are relative to the autoharness `data/templates/` directory.

| # | Defect | Severity | Local status |
|---|---|---|---|
| 1 | `release_lock.ps1` default root, containment, and exit codes | P1 | Fixed locally, `drift_allowed` in the manifest |
| 2 | Multi-line security pattern lists substituted inside inline code | P2 | Fixed in the rendered artifacts only |
| 3 | Race between the release ownership recheck and the delete | P2 | Not fixed; upstream design change required |

## 1. Lock Script Fixes

### 1.1 P1: Release Resolves a Different Default Root Than Acquire

**Defect.** Without `-WorkspaceRoot`, `acquire_lock.ps1` anchors relative paths
to the git top-level, and only trusts it when the script lives in
`<root>\scripts`. `release_lock.ps1` resolved relative paths against the current
directory instead. Acquiring `logs\x.txt` from the root and then releasing it
from `docs\` looked for `docs\logs\.x.txt.lock`, reported "No lock file found",
and exited 0. The lock was left behind even though the release looked
successful.

**Fix.** Release uses the same trusted-root derivation as acquire:

```powershell
function Get-AutoharnessDefaultWorkspaceRoot {
    $scriptDir = $PSScriptRoot
    if (-not $scriptDir) { return $null }
    try {
        $gitOutput = & git -C $scriptDir rev-parse --show-toplevel 2>$null
        if ($LASTEXITCODE -ne 0 -or -not $gitOutput) { return $null }
        $realGitTopLevel = Get-AutoharnessRealPath $gitOutput.Trim()
        $realScriptDir = Get-AutoharnessRealPath $scriptDir
        $expectedScriptsDir = Join-Path $realGitTopLevel 'scripts'
        if ($expectedScriptsDir.Equals($realScriptDir, $autoharnessPathComparisonMode)) {
            return $realGitTopLevel
        }
    }
    catch { return $null }
    return $null
}
```

Relative `FilePath` values are joined to that root before resolution. When no
trusted root exists, release warns and falls back to the current directory,
which is the old behavior. Upstream should extract this function into a shared
dot-sourced helper, so acquire and release cannot drift apart again.

### 1.2 P2: Release Had No Containment Boundary

**Defect.** Acquire rejects targets outside the workspace root. Release did not.
With `-Force`, release would delete any `.<name>.lock` it could reach, including
paths outside the workspace.

**Fix.** Once `$lockFile` is resolved, release applies the same boundary:

* If a root is known and `$lockFile` does not start with `<root><separator>`,
  release fails with `PATH_ESCAPE` and exit 1.
* If no root is known and `-Force` is supplied, release refuses. It will not
  break a lock without a containment boundary.

The comparison uses the same per-root helper as acquire
(`Get-AutoharnessContainmentComparisonMode`). It reads the root parent's
per-directory case-sensitivity attribute from `fsutil` and falls back to
`OrdinalIgnoreCase` on Windows and `Ordinal` elsewhere. An OS-wide
`OrdinalIgnoreCase` check is not enough. Under a case-sensitive parent with
sibling `ws` and `WS` directories, a lock under `WS` passes a prefix check
for root `ws`. This was reproduced in PR #454: the pre-fix script deleted
`WS\.t.txt.lock` with `-Force`, and the fixed script refuses with
`PATH_ESCAPE`. Upstream should put the helper in one shared file that both
scripts load, rather than keeping two copies.

`release_lock.sh` had the same gap. With `--workspace-root` omitted, it applied
no containment check at all. The local fix derives the same trusted default
root as `acquire_lock.sh`, anchors relative paths to it, and rejects lock
paths outside it with `PATH_ESCAPE`. It refuses `--force` when no root
resolves. Under Git Bash on Windows, `git rev-parse --show-toplevel` returns
`C:/...` while `pwd -P` returns `/c/...`. The derived-root check therefore
fails in both scripts there, and callers must pass `--workspace-root`.
Upstream should normalize both forms before comparing them.

### 1.3 P3: `Write-Error` Before `exit 1` Made Failures Unreliable

**Defect.** The failure paths followed the pattern `Write-Error "..."; exit 1`.
When the caller has set `$ErrorActionPreference = 'Stop'`, which agent wrappers
and CI shells commonly do, `Write-Error` becomes a terminating error and
`exit 1` never runs. The caller gets an exception instead of exit code 1, and
`$LASTEXITCODE` keeps whatever value the previous native command left. Output
is also mixed into the error stream with a PowerShell error-record prefix,
which makes it harder to match the error token.

**Fix.** Every such site writes directly to stderr and then exits:

```powershell
[Console]::Error.WriteLine("autoharness-file-lock: PATH_ESCAPE -- lock path '$lockFile' is outside the workspace root '$normalizedRoot'; refusing to release.")
exit 1
```

This covers 7 sites in acquire and 7 in release.

**Test requirement.** The first local pass of this fix lost the message argument
at 11 sites, which left bare `[Console]::Error.WriteLine()` calls behind. It was
caught during this review and fixed. The upstream fix should include a test
that runs each failure path and asserts all three outcomes:

1. The exit code is 1.
2. Stderr contains the expected error code token, such as `TOKEN_MALFORMED`,
   `PATH_ESCAPE`, or "ownership could not be verified".
3. Stdout contains no false success message.

Run the test under Windows PowerShell 5.1 and PowerShell 7.

## 2. Security Pattern List Substitution Defect

### 2.1 Defect

The install-harness skill defines four security variables as **block content**,
meaning bullet lists or rule sets derived from the detected stack
(`.github/skills/install-harness/SKILL.md`, lines 344-354):

* `{{SECURITY_SCAN_PATTERNS}}`
* `{{SECURITY_OWASP_PATTERNS}}`
* `{{SECURITY_CONFIG_RULES}}`
* `{{SECURITY_REVIEW_PATTERNS}}`

Several templates place these variables **inline**, inside backticks or table
cells:

| Template | Line | Usage |
|---|---|---|
| `agents/security-sentinel.agent.md.tmpl` | 32 | ``Language-specific detection uses `{{SECURITY_SCAN_PATTERNS}}`.`` |
| `agents/security-sentinel.agent.md.tmpl` | 55 | ``Apply injection detection patterns from `{{SECURITY_SCAN_PATTERNS}}` `` |
| `agents/security-sentinel.agent.md.tmpl` | 79 | ``...OWASP Top 10 categories using `{{SECURITY_SCAN_PATTERNS}}`:`` |
| `skills/review/SKILL.md.tmpl` | 161 | ``...or files matching `{{SECURITY_REVIEW_PATTERNS}}` `` |
| `skills/security-audit/SKILL.md.tmpl` | 40-41 | Variable table cells for `SECURITY_CONFIG_RULES` and `SECURITY_OWASP_PATTERNS` |
| `skills/security-audit/SKILL.md.tmpl` | 64, 91 | ``Rules from `{{SECURITY_CONFIG_RULES}}`:`` and ``...using `{{SECURITY_OWASP_PATTERNS}}`:`` |

`agents/review/security-reviewer.agent.md.tmpl` line 30 already does it
correctly: the variable sits alone on its own line, so the list renders as a
block.

### 2.2 Rendered Result

The backlogit render at commit `2e216571` shows the failure:

```text
Language-specific detection uses `- Path traversal (.., symlink following)
- SQL injection in dynamic queries
- Command injection in shell invocations
...
```

The code span opens on one line and closes several lines later. CommonMark may
render the whole list as a single run of inline code, or leave a stray backtick
that swallows the text after it. In `security-audit/SKILL.md`, each list item
became its own line inside a table cell, which split the variable table into
malformed rows. Markdownlint did not catch any of this, because the enforced
P-008 rule set (MD001, MD025, MD041) has no code-span or table-shape rule.

### 2.3 Proposed Upstream Fix

1. **Put block variables at block level.** Rewrite each inline site to the
   pattern that `security-reviewer` already uses:

   ```markdown
   Language-specific detection uses these patterns:

   {{SECURITY_SCAN_PATTERNS}}
   ```

   Later references use prose, for example "Apply the injection detection
   patterns listed in Scope above", and do not repeat the substitution.
2. **Stop substituting multi-line values into table cells.** In the
   security-audit variable table, either:
   * show the variable name without braces and point to the section where the
     rendered list appears; or
   * add an inline render form, for example `{{SECURITY_OWASP_PATTERNS|inline}}`,
     that joins list items with `; `.

   The backlogit local fix used the `; ` join by hand.
3. **Declare the value shape.** Add `shape: block | inline` to the variable
   catalog. A template lint in autoharness CI then fails any template that uses
   a `block` variable inside backticks, in a table cell, or mid-sentence.
4. **Add a verify-workspace targeted check.** Flag rendered markdown where an
   inline code span crosses a newline, for example the regex
   ``(?m)`[^`\n]*\n\s*- ``, or where a table row's cell count differs from the
   header row's. This catches the defect in already-installed workspaces.

Local status: the rendered `security-sentinel.agent.md`, `review/SKILL.md`, and
`security-audit/SKILL.md` were fixed by hand and marked `drift_allowed`. They
will regress on the next re-render until the templates change.

## 3. Release Check-Then-Delete Race

### 3.1 Defect

On the token-verified path, both release scripts do this:

1. Read the lock file and verify the token digest against `owner_digest`.
2. Re-read the lock file, the "Round-9 recheck", and confirm `owner_digest` is
   unchanged.
3. Delete the file **by path**: `Remove-Item -LiteralPath $lockFile -Force`
   (PowerShell) or `rm -f "$LOCKFILE"` (POSIX).

Nothing ties step 3 to the file checked in step 2. Suppose the following
happens between steps 2 and 3:

* another process force-breaks or releases the lock;
* a third process then acquires a new lock at the same path.

Step 3 then deletes the new owner's lock. The recheck makes the window smaller
but cannot close it. The script comments admit this ("absent an atomic
compare-and-delete filesystem primitive, cannot fully eliminate"). The
`Test-Path` check before the recheck read has the same problem in a smaller
form.

The locks are advisory, and the O3 note says they are not an adversarial
guarantee. Even so, a legitimate multi-agent session can hit this window when
one agent releases while another is polling to acquire. The failure is silent:
the new owner believes it still holds a lock that no longer exists.

### 3.2 Proposed Upstream Fix (Windows): Delete by Handle

Windows can delete through the same handle the script used to verify the file.
Open the lock once, then read, verify, and delete through that handle:

1. Call `CreateFile` with:
   * access `GENERIC_READ | DELETE`;
   * share mode `FILE_SHARE_READ` only (without `FILE_SHARE_WRITE` or
     `FILE_SHARE_DELETE`);
   * `OPEN_EXISTING`;
   * `FILE_FLAG_OPEN_REPARSE_POINT`, so a symlink swapped in at the path is not
     followed.

   While the handle is open, no other process can modify, rename, or delete the
   file. Because the path stays occupied, acquire's create-new also fails.
2. Read the lock content **through that handle** and verify `owner_digest`
   against the supplied token. This single read replaces both the first read
   and the Round-9 recheck.
3. If the digest matches, call `SetFileInformationByHandle` with
   `FileDispositionInfo` (class 4) and `DeleteFile = TRUE`, then close the
   handle. The file deleted is exactly the one that was verified.
4. If the digest does not match, close the handle without setting the delete
   disposition and exit 1.

```csharp
[StructLayout(LayoutKind.Sequential)]
public struct FILE_DISPOSITION_INFO { [MarshalAs(UnmanagedType.U1)] public bool DeleteFile; }

[DllImport("kernel32.dll", SetLastError = true)]
private static extern bool SetFileInformationByHandle(
    SafeFileHandle hFile, int fileInformationClass,
    ref FILE_DISPOSITION_INFO info, uint bufferSize);

private const uint DELETE = 0x00010000;
private const uint FILE_FLAG_OPEN_REPARSE_POINT = 0x00200000;
private const int FileDispositionInfo = 4;
```

This fits next to the existing `AutoharnessFileLockPathResolver` P/Invoke type.

**Why not `FileOptions.DeleteOnClose`?** A `DeleteOnClose` handle always deletes
the file when it closes. It cannot be cancelled after a digest mismatch. It also
has to be requested at open time, before the content has been verified. Setting
the disposition by handle deletes only after verification succeeds.

`-Force` should use the same handle-bound delete without the digest comparison,
so a forced break also removes only the file it opened.

### 3.3 Proposed Upstream Fix (POSIX)

POSIX has no portable delete-by-descriptor call. `unlinkat` removes a name, not
a verified inode. Two options:

* **Short critical-section mutex (preferred).** Acquire and release both wrap
  their create or verify-and-delete step in an atomic `mkdir` of a sidecar
  directory, `.<name>.lock.mutex`. They wait a bounded time, and they treat a
  mutex older than a small timeout as stale. `mkdir` is atomic on every POSIX
  filesystem and needs no `flock(1)`, which macOS lacks by default.
* **Inode check (narrower).** Record the inode with `stat` at verification time
  and compare it again just before `rm`. This leaves a small window but catches
  most same-path replacements.

In backlogit, `.sh` script gaps are accepted technical debt (TUNE-015), so the
POSIX fix has lower local priority. It still matters for upstream consumers on
Linux and macOS.

### 3.4 Regression Test

The scripts already provide the `AUTOHARNESS_TEST_RELEASE_RACE_DELAY_MS` and
`AUTOHARNESS_TEST_RELEASE_RACE_SIGNAL` hooks. With the handle-bound design, move
the delay so it runs while the handle is held, then assert both of the
following:

1. A concurrent force-break or replacement attempted during the delay fails with
   a sharing violation.
2. The original release still deletes only the lock it verified and exits 0.

Under the current design, the same test shows the new owner's lock being
deleted when the swap lands between the recheck and the delete.
