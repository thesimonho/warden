---
paths:
  - "eventbus/**/*"
  - "container/scripts/**/*"
  - "service/**/*"
  - "internal/terminal/**/*"
  - "db/**/*"
  - "web/src/**/event*"
  - "web/src/**/sse*"
  - "web/src/**/attention*"
---

# Event System

## Attention tracking

Agent hook events are pushed via agent-specific scripts (`warden-event-claude.sh`, `warden-event-codex.sh`) to a bind-mounted event directory (`WARDEN_EVENT_DIR`) → `eventbus/watcher.go` detects files and parses events → `eventbus/store.go` tracks attention state → SSE broadcasts to frontend → audit log write. The watcher watches the directory using fsnotify (fast path) + polling every 2s (reliable fallback). Filesystem permissions handle access control (no bearer token needed). Claude Code's `UserPromptSubmit` fires two events: `attention_clear` (internal state — not written to audit log) and `user_prompt` (logged with prompt text, truncated to 500 chars). ContainerEvent includes `agentType` field for scoping events to a specific agent type per directory.

The `Stop` hook is backgrounded in `warden-event-claude.sh` so it doesn't block Claude Code from firing the subsequent `Notification` hook. In `HandleEvent()`, SSE broadcast runs before audit DB write — both are independent operations outside the store lock, but broadcasting first minimizes frontend notification latency.

Every attention state change emits both a `worktree_state` SSE event (per-worktree) and a `project_state` SSE event (aggregated across all worktrees, with the highest-priority notification type). This keeps project cards and browser notifications in sync without the frontend needing to aggregate.

## Hook data enrichment

`warden-event-claude.sh` forwards additional data from Claude Code hooks:

- `session_start` includes `sessionId`, `model`, `source`
- `session_end` includes `reason`
- `pre_tool_use` sends both a `tool_use` audit event (with `toolName`, `toolInput` truncated to 1000 chars) and an attention state event
- `post_tool_use_failure` sends a `tool_use_failure` event (with `toolName`)
- `notification` maps to `attention` event type
- `user_prompt` includes `prompt` text
