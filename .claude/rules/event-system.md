---
paths:
  - "event/**/*"
  - "eventbus/**/*"
  - "agent/**/*"
  - "watcher/hook/**/*"
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

Agent hook events are pushed via agent-specific scripts (`warden-event-claude.sh`, `warden-event-codex.sh`) to a bind-mounted event directory (`WARDEN_EVENT_DIR`) → `watcher/hook/watcher.go` detects files and parses events → `eventbus/store.go` tracks attention state → SSE broadcasts to frontend → audit log write. The watcher watches the directory using fsnotify (fast path) + polling every 2s (reliable fallback). Filesystem permissions handle access control (no bearer token needed). Claude Code's `UserPromptSubmit` fires two events: `attention_clear` (internal state — not written to audit log) and `user_prompt` (logged with prompt text, truncated to 500 chars). ContainerEvent includes `agentType` field for scoping events to a specific agent type per directory. Event types are defined in the `event/` leaf package; both `agent/` and `eventbus/` import from it.

In `HandleEvent()`, SSE broadcast runs before audit DB write — both are independent operations outside the store lock, but broadcasting first minimizes frontend notification latency.

Every attention state change emits both a `worktree_state` SSE event (per-worktree) and a `project_state` SSE event (aggregated across all worktrees, with the highest-priority notification type). This keeps project cards and desktop notifications (via the system tray) in sync without the frontend needing to aggregate.

Attention state is set from two sources: real-time hook events (`attention`, `needs_answer`) AND JSONL-parsed `turn_complete` events. The `turn_complete` handler sets `NeedsInput=true` with `idle_prompt` notification type when the agent finishes a turn during an active session, supplementing the hook path which may not fire in all cases (e.g. after `--continue` resume). Stale `turn_complete` events (older than the current attention state) are ignored to prevent race conditions between the hook and JSONL channels.

`SessionActive` is set via JSONL-synthesized `session_start` events for both agent types: Codex emits one from `session_meta` entries, and Claude Code emits one when the parser detects a new session ID. The `session_start` hook was removed from Claude Code to avoid cross-channel coordination bugs — the JSONL path is reliable and fires before any `turn_complete` that depends on it.

## Hook data enrichment

`warden-event-claude.sh` forwards additional data from Claude Code hooks:

- `session_end` includes `reason`
- `pre_tool_use` sends both a `tool_use` audit event (with `toolName`, `toolInput` truncated to 1000 chars) and an attention state event
- `post_tool_use_failure` sends a `tool_use_failure` event (with `toolName`)
- `notification` maps to `attention` event type
- `user_prompt` includes `prompt` text
