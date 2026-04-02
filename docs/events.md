# Audit Event Source Catalog

MUST keep this up to date with the latest JSONL sources.

The goal is to eventually move entirely over to JSONL.

Each audit event has a source: JSONL parser (Go backend tails session files), hook (container shell script writes to event dir), or backend (Go service writes directly). Codex has no hooks — everything comes from JSONL or backend.

## Events from JSONL parser

| Event                | Claude source                                       | Codex source                                                                               | Audit category |
| -------------------- | --------------------------------------------------- | ------------------------------------------------------------------------------------------ | -------------- |
| `tool_use`           | `assistant` → tool_use blocks                       | `response_item/{function_call,local_shell_call,web_search_call,tool_search_call}`          | agent          |
| `tool_use_failure`   | `user` → tool_result with is_error                  | `response_item/function_call_output` (heuristic) + `event_msg/*_end` with error (extended) | agent          |
| `stop` (cost)        | `assistant` → usage                                 | `event_msg/token_count`                                                                    | session        |
| `user_prompt`        | `user` → text content, `queue-operation` → enqueue  | `event_msg/user_message`                                                                   | prompt         |
| `stop_failure`       | `system/api_error`                                  | `event_msg/error` (extended) + `event_msg/turn_aborted`                                    | session        |
| `turn_complete`      | `assistant` → stop_reason=end_turn                  | `event_msg/task_complete`                                                                  | session        |
| `turn_duration`      | `system/turn_duration`                              | —                                                                                          | session        |
| `session_start`      | —                                                   | `session_meta`                                                                             | session        |
| `permission_request` | —                                                   | — (app-server only)                                                                        | agent          |
| `elicitation`        | —                                                   | — (app-server only)                                                                        | agent          |
| `subagent_stop`      | `system/agents_killed`                              | —                                                                                          | agent          |
| `api_metrics`        | `system/api_metrics` (TTFT, OPS)                    | —                                                                                          | system         |
| `permission_grant`   | `system/permission_retry` (commands list)           | —                                                                                          | agent          |
| `context_compact`    | `system/compact_boundary` + `microcompact_boundary` | `event_msg/context_compacted` + `event_msg/thread_rolled_back` + `compacted`               | session        |
| `system_info`        | `system/{informational,memory_saved,...}`           | —                                                                                          | session        |

### Claude system subtypes parsed as `system_info`

These are informational system messages from Claude's JSONL. All parsed as `system_info` events in the session audit category.

- `informational` — General informational message
- `memory_saved` — Memory file write notification
- `away_summary` — Summary of activity while user was away
- `stop_hook_summary` — Hook execution summary
- `bridge_status` — Remote control bridge connection
- `local_command` — Slash command execution
- `scheduled_task_fire` — Scheduled task notification

## Events from hooks (Claude Code only)

These events are not in Claude's JSONL format yet. Codex either has them in JSONL (above) or doesn't support the concept.

### Attention state (real-time, not written to audit)

| Event             | Hook                         | Re-evaluate when...                    |
| ----------------- | ---------------------------- | -------------------------------------- |
| `attention`       | Notification                 | Claude adds notification data to JSONL |
| `needs_answer`    | PreToolUse (AskUserQuestion) | Claude adds attention state to JSONL   |
| `attention_clear` | UserPromptSubmit, PreToolUse | Claude adds attention state to JSONL   |

### Audit events

| Event                 | Hook               | Re-evaluate when...                                             |
| --------------------- | ------------------ | --------------------------------------------------------------- |
| `session_start`       | SessionStart       | Claude adds session lifecycle to JSONL                          |
| `session_end`         | SessionEnd         | Claude adds session lifecycle to JSONL                          |
| `permission_request`  | PermissionRequest  | Claude adds permission events to JSONL (grant is via JSONL now) |
| `config_change`       | ConfigChange       | Claude adds config events to JSONL                              |
| `instructions_loaded` | InstructionsLoaded | Claude adds instruction events to JSONL                         |
| `task_completed`      | TaskCompleted      | Claude adds task events to JSONL                                |
| `elicitation`         | Elicitation        | Claude adds MCP events to JSONL                                 |
| `elicitation_result`  | ElicitationResult  | Claude adds MCP events to JSONL                                 |
| `subagent_start`      | SubagentStart      | Claude adds subagent lifecycle to JSONL (stop is via JSONL now) |
| `subagent_stop`       | SubagentStop       | Claude adds subagent lifecycle to JSONL (stop is via JSONL now) |

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
