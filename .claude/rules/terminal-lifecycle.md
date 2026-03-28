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

The critical invariant: **WebSocket connections are disposable, abduco is not**. Disconnecting closes the WebSocket but leaves abduco alive so Claude keeps working in the background. Only an explicit "kill" destroys abduco.

Browser connects via `GET /api/v1/projects/{id}/ws/{wid}` (WebSocket) → Go backend proxy (`internal/terminal/`) → `docker exec` with TTY mode attached to existing abduco session. Backend calls `create-terminal.sh` to initialize abduco for new worktrees.
