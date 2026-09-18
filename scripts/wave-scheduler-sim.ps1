#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Read-only P-002.6 wave-scheduler contract simulation.

.DESCRIPTION
    Replays the dependency-aware wave scheduler defined by
    `.github/policies/workflow-policies.md` P-002.6 against the tracked fixture
    `tests/simulation/wave-scheduler-contract.json`, and checks every scenario
    expectation the fixture declares.

    The simulation is read-only: it reads the fixture (and, with -VerifyAgainstQueue,
    the backlog Markdown plus workspace/registry YAML configuration), computes
    in memory, and writes nothing. It runs no `go` command and touches no
    repository state. When a baseline-convergence block exists, it invokes only
    read-only Git/file hashing to bind its committed inventory. It is therefore
    safe to run at any gate point, including under the P-002.5 read-only command
    screen.

    Assertion coverage:
      * real shipment-manifest parsing, task-type filtering, and excluded-ID report
      * live workspace status catalog plus registry status-mapping/feature parsing
      * exact manifest-M versus explicit non-shipment fallback-set comparison
      * archived historical sibling exclusion from M and the fallback set
      * all five red-deliverable keys, with data-driven in-memory mutation checks
      * canonical empty-default green-regression contract projection
      * baseline 18-wave schedule, zero stalls, zero compile-order violations
      * persistent red-deliverable mapping and the open-red convergence rule
      * open-red re-confirmation: every still-open selector, including carried-in
        entries, is re-run at each convergence gate and must stay RED; every
        selector closed at that gate is re-run and must be GREEN; an early-green
        entry halts with WAVE_RED_DELIVERABLE_EARLY_GREEN and a still-red closed
        entry halts with WAVE_GREEN_MAKER_UNVERIFIED
      * blocked-member injection (initial and mid-run)
      * unsupported status tokens (catalog, off-catalog, catalog unavailable)
      * active residual at wave admission
      * dependency-cycle injection
      * sibling-red wave: withdrawn repo-wide gate vs task-scoped gate
      * installed Ship/build-feature task-lint contract symmetry
      * strict-by-default and explicit baseline-convergence repository lint modes
      * exact intermediate residual identities and terminal zero-warning global lint
      * exact, transition- and digest-validated harness-exempt governed claim delta
      * queue-backed shipment 156-S projection (40 tasks, 75 edges, U1-only first wave)
      * non-frozen-M negative control
      * red-to-green-maker mapping fail-closed cases

.PARAMETER Fixture
    Path to the simulation fixture. Defaults to
    tests/simulation/wave-scheduler-contract.json relative to the repository root.

.PARAMETER VerifyAgainstQueue
    Parse the real shipment artifact, resolve and filter its task-type members into
    M, report excluded non-task IDs, parse the live workspace status catalog and
    registry status mapping/features, and re-derive statuses, dependency edges,
    harness-exempt labels, all red-deliverable keys, and green-regression contracts
    from repository artifacts. Also compare M with the fixture's explicit
    non-shipment fallback task-ID set and run fixture-declared in-memory mutation
    checks proving that each comparison detects drift.

.PARAMETER QueueDir
    Backlog queue directory used by -VerifyAgainstQueue. Default `.backlogit/queue`.

.PARAMETER OperatorAuthorizationRecord
    Operator-supplied opaque authorization identity exact-compared with a live
    baseline-lint-convergence contract. Required only when that contract exists.

.PARAMETER Scenario
    Run only the named scenario id. Default: all scenarios.

.PARAMETER Quiet
    Print only the summary line and any failures.

.EXAMPLE
    pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1

.EXAMPLE
    pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue

.OUTPUTS
    Exit code 0 when every assertion passes, 1 otherwise.
#>
[CmdletBinding()]
param(
    [string]$Fixture = '',
    [switch]$VerifyAgainstQueue,
    [string]$QueueDir = '',
    [string]$OperatorAuthorizationRecord = '',
    [string]$Scenario = '',
    [switch]$Quiet
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# --- repository root resolution (script lives in scripts/) ---------------------
$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($Fixture)) {
    $Fixture = Join-Path $repoRoot 'tests/simulation/wave-scheduler-contract.json'
}
if ([string]::IsNullOrWhiteSpace($QueueDir)) {
    $QueueDir = Join-Path $repoRoot '.backlogit/queue'
}

$script:Assertions = [System.Collections.Generic.List[object]]::new()

function Add-Assertion {
    param([string]$Scenario, [string]$Name, [bool]$Ok, [string]$Expected, [string]$Actual)
    $script:Assertions.Add([pscustomobject]@{
            Scenario = $Scenario
            Name     = $Name
            Ok       = $Ok
            Expected = $Expected
            Actual   = $Actual
        })
}

function Format-Value {
    param($Value)
    if ($null -eq $Value) { return '<null>' }
    if ($Value -is [bool]) { return $Value.ToString().ToLowerInvariant() }
    if ($Value -is [string]) { return $Value }
    if ($Value -is [System.Collections.IEnumerable]) { return '[' + (($Value | ForEach-Object { "$_" }) -join ', ') + ']' }
    return "$Value"
}

function Sort-Ordinal {
    param($Values)
    [string[]]$items = @($Values | ForEach-Object { "$_" })
    [System.Array]::Sort($items, [System.StringComparer]::Ordinal)
    return $items
}

function Test-ExactOrdinalSet {
    param($Expected, $Actual)
    [string[]]$expectedItems = @(Sort-Ordinal $Expected)
    [string[]]$actualItems = @($Actual | ForEach-Object { "$_" })
    $unique = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    foreach ($item in $actualItems) {
        if (-not $unique.Add($item)) { return $false }
    }
    [string[]]$actualSorted = @(Sort-Ordinal $actualItems)
    return [bool](($expectedItems -join "`n") -ceq ($actualSorted -join "`n"))
}

function Test-Equal {
    param([string]$Scenario, [string]$Name, $Expected, $Actual)
    $e = Format-Value $Expected
    $a = Format-Value $Actual
    Add-Assertion -Scenario $Scenario -Name $Name -Ok ($e -ceq $a) -Expected $e -Actual $a
}

function Get-MarkdownSection {
    param([string]$Text, [string]$Heading)

    if ($Heading -notmatch '^(#+)\s') { return '' }
    $level = $Matches[1].Length
    $headingPattern = [regex]::Escape($Heading)
    $match = [regex]::Match(
        $Text,
        "(?ms)^$headingPattern\r?\n(?<body>.*?)(?=^#{1,$level}\s|\z)"
    )
    if (-not $match.Success) { return '' }
    return $match.Value
}

