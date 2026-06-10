# portside

A terminal file explorer for remote machines. It connects over plain SSH
(nothing to install on the server), shows the remote filesystem as a tree,
pulls files down to your machine with one key, and forwards ports like VS
Code's Ports panel. It's built to sit in a split next to an AI agent — the
`work` command opens both panes at once — but it's useful on its own too.

## Watching your agent work

portside can watch the files an agent is touching in real time. With watch
mode on (default), the tree polls every 3 seconds and highlights any file
that changed in the last 45 seconds with an orange dot marker. Modified
files are coloured yellow and untracked files green when the directory is
inside a git repository (no extra tooling required on the server — portside
runs `git status` itself). Press `w` to toggle watch on or off, and press
`C` to type the list of recently changed paths straight into the agent pane
next door, so you can ask about the files it just edited without copy-pasting.

![demo](demo/demo.gif)

I made this because my actual workday was: a terminal with an AI session on a
server, a second terminal for scp, and a VS Code remote window I only opened
to click "download" on some file. That's three tools for one job. portside is
the one tool: browse the server, grab the file, forward the port, and hand
file paths straight to the agent pane next door (`c` types the selected path
into it).

## Install

Linux and macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/Mapika/portside/main/install.sh | sh
```

Needs tmux for the split layout (`apt install tmux` / `brew install tmux`).

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/Mapika/portside/main/install.ps1 | iex
```

Needs [Windows Terminal](https://aka.ms/terminal); the split uses `wt` panes
instead of tmux, so there's no session reattach on Windows. Hosts and keys
come from your Windows `~/.ssh/config` and the Windows OpenSSH agent.

## Usage

```sh
work                     # local workspace: portside left, claude right
work <host>              # remote workspace: browse <host>, claude runs on it via ssh
work <dir>               # local workspace in that directory
work -a codex <host>     # use codex (or any other agent) instead of claude
portside --host <host>   # just the explorer, no agent pane
```

`<host>` is any Host alias from your `~/.ssh/config`. Auth uses your SSH
agent and unencrypted key files — if `ssh <host>` works without a password
prompt, portside works. Nothing runs on the server (in remote `work` mode,
the agent does — install it there first).

The agent defaults to `claude` (Claude Code). Override with `-a <cmd>` or by
setting `WORK_AGENT` in your environment.

Downloads go to `~/Downloads` by default (`C:\Users\<you>\Downloads` on
Windows); the `save to:` prompt remembers what you last typed.

## Keys

| Key | Action |
| --- | --- |
| `↑/↓` `j/k` | move |
| `enter` `→/l` | expand folder (enter also collapses) |
| `←/h` | collapse folder |
| `:` or `Ctrl+L` | type a path to jump to |
| `Ctrl+H` | switch host (local / ~/.ssh/config hosts) |
| `d` | download selected file/folder |
| `u` | upload a local file/folder into the selected directory |
| `m` | rename selected file/folder |
| `D` | delete selected file/folder (recursive; prompts y/N) |
| `n` | create a new folder inside the selected directory |
| `c` | send the selected path to the agent pane (types it in; clipboard fallback if no tmux) |
| `C` | send all recently changed paths (last 45s, most recent first) to the agent pane |
| `w` | toggle watch mode (auto-refresh every 3s with change highlights) on/off |
| `r` | refresh |
| `.` | toggle hidden files |
| `R` | reconnect current host |
| `Ctrl+P` | toggle Ports view |
| `a` / `x` | (Ports) add / stop a forward |
| `q` / `Ctrl+C` | quit |

## Notes

- The demo gif is scripted and reproducible: `demo/record.sh` (needs
  [vhs](https://github.com/charmbracelet/vhs)). The "server" in it is a
  throwaway SSH server from `demo/sshserver`, so no real machines were
  harmed.
- Not yet supported, planned: passphrase-protected keys without an agent,
  password auth, ProxyJump/bastion hosts, file rename/delete/upload. If one
  of these blocks you, open an issue so I know what to do first.
