# work [host|dir] - Windows Terminal workspace: portside (left 35%) + claude (right 65%).
# If the argument is a Host alias from ~/.ssh/config, both panes target that
# machine: portside connects over SFTP and claude runs there via ssh.
param([string]$Target = "")

$ErrorActionPreference = "Stop"

function Test-SshHost([string]$Name) {
    if (-not $Name) { return $false }
    $config = Join-Path $env:USERPROFILE ".ssh\config"
    if (-not (Test-Path $config)) { return $false }
    foreach ($line in Get-Content $config) {
        if ($line -match '^\s*[Hh]ost\s+(.+)$') {
            foreach ($alias in ($Matches[1] -split '\s+')) {
                if ($alias -eq $Name -and $alias -notmatch '[*?!]') { return $true }
            }
        }
    }
    return $false
}

if (-not (Get-Command wt -ErrorAction SilentlyContinue)) {
    Write-Error "Windows Terminal (wt) is required: install it from the Microsoft Store."
}

if (Test-SshHost $Target) {
    # remote mode: browse + run claude on the server
    wt -w 0 new-tab --title work powershell -NoExit -Command "portside --host $Target" `; split-pane -V --size 0.65 ssh -t $Target -- bash -lc claude
} else {
    $dir = if ($Target) { $Target } else { (Get-Location).Path }
    if (-not (Test-Path $dir)) { Write-Error "no such directory or ssh host: $Target" }
    $claude = "claude"
    if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
        if (Get-Command wsl -ErrorAction SilentlyContinue) {
            $claude = "wsl -e claude"
        } else {
            Write-Error "claude not found on PATH (install Claude Code, or WSL with claude inside)"
        }
    }
    wt -w 0 new-tab --title work -d $dir portside `; split-pane -V --size 0.65 -d $dir $claude
}
