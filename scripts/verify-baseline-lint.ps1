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

function Test-NonNegInt($Value) {
    if ($null -eq $Value -or $Value -is [bool] -or $Value -is [string]) { return $false }
    if ($Value -isnot [int] -and $Value -isnot [long] -and $Value -isnot [double] -and $Value -isnot [decimal] -and $Value -isnot [System.Numerics.BigInteger]) { return $false }
    if ($Value -is [double] -and ([double]::IsNaN($Value) -or [double]::IsInfinity($Value))) { return $false }
    if ($Value -lt 0 -or $Value -gt 2147483647) { return $false }
    return ($Value -eq [math]::Truncate([decimal]$Value))
}
function Get-Fingerprint($Item) {
    $msg = ([string]$Item.message).Normalize([System.Text.NormalizationForm]::FormC)
    $msg = ($msg -replace '\s+', ' ').Trim()
    return (ConvertTo-Json -InputObject @($Item.path, $Item.linter, $msg) -Compress)
}

function Test-DupKey([System.Text.Json.JsonElement]$El) {
    if ($El.ValueKind -eq [System.Text.Json.JsonValueKind]::Object) {
        $seen = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
        foreach ($p in $El.EnumerateObject()) {
            if (-not $seen.Add($p.Name)) { return $true }
            if (Test-DupKey $p.Value) { return $true }
        }
    }
    elseif ($El.ValueKind -eq [System.Text.Json.JsonValueKind]::Array) {
        foreach ($e in $El.EnumerateArray()) { if (Test-DupKey $e) { return $true } }
    }
    return $false
}

function Read-StrictJson([byte[]]$Bytes) {
    $start = 0
    if ($Bytes.Length -ge 3 -and $Bytes[0] -eq 0xEF -and $Bytes[1] -eq 0xBB -and $Bytes[2] -eq 0xBF) { $start = 3 }
    $text = [System.Text.Encoding]::UTF8.GetString($Bytes, $start, $Bytes.Length - $start)
    if ([string]::IsNullOrWhiteSpace($text)) { Stop-Closed 'malformed inventory: empty' }
    try { $doc = [System.Text.Json.JsonDocument]::Parse($text) }
    catch { Stop-Closed "malformed inventory: $($_.Exception.Message)" }
    try {
        if ($doc.RootElement.ValueKind -ne [System.Text.Json.JsonValueKind]::Object) { Stop-Closed 'malformed inventory: top level must be an object' }
        if (Test-DupKey $doc.RootElement) { Stop-Closed 'malformed inventory: duplicate object keys' }
    }
    finally { $doc.Dispose() }
    $opts = @{}
    if ((Get-Command ConvertFrom-Json).Parameters.ContainsKey('DateKind')) { $opts['DateKind'] = 'String' }
    return (ConvertFrom-Json -InputObject $text @opts)
}

function Assert-Fingerprint($Item, [string]$Where) {
    foreach ($n in 'path', 'linter') {
        $v = $Item.PSObject.Properties[$n]?.Value
        if ($v -isnot [string] -or [string]::IsNullOrWhiteSpace($v)) { Stop-Closed "malformed inventory: $Where $n must be a non-empty string" }
    }
    if ($Item.PSObject.Properties['message']?.Value -isnot [string]) { Stop-Closed "malformed inventory: $Where message must be a string" }
}

function Assert-Id([string]$Name, [string]$Value, [string]$Pattern) {
    if ([string]::IsNullOrWhiteSpace($Value)) { Stop-Closed "$Name is required" }
    if ($Value -cnotmatch $Pattern) { Stop-Closed "invalid $Name '$Value'" }
}

try {
    Assert-Id 'Shipment' $Shipment '^[0-9]+-S\z'
    Assert-Id 'FeatureId' $FeatureId '^[0-9]+-F\z'
    Assert-Id 'TerminalTask' $TerminalTask '^[0-9]+(\.[0-9]+)*-(T|ST)\z'
    if ([string]::IsNullOrWhiteSpace($Inventory)) { Stop-Closed 'Inventory is required' }
    if ($InventorySha256 -cnotmatch '^[0-9A-Fa-f]{64}\z') { Stop-Closed 'InventorySha256 must be 64 hex characters' }
    if (-not (Test-Path -LiteralPath $Inventory -PathType Leaf)) { Stop-Closed "inventory not found: $Inventory" }

    $resolvedInventory = Resolve-Path -LiteralPath $Inventory
    if ($resolvedInventory.Provider.Name -cne 'FileSystem') { Stop-Closed "inventory is not a file-system path: $Inventory" }

    # Single read: the hashed bytes are the exact bytes parsed below. The path is resolved
    # against PowerShell's current location, not the process working directory.
    $inventoryBytes = [System.IO.File]::ReadAllBytes($resolvedInventory.ProviderPath)
    $actualHash = [System.Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($inventoryBytes))
    if ($actualHash -ine $InventorySha256) {
        Stop-Closed "inventory hash mismatch (actual $actualHash)" 1
    }

    $inv = Read-StrictJson $inventoryBytes
    if ($null -eq $inv -or $inv -isnot [pscustomobject] -or
        $null -eq $inv.PSObject.Properties['schema'] -or $inv.schema -isnot [string] -or $inv.schema -cne 'baseline-inventory/v1' -or
        $null -eq $inv.PSObject.Properties['groups'] -or $null -eq $inv.PSObject.Properties['remaining']) {
        Stop-Closed 'malformed inventory: expected schema baseline-inventory/v1 with groups and remaining'
    }

    foreach ($name in 'groups', 'remaining') {
        $list = $inv.PSObject.Properties[$name].Value
        if ($list -isnot [System.Array]) { Stop-Closed "malformed inventory: $name must be an array" }
        foreach ($m in $list) {
            if ($null -eq $m -or $m -isnot [pscustomobject]) { Stop-Closed "malformed inventory: $name members must be objects" }
            Assert-Fingerprint $m $name
        }
    }
    $seenGroups = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($g in $inv.groups) {
        if (-not $seenGroups.Add((Get-Fingerprint $g))) { Stop-Closed 'malformed inventory: duplicate group fingerprint' }
        $owners = $g.PSObject.Properties['owners']
        if ($null -eq $owners -or $owners.Value -isnot [System.Array]) { Stop-Closed 'malformed inventory: group owners must be an array' }
        $seenOwners = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
        foreach ($o in $owners.Value) {
            if ($null -eq $o -or $o -isnot [pscustomobject]) { Stop-Closed 'malformed inventory: owners members must be objects' }
            $ot = $o.PSObject.Properties['task']?.Value
            if ($ot -isnot [string] -or $ot -cnotmatch '^[0-9]+(\.[0-9]+)*-(T|ST)\z') { Stop-Closed 'malformed inventory: owner task must be a task id string' }
            if (-not $seenOwners.Add($ot)) { Stop-Closed 'malformed inventory: duplicate owner task in group' }
            if ($o.PSObject.Properties['completed']?.Value -isnot [bool]) { Stop-Closed 'malformed inventory: owner completed must be a boolean' }
            if (-not (Test-NonNegInt $o.PSObject.Properties['resolves']?.Value)) { Stop-Closed 'malformed inventory: owner resolves must be a non-negative integer' }
        }
        if (-not (Test-NonNegInt $g.PSObject.Properties['baseline_count']?.Value)) { Stop-Closed 'malformed inventory: baseline_count must be a non-negative integer' }
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
