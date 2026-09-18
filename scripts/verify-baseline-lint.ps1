#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Verifies an authorized baseline-lint convergence residual exactly.
.DESCRIPTION
    Runs repository-wide golangci-lint in structured-output mode and compares
    every normalized finding with a committed, digest-bound inventory. The only
    accepted residual findings are rows owned by shipment tasks not yet done.
    The script is read-only and preserves native/configuration/parser failures.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$Inventory,
    [Parameter(Mandatory)][ValidatePattern('^[0-9a-f]{64}$')][string]$InventorySha256,
    [Parameter(Mandatory)][ValidatePattern('^\d+-S$')][string]$Shipment,
    [Parameter(Mandatory)][ValidatePattern('^\d+\.\d{3}-T$')][string]$TerminalTask,
    [string]$QueueDir = '.backlogit/queue'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Stop-Gate {
    param([int]$Code, [string]$Message)
    Write-Output $Message
    exit $Code
}

function Resolve-ContainedFile {
    param([string]$Root, [string]$Relative, [switch]$Directory)
    if ([System.IO.Path]::IsPathRooted($Relative) -or $Relative -match '(^|[\\/])\.\.([\\/]|$)') {
        Stop-Gate 81 "BASELINE-CONTRACT-FAIL:uncontained-path:$Relative"
    }
    $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath((Join-Path $rootFull $Relative))
    if (-not $full.StartsWith("$rootFull$([System.IO.Path]::DirectorySeparatorChar)",
            [System.StringComparison]::OrdinalIgnoreCase)) {
        Stop-Gate 81 "BASELINE-CONTRACT-FAIL:uncontained-path:$Relative"
    }
    $pathType = if ($Directory) { 'Container' } else { 'Leaf' }
    if (-not (Test-Path $full -PathType $pathType)) {
        Stop-Gate 81 "BASELINE-CONTRACT-FAIL:missing-file:$Relative"
    }
    $cursor = $rootFull
    foreach ($segment in ($Relative -split '[\\/]')) {
        $cursor = Join-Path $cursor $segment
        $item = Get-Item $cursor -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            Stop-Gate 81 "BASELINE-CONTRACT-FAIL:reparse-point:$Relative"
        }
    }
    return $full
}

function Get-FrontmatterScalar {
    param([string]$Path, [string]$Name)
    $inFrontmatter = $false
    foreach ($line in Get-Content $Path) {
        if ($line -ceq '---') {
            if (-not $inFrontmatter) { $inFrontmatter = $true; continue }
            break
        }
        if ($inFrontmatter -and $line -match "^$([regex]::Escape($Name)):\s*(\S+)\s*$") {
            return $Matches[1]
        }
    }
    return ''
}

function Get-ShipmentItems {
    param([string]$Path)
    $items = @()
    $inFrontmatter = $false
    $inCustomFields = $false
    $inItems = $false
    foreach ($line in Get-Content $Path) {
        if ($line -ceq '---') {
            if (-not $inFrontmatter) { $inFrontmatter = $true; continue }
            break
        }
        if (-not $inFrontmatter) { continue }
        if ($line -match '^custom_fields:\s*$') { $inCustomFields = $true; continue }
        if ($inCustomFields -and $line -match '^\s{4}items:\s*$') { $inItems = $true; continue }
        if ($inItems -and $line -match '^\s{8}-\s+(\S+)\s*$') { $items += $Matches[1]; continue }
        if ($inItems -and $line -match '^\s{0,7}\S') { break }
    }
    return $items
}

function Get-FindingKey {
    param($Finding)
    $path = ('' + $Finding.path) -replace '\\', '/'
    return @(
        $path,
        [string]([int]$Finding.line),
        [string]([int]$Finding.column),
        '' + $Finding.linter,
        '' + $Finding.message
    ) -join '|'
}

$repoRoot = (& git rev-parse --show-toplevel)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($repoRoot)) {
    Stop-Gate 80 'BASELINE-NATIVE-FAIL:git-root'
}
$repoRoot = $repoRoot.Trim()
$inventoryPath = Resolve-ContainedFile -Root $repoRoot -Relative $Inventory
$queuePath = Resolve-ContainedFile -Root $repoRoot -Relative $QueueDir -Directory
$shipmentPath = Resolve-ContainedFile -Root $repoRoot -Relative (Join-Path $QueueDir "$Shipment.md")

