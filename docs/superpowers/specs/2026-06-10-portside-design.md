# portside — Design Spec

**Date:** 2026-06-10
**Status:** Approved pending user review

## Purpose

A VS Code-like terminal workspace for Linux (developed on WSL2 Ubuntu 24.04). One
command opens a tmux session split into two panes:

- **Left (~35%):** `portside`, a custom Go TUI — a VS Code Explorer-style file
  browser for local and remote (SSH/SFTP) filesystems, with file download and an
  SSH port-forwarding panel.
- **Right (~65%):** Claude Code, running natively. portside never wraps or embeds
  it; tmux owns the split.

Primary use case: SSH into remote servers, browse their files in a tree sidebar,
download files to the local machine, and forward remote ports — while working with
Claude Code alongside.

## Components

### 1. `portside` binary (Go + Bubble Tea)

A single static binary (~10MB, no runtime dependencies), so it can be installed
with `curl | sh` on any Linux box.

Libraries:
- TUI: `github.com/charmbracelet/bubbletea` + `lipgloss` (styling) + `bubbles`
  (text input, list components)
- SSH/SFTP: `golang.org/x/crypto/ssh` + `github.com/pkg/sftp`
- SSH config parsing: `github.com/kevinburke/ssh_config` (reads `~/.ssh/config`
  for hosts, users, keys, ports)

Two views, toggled with a key combo (`Ctrl+P`):

**Explorer view**
- Collapsible directory tree (VS Code style: `▸`/`▾` folders, file/folder icons,
  dim colors for hidden files). Arrow keys + mouse to navigate; Enter/click
  expands or collapses folders.
- Path bar (`:` or `Ctrl+L` to focus): type an absolute path and jump there.
- Host switcher (`Ctrl+H`): pick `local` or any Host from `~/.ssh/config`;
  selecting a remote host opens an SSH+SFTP connection and the tree shows the
  remote filesystem. One active connection at a time (v1).
- Download (`d` on a remote file or folder): copies it via SFTP into a local
  destination directory (prompted, default `~/Downloads`, remembered per
  session). Folders download recursively. Progress shown in the status bar.
- Basic file ops (v1 scope): refresh (`r`), toggle hidden files (`.`). Rename /
  delete / open-in-editor are explicitly out of scope for v1.

**Ports view**
- Lists active forwards: `localhost:LOCAL → remote:REMOTE  [open|error]`.
- `a` adds a forward: prompts for local port and remote port; runs the
  equivalent of `ssh -L` over the already-open SSH connection (a goroutine per
  forward: local TCP listener → ssh channel dial).
- `x` stops the selected forward.
- Forwards live as long as portside runs; they are torn down cleanly on quit.
- Requires a remote host to be connected; on `local` the view shows a hint.

**Auth:** key-based auth via `~/.ssh/config` (IdentityFile) and the SSH agent
(`SSH_AUTH_SOCK`). Password prompts are out of scope for v1 — same keys that
make plain `ssh host` work make portside work.

**Error handling:** connection failures, auth failures, permission-denied on
directories, and broken forwards surface as a status-bar message (red), never a
crash. SFTP disconnects mark the host disconnected and offer reconnect (`R`).

### 2. `work` launcher script

Installed alongside the binary. `work [dir]`:
- If a tmux session `work` exists, reattach.
- Otherwise create it: left pane runs `portside`, right pane runs `claude`,
  both starting in `dir` (default: cwd). 35/65 horizontal split, mouse mode on.

### 3. Distribution

- GitHub repo (user's account) with the Go source.
- GoReleaser + GitHub Actions: on tag push, build linux/amd64 and linux/arm64
  binaries and attach to a GitHub Release.
- `install.sh` in the repo root: detects arch, downloads the latest release
  binary to `~/.local/bin/portside`, installs `work` next to it. Usage:
  `curl -fsSL https://raw.githubusercontent.com/<user>/portside/main/install.sh | sh`

## Architecture notes

- Bubble Tea Model-Update-View. A `Filesystem` interface (`List(path)`,
  `Download(remote, local)`, …) with two implementations: `localFS` (os calls)
  and `sftpFS`. The tree view renders against the interface and does not know
  which backend it is on.
- SSH connection manager owns the single `*ssh.Client`; SFTP client and port
  forwards both hang off it, so one connection serves both views.
- All network calls run as `tea.Cmd`s (async) so the UI never blocks; a spinner
  shows during slow operations.

## Testing

- Unit tests for: SSH config parsing/host listing, tree model
  (expand/collapse/flatten), path-bar navigation logic, and the `Filesystem`
  interface against `localFS` (tempdir fixtures).
- `sftpFS` and port forwarding tested against a local in-process SSH server
  (`gliderlabs/ssh`) in integration tests — no real remote needed in CI.
- TUI logic tested at the Update-function level (feed messages, assert model
  state), not by screen-scraping.

## Out of scope for v1

Rename/delete/edit files, multiple simultaneous host connections, password auth,
remote port auto-detection (VS Code-style), file preview, search, Windows/macOS
builds.
