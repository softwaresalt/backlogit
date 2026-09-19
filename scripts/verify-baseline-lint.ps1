#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Verifies the exact lifecycle-aware residual for every supported lint surface.
.DESCRIPTION
    The primary inventory is normally captured on Windows. Secondary surfaces
    reuse common identities, declare primary-only exclusions, and enumerate only
    their additional identities. This keeps one owner per finding without
    pretending one host analyzes every build-tag surface.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$Inventory,
    [Parameter(Mandatory)][ValidatePattern('^[0-9a-f]{64}$')][string]$InventorySha256,
    [Parameter(Mandatory)][ValidatePattern('^\d+-S$')][string]$Shipment,
    [Parameter(Mandatory)][ValidatePattern('^\d+-F$')][string]$FeatureId,
    [Parameter(Mandatory)][ValidatePattern('^\d+\.\d{3}-T$')][string]$TerminalTask
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'lint-runtime.ps1')

function Stop-Baseline {
    param([int]$Code, [string]$Message)
    [Console]::Error.WriteLine($Message)
    exit $Code
}

function Get-ShipmentItems {
    param([string]$Path)
    $items = @()
    $inFrontmatter = $false
    $inCustomFields = $false
    $inItems = $false
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -ceq '---') {
            if (-not $inFrontmatter) { $inFrontmatter = $true; continue }
            break
        }
        if (-not $inFrontmatter) { continue }
        if ($line -match '^custom_fields:\s*$') { $inCustomFields = $true; continue }
        if ($inCustomFields -and $line -match '^\s{4}items:\s*$') {
            $inItems = $true
            continue
        }
        if ($inItems -and $line -match '^\s{8}-\s+(\S+)\s*$') {
            $items += $Matches[1]
            continue
        }
        if ($inItems -and $line -match '^\s{0,7}\S') { break }
    }
    return $items
}

