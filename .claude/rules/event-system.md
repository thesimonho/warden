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

Claude Code's hook events (Notification, PreToolUse, UserPromptSubmit) are pushed via `warden-event.sh` to a bind-mounted event directory (`WARDEN_EVENT_DIR`) → `eventbus/watcher.go` detects files and parses events → `eventbus/store.go` tracks attention state → SSE broadcasts to frontend. The watcher watches the directory using fsnotify (fast path) + polling every 2s (reliable fallback). Filesystem permissions handle access control (no bearer token needed). `UserPromptSubmit` fires two events: `attention_clear` (real-time state) and `user_prompt` (logged with prompt text, truncated to 500 chars).

## Hook data enrichment

`warden-event.sh` forwards additional data from Claude Code hooks:

- `session_start` includes `sessionId`, `model`, `source`
- `session_end` includes `reason`
- `pre_tool_use` sends both a `tool_use` audit event (with `toolName`, `toolInput` truncated to 1000 chars) and an attention state event
- `post_tool_use_failure` sends a `tool_use_failure` event (with `toolName`)
- `notification` maps to `attention` event type
- `user_prompt` includes `prompt` text
