# Claude Code Hooks — Findings & Limitations

> **Context:** With the JSONL session file parser as the primary data source for agent events (session lifecycle, tool use, cost, prompts), Claude Code hooks are now a **supplementary channel** used only for attention/notification state. Only three hooks remain active: `Notification`, `PreToolUse` (for attention state only), and `UserPromptSubmit` (for attention clearing). All other data is parsed from the JSONL session file by the Go backend. Codex does not support hooks — attention tracking for Codex is a known upstream gap.

Documented during event bus validation testing on 2026-03-18.

## Hook Compatibility Matrix

| Hook Event         | Fires in Warden? | Notes                                                                                                                                                                                                                                                                                                  |
| ------------------ | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `SessionStart`     | Yes              | Fires on worktree creation. Includes `session_id`, `model`, `source`. **Warden no longer registers this hook** — `session_start` is synthesized from JSONL session ID changes for reliability.                                                                                                         |
| `SessionEnd`       | Yes              | Fires on `/exit`. **Caveat:** `cwd` is `/project` (worktree dir already removed), so `worktreeId` resolves to `"main"`. Worktree ID must be extracted from `transcript_path`.                                                                                                                          |
| `Stop`             | Yes              | Fires after every Claude response. Includes `last_assistant_message`. Ideal trigger for cost reads.                                                                                                                                                                                                    |
| `UserPromptSubmit` | Yes              | Fires before Claude processes a prompt. Includes full `prompt` text.                                                                                                                                                                                                                                   |
| `PreToolUse`       | Yes              | Fires before tool execution. Includes `tool_name` and `tool_input`.                                                                                                                                                                                                                                    |
| `PostToolUse`      | Yes              | Fires after tool execution. Includes full `tool_response`. Very verbose — not recommended for production logging.                                                                                                                                                                                      |
| `Notification`     | **No**           | Never fired in any test, even without skip-permissions. May require specific conditions not triggered in Warden's terminal architecture.                                                                                                                                                               |
| `WorktreeCreate`   | **No**           | Registering this hook **replaces** Claude Code's default `git worktree add`. Even with a correct reimplementation that outputs the worktree path, it fails silently inside tmux sessions. Claude hangs with no worktree created.                                                                       |
| `WorktreeRemove`   | **No**           | Never fired on `/exit` or any exit method (Ctrl-C, `/clear`, `/exit`). Likely only fires for subagent `isolation: "worktree"` sessions, not external `--worktree` launches. The docs say it fires "when you exit a `--worktree` session and choose to remove it" but no removal prompt was ever shown. |

## Managed Settings & User Settings Merge

Managed settings (`/etc/claude-code/managed-settings.json`) and user settings (`~/.claude/settings.json`) **merge correctly**. Both hooks fire for the same event — neither overrides the other. Confirmed by registering a user `Stop` hook alongside the managed `Stop` hook; both produced output.

## Settings.json Overwrite Risk

**Critical:** Claude Code performs a full read-modify-write on `settings.json`. When it saves any setting (e.g., `skipDangerousModePermissionPrompt`), it serializes its internal model to disk, dropping any keys it doesn't recognize. If `~/.claude` is bind-mounted read-write into the container, this **overwrites the host's settings.json**, destroying user hooks and preferences.

Mitigation options:

- Mount `~/.claude` as read-only (but Claude needs to write session data, auth tokens, etc.)
- Selectively mount only safe subdirectories
- Copy settings into the container at startup instead of mounting
- Mount `settings.json` specifically as read-only, let other files be read-write

## Detecting "Needs Input" States

Claude Code's attention states must be detected through different hooks:

| Attention State              | How to Detect                                                                                                                                                                                                        |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Needs permission**         | `Notification` hook with `notification_type: "permission_prompt"` (via `warden-event.sh` → event bus) — but `Notification` hook didn't fire in testing. Fallback: `PreToolUse` events can indicate pending tool use. |
| **Needs answer**             | `PreToolUse` where `tool_name == "AskUserQuestion"`                                                                                                                                                                  |
| **Idle / waiting for input** | `Stop` event fires when Claude finishes responding                                                                                                                                                                   |
| **Working**                  | `UserPromptSubmit` or `PreToolUse` clears idle state                                                                                                                                                                 |

## SessionEnd Worktree ID Extraction

When Claude exits a `--worktree` session, it removes the worktree directory before `SessionEnd` fires. The `cwd` falls back to `/project`, so the worktree ID resolves to `"main"`. To identify the original worktree:

```
transcript_path: "/home/warden/.claude/projects/-project--claude-worktrees-hooktest3/..."
                                                                  ^^^^^^^^^^
                                                           extract from here
```

Pattern: `/home/warden/.claude/projects/-project--claude-worktrees-{WORKTREE_ID}/`

## Event Frequency

In a typical session (3 prompts, 1 tool use, `/exit`):

| Event                | Count | Notes                              |
| -------------------- | ----- | ---------------------------------- |
| `session_start`      | 1     | Once at session start              |
| `user_prompt_submit` | 3-5   | Once per prompt + internal retries |
| `pre_tool_use`       | 4-5   | Once per tool call                 |
| `post_tool_use`      | 4-5   | Once per tool call                 |
| `stop`               | 3-5   | Once per Claude response           |
| `session_end`        | 1     | Once at session end                |

`PostToolUse` includes full tool responses (including file contents read by Claude), making it extremely verbose. For production event bus use, filter to only the fields needed.

## Data Available Per Hook

All hooks include common fields: `session_id`, `transcript_path`, `cwd`, `hook_event_name`.

| Hook               | Key Data                                                 |
| ------------------ | -------------------------------------------------------- |
| `SessionStart`     | `source` (startup/resume/clear/compact), `model`         |
| `Stop`             | `last_assistant_message`, `stop_hook_active`             |
| `SessionEnd`       | `reason` (prompt_input_exit, clear, logout, etc.)        |
| `UserPromptSubmit` | `prompt` (full user prompt text)                         |
| `PreToolUse`       | `tool_name`, `tool_input` (full parameters)              |
| `PostToolUse`      | `tool_name`, `tool_input`, `tool_response` (full output) |
