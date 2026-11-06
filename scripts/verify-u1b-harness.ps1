# Fail closed unless the exact U1b test, all fixtures, and the package pass.
$ErrorActionPreference = 'Stop'
$workspace = Split-Path -Parent $PSScriptRoot
Set-Location $workspace
$env:GOROOT = Join-Path $workspace 'bin/_authoring_gomodcache/golang.org/toolchain@v0.0.1-go1.24.0.windows-amd64'
$env:PATH = (Join-Path $env:GOROOT 'bin') + [IO.Path]::PathSeparator + $env:PATH
$env:GOTOOLCHAIN = 'local'
$env:GOFLAGS = '-mod=readonly'
$env:GOPROXY = 'off'
$env:GOSUMDB = 'off'
$env:GOCACHE = Join-Path $workspace 'bin/_authoring_gocache'
$env:GOMODCACHE = Join-Path $workspace 'bin/_authoring_gomodcache'
$env:GOPATH = Join-Path $workspace 'bin/_authoring_gopath'
$env:TEMP = Join-Path $workspace 'bin/_authoringtmp'
$env:TMP = $env:TEMP
$env:GOLANGCI_LINT_CACHE = Join-Path $workspace 'bin/_authoring_golangci_cache'
$go = Join-Path $env:GOROOT 'bin/go.exe'
& $go version
if ($LASTEXITCODE -ne 0) { exit 1 }
$lines = @(& $go test -json ./internal/cli -count=1 -timeout=30m -run '^TestU1b_' 2>&1)
$nativeExit = $LASTEXITCODE
$lines | ForEach-Object { Write-Output "$_" }
Write-Output "U1b go test native exit: $nativeExit"
$events = @()
try {
    $events = @($lines | ForEach-Object { "$_" | ConvertFrom-Json -ErrorAction Stop })
} catch {
    Write-Output 'U1b guard RED: invalid or missing test JSON'
    exit 1
}
$pkg = 'github.com/softwaresalt/backlogit/internal/cli'
$name = 'TestU1b_NestedRuntimeRootUsesCanonicalAbsolutePathForReparseValidation'
$namedPass = @($events | Where-Object { $_.Package -eq $pkg -and $_.Test -eq $name -and $_.Action -eq 'pass' })
$packagePass = @($events | Where-Object { $_.Package -eq $pkg -and -not $_.Test -and $_.Action -eq 'pass' })
$bad = @($events | Where-Object { $_.Action -in @('skip', 'fail') })
$coverage = @($events | Where-Object { $_.Test -eq "$name/malicious_lock_and_sidecar_leaf_refusal_no_skip" -and $_.Output -match 'U1b security coverage: executedRefusals=4 skippedFixtures=0' })
if ($nativeExit -ne 0 -or $namedPass.Count -ne 1 -or $packagePass.Count -ne 1 -or $bad.Count -ne 0 -or $coverage.Count -ne 1) {
    Write-Output 'U1b guard RED: require named PASS, package PASS, four refusals, and zero skips'
    exit 1
}
Write-Output "U1b guard GREEN: $name"
exit 0
