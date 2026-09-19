#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Runs one canonical, lifecycle-aware task lint contract.
.DESCRIPTION
    This is the only supported task-lint entrypoint. Task artifacts contain
    structured scope and evidence, not copied PowerShell implementations.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidatePattern('^\d+\.\d{3}-T$')][string]$TaskId,
    [Parameter(Mandatory)][ValidatePattern('^\d+-F$')][string]$FeatureId,
    [ValidateSet('Schema', 'PreCommit', 'PostCommit')][string]$Phase = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'lint-runtime.ps1')

function Stop-TaskLint {
    param([int]$Code, [string]$Message)
    [Console]::Error.WriteLine($Message)
    exit $Code
}

try {
    Assert-CanonicalTaskPhase -Phase $Phase
    $root = Get-RepositoryRoot
    $task = Resolve-CanonicalArtifact -Root $root -Id $TaskId
    $feature = Resolve-CanonicalArtifact -Root $root -Id $FeatureId
    if ((Get-FrontmatterScalar -Path $task.Path -Name artifact_type) -cne 'task') {
        throw "artifact-type:$TaskId"
    }
    if ((Get-FrontmatterScalar -Path $task.Path -Name parent_id) -cne $FeatureId) {
        throw "task-parent:$TaskId"
    }
    if ((Get-FrontmatterScalar -Path $feature.Path -Name artifact_type) -cne 'feature') {
        throw "artifact-type:$FeatureId"
    }

    $featureBlock = (Get-JsonContractBlock -Path $feature.Path `
        -Name 'baseline-lint-convergence-contract').Object
    $members = @($featureBlock.member_scope | Where-Object { $_.task_id -ceq $TaskId })
    if ($members.Count -ne 1) { throw "member-scope:$TaskId" }
    $member = $members[0]
    if ($member.PSObject.Properties.Name -cnotcontains 'task_contract_sha256' -or
        ('' + $member.task_contract_sha256) -cnotmatch '^[0-9a-f]{64}$') {
        throw "task-contract-digest-missing:$TaskId"
    }
    $digest = Get-TaskContractDigest -TaskPath $task.Path
    if ($digest -cne ('' + $member.task_contract_sha256)) {
        throw "task-contract-digest-mismatch:$TaskId"
    }

    $block = (Get-JsonContractBlock -Path $task.Path -Name 'task-lint-contract').Object
    $canonicalCommand = "pwsh -NoProfile -File scripts/verify-task-lint.ps1 -TaskId $TaskId -FeatureId $FeatureId"
    if (('' + $block.task_lint_cmd) -cne $canonicalCommand) {
        throw "noncanonical-command:$TaskId"
    }
    $scope = $block.lint_scope
    if ($null -eq $scope -or $scope -isnot [System.Management.Automation.PSCustomObject]) {
        throw "lint-scope:$TaskId"
    }
    $role = '' + $member.role
    Assert-TaskLintContractEnvelope -Contract $block -TaskId $TaskId -Role $role

    if ($scope.kind -ceq 'bounded-finding-set-go-lint') {
        if ($role -cne 'finding-remediation') { throw "role-scope:$TaskId" }
        if (-not (Test-OrdinalSetEqual `
                -Expected @('kind', 'packages', 'owned_files', 'owned_findings') `
                -Actual @($scope.PSObject.Properties.Name))) {
            throw "lint-scope-schema:$TaskId"
        }
        $packages = @($scope.packages)
        $files = @($scope.owned_files)
        $findings = @($scope.owned_findings)
        if ($packages.Count -lt 1 -or $packages.Count -gt 2 -or
            $files.Count -lt 1 -or $files.Count -gt 2 -or
            $findings.Count -lt 1 -or $findings.Count -gt 16 -or
            @($packages | Sort-Object -Unique).Count -ne $packages.Count -or
            @($files | Sort-Object -Unique).Count -ne $files.Count -or
            @($packages | Where-Object {
                    "$_" -cnotmatch '^\./(?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+$'
                }).Count -gt 0) {
            throw "bounded-scope:$TaskId"
        }
        $fileSet = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal)
        $expectedPackages = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal)
        foreach ($file in $files) {
            $normalized = "$file" -replace '\\', '/'
            [void]$fileSet.Add($normalized)
            $full = Resolve-ContainedPath -Root $root -Relative $normalized
            & git -C $root ls-files --error-unmatch -- $normalized 2>$null | Out-Null
            if ($LASTEXITCODE -ne 0) { throw "untracked:$normalized" }
            & git -C $root check-ignore -q -- $normalized
            if ($LASTEXITCODE -eq 0) { throw "ignored:$normalized" }
            if ([System.IO.Path]::GetExtension($full) -cne '.go') {
                throw "non-go-owned-file:$normalized"
            }
            [void]$expectedPackages.Add(
                (Resolve-OwnedFilePackage -File $normalized -Packages $packages))
        }
        if (-not (Test-OrdinalSetEqual -Expected @($expectedPackages) -Actual $packages)) {
            throw "package-ownership:$TaskId"
        }
        $ownedKeys = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal)
        foreach ($finding in $findings) {
            foreach ($name in @('path', 'line', 'column', 'linter', 'message')) {
                if ($finding.PSObject.Properties.Name -cnotcontains $name) {
                    throw "finding-schema:$TaskId"
                }
            }
            $findingPath = ('' + $finding.path) -replace '\\', '/'
            if (-not $fileSet.Contains($findingPath)) {
                throw "finding-file:$TaskId"
            }
            if (-not $ownedKeys.Add((Get-LintFindingKey -Finding $finding))) {
                throw "duplicate-finding:$TaskId"
            }
        }

        $inventoryBinding = $featureBlock.baseline_inventory
        $inventoryPath = Resolve-ContainedPath -Root $root `
            -Relative ('' + $inventoryBinding.path)
        if ((Get-FileHash -LiteralPath $inventoryPath -Algorithm SHA256).Hash.ToLowerInvariant() `
            -cne ('' + $inventoryBinding.sha256)) {
            throw 'inventory-digest'
        }
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
        Assert-PlatformInventoryShape -Inventory $inventory `
            -FeatureSupportedGoos @($featureBlock.supported_goos)
        $allowedByGoos = @{
            windows = [System.Collections.Generic.HashSet[string]]::new(
                [System.StringComparer]::Ordinal)
            linux = [System.Collections.Generic.HashSet[string]]::new(
                [System.StringComparer]::Ordinal)
        }
        $inventoryOwnerByKey = [System.Collections.Generic.Dictionary[string,string]]::new(
            [System.StringComparer]::Ordinal)
        $primaryKeys = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal)
        foreach ($row in @($inventory.findings)) {
            [void]$primaryKeys.Add((Get-LintFindingKey -Finding $row))
        }
        $memberIds = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal)
        $memberRoleById = @{}
        foreach ($entry in @($featureBlock.member_scope)) {
            if (-not $memberIds.Add('' + $entry.task_id)) {
                throw "duplicate-member:$($entry.task_id)"
            }
            $memberRoleById['' + $entry.task_id] = '' + $entry.role
        }
        $statusCache = @{}
        $inventoryKeysForCurrent = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal)
        $requiredGoosByFile = @{}
        foreach ($file in $files) {
            $requiredGoosByFile["$file"] =
                [System.Collections.Generic.HashSet[string]]::new(
                    [System.StringComparer]::Ordinal)
        }
        foreach ($row in @($inventory.findings) + @($inventory.additional_findings)) {
            $key = Get-LintFindingKey -Finding $row
            $owner = '' + $row.owner_task_id
            if (-not $memberIds.Contains($owner)) { throw "inventory-owner:$owner" }
            if ($memberRoleById[$owner] -cne 'finding-remediation') {
                throw "inventory-owner-role:$owner"
            }
            if (-not $inventoryOwnerByKey.TryAdd($key, $owner)) {
                throw "duplicate-inventory-finding:$key"
            }
            if ($owner -ceq $TaskId) {
                [void]$inventoryKeysForCurrent.Add($key)
                $rowPath = ('' + $row.path) -replace '\\', '/'
                if (-not $requiredGoosByFile.ContainsKey($rowPath)) {
                    throw "inventory-file-ownership:$rowPath"
                }
                if ($primaryKeys.Contains($key)) {
                    [void]$requiredGoosByFile[$rowPath].Add('windows')
                    if (@($inventory.surface_exclusions.linux) -cnotcontains $key) {
                        [void]$requiredGoosByFile[$rowPath].Add('linux')
                    }
                }
                else {
                    [void]$requiredGoosByFile[$rowPath].Add('linux')
                }
            }
            if (-not $statusCache.ContainsKey($owner)) {
                $ownerArtifact = Resolve-CanonicalArtifact -Root $root -Id $owner
                $statusCache[$owner] = $ownerArtifact.Status
            }
            if ($owner -ceq $TaskId -or
                $statusCache[$owner] -cnotin $script:ExecutableStatuses) {
                continue
            }
            if ($primaryKeys.Contains($key)) {
                $rowDirectory = ([System.IO.Path]::GetDirectoryName(
                        (('' + $row.path) -replace '\\', '/')) -replace '\\', '/')
                if ($packages -ccontains "./$rowDirectory") {
                    [void]$allowedByGoos.windows.Add($key)
                    if (@($inventory.surface_exclusions.linux) -cnotcontains $key) {
                        [void]$allowedByGoos.linux.Add($key)
                    }
                }
            }
            else {
                if ($row.PSObject.Properties.Name -cnotcontains 'goos' -or
                    ('' + $row.goos) -cne 'linux') {
                    throw "inventory-goos:$key"
                }
                $rowDirectory = ([System.IO.Path]::GetDirectoryName(
                        (('' + $row.path) -replace '\\', '/')) -replace '\\', '/')
                if ($packages -ccontains "./$rowDirectory") {
                    [void]$allowedByGoos['' + $row.goos].Add($key)
                }
            }
        }
        if ($featureBlock.baseline_inventory.PSObject.Properties.Name -cnotcontains
            'finding_count' -or
            [int]$featureBlock.baseline_inventory.finding_count -ne
            $inventoryOwnerByKey.Count) {
            throw 'inventory-finding-count'
        }
        foreach ($key in $ownedKeys) {
            if (-not $inventoryOwnerByKey.ContainsKey($key) -or
                $inventoryOwnerByKey[$key] -cne $TaskId) {
                throw "inventory-ownership:$key"
            }
        }
        if (-not (Test-OrdinalSetEqual -Expected @($ownedKeys) `
                -Actual @($inventoryKeysForCurrent))) {
            throw "inventory-owner-set:$TaskId"
        }

        [void](Assert-GolangCILintVersion -Root $root)
        $participation = @{}
        foreach ($goos in @('windows', 'linux')) {
            foreach ($file in $files) {
                $package = Resolve-OwnedFilePackage -File "$file" -Packages $packages
                $listed = Invoke-CapturedProcess -FileName 'go' `
                    -Arguments @('list', '-f',
                        '{{range .GoFiles}}{{.}}|{{end}}{{range .CgoFiles}}{{.}}|{{end}}{{range .TestGoFiles}}{{.}}|{{end}}{{range .XTestGoFiles}}{{.}}|{{end}}',
                        '--', $package) -WorkingDirectory $root -Environment @{ GOOS = $goos }
                if ($listed.ExitCode -ne 0) { throw "go-list:${goos}:$package" }
                if (Test-GoListFileMembership -Output $listed.Stdout -File "$file") {
                    $participation["$file|$goos"] = $true
                }
            }
            $lint = Invoke-StructuredGolangCILint -Root $root -Packages $packages -Goos $goos
            $ownedObserved = @(Get-OwnedObservedLintKeys -Issues $lint.Issues `
                    -OwnedFiles $fileSet -OwnedKeys $ownedKeys)
            if ($ownedObserved.Count -ne 0) {
                throw "owned-finding-remains:${goos}:$($ownedObserved.Count)"
            }
            $unexpectedObserved = @(Get-UnauthorizedLintKeys -Issues $lint.Issues `
                    -CurrentOwnerKeys $ownedKeys `
                    -AllowedOtherOwnerKeys $allowedByGoos[$goos])
            if ($unexpectedObserved.Count -ne 0) {
                throw "new-or-mutated-finding:${goos}:$($unexpectedObserved.Count)"
            }
            $observedKeys = @($lint.Issues | ForEach-Object {
                    Get-LintFindingKey -Finding $_ -Native
                })
            if (-not (Test-OrdinalSetEqual -Expected @($allowedByGoos[$goos]) `
                    -Actual $observedKeys)) {
                throw "other-owner-residual-mismatch:$goos"
            }
        }
        Assert-RequiredGoosParticipation -RequiredByFile $requiredGoosByFile `
            -ObservedByFileGoos $participation
    }
    elseif ($scope.kind -ceq 'harness-file-scoped-go-lint') {
        if ($role -notin @('baseline-control', 'support')) { throw "role-scope:$TaskId" }
        if (-not (Test-OrdinalSetEqual -Expected @('kind', 'owned_file', 'package') `
                -Actual @($scope.PSObject.Properties.Name))) {
            throw "lint-scope-schema:$TaskId"
        }
        $ownedFile = ('' + $scope.owned_file) -replace '\\', '/'
        $package = '' + $scope.package
        if ([string]::IsNullOrWhiteSpace($ownedFile) -or
            [string]::IsNullOrWhiteSpace($package) -or
            $package -cnotmatch '^\./(?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+$') {
            throw "harness-scope:$TaskId"
        }
        [void](Resolve-OwnedFilePackage -File $ownedFile -Packages @($package))
        [void](Resolve-ContainedPath -Root $root -Relative $ownedFile)
        & git -C $root ls-files --error-unmatch -- $ownedFile 2>$null | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "untracked:$ownedFile" }
        & git -C $root check-ignore -q -- $ownedFile
        if ($LASTEXITCODE -eq 0) { throw "ignored:$ownedFile" }
        [void](Assert-GolangCILintVersion -Root $root)
        $participates = 0
        foreach ($goos in @('windows', 'linux')) {
            $listed = Invoke-CapturedProcess -FileName 'go' `
                -Arguments @('list', '-f',
                    '{{range .GoFiles}}{{.}}|{{end}}{{range .CgoFiles}}{{.}}|{{end}}{{range .TestGoFiles}}{{.}}|{{end}}{{range .XTestGoFiles}}{{.}}|{{end}}',
                    '--', $package) -WorkingDirectory $root -Environment @{ GOOS = $goos }
            if ($listed.ExitCode -ne 0) { throw "go-list:${goos}:$package" }
            if (-not (Test-GoListFileMembership -Output $listed.Stdout -File $ownedFile)) { continue }
            $participates++
            $lint = Invoke-StructuredGolangCILint -Root $root -Packages @($package) -Goos $goos
            $hits = @($lint.Issues | Where-Object {
                    (('' + $_.Pos.Filename) -replace '\\', '/') -ceq $ownedFile
                })
            if ($hits.Count -ne 0) { throw "owned-finding-remains:${goos}:$($hits.Count)" }
        }
        if ($participates -lt 1) { throw "not-in-supported-package:$ownedFile" }
    }
    elseif ($scope.kind -ceq 'no-go-lint-surface') {
        if ($role -cne 'terminal-convergence') { throw "role-scope:$TaskId" }
        if (-not (Test-OrdinalSetEqual `
                -Expected @('kind', 'owned_paths', 'governed_claim_paths') `
                -Actual @($scope.PSObject.Properties.Name))) {
            throw "lint-scope-schema:$TaskId"
        }
        $deliverables = @($scope.owned_paths)
        $claimPaths = @($scope.governed_claim_paths)
        if ($deliverables.Count -lt 1 -or $claimPaths.Count -lt 1 -or
            @(Get-OrdinalDistinct -Values @($deliverables + $claimPaths)).Count -ne
            ($deliverables.Count + $claimPaths.Count) -or
            @($deliverables + $claimPaths | Where-Object { "$_" -match '\.go$' }).Count -gt 0) {
            throw "no-go-scope:$TaskId"
        }
        foreach ($path in $deliverables + $claimPaths) {
            if ([System.IO.Path]::IsPathRooted("$path") -or "$path" -match '(^|[\\/])\.\.([\\/]|$)') {
                throw "uncontained-path:$path"
            }
        }
        if ([string]::IsNullOrWhiteSpace($Phase)) { throw 'claim-phase-required' }
        if ($Phase -ne 'Schema') {
            $working = @(Get-OrdinalDistinct -Values @(
                    & git -C $root diff --name-only
                    & git -C $root diff --cached --name-only
                    & git -C $root ls-files --others --exclude-standard
                ))
        }
        if ($Phase -eq 'PreCommit') {
            if (-not (Test-OrdinalSetEqual -Expected @($deliverables + $claimPaths) -Actual $working)) {
                throw 'claim-union-precommit'
            }
        }
        elseif ($Phase -eq 'PostCommit') {
            $committed = @(& git -C $root diff-tree --no-commit-id --name-only -r HEAD)
            if (-not (Test-OrdinalSetEqual -Expected $deliverables -Actual $committed) -or
                -not (Test-OrdinalSetEqual -Expected $claimPaths -Actual $working) -or
                -not (Test-OrdinalSetEqual -Expected @($deliverables + $claimPaths) `
                    -Actual @($committed + $working))) {
                throw 'claim-union-postcommit'
            }
        }
    }
    else {
        throw "scope-kind:$($scope.kind)"
    }

    Write-Output "TASK-LINT-OK:${TaskId}:${role}"
    exit 0
}
catch {
    Stop-TaskLint 81 "TASK-LINT-FAIL:${TaskId}:$($_.Exception.Message)"
}
