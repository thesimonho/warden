---
paths:
  - "internal/terminal/**/*"
  - "container/scripts/**/*"
  - "engine/**/*"
  - "service/**/*"
  - "eventbus/**/*"
  - "web/src/**/terminal*"
  - "web/src/**/ws*"
  - "web/src/**/websocket*"
  - "web/src/**/worktree*"
  - "docs/developer/terminology.md"
  - "docs/developer/ux-flows.md"
---

# Terminal Lifecycle

See `docs/developer/terminology.md` for the full state machine (worktree states, terminal actions, Claude activity sub-states) and `docs/developer/ux-flows.md` for the UX flows that use them.

The critical invariant: **WebSocket connections are disposable, the tmux session is not**. Disconnecting closes the WebSocket but leaves the tmux session alive so the agent keeps working in the background. Only an explicit "kill" destroys the tmux session.

Browser connects via `GET /api/v1/projects/{id}/{agentType}/ws/{wid}` (WebSocket) → Go backend proxy (`internal/terminal/`) → scrollback replay via `tmux capture-pane` → `docker exec` with TTY mode attached to existing tmux session.

Backend calls `create-terminal.sh` to initialize tmux sessions for new worktrees. The script:

- Detects previous sessions via `exit_code` + JSONL files for auto-resume (`--continue` / `resume --last`)
- Configures tmux: `status off`, `mouse off`, `history-limit 50000`, `window-size latest`, `-u` (UTF-8)
- Unsets `TMUX` env var so agents don't detect the tmux wrapper
- Writes `exit_code` on agent exit for future auto-resume

All tmux commands (`has-session`, `list-sessions`, `kill-session`, `capture-pane`) must run as `ContainerUser` ("warden") — tmux sessions are user-scoped. Use `TmuxSessionName(worktreeID)` for the agent session and `TmuxShellSessionName(worktreeID)` for the auxiliary bash-shell session (both from `constants/`) — never hardcode the `warden-` / `warden-shell-` prefixes.

Every worktree has **two** tmux sessions: the agent session and the shell session. When you add lifecycle logic (kill, reset, validate, orphan detection), handle both — missing one leaves stray processes or dangling state.
