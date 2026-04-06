<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Worktrees API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## List worktrees

- **Method:** `GET`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees`
- **Tags:** worktrees

Returns all worktrees for the given project, including terminal connection state, Claude attention status, and git branch information.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

**Array of:**

- **`branch`**

  `string` — Branch is the git branch checked out in this worktree.

- **`exitCode`**

  `integer` — ExitCode is the agent's exit code when in shell state. Nil means the agent is still running (or no exit code captured).

- **`id`**

  `string` — ID is the worktree identifier — directory name for git worktrees, "main" for project root.

- **`needsInput`**

  `boolean` — NeedsInput is true when Claude is blocked waiting for user attention.

- **`notificationType`**

  `string`, possible values: `"permission_prompt", "idle_prompt", "auth_success", "elicitation_dialog"` — NotificationType indicates why Claude needs attention.

- **`path`**

  `string` — Path is the filesystem path inside the container.

- **`projectId`**

  `string` — ProjectID is the container ID this worktree belongs to.

- **`state`**

  `string`, possible values: `"connected", "shell", "background", "stopped"` — State is the terminal connection state (connected, shell, background, stopped).

**Example:**

```json
[
  {
    "branch": "",
    "exitCode": 1,
    "id": "",
    "needsInput": true,
    "notificationType": "permission_prompt",
    "path": "",
    "projectId": "",
    "state": "connected"
  }
]
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Create worktree

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees`
- **Tags:** worktrees

Creates a new git worktree inside the container and automatically connects a terminal.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`name`**

  `string` — Name is the worktree name (must be a valid git branch name).

**Example:**

```json
{}
```

#### Responses

##### Status: 201 Created

###### Content-Type: application/json

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier the worktree belongs to.

- **`state`**

  `string` — State is the worktree's terminal state after the mutation ("connected", "shell", "background", "stopped"). Best-effort — may be empty if the state could not be determined (e.g. container not running).

- **`worktreeId`**

  `string` — WorktreeID is the worktree identifier.

**Example:**

```json
{
  "projectId": "",
  "state": "",
  "worktreeId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Remove worktree

- **Method:** `DELETE`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}`
- **Tags:** worktrees

Fully removes a worktree: kills any running processes, runs `git worktree remove`, and cleans up tracking state. Cannot remove the "main" worktree.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier the worktree belongs to.

- **`state`**

  `string` — State is the worktree's terminal state after the mutation ("connected", "shell", "background", "stopped"). Best-effort — may be empty if the state could not be determined (e.g. container not running).

- **`worktreeId`**

  `string` — WorktreeID is the worktree identifier.

**Example:**

```json
{
  "projectId": "",
  "state": "",
  "worktreeId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Connect terminal

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/connect`
- **Tags:** worktrees

Starts a tmux terminal session for the given worktree. If a background session already exists, reconnects to it instead of creating a new one.

#### Responses

##### Status: 201 Created

###### Content-Type: application/json

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier the worktree belongs to.

- **`state`**

  `string` — State is the worktree's terminal state after the mutation ("connected", "shell", "background", "stopped"). Best-effort — may be empty if the state could not be determined (e.g. container not running).

- **`worktreeId`**

  `string` — WorktreeID is the worktree identifier.

**Example:**

```json
{
  "projectId": "",
  "state": "",
  "worktreeId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Get worktree diff

- **Method:** `GET`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/diff`
- **Tags:** worktrees

Returns uncommitted changes (tracked and untracked files) for a worktree as a unified diff with per-file statistics.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`files`**

  `array` — Files lists per-file change statistics.

  **Items:**

  - **`additions`**

    `integer` — Additions is the number of lines added.

  - **`deletions`**

    `integer` — Deletions is the number of lines removed.

  - **`isBinary`**

    `boolean` — IsBinary is true when the file is a binary file.

  - **`oldPath`**

    `string` — OldPath is the previous path for renamed files.

  - **`path`**

    `string` — Path is the file path relative to the worktree root.

  - **`status`**

    `string` — Status is the change type: "added", "modified", "deleted", or "renamed".

- **`rawDiff`**

  `string` — RawDiff is the unified diff output from git.

- **`totalAdditions`**

  `integer` — TotalAdditions is the sum of additions across all files.

- **`totalDeletions`**

  `integer` — TotalDeletions is the sum of deletions across all files.

- **`truncated`**

  `boolean` — Truncated is true when the raw diff exceeded the size limit and was capped.

**Example:**

```json
{
  "files": [
    {
      "additions": 1,
      "deletions": 1,
      "isBinary": true,
      "oldPath": "",
      "path": "",
      "status": ""
    }
  ],
  "rawDiff": "",
  "totalAdditions": 1,
  "totalDeletions": 1,
  "truncated": true
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Disconnect terminal

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/disconnect`
- **Tags:** worktrees

Closes the terminal viewer WebSocket. The tmux session (and Claude/bash) continues running in the background.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier the worktree belongs to.

- **`state`**

  `string` — State is the worktree's terminal state after the mutation ("connected", "shell", "background", "stopped"). Best-effort — may be empty if the state could not be determined (e.g. container not running).

- **`worktreeId`**

  `string` — WorktreeID is the worktree identifier.

**Example:**

```json
{
  "projectId": "",
  "state": "",
  "worktreeId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Kill worktree process

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/kill`
- **Tags:** worktrees

Kills the tmux session and all child processes for the worktree. The git worktree directory on disk is preserved. This is destructive — any running Claude session is terminated immediately.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier the worktree belongs to.

- **`state`**

  `string` — State is the worktree's terminal state after the mutation ("connected", "shell", "background", "stopped"). Best-effort — may be empty if the state could not be determined (e.g. container not running).

- **`worktreeId`**

  `string` — WorktreeID is the worktree identifier.

**Example:**

```json
{
  "projectId": "",
  "state": "",
  "worktreeId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Reset worktree

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/reset`
- **Tags:** worktrees

Clears session state for a worktree: kills any running process, deletes agent session files, and removes terminal tracking state. Audit events are preserved. The worktree itself is preserved.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier the worktree belongs to.

- **`state`**

  `string` — State is the worktree's terminal state after the mutation ("connected", "shell", "background", "stopped"). Best-effort — may be empty if the state could not be determined (e.g. container not running).

- **`worktreeId`**

  `string` — WorktreeID is the worktree identifier.

**Example:**

```json
{
  "projectId": "",
  "state": "",
  "worktreeId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Cleanup orphaned worktrees

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/worktrees/cleanup`
- **Tags:** worktrees

Removes worktree directories that are not tracked by git, kills orphaned tmux sessions, and prunes stale git worktree metadata.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`removed`**

  `array` — Removed is the list of orphaned worktree IDs that were cleaned up.

  **Items:**

  `string`

**Example:**

```json
{
  "removed": [
    ""
  ]
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