& git -C $repoRoot ls-files --error-unmatch -- $Inventory 2>$null | Out-Null
if ($LASTEXITCODE -ne 0) { Stop-Gate 81 'BASELINE-CONTRACT-FAIL:inventory-not-tracked' }
& git -C $repoRoot diff --quiet HEAD -- $Inventory
if ($LASTEXITCODE -ne 0) { Stop-Gate 81 'BASELINE-CONTRACT-FAIL:inventory-not-committed' }
& git -C $repoRoot diff --cached --quiet HEAD -- $Inventory
if ($LASTEXITCODE -ne 0) { Stop-Gate 81 'BASELINE-CONTRACT-FAIL:inventory-not-committed' }
$actualDigest = (Get-FileHash $inventoryPath -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actualDigest -cne $InventorySha256) { Stop-Gate 81 'BASELINE-CONTRACT-FAIL:stale-digest' }

try {
    $inventoryJson = Get-Content $inventoryPath -Raw |
        ConvertFrom-Json -NoEnumerate -DateKind String -ErrorAction Stop
}
catch { Stop-Gate 82 'BASELINE-PARSER-FAIL:inventory-json' }
$inventoryRows = @(
    if ($inventoryJson -is [System.Array]) { $inventoryJson }
    elseif ($inventoryJson.PSObject.Properties.Name -contains 'findings') { $inventoryJson.findings }
)
if ($inventoryRows.Count -eq 0) { Stop-Gate 82 'BASELINE-PARSER-FAIL:empty-inventory' }

$shipmentItems = @(Get-ShipmentItems $shipmentPath)
$taskIDs = @()
$taskIDSet = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
$statusByTask = @{}
foreach ($id in $shipmentItems) {
    if ($id -notmatch '^[A-Za-z0-9.-]+$') { Stop-Gate 81 'BASELINE-CONTRACT-FAIL:member-id' }
    $taskRelative = (Join-Path $QueueDir "$id.md") -replace '\\', '/'
    $taskPath = Resolve-ContainedFile -Root $repoRoot -Relative $taskRelative
    if ((Get-FrontmatterScalar -Path $taskPath -Name 'artifact_type') -cne 'task') { continue }
    $resolvedID = Get-FrontmatterScalar -Path $taskPath -Name 'id'
    if ($resolvedID -cne $id) { Stop-Gate 81 "BASELINE-CONTRACT-FAIL:task-id:$id" }
    $taskIDs += $id
    [void]$taskIDSet.Add($id)
    $status = Get-FrontmatterScalar -Path $taskPath -Name 'status'
    if ($status -cnotmatch '^(queued|active|blocked|done)$') {
        Stop-Gate 81 "BASELINE-CONTRACT-FAIL:unsupported-member-status:${id}:$status"
    }
    $statusByTask[$id] = $status
}
if (-not $taskIDSet.Contains($TerminalTask)) { Stop-Gate 81 'BASELINE-CONTRACT-FAIL:terminal-task' }

$baselineByKey = [System.Collections.Generic.Dictionary[string,string]]::new(
    [System.StringComparer]::Ordinal
)
foreach ($row in $inventoryRows) {
    $line = 0
    $column = 0
    $required = @('path', 'line', 'column', 'linter', 'message', 'owner_task_id')
    if (@($required | Where-Object { $row.PSObject.Properties.Name -cnotcontains $_ }).Count -gt 0 -or
        -not [int]::TryParse("$($row.line)", [ref]$line) -or
        -not [int]::TryParse("$($row.column)", [ref]$column)) {
        Stop-Gate 82 'BASELINE-PARSER-FAIL:malformed-finding'
    }
    $normalized = [pscustomobject]@{
        path = '' + $row.path
        line = $line
        column = $column
        linter = '' + $row.linter
        message = '' + $row.message
    }
    $key = Get-FindingKey $normalized
    $owner = '' + $row.owner_task_id
    if ([string]::IsNullOrWhiteSpace($normalized.path) -or $line -lt 1 -or
        $column -lt 1 -or [string]::IsNullOrWhiteSpace($normalized.linter) -or
        [string]::IsNullOrWhiteSpace($normalized.message) -or -not $taskIDSet.Contains($owner) -or
        $baselineByKey.ContainsKey($key)) {
        Stop-Gate 82 "BASELINE-PARSER-FAIL:malformed-or-duplicate:$key"
    }
    $baselineByKey[$key] = $owner
}

