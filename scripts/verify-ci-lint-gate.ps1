#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Selects the fail-closed repository lint gate for the current lifecycle.
.DESCRIPTION
    Ordinary repositories run the pinned zero-warning global lint command.
    One unfinished baseline-convergence release unit runs its canonical exact
    residual verifier until all finding-remediation members are complete, then
    runs the canonical terminal zero-warning verifier.
#>
[CmdletBinding()]
param(
    [string]$RepositoryRoot = '',
    [string]$GolangCILintExecutable = 'golangci-lint',
    [switch]$PlanOnly
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'lint-runtime.ps1')

function Stop-CILintGate {
    param([string]$Message)
    [Console]::Error.WriteLine("CI-LINT-GATE-FAIL:$Message")
    exit 81
}

function Assert-TrackedCommittedFile {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$Relative
    )
    $path = Resolve-ContainedPath -Root $Root -Relative $Relative
    & git -C $Root ls-files --error-unmatch -- $Relative 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "untracked:$Relative" }
    & git -C $Root diff --quiet HEAD -- $Relative
    if ($LASTEXITCODE -ne 0) { throw "uncommitted:$Relative" }
    & git -C $Root diff --cached --quiet HEAD -- $Relative
    if ($LASTEXITCODE -ne 0) { throw "uncommitted:$Relative" }
    return $path
}

function Get-NativeLintGoos {
    if ($env:GOOS -cin @('windows', 'linux')) {
        return $env:GOOS.ToLowerInvariant()
    }
    if ($IsWindows) { return 'windows' }
    return 'linux'
}

