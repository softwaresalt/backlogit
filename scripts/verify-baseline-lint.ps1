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

    foreach ($name in 'groups', 'remaining') {
        $list = $inv.PSObject.Properties[$name].Value
        if ($list -isnot [System.Array]) { Stop-Closed "malformed inventory: $name must be an array" }
        foreach ($m in $list) {
            if ($null -eq $m -or $m -isnot [pscustomobject]) { Stop-Closed "malformed inventory: $name members must be objects" }
        }
    }
    foreach ($g in $inv.groups) {
        $owners = $g.PSObject.Properties['owners']
        if ($null -eq $owners -or $owners.Value -isnot [System.Array]) { Stop-Closed 'malformed inventory: group owners must be an array' }
        foreach ($o in $owners.Value) {
            if ($null -eq $o -or $o -isnot [pscustomobject]) { Stop-Closed 'malformed inventory: owners members must be objects' }
        }
    }

    $expected = [System.Collections.Generic.Dictionary[string, int]]::new([StringComparer]::Ordinal)
    foreach ($g in $inv.groups) {
        $resolved = 0
        foreach ($o in $g.owners) {
            if ($o.completed -eq $true) { $resolved += [int]$o.resolves }
        }
        $fp = Get-Fingerprint $g
        $prev = 0
        [void]$expected.TryGetValue($fp, [ref]$prev)
        $expected[$fp] = $prev + [int]$g.baseline_count - $resolved
    }
    $actual = [System.Collections.Generic.Dictionary[string, int]]::new([StringComparer]::Ordinal)
    foreach ($r in $inv.remaining) {
        $fp = Get-Fingerprint $r
        $prev = 0
        [void]$actual.TryGetValue($fp, [ref]$prev)
        $actual[$fp] = $prev + 1
    }

    $keys = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($k in $expected.Keys) { [void]$keys.Add($k) }
    foreach ($k in $actual.Keys) { [void]$keys.Add($k) }
    $sorted = [string[]]@($keys)
    [Array]::Sort($sorted, [StringComparer]::Ordinal)

    $violations = @()
    foreach ($fp in $sorted) {
        $e = 0
        $a = 0
        [void]$expected.TryGetValue($fp, [ref]$e)
        [void]$actual.TryGetValue($fp, [ref]$a)
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
