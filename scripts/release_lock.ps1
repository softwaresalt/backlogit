<#
.SYNOPSIS
    Release an advisory file lock.
.DESCRIPTION
    Removes the .{filename}.lock file for the specified file.
.PARAMETER FilePath
    Path to the file to unlock, relative to the workspace root.
.EXAMPLE
    scripts/release_lock.ps1 internal/core/artifacts.go
#>
param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$FilePath
)

$resolvedPath = Resolve-Path $FilePath -ErrorAction SilentlyContinue
if (-not $resolvedPath) {
    Write-Warning "File not found: $FilePath (lock may already be released)"
    exit 0
}

$dir = Split-Path $resolvedPath -Parent
$filename = Split-Path $resolvedPath -Leaf
$lockFile = Join-Path $dir ".$filename.lock"

if (-not (Test-Path $lockFile)) {
    Write-Warning "No lock file found for: $FilePath"
    exit 0
}

Remove-Item $lockFile -Force
Write-Host "Lock released: $FilePath"
exit 0
