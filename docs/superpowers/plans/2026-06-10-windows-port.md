# Windows Port (v0.3.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Native Windows support: build-tagged SSH-agent dialing, Windows Terminal launcher (work.ps1/work.cmd), windows release artifacts + install.ps1, windows CI.

**Architecture:** Isolate the only OS-specific code (agent socket vs named pipe) behind a `dialAgent()` helper with build tags. Everything else ships as-is. Launcher and installer are PowerShell siblings of the existing bash scripts.

**Tech Stack:** Go 1.25 (+ github.com/Microsoft/go-winio, windows-only), PowerShell 5.1-compatible scripts, Windows Terminal `wt` CLI, GoReleaser.

**Spec:** docs/superpowers/specs/2026-06-10-windows-port-design.md

---

### Task 1: Build-tagged agent dialer

**Files:**
- Create: `internal/sshconn/agent_unix.go`, `internal/sshconn/agent_windows.go`
- Modify: `internal/sshconn/conn.go` (AuthMethods uses dialAgent)

- [ ] **Step 1: Write the failing test** — add to `internal/sshconn/conn_test.go`:

```go
func TestAuthMethodsWithoutAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "") // no agent reachable on any platform
	methods, closers := AuthMethods(Params{})
	if len(closers) != 0 {
		t.Fatalf("want no closers without an agent, got %d", len(closers))
	}
	_ = methods // may be empty or key-only; must not panic
}
```

- [ ] **Step 2:** `go test ./internal/sshconn/ -run TestAuthMethodsWithoutAgent -v` — PASS already on linux (guard exists); this is a regression net for the refactor.

- [ ] **Step 3: Create the dialers.**

`internal/sshconn/agent_unix.go`:

```go
//go:build !windows

package sshconn

import (
	"errors"
	"net"
	"os"
)

// dialAgent connects to the OpenSSH agent via the unix socket in
// SSH_AUTH_SOCK.
func dialAgent() (net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK not set")
	}
	return net.Dial("unix", sock)
}
```

`internal/sshconn/agent_windows.go`:

```go
//go:build windows

package sshconn

import (
	"net"

	"github.com/Microsoft/go-winio"
)

// dialAgent connects to the Windows OpenSSH agent service's named pipe.
func dialAgent() (net.Conn, error) {
	return winio.DialPipe(`\\.\pipe\openssh-ssh-agent`, nil)
}
```

- [ ] **Step 4: Refactor AuthMethods** in `conn.go` — replace the SSH_AUTH_SOCK block:

```go
	if conn, err := dialAgent(); err == nil {
		methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		closers = append(closers, conn)
	}
```

(drop the now-unused "os" usage only if nothing else needs it — key reading still does)

- [ ] **Step 5:** `go get github.com/Microsoft/go-winio@latest`, then verify both targets:

```bash
go vet ./... && go test ./... -count=1
GOOS=windows go vet ./... && GOOS=windows go build ./...
```

- [ ] **Step 6: Commit** — `git add -A && git commit -m "feat: build-tagged ssh agent dialing for windows named pipe"`

---

### Task 2: work.ps1 + work.cmd

**Files:**
- Create: `scripts/work.ps1`, `scripts/work.cmd`

- [ ] **Step 1: Create `scripts/work.ps1`:**

```powershell
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
```

Note for the implementer: the backtick-semicolon (`` `; ``) is REQUIRED — `;` separates wt subcommands but PowerShell would otherwise eat it. Test the exact quoting by running the wt command with `echo` substituted if unsure; `wt` argument parsing is finicky. If `powershell -NoExit -Command "portside --host X"` proves unnecessary complexity, plain `portside --host $Target` as the pane command is fine — prefer the simplest form that works.

- [ ] **Step 2: Create `scripts/work.cmd`:**

```bat
@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0work.ps1" %*
```

- [ ] **Step 3: Validate.** No Windows box here: at minimum run a PowerShell parse check if `pwsh` exists (`pwsh -NoProfile -Command "[System.Management.Automation.Language.Parser]::ParseFile('scripts/work.ps1', [ref]$null, [ref]$errs); $errs"`); if pwsh is unavailable locally, note it — the CI windows job (Task 4) performs the parse check.

- [ ] **Step 4: Commit** — `git add scripts && git commit -m "feat: windows terminal launcher (work.ps1/work.cmd)"`

---

### Task 3: install.ps1

**Files:**
- Create: `install.ps1`

- [ ] **Step 1: Create `install.ps1`:**

```powershell
# portside installer for Windows:
#   irm https://raw.githubusercontent.com/Mapika/portside/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = "Mapika/portside"
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
$dest = Join-Path $env:LOCALAPPDATA "Programs\portside"

$url = "https://github.com/$repo/releases/latest/download/portside_windows_$arch.zip"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) "portside_install"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $tmp | Out-Null

