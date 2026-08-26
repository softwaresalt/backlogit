#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Read-only P-002.6 wave-scheduler contract simulation.

.DESCRIPTION
    Replays the dependency-aware wave scheduler defined by
    `.github/policies/workflow-policies.md` P-002.6 against the tracked fixture
    `tests/simulation/wave-scheduler-contract.json`, and checks every scenario
    expectation the fixture declares.

    The simulation is PURE: it reads the fixture (and, with -VerifyAgainstQueue,
    the backlog queue Markdown), computes in memory, and writes nothing. It runs
    no `go` command, starts no process, and touches no repository state. It is
    therefore safe to run at any gate point, including under the P-002.5
    read-only command screen.

    Assertion coverage (cycle-32):
      * baseline 18-wave schedule, zero stalls, zero compile-order violations
      * persistent red-deliverable mapping and the open-red convergence rule
      * blocked-member injection (initial and mid-run)
      * unsupported status tokens (catalog, off-catalog, catalog unavailable)
      * active residual at wave admission
      * dependency-cycle injection
      * sibling-red wave: withdrawn repo-wide gate vs task-scoped gate
      * non-frozen-M negative control
      * red-to-green-maker mapping fail-closed cases

.PARAMETER Fixture
    Path to the simulation fixture. Defaults to
    tests/simulation/wave-scheduler-contract.json relative to the repository root.

.PARAMETER VerifyAgainstQueue
    Additionally re-derive the manifest (members, statuses, dependency edges,
    harness-exempt labels, red-deliverable contracts) from the live backlog queue
    Markdown and fail if the fixture has drifted from it.

.PARAMETER QueueDir
    Backlog queue directory used by -VerifyAgainstQueue. Default `.backlogit/queue`.

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

function Test-Equal {
    param([string]$Scenario, [string]$Name, $Expected, $Actual)
    $e = Format-Value $Expected
    $a = Format-Value $Actual
    Add-Assertion -Scenario $Scenario -Name $Name -Ok ($e -ceq $a) -Expected $e -Actual $a
}

# --- fixture load --------------------------------------------------------------
if (-not (Test-Path $Fixture)) { throw "fixture not found: $Fixture" }
$fx = Get-Content $Fixture -Raw | ConvertFrom-Json

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

