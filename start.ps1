# Load globally-installed Copilot CLI plugins (their agents + skills) into this
# workspace WITHOUT copying files (copies go stale when the global install
# updates) and WITHOUT moving COPILOT_HOME. Plugin agents are namespaced
# "<plugin>:<agent>", so they never collide with this workspace's .github/agents/
# entries. Discover every installed plugin (a directory holding a plugin.json)
# under the user's real Copilot home and pass each as a --plugin-dir argument.
$pluginArgs = @()
$globalPluginHome = Join-Path ([Environment]::GetFolderPath('UserProfile')) '.copilot\installed-plugins'
if (Test-Path $globalPluginHome) {
    Get-ChildItem $globalPluginHome -Recurse -Depth 2 -Filter 'plugin.json' -File -ErrorAction SilentlyContinue |
        ForEach-Object {
            $pluginArgs += '--plugin-dir'
            $pluginArgs += $_.DirectoryName
        }
}

if (Test-Path .env.local) {
  Get-Content .env.local | ForEach-Object {
    if ($_ -match '^\s*([A-Z_][A-Z0-9_]*)\s*=\s*(.+?)\s*$') {
      Set-Item -Path "env:$($matches[1])" -Value $matches[2]
    }
  }
}

$env:COPILOT_HOME = if ($env:COPILOT_HOME) { $env:COPILOT_HOME } else { ".\.copilot" }
$env:ENGRAM_DATA_DIR = ".\.engram"   # Uncomment when the agent-engram capability pack is active
$env:GITHUB_TOKEN = (gh auth token)
$copilotExe = if ($env:COPILOT_EXE_PATH) {
    $env:COPILOT_EXE_PATH
} elseif ($env:COPILOT_EXE) {
    $env:COPILOT_EXE
} else {
    $copilotCommand = Get-Command "copilot" -ErrorAction SilentlyContinue
    if ($copilotCommand) { $copilotCommand.Source } else { $null }
}

if (-not $copilotExe) {
    throw "Unable to locate Copilot CLI. Set COPILOT_EXE_PATH (or COPILOT_EXE for backward compatibility) or add 'copilot' to PATH."
}
$backlogitCmd = Get-Command backlogit -ErrorAction SilentlyContinue
if ($backlogitCmd -and (Test-Path ".\.backlogit")) {
    backlogit sync --cwd .
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "backlogit sync failed (non-fatal) with exit code $LASTEXITCODE."
    }
}

& $copilotExe @pluginArgs @args

# ── Claude Code ─────────────────────────────────────────────────────────────
# Uncomment to run Claude Code with workspace-local state directories.
# CLAUDE_CONFIG_DIR redirects Claude's config and history to the workspace.
# Verify that your installed version of Claude Code supports this env variable.
#
# $env:CLAUDE_CONFIG_DIR = ".\.claude"
# claude

# ── OpenAI Codex / Agents ────────────────────────────────────────────────────
# Uncomment to run Codex with a workspace-local API key file.
#
# $env:OPENAI_API_KEY = (Get-Content .openai-token -Raw).Trim()
# codex