Write-Host "downloading $url"
Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmp "portside.zip")
Expand-Archive -Path (Join-Path $tmp "portside.zip") -DestinationPath $tmp -Force

New-Item -ItemType Directory -Force $dest | Out-Null
Copy-Item (Join-Path $tmp "portside.exe") $dest -Force
Copy-Item (Join-Path $tmp "scripts\work.ps1") $dest -Force
Copy-Item (Join-Path $tmp "scripts\work.cmd") $dest -Force
Copy-Item (Join-Path $tmp "scripts\work.cmd") (Join-Path $dest "work.bat") -Force
Remove-Item -Recurse -Force $tmp

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $dest) {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
    Write-Host "added $dest to your user PATH (restart the terminal to pick it up)"
}
Write-Host "installed portside and work to $dest"
```

- [ ] **Step 2:** Parse-check like Task 2. Commit — `git add install.ps1 && git commit -m "feat: windows powershell installer"`

---

### Task 4: GoReleaser windows builds + windows CI

**Files:**
- Modify: `.goreleaser.yaml`, `.github/workflows/ci.yml`

- [ ] **Step 1: `.goreleaser.yaml`** — replace builds/archives:

```yaml
version: 2
project_name: portside
builds:
  - main: .
    binary: portside
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - windows
    goarch:
      - amd64
      - arm64
archives:
  - id: linux
    formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    files:
      - scripts/work
  - id: windows
    formats: [zip]
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    files:
      - scripts/work.ps1
      - scripts/work.cmd
```

Caveat for the implementer: a single `archives` entry cannot vary format AND file-set per OS in goreleaser v2 without `format_overrides`; if two archive ids over the same build produce duplicate-artifact errors, use ONE archive entry with `format_overrides: [{goos: windows, formats: [zip]}]` and include both script sets (work, work.ps1, work.cmd) in all archives — the linux installer only extracts `scripts/work`, the windows installer only `scripts/work.ps1`/`work.cmd`, so extra files are harmless. Verify with `goreleaser check` if available, else rely on the tag build and fix forward.

- [ ] **Step 2: `.github/workflows/ci.yml`** — add a windows job:

```yaml
  test-windows:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: go vet ./...
      - run: go test ./...
      - name: parse powershell scripts
        shell: pwsh
        run: |
          $errs = $null
          [System.Management.Automation.Language.Parser]::ParseFile("$PWD/scripts/work.ps1", [ref]$null, [ref]$errs) | Out-Null
          if ($errs) { $errs; exit 1 }
          [System.Management.Automation.Language.Parser]::ParseFile("$PWD/install.ps1", [ref]$null, [ref]$errs) | Out-Null
          if ($errs) { $errs; exit 1 }
```

(keep the existing ubuntu job; note: skip `-race` on windows if it needs CGO — plain `go test ./...` is fine there)

- [ ] **Step 3:** `git add -A && git commit -m "feat: windows release artifacts and ci"`

---

### Task 5: README + release v0.3.0

- [ ] **Step 1: README.md** — add a Windows section under Install:

```markdown
### Windows

```powershell
irm https://raw.githubusercontent.com/Mapika/portside/main/install.ps1 | iex
```

Requires [Windows Terminal](https://aka.ms/terminal). `work <host>` opens
portside (left) + Claude Code on that server via ssh (right). Uses your
Windows `~/.ssh/config` and the Windows OpenSSH agent
(`Get-Service ssh-agent | Set-Service -StartupType Automatic; Start-Service ssh-agent`).
Downloads land in `C:\Users\<you>\Downloads`. Note: no session reattach on
Windows (Windows Terminal has no tmux-style attach).
```

- [ ] **Step 2:** Full local gates: `go vet ./... && go test ./... -count=1 && GOOS=windows go build ./...`. Commit.

- [ ] **Step 3:** Merge to main, push, CI green (both jobs), tag v0.3.0, release green, verify `gh release view v0.3.0` lists 4 archives (linux amd64/arm64 tar.gz, windows amd64/arm64 zip). Linux curl-install re-verify. Windows install verified manually by the user.