function Get-ShipmentItems {
    param([Parameter(Mandatory)][string]$Path)
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

function Get-ExpectedResidualCount {
    param(
        [Parameter(Mandatory)]$Inventory,
        [Parameter(Mandatory)][hashtable]$StatusByTask,
        [Parameter(Mandatory)][ValidateSet('windows', 'linux')][string]$Goos
    )
    $excluded = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    foreach ($key in @($Inventory.surface_exclusions.$Goos)) {
        [void]$excluded.Add("$key")
    }
    $expected = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    foreach ($row in @($Inventory.findings)) {
        $key = Get-LintFindingKey -Finding $row
        if (-not $excluded.Contains($key) -and
            $StatusByTask['' + $row.owner_task_id] -cin $script:ExecutableStatuses) {
            [void]$expected.Add($key)
        }
    }
    foreach ($row in @($Inventory.additional_findings)) {
        if (('' + $row.goos) -ceq $Goos -and
            $StatusByTask['' + $row.owner_task_id] -cin $script:ExecutableStatuses) {
            [void]$expected.Add((Get-LintFindingKey -Finding $row))
        }
    }
    return $expected.Count
}

try {
    $root = if ([string]::IsNullOrWhiteSpace($RepositoryRoot)) {
        Get-RepositoryRoot
    }
    else {
        [System.IO.Path]::GetFullPath($RepositoryRoot)
    }
    [void](Assert-GolangCILintVersion -Root $root -Executable $GolangCILintExecutable)

    $queue = Resolve-ContainedPath -Root $root -Relative '.backlogit/queue' -Directory
    $candidates = @(
        Get-ChildItem -LiteralPath $queue -File -Filter '*.md' |
            Where-Object {
                (Get-Content -LiteralPath $_.FullName -Raw).Contains(
                    'baseline-lint-convergence-contract',
                    [System.StringComparison]::Ordinal)
            }
    )
    if ($candidates.Count -gt 1) {
        throw "multiple-baseline-convergence-units:$($candidates.Count)"
    }

    if ($candidates.Count -eq 0) {
        Write-Output 'CI-LINT-GATE:global-zero-warning'
        if ($PlanOnly) { exit 0 }
        & $GolangCILintExecutable run --timeout=5m --max-issues-per-linter=0 `
            --max-same-issues=0 --uniq-by-line=false
        if ($LASTEXITCODE -ne 0) { throw "global-lint-exit:$LASTEXITCODE" }
        exit 0
    }

    $candidate = $candidates[0]
    $featureId = Get-FrontmatterScalar -Path $candidate.FullName -Name id
    if ($featureId -cnotmatch '^\d+-F$') { throw "feature-id:$featureId" }
    $feature = Resolve-CanonicalArtifact -Root $root -Id $featureId
    if ($feature.Location -cne 'queue' -or
        (Get-FrontmatterScalar -Path $feature.Path -Name artifact_type) -cne 'feature') {
        throw "candidate-not-executable-feature:$featureId"
    }
    $contract = (Get-JsonContractBlock -Path $feature.Path `
        -Name 'baseline-lint-convergence-contract').Object
    $contractKeys = @(
        'mode', 'purpose', 'shipment_id', 'release_unit_id', 'terminal_task_id',
        'supported_goos', 'member_scope', 'topology', 'baseline_inventory',
        'intermediate_wave_lint_cmd', 'terminal_global_lint_cmd',
        'operator_authorization'
    )
    if (-not (Test-OrdinalSetEqual -Expected $contractKeys `
            -Actual @($contract.PSObject.Properties.Name)) -or
        ('' + $contract.mode) -cne 'baseline-convergence' -or
        ('' + $contract.purpose) -cne 'known-global-lint-baseline-removal' -or
        ('' + $contract.release_unit_id) -cne $featureId -or
        ('' + $contract.shipment_id) -cnotmatch '^\d+-S$' -or
        ('' + $contract.terminal_task_id) -cnotmatch '^\d+\.\d{3}-T$' -or
        -not (Test-OrdinalSetEqual -Expected @('windows', 'linux') `
            -Actual @($contract.supported_goos))) {
        throw "contract-shape:$featureId"
    }

    $statusByTask = @{}
    $roleByTask = @{}
    $memberIds = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    $terminalMembers = 0
    $remediationMembers = 0
    $unfinishedRemediation = 0
    foreach ($member in @($contract.member_scope)) {
        if (-not (Test-OrdinalSetEqual `
                -Expected @('task_id', 'role', 'task_contract_sha256') `
                -Actual @($member.PSObject.Properties.Name))) {
            throw 'member-schema'
        }
        $taskId = '' + $member.task_id
        $role = '' + $member.role
        if ($taskId -cnotmatch '^\d+\.\d{3}-T$' -or
            -not $memberIds.Add($taskId) -or
            $role -cnotin @(
                'baseline-control', 'finding-remediation',
                'support', 'terminal-convergence') -or
            ('' + $member.task_contract_sha256) -cnotmatch '^[0-9a-f]{64}$') {
            throw "member-shape:$taskId"
        }
        $task = Resolve-CanonicalArtifact -Root $root -Id $taskId
        if ((Get-FrontmatterScalar -Path $task.Path -Name artifact_type) -cne 'task' -or
            (Get-TaskContractDigest -TaskPath $task.Path) -cne
                ('' + $member.task_contract_sha256)) {
            throw "member-binding:$taskId"
        }
        $taskContract = (Get-JsonContractBlock -Path $task.Path `
            -Name 'task-lint-contract').Object
        Assert-TaskLintContractEnvelope -Contract $taskContract `
            -TaskId $taskId -Role $role
        if (('' + $taskContract.task_lint_cmd) -cne
            "pwsh -NoProfile -File scripts/verify-task-lint.ps1 -TaskId $taskId -FeatureId $featureId") {
            throw "noncanonical-task-command:$taskId"
        }
        $statusByTask[$taskId] = $task.Status
        $roleByTask[$taskId] = $role
        if ($role -ceq 'terminal-convergence') {
            $terminalMembers++
            if ($taskId -cne ('' + $contract.terminal_task_id)) {
                throw "terminal-identity:$taskId"
            }
        }
        elseif ($role -ceq 'finding-remediation') {
            $remediationMembers++
            if ($task.Status -cin $script:ExecutableStatuses) {
                $unfinishedRemediation++
            }
        }
    }
    if ($memberIds.Count -lt 1 -or $terminalMembers -ne 1 -or
        $remediationMembers -lt 1) {
        throw 'member-role-partition'
    }

    $shipment = '' + $contract.shipment_id
    $shipmentArtifact = Resolve-CanonicalArtifact -Root $root -Id $shipment
    if ((Get-FrontmatterScalar -Path $shipmentArtifact.Path -Name artifact_type) -cne
        'shipment') {
        throw "shipment-artifact-type:$shipment"
    }
    $shipmentItems = @(Get-ShipmentItems -Path $shipmentArtifact.Path)
    if (-not (Test-OrdinalSetEqual -Expected (@($featureId) + @($memberIds)) `
            -Actual $shipmentItems)) {
        throw 'shipment-member-scope'
    }

    $inventoryBinding = $contract.baseline_inventory
    if (-not (Test-OrdinalSetEqual `
            -Expected @('path', 'sha256', 'finding_count', 'identity_fields') `
            -Actual @($inventoryBinding.PSObject.Properties.Name)) -or
        ('' + $inventoryBinding.sha256) -cnotmatch '^[0-9a-f]{64}$') {
        throw 'inventory-binding'
    }
    $inventoryRelative = '' + $inventoryBinding.path
    $inventoryPath = Assert-TrackedCommittedFile -Root $root -Relative $inventoryRelative
    $inventoryDigest =
        (Get-FileHash -LiteralPath $inventoryPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($inventoryDigest -cne ('' + $inventoryBinding.sha256)) {
        throw 'inventory-digest'
    }
    try {
        $inventory = Get-Content -LiteralPath $inventoryPath -Raw |
            ConvertFrom-Json -NoEnumerate -DateKind String -ErrorAction Stop
    }
    catch { throw 'inventory-json' }
    Assert-PlatformInventoryShape -Inventory $inventory `
        -FeatureSupportedGoos @($contract.supported_goos)
    $rows = @($inventory.findings) + @($inventory.additional_findings)
    if ([int]$inventoryBinding.finding_count -ne $rows.Count -or $rows.Count -lt 1) {
        throw 'inventory-finding-count'
    }
    foreach ($row in $rows) {
        $owner = '' + $row.owner_task_id
        if (-not $statusByTask.ContainsKey($owner) -or
            $roleByTask[$owner] -cne 'finding-remediation') {
            throw "inventory-owner:$($row.owner_task_id)"
        }
    }

    $authorization = $contract.operator_authorization
    if (-not (Test-OrdinalSetEqual `
            -Expected @('record', 'authorized_at', 'authorized_by', 'shipment_id', 'scope') `
            -Actual @($authorization.PSObject.Properties.Name)) -or
        [string]::IsNullOrWhiteSpace('' + $authorization.authorized_by) -or
        ('' + $authorization.shipment_id) -cne ('' + $contract.shipment_id) -or
        ('' + $authorization.scope) -cne
            'retain-only-unfinished-owned-baseline-findings-until-terminal') {
        throw 'operator-authorization'
    }
    $authorizedAt = [datetimeoffset]::MinValue
    if (-not [datetimeoffset]::TryParse(
            '' + $authorization.authorized_at,
            [System.Globalization.CultureInfo]::InvariantCulture,
            [System.Globalization.DateTimeStyles]::RoundtripKind,
            [ref]$authorizedAt)) {
        throw 'operator-authorization-time'
    }
    $authorizationRelative = '' + $authorization.record
    $authorizationPath =
        Assert-TrackedCommittedFile -Root $root -Relative $authorizationRelative
    if ((Get-FrontmatterScalar -Path $authorizationPath -Name source) -cne
        $authorizationRelative) {
        throw 'operator-authorization-record'
    }

    $terminalTask = '' + $contract.terminal_task_id
    $expectedIntermediate = 'pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 ' +
        "-Inventory $inventoryRelative -InventorySha256 $inventoryDigest " +
        "-Shipment $shipment -FeatureId $featureId -TerminalTask $terminalTask"
    $expectedTerminal =
        "pwsh -NoProfile -File scripts/verify-terminal-lint.ps1 -FeatureId $featureId"
    if (('' + $contract.intermediate_wave_lint_cmd) -cne $expectedIntermediate -or
        ('' + $contract.terminal_global_lint_cmd) -cne $expectedTerminal) {
        throw 'canonical-command-binding'
    }
    $baselineVerifier = Assert-TrackedCommittedFile -Root $root `
        -Relative 'scripts/verify-baseline-lint.ps1'
    $terminalVerifier = Assert-TrackedCommittedFile -Root $root `
        -Relative 'scripts/verify-terminal-lint.ps1'

    $nativeGoos = Get-NativeLintGoos
    if ($unfinishedRemediation -gt 0) {
        $expectedCount = Get-ExpectedResidualCount -Inventory $inventory `
            -StatusByTask $statusByTask -Goos $nativeGoos
        Write-Output (
            "CI-LINT-GATE:baseline-residual:${featureId}:goos=${nativeGoos}:" +
            "expected=${expectedCount}")
        if ($PlanOnly) { exit 0 }
        & pwsh -NoProfile -File $baselineVerifier -Inventory $inventoryRelative `
            -InventorySha256 $inventoryDigest -Shipment $shipment `
            -FeatureId $featureId -TerminalTask $terminalTask
        if ($LASTEXITCODE -ne 0) { throw "baseline-verifier-exit:$LASTEXITCODE" }
        exit 0
    }

    Write-Output "CI-LINT-GATE:terminal-zero-warning:$featureId"
    if ($PlanOnly) { exit 0 }
    & pwsh -NoProfile -File $terminalVerifier -FeatureId $featureId
    if ($LASTEXITCODE -ne 0) { throw "terminal-verifier-exit:$LASTEXITCODE" }
    exit 0
}
catch {
    Stop-CILintGate -Message $_.Exception.Message
}
