# Worktrees

A **worktree** is an isolated working directory within a project, backed by `git worktree`. Each worktree gets its own branch and directory, letting multiple agents work on different tasks in the same repository simultaneously. For non-git projects, a single implicit "main" worktree represents the project root.

Worktrees are the unit of independent work in Warden. Every terminal connects to exactly one worktree. Every cost entry, audit event, and attention notification is scoped to a worktree.

## Key concepts

### Worktree storage

Where worktree directories live on disk depends on the agent type:

| Agent        | Path                               | Who manages it                          |
| ------------ | ---------------------------------- | --------------------------------------- |
| Claude Code  | `.claude/worktrees/{worktree-id}/` | Claude Code (via `--worktree`)          |
| Codex        | `.warden/worktrees/{worktree-id}/` | Warden (via `git worktree add/remove`)  |

Claude Code manages its own worktrees internally. Codex worktrees are created and removed by Warden. From the API consumer's perspective, both behave identically: you create, list, reset, and remove worktrees through the same endpoints.

The "main" worktree always exists and represents the project root directory. It cannot be removed.

### Worktree states

Every worktree is in exactly one of four states:

| State          | tmux running | WebSocket connected | Description                              |
| -------------- | ------------ | ------------------- | ---------------------------------------- |
| `connected`    | Yes          | Yes                 | Agent is running, terminal viewer active |
| `shell`        | Yes          | Yes                 | Agent exited, user sees a bash prompt    |
| `background`   | Yes          | No                  | Agent is running, no viewer attached     |
| `stopped`      | No           | No                  | Nothing running                          |

State transitions:

```
                    +-----------+
          connect   |           |   agent exits
     +------------->| connected |-------------+
     |              |           |              |
     |              +-----+-----+              v
     |                    |               +---------+
     |          disconnect|               |  shell  |
     |                    v               +---------+
     |            +------------+               |
     |            | background |               | disconnect
     |            +------+-----+               |      or kill
     |                   |                     v
     |             kill  |              +-----------+
     |                   +------------->|  stopped  |
     |                                  +-----------+
     |                                        |
     +----------------------------------------+
                       connect
```

Key transitions:

- **Disconnect** (close viewer): `connected` becomes `background`. The agent keeps running.
- **Agent exits**: `connected` becomes `shell`. The user sees a bash prompt and can resume.
- **Kill**: any running state becomes `stopped`. The tmux session is destroyed.
- **Connect**: `stopped` starts a fresh session. `background` reattaches the viewer.

### Agent activity sub-states

When a worktree is in the `connected` state, Warden tracks what the agent is doing via the `needsInput` and `notificationType` fields. These come from Claude Code hook events. Codex does not yet support hooks, so activity sub-states are not available for Codex.

| Activity          | `needsInput` | `notificationType`    | Meaning                                |
| ----------------- | ------------ | --------------------- | -------------------------------------- |
| Working           | `false`      | (absent)              | Agent is generating or executing tools |
| Idle              | `false`      | `idle_prompt`         | Agent is running but not working       |
| Need Permission   | `true`       | `permission_prompt`   | Agent needs tool approval              |
| Need Answer       | `true`       | `elicitation_dialog`  | Agent is asking a question             |
| Need Input        | `true`       | (absent)              | Agent is done, waiting for next prompt |

These fields are included in the list worktrees response and broadcast via SSE `worktree_state` events, so you can show attention indicators without opening each terminal.

## API patterns

All worktree endpoints are scoped to a project and agent type:

```
/api/v1/projects/{projectId}/{agentType}/worktrees
```

Replace `{projectId}` with the 12-character hex project ID and `{agentType}` with `claude-code` or `codex`.

### List worktrees

Returns all worktrees for a project, including state, branch, and attention status.

```bash
curl http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees
```

Response:

```json
[
  {
    "id": "main",
    "state": "connected",
    "branch": "main",
    "path": "/home/warden/workspace",
    "projectId": "a1b2c3d4e5f6",
    "needsInput": true,
    "notificationType": "permission_prompt"
  },
  {
    "id": "fix-auth-bug",
    "state": "background",
    "branch": "fix-auth-bug",
    "path": "/home/warden/workspace/.claude/worktrees/fix-auth-bug",
    "projectId": "a1b2c3d4e5f6",
    "needsInput": false
  }
]
```

The `exitCode` field appears when the agent has exited (state is `shell` or `stopped`). A value of `137` means the process was killed (e.g., via the stop button or container restart).

