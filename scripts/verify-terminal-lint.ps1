<#
.SYNOPSIS
Terminal zero-warning global runner screening both supported GOOS surfaces.

.DESCRIPTION
Screens the `windows` and `linux` surfaces independently. Prints exactly
TERMINAL-LINT-OK:<FeatureId> only when both surfaces report zero residual
warnings. Both surfaces are always evaluated so a residual on either one fails.
Real golangci-lint is never run and the repository is never mutated.

Fixture injection: -FixtureRoot points at a directory containing windows.json
and linux.json, each:
  { "goos": "<surface>", "findings": [ { "path", "linter", "message", "line", "column" } ] }
A surface is clean iff its findings array is empty. Missing FixtureRoot or a
missing/malformed surface fixture fails closed.

Each fixture goos must equal its surface name (case-sensitive); otherwise fail closed.

Exit codes: 0 both clean; 1 residual warnings on at least one surface;
2 invalid or missing input / precondition.

.PARAMETER FeatureId
Feature id such as 175-F (required).
.PARAMETER FixtureRoot
Directory holding windows.json and linux.json.
#>
[CmdletBinding()]
param(
    [string]$FeatureId,
    [string]$FixtureRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Stop-Closed([string]$Message, [int]$Code = 2) {
    [Console]::Error.WriteLine("TERMINAL-LINT-FAIL: INVALID-INPUT: $Message")
    exit $Code
}

try {
    if ([string]::IsNullOrWhiteSpace($FeatureId)) { Stop-Closed 'FeatureId is required' }
    if ($FeatureId -cnotmatch '^[0-9]+(\.[0-9]+)*-[A-Z]{1,2}$') { Stop-Closed "invalid FeatureId '$FeatureId'" }
    if ([string]::IsNullOrWhiteSpace($FixtureRoot)) { Stop-Closed 'FixtureRoot is required (real lint is never run)' }

    $residual = @{}
    foreach ($goos in @('windows', 'linux')) {
        $path = Join-Path $FixtureRoot "$goos.json"
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { Stop-Closed "missing $goos surface fixture: $path" }
        try {
            $doc = Get-Content -LiteralPath $path -Raw | ConvertFrom-Json
        }
        catch {
            Stop-Closed "malformed $goos fixture: $($_.Exception.Message)"
        }
        if ($null -eq $doc -or $doc -isnot [pscustomobject] -or $null -eq $doc.PSObject.Properties['findings']) {
            Stop-Closed "malformed $goos fixture: findings missing"
        }
        if ($null -eq $doc.PSObject.Properties['goos'] -or $doc.goos -isnot [string] -or $doc.goos -cne $goos) {
            Stop-Closed "malformed $goos fixture: goos must equal '$goos'"
        }
        $list = $doc.findings
        if ($list -isnot [System.Array]) { Stop-Closed "malformed $goos fixture: findings must be an array" }
        foreach ($m in $list) {
            if ($null -eq $m) { Stop-Closed "malformed $goos fixture: findings members must be non-null" }
        }
        $residual[$goos] = $list
    }

    $failed = $false
    foreach ($goos in @('windows', 'linux')) {
        foreach ($f in $residual[$goos]) {
            Write-Output ("SURFACE-RESIDUAL[{0}]: {1} {2} {3}" -f $goos, $f.path, $f.linter, $f.message)
            $failed = $true
        }
    }
    if ($failed) { exit 1 }

    Write-Output "TERMINAL-LINT-OK:$FeatureId"
    exit 0
}
catch {
    [Console]::Error.WriteLine("TERMINAL-LINT-FAIL: INVALID-INPUT: $($_.Exception.Message)")
    exit 2
}
