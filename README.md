# portside

A VS Code-style terminal workspace: file explorer + SSH downloads + port
forwards on the left, [Claude Code](https://claude.com/claude-code) on the
right, composed with tmux.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/Mapika/portside/main/install.sh | sh
```

Requires tmux and (for the right pane) Claude Code.

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

## Usage

```sh
work [dir]      # local workspace (or reattach)
work <host>     # remote workspace: browse + run Claude Code on that server
portside [dir]  # just the explorer, local
portside --host <host>  # just the explorer, connected to a server
```

> Remote mode needs Claude Code installed on the server (it runs there over ssh); portside itself needs nothing remote.

## Keys

| Key | Action |
| --- | --- |
| `↑/↓` `j/k` | move |
| `enter` | expand/collapse folder |
| `:` or `Ctrl+L` | type a path to jump to |
| `Ctrl+H` | switch host (local / ~/.ssh/config hosts) |
| `d` | download selected file/folder to local |
| `r` | refresh |
| `.` | toggle hidden files |
| `R` | reconnect current host |
| `Ctrl+P` | toggle Ports view |
| `a` / `x` | (Ports) add / stop a forward |
| `q` / `Ctrl+C` | quit |

SSH auth uses your agent and unencrypted keys from `~/.ssh/config`.
