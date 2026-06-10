# portside

A VS Code-style terminal workspace: file explorer + SSH downloads + port
forwards on the left, [Claude Code](https://claude.com/claude-code) on the
right, composed with tmux.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/Mapika/portside/main/install.sh | sh
```

Requires tmux and (for the right pane) Claude Code.

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
