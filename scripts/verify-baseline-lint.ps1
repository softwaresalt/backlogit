<#
.SYNOPSIS
Intermediate-wave remaining-baseline monotonicity runner.

.DESCRIPTION
Verifies inventory integrity (SHA-256 equals -InventorySha256, case-insensitive)
and remaining-baseline monotonicity under the line-stable-group-multiset/v1
identity model: groups are keyed on (path, linter, normalized_message) with
multiplicity; line and column are never compared.

Inventory fixture (JSON, passed via -Inventory):
  { "schema": "baseline-inventory/v1",
    "groups": [ { "path", "linter", "message", "baseline_count",
                  "owners": [ { "task", "resolves", "completed" } ] } ],
    "remaining": [ { "path", "linter", "message", "line", "column" } ] }
Expected residual per group = baseline_count - sum(resolves of completed owners).
Actual residual = count of matching entries in "remaining". Any difference fails:
actual > expected (unresolved), actual < expected (resolved by no completed
owner), or a remaining fingerprint absent from groups (new fingerprint).
The runner is read-only and never runs real golangci-lint.

Exit codes: 0 monotone (prints BASELINE-LINT-OK:<Shipment>); 1 hash mismatch or
monotonicity violation; 2 missing/malformed inventory or invalid parameters.

.PARAMETER Inventory
Path to the inventory JSON file.
.PARAMETER InventorySha256
Expected SHA-256 (hex) of the inventory file.
.PARAMETER Shipment
Replacement shipment id, e.g. 176-S.
.PARAMETER FeatureId
Feature id, e.g. 175-F.
.PARAMETER TerminalTask
Terminal task id, e.g. 175.012-T.
#>
[CmdletBinding()]
param(
    [string]$Inventory,
    [string]$InventorySha256,
    [string]$Shipment,
    [string]$FeatureId,
    [string]$TerminalTask
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Stop-Closed([string]$Message, [int]$Code = 2) {
    $reason = if ($Code -eq 1) { 'HASH-MISMATCH' } else { 'INVALID-INPUT' }
    [Console]::Error.WriteLine("BASELINE-LINT-FAIL: ${reason}: $Message")
    exit $Code
}

function Get-Fingerprint($Item) {
    $msg = ([string]$Item.message -replace '\s+', ' ').Trim()
    return ('{0}|{1}|{2}' -f $Item.path, $Item.linter, $msg)
}

function Assert-Id([string]$Name, [string]$Value) {
    if ([string]::IsNullOrWhiteSpace($Value)) { Stop-Closed "$Name is required" }
    if ($Value -cnotmatch '^[0-9]+(\.[0-9]+)*-[A-Z]{1,2}$') { Stop-Closed "invalid $Name '$Value'" }
}

try {
    Assert-Id 'Shipment' $Shipment
    Assert-Id 'FeatureId' $FeatureId
    Assert-Id 'TerminalTask' $TerminalTask
    if ([string]::IsNullOrWhiteSpace($Inventory)) { Stop-Closed 'Inventory is required' }
    if ($InventorySha256 -cnotmatch '^[0-9A-Fa-f]{64}$') { Stop-Closed 'InventorySha256 must be 64 hex characters' }
    if (-not (Test-Path -LiteralPath $Inventory -PathType Leaf)) { Stop-Closed "inventory not found: $Inventory" }

    $actualHash = (Get-FileHash -LiteralPath $Inventory -Algorithm SHA256).Hash
    if ($actualHash -ine $InventorySha256) {
        Stop-Closed "inventory hash mismatch (actual $actualHash)" 1
    }

    try {
        $inv = Get-Content -LiteralPath $Inventory -Raw | ConvertFrom-Json
    }
    catch {
        Stop-Closed "malformed inventory: $($_.Exception.Message)"
    }
    if ($null -eq $inv -or $inv -isnot [pscustomobject] -or
        $null -eq $inv.PSObject.Properties['schema'] -or $inv.schema -ne 'baseline-inventory/v1' -or
        $null -eq $inv.PSObject.Properties['groups'] -or $null -eq $inv.PSObject.Properties['remaining']) {
        Stop-Closed 'malformed inventory: expected schema baseline-inventory/v1 with groups and remaining'
    }

    $expected = @{}
    foreach ($g in @($inv.groups)) {
        if ($null -eq $g) { continue }
        $resolved = 0
        foreach ($o in @($g.owners)) {
            if ($null -ne $o -and $o.completed -eq $true) { $resolved += [int]$o.resolves }
        }
        $fp = Get-Fingerprint $g
        $expected[$fp] = [int]$expected[$fp] + [int]$g.baseline_count - $resolved
    }
    $actual = @{}
    foreach ($r in @($inv.remaining)) {
        if ($null -eq $r) { continue }
        $fp = Get-Fingerprint $r
        $actual[$fp] = [int]$actual[$fp] + 1
    }

    $violations = @()
    foreach ($fp in (@($expected.Keys) + @($actual.Keys) | Sort-Object -Unique)) {
        $e = [int]$expected[$fp]
        $a = [int]$actual[$fp]
        if (-not $expected.ContainsKey($fp)) { $violations += "new fingerprint: $fp actual=$a" }
        elseif ($a -gt $e) { $violations += "unresolved: $fp expected=$e actual=$a" }
        elseif ($a -lt $e) { $violations += "resolved by no completed owner: $fp expected=$e actual=$a" }
    }
    if ($violations.Count -gt 0) {
        $violations | ForEach-Object { Write-Output "MONOTONICITY-VIOLATION: $_" }
        exit 1
    }
    Write-Output "BASELINE-LINT-OK:$Shipment"
    exit 0
}
catch {
    [Console]::Error.WriteLine("BASELINE-LINT-FAIL: INVALID-INPUT: $($_.Exception.Message)")
    exit 2
}
