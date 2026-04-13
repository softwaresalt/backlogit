<#
.SYNOPSIS
    Acquire an advisory file lock.
.DESCRIPTION
    Creates a .{filename}.lock file in the same directory as the target file.
    Fails with exit code 1 if the lock already exists (file is locked by
    another process).
.PARAMETER FilePath
    Path to the file to lock, relative to the workspace root.
.EXAMPLE
    scripts/acquire_lock.ps1 internal/core/artifacts.go
#>
param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$FilePath
)

$resolvedPath = Resolve-Path $FilePath -ErrorAction SilentlyContinue
if (-not $resolvedPath) {
    Write-Error "File not found: $FilePath"
    exit 1
}

$dir = Split-Path $resolvedPath -Parent
$filename = Split-Path $resolvedPath -Leaf
$lockFile = Join-Path $dir ".$filename.lock"

if (Test-Path $lockFile) {
    $lockContent = Get-Content $lockFile -Raw
    Write-Error "File is already locked: $FilePath`nLock info: $lockContent"
    exit 1
}

$agentName = if ($env:AGENT_NAME) { $env:AGENT_NAME } else { "unknown" }
$timestamp = Get-Date -Format "o"
$pid = $PID

$lockContent = @"
agent: $agentName
timestamp: $timestamp
pid: $pid
"@

Set-Content -Path $lockFile -Value $lockContent -NoNewline
Write-Host "Lock acquired: $FilePath"
exit 0