try {
    $root = Get-RepositoryRoot
    $inventoryPath = Resolve-ContainedPath -Root $root -Relative $Inventory
    & git -C $root ls-files --error-unmatch -- $Inventory 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'inventory-not-tracked' }
    & git -C $root diff --quiet HEAD -- $Inventory
    if ($LASTEXITCODE -ne 0) { throw 'inventory-not-committed' }
    & git -C $root diff --cached --quiet HEAD -- $Inventory
    if ($LASTEXITCODE -ne 0) { throw 'inventory-not-committed' }
    $digest = (Get-FileHash -LiteralPath $inventoryPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($digest -cne $InventorySha256) { throw 'inventory-digest' }

    try {
        $inventory = Get-Content -LiteralPath $inventoryPath -Raw |
            ConvertFrom-Json -NoEnumerate -DateKind String -ErrorAction Stop
    }
    catch { throw 'inventory-json' }
    foreach ($name in @(
            'primary_goos', 'supported_goos', 'findings',
            'surface_exclusions', 'additional_findings')) {
        if ($inventory.PSObject.Properties.Name -cnotcontains $name) {
            throw "inventory-schema:$name"
        }
    }
    Assert-PlatformInventoryShape -Inventory $inventory
    $primaryRows = @($inventory.findings)
    $additionalRows = @($inventory.additional_findings)
    if ($primaryRows.Count -lt 1) { throw 'empty-inventory' }

    $shipmentArtifact = Resolve-CanonicalArtifact -Root $root -Id $Shipment
    $featureArtifact = Resolve-CanonicalArtifact -Root $root -Id $FeatureId
    if ((Get-FrontmatterScalar -Path $shipmentArtifact.Path -Name artifact_type) -cne 'shipment' -or
        (Get-FrontmatterScalar -Path $featureArtifact.Path -Name artifact_type) -cne 'feature') {
        throw 'release-artifact-type'
    }
    $shipmentItems = @(Get-ShipmentItems -Path $shipmentArtifact.Path)
    if (@($shipmentItems | Where-Object { $_ -ceq $FeatureId }).Count -ne 1) {
        throw 'feature-not-in-shipment'
    }
    $featureContract = (Get-JsonContractBlock -Path $featureArtifact.Path `
        -Name 'baseline-lint-convergence-contract').Object
    if (('' + $featureContract.shipment_id) -cne $Shipment -or
        ('' + $featureContract.release_unit_id) -cne $FeatureId -or
        ('' + $featureContract.terminal_task_id) -cne $TerminalTask) {
        throw 'release-contract-identity'
    }
    if (-not (Test-OrdinalSetEqual -Expected @('windows', 'linux') `
            -Actual @($featureContract.supported_goos))) {
        throw 'release-contract-platforms'
    }
    $inventoryContract = $featureContract.baseline_inventory
    if ($null -eq $inventoryContract -or
        ('' + $inventoryContract.path) -cne $Inventory -or
        ('' + $inventoryContract.sha256) -cne $InventorySha256 -or
        -not (Test-OrdinalSetEqual `
            -Expected @('path', 'line', 'column', 'linter', 'message', 'owner_task_id') `
            -Actual @($inventoryContract.identity_fields))) {
        throw 'release-contract-inventory-binding'
    }
    $expectedIntermediate = 'pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 ' +
        "-Inventory $Inventory -InventorySha256 $InventorySha256 -Shipment $Shipment " +
        "-FeatureId $FeatureId -TerminalTask $TerminalTask"
    if (('' + $featureContract.intermediate_wave_lint_cmd) -cne $expectedIntermediate -or
        ('' + $featureContract.terminal_global_lint_cmd) -cne
            "pwsh -NoProfile -File scripts/verify-terminal-lint.ps1 -FeatureId $FeatureId") {
        throw 'release-contract-command-binding'
    }

    $statusByTask = @{}
    $ownerSet = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    foreach ($member in @($featureContract.member_scope)) {
        $id = '' + $member.task_id
        if (-not $ownerSet.Add($id)) { throw "duplicate-member:$id" }
        if (@($shipmentItems | Where-Object { $_ -ceq $id }).Count -ne 1) {
            throw "member-not-in-shipment:$id"
        }
        $task = Resolve-CanonicalArtifact -Root $root -Id $id
        if ((Get-FrontmatterScalar -Path $task.Path -Name artifact_type) -cne 'task') {
            throw "member-not-task:$id"
        }
        if ($member.PSObject.Properties.Name -cnotcontains 'task_contract_sha256' -or
            ('' + $member.task_contract_sha256) -cnotmatch '^[0-9a-f]{64}$' -or
            (Get-TaskContractDigest -TaskPath $task.Path) -cne
                ('' + $member.task_contract_sha256)) {
            throw "task-contract-digest:$id"
        }
        $statusByTask[$id] = $task.Status
    }
    $shipmentTasks = @(
        foreach ($id in $shipmentItems) {
            if ($id -eq $FeatureId) { continue }
            $artifact = Resolve-CanonicalArtifact -Root $root -Id $id
            if ((Get-FrontmatterScalar -Path $artifact.Path -Name artifact_type) -ceq 'task') {
                $id
            }
        }
    )
    if (-not (Test-OrdinalSetEqual -Expected @($ownerSet) -Actual $shipmentTasks)) {
        throw 'member-scope-not-exact'
    }
    if (-not $ownerSet.Contains($TerminalTask)) { throw 'terminal-not-member' }

    $rowByKey = [System.Collections.Generic.Dictionary[string,object]]::new(
        [System.StringComparer]::Ordinal)
    foreach ($row in @($primaryRows + $additionalRows)) {
        foreach ($name in @('path', 'line', 'column', 'linter', 'message', 'owner_task_id')) {
            if ($row.PSObject.Properties.Name -cnotcontains $name) {
                throw "finding-schema:$name"
            }
        }
        $owner = '' + $row.owner_task_id
        if (-not $ownerSet.Contains($owner)) { throw "finding-owner:$owner" }
        $key = Get-LintFindingKey -Finding $row
        if (-not $rowByKey.TryAdd($key, $row)) { throw "duplicate-finding:$key" }
    }
    try {
        Assert-InventoryFindingCount -Declared ([int]$inventoryContract.finding_count) `
            -RowsByKey $rowByKey
    }
    catch { throw 'release-contract-finding-count' }
    [void](Assert-GolangCILintVersion -Root $root)
    $residualUnion = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    foreach ($goos in @($inventory.supported_goos)) {
        $exclusions = @($inventory.surface_exclusions.$goos)
        foreach ($key in $exclusions) {
            if (-not $rowByKey.ContainsKey("$key") -or
                @($primaryRows | Where-Object {
                        (Get-LintFindingKey -Finding $_) -ceq "$key"
                    }).Count -ne 1) {
                throw "surface-exclusion:${goos}:$key"
            }
        }
        $expected = @(
            foreach ($row in $primaryRows) {
                $key = Get-LintFindingKey -Finding $row
                if ($exclusions -ccontains $key) { continue }
                if ($statusByTask['' + $row.owner_task_id] -cin $script:ExecutableStatuses) {
                    $key
                }
            }
            foreach ($row in $additionalRows) {
                if (('' + $row.goos) -ceq "$goos" -and
                    $statusByTask['' + $row.owner_task_id] -cin $script:ExecutableStatuses) {
                    Get-LintFindingKey -Finding $row
                }
            }
        )
        $lint = Invoke-StructuredGolangCILint -Root $root -Goos "$goos"
        $actual = @($lint.Issues | ForEach-Object {
                Get-LintFindingKey -Finding $_ -Native
            })
        if (@($actual | Sort-Object -Unique).Count -ne $actual.Count) {
            throw "duplicate-observation:$goos"
        }
        if (-not (Test-OrdinalSetEqual -Expected $expected -Actual $actual)) {
            $new = @($actual | Where-Object { $expected -cnotcontains $_ })
            $missing = @($expected | Where-Object { $actual -cnotcontains $_ })
            throw "residual-mismatch:${goos}:new=$($new.Count):missing=$($missing.Count)"
        }
        foreach ($key in $expected) { [void]$residualUnion.Add("$key") }
    }

    Write-Output "BASELINE-LINT-RESIDUAL-OK:${Shipment}:surfaces=2:residual=$($residualUnion.Count)"
    exit 0
}
catch {
    Stop-Baseline 81 "BASELINE-LINT-FAIL:$($_.Exception.Message)"
}
