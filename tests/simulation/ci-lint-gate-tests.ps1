#!/usr/bin/env pwsh
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = (& git rev-parse --show-toplevel).Trim()
$gate = Join-Path $root 'scripts/verify-ci-lint-gate.ps1'
$passed = 0
$failed = 0

function Assert-True {
    param([string]$Name, [bool]$Condition)
    if ($Condition) {
        $script:passed++
        Write-Output "PASS: $Name"
    }
    else {
        $script:failed++
        Write-Output "FAIL: $Name"
    }
}

function Invoke-GatePlan {
    param([string]$Fixture, [string]$Goos = 'linux')
    $prior = $env:GOOS
    try {
        $env:GOOS = $Goos
        $output = @(& pwsh -NoProfile -File $gate -RepositoryRoot $Fixture -PlanOnly 2>&1)
        [pscustomobject]@{
            ExitCode = $LASTEXITCODE
            Output = ($output -join "`n")
        }
    }
    finally {
        $env:GOOS = $prior
    }
}

function Write-Text {
    param([string]$Path, [string]$Text)
    $parent = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $parent)) {
        [void](New-Item -ItemType Directory -Path $parent -Force)
    }
    [System.IO.File]::WriteAllText(
        $Path, $Text.Replace("`r`n", "`n"), [System.Text.UTF8Encoding]::new($false))
}

function Write-Task {
    param(
        [string]$Fixture,
        [string]$Id,
        [string]$Status,
        [string]$Location,
        [string]$Role
    )
    $contract = [pscustomobject][ordered]@{
        task_lint_cmd = "pwsh -NoProfile -File scripts/verify-task-lint.ps1 -TaskId $Id -FeatureId 175-F"
        lint_scope = if ($Role -eq 'terminal-convergence') {
            [pscustomobject][ordered]@{
                kind = 'no-go-lint-surface'
                owned_paths = @('docs/closure/fixture.md')
                governed_claim_paths = @('.backlogit/queue/175.002-T.md')
            }
        }
        else {
            [pscustomobject][ordered]@{
                kind = 'bounded-finding-set-go-lint'
                packages = @('./internal/example')
                owned_files = @('internal/example/example.go')
                owned_findings = @()
            }
        }
        non_vacuity_evidence = [pscustomobject][ordered]@{
            success_marker = "TASK-LINT-OK:${Id}:${Role}"
        }
    }
    $json = $contract | ConvertTo-Json -Depth 10
    $text = @"
---
artifact_type: task
id: $Id
status: $Status
---

<!-- BEGIN:task-lint-contract -->
``````json
$json
``````
<!-- END:task-lint-contract -->
"@
    $path = Join-Path $Fixture ".backlogit/$Location/$Id.md"
    Write-Text -Path $path -Text $text
    $canonical = $contract | ConvertTo-Json -Depth 100 -Compress
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($canonical)
    return [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($bytes)).ToLowerInvariant()
}