function Test-SectionTerms {
    param(
        [string]$Scenario,
        [string]$Artifact,
        [string]$Section,
        [string[]]$Required,
        [string[]]$Forbidden = @()
    )

    $path = Join-Path $repoRoot $Artifact
    $text = if (Test-Path $path) { Get-Content $path -Raw } else { '' }
    $sectionText = Get-MarkdownSection -Text $text -Heading $Section
    Test-Equal -Scenario $Scenario -Name "$Artifact section $Section exists" `
        -Expected $true -Actual (-not [string]::IsNullOrWhiteSpace($sectionText))
    foreach ($term in $Required) {
        Test-Equal -Scenario $Scenario -Name "$Artifact $Section contains $term" `
            -Expected $true -Actual ($sectionText.IndexOf($term, [System.StringComparison]::OrdinalIgnoreCase) -ge 0)
    }
    foreach ($term in $Forbidden) {
        Test-Equal -Scenario $Scenario -Name "$Artifact $Section excludes $term" `
            -Expected $false -Actual ($sectionText.IndexOf($term, [System.StringComparison]::OrdinalIgnoreCase) -ge 0)
    }
}

function Invoke-LintGateContractChecks {
    $contract = $fx.lint_gate_contract
    $sc = 'lint_gate_contract'
    $taskCommand = "$($contract.task_command_input)"
    $canonicalBlock = "$($contract.canonical_block)"
    $taskContractName = "$($contract.task_contract_name)"
    $globalCommand = "$($contract.global_command)"
    $baselineBlock = "$($contract.baseline_block)"
    $baselineMode = "$($contract.baseline_mode)"
    $strictMode = "$($contract.strict_mode)"
    $claimInput = "$($contract.claim_input)"
    $lintModeInput = "$($contract.lint_mode_input)"

    foreach ($constitutionArtifact in @($contract.constitution_artifacts)) {
        $constitutionHeading = if ($constitutionArtifact -ceq 'AGENTS.md') {
            '### I. Safety-First Go (NON-NEGOTIABLE)'
        } else {
            '### I. Safety-First Go'
        }
        Test-SectionTerms -Scenario $sc -Artifact "$constitutionArtifact" `
            -Section $constitutionHeading `
            -Required @(
                $baselineMode, 'sole exception', 'exact shipment-scoped operator authorization',
                'native-failure-preserving', 'exact residual-set verification', 'CONVERGING',
                'unique terminal boundary', $globalCommand, 'zero warnings'
            )
        $constitutionText = Get-Content (Join-Path $repoRoot $constitutionArtifact) -Raw
        Test-Equal -Scenario $sc -Name "$constitutionArtifact carries constitution version" `
            -Expected $true -Actual (
                $constitutionText.IndexOf(
                    "$($contract.constitution_version)",
                    [System.StringComparison]::Ordinal
                ) -ge 0
            )
    }
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.build_feature_artifact)" `
        -Section '## Inputs' `
        -Required @($taskCommand, $canonicalBlock, $taskContractName, 'wave_scoped', 'harness-exempt', $claimInput, $lintModeInput)
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.build_feature_artifact)" `
        -Section '### Wave-Scoped Lint Contract Precondition' `
        -Required @($taskCommand, 'native', 'non-vacu', 'before any mutation', 'harness-exempt')
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.build_feature_artifact)" `
        -Section '### Post-Loop Quality Gates' `
        -Required @($taskCommand, 'wave_scoped: true', 'wave_scoped: false', $globalCommand, 'native', 'non-vacu', $claimInput, 'task_owned_delta', $lintModeInput, $strictMode, $baselineMode) `
        -Forbidden @("1. **Lint**: ``$globalCommand``")
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.build_feature_artifact)" `
        -Section '### Governed Claim Delta Precondition' `
        -Required @($claimInput, 'registry', 'after-digest', 'EXEMPT_CLAIM_DELTA_INVALID', 'before any mutation')
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.build_feature_artifact)" `
        -Section '#### Step 0.5d: Inverted quality gates, no fix iteration' `
        -Required @($taskCommand, $globalCommand, 'native', 'vacuous', $lintModeInput, $strictMode, $baselineMode) `
        -Forbidden @("1. **Lint**: ``$globalCommand`` — unchanged.")

    Test-SectionTerms -Scenario $sc -Artifact "$($contract.ship_artifact)" `
        -Section '### Step 3: Build Wave Schedule (P-002.6)' `
        -Required @($taskCommand, $canonicalBlock, $taskContractName, 'freeze', 'harness-exempt', 'native', 'non-vacu', $baselineBlock, $baselineMode, $strictMode, 'terminal_task_id', 'operator_authorization', 'member_scope', 'task_artifact_sha256', 'BASELINE-LINT-RESIDUAL-OK')
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.ship_artifact)" `
        -Section '#### Step 4.1: Wave-Scoped Task-Lint Claim-Time Gate' `
        -Required @($taskCommand, 'read-only screen', 'before', 'claim')
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.ship_artifact)" `
        -Section '#### Step 4.2: Delegate to Build Feature' `
        -Required @($taskCommand, $canonicalBlock, 'verbatim', $claimInput, $lintModeInput)
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.ship_artifact)" `
        -Section '#### Step 4.3: Quality Gates' `
        -Required @($taskCommand, 'native', 'non-vacu', $claimInput, 'task_owned_delta', $lintModeInput, $strictMode, $baselineMode, $globalCommand) `
        -Forbidden @("1. **Lint**: ``$globalCommand``")
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.ship_artifact)" `
        -Section '#### Step 4.6: Wave Convergence Gate (P-002.6)' `
        -Required @($globalCommand, $baselineMode, $strictMode, 'intermediate_wave_lint_cmd', 'WAVE_BASELINE_LINT_UNCLOSED', 'zero findings')
    Test-SectionTerms -Scenario $sc -Artifact "$($contract.policy_artifact)" `
        -Section '### P-002.6 — Dependency-Aware Harness Waves (Scheduling Contract)' `
        -Required @(
            $baselineBlock, $baselineMode, $strictMode, 'baseline_inventory',
            'scripts/verify-baseline-lint.ps1', 'operator-supplied record',
            'member_scope', 'task_artifact_sha256', 'BASELINE-LINT-RESIDUAL-OK',
            'WAVE_BASELINE_LINT_CONTRACT_INVALID', 'WAVE_BASELINE_LINT_EXEC_FAILED',
            'WAVE_BASELINE_LINT_RESIDUAL_MISMATCH', 'WAVE_BASELINE_LINT_UNCLOSED',
            'golangci-lint run', 'terminal boundary'
        )
}

# --- fixture load --------------------------------------------------------------
if (-not (Test-Path $Fixture)) { throw "fixture not found: $Fixture" }
$fx = Get-Content $Fixture -Raw | ConvertFrom-Json -DateKind String

function ConvertTo-List {
    param($Value)
    if ($null -eq $Value) { return , @() }
    return , @($Value)
}

function Get-PropertyNames {
    param($Object)
    if ($null -eq $Object) { return @() }
    return @($Object.PSObject.Properties | ForEach-Object { $_.Name })
}

function Test-HasProperty {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $false }
    return [bool](@($Object.PSObject.Properties | ForEach-Object { $_.Name }) -contains $Name)
}

function ConvertFrom-SimpleYamlScalar {
    param([string]$Text)
    $value = $Text.Trim()
    if ($value -match '^(.*?)(?:\s+#.*)?$') { $value = $Matches[1].Trim() }
    if ($value.Length -ge 2 -and $value.StartsWith('"') -and $value.EndsWith('"')) {
        try { return ($value | ConvertFrom-Json) }
        catch { return $value.Substring(1, $value.Length - 2) }
    }
    if ($value.Length -ge 2 -and $value.StartsWith("'") -and $value.EndsWith("'")) {
        return $value.Substring(1, $value.Length - 2).Replace("''", "'")
    }
    if ($value -ceq 'true') { return $true }
    if ($value -ceq 'false') { return $false }
    return $value
}

function Read-WorkspaceStatusCatalog {
    param([string]$Path)
    $errors = [System.Collections.Generic.List[string]]::new()
    if (-not (Test-Path $Path)) {
        $errors.Add("workspace status catalog not found: $Path")
        return [pscustomobject]@{ Values = @(); Errors = @($errors) }
    }

    $lines = @(Get-Content $Path)
    $fields = @(
        for ($i = 0; $i -lt $lines.Count; $i++) {
            if ($lines[$i] -match '^fields:\s*(?:#.*)?$') { $i }
        }
    )
    if ($fields.Count -ne 1) {
        $errors.Add("fields block count is $($fields.Count), expected 1")
        return [pscustomobject]@{ Values = @(); Errors = @($errors) }
    }

    $fieldEnd = $lines.Count
    for ($i = $fields[0] + 1; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match '^\S') { $fieldEnd = $i; break }
    }
    $status = @(
        for ($i = $fields[0] + 1; $i -lt $fieldEnd; $i++) {
            if ($lines[$i] -match '^\s{2}status:\s*(?:#.*)?$') { $i }
        }
    )
    if ($status.Count -ne 1) {
        $errors.Add("fields.status block count is $($status.Count), expected 1")
        return [pscustomobject]@{ Values = @(); Errors = @($errors) }
    }

    $statusEnd = $fieldEnd
    for ($i = $status[0] + 1; $i -lt $fieldEnd; $i++) {
        if ($lines[$i] -match '^\s{2}\S') { $statusEnd = $i; break }
    }
    $valuesLine = @(
        for ($i = $status[0] + 1; $i -lt $statusEnd; $i++) {
            if ($lines[$i] -match '^\s{4}values:\s*(?:#.*)?$') { $i }
        }
    )
    if ($valuesLine.Count -ne 1) {
        $errors.Add("fields.status.values block count is $($valuesLine.Count), expected 1")
        return [pscustomobject]@{ Values = @(); Errors = @($errors) }
    }

    $values = @()
    for ($i = $valuesLine[0] + 1; $i -lt $statusEnd; $i++) {
        if ($lines[$i] -match '^\s{6}-\s+(.+?)\s*$') {
            $values += "$(ConvertFrom-SimpleYamlScalar $Matches[1])"
            continue
        }
        if ($lines[$i] -match '^\s{0,4}\S') { break }
    }
    if ($values.Count -eq 0) { $errors.Add('fields.status.values is empty') }
    if (@($values | Sort-Object -Unique).Count -ne $values.Count) {
        $errors.Add('fields.status.values contains a duplicate')
    }
    return [pscustomobject]@{ Values = @($values); Errors = @($errors) }
}

function Read-TopLevelYamlMapping {
    param([string]$Path, [string]$Name)
    $errors = [System.Collections.Generic.List[string]]::new()
    if (-not (Test-Path $Path)) {
        $errors.Add("registry file not found: $Path")
        return [pscustomobject]@{ Entries = [pscustomobject]@{}; Errors = @($errors) }
    }

    $lines = @(Get-Content $Path)
    $heads = @(
        for ($i = 0; $i -lt $lines.Count; $i++) {
            if ($lines[$i] -match ('^{0}:\s*(?:#.*)?$' -f [regex]::Escape($Name))) { $i }
        }
    )
    if ($heads.Count -ne 1) {
        $errors.Add("$Name block count is $($heads.Count), expected 1")
        return [pscustomobject]@{ Entries = [pscustomobject]@{}; Errors = @($errors) }
    }

    $entries = [ordered]@{}
    for ($i = $heads[0] + 1; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        if ([string]::IsNullOrWhiteSpace($line) -or $line -match '^\s*#') { continue }
        if ($line -match '^\S') { break }
        if ($line -notmatch '^\s{2}([A-Za-z0-9_-]+):\s*(.*?)\s*$') {
            $errors.Add("unparseable $Name mapping line: $line")
            continue
        }
        $key = $Matches[1]
        if ($entries.Contains($key)) {
            $errors.Add("duplicate $Name key: $key")
            continue
        }
        $entries[$key] = ConvertFrom-SimpleYamlScalar $Matches[2]
    }
    if ($entries.Count -eq 0) { $errors.Add("$Name mapping is empty") }
    return [pscustomobject]@{ Entries = [pscustomobject]$entries; Errors = @($errors) }
}

function Get-LiveStatusSourceProjection {
    param([string]$Dir)
    $workspaceDir = Split-Path -Parent $Dir
    $root = Split-Path -Parent $workspaceDir
    $catalog = Read-WorkspaceStatusCatalog -Path (Join-Path $workspaceDir 'config.yaml')
    $registryPath = Join-Path $root '.autoharness/backlog-registry.yaml'
    $mapping = Read-TopLevelYamlMapping -Path $registryPath -Name 'status_values'
    $allFeatures = Read-TopLevelYamlMapping -Path $registryPath -Name 'features'
    $errors = @($catalog.Errors + $mapping.Errors + $allFeatures.Errors)

    $features = [ordered]@{}
    foreach ($name in @('sql_query', 'shipments')) {
        if (-not (Test-HasProperty $allFeatures.Entries $name)) {
            $errors += "features.$name is absent"
            continue
        }
        $value = $allFeatures.Entries.$name
        if ($value -isnot [bool]) {
            $errors += "features.$name is not a boolean"
            continue
        }
        $features[$name] = $value
    }

    return [pscustomobject]@{
        status_catalog         = @($catalog.Values)
        registry_status_mapping = $mapping.Entries
        registry_features      = [pscustomobject]$features
        source_errors          = @($errors)
    }
}

function Format-KeyValueProjection {
    param($Object)
    if ($null -eq $Object) { return @() }
    return @(
        Get-PropertyNames $Object |
            Sort-Object |
            ForEach-Object { "$_=$(Format-Value $Object.$_)" }
    )
}

# --- optional live-shipment drift check ---------------------------------------
function Find-ArtifactPath {
    param([string]$Dir, [string]$Id)
    if ($Id -notmatch '^[A-Za-z0-9.-]+$') { return $null }
    $rootFull = [System.IO.Path]::GetFullPath($repoRoot).TrimEnd('\', '/')
    $dirFull = [System.IO.Path]::GetFullPath($Dir).TrimEnd('\', '/')
    if (-not $dirFull.StartsWith("$rootFull$([System.IO.Path]::DirectorySeparatorChar)",
            [System.StringComparison]::OrdinalIgnoreCase)) {
        return $null
    }
    $relativeDir = [System.IO.Path]::GetRelativePath($rootFull, $dirFull)
    $cursor = $rootFull
    foreach ($segment in ($relativeDir -split '[\\/]')) {
        $cursor = Join-Path $cursor $segment
        if (-not (Test-Path $cursor)) { return $null }
        if (((Get-Item $cursor -Force).Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            return $null
        }
    }
    $workspaceDir = Split-Path -Parent $dirFull
    $candidates = @(
        (Join-Path $dirFull "$Id.md"),
        (Join-Path (Join-Path $workspaceDir 'archive') "$Id.md")
    )
    $found = @(
        $candidates | Where-Object {
            if (-not (Test-Path $_ -PathType Leaf)) { return $false }
            $candidateCursor = $rootFull
            foreach ($segment in ([System.IO.Path]::GetRelativePath($rootFull, $_) -split '[\\/]')) {
                $candidateCursor = Join-Path $candidateCursor $segment
                $item = Get-Item $candidateCursor -Force
                if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
                    return $false
                }
            }
            return $true
        }
    )
    if ($found.Count -ne 1) { return $null }
    return $found[0]
}

function Get-DelimitedContractBlock {
    param([string]$Raw, [string]$Name, [string]$Fence)
    $begin = "<!-- BEGIN:$Name -->"
    $end = "<!-- END:$Name -->"
    $beginCount = [regex]::Matches($Raw, "(?m)^\s*$([regex]::Escape($begin))\s*$").Count
    $endCount = [regex]::Matches($Raw, "(?m)^\s*$([regex]::Escape($end))\s*$").Count
    if ($beginCount -eq 0) {
        return [pscustomobject]@{ Present = $false; Content = ''; Errors = @() }
    }
    $pattern = '(?s){0}\s*```{1}\s*(.*?)\s*```\s*{2}' -f
        [regex]::Escape($begin), [regex]::Escape($Fence), [regex]::Escape($end)
    $matches = [regex]::Matches($Raw, $pattern)
    $errors = @()
    if ($beginCount -ne 1) { $errors += "$Name block count is $beginCount, expected 1" }
    if ($endCount -ne 1) { $errors += "$Name end-marker count is $endCount, expected 1" }
    if ($matches.Count -ne 1) { $errors += "$Name block is malformed or missing its $Fence fence" }
    $content = if ($matches.Count -eq 1) { $matches[0].Groups[1].Value.Trim() } else { '' }
    return [pscustomobject]@{ Present = $true; Content = $content; Errors = $errors }
}

function Read-TaskLintContract {
    param([string]$Raw)
    $block = Get-DelimitedContractBlock -Raw $Raw -Name 'task-lint-contract' -Fence 'json'
    if (-not $block.Present) {
        return [pscustomobject]@{ Contract = $null; Errors = @('task-lint-contract is missing') }
    }
    $errors = @($block.Errors)
    $contract = $null
    try { $contract = $block.Content | ConvertFrom-Json -DateKind String -ErrorAction Stop }
    catch { $errors += "task-lint JSON is invalid: $($_.Exception.Message)" }
    if ($null -ne $contract) {
        $expected = @('task_lint_cmd', 'lint_scope', 'non_vacuity_evidence')
        if (((Get-PropertyNames $contract) -join ',') -cne ($expected -join ',')) {
            $errors += 'task-lint keys/order must be task_lint_cmd,lint_scope,non_vacuity_evidence'
        }
        $errors += Test-TaskLintContractShape -Contract $contract
    }
    return [pscustomobject]@{ Contract = $contract; Errors = @($errors) }
}

function Test-TaskLintContractShape {
    param($Contract)
    $errors = @()
    if ($null -eq $Contract) { return 'contract is null' }
    $cmd = if (Test-HasProperty $Contract 'task_lint_cmd') { "$($Contract.task_lint_cmd)" } else { '' }
    if ([string]::IsNullOrWhiteSpace($cmd)) { $errors += 'task_lint_cmd is empty' }
    if ($cmd.Trim() -ceq 'golangci-lint run') { $errors += 'task_lint_cmd is bare global lint' }
    if ($cmd -match '\|\|\s*true') { $errors += 'task_lint_cmd masks native failure' }
    if ($cmd -match '(?i)\$nativeExit\s*=\s*0') { $errors += 'task_lint_cmd hard-codes native success' }
    if ($cmd -notmatch '(?i)(ExitCode|LASTEXITCODE|nativeExit)' -or $cmd -notmatch '(?i)\bexit\b') {
        $errors += 'task_lint_cmd does not preserve native failure'
    }
    $scope = if (Test-HasProperty $Contract 'lint_scope') { $Contract.lint_scope } else { $null }
    $target = ''
    if ($null -ne $scope) {
        if (Test-HasProperty $scope 'owned_file') { $target = "$($scope.owned_file)" }
        elseif (Test-HasProperty $scope 'owned_delta_path') { $target = "$($scope.owned_delta_path)" }
    }
    if ([string]::IsNullOrWhiteSpace($target)) { $errors += 'lint_scope has no exact owned target' }
    elseif ($cmd.IndexOf($target, [System.StringComparison]::Ordinal) -lt 0) {
        $errors += 'task_lint_cmd does not bind the exact owned target'
    }
    $evidence = if (Test-HasProperty $Contract 'non_vacuity_evidence') {
        $Contract.non_vacuity_evidence
    } else { $null }
    $marker = if ($null -ne $evidence -and (Test-HasProperty $evidence 'success_marker')) {
        "$($evidence.success_marker)"
    } elseif ($null -ne $scope -and (Test-HasProperty $scope 'success_marker')) {
        "$($scope.success_marker)"
    } else { '' }
    if ([string]::IsNullOrWhiteSpace($marker)) { $errors += 'non_vacuity_evidence success_marker is empty' }
    elseif ($cmd.IndexOf($marker, [System.StringComparison]::Ordinal) -lt 0) {
        $errors += 'task_lint_cmd does not emit its exact non-vacuity marker'
    }
    $kind = if ($null -ne $scope -and (Test-HasProperty $scope 'kind')) { "$($scope.kind)" } else { '' }
    if ($kind -ceq 'no-go-lint-surface') {
        $expectedScopeKeys = @('kind', 'owned_delta_path', 'package', 'linter', 'success_marker')
        $expectedEvidenceKeys = @('mode', 'proof', 'distinct_exits', 'native_failure_preserved')
        if (((Get-PropertyNames $scope) -join ',') -cne ($expectedScopeKeys -join ',') -or
            "$($scope.package)" -or "$($scope.linter)" -or
            "$($evidence.mode)" -cne 'positive-no-go-surface-proof' -or
            ((Get-PropertyNames $evidence) -join ',') -cne ($expectedEvidenceKeys -join ',')) {
            $errors += 'no-Go task lint inner schema is not canonical'
        }
        if ($cmd -notmatch '(?i)\bgit\b' -or $cmd -notmatch '(?i)\bstatus\b' -or
            $cmd -notmatch '(?i)\bdiff\b' -or $cmd -notmatch '(?i)\.go') {
            $errors += 'no-Go task lint command does not prove its exact delta'
        }
    }
    elseif ($kind -ceq 'file-scoped-go-lint' -or $kind -ceq 'harness-file-scoped-go-lint') {
        $expectedScopeKeys = @('kind', 'package', 'linter', 'owned_file')
        $expectedEvidenceKeys = @('mode', 'success_marker', 'asserts')
        if (((Get-PropertyNames $scope) -join ',') -cne ($expectedScopeKeys -join ',') -or
            [string]::IsNullOrWhiteSpace("$($scope.package)") -or
            "$($evidence.mode)" -cne 'structured-json-schema-validated' -or
            ((Get-PropertyNames $evidence) -join ',') -cne ($expectedEvidenceKeys -join ',') -or
            @($evidence.asserts).Count -eq 0) {
            $errors += 'Go task lint inner schema is not canonical'
        }
        if ($kind -ceq 'file-scoped-go-lint' -and [string]::IsNullOrWhiteSpace("$($scope.linter)")) {
            $errors += 'file-scoped task lint linter is empty'
        }
        if ($kind -ceq 'harness-file-scoped-go-lint' -and $null -ne $scope.linter) {
            $errors += 'harness-file task lint linter must be null'
        }
        if ($cmd -notmatch '(?i)\bgolangci-lint\b' -or
            $cmd -notmatch 'ProcessStartInfo' -or $cmd -notmatch 'ConvertFrom-Json' -or
            $cmd -notmatch '\bIssues\b' -or $cmd -notmatch 'RedirectStandardOutput' -or
            $cmd -notmatch 'RedirectStandardError' -or $cmd -notmatch 'ReadToEndAsync' -or
            $cmd -notmatch 'WaitForExit') {
            $errors += 'task lint command does not execute and parse structured native lint'
        }
    }
    else { $errors += "unknown task lint scope kind: $kind" }
    return $errors
}

function Get-LintIdentity {
    param($Finding)
    return @(
        "$($Finding.path)", "$($Finding.line)", "$($Finding.column)",
        "$($Finding.linter)", "$($Finding.message)", "$($Finding.owner_task_id)"
    ) -join '|'
}

function Get-LintFindingKey {
    param($Finding)
    return @(
        "$($Finding.path)", "$($Finding.line)", "$($Finding.column)",
        "$($Finding.linter)", "$($Finding.message)"
    ) -join '|'
}

function Copy-JsonObject {
    param($Value)
    return ($Value | ConvertTo-Json -Depth 30 | ConvertFrom-Json -DateKind String)
}

function Get-BaselineMemberScopeObservation {
    param($Contract)
    if ($null -eq $Contract -or -not (Test-HasProperty $Contract 'member_scope')) {
        return [pscustomobject]@{ committed = $false; digests_match = $false }
    }
    $allCommitted = $true
    $allMatch = $true
    foreach ($binding in @($Contract.member_scope)) {
        $taskID = "$($binding.task_id)"
        if ($taskID -cnotmatch '^\d+\.\d{3}-T$') {
            $allCommitted = $false
            $allMatch = $false
            continue
        }
        $relative = ".backlogit/queue/$taskID.md"
        $full = Join-Path $repoRoot $relative
        if (-not (Test-Path $full -PathType Leaf)) {
            $allCommitted = $false
            $allMatch = $false
            continue
        }
        $item = Get-Item $full -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            $allCommitted = $false
            $allMatch = $false
            continue
        }
        & git -C $repoRoot ls-files --error-unmatch -- $relative 2>$null | Out-Null
        $tracked = [bool]($LASTEXITCODE -eq 0)
        & git -C $repoRoot diff --quiet HEAD -- $relative
        $workingClean = [bool]($LASTEXITCODE -eq 0)
        & git -C $repoRoot diff --cached --quiet HEAD -- $relative
        $indexClean = [bool]($LASTEXITCODE -eq 0)
        $allCommitted = [bool]($allCommitted -and $tracked -and $workingClean -and $indexClean)
        $actual = (Get-FileHash $full -Algorithm SHA256).Hash.ToLowerInvariant()
        $allMatch = [bool]($allMatch -and
            $actual -ceq "$($binding.task_artifact_sha256)")
    }
    return [pscustomobject]@{ committed = $allCommitted; digests_match = $allMatch }
}

function Test-BaselineLintContractShape {
    param(
        $Contract,
        [string[]]$Members,
        $InventoryObservation,
        [string]$ExpectedShipmentID,
        [string]$ExpectedReleaseUnitID,
        [string]$UniqueTerminalSink,
        $AuthorizationObservation,
        $MemberScopeObservation
    )
    $errors = @()
    $expectedIdentityFields = @('path', 'line', 'column', 'linter', 'message', 'owner_task_id')
    if ($null -eq $Contract) { return 'contract is null' }
    if ("$($Contract.mode)" -cne 'baseline-convergence') { $errors += 'mode is invalid' }
    if ("$($Contract.purpose)" -cne 'known-global-lint-baseline-removal') { $errors += 'purpose is invalid' }
    if ("$($Contract.shipment_id)" -cne $ExpectedShipmentID -or
        "$($Contract.release_unit_id)" -cne $ExpectedReleaseUnitID) {
        $errors += 'release identity does not match its enclosing artifacts'
    }
    if ($Members -notcontains "$($Contract.terminal_task_id)" -or
        "$($Contract.terminal_task_id)" -cne $UniqueTerminalSink) {
        $errors += 'terminal task is outside M or is not the unique terminal sink'
    }
    $bindings = if (Test-HasProperty $Contract 'member_scope') { @($Contract.member_scope) } else { @() }
    $bindingIDs = @()
    $roleByTask = @{}
    foreach ($binding in $bindings) {
        $bindingKeys = @('task_id', 'role', 'task_artifact_sha256')
        $taskID = "$($binding.task_id)"
        $role = "$($binding.role)"
        if (((Get-PropertyNames $binding) -join ',') -cne ($bindingKeys -join ',') -or
            $taskID -cnotmatch '^\d+\.\d{3}-T$' -or
            $role -cnotmatch '^(baseline-control|finding-remediation|terminal-convergence)$' -or
            "$($binding.task_artifact_sha256)" -cnotmatch '^[0-9a-f]{64}$') {
            $errors += "member scope binding is malformed: $taskID"
        }
        $bindingIDs += $taskID
        $roleByTask[$taskID] = $role
    }
    if (-not (Test-ExactOrdinalSet -Expected $Members -Actual $bindingIDs) -or
        @($bindingIDs | Sort-Object -Unique).Count -ne $bindingIDs.Count) {
        $errors += 'member scope does not bind exact M'
    }
    $terminalRoles = @($bindings | Where-Object { "$($_.role)" -ceq 'terminal-convergence' })
    if ($terminalRoles.Count -ne 1 -or
        "$($terminalRoles[0].task_id)" -cne "$($Contract.terminal_task_id)") {
        $errors += 'member scope terminal role is not exact'
    }
    if ($null -eq $MemberScopeObservation -or -not [bool]$MemberScopeObservation.committed -or
        -not [bool]$MemberScopeObservation.digests_match) {
        $errors += 'member task artifact digest is stale or uncommitted'
    }
    $inventory = if (Test-HasProperty $Contract 'baseline_inventory') { $Contract.baseline_inventory } else { $null }
    if ($null -eq $inventory) { $errors += 'baseline inventory is missing' }
    else {
        $inventoryKeys = @('path', 'sha256', 'finding_count', 'identity_fields')
        if (((Get-PropertyNames $inventory) -join ',') -cne ($inventoryKeys -join ',')) {
            $errors += 'baseline inventory keys/order are not canonical'
        }
        if ([string]::IsNullOrWhiteSpace("$($inventory.path)")) { $errors += 'inventory path is missing' }
        if ("$($inventory.sha256)" -cnotmatch '^[0-9a-f]{64}$') { $errors += 'inventory digest is malformed' }
        if ((@($inventory.identity_fields) -join ',') -cne ($expectedIdentityFields -join ',')) {
            $errors += 'inventory identity fields are not exact'
        }
        if ($null -eq $InventoryObservation -or -not [bool]$InventoryObservation.committed -or
            -not [bool]$InventoryObservation.machine_readable) {
            $errors += 'inventory is not committed and machine-readable'
        }
        elseif (-not [bool]$InventoryObservation.digest_matches) { $errors += 'inventory digest is stale' }
        $findings = if ($null -ne $InventoryObservation) { @($InventoryObservation.findings) } else { @() }
        if ($findings.Count -eq 0 -or ([int]$inventory.finding_count) -ne $findings.Count) {
            $errors += 'inventory finding count is empty or mismatched'
        }
        $ids = @()
        $findingKeys = @()
        foreach ($finding in $findings) {
            $id = Get-LintIdentity -Finding $finding
            $ids += $id
            $findingKeys += Get-LintFindingKey -Finding $finding
            if ([string]::IsNullOrWhiteSpace("$($finding.path)") -or
                ([int]$finding.line) -lt 1 -or ([int]$finding.column) -lt 1 -or
                [string]::IsNullOrWhiteSpace("$($finding.linter)") -or
                [string]::IsNullOrWhiteSpace("$($finding.message)") -or
                $Members -notcontains "$($finding.owner_task_id)") {
                $errors += "malformed or unowned finding: $id"
            }
            if ($roleByTask["$($finding.owner_task_id)"] -cne 'finding-remediation') {
                $errors += "finding owner is not a remediation member: $id"
            }
        }
        if (@($ids | Sort-Object -Unique).Count -ne $ids.Count -or
            @($findingKeys | Sort-Object -Unique).Count -ne $findingKeys.Count) {
            $errors += 'duplicate or multiply owned finding identity'
        }
        foreach ($binding in $bindings) {
            $ownedCount = @($findings | Where-Object {
                    "$($_.owner_task_id)" -ceq "$($binding.task_id)"
                }).Count
            if (("$($binding.role)" -ceq 'finding-remediation' -and $ownedCount -eq 0) -or
                ("$($binding.role)" -cne 'finding-remediation' -and $ownedCount -ne 0)) {
                $errors += "member role and finding ownership disagree: $($binding.task_id)"
            }
        }
    }
    $intermediate = "$($Contract.intermediate_wave_lint_cmd)"
    $expectedIntermediate = if ($null -ne $inventory) {
        'pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 ' +
            "-Inventory $($inventory.path) -InventorySha256 $($inventory.sha256) " +
            "-Shipment $($Contract.shipment_id) -TerminalTask $($Contract.terminal_task_id)"
    } else { '' }
    $verifierPath = Join-Path $repoRoot 'scripts/verify-baseline-lint.ps1'
    $verifierText = if (Test-Path $verifierPath) { Get-Content $verifierPath -Raw } else { '' }
    $verifierReady = $false
    if (Test-Path $verifierPath -PathType Leaf) {
        $item = Get-Item $verifierPath -Force
        $reparse = [bool](($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0)
        & git -C $repoRoot ls-files --error-unmatch -- scripts/verify-baseline-lint.ps1 2>$null | Out-Null
        $tracked = [bool]($LASTEXITCODE -eq 0)
        & git -C $repoRoot diff --quiet HEAD -- scripts/verify-baseline-lint.ps1
        $workingClean = [bool]($LASTEXITCODE -eq 0)
        & git -C $repoRoot diff --cached --quiet HEAD -- scripts/verify-baseline-lint.ps1
        $indexClean = [bool]($LASTEXITCODE -eq 0)
        $tokens = $null
        $parseErrors = $null
        [void][System.Management.Automation.Language.Parser]::ParseFile(
            $verifierPath, [ref]$tokens, [ref]$parseErrors
        )
        $verifierReady = [bool](
            -not $reparse -and $tracked -and $workingClean -and $indexClean -and
            $parseErrors.Count -eq 0
        )
    }
    if ($intermediate -cne $expectedIntermediate -or
        -not $verifierReady -or
        $verifierText -notmatch 'golangci-lint' -or
        $verifierText -notmatch 'ConvertFrom-Json' -or
        $verifierText -notmatch '--output\.text\.path' -or
        $verifierText -notmatch '--max-issues-per-linter' -or
        $verifierText -notmatch '--max-same-issues' -or
        $verifierText -notmatch '--uniq-by-line=false' -or
        $verifierText -notmatch 'unsupported-member-status' -or
        $verifierText -notmatch '\^\(queued\|active\|blocked\|done\)\$' -or
        $verifierText -notmatch 'BASELINE-RESIDUAL-MISMATCH' -or
        $verifierText -notmatch 'BASELINE-LINT-RESIDUAL-OK') {
        $errors += 'intermediate wave command is missing, vacuous, or failure-masking'
    }
    if ("$($Contract.terminal_global_lint_cmd)" -cne 'golangci-lint run') {
        $errors += 'terminal global lint command is not exact'
    }
    $auth = if (Test-HasProperty $Contract 'operator_authorization') {
        $Contract.operator_authorization
    } else { $null }
    $authRecord = if ($null -ne $auth -and (Test-HasProperty $auth 'record')) { "$($auth.record)" } else { '' }
    $authAt = if ($null -ne $auth -and (Test-HasProperty $auth 'authorized_at')) { "$($auth.authorized_at)" } else { '' }
    $authBy = if ($null -ne $auth -and (Test-HasProperty $auth 'authorized_by')) { "$($auth.authorized_by)" } else { '' }
    $authShipment = if ($null -ne $auth -and (Test-HasProperty $auth 'shipment_id')) { "$($auth.shipment_id)" } else { '' }
    $authScope = if ($null -ne $auth -and (Test-HasProperty $auth 'scope')) { "$($auth.scope)" } else { '' }
    $authTimeValid = [bool]($authAt -match '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$')
    $runtimeAuthValid = [bool](
        $null -ne $AuthorizationObservation -and
        [bool]$AuthorizationObservation.operator_supplied -and
        "$($AuthorizationObservation.record)" -ceq $authRecord -and
        "$($AuthorizationObservation.shipment_id)" -ceq $ExpectedShipmentID -and
        "$($AuthorizationObservation.scope)" -ceq $authScope
    )
    if ($null -eq $auth -or [string]::IsNullOrWhiteSpace($authRecord) -or
        [string]::IsNullOrWhiteSpace($authAt) -or [string]::IsNullOrWhiteSpace($authBy) -or
        -not $authTimeValid -or $authShipment -cne $ExpectedShipmentID -or
        $authScope -cne 'retain-only-unfinished-owned-baseline-findings-until-terminal' -or
        -not $runtimeAuthValid) {
        $errors += 'operator authorization is missing or out of scope'
    }
    return $errors
}

function Read-BaselineLintContract {
    param(
        [string]$Raw,
        [string[]]$Members,
        $InventoryObservation,
        [string]$ExpectedShipmentID,
        [string]$ExpectedReleaseUnitID,
        [string]$UniqueTerminalSink,
        $AuthorizationObservation,
        $MemberScopeObservation
    )
    $block = Get-DelimitedContractBlock -Raw $Raw -Name 'baseline-lint-convergence-contract' -Fence 'json'
    if (-not $block.Present) {
        return [pscustomobject]@{
            Contract = $null
            Errors = @('baseline-lint-convergence-contract is missing')
        }
    }
    $errors = @($block.Errors)
    $contract = $null
    try { $contract = $block.Content | ConvertFrom-Json -DateKind String -ErrorAction Stop }
    catch { $errors += "baseline-lint JSON is invalid: $($_.Exception.Message)" }
    if ($null -ne $contract) {
        $expectedKeys = @(
            'mode', 'purpose', 'shipment_id', 'release_unit_id', 'terminal_task_id',
            'member_scope', 'baseline_inventory', 'intermediate_wave_lint_cmd', 'terminal_global_lint_cmd',
            'operator_authorization'
        )
        if (((Get-PropertyNames $contract) -join ',') -cne ($expectedKeys -join ',')) {
            $errors += 'baseline-lint keys/order are not canonical'
        }
        if ($null -eq $MemberScopeObservation) {
            $MemberScopeObservation = Get-BaselineMemberScopeObservation -Contract $contract
        }
        $errors += Test-BaselineLintContractShape -Contract $contract -Members $Members `
            -InventoryObservation $InventoryObservation -ExpectedShipmentID $ExpectedShipmentID `
            -ExpectedReleaseUnitID $ExpectedReleaseUnitID -UniqueTerminalSink $UniqueTerminalSink `
            -AuthorizationObservation $AuthorizationObservation `
            -MemberScopeObservation $MemberScopeObservation
    }
    return [pscustomobject]@{ Contract = $contract; Errors = @($errors) }
}

function Get-BaselineInventoryObservation {
    param($Contract)
    $empty = [pscustomobject]@{
        digest_matches = $false
        committed = $false
        machine_readable = $false
        findings = @()
    }
    if ($null -eq $Contract -or -not (Test-HasProperty $Contract 'baseline_inventory')) {
        return $empty
    }
    $relative = "$($Contract.baseline_inventory.path)" -replace '\\', '/'
    if ([string]::IsNullOrWhiteSpace($relative) -or [System.IO.Path]::IsPathRooted($relative) -or
        $relative -match '(^|/)\.\.(/|$)') {
        return $empty
    }
    $rootFull = [System.IO.Path]::GetFullPath($repoRoot).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath((Join-Path $rootFull $relative))
    if (-not $full.StartsWith("$rootFull$([System.IO.Path]::DirectorySeparatorChar)",
            [System.StringComparison]::OrdinalIgnoreCase)) {
        return $empty
    }
    if (-not (Test-Path $full -PathType Leaf)) { return $empty }
    $cursor = $rootFull
    foreach ($segment in ($relative -split '/')) {
        $cursor = Join-Path $cursor $segment
        $item = Get-Item $cursor -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) { return $empty }
    }
    & git -C $repoRoot ls-files --error-unmatch -- $relative 2>$null | Out-Null
    $tracked = [bool]($LASTEXITCODE -eq 0)
    & git -C $repoRoot diff --quiet HEAD -- $relative
    $workingClean = [bool]($LASTEXITCODE -eq 0)
    & git -C $repoRoot diff --cached --quiet HEAD -- $relative
    $indexClean = [bool]($LASTEXITCODE -eq 0)
    $committed = [bool]($tracked -and $workingClean -and $indexClean)
    $digest = (Get-FileHash $full -Algorithm SHA256).Hash.ToLowerInvariant()
    $parsed = $null
    try { $parsed = Get-Content $full -Raw | ConvertFrom-Json -DateKind String -ErrorAction Stop }
    catch { return [pscustomobject]@{
            digest_matches = [bool]($digest -ceq "$($Contract.baseline_inventory.sha256)")
            committed = $committed
            machine_readable = $false
            findings = @()
        } }
    $findings = if ($parsed -is [System.Array]) { @($parsed) }
    elseif (Test-HasProperty $parsed 'findings') { @($parsed.findings) }
    else { @() }
    return [pscustomobject]@{
        digest_matches = [bool]($digest -ceq "$($Contract.baseline_inventory.sha256)")
        committed = $committed
        machine_readable = [bool]($findings.Count -gt 0)
        findings = $findings
    }
}

function Get-BaselineLintGateOutcome {
    param($Root, $Control)
    if ((Test-HasProperty $Control 'remove_contract') -and [bool]$Control.remove_contract) {
        if ((Test-HasProperty $Control 'baseline_requested') -and [bool]$Control.baseline_requested) {
            return 'WAVE_BASELINE_LINT_CONTRACT_INVALID'
        }
        if ([bool]$Control.global_lint_ok) { return 'PASS' }
        return 'STRICT_GLOBAL_LINT_FAILED'
    }
    $contract = Copy-JsonObject $Root.valid_contract
    $inventoryObservation = Copy-JsonObject $Root.inventory_observation
    $authorizationObservation = Copy-JsonObject $Root.authorization_observation
    $memberScopeObservation = Copy-JsonObject $Root.member_scope_observation
    $initialAttestation = Copy-JsonObject $Root.initial_attestation_observation
    if (Test-HasProperty $Control 'mode_override') { $contract.mode = "$($Control.mode_override)" }
    if (Test-HasProperty $Control 'remove') {
        [void]$contract.PSObject.Properties.Remove("$($Control.remove)")
    }
    if (Test-HasProperty $Control 'set_terminal_task_id') {
        $contract.terminal_task_id = "$($Control.set_terminal_task_id)"
    }
    if (Test-HasProperty $Control 'set_shipment_id') {
        $contract.shipment_id = "$($Control.set_shipment_id)"
    }
    if (Test-HasProperty $Control 'set_release_unit_id') {
        $contract.release_unit_id = "$($Control.set_release_unit_id)"
    }
    if (Test-HasProperty $Control 'set_authorized_at') {
        $contract.operator_authorization.authorized_at = "$($Control.set_authorized_at)"
    }
    if (Test-HasProperty $Control 'operator_supplied') {
        $authorizationObservation.operator_supplied = [bool]$Control.operator_supplied
    }
    if (Test-HasProperty $Control 'remove_operator_scope') {
        [void]$contract.operator_authorization.PSObject.Properties.Remove('scope')
    }
    if (Test-HasProperty $Control 'set_digest_matches') {
        $inventoryObservation.digest_matches = [bool]$Control.set_digest_matches
    }
    if (Test-HasProperty $Control 'set_member_scope_digests_match') {
        $memberScopeObservation.digests_match = [bool]$Control.set_member_scope_digests_match
    }
    if (Test-HasProperty $Control 'set_initial_attestation_exact') {
        $initialAttestation.exact_inventory = [bool]$Control.set_initial_attestation_exact
    }
    if ("$($contract.mode)" -ceq 'strict') {
        if ([bool]$Control.global_lint_ok) { return 'PASS' }
        return 'STRICT_GLOBAL_LINT_FAILED'
    }
    $raw = @(
        '<!-- BEGIN:baseline-lint-convergence-contract -->'
        '```json'
        ($contract | ConvertTo-Json -Depth 30)
        '```'
        '<!-- END:baseline-lint-convergence-contract -->'
    ) -join "`n"
    $parsed = Read-BaselineLintContract -Raw $raw -Members @($Root.members) `
        -InventoryObservation $inventoryObservation -ExpectedShipmentID "$($Root.expected_shipment_id)" `
        -ExpectedReleaseUnitID "$($Root.expected_release_unit_id)" `
        -UniqueTerminalSink "$($Root.unique_terminal_sink)" `
        -AuthorizationObservation $authorizationObservation `
        -MemberScopeObservation $memberScopeObservation
    if ($parsed.Errors.Count -gt 0) { return 'WAVE_BASELINE_LINT_CONTRACT_INVALID' }
    if (-not [bool]$initialAttestation.executed_before_claim -or
        -not [bool]$initialAttestation.exact_inventory -or
        [int]$initialAttestation.residual_count -ne
            [int]$contract.baseline_inventory.finding_count) {
        return 'WAVE_BASELINE_LINT_CONTRACT_INVALID'
    }
    $contract = $parsed.Contract
    if (((Test-HasProperty $Control 'native_exec_ok') -and -not [bool]$Control.native_exec_ok) -or
        ((Test-HasProperty $Control 'structured_output_valid') -and
            -not [bool]$Control.structured_output_valid)) {
        return 'WAVE_BASELINE_LINT_EXEC_FAILED'
    }
    $completed = @($Control.completed)
    $expected = @(
        @($inventoryObservation.findings) |
            Where-Object { $completed -notcontains "$($_.owner_task_id)" } |
            ForEach-Object { Get-LintIdentity -Finding $_ } |
            Sort-Object
    )
    $observed = @(@($Control.observed) | ForEach-Object { "$_" } | Sort-Object)
    if (($expected -join "`n") -cne ($observed -join "`n")) {
        return 'WAVE_BASELINE_LINT_RESIDUAL_MISMATCH'
    }
    $members = @($Root.members)
    $atTerminalBoundary = [bool](
        $completed -contains "$($contract.terminal_task_id)" -and
        @($members | Where-Object { $completed -notcontains $_ }).Count -eq 0
    )
    if ($atTerminalBoundary) {
        if ($observed.Count -ne 0 -or -not [bool]$Control.global_lint_ok) {
            return 'WAVE_BASELINE_LINT_UNCLOSED'
        }
        return 'PASS'
    }
    return 'CONVERGING'
}

function Invoke-BaselineLintContractControls {
    foreach ($control in @($fx.baseline_lint_contract_control.controls)) {
        $actual = Get-BaselineLintGateOutcome -Root $fx.baseline_lint_contract_control -Control $control
        Test-Equal -Scenario "baseline_lint/$($control.id)" -Name 'gate outcome' `
            -Expected "$($control.expect)" -Actual $actual
    }
}