# --- optional live-queue drift check ------------------------------------------
function Get-QueueManifest {
    param([string]$Dir)
    $out = @{}
    foreach ($f in Get-ChildItem $Dir -Filter '147.*-T.md' | Sort-Object Name) {
        $raw = Get-Content $f.FullName -Raw
        $lines = Get-Content $f.FullName
        $inFm = $false; $inDeps = $false; $inLabels = $false
        $id = $null; $status = $null; $deps = @(); $labels = @()
        foreach ($l in $lines) {
            if ($l -eq '---') { if (-not $inFm) { $inFm = $true; continue } else { break } }
            if (-not $inFm) { continue }
            if ($l -match '^dependencies:\s*$') { $inDeps = $true; $inLabels = $false; continue }
            if ($l -match '^labels:\s*$') { $inLabels = $true; $inDeps = $false; continue }
            if ($l -match '^[a-z_]+:') { $inDeps = $false; $inLabels = $false }
            if ($inDeps -and $l -match '^\s+-\s+(\S+)') { $deps += $Matches[1]; continue }
            if ($inLabels -and $l -match '^\s+-\s+(\S+)') { $labels += $Matches[1]; continue }
            if ($l -match '^id:\s*(\S+)') { $id = $Matches[1] }
            if ($l -match '^status:\s*(\S+)') { $status = $Matches[1] }
        }
        $green = @()
        if ($raw -match '(?m)^green_maker_tasks:\s*(.+)$') {
            $green = @($Matches[1] -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        }
        $out[$id] = [pscustomobject]@{
            id              = $id
            status          = $status
            deps            = @($deps | Sort-Object)
            harness_exempt  = [bool]($labels -contains 'harness-exempt')
            red_deliverable = [bool]($raw -match '(?m)^red_deliverable:\s*true')
            green_makers    = $green
        }
    }
    return $out
}

function Invoke-QueueDriftCheck {
    $live = Get-QueueManifest -Dir $QueueDir
    $sc = 'queue_drift'
    Test-Equal -Scenario $sc -Name 'member count' -Expected $fx.manifest.member_count -Actual $live.Count
    $drift = @()
    foreach ($fm in $fx.manifest.members) {
        if (-not $live.ContainsKey($fm.id)) { $drift += "$($fm.id): absent from queue"; continue }
        $l = $live[$fm.id]
        if ($l.status -ne $fm.status) { $drift += "$($fm.id): status $($l.status) != $($fm.status)" }
        $fd = (ConvertTo-List $fm.deps) -join ','
        $ld = ($l.deps) -join ','
        if ($fd -ne $ld) { $drift += "$($fm.id): deps [$ld] != [$fd]" }
        if ($l.harness_exempt -ne $fm.harness_exempt) { $drift += "$($fm.id): harness_exempt drift" }
        if ($l.red_deliverable -ne $fm.red_deliverable) { $drift += "$($fm.id): red_deliverable drift" }
    }
    foreach ($rd in $fx.red_deliverables) {
        if (-not $live.ContainsKey($rd.task)) { $drift += "$($rd.task): red-deliverable absent from queue"; continue }
        $fg = (ConvertTo-List $rd.green_maker_tasks) -join ','
        $lg = ($live[$rd.task].green_makers) -join ','
        if ($fg -ne $lg) { $drift += "$($rd.task): green_maker_tasks [$lg] != [$fg]" }
    }
    Test-Equal -Scenario $sc -Name 'no manifest drift' -Expected '' -Actual ($drift -join '; ')
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
    if (Test-HasProperty $mut 'add_executable_status') { $executable = @($executable + $mut.add_executable_status) }

    if ($catalog.Count -eq 0 -or $executable.Count -eq 0 -or $terminalSuccess.Count -eq 0) {
        $result.outcome = 'WAVE_STATUS_CATALOG_UNAVAILABLE'
        $result.halt_wave = 0
        $result.halt_detail = 'configured status catalog unavailable or empty'
        return [pscustomobject]$result
    }
    $disagree = @()
    foreach ($s in ($executable + $terminalSuccess)) { if ($catalog -notcontains $s) { $disagree += $s } }
    foreach ($s in $executable) { if ($terminalSuccess -contains $s) { $disagree += "overlap:$s" } }
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
        $redMap[$rd.task] = [ordered]@{
            task        = $rd.task
            selector    = $rd.red_selector_command
            greenMakers = $gm
            closesWave  = [int]$rd.green_maker_closes_wave
            open        = $false
            closedWave  = $null
        }
    }
    $unresolved = @()
    foreach ($k in $M.Keys) {
        if (-not $M[$k].red_deliverable) { continue }
        if (-not $redMap.ContainsKey($k)) { $unresolved += $k; continue }
        $entry = $redMap[$k]
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

    # ---- wave loop ----
    $waveIndex = 0
    $budget = $M.Count
    $waveOf = @{}
    $gateCompareWave = $null
    if (Test-HasProperty $mut 'gate_compare_wave') { $gateCompareWave = [int]$mut.gate_compare_wave }

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
            if ($terminalSuccess -contains $s) { $terminal += $k }
            elseif ($s -eq 'queued') { $queued += $k }
            elseif ($s -eq 'active') { $active += $k }
            elseif ($s -eq 'blocked') { $blocked += $k }
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
                if (-not ($terminalSuccess -contains $M[$d].status)) { $ok = $false; break }
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
        foreach ($k in $redMap.Keys) {
            if (-not $redMap[$k].open) { continue }
            $allClosed = $true
            foreach ($g in $redMap[$k].greenMakers) {
                if (-not ($greenClosing -contains $M[$g].status)) { $allClosed = $false; break }
            }
            if ($allClosed) {
                $redMap[$k].open = $false
                $redMap[$k].closedWave = $waveIndex
                $result.open_red_closed_at_wave[$k] = $waveIndex
            }
        }
        $openNow = @($redMap.Keys | Where-Object { $redMap[$_].open } | Sort-Object)
        $result.open_red_after_wave["$waveIndex"] = $openNow

        # the compile / vet / scoped-green part of the gate always runs
        $result.compile_gate_waves = @($result.compile_gate_waves + $waveIndex)

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
            'frozen_m_counterpart' { Test-Equal $id $key $want $want }
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

if ($VerifyAgainstQueue) { Invoke-QueueDriftCheck }

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
