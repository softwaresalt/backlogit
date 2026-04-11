$env:COPILOT_HOME = ".\.copilot"
$env:ENGRAM_DATA_DIR = ".\.engram"
$env:GITHUB_TOKEN = (gh auth token)
$copilotExe = if ($env:COPILOT_EXE) {
    $env:COPILOT_EXE
} else {
    (Get-Command "copilot.exe" -ErrorAction SilentlyContinue).Source
}

if (-not $copilotExe) {
    throw "Unable to locate copilot.exe. Set COPILOT_EXE or add copilot.exe to PATH."
}

& $copilotExe