### Create a worktree

Creates a new git worktree with its own branch and automatically connects a terminal. The name must be a valid git branch name.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees \
  -H "Content-Type: application/json" \
  -d '{"name": "fix-auth-bug"}'
```

Response (201 Created):

```json
{
  "projectId": "a1b2c3d4e5f6",
  "worktreeId": "fix-auth-bug"
}
```

The worktree is immediately in the `connected` state with the agent running. You can attach a WebSocket viewer right away.

### Remove a worktree

Kills any running processes, runs `git worktree remove`, and cleans up all tracking state including terminal state and exit codes. The worktree directory is deleted from disk.

```bash
curl -X DELETE http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/fix-auth-bug
```

Response:

```json
{
  "projectId": "a1b2c3d4e5f6",
  "worktreeId": "fix-auth-bug"
}
```

You cannot remove the "main" worktree. Attempting to do so returns a `400 Bad Request`.

### Reset a worktree

Kills the running process, deletes agent session files, and removes terminal tracking state. The worktree directory and git branch are preserved. Audit events are preserved.

The critical difference from remove: reset clears conversation history so the next connect starts fresh (no auto-resume). Remove deletes the worktree from disk entirely.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/fix-auth-bug/reset
```

Response:

```json
{
  "projectId": "a1b2c3d4e5f6",
  "worktreeId": "fix-auth-bug"
}
```

### Kill worktree process

Stops the tmux session and all child processes. The worktree directory, session files, and exit code are preserved. Because the exit code is preserved, the next connect will auto-resume the agent's conversation.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/fix-auth-bug/kill
```

Response:

```json
{
  "projectId": "a1b2c3d4e5f6",
  "worktreeId": "fix-auth-bug"
}
```

### Reset vs Kill vs Remove

These three destructive actions differ in what they preserve:

| Action   | Process killed | Session files cleared | Exit code cleared | Worktree deleted | Auto-resume on next connect |
| -------- | -------------- | --------------------- | ----------------- | ---------------- | --------------------------- |
| **Kill** | Yes            | No                    | No                | No               | Yes                         |
| **Reset**| Yes            | Yes                   | Yes               | No               | No (starts fresh)           |
| **Remove**| Yes           | Yes                   | Yes               | Yes              | N/A (worktree gone)         |

Use **kill** when you want to stop the agent but resume the conversation later. Use **reset** when you want a clean slate in the same worktree. Use **remove** when the worktree is no longer needed.

### Cleanup orphaned worktrees

Over time, worktrees can become orphaned if the git branch or tracking data is removed outside of Warden. Cleanup scans for and removes these orphaned worktrees, along with stale terminal tracking directories.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/cleanup
```

Response:

```json
{
  "removed": ["stale-branch-1", "old-experiment"]
}
```

The `removed` array lists the worktree IDs that were cleaned up. An empty array means no orphans were found.

### Get worktree diff

Returns uncommitted changes (tracked and untracked files) in a worktree as a unified diff with per-file statistics. Useful for reviewing what the agent has done before committing.

```bash
curl http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/fix-auth-bug/diff
```

Response:

```json
{
  "rawDiff": "diff --git a/auth.go b/auth.go\nindex abc123..def456 100644\n--- a/auth.go\n+++ b/auth.go\n@@ -10,3 +10,5 @@\n...",
  "files": [
    {
      "path": "auth.go",
      "status": "modified",
      "additions": 12,
      "deletions": 3,
      "isBinary": false
    }
  ],
  "totalAdditions": 12,
  "totalDeletions": 3,
  "truncated": false
}
```

The `truncated` field is `true` when the raw diff exceeded the size limit and was capped. Individual file statistics are still accurate even when the raw diff is truncated.

## Edge cases and important behaviors

- **Creating a worktree in a non-git project** returns an error. Only the implicit "main" worktree is available for non-git projects.
- **Creating a worktree with a duplicate name** returns a `400 Bad Request`. Worktree names must be unique within a project.
- **The "main" worktree** always exists and cannot be removed or renamed. Reset is allowed.
- **State is eventually consistent.** After a connect or kill, the list endpoint reflects the new state immediately. However, agent activity sub-states (`needsInput`, `notificationType`) are updated asynchronously via hook events and may take a moment to reflect.
- **SSE events** for worktree state changes are broadcast as `worktree_state` events on the `/api/v1/events` stream. Subscribe to this stream to get real-time updates instead of polling the list endpoint.