function Get-GovernedClaimDeltaOutcome {
    param($Control)
    if ("$($Control.task_id)" -notmatch '^\d+\.\d{3}-T$' -or
        -not (Test-HasProperty $Control 'exempt_baseline_sha') -or
        (Test-HasProperty $Control 'baseline_sha') -or
        "$($Control.exempt_baseline_sha)" -notmatch '^[0-9a-f]{40}$') {
        return 'EXEMPT_CLAIM_DELTA_INVALID'
    }
    $paths = @(Sort-Ordinal @($Control.paths))
    $derived = @(Sort-Ordinal @($Control.derived_paths))
    $transition = $Control.validated_transitions
    $canonical = @(".backlogit/queue/$($Control.task_id).md")
    if (([int]$transition.event_append_count) -eq 1) {
        $canonical += '.backlogit/hooks_queue.jsonl'
    }
    $canonical = @(Sort-Ordinal $canonical)
    $beforeKeys = @(Sort-Ordinal (Get-PropertyNames $Control.before_sha256_by_path))
    $afterKeys = @(Sort-Ordinal (Get-PropertyNames $Control.after_sha256_by_path))
    $digestShapeValid = [bool](
        ($beforeKeys -join "`n") -ceq ($paths -join "`n") -and
        ($afterKeys -join "`n") -ceq ($paths -join "`n")
    )
    foreach ($path in $paths) {
        if ("$($Control.before_sha256_by_path.$path)" -cnotmatch '^[0-9a-f]{64}$' -or
            "$($Control.after_sha256_by_path.$path)" -cnotmatch '^[0-9a-f]{64}$') {
            $digestShapeValid = $false
        }
    }
    $transitionShapeValid = [bool](
        "$($transition.task_status)" -ceq 'queued->active' -and
        (([int]$transition.event_append_count) -eq 0 -or
            ([int]$transition.event_append_count) -eq 1) -and
        (([int]$transition.event_append_count) -eq 0 -or
            ("$($transition.event_task_id)" -ceq "$($Control.task_id)" -and
                "$($transition.event_status)" -ceq 'active'))
    )
    if (($paths -join "`n") -cne ($canonical -join "`n") -or
        ($derived -join "`n") -cne ($canonical -join "`n") -or
        -not $digestShapeValid -or -not $transitionShapeValid -or
        -not [bool]$Control.digests_match -or -not [bool]$Control.transitions_valid) {
        return 'EXEMPT_CLAIM_DELTA_INVALID'
    }
    $owned = @(Sort-Ordinal @($Control.task_owned_delta))
    if ($owned.Count -eq 0) { return 'EXEMPT_DELTA_EXCEEDS_CLASS' }
    if ("$($Control.exempt_class)" -ceq 'verification-only') {
        $outsideClass = @(
            $owned | Where-Object {
                $_ -notlike 'docs/closure/*' -and $_ -notlike '*_test.go'
            }
        )
        if ($outsideClass.Count -gt 0) { return 'EXEMPT_DELTA_EXCEEDS_CLASS' }
    }
    $expectedUnion = @(Sort-Ordinal ($paths + $owned))
    if (-not (Test-ExactOrdinalSet -Expected $expectedUnion `
            -Actual @($Control.observed_or_committed_paths))) {
        return 'EXEMPT_CLAIM_DELTA_INVALID'
    }
    if (-not (Test-ExactOrdinalSet -Expected $paths -Actual @($Control.pre_dispatch_paths))) {
        return 'EXEMPT_CLAIM_DELTA_INVALID'
    }
    foreach ($phaseName in @('pre_completion_paths', 'staged_paths', 'post_commit_paths')) {
        if (-not (Test-HasProperty $Control $phaseName)) { return 'EXEMPT_CLAIM_DELTA_INVALID' }
        if (-not (Test-ExactOrdinalSet -Expected $expectedUnion -Actual @($Control.$phaseName))) {
            return 'EXEMPT_CLAIM_DELTA_INVALID'
        }
    }
    return 'PASS'
}

function Invoke-GovernedClaimDeltaControls {
    foreach ($control in @($fx.governed_claim_delta_controls)) {
        $actual = Get-GovernedClaimDeltaOutcome -Control $control
        Test-Equal -Scenario "claim_delta/$($control.id)" -Name 'lifecycle outcome' `
            -Expected "$($control.expect)" -Actual $actual
    }
}

function Invoke-TaskLintContractControls {
    foreach ($control in @($fx.task_lint_contract_controls)) {
        $errors = @(Test-TaskLintContractShape -Contract $control.contract)
        Test-Equal -Scenario "task_lint/$($control.id)" -Name 'shape enforcement' `
            -Expected ([bool]$control.expect_valid) -Actual ([bool]($errors.Count -eq 0))
    }
}

function Read-RedDeliverableContract {
    param([string]$Raw)
    $block = Get-DelimitedContractBlock -Raw $Raw -Name 'red-deliverable-contract' -Fence 'text'
    if (-not $block.Present) {
        return [pscustomobject]@{ Contract = $null; Errors = @() }
    }

    $expectedKeys = @(
        'red_deliverable',
        'red_deliverable_reason',
        'red_selector_command',
        'green_maker_tasks',
        'green_maker_closes_wave'
    )
    $values = @{}
    $keys = @()
    $errors = @($block.Errors)
    foreach ($line in ($block.Content -split "\r?\n")) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        if ($line -notmatch '^([a-z_]+):\s*(.*)$') {
            $errors += "unparseable red-deliverable line: $line"
            continue
        }
        $key = $Matches[1]
        $value = $Matches[2].Trim()
        if ($values.ContainsKey($key)) { $errors += "duplicate red-deliverable key: $key" }
        $keys += $key
        $values[$key] = $value
    }
    if (($keys -join ',') -cne ($expectedKeys -join ',')) {
        $errors += "red-deliverable keys/order [$($keys -join ',')] != [$($expectedKeys -join ',')]"
    }
    foreach ($key in $expectedKeys) {
        if (-not $values.ContainsKey($key)) { $values[$key] = '' }
    }

    $closeWave = 0
    if (-not [int]::TryParse($values['green_maker_closes_wave'], [ref]$closeWave)) {
        $errors += 'green_maker_closes_wave is not an integer'
    }
    $greenMakers = @(
        $values['green_maker_tasks'] -split ',' |
            ForEach-Object { $_.Trim() } |
            Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    )
    $contract = [pscustomobject]@{
        red_deliverable          = [bool]($values['red_deliverable'] -ceq 'true')
        red_deliverable_reason   = $values['red_deliverable_reason']
        red_selector_command     = $values['red_selector_command']
        green_maker_tasks        = $greenMakers
        green_maker_closes_wave  = $closeWave
    }
    return [pscustomobject]@{ Contract = $contract; Errors = $errors }
}

function Read-GreenRegressionContract {
    param([string]$Raw)
    $block = Get-DelimitedContractBlock -Raw $Raw -Name 'green-regression-contract' -Fence 'json'
    if (-not $block.Present) {
        return [pscustomobject]@{ Commands = @(); Errors = @() }
    }

    $errors = @($block.Errors)
    $commands = @()
    try {
        $payload = $block.Content | ConvertFrom-Json -NoEnumerate
        if ($payload -is [System.Array] -or $payload -isnot [pscustomobject]) {
            $errors += 'green-regression payload must be a JSON object'
        }
        else {
            $keys = Get-PropertyNames $payload
            if (($keys -join ',') -cne 'green_regression_cmds') {
                $errors += "green-regression keys [$($keys -join ',')] != [green_regression_cmds]"
            }
            if (-not (Test-HasProperty $payload 'green_regression_cmds')) {
                $errors += 'green_regression_cmds is absent'
            }
            elseif ($payload.green_regression_cmds -isnot [System.Array]) {
                $errors += 'green_regression_cmds must be a JSON array'
            }
            else {
                $commands = @($payload.green_regression_cmds)
                if ($commands.Count -eq 0) {
                    $errors += 'green_regression_cmds must be non-empty when the block is present'
                }
                foreach ($command in $commands) {
                    if ($command -isnot [string] -or [string]::IsNullOrWhiteSpace("$command")) {
                        $errors += 'green_regression_cmds contains a non-string or empty entry'
                    }
                }
                if (@($commands | Sort-Object -Unique).Count -ne $commands.Count) {
                    $errors += 'green_regression_cmds contains a duplicate'
                }
            }
        }
    }
    catch {
        $errors += "green-regression JSON is invalid: $($_.Exception.Message)"
    }
    return [pscustomobject]@{ Commands = @($commands); Errors = $errors }
}

function Invoke-GreenRegressionParserControls {
    foreach ($control in (ConvertTo-List $fx.green_regression_parser_controls)) {
        $raw = @(
            '<!-- BEGIN:green-regression-contract -->'
            '```json'
            "$($control.payload)"
            '```'
            '<!-- END:green-regression-contract -->'
        ) -join "`n"
        $parsed = Read-GreenRegressionContract -Raw $raw
        if ([bool]$control.expect_valid) {
            $expectedCommands = @((ConvertTo-List $control.commands))
            $actual = [bool](
                $parsed.Errors.Count -eq 0 -and
                ($expectedCommands -join '||') -ceq (@($parsed.Commands) -join '||')
            )
        }
        else {
            $actual = [bool](
                @($parsed.Errors | Where-Object { $_ -like "*$($control.error_contains)*" }).Count -gt 0
            )
        }
        Test-Equal -Scenario "green_parser/$($control.id)" -Name 'shape enforcement' `
            -Expected $true -Actual $actual
    }
}

function Test-TaskScopedCommandShape {
    param([string]$Command)
    $errors = @()
    if ([string]::IsNullOrWhiteSpace($Command)) {
        return , @('command is empty')
    }
    if ($Command -match '(^|\s)\./\.\.\.(\s|$)') { $errors += 'bare ./... package selector' }
    if ($Command -notmatch '-count=1') { $errors += 'missing -count=1' }
    if ($Command -notmatch "-run\s+'?\^TestU") { $errors += 'selector not anchored to ^TestU<unit>_' }
    if ($Command -match '(^|\s)-short(\s|$)') { $errors += '-short weakening' }
    if ($Command -match '(^|\s)-tags(=|\s)') { $errors += 'added build tag' }
    if ($Command -match '\|\|\s*true') { $errors += '|| true weakening' }
    return , @($errors)
}

function Get-RedDeliverableBranchOutcome {
    <#
        Classifies one build-feature Step 0.5 dispatch exactly as
        .github/skills/build-feature/SKILL.md specifies. Pure: it reads a declared
        observation record and returns the branch outcome plus the routing fact.
        P-002.2 halt codes are returned verbatim; branch-local labels carry the
        RED_DELIVERABLE_ prefix.
    #>
    param($Control)

    if (-not [bool]$Control.red_deliverable) {
        return [pscustomobject]@{ Outcome = 'GENERIC_LOOP'; EntersGenericLoop = $true }
    }

    $out = { param([string]$O) [pscustomobject]@{ Outcome = $O; EntersGenericLoop = $false } }

    # Dispatch precondition 1 - red_deliverable and harness-exempt are exclusive.
    $exemptClass = if (Test-HasProperty $Control 'exempt_class') { "$($Control.exempt_class)" } else { '' }
    $exemptCmd = if (Test-HasProperty $Control 'exempt_gate_cmd') { "$($Control.exempt_gate_cmd)" } else { '' }
    if (-not [string]::IsNullOrWhiteSpace($exemptClass) -or -not [string]::IsNullOrWhiteSpace($exemptCmd)) {
        return & $out 'WAVE_RED_MAPPING_UNRESOLVED'
    }

    # Dispatch precondition 2 - harness_cmd is the declared selector, verbatim and unweakened.
    if ("$($Control.harness_cmd)" -cne "$($Control.red_selector_command)") {
        return & $out 'WAVE_RED_MAPPING_UNRESOLVED'
    }
    if (@((Test-TaskScopedCommandShape -Command "$($Control.harness_cmd)")).Count -gt 0) {
        return & $out 'WAVE_RED_MAPPING_UNRESOLVED'
    }

    # Dispatch precondition 3 - Ship must supply the post-scaffolding baseline SHA.
    $baseline = if (Test-HasProperty $Control 'red_baseline_sha') { "$($Control.red_baseline_sha)" } else { '' }
    if ([string]::IsNullOrWhiteSpace($baseline)) {
        return & $out 'WAVE_RED_MAPPING_UNRESOLVED'
    }

    # Step 0.5a - consume and validate the harness harness-architect already scaffolded.
    if (-not [bool]$Control.compile_ok) {
        return & $out 'RED_DELIVERABLE_HARNESS_UNCOMPILABLE'
    }
    switch ("$($Control.selector_signal)") {
        'assertion_fail' { }
        'pass' { return & $out 'WAVE_RED_DELIVERABLE_EARLY_GREEN' }
        'no_tests_to_run' { return & $out 'WAVE_RED_DELIVERABLE_VACUOUS' }
        default { return & $out 'RED_DELIVERABLE_NOT_ASSERTION_RED' }
    }

    # Step 0.5b - zero-delta gate against the Ship-supplied baseline.
    # The set is the union of three passes: tracked-unstaged, tracked-staged, and UNTRACKED.
    # git diff reports only files Git already tracks, so a newly created file would otherwise
    # satisfy an "empty changed-file set" claim - the most likely way this branch could violate
    # its own contract.
    $changed = @((ConvertTo-List $Control.changed_files)) + @((ConvertTo-List $Control.untracked_files))
    $changed = @($changed | Where-Object { -not [string]::IsNullOrWhiteSpace("$_") })
    $production = @($changed | Where-Object { $_ -like '*.go' -and $_ -notlike '*_test.go' })
    if ($production.Count -gt 0) {
        return & $out 'RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED'
    }
    if ($changed.Count -gt 0) {
        return & $out 'RED_DELIVERABLE_DELTA_OUT_OF_SURFACE'
    }

    # Step 0.5c - the evidence report Ship turns into the open_red_deliverables entry.
    if (-not [bool]$Control.evidence_report_complete) {
        return & $out 'RED_DELIVERABLE_EVIDENCE_INCOMPLETE'
    }

    return & $out 'RED_DELIVERABLE_ACCEPTED'
}

function Invoke-RedDeliverableBranchControls {
    foreach ($control in (ConvertTo-List $fx.red_deliverable_branch_controls)) {
        $r = Get-RedDeliverableBranchOutcome -Control $control
        Test-Equal -Scenario "red_branch/$($control.id)" -Name 'branch outcome' `
            -Expected "$($control.expect_outcome)" -Actual $r.Outcome
        Test-Equal -Scenario "red_branch/$($control.id)" -Name 'enters generic loop' `
            -Expected ([bool]$control.expect_enters_generic_loop) -Actual $r.EntersGenericLoop
    }
}

function Get-ArtifactProjection {
    param([string]$Path)
    $raw = Get-Content $Path -Raw
    $lines = Get-Content $Path
    $inFm = $false
    $inDeps = $false
    $inLabels = $false
    $id = $null
    $artifactType = $null
    $status = $null
    $deps = @()
    $labels = @()
    foreach ($line in $lines) {
        if ($line -eq '---') {
            if (-not $inFm) { $inFm = $true; continue }
            break
        }
        if (-not $inFm) { continue }
        if ($line -match '^dependencies:\s*$') { $inDeps = $true; $inLabels = $false; continue }
        if ($line -match '^labels:\s*$') { $inLabels = $true; $inDeps = $false; continue }
        if ($line -match '^[a-z_]+:') { $inDeps = $false; $inLabels = $false }
        if ($inDeps -and $line -match '^\s+-\s+(\S+)') { $deps += $Matches[1]; continue }
        if ($inLabels -and $line -match '^\s+-\s+(\S+)') { $labels += $Matches[1]; continue }
        if ($line -match '^artifact_type:\s*(\S+)') { $artifactType = $Matches[1] }
        if ($line -match '^id:\s*(\S+)') { $id = $Matches[1] }
        if ($line -match '^status:\s*(\S+)') { $status = $Matches[1] }
    }

    $red = Read-RedDeliverableContract -Raw $raw
    $green = Read-GreenRegressionContract -Raw $raw
    $exemptClass = $null
    if ($raw -match '(?m)^harness_exemption_class:\s*(\S+)\s*$') { $exemptClass = $Matches[1] }
    return [pscustomobject]@{
        id                         = $id
        artifact_type              = $artifactType
        status                     = $status
        deps                       = @($deps | Sort-Object)
        harness_exempt             = [bool]($labels -contains 'harness-exempt')
        exempt_class               = $exemptClass
        red_deliverable            = [bool]($null -ne $red.Contract -and $red.Contract.red_deliverable)
        red_contract               = $red.Contract
        green_regression_cmds      = @($green.Commands)
        contract_errors            = @($red.Errors + $green.Errors)
    }
}

function Get-ShipmentItemIDs {
    param([string]$Path)
    $items = @()
    $inFm = $false
    $inCustomFields = $false
    $inItems = $false
    foreach ($line in (Get-Content $Path)) {
        if ($line -eq '---') {
            if (-not $inFm) { $inFm = $true; continue }
            break
        }
        if (-not $inFm) { continue }
        if ($line -match '^custom_fields:\s*$') {
            $inCustomFields = $true
            $inItems = $false
            continue
        }
        if ($line -match '^[a-z_]+:' -and $line -notmatch '^custom_fields:') {
            $inCustomFields = $false
            $inItems = $false
        }
        if ($inCustomFields -and $line -match '^\s{4}items:\s*$') { $inItems = $true; continue }
        if (-not $inItems) { continue }
        if ($line -match '^\s{8}-\s+(\S+)\s*$') { $items += $Matches[1]; continue }
        if (-not [string]::IsNullOrWhiteSpace($line) -and $line -match '^\s{0,7}\S') { break }
    }
    return , $items
}

function Get-LiveShipmentProjection {
    param([string]$Dir, [string]$ShipmentID, $FallbackTaskIDs)
    $errors = [System.Collections.Generic.List[string]]::new()
    $statusSources = Get-LiveStatusSourceProjection -Dir $Dir
    $shipmentPath = Find-ArtifactPath -Dir $Dir -Id $ShipmentID
    if ($null -eq $shipmentPath) {
        $errors.Add("shipment artifact $ShipmentID was not found exactly once")
        return [pscustomobject]@{
            shipment_items = @(); members = @(); excluded_non_task_members = @()
            fallback_frozen_task_ids = @($FallbackTaskIDs)
            status_catalog = @($statusSources.status_catalog)
            registry_status_mapping = $statusSources.registry_status_mapping
            registry_features = $statusSources.registry_features
            source_errors = @($statusSources.source_errors)
            errors = @($errors)
        }
    }

    $shipmentItems = Get-ShipmentItemIDs -Path $shipmentPath
    if ($shipmentItems.Count -eq 0) { $errors.Add("shipment $ShipmentID has no parseable custom_fields.items") }
    $members = @()
    $excluded = @()
    foreach ($itemID in $shipmentItems) {
        $path = Find-ArtifactPath -Dir $Dir -Id $itemID
        if ($null -eq $path) {
            $errors.Add("manifest item $itemID was not found exactly once")
            $excluded += [pscustomobject]@{ id = $itemID; artifact_type = '<unresolved>' }
            continue
        }
        $item = Get-ArtifactProjection -Path $path
        if ($item.id -cne $itemID) { $errors.Add("manifest ID $itemID resolved to artifact ID $($item.id)") }
        if ([string]::IsNullOrWhiteSpace($item.artifact_type)) {
            $errors.Add("manifest item $itemID has no artifact_type")
            $excluded += [pscustomobject]@{ id = $itemID; artifact_type = '<unresolved>' }
        }
        elseif ($item.artifact_type -ceq 'task') {
            $members += $item
        }
        else {
            $excluded += [pscustomobject]@{ id = $itemID; artifact_type = $item.artifact_type }
        }
    }
    return [pscustomobject]@{
        shipment_items            = @($shipmentItems)
        members                   = @($members)
        excluded_non_task_members = @($excluded)
        fallback_frozen_task_ids   = @($FallbackTaskIDs)
        status_catalog             = @($statusSources.status_catalog)
        registry_status_mapping    = $statusSources.registry_status_mapping
        registry_features          = $statusSources.registry_features
        source_errors              = @($statusSources.source_errors)
        errors                    = @($errors)
    }
}

function Add-Drift {
    param($List, [string]$Code, [string]$Detail)
    $List.Add([pscustomobject]@{ Code = $Code; Detail = $Detail })
}

function Compare-LiveShipmentProjection {
    param($Fx, $Live)
    $drift = [System.Collections.Generic.List[object]]::new()
    foreach ($errorText in (ConvertTo-List $Live.errors)) {
        Add-Drift -List $drift -Code 'PARSE_ERROR' -Detail "$errorText"
    }
    foreach ($errorText in (ConvertTo-List $Live.source_errors)) {
        Add-Drift -List $drift -Code 'STATUS_SOURCE_PARSE' -Detail "$errorText"
    }

    $fixtureMembers = ConvertTo-List $Fx.manifest.members
    $expectedMIDs = @($fixtureMembers | ForEach-Object { $_.id } | Sort-Object)
    $actualMIDs = @($Live.members | ForEach-Object { $_.id } | Sort-Object)
    $fallbackMIDs = @((ConvertTo-List $Live.fallback_frozen_task_ids) | Sort-Object)
    $fallbackUnique = @($fallbackMIDs | Sort-Object -Unique)
    $expectedExcluded = @(
        (ConvertTo-List $Fx.manifest.excluded_non_task_members) |
            ForEach-Object { "$($_.id)=$($_.artifact_type)" } |
            Sort-Object
    )
    $actualExcluded = @(
        (ConvertTo-List $Live.excluded_non_task_members) |
            ForEach-Object { "$($_.id)=$($_.artifact_type)" } |
            Sort-Object
    )
    $expectedShipmentIDs = @(
        $expectedMIDs + ((ConvertTo-List $Fx.manifest.excluded_non_task_members) | ForEach-Object { $_.id }) |
            Sort-Object
    )
    $actualShipmentIDs = @($Live.shipment_items | Sort-Object)

    if ([int]$Fx.manifest.shipment_member_count -ne $Live.shipment_items.Count) {
        Add-Drift $drift 'SHIPMENT_MEMBER_COUNT' "expected $($Fx.manifest.shipment_member_count), observed $($Live.shipment_items.Count)"
    }
    if ([int]$Fx.manifest.member_count -ne $Live.members.Count) {
        Add-Drift $drift 'WAVE_MEMBER_COUNT' "expected $($Fx.manifest.member_count), observed $($Live.members.Count)"
    }
    if (($expectedShipmentIDs -join ',') -cne ($actualShipmentIDs -join ',')) {
        Add-Drift $drift 'SHIPMENT_MEMBER_IDS' "expected [$($expectedShipmentIDs -join ',')], observed [$($actualShipmentIDs -join ',')]"
    }
    if (($expectedMIDs -join ',') -cne ($actualMIDs -join ',')) {
        Add-Drift $drift 'WAVE_MEMBER_IDS' "expected [$($expectedMIDs -join ',')], observed [$($actualMIDs -join ',')]"
    }
    if ($fallbackUnique.Count -ne $fallbackMIDs.Count) {
        Add-Drift $drift 'FROZEN_FALLBACK_DUPLICATE' "fallback contains $($fallbackMIDs.Count - $fallbackUnique.Count) duplicate ID(s)"
    }
    if (($expectedMIDs -join ',') -cne ($fallbackMIDs -join ',')) {
        Add-Drift $drift 'FROZEN_FALLBACK_IDS' "M [$($expectedMIDs -join ',')] != explicit fallback [$($fallbackMIDs -join ',')]"
    }
    if (($actualMIDs -join ',') -cne ($fallbackMIDs -join ',')) {
        Add-Drift $drift 'MANIFEST_FALLBACK_DISAGREEMENT' "manifest M [$($actualMIDs -join ',')] != explicit fallback [$($fallbackMIDs -join ',')]"
    }
    if (($expectedExcluded -join ',') -cne ($actualExcluded -join ',')) {
        Add-Drift $drift 'EXCLUDED_NON_TASKS' "expected [$($expectedExcluded -join ',')], observed [$($actualExcluded -join ',')]"
    }
    foreach ($forbiddenID in (ConvertTo-List $Fx.manifest.forbidden_wave_ids)) {
        if ($actualMIDs -contains $forbiddenID -or $fallbackMIDs -contains $forbiddenID) {
            Add-Drift $drift 'FORBIDDEN_WAVE_MEMBER' "$forbiddenID entered M or the explicit fallback set"
        }
    }

    $expectedCatalog = @((ConvertTo-List $Fx.status_model.catalog) | Sort-Object)
    $actualCatalog = @((ConvertTo-List $Live.status_catalog) | Sort-Object)
    if (($expectedCatalog -join ',') -cne ($actualCatalog -join ',')) {
        Add-Drift $drift 'STATUS_CATALOG' "expected [$($expectedCatalog -join ',')], observed [$($actualCatalog -join ',')]"
    }
    $expectedMapping = Format-KeyValueProjection $Fx.status_model.registry_status_mapping
    $actualMapping = Format-KeyValueProjection $Live.registry_status_mapping
    if (($expectedMapping -join ',') -cne ($actualMapping -join ',')) {
        Add-Drift $drift 'REGISTRY_STATUS_MAPPING' "expected [$($expectedMapping -join ',')], observed [$($actualMapping -join ',')]"
    }
    $expectedFeatures = Format-KeyValueProjection $Fx.status_model.registry_features
    $actualFeatures = Format-KeyValueProjection $Live.registry_features
    if (($expectedFeatures -join ',') -cne ($actualFeatures -join ',')) {
        Add-Drift $drift 'REGISTRY_FEATURES' "expected [$($expectedFeatures -join ',')], observed [$($actualFeatures -join ',')]"
    }

    $liveByID = @{}
    foreach ($member in $Live.members) { $liveByID[$member.id] = $member }
    foreach ($fixtureMember in $fixtureMembers) {
        if (-not $liveByID.ContainsKey($fixtureMember.id)) { continue }
        $liveMember = $liveByID[$fixtureMember.id]
        foreach ($contractError in (ConvertTo-List $liveMember.contract_errors)) {
            Add-Drift $drift 'CONTRACT_PARSE' "$($fixtureMember.id): $contractError"
        }
        if ($liveMember.status -cne $fixtureMember.status) {
            # Skip MEMBER_STATUS drift when the live member has reached terminal_success
            # (done/archived). A shipped shipment's tasks will always appear archived in
            # the real queue; the fixture retains artificial simulated states for wave
            # scheduling tests and must not be updated to archived (that would collapse
            # the wave schedule to 0). Post-shipment archival is expected, not a defect.
            $terminalSuccessTokens = @((ConvertTo-List $Fx.status_model.terminal_success))
            if ($liveMember.status -cnotin $terminalSuccessTokens) {
                Add-Drift $drift 'MEMBER_STATUS' "$($fixtureMember.id): $($liveMember.status) != $($fixtureMember.status)"
            }
        }
        $expectedDeps = @((ConvertTo-List $fixtureMember.deps) | Sort-Object)
        $actualDeps = @((ConvertTo-List $liveMember.deps) | Sort-Object)
        if (($expectedDeps -join ',') -cne ($actualDeps -join ',')) {
            Add-Drift $drift 'MEMBER_DEPS' "$($fixtureMember.id): [$($actualDeps -join ',')] != [$($expectedDeps -join ',')]"
        }
        if ([bool]$liveMember.harness_exempt -ne [bool]$fixtureMember.harness_exempt) {
            Add-Drift $drift 'MEMBER_HARNESS_EXEMPT' "$($fixtureMember.id): harness_exempt drift"
        }
        $expectedClass = if ($null -eq $fixtureMember.exempt_class) { '' } else { "$($fixtureMember.exempt_class)" }
        $actualClass = if ($null -eq $liveMember.exempt_class) { '' } else { "$($liveMember.exempt_class)" }
        if ($actualClass -cne $expectedClass) {
            Add-Drift $drift 'MEMBER_EXEMPT_CLASS' "$($fixtureMember.id): exempt_class [$actualClass] != [$expectedClass]"
        }
        if ([bool]$liveMember.red_deliverable -ne [bool]$fixtureMember.red_deliverable) {
            Add-Drift $drift 'MEMBER_RED_DELIVERABLE' "$($fixtureMember.id): red_deliverable drift"
        }
    }

    $fixtureRed = @{}
    foreach ($contract in (ConvertTo-List $Fx.red_deliverables)) { $fixtureRed[$contract.task] = $contract }
    $liveRed = @{}
    foreach ($member in $Live.members) {
        if ($null -ne $member.red_contract) { $liveRed[$member.id] = $member.red_contract }
    }
    $expectedRedIDs = @($fixtureRed.Keys | Sort-Object)
    $actualRedIDs = @($liveRed.Keys | Sort-Object)
    if (($expectedRedIDs -join ',') -cne ($actualRedIDs -join ',')) {
        Add-Drift $drift 'RED_CONTRACT_TASK_IDS' "expected [$($expectedRedIDs -join ',')], observed [$($actualRedIDs -join ',')]"
    }
    foreach ($taskID in $expectedRedIDs) {
        if (-not $liveRed.ContainsKey($taskID)) { continue }
        $expected = $fixtureRed[$taskID]
        $actual = $liveRed[$taskID]
        if ([bool]$actual.red_deliverable -ne [bool]$expected.red_deliverable) {
            Add-Drift $drift 'RED_CONTRACT_RED_DELIVERABLE' "${taskID}: red_deliverable drift"
        }
        if ($actual.red_deliverable_reason -cne $expected.red_deliverable_reason) {
            Add-Drift $drift 'RED_CONTRACT_RED_DELIVERABLE_REASON' "${taskID}: red_deliverable_reason drift"
        }
        if ($actual.red_selector_command -cne $expected.red_selector_command) {
            Add-Drift $drift 'RED_CONTRACT_RED_SELECTOR_COMMAND' "${taskID}: red_selector_command drift"
        }
        $expectedGreen = @((ConvertTo-List $expected.green_maker_tasks) | Sort-Object)
        $actualGreen = @((ConvertTo-List $actual.green_maker_tasks) | Sort-Object)
        if (($expectedGreen -join ',') -cne ($actualGreen -join ',')) {
            Add-Drift $drift 'RED_CONTRACT_GREEN_MAKER_TASKS' "${taskID}: green_maker_tasks drift"
        }
        if ([int]$actual.green_maker_closes_wave -ne [int]$expected.green_maker_closes_wave) {
            Add-Drift $drift 'RED_CONTRACT_GREEN_MAKER_CLOSES_WAVE' "${taskID}: green_maker_closes_wave drift"
        }
    }

    $expectedGreenContracts = @(
        (ConvertTo-List $Fx.green_regression_contracts) |
            ForEach-Object { "$($_.task)::$((ConvertTo-List $_.green_regression_cmds) -join '||')" } |
            Sort-Object
    )
    $actualGreenContracts = @(
        $Live.members |
            Where-Object { (ConvertTo-List $_.green_regression_cmds).Count -gt 0 } |
            ForEach-Object { "$($_.id)::$((ConvertTo-List $_.green_regression_cmds) -join '||')" } |
            Sort-Object
    )
    if (($expectedGreenContracts -join ',') -cne ($actualGreenContracts -join ',')) {
        Add-Drift $drift 'GREEN_REGRESSION_CONTRACTS' "expected [$($expectedGreenContracts -join ',')], observed [$($actualGreenContracts -join ',')]"
    }
    return , @($drift)
}

function Copy-Projection {
    param($Projection)
    return ($Projection | ConvertTo-Json -Depth 30 | ConvertFrom-Json)
}

function Apply-VerificationMutation {
    param($Projection, $Mutation)
    switch ($Mutation.operation) {
        'remove_shipment_item' {
            $Projection.shipment_items = @($Projection.shipment_items | Where-Object { $_ -cne $Mutation.item })
            $Projection.members = @($Projection.members | Where-Object { $_.id -cne $Mutation.item })
            $Projection.excluded_non_task_members = @(
                $Projection.excluded_non_task_members | Where-Object { $_.id -cne $Mutation.item }
            )
        }
        'include_excluded_in_m' {
            $excluded = @($Projection.excluded_non_task_members | Where-Object { $_.id -ceq $Mutation.item })
            if ($excluded.Count -ne 1) { throw "verification mutation $($Mutation.id) cannot find excluded item $($Mutation.item)" }
            $Projection.excluded_non_task_members = @(
                $Projection.excluded_non_task_members | Where-Object { $_.id -cne $Mutation.item }
            )
            $Projection.members = @($Projection.members) + [pscustomobject]@{
                id = $Mutation.item; artifact_type = $excluded[0].artifact_type; status = 'queued'; deps = @()
                harness_exempt = $false; exempt_class = $null; red_deliverable = $false
                red_contract = $null; green_regression_cmds = @(); contract_errors = @()
            }
        }
        'remove_excluded_report' {
            $Projection.excluded_non_task_members = @(
                $Projection.excluded_non_task_members | Where-Object { $_.id -cne $Mutation.item }
            )
        }
        'set_red_contract_key' {
            $member = @($Projection.members | Where-Object { $_.id -ceq $Mutation.task })
            if ($member.Count -ne 1 -or $null -eq $member[0].red_contract) {
                throw "verification mutation $($Mutation.id) cannot find red contract for $($Mutation.task)"
            }
            $key = "$($Mutation.key)"
            $member[0].red_contract.$key = $Mutation.value
            if ($key -ceq 'red_deliverable') { $member[0].red_deliverable = [bool]$Mutation.value }
        }
        'add_green_regression_contract' {
            $member = @($Projection.members | Where-Object { $_.id -ceq $Mutation.task })
            if ($member.Count -ne 1) {
                throw "verification mutation $($Mutation.id) cannot find task $($Mutation.task)"
            }
            $member[0].green_regression_cmds = ConvertTo-List $Mutation.commands
        }
        'remove_status_catalog_token' {
            $Projection.status_catalog = @(
                $Projection.status_catalog | Where-Object { $_ -cne $Mutation.token }
            )
        }
        'set_registry_status_mapping' {
            if (-not (Test-HasProperty $Projection.registry_status_mapping "$($Mutation.key)")) {
                throw "verification mutation $($Mutation.id) cannot find registry status key $($Mutation.key)"
            }
            $Projection.registry_status_mapping.("$($Mutation.key)") = $Mutation.value
        }
        'set_registry_feature' {
            if (-not (Test-HasProperty $Projection.registry_features "$($Mutation.key)")) {
                throw "verification mutation $($Mutation.id) cannot find registry feature $($Mutation.key)"
            }
            $Projection.registry_features.("$($Mutation.key)") = [bool]$Mutation.value
        }
        'include_fallback_id' {
            $Projection.fallback_frozen_task_ids = @($Projection.fallback_frozen_task_ids) + "$($Mutation.item)"
        }
        default { throw "unknown verification mutation operation: $($Mutation.operation)" }
    }
    return $Projection
}

function Invoke-QueueDriftCheck {
    $live = Get-LiveShipmentProjection -Dir $QueueDir -ShipmentID $fx.source.shipment `
        -FallbackTaskIDs $fx.manifest.explicit_non_shipment_task_ids
    $sc = 'queue_drift'
    $expectedMIDs = @($fx.manifest.members | ForEach-Object { $_.id } | Sort-Object)
    $actualMIDs = @($live.members | ForEach-Object { $_.id } | Sort-Object)
    $fallbackMIDs = @((ConvertTo-List $live.fallback_frozen_task_ids) | Sort-Object)
    $expectedExcluded = @(
        (ConvertTo-List $fx.manifest.excluded_non_task_members) |
            ForEach-Object { "$($_.id)=$($_.artifact_type)" } |
            Sort-Object
    )
    $actualExcluded = @(
        $live.excluded_non_task_members |
            ForEach-Object { "$($_.id)=$($_.artifact_type)" } |
            Sort-Object
    )
    Test-Equal -Scenario $sc -Name 'shipment member count' -Expected $fx.manifest.shipment_member_count -Actual $live.shipment_items.Count
    Test-Equal -Scenario $sc -Name 'M task count' -Expected $fx.manifest.member_count -Actual $live.members.Count
    Test-Equal -Scenario $sc -Name 'M task IDs' -Expected $expectedMIDs -Actual $actualMIDs
    Test-Equal -Scenario $sc -Name 'manifest M equals explicit non-shipment fallback' -Expected $expectedMIDs -Actual $fallbackMIDs
    Test-Equal -Scenario $sc -Name 'excluded non-task members' -Expected $expectedExcluded -Actual $actualExcluded
    Test-Equal -Scenario $sc -Name 'workspace status catalog' `
        -Expected @((ConvertTo-List $fx.status_model.catalog) | Sort-Object) `
        -Actual @((ConvertTo-List $live.status_catalog) | Sort-Object)
    Test-Equal -Scenario $sc -Name 'registry status mapping' `
        -Expected (Format-KeyValueProjection $fx.status_model.registry_status_mapping) `
        -Actual (Format-KeyValueProjection $live.registry_status_mapping)
    Test-Equal -Scenario $sc -Name 'registry snapshot features' `
        -Expected (Format-KeyValueProjection $fx.status_model.registry_features) `
        -Actual (Format-KeyValueProjection $live.registry_features)
    $drift = Compare-LiveShipmentProjection -Fx $fx -Live $live
    $driftText = @($drift | ForEach-Object { "$($_.Code): $($_.Detail)" }) -join '; '
    Test-Equal -Scenario $sc -Name 'no shipment or contract drift' -Expected '' -Actual $driftText

    if (-not $Quiet) {
        Write-Host "manifest : $($fx.source.shipment) $($live.shipment_items.Count) total member(s)"
        Write-Host "wave M   : $($live.members.Count) task member(s)"
        Write-Host "fallback : $($fallbackMIDs.Count) explicit task ID(s), exact M match"
        Write-Host "statuses : $($live.status_catalog.Count) catalog token(s); registry mapping/features parsed live"
        Write-Host "excluded : $(if ($actualExcluded.Count -eq 0) { '<none>' } else { $actualExcluded -join ', ' })"
        Write-Host "mutations: $(@($fx.verification_mutations).Count) in-memory drift check(s)"
        Write-Host ""
    }

    foreach ($mutation in (ConvertTo-List $fx.verification_mutations)) {
        $mutated = Copy-Projection -Projection $live
        $mutated = Apply-VerificationMutation -Projection $mutated -Mutation $mutation
        $mutationDrift = Compare-LiveShipmentProjection -Fx $fx -Live $mutated
        $codes = @($mutationDrift | ForEach-Object { $_.Code } | Sort-Object -Unique)
        Test-Equal -Scenario "queue_mutation/$($mutation.id)" -Name "detect $($mutation.expect_code)" `
            -Expected $true -Actual ([bool]($codes -contains $mutation.expect_code))
    }
}

function Invoke-BaselineConvergenceQueueProjection {
    $projection = $fx.baseline_convergence_projection
    $sc = 'queue_projection/156-S'
    $shipmentPath = Find-ArtifactPath -Dir $QueueDir -Id "$($projection.shipment_id)"
    if ($null -eq $shipmentPath) {
        Test-Equal -Scenario $sc -Name 'shipment present' -Expected $true -Actual $false
        return
    }
    $itemIDs = Get-ShipmentItemIDs -Path $shipmentPath
    $members = @()
    $excluded = @()
    $resolutionErrors = @()
    foreach ($id in $itemIDs) {
        $path = Find-ArtifactPath -Dir $QueueDir -Id $id
        if ($null -eq $path) { $resolutionErrors += "unresolved:$id"; continue }
        $artifact = Get-ArtifactProjection -Path $path
        if ("$($artifact.id)" -cne "$id") { $resolutionErrors += "id-mismatch:$id"; continue }
        if ("$($artifact.artifact_type)" -ceq 'task') { $members += $artifact }
        else { $excluded += [pscustomobject]@{ id = "$id"; artifact_type = "$($artifact.artifact_type)" } }
    }
    $taskIDs = @($members | ForEach-Object { $_.id } | Sort-Object)
    $expectedTaskIDs = @($projection.task_ids | ForEach-Object { "$_" } | Sort-Object)
    $actualExcluded = @($excluded | ForEach-Object { "$($_.id)=$($_.artifact_type)" } | Sort-Object)
    $expectedExcluded = @(
        $projection.excluded_members |
            ForEach-Object { "$($_.id)=$($_.artifact_type)" } |
            Sort-Object
    )
    $lintValid = 0
    foreach ($artifact in $members) {
        $path = Find-ArtifactPath -Dir $QueueDir -Id "$($artifact.id)"
        $lint = Read-TaskLintContract -Raw (Get-Content $path -Raw)
        if ($lint.Errors.Count -eq 0) { $lintValid++ }
    }
    $edgeCount = @($members | ForEach-Object { @($_.deps).Count } | Measure-Object -Sum).Sum
    $externalEdges = @(
        $members |
            ForEach-Object { $_.deps } |
            Where-Object { $taskIDs -notcontains "$_" }
    )
    $actualDependencyProjection = @(
        $members |
            Sort-Object id |
            ForEach-Object { "$($_.id)=>$(@($_.deps | Sort-Object) -join ',')" }
    )
    $expectedDependencyProjection = @(
        (Get-PropertyNames $projection.dependencies_by_task) |
            Sort-Object |
            ForEach-Object {
                "$_=>$(@($projection.dependencies_by_task.$_ | Sort-Object) -join ',')"
            }
    )
    $firstWave = @($members | Where-Object { @($_.deps).Count -eq 0 } | ForEach-Object { $_.id } | Sort-Object)
    $u1Dependents = @(
        $members | Where-Object { @($_.deps) -contains '175.001-T' }
    ).Count
    $terminal = @($members | Where-Object { $_.id -ceq "$($projection.terminal_task_id)" })
    $terminalDeps = if ($terminal.Count -eq 1) { @($terminal[0].deps).Count } else { -1 }
    $dependedOn = @($members | ForEach-Object { $_.deps } | Sort-Object -Unique)
    $sinks = @($taskIDs | Where-Object { $dependedOn -notcontains $_ } | Sort-Object)
    $releasePath = Find-ArtifactPath -Dir $QueueDir -Id "$($projection.release_unit_id)"
    $baselinePresent = $false
    $baselineValidWhenPresent = $true
    if ($null -ne $releasePath) {
        $baselineBlock = Get-DelimitedContractBlock -Raw (Get-Content $releasePath -Raw) `
            -Name 'baseline-lint-convergence-contract' -Fence 'json'
        $baselinePresent = [bool]$baselineBlock.Present
        if ($baselinePresent) {
            $rawContract = $null
            try { $rawContract = $baselineBlock.Content | ConvertFrom-Json -DateKind String -ErrorAction Stop }
            catch { $rawContract = $null }
            $inventoryObservation = Get-BaselineInventoryObservation -Contract $rawContract
            $authorizationObservation = [pscustomobject]@{
                operator_supplied = -not [string]::IsNullOrWhiteSpace($OperatorAuthorizationRecord)
                record = $OperatorAuthorizationRecord
                shipment_id = "$($projection.shipment_id)"
                scope = 'retain-only-unfinished-owned-baseline-findings-until-terminal'
            }
            $parsedBaseline = Read-BaselineLintContract -Raw (Get-Content $releasePath -Raw) `
                -Members $taskIDs -InventoryObservation $inventoryObservation `
                -ExpectedShipmentID "$($projection.shipment_id)" `
                -ExpectedReleaseUnitID "$($projection.release_unit_id)" `
                -UniqueTerminalSink "$($projection.terminal_task_id)" `
                -AuthorizationObservation $authorizationObservation
            $baselineValidWhenPresent = [bool]($parsedBaseline.Errors.Count -eq 0)
        }
    }

    Test-Equal -Scenario $sc -Name 'shipment member count' `
        -Expected $projection.shipment_member_count -Actual $itemIDs.Count
    Test-Equal -Scenario $sc -Name 'all member IDs/types resolve' -Expected @() -Actual $resolutionErrors
    Test-Equal -Scenario $sc -Name 'task count' -Expected $projection.task_count -Actual $taskIDs.Count
    Test-Equal -Scenario $sc -Name 'exact frozen task IDs' -Expected $expectedTaskIDs -Actual $taskIDs
    Test-Equal -Scenario $sc -Name 'excluded non-task members' `
        -Expected $expectedExcluded -Actual $actualExcluded
    Test-Equal -Scenario $sc -Name 'edge count' -Expected $projection.edge_count -Actual $edgeCount
    Test-Equal -Scenario $sc -Name 'all dependency edges stay inside M' -Expected @() -Actual $externalEdges
    Test-Equal -Scenario $sc -Name 'exact dependency edge identities' `
        -Expected $expectedDependencyProjection -Actual $actualDependencyProjection
    Test-Equal -Scenario $sc -Name 'U1-only first wave' `
        -Expected (ConvertTo-List $projection.first_wave) -Actual $firstWave
    Test-Equal -Scenario $sc -Name 'U1 direct dependents' `
        -Expected $projection.u1_direct_dependents -Actual $u1Dependents
    Test-Equal -Scenario $sc -Name 'terminal dependency count' `
        -Expected $projection.terminal_dependency_count -Actual $terminalDeps
    Test-Equal -Scenario $sc -Name 'unique terminal sink' `
        -Expected @("$($projection.terminal_task_id)") -Actual $sinks
    Test-Equal -Scenario $sc -Name 'valid task-lint contract count' `
        -Expected $projection.task_lint_contract_count -Actual $lintValid
    Test-Equal -Scenario $sc -Name 'Stage baseline contract presence' `
        -Expected ([bool]$projection.stage_contract_present) -Actual $baselinePresent
    Test-Equal -Scenario $sc -Name 'Stage baseline contract valid when present' `
        -Expected $true -Actual $baselineValidWhenPresent

    if (-not $Quiet) {
        Write-Host "156-S    : $($taskIDs.Count) tasks, $edgeCount edges, first wave [$($firstWave -join ', ')]"
        Write-Host "lint      : $lintValid/$($taskIDs.Count) task-lint contracts structurally valid"
        Write-Host "Stage mode: baseline-lint contract present=$baselinePresent (expected $($projection.stage_contract_present))"
        Write-Host ""
    }
}

# --- scheduler ------------------------------------------------------------------
function Invoke-WaveScheduler {
    param($Fx, $Sc)

    $mut = $Sc.mutations
    $result = [ordered]@{
        outcome                  = $null
        halt_wave                = $null
        halt_detail              = ''
        waves                    = 0
        wave_sizes               = @()
        wave_members             = @()
        scheduled                = 0
        stalls                   = 0
        compile_order_violations = 0
        snapshot_calls           = 0
        unsupported              = @()
        blocked_ids              = @()
        active_ids               = @()
        dependency_impact        = @{}
        cycle_path               = @()
        members_dropped          = @()
        unresolved               = @()
        unclosed                 = @()
        open_red_after_wave      = @{}
        open_red_closed_at_wave  = @{}
        open_red_reconfirmed_at_wave = @{}
        open_red_newly_closed_at_wave = @{}
        early_green              = @()
        green_maker_unverified   = @()
        full_suite_waves         = @()
        deferred_waves           = @()
        compile_gate_waves       = @()
        final_open_red           = @()
        completion_claimed       = $false
        member_retained_in_m     = $true
        full_suite_at_final_closure = $false
        hidden_unexpected_failures = 0
        gate_compare             = $null
    }

    # ---- status model (configured, never hard-coded) ----
    $catalog = ConvertTo-List $Fx.status_model.catalog
    if (Test-HasProperty $mut 'clear_catalog') { if ($mut.clear_catalog) { $catalog = @() } }
    $executable = ConvertTo-List $Fx.status_model.executable
    $terminalSuccess = ConvertTo-List $Fx.status_model.terminal_success
    $greenClosing = ConvertTo-List $Fx.status_model.green_maker_closing
    $registryMapping = $Fx.status_model.registry_status_mapping
    $registryValues = @(
        Get-PropertyNames $registryMapping | ForEach-Object { "$($registryMapping.$_)" }
    )
    if (Test-HasProperty $mut 'add_executable_status') { $executable = @($executable + $mut.add_executable_status) }

    if ($catalog.Count -eq 0 -or $registryValues.Count -eq 0 -or
        $executable.Count -eq 0 -or $terminalSuccess.Count -eq 0) {
        $result.outcome = 'WAVE_STATUS_CATALOG_UNAVAILABLE'
        $result.halt_wave = 0
        $result.halt_detail = 'configured status catalog unavailable or empty'
        return [pscustomobject]$result
    }
    $disagree = @()
    foreach ($s in ($executable + $terminalSuccess)) { if ($catalog -cnotcontains $s) { $disagree += $s } }
    foreach ($s in ($executable + $greenClosing)) {
        if ($registryValues -cnotcontains $s) { $disagree += "registry:$s" }
    }
    foreach ($s in $executable) { if ($terminalSuccess -ccontains $s) { $disagree += "overlap:$s" } }
    if ($disagree.Count -gt 0) {
        $result.outcome = 'WAVE_STATUS_CATALOG_UNAVAILABLE'
        $result.halt_wave = 0
        $result.halt_detail = "configured status disagrees with catalog: $($disagree -join ',')"
        return [pscustomobject]$result
    }

    # ---- frozen manifest M (Ship Step 3) ----
    $M = [ordered]@{}
    $deps = @{}
    foreach ($mem in $Fx.manifest.members) {
        $M[$mem.id] = [ordered]@{
            id             = $mem.id
            unit           = $mem.unit
            status         = $mem.status
            harness_exempt = [bool]$mem.harness_exempt
            red_deliverable = [bool]$mem.red_deliverable
        }
        $deps[$mem.id] = ConvertTo-List $mem.deps
    }
    if (Test-HasProperty $mut 'set_status') {
        foreach ($p in (Get-PropertyNames $mut.set_status)) { $M[$p].status = $mut.set_status.$p }
    }
    if (Test-HasProperty $mut 'add_edge') {
        foreach ($e in (ConvertTo-List $mut.add_edge)) { $deps[$e.from] = @($deps[$e.from] + $e.to) }
    }
    $liveItems = @{}
    foreach ($k in $M.Keys) { $liveItems[$k] = $true }
    $frozen = $true
    if (Test-HasProperty $mut 'freeze_m') { $frozen = ($mut.freeze_m -ne 'per-wave') }

    # ---- red-deliverable mapping (mechanical, fail closed) ----
    $redMap = @{}
    $dropped = @()
    if (Test-HasProperty $mut 'drop_red_contract') { $dropped = ConvertTo-List $mut.drop_red_contract }
    foreach ($rd in $Fx.red_deliverables) {
        if ($dropped -contains $rd.task) { continue }
        $gm = ConvertTo-List $rd.green_maker_tasks
        if ((Test-HasProperty $mut 'override_green_makers') -and ((Get-PropertyNames $mut.override_green_makers) -contains $rd.task)) {
            $gm = ConvertTo-List $mut.override_green_makers.($rd.task)
        }
        $redFlag = [bool]$rd.red_deliverable
        if ((Test-HasProperty $mut 'override_red_deliverable') -and ((Get-PropertyNames $mut.override_red_deliverable) -contains $rd.task)) {
            $redFlag = [bool]$mut.override_red_deliverable.($rd.task)
        }
        $reason = "$($rd.red_deliverable_reason)"
        if ((Test-HasProperty $mut 'override_red_reason') -and ((Get-PropertyNames $mut.override_red_reason) -contains $rd.task)) {
            $reason = "$($mut.override_red_reason.($rd.task))"
        }
        $selector = "$($rd.red_selector_command)"
        if ((Test-HasProperty $mut 'override_red_selector') -and ((Get-PropertyNames $mut.override_red_selector) -contains $rd.task)) {
            $selector = "$($mut.override_red_selector.($rd.task))"
        }
        $closesWave = [int]$rd.green_maker_closes_wave
        if ((Test-HasProperty $mut 'override_closes_wave') -and ((Get-PropertyNames $mut.override_closes_wave) -contains $rd.task)) {
            $closesWave = [int]$mut.override_closes_wave.($rd.task)
        }
        $redMap[$rd.task] = [ordered]@{
            task           = $rd.task
            redDeliverable = $redFlag
            reason         = $reason
            selector       = $selector
            greenMakers    = $gm
            closesWave     = $closesWave
            open           = $false
            closedWave     = $null
        }
    }
    $unresolved = @()
    foreach ($redTask in $redMap.Keys) {
        if (-not $M.Contains($redTask) -or -not $M[$redTask].red_deliverable) { $unresolved += $redTask }
    }
    foreach ($k in $M.Keys) {
        if (-not $M[$k].red_deliverable) { continue }
        if (-not $redMap.ContainsKey($k)) { $unresolved += $k; continue }
        $entry = $redMap[$k]
        if (-not $entry.redDeliverable) { $unresolved += $k; continue }
        if ([string]::IsNullOrWhiteSpace($entry.reason)) { $unresolved += $k; continue }
        if ([string]::IsNullOrWhiteSpace($entry.selector)) { $unresolved += $k; continue }
        if ($entry.closesWave -le 0) { $unresolved += $k; continue }
        if ($entry.greenMakers.Count -eq 0) { $unresolved += $k; continue }
        foreach ($g in $entry.greenMakers) {
            if (-not $M.Contains($g)) { $unresolved += $k; break }
            if ($g -eq $k) { $unresolved += $k; break }
        }
    }
    $unresolved = @($unresolved | Sort-Object -Unique)
    if ($unresolved.Count -gt 0) {
        $result.outcome = 'WAVE_RED_MAPPING_UNRESOLVED'
        $result.halt_wave = 0
        $result.unresolved = $unresolved
        $result.halt_detail = "red-deliverable green-maker mapping missing or ambiguous: $($unresolved -join ',')"
        return [pscustomobject]$result
    }

    # ---- Step 3 acyclicity (Kahn) ----
    $indeg = @{}
    foreach ($k in $M.Keys) { $indeg[$k] = 0 }
    foreach ($k in $M.Keys) { foreach ($d in $deps[$k]) { if ($M.Contains($d)) { $indeg[$k] = $indeg[$k] + 1 } } }
    $queue = [System.Collections.Generic.Queue[string]]::new()
    foreach ($k in ($M.Keys | Sort-Object)) { if ($indeg[$k] -eq 0) { $queue.Enqueue($k) } }
    $sorted = 0
    $indegWork = @{}
    foreach ($k in $indeg.Keys) { $indegWork[$k] = $indeg[$k] }
    while ($queue.Count -gt 0) {
        $n = $queue.Dequeue(); $sorted++
        foreach ($k in ($M.Keys | Sort-Object)) {
            if ($deps[$k] -contains $n) {
                $indegWork[$k] = $indegWork[$k] - 1
                if ($indegWork[$k] -eq 0) { $queue.Enqueue($k) }
            }
        }
    }
    if ($sorted -lt $M.Count) {
        $result.outcome = 'WAVE_CYCLE_DETECTED'
        $result.halt_wave = 0
        $result.cycle_path = @($M.Keys | Where-Object { $indegWork[$_] -gt 0 } | Sort-Object)
        $result.halt_detail = "cycle over $($result.cycle_path.Count) member(s)"
        return [pscustomobject]$result
    }

    # Static wave map validates strict-later red mappings and their declared close wave.
    $staticWaveOf = @{}
    $remaining = @($M.Keys)
    $staticWave = 0
    while ($remaining.Count -gt 0) {
        $staticWave++
        $frontier = @(
            $remaining |
                Where-Object {
                    $taskID = $_
                    @($deps[$taskID] | Where-Object { $M.Contains($_) -and -not $staticWaveOf.ContainsKey($_) }).Count -eq 0
                } |
                Sort-Object
        )
        if ($frontier.Count -eq 0) { break }
        foreach ($taskID in $frontier) { $staticWaveOf[$taskID] = $staticWave }
        $remaining = @($remaining | Where-Object { $frontier -notcontains $_ })
    }
    $unresolved = @()
    foreach ($taskID in $redMap.Keys) {
        $entry = $redMap[$taskID]
        $latestGreenWave = 0
        foreach ($greenMaker in $entry.greenMakers) {
            if (-not $staticWaveOf.ContainsKey($greenMaker) -or
                $staticWaveOf[$greenMaker] -le $staticWaveOf[$taskID]) {
                $unresolved += $taskID
                break
            }
            if ($staticWaveOf[$greenMaker] -gt $latestGreenWave) {
                $latestGreenWave = $staticWaveOf[$greenMaker]
            }
        }
        if ($entry.closesWave -ne $latestGreenWave) { $unresolved += $taskID }
    }
    $unresolved = @($unresolved | Sort-Object -Unique)
    if ($unresolved.Count -gt 0) {
        $result.outcome = 'WAVE_RED_MAPPING_UNRESOLVED'
        $result.halt_wave = 0
        $result.unresolved = $unresolved
        $result.halt_detail = "red-deliverable green-maker mapping missing or ambiguous: $($unresolved -join ',')"
        return [pscustomobject]$result
    }

    # ---- wave loop ----
    $waveIndex = 0
    $budget = $M.Count
    $waveOf = @{}
    $gateCompareWave = $null
    if (Test-HasProperty $mut 'gate_compare_wave') { $gateCompareWave = [int]$mut.gate_compare_wave }

    # early-green injection: an open red deliverable whose selector starts passing at a stated
    # wave while its declared green-makers are still open. Models the P-002.6 contract violation
    # the convergence gate's open-red re-confirmation exists to catch.
    $earlyGreenAtWave = @{}
    if (Test-HasProperty $mut 'open_red_early_green') {
        foreach ($eg in (ConvertTo-List $mut.open_red_early_green)) {
            $earlyGreenAtWave[$eg.id] = [int]$eg.wave
        }
    }

    # still-red-after-close injection: an entry whose declared green-makers all reach done but
    # whose selector keeps failing. Models the green-maker contract violation the convergence
    # gate's newly-closed verification exists to catch.
    $stayRedAfterClose = @()
    if (Test-HasProperty $mut 'green_maker_leaves_red') {
        $stayRedAfterClose = ConvertTo-List $mut.green_maker_leaves_red
    }

    while ($true) {
        $waveIndex++
        if ($waveIndex -gt $budget) {
            $result.outcome = 'WAVE_BUDGET_EXCEEDED'; $result.halt_wave = $waveIndex; break
        }

        # mid-run status injections that apply at THIS admission
        if (Test-HasProperty $mut 'set_status_at_wave') {
            foreach ($inj in (ConvertTo-List $mut.set_status_at_wave)) {
                $onCompletion = $false
                if (Test-HasProperty $inj 'on_completion') { $onCompletion = [bool]$inj.on_completion }
                if (-not $onCompletion -and [int]$inj.wave -eq $waveIndex) { $M[$inj.id].status = $inj.status }
            }
        }

        # one snapshot call per wave, over all of M, unfiltered
        $result.snapshot_calls = $result.snapshot_calls + 1
        $scope = @($M.Keys | Where-Object { $frozen -or $liveItems.ContainsKey($_) })

        $terminal = @(); $queued = @(); $active = @(); $blocked = @(); $unsupported = @()
        foreach ($k in ($scope | Sort-Object)) {
            $s = $M[$k].status
            if ($terminalSuccess -ccontains $s) { $terminal += $k }
            elseif ($s -ceq 'queued') { $queued += $k }
            elseif ($s -ceq 'active') { $active += $k }
            elseif ($s -ceq 'blocked') { $blocked += $k }
            else { $unsupported += "$k=$s" }
        }

        if ($unsupported.Count -gt 0) {
            $result.outcome = 'WAVE_STATUS_UNSUPPORTED'; $result.halt_wave = $waveIndex
            $result.unsupported = @($unsupported); $result.halt_detail = 'status outside configured executable and terminal-success sets'
            break
        }
        if ($blocked.Count -gt 0) {
            $result.outcome = 'WAVE_MEMBER_BLOCKED'; $result.halt_wave = $waveIndex
            $result.blocked_ids = @($blocked)
            foreach ($b in $blocked) {
                $impact = @{}
                $frontier = @($b)
                while ($frontier.Count -gt 0) {
                    $next = @()
                    foreach ($k in $M.Keys) {
                        foreach ($f in $frontier) {
                            if (($deps[$k] -contains $f) -and -not $impact.ContainsKey($k) -and $k -ne $b) {
                                $impact[$k] = $true; $next += $k
                            }
                        }
                    }
                    $frontier = @($next | Sort-Object -Unique)
                }
                $result.dependency_impact[$b] = $impact.Count
            }
            $result.member_retained_in_m = ($M.Contains($blocked[0]))
            break
        }        if ($active.Count -gt 0) {
            $result.outcome = 'WAVE_NO_PROGRESS'; $result.halt_wave = $waveIndex
            $result.halt_detail = 'active residual'; $result.active_ids = @($active)
            break
        }
        if ($terminal.Count -eq $scope.Count) {
            $result.completion_claimed = $true
            $result.outcome = 'COMPLETE'
            $result.waves = $waveIndex - 1
            break
        }

        $ready = @()
        foreach ($k in ($queued | Sort-Object)) {
            $ok = $true
            foreach ($d in $deps[$k]) {
                if (-not $M.Contains($d)) { continue }
                if (-not ($terminalSuccess -ccontains $M[$d].status)) { $ok = $false; break }
            }
            if ($ok) { $ready += $k }
        }
        if ($ready.Count -eq 0) {
            $result.outcome = 'WAVE_NO_PROGRESS'; $result.halt_wave = $waveIndex
            $result.stalls = $result.stalls + 1
            $result.halt_detail = "empty frontier with $($scope.Count - $terminal.Count) non-terminal member(s)"
            break
        }

        # gate-model comparison for the requested wave (read-only bookkeeping)
        if ($null -ne $gateCompareWave -and $waveIndex -eq $gateCompareWave) {
            $nonExempt = @($ready | Where-Object { -not $M[$_].harness_exempt })
            $openBefore = @($redMap.Keys | Where-Object { $redMap[$_].open } | Sort-Object)
            $oldProgressed = 0
            foreach ($t in $ready) {
                $siblingsRed = @($nonExempt | Where-Object { $_ -ne $t }).Count
                if ($siblingsRed -eq 0 -and $openBefore.Count -eq 0) { $oldProgressed++ }
            }
            $result.gate_compare = [ordered]@{
                wave                                  = $waveIndex
                wave_non_exempt_members               = $nonExempt.Count
                old_repo_wide_progressed              = $oldProgressed
                new_scoped_progressed                 = $ready.Count
                full_suite_runs_inside_per_task_loops = 0
                scoped_commands_run_in_wave           = $ready.Count
            }
        }

        # execute the wave, task-scoped: each member reaches done
        foreach ($t in $ready) { $M[$t].status = 'done'; $waveOf[$t] = $waveIndex }

        # completion-time status injections (e.g. descope of a green-maker)
        if (Test-HasProperty $mut 'set_status_at_wave') {
            foreach ($inj in (ConvertTo-List $mut.set_status_at_wave)) {
                $onCompletion = $false
                if (Test-HasProperty $inj 'on_completion') { $onCompletion = [bool]$inj.on_completion }
                if ($onCompletion -and [int]$inj.wave -eq $waveIndex) { $M[$inj.id].status = $inj.status }
            }
        }
        if (Test-HasProperty $mut 'return_blocked_at_wave') {
            foreach ($rb in (ConvertTo-List $mut.return_blocked_at_wave)) {
                if ([int]$rb.wave -eq $waveIndex) {
                    $M[$rb.id].status = 'blocked'
                    $liveItems.Remove($rb.id) | Out-Null
                    if (-not $frozen) { $result.members_dropped = @($result.members_dropped + $rb.id) }
                }
            }
        }

        $result.waves = $waveIndex
        $result.wave_sizes = @($result.wave_sizes + $ready.Count)
        $result.wave_members = @($result.wave_members + (, @($ready)))
        $result.scheduled = $result.scheduled + $ready.Count

        # ---- Step 4.6 convergence gate ----
        # open-red set: completed red deliverables whose green-makers are not all closed
        foreach ($t in $ready) { if ($redMap.ContainsKey($t)) { $redMap[$t].open = $true } }
        $newlyClosed = @()
        foreach ($k in $redMap.Keys) {
            if (-not $redMap[$k].open) { continue }
            $allClosed = $true
            foreach ($g in $redMap[$k].greenMakers) {
                if (-not ($greenClosing -ccontains $M[$g].status)) { $allClosed = $false; break }
            }
            if ($allClosed) {
                $redMap[$k].open = $false
                $redMap[$k].closedWave = $waveIndex
                $result.open_red_closed_at_wave[$k] = $waveIndex
                $newlyClosed = @($newlyClosed + $k)
            }
        }
        $newlyClosed = @($newlyClosed | Sort-Object)
        $openNow = @($redMap.Keys | Where-Object { $redMap[$_].open } | Sort-Object)
        $result.open_red_after_wave["$waveIndex"] = $openNow

        # the compile / vet / scoped-green part of the gate always runs
        $result.compile_gate_waves = @($result.compile_gate_waves + $waveIndex)

        # P-002.6 always-run items 4 and 5. The pre-recomputation open set partitions exactly into
        # entries still open (selector re-run, must stay RED) and entries newly closed at this
        # recomputation (selector re-run, must now be GREEN). No entry is skipped: while another
        # entry keeps the set non-empty the unfiltered suite stays deferred, so neither an
        # early-green open entry nor a still-red newly closed entry would be caught anywhere else.
        $result.open_red_reconfirmed_at_wave["$waveIndex"] = $openNow
        $result.open_red_newly_closed_at_wave["$waveIndex"] = $newlyClosed
        $earlyGreen = @()
        foreach ($k in $openNow) {
            if ($earlyGreenAtWave.ContainsKey($k) -and [int]$earlyGreenAtWave[$k] -le $waveIndex) {
                $earlyGreen = @($earlyGreen + $k)
            }
        }
        if ($earlyGreen.Count -gt 0) {
            $result.outcome = 'WAVE_RED_DELIVERABLE_EARLY_GREEN'; $result.halt_wave = $waveIndex
            $result.early_green = @($earlyGreen | Sort-Object)
            $result.halt_detail = "open red observed green before its declared green-maker: $($result.early_green -join ',')"
            break
        }
        $unverified = @()
        foreach ($k in $newlyClosed) {
            $goesGreen = $earlyGreenAtWave.ContainsKey($k) -and [int]$earlyGreenAtWave[$k] -le $waveIndex
            if ($stayRedAfterClose -contains $k) { $goesGreen = $false }
            elseif (-not $earlyGreenAtWave.ContainsKey($k)) { $goesGreen = $true }
            if (-not $goesGreen) { $unverified = @($unverified + $k) }
        }
        if ($unverified.Count -gt 0) {
            $result.outcome = 'WAVE_GREEN_MAKER_UNVERIFIED'; $result.halt_wave = $waveIndex
            $result.green_maker_unverified = @($unverified | Sort-Object)
            $result.halt_detail = "green-maker landed but its open red is still failing: $($result.green_maker_unverified -join ',')"
            break
        }

        # deferral budget: an entry still open past its declared closing wave fails closed
        $overdue = @($openNow | Where-Object { $redMap[$_].closesWave -lt $waveIndex })
        if ($overdue.Count -gt 0) {
            $result.outcome = 'WAVE_OPEN_RED_UNCLOSED'; $result.halt_wave = $waveIndex
            $result.unclosed = @($overdue)
            $result.halt_detail = "open red past its declared closing wave: $($overdue -join ',')"
            break
        }

        if ($openNow.Count -eq 0) { $result.full_suite_waves = @($result.full_suite_waves + $waveIndex) }
        else { $result.deferred_waves = @($result.deferred_waves + $waveIndex) }
    }

    # every frozen member must still be visible to the scheduler at halt or exit
    $visible = @($M.Keys | Where-Object { $frozen -or $liveItems.ContainsKey($_) }).Count
    $result.member_retained_in_m = ($result.member_retained_in_m -and ($visible -eq $M.Count))

    if ($result.outcome -eq 'COMPLETE') {
        $result.final_open_red = @($redMap.Keys | Where-Object { $redMap[$_].open } | Sort-Object)
        if ($result.final_open_red.Count -gt 0) {
            $result.outcome = 'WAVE_OPEN_RED_UNCLOSED'
            $result.completion_claimed = $false
            $result.unclosed = $result.final_open_red
        }
        else {
            $result.full_suite_at_final_closure = $true
        }
        $viol = 0
        foreach ($k in $M.Keys) {
            if (-not $waveOf.ContainsKey($k)) { continue }
            foreach ($d in $deps[$k]) {
                if (-not $M.Contains($d)) { continue }
                if (-not $waveOf.ContainsKey($d)) { $viol++; continue }
                if ($waveOf[$d] -ge $waveOf[$k]) { $viol++ }
            }
        }
        $result.compile_order_violations = $viol
    }

    return [pscustomobject]$result
}

# --- expectation checking -------------------------------------------------------
function Test-Scenario {
    param($Fx, $Sc)
    $r = Invoke-WaveScheduler -Fx $Fx -Sc $Sc
    $id = $Sc.id
    foreach ($key in (Get-PropertyNames $Sc.expect)) {
        $want = $Sc.expect.$key
        switch ($key) {
            'outcome' { Test-Equal $id $key $want $r.outcome }
            'halt_wave' { Test-Equal $id $key $want $r.halt_wave }
            'halt_detail' { Test-Equal $id $key $want $r.halt_detail }
            'waves' { Test-Equal $id $key $want $r.waves }
            'wave_sizes' { Test-Equal $id $key (ConvertTo-List $want) $r.wave_sizes }
            'scheduled' { Test-Equal $id $key $want $r.scheduled }
            'stalls' { Test-Equal $id $key $want $r.stalls }
            'compile_order_violations' { Test-Equal $id $key $want $r.compile_order_violations }
            'snapshot_calls' { Test-Equal $id $key $want $r.snapshot_calls }
            'unsupported_count' { Test-Equal $id $key $want $r.unsupported.Count }
            'unsupported' { Test-Equal $id $key (ConvertTo-List $want) $r.unsupported }
            'blocked_ids' { Test-Equal $id $key (ConvertTo-List $want) $r.blocked_ids }
            'active_ids' { Test-Equal $id $key (ConvertTo-List $want) $r.active_ids }
            'unresolved' { Test-Equal $id $key (ConvertTo-List $want) $r.unresolved }
            'unclosed' { Test-Equal $id $key (ConvertTo-List $want) $r.unclosed }
            'members_dropped' { Test-Equal $id $key (ConvertTo-List $want) $r.members_dropped }
            'completion_claimed' { Test-Equal $id $key $want $r.completion_claimed }
            'member_retained_in_m' { Test-Equal $id $key $want $r.member_retained_in_m }
            'full_suite_at_final_closure' { Test-Equal $id $key $want $r.full_suite_at_final_closure }
            'hidden_unexpected_failures' { Test-Equal $id $key $want $r.hidden_unexpected_failures }
            'full_suite_waves' { Test-Equal $id $key (ConvertTo-List $want) $r.full_suite_waves }
            'deferred_waves' { Test-Equal $id $key (ConvertTo-List $want) $r.deferred_waves }
            'compile_gate_waves' { Test-Equal $id $key (ConvertTo-List $want) $r.compile_gate_waves }
            'final_open_red' { Test-Equal $id $key (ConvertTo-List $want) $r.final_open_red }
            'wave_4_advanced' { Test-Equal $id $key $want ([bool]($r.waves -ge 5)) }
            'cycle_path_non_empty' { Test-Equal $id $key $want ([bool]($r.cycle_path.Count -gt 0)) }
            'manifest_matches_explicit_fallback' {
                $manifestIDs = @($Fx.manifest.members | ForEach-Object { $_.id } | Sort-Object)
                $fallbackIDs = @(
                    (ConvertTo-List $Fx.manifest.explicit_non_shipment_task_ids) | Sort-Object
                )
                $fallbackUnique = @($fallbackIDs | Sort-Object -Unique)
                $actual = [bool](
                    $fallbackUnique.Count -eq $fallbackIDs.Count -and
                    ($manifestIDs -join ',') -ceq ($fallbackIDs -join ',')
                )
                Test-Equal $id $key $want $actual
            }
            'open_red_after_wave_4' { Test-Equal $id $key (ConvertTo-List $want) (ConvertTo-List $r.open_red_after_wave['4']) }
            'open_red_after_wave_6' { Test-Equal $id $key (ConvertTo-List $want) (ConvertTo-List $r.open_red_after_wave['6']) }
            'dependency_impact' {
                foreach ($p in (Get-PropertyNames $want)) {
                    $actual = if ($r.dependency_impact.ContainsKey($p)) { $r.dependency_impact[$p] } else { -1 }
                    Test-Equal $id "dependency_impact[$p]" $want.$p $actual
                }
            }
            'open_red_closed_at_wave' {
                foreach ($p in (Get-PropertyNames $want)) {
                    $actual = if ($r.open_red_closed_at_wave.ContainsKey($p)) { $r.open_red_closed_at_wave[$p] } else { -1 }
                    Test-Equal $id "open_red_closed_at_wave[$p]" $want.$p $actual
                }
            }
            'early_green' { Test-Equal $id $key (ConvertTo-List $want) $r.early_green }
            'green_maker_unverified' { Test-Equal $id $key (ConvertTo-List $want) $r.green_maker_unverified }
            'open_red_newly_closed_at_wave' {
                foreach ($p in (Get-PropertyNames $want)) {
                    $actual = if ($r.open_red_newly_closed_at_wave.ContainsKey($p)) { $r.open_red_newly_closed_at_wave[$p] } else { @() }
                    Test-Equal $id "open_red_newly_closed_at_wave[$p]" (ConvertTo-List $want.$p) (ConvertTo-List $actual)
                }
            }
            'open_red_reconfirmed_at_wave' {
                foreach ($p in (Get-PropertyNames $want)) {
                    $actual = if ($r.open_red_reconfirmed_at_wave.ContainsKey($p)) { $r.open_red_reconfirmed_at_wave[$p] } else { @() }
                    Test-Equal $id "open_red_reconfirmed_at_wave[$p]" (ConvertTo-List $want.$p) (ConvertTo-List $actual)
                }
            }
            'open_red_reconfirmed_waves' {
                $actual = @($r.open_red_reconfirmed_at_wave.Keys |
                    Where-Object { @($r.open_red_reconfirmed_at_wave[$_]).Count -gt 0 } |
                    ForEach-Object { [int]$_ } | Sort-Object)
                Test-Equal $id $key (ConvertTo-List $want) $actual
            }
            'gate_compare_wave' {
                $actualWave = -1
                if ($null -ne $r.gate_compare) { $actualWave = $r.gate_compare['wave'] }
                Test-Equal $id $key $want $actualWave
            }
            default {
                if ($null -ne $r.gate_compare -and $r.gate_compare.Contains($key)) {
                    Test-Equal $id $key $want $r.gate_compare[$key]
                }
                else {
                    Add-Assertion -Scenario $id -Name $key -Ok $false -Expected (Format-Value $want) -Actual '<no such observable>'
                }
            }
        }
    }
    return $r
}

# --- run ------------------------------------------------------------------------
if (-not $Quiet) {
    Write-Host "P-002.6 wave-scheduler contract simulation" -ForegroundColor Cyan
    Write-Host "fixture : $Fixture"
    Write-Host "contract: $($fx.contract.policy) $($fx.contract.policy_version)"
    Write-Host ""
}

Invoke-GreenRegressionParserControls
Invoke-RedDeliverableBranchControls
Invoke-TaskLintContractControls
Invoke-BaselineLintContractControls
Invoke-GovernedClaimDeltaControls
Invoke-LintGateContractChecks
if ($VerifyAgainstQueue) {
    Invoke-QueueDriftCheck
    Invoke-BaselineConvergenceQueueProjection
}

foreach ($sc in $fx.scenarios) {
    if ($Scenario -and $sc.id -ne $Scenario) { continue }
    $r = Test-Scenario -Fx $fx -Sc $sc
    if (-not $Quiet) {
        $line = "  {0,-30} outcome={1}" -f $sc.id, $r.outcome
        if ($null -ne $r.halt_wave -and $r.outcome -ne 'COMPLETE') { $line += " wave=$($r.halt_wave)" }
        if ($r.halt_detail) { $line += " detail=$($r.halt_detail)" }
        Write-Host $line
    }
}

$fail = @($script:Assertions | Where-Object { -not $_.Ok })
if (-not $Quiet -or $fail.Count -gt 0) { Write-Host "" }
foreach ($f in $fail) {
    Write-Host ("FAIL {0} :: {1}`n     expected: {2}`n     actual  : {3}" -f $f.Scenario, $f.Name, $f.Expected, $f.Actual) -ForegroundColor Red
}
$total = $script:Assertions.Count
$pass = $total - $fail.Count
if ($fail.Count -eq 0) {
    Write-Host "WAVE_SIM_OK: $pass/$total assertions PASS across $(@($fx.scenarios).Count) scenario(s)" -ForegroundColor Green
    exit 0
}
Write-Host "WAVE_SIM_FAIL: $pass/$total assertions PASS, $($fail.Count) FAIL" -ForegroundColor Red
exit 1
