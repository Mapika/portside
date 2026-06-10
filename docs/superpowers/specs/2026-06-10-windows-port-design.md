# portside Windows port (v0.3.0) — Design Spec

**Date:** 2026-06-10
**Status:** Approved (user chose "Native Windows port")

## Purpose

Run the portside workspace natively on Windows, started from Windows Terminal:
`portside.exe` uses the **Windows** `~/.ssh/config` and the Windows OpenSSH
agent, downloads land in `C:\Users\<user>\Downloads`, and the two-pane layout
is composed with **Windows Terminal split panes** (`wt`) instead of tmux.
Linux/WSL behavior is unchanged.

## Components

### 1. portside.exe (Go changes)

- **SSH agent:** Windows OpenSSH agent listens on the named pipe
  `\\.\pipe\openssh-ssh-agent`, not a unix socket. Add build-tagged dialers:
  - `internal/sshconn/agent_unix.go` (`//go:build !windows`): current
    `SSH_AUTH_SOCK` unix dial.
  - `internal/sshconn/agent_windows.go` (`//go:build windows`):
    `winio.DialPipe` to the named pipe (dep: `github.com/Microsoft/go-winio`).
  - `AuthMethods` calls the shared `dialAgent() (net.Conn, error)` helper.
- Everything else is already portable: `os.UserHomeDir()` yields
  `C:\Users\<user>`, so ssh config / known_hosts / key paths and the
  `~/Downloads` default all resolve correctly; `fs.Local` uses
  `path/filepath`; remote SFTP paths already use POSIX `path` +
  `filepath.FromSlash` on the local side; Bubble Tea supports Windows
  Terminal (mouse included).
- Verification on Linux: `GOOS=windows go build` + `go vet`; CI gains a
  `windows-latest` test job (the in-process SSH test server is pure Go and
  runs on Windows).

### 2. work.ps1 + work.cmd (launcher)

- `scripts/work.ps1`: PowerShell equivalent of `scripts/work`:
  - `work <host>` → host alias found in `$env:USERPROFILE\.ssh\config` →
    left pane `portside --host <host>`, right pane `ssh -t <host> bash -lc claude`.
  - `work [dir]` → left `portside <dir>`, right `claude` started in `<dir>`
    (if `claude` is not on PATH, fall back to `wsl -e claude`; if neither
    exists, print guidance).
  - Layout via Windows Terminal CLI: `wt -w 0 new-tab … ; split-pane -V --size 0.65 …`
    (left 35% / right 65%).
  - **No session reattach** (Windows Terminal has no attach concept) —
    documented limitation; running `work` again opens a new tab.
- `scripts/work.cmd`: thin shim so `work` also works from `cmd.exe`
  (`powershell -ExecutionPolicy Bypass -File …\work.ps1 %*`).

### 3. Distribution

- `.goreleaser.yaml`: add `windows` to `goos` (amd64 + arm64); windows
  archives are `.zip` and include `scripts/work.ps1` + `scripts/work.cmd`;
  linux archives unchanged.
- `install.ps1` (repo root): PowerShell installer —
  `irm https://raw.githubusercontent.com/Mapika/portside/main/install.ps1 | iex`:
  detects arch, downloads the latest `portside_windows_<arch>.zip`, installs
  `portside.exe`, `work.ps1`, `work.cmd` into
  `$env:LOCALAPPDATA\Programs\portside`, and adds that dir to the **user**
  PATH if missing.
- CI: existing ubuntu job + new `windows-latest` job (`go vet`, `go test`).

## Out of scope

tmux-style reattach on Windows, Windows ARM testing (built but untested),
code signing, winget/scoop packaging, the reverse-tunnel "send home" mode
(superseded — the user runs portside on Windows directly).

## Testing

- Go: existing suites must pass on `windows-latest` CI; agent dialers are
  build-tagged and excluded from coverage assertions (no agent in CI).
- Scripts: PowerShell syntax check (`pwsh -NoProfile -Command` parse) in CI
  on the windows job; behavioral testing is manual on the user's machine.
