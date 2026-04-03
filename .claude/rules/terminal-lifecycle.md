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
  - "docs/terminology.md"
  - "docs/ux-flows.md"
---

# Terminal Lifecycle

See `docs/terminology.md` for the full state machine (worktree states, terminal actions, Claude activity sub-states) and `docs/ux-flows.md` for the UX flows that use them.

The critical invariant: **WebSocket connections are disposable, the tmux session is not**. Disconnecting closes the WebSocket but leaves the tmux session alive so Claude keeps working in the background. Only an explicit "kill" destroys the tmux session.

Browser connects via `GET /api/v1/projects/{id}/{agentType}/ws/{wid}` (WebSocket) → Go backend proxy (`internal/terminal/`) → `docker exec` with TTY mode attached to existing tmux session. Backend calls `create-terminal.sh` to initialize tmux session for new worktrees.
