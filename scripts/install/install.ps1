[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$Repo = "softwaresalt/backlogit"
$InstallDir = if ($env:BACKLOGIT_INSTALL_DIR) {
    $env:BACKLOGIT_INSTALL_DIR
} else {
    Join-Path $HOME "bin"
}

function Get-BacklogitArch {
    switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "unsupported architecture: $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
    }
}

$arch = Get-BacklogitArch
$assetName = "backlogit-windows-$arch.exe"
$baseUrl = "https://github.com/$Repo/releases/latest/download"
$assetUrl = "$baseUrl/$assetName"
$checksumsUrl = "$baseUrl/SHA256SUMS"
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("backlogit-install-" + [guid]::NewGuid().ToString("N"))
$assetPath = Join-Path $tmpDir $assetName
$checksumsPath = Join-Path $tmpDir "SHA256SUMS"

New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    Write-Host "Downloading $assetName from GitHub releases/latest..."
    Invoke-WebRequest -Uri $assetUrl -OutFile $assetPath
    Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath

    $checksumLine = Select-String -Path $checksumsPath -Pattern ([regex]::Escape($assetName) + '$') | Select-Object -First 1
    if (-not $checksumLine) {
        throw "checksum entry not found for $assetName in SHA256SUMS"
    }

    $expectedHash = ($checksumLine.Line -split '\s+')[0].ToLowerInvariant()
    $actualHash = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expectedHash -ne $actualHash) {
        throw "checksum mismatch for $assetName"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path $assetPath -Destination (Join-Path $InstallDir "backlogit.exe") -Force

    Write-Host "Installed backlogit to $(Join-Path $InstallDir 'backlogit.exe')"

    $pathEntries = ($env:PATH -split ';') | Where-Object { $_ }
    if ($pathEntries -contains $InstallDir) {
        Write-Host "backlogit is ready to use."
    } else {
        Write-Host "Add this directory to PATH:"
        Write-Host "  $InstallDir"
    }
}
finally {
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
}
