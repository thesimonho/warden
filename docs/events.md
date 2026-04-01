# Audit Event Source Catalog

MUST keep this up to date with the latest JSONL sources.

The goal is to eventually move entirely over to JSONL.

Each audit event has a source: JSONL parser (Go backend tails session files), hook (container shell script writes to event dir), or backend (Go service writes directly). Codex has no hooks — everything comes from JSONL or backend.

## Events from JSONL parser

| Event                | Claude source                      | Codex source                                                                               |
| -------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------ |
| `tool_use`           | `assistant` → tool_use blocks      | `response_item/function_call` + `event_msg/{exec_command,mcp_tool_call,patch_apply}_begin` |
| `tool_use_failure`   | `user` → tool_result with is_error | `response_item/function_call_output` (heuristic) + `event_msg/*_end` with error            |
| `stop` (cost)        | `assistant` → usage                | `event_msg/token_count`                                                                    |
| `user_prompt`        | `user` → text content              | `event_msg/user_message`                                                                   |
| `stop_failure`       | `system/api_error`                 | `event_msg/error` + `event_msg/stream_error`                                               |
| `turn_complete`      | `assistant` → stop_reason=end_turn | `event_msg/task_complete`                                                                  |
| `session_start`      | —                                  | `session_meta`                                                                             |
| `permission_request` | —                                  | `event_msg/exec_approval_request` + `event_msg/request_permissions`                        |
| `elicitation`        | —                                  | `event_msg/elicitation_request`                                                            |

## Events from hooks (Claude Code only)

These events are not in Claude's JSONL format. Codex either has them in JSONL (above) or doesn't support the concept.

### Attention state (real-time, not written to audit)

| Event             | Hook                         | Re-evaluate when...                    |
| ----------------- | ---------------------------- | -------------------------------------- |
| `attention`       | Notification                 | Claude adds notification data to JSONL |
| `needs_answer`    | PreToolUse (AskUserQuestion) | Claude adds attention state to JSONL   |
| `attention_clear` | UserPromptSubmit, PreToolUse | Claude adds attention state to JSONL   |

### Audit events

| Event                 | Hook               | Re-evaluate when...                     |
| --------------------- | ------------------ | --------------------------------------- |
| `session_start`       | SessionStart       | Claude adds session lifecycle to JSONL  |
| `session_end`         | SessionEnd         | Claude adds session lifecycle to JSONL  |
| `permission_request`  | PermissionRequest  | Claude adds permission events to JSONL  |
| `config_change`       | ConfigChange       | Claude adds config events to JSONL      |
| `instructions_loaded` | InstructionsLoaded | Claude adds instruction events to JSONL |
| `task_completed`      | TaskCompleted      | Claude adds task events to JSONL        |
| `elicitation`         | Elicitation        | Claude adds MCP events to JSONL         |
| `elicitation_result`  | ElicitationResult  | Claude adds MCP events to JSONL         |
| `subagent_start`      | SubagentStart      | Claude adds subagent lifecycle to JSONL |
| `subagent_stop`       | SubagentStop       | Claude adds subagent lifecycle to JSONL |

## Events from backend / container scripts

| Event                          | Source                  | Notes                           |
| ------------------------------ | ----------------------- | ------------------------------- |
| `terminal_connected`           | Container shell scripts | Not agent-specific              |
| `terminal_disconnected`        | Container shell scripts | Not agent-specific              |
| `process_killed`               | Container shell scripts | Not agent-specific              |
| `session_exit`                 | Container shell scripts | Not agent-specific              |
| `heartbeat`                    | Container shell scripts | Not agent-specific, not audited |
| `container_heartbeat_stale`    | Go backend              | Liveness checker                |
| `container_startup_failed`     | Go backend              | Startup health check            |
| `budget_exceeded`              | Go backend              | Cost enforcement                |
| `budget_worktrees_stopped`     | Go backend              | Cost enforcement                |
| `budget_container_stopped`     | Go backend              | Cost enforcement                |
| `budget_enforcement_failed`    | Go backend              | Cost enforcement                |
| `cost_reset`                   | Go backend              | Cost enforcement                |
| `project_removed`              | Go backend              | Lifecycle                       |
| `container_deleted`            | Go backend              | Lifecycle                       |
| `audit_purged`                 | Go backend              | Lifecycle                       |
| `restart_blocked_stale_mounts` | Go backend              | Stale mount detection           |
| `worktree_created`             | Go backend              | Worktree lifecycle              |
| `worktree_removed`             | Go backend              | Worktree lifecycle              |
| `worktree_cleaned_up`          | Go backend              | Worktree lifecycle              |
| `worktree_create_failed`       | Go backend              | Worktree lifecycle (error)      |
| `terminal_connect_failed`      | Go backend              | Terminal lifecycle (error)      |
| `terminal_disconnect_failed`   | Go backend              | Terminal lifecycle (error)      |
| `worktree_kill_failed`         | Go backend              | Worktree lifecycle (error)      |
| `worktree_remove_failed`       | Go backend              | Worktree lifecycle (error)      |
| `worktree_cleanup_failed`      | Go backend              | Worktree lifecycle (error)      |