function New-ContractFixture {
    param(
        [string]$Base,
        [string]$RemediationStatus = 'queued',
        [switch]$Malformed,
        [switch]$Multiple
    )
    $fixture = Join-Path $Base ([guid]::NewGuid().ToString('N'))
    foreach ($dir in @(
            '.backlogit/queue', '.backlogit/archive', 'scripts',
            'docs/decisions', 'internal/example')) {
        [void](New-Item -ItemType Directory -Path (Join-Path $fixture $dir) -Force)
    }
    Write-Text -Path (Join-Path $fixture 'scripts/verify-baseline-lint.ps1') `
        -Text 'exit 0'
    Write-Text -Path (Join-Path $fixture 'scripts/verify-terminal-lint.ps1') `
        -Text 'exit 0'
    Write-Text -Path (Join-Path $fixture 'internal/example/example.go') `
        -Text "package example`n"
    Write-Text -Path (Join-Path $fixture '.backlogit/queue/156-S.md') -Text @'
---
artifact_type: shipment
custom_fields:
    items:
        - 175-F
        - 175.001-T
        - 175.002-T
id: 156-S
status: queued
---
'@
    $location = if ($RemediationStatus -eq 'done') { 'archive' } else { 'queue' }
    $digest = Write-Task -Fixture $fixture -Id '175.001-T' `
        -Status $RemediationStatus -Location $location -Role 'finding-remediation'
    $terminalDigest = Write-Task -Fixture $fixture -Id '175.002-T' `
        -Status 'queued' -Location 'queue' -Role 'terminal-convergence'

    $finding = [pscustomobject][ordered]@{
        path = 'internal/example/example.go'
        line = 1
        column = 1
        linter = 'errcheck'
        message = 'fixture finding'
        owner_task_id = '175.001-T'
    }
    $inventory = [pscustomobject][ordered]@{
        primary_goos = 'windows'
        supported_goos = @('windows', 'linux')
        findings = @($finding)
        surface_exclusions = [pscustomobject][ordered]@{
            windows = @()
            linux = @()
        }
        additional_findings = @()
    }
    $inventoryPath = Join-Path $fixture 'docs/decisions/baseline.json'
    Write-Text -Path $inventoryPath `
        -Text ($inventory | ConvertTo-Json -Depth 10)
    $inventorySha =
        (Get-FileHash -LiteralPath $inventoryPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $authRelative = 'docs/decisions/authorization.md'
    Write-Text -Path (Join-Path $fixture $authRelative) -Text @"
---
source: $authRelative
---

# Fixture authorization
"@
    $contract = [pscustomobject][ordered]@{
        mode = 'baseline-convergence'
        purpose = 'known-global-lint-baseline-removal'
        shipment_id = '156-S'
        release_unit_id = '175-F'
        terminal_task_id = '175.002-T'
        supported_goos = @('windows', 'linux')
        member_scope = @(
            [pscustomobject][ordered]@{
                task_id = '175.001-T'
                role = 'finding-remediation'
                task_contract_sha256 = $digest
            },
            [pscustomobject][ordered]@{
                task_id = '175.002-T'
                role = 'terminal-convergence'
                task_contract_sha256 = $terminalDigest
            }
        )
        topology = [pscustomobject]@{}
        baseline_inventory = [pscustomobject][ordered]@{
            path = 'docs/decisions/baseline.json'
            sha256 = $inventorySha
            finding_count = 1
            identity_fields = @(
                'path', 'line', 'column', 'linter', 'message', 'owner_task_id')
        }
        intermediate_wave_lint_cmd =
            "pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 -Inventory docs/decisions/baseline.json -InventorySha256 $inventorySha -Shipment 156-S -FeatureId 175-F -TerminalTask 175.002-T"
        terminal_global_lint_cmd =
            'pwsh -NoProfile -File scripts/verify-terminal-lint.ps1 -FeatureId 175-F'
        operator_authorization = [pscustomobject][ordered]@{
            record = $authRelative
            authorized_at = '2026-09-19T01:01:22Z'
            authorized_by = 'Fixture Operator'
            shipment_id = '156-S'
            scope = 'retain-only-unfinished-owned-baseline-findings-until-terminal'
        }
    }
    $contractJson = if ($Malformed) {
        '{not-json'
    }
    else {
        $contract | ConvertTo-Json -Depth 20
    }
    $feature = @"
---
artifact_type: feature
id: 175-F
status: queued
---

<!-- BEGIN:baseline-lint-convergence-contract -->
``````json
$contractJson
``````
<!-- END:baseline-lint-convergence-contract -->
"@
    Write-Text -Path (Join-Path $fixture '.backlogit/queue/175-F.md') -Text $feature
    if ($Multiple) {
        Write-Text -Path (Join-Path $fixture '.backlogit/queue/176-F.md') `
            -Text ($feature -replace 'id: 175-F', 'id: 176-F')
    }
    & git -C $fixture init -q
    & git -C $fixture add .
    & git -C $fixture -c user.name=Fixture -c user.email=fixture@example.invalid `
        commit -q -m fixture
    return $fixture
}

$temp = Join-Path $root ".autoharness/staging/ci-lint-gate-test-$(
    [guid]::NewGuid().ToString('N'))"
try {
    [void](New-Item -ItemType Directory -Path $temp -Force)

    $noContract = Join-Path $temp 'no-contract'
    [void](New-Item -ItemType Directory -Path (
            Join-Path $noContract '.backlogit/queue') -Force)
    & git -C $noContract init -q
    & git -C $noContract -c user.name=Fixture -c user.email=fixture@example.invalid `
        commit -q --allow-empty -m fixture
    $global = Invoke-GatePlan -Fixture $noContract
    Assert-True 'no convergence contract selects ordinary global zero-warning lint' (
        $global.ExitCode -eq 0 -and
        $global.Output -match 'CI-LINT-GATE:global-zero-warning')

    $unfinishedFixture = New-ContractFixture -Base $temp
    $unfinished = Invoke-GatePlan -Fixture $unfinishedFixture
    Assert-True 'one valid unfinished contract selects exact residual verifier' (
        $unfinished.ExitCode -eq 0 -and
        $unfinished.Output -match
            'CI-LINT-GATE:baseline-residual:175-F:goos=linux:expected=1')

    $terminalFixture = New-ContractFixture -Base $temp -RemediationStatus done
    $terminal = Invoke-GatePlan -Fixture $terminalFixture
    Assert-True 'all remediation complete selects terminal zero-warning verifier' (
        $terminal.ExitCode -eq 0 -and
        $terminal.Output -match 'CI-LINT-GATE:terminal-zero-warning:175-F')

    $malformedFixture = New-ContractFixture -Base $temp -Malformed
    $malformed = Invoke-GatePlan -Fixture $malformedFixture
    Assert-True 'malformed convergence contract fails closed' (
        $malformed.ExitCode -ne 0 -and
        $malformed.Output -match 'CI-LINT-GATE-FAIL:contract-json')

    $multipleFixture = New-ContractFixture -Base $temp -Multiple
    $multiple = Invoke-GatePlan -Fixture $multipleFixture
    Assert-True 'multiple convergence contracts fail closed' (
        $multiple.ExitCode -ne 0 -and
        $multiple.Output -match 'CI-LINT-GATE-FAIL:multiple-baseline-convergence-units:2')

    $live = Invoke-GatePlan -Fixture $root
    Assert-True 'current 175-F Linux projection declares exact 493 residual' (
        $live.ExitCode -eq 0 -and
        $live.Output -match
            'CI-LINT-GATE:baseline-residual:175-F:goos=linux:expected=493')
}
finally {
    if (Test-Path -LiteralPath $temp) {
        Remove-Item -LiteralPath $temp -Recurse -Force
    }
}

if ($failed -ne 0) {
    throw "CI_LINT_GATE_TESTS_FAILED:$failed"
}
Write-Output "CI_LINT_GATE_TESTS_OK:$passed"