$psi = [System.Diagnostics.ProcessStartInfo]::new()
$psi.FileName = 'golangci-lint'
$psi.ArgumentList.Add('run')
$psi.ArgumentList.Add('--output.json.path')
$psi.ArgumentList.Add('stdout')
$psi.ArgumentList.Add('--output.text.path')
$psi.ArgumentList.Add('stderr')
$psi.ArgumentList.Add('--show-stats=false')
$psi.ArgumentList.Add('--max-issues-per-linter')
$psi.ArgumentList.Add('0')
$psi.ArgumentList.Add('--max-same-issues')
$psi.ArgumentList.Add('0')
$psi.ArgumentList.Add('--uniq-by-line=false')
$psi.ArgumentList.Add('--issues-exit-code')
$psi.ArgumentList.Add('0')
$psi.WorkingDirectory = $repoRoot
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.UseShellExecute = $false
try { $process = [System.Diagnostics.Process]::Start($psi) }
catch { Stop-Gate 83 'BASELINE-NATIVE-FAIL:launch' }
$stdoutTask = $process.StandardOutput.ReadToEndAsync()
$stderrTask = $process.StandardError.ReadToEndAsync()
$process.WaitForExit()
$stdout = $stdoutTask.GetAwaiter().GetResult()
$stderr = $stderrTask.GetAwaiter().GetResult()
if ($process.ExitCode -ne 0) {
    Write-Output $stderr
    Stop-Gate 83 "BASELINE-NATIVE-FAIL:exit-$($process.ExitCode)"
}
if ([string]::IsNullOrWhiteSpace($stdout)) { Stop-Gate 84 'BASELINE-PARSER-FAIL:empty-output' }
try { $lintJson = $stdout | ConvertFrom-Json -ErrorAction Stop }
catch { Stop-Gate 84 'BASELINE-PARSER-FAIL:lint-json' }
if ($null -eq $lintJson -or $lintJson -isnot [System.Management.Automation.PSCustomObject] -or
    -not ($lintJson.PSObject.Properties.Name -contains 'Issues') -or
    ($null -ne $lintJson.Issues -and $lintJson.Issues -isnot [System.Array])) {
    Stop-Gate 84 'BASELINE-PARSER-FAIL:lint-schema'
}

$observed = @()
foreach ($issue in @($lintJson.Issues)) {
    $line = 0
    $column = 0
    if ($null -eq $issue.Pos -or
        -not [int]::TryParse("$($issue.Pos.Line)", [ref]$line) -or
        -not [int]::TryParse("$($issue.Pos.Column)", [ref]$column)) {
        Stop-Gate 84 'BASELINE-PARSER-FAIL:lint-position'
    }
    $candidate = [pscustomobject]@{
        path = ('' + $issue.Pos.Filename) -replace '\\', '/'
        line = $line
        column = $column
        linter = '' + $issue.FromLinter
        message = '' + $issue.Text
    }
    $key = Get-FindingKey $candidate
    if (-not $baselineByKey.ContainsKey($key)) {
        Stop-Gate 85 "BASELINE-RESIDUAL-MISMATCH:new-or-changed:$key"
    }
    $observed += "$key|$($baselineByKey[$key])"
}
if (@($observed | Sort-Object -Unique).Count -ne $observed.Count) {
    Stop-Gate 85 'BASELINE-RESIDUAL-MISMATCH:duplicate-observation'
}

$expected = @(
    foreach ($row in $inventoryRows) {
        $owner = '' + $row.owner_task_id
        if ($statusByTask[$owner] -cin @('queued', 'active', 'blocked')) {
            "$(Get-FindingKey $row)|$owner"
        }
    }
) | Sort-Object
$actual = @($observed | Sort-Object)
if (($expected -join "`n") -cne ($actual -join "`n")) {
    Stop-Gate 85 'BASELINE-RESIDUAL-MISMATCH:expected-set'
}

Write-Output "BASELINE-LINT-RESIDUAL-OK:${Shipment}:$($actual.Count)"
exit 0
