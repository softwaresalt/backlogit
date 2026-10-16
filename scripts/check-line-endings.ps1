<#
.SYNOPSIS
Fail-closed line-ending guard over the fixed tracked *.go / *.json set.

.DESCRIPTION
Asserts three independent properties and exits 1 if any fails, echoing the
offending paths:
  1. .gitattributes contract: *.go, *.json and the golden JSON are normalized to eol=lf.
  2. Working tree: no enumerated file contains CRLF bytes.
  3. Stored index blob: each enumerated path's blob is LF-normalized, independent
     of .gitattributes.
The tracked set comes from `git ls-files -- '*.go' '*.json'` (never an
attribute-driven query), so deleting or narrowing .gitattributes cannot make
the guard vacuous. Run from the repository root or pass -RepoRoot.

Exit codes: 0 all assertions hold; 1 violation; 2 invalid environment.
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = (Get-Location).Path
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$goldenJson = 'internal/faultline/testdata/parity_v1.golden.json'

function Invoke-GitBytes([string[]]$GitArgs, [int[]]$OkExit = @(0)) {
    $psi = [System.Diagnostics.ProcessStartInfo]::new('git')
    $psi.WorkingDirectory = $RepoRoot
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    foreach ($a in $GitArgs) { $psi.ArgumentList.Add($a) }
    $p = [System.Diagnostics.Process]::Start($psi)
    $ms = [System.IO.MemoryStream]::new()
    $errTask = $p.StandardError.ReadToEndAsync()
    $p.StandardOutput.BaseStream.CopyTo($ms)
    $p.WaitForExit()
    if ($OkExit -notcontains $p.ExitCode) {
        [Console]::Error.WriteLine("LINE-ENDINGS-FAIL: git $($GitArgs -join ' ') failed: $($errTask.Result)")
        exit 2
    }
    return , $ms.ToArray()
}

function Test-HasCrlf([byte[]]$Bytes) {
    for ($i = 0; $i -lt $Bytes.Length - 1; $i++) {
        if ($Bytes[$i] -eq 13 -and $Bytes[$i + 1] -eq 10) { return $true }
    }
    return $false
}

$listing = [System.Text.Encoding]::UTF8.GetString((Invoke-GitBytes @('ls-files', '-z', '-s', '--', '*.go', '*.json')))
$entries = @()
foreach ($rec in ($listing -split "`0")) {
    if ($rec.Length -eq 0) { continue }
    $meta, $path = $rec -split "`t", 2
    $entries += [pscustomobject]@{ Oid = ($meta -split ' ')[1]; Path = $path }
}
if ($entries.Count -eq 0) {
    [Console]::Error.WriteLine('LINE-ENDINGS-FAIL: no tracked *.go/*.json files enumerated')
    exit 2
}

$failures = [System.Collections.Generic.List[string]]::new()

# 1. .gitattributes contract
$attrPath = Join-Path $RepoRoot '.gitattributes'
$attrLines = @()
if (Test-Path -LiteralPath $attrPath) {
    $attrLines = @(Get-Content -LiteralPath $attrPath | ForEach-Object { $_.Trim() } | Where-Object { $_ -and -not $_.StartsWith('#') })
}
foreach ($pattern in @('*.go', '*.json', $goldenJson)) {
    $ok = $false
    foreach ($line in $attrLines) {
        $parts = $line -split '\s+'
        if ($parts[0] -eq $pattern -and ($parts[1..($parts.Count - 1)] -contains 'eol=lf')) { $ok = $true }
    }
    if (-not $ok) { $failures.Add("ATTRIBUTE-CONTRACT: .gitattributes lacks 'eol=lf' rule for $pattern") }
}

foreach ($e in $entries) {
    # 2. working tree bytes
    $full = Join-Path $RepoRoot $e.Path
    if (Test-Path -LiteralPath $full -PathType Leaf) {
        if (Test-HasCrlf ([System.IO.File]::ReadAllBytes($full))) { $failures.Add("WORKTREE-CRLF: $($e.Path)") }
    }
}

# 3. stored index blobs, independent of .gitattributes (git grep exit 1 = no match)
$crlfBlobs = [System.Text.Encoding]::UTF8.GetString((Invoke-GitBytes @('grep', '--cached', '-l', '-z', '-P', '\r$', '--', '*.go', '*.json') @(0, 1)))
foreach ($p in ($crlfBlobs -split "`0")) {
    if ($p.Length -gt 0) { $failures.Add("BLOB-CRLF: $p") }
}

if ($failures.Count -gt 0) {
    foreach ($f in $failures) { [Console]::Error.WriteLine("LINE-ENDINGS-FAIL: $f") }
    exit 1
}
Write-Output "LINE-ENDINGS-OK: $($entries.Count) tracked files"
exit 0
