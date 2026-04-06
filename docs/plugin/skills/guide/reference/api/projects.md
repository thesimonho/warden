<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Projects API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## List projects

- **Method:** `GET`
- **Path:** `/api/v1/projects`
- **Tags:** projects

Returns all configured projects enriched with live container state, Claude status, worktree counts, and cost data.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

**Array of:**

- **`activeWorktreeCount`**

  `integer` — ActiveWorktreeCount is the number of worktrees with connected terminals.

- **`agentStatus`**

  `string`, possible values: `"idle", "working", "unknown"`

- **`agentType`**

  `string`, possible values: `"claude-code", "codex", "claude-code"` — AgentType identifies the CLI agent running in this project (e.g. "claude-code", "codex").

- **`agentVersion`**

  `string` — AgentVersion is the pinned CLI version installed in this container.

- **`allowedDomains`**

  `array` — AllowedDomains lists domains accessible when NetworkMode is "restricted".

  **Items:**

  `string`

- **`costBudget`**

  `number` — CostBudget is the per-project cost limit in USD (0 = use global default).

- **`createdAt`**

  `integer`

- **`hasContainer`**

  `boolean` — HasContainer is true when a Docker container is associated with this project.

- **`hostPath`**

  `string` — HostPath is the absolute host directory mounted into the container.

- **`id`**

  `string` — ID is the Docker container ID (empty when no container exists).

- **`image`**

  `string`

- **`isEstimatedCost`**

  `boolean` — IsEstimatedCost is true when the cost is an estimate (e.g. subscription users). When false, the cost reflects actual API spend.

- **`isGitRepo`**

  `boolean` — IsGitRepo indicates whether the container's /project is a git repository.

- **`mountedDir`**

  `string` — MountedDir is the host directory mounted into the container.

- **`name`**

  `string` — Name is the user-chosen display label / Docker container name.

- **`needsInput`**

  `boolean` — NeedsInput is true when any worktree requires user attention.

- **`networkMode`**

  `string`, possible values: `"full", "restricted", "none"` — NetworkMode controls the container's network isolation level.

- **`notificationType`**

  `string`, possible values: `"permission_prompt", "idle_prompt", "auth_success", "elicitation_dialog"` — NotificationType indicates why Claude needs attention.

- **`os`**

  `string`

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier (sha256 of host path, 12 hex chars).

- **`skipPermissions`**

  `boolean` — SkipPermissions indicates whether terminals should skip permission prompts.

- **`sshPort`**

  `string`

- **`state`**

  `string`

- **`status`**

  `string`

- **`totalCost`**

  `number` — TotalCost is the aggregate cost across all worktrees in USD (from agent status provider).

- **`type`**

  `string`

- **`workspaceDir`**

  `string` — WorkspaceDir is the container-side workspace directory (mount destination).

**Example:**

```json
[
  {
    "activeWorktreeCount": 1,
    "agentStatus": "idle",
    "agentType": "claude-code",
    "agentVersion": "",
    "allowedDomains": [
      ""
    ],
    "costBudget": 1,
    "createdAt": 1,
    "hasContainer": true,
    "hostPath": "",
    "id": "",
    "image": "",
    "isEstimatedCost": true,
    "isGitRepo": true,
    "mountedDir": "",
    "name": "",
    "needsInput": true,
    "networkMode": "full",
    "notificationType": "permission_prompt",
    "os": "",
    "projectId": "",
    "skipPermissions": true,
    "sshPort": "",
    "state": "",
    "status": "",
    "totalCost": 1,
    "type": "",
    "workspaceDir": ""
  }
]
```

##### Status: 500 Internal Server Error
---

## Add project

- **Method:** `POST`
- **Path:** `/api/v1/projects`
- **Tags:** projects

Registers a host directory as a Warden project.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`name`**

  `string` — Name is an optional container name override.

- **`projectPath`**

  `string` — ProjectPath is the absolute host directory to register as a project.

**Example:**

```json
{}
```

#### Responses

##### Status: 201 Created

###### Content-Type: application/json

- **`agentType`**

  `string` — AgentType is the agent type for this project (e.g. "claude-code", "codex").

- **`containerId`**

  `string` — ContainerID is the Docker container ID, when available.

- **`name`**

  `string` — Name is the user-chosen project display name.

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier.

**Example:**

```json
{
  "agentType": "",
  "containerId": "",
  "name": "",
  "projectId": ""
}
```

##### Status: 400 Bad Request
##### Status: 500 Internal Server Error
---

## Remove project

- **Method:** `DELETE`
- **Path:** `/api/v1/projects/{projectId}/{agentType}`
- **Tags:** projects

Removes a project by its ID. Does not stop or delete the container.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`agentType`**

  `string` — AgentType is the agent type for this project (e.g. "claude-code", "codex").

- **`containerId`**

  `string` — ContainerID is the Docker container ID, when available.

- **`name`**

  `string` — Name is the user-chosen project display name.

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier.

**Example:**

```json
{
  "agentType": "",
  "containerId": "",
  "name": "",
  "projectId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Purge project audit

- **Method:** `DELETE`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/audit`
- **Tags:** projects

Removes all audit events for the given project.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

**Example:**

```json
{
  "additionalProperty": 1
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Upload clipboard image

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/clipboard`
- **Tags:** clipboard

Stages an image file in the container's clipboard directory. The xclip shim serves it when the agent reads the clipboard. Used by the web frontend for image paste support.

#### Request Body

##### Content-Type: application/x-www-form-urlencoded

`null`

**Example:**

```json
null
```

##### Content-Type: multipart/form-data

**Example:**

```json
{}
```

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`path`**

  `string` — Path is the absolute path of the staged file inside the container.

**Example:**

```json
{
  "path": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 413 Request Entity Too Large
##### Status: 500 Internal Server Error
---

## Reset project costs

- **Method:** `DELETE`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/costs`
- **Tags:** projects

Removes all cost history for the given project.

#### Responses

##### Status: 204 No Content

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Restart project

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/restart`
- **Tags:** projects

Restarts the container for the given project. Fails with STALE\_MOUNTS if bind mounts reference missing host paths.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`agentType`**

  `string` — AgentType is the agent type for this project (e.g. "claude-code", "codex").

- **`containerId`**

  `string` — ContainerID is the Docker container ID, when available.

- **`name`**

  `string` — Name is the user-chosen project display name.

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier.

**Example:**

```json
{
  "agentType": "",
  "containerId": "",
  "name": "",
  "projectId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 409 Stale mounts prevent restart
##### Status: 500 Internal Server Error
---

## Stop project

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/stop`
- **Tags:** projects

Gracefully stops the container for the given project.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`agentType`**

  `string` — AgentType is the agent type for this project (e.g. "claude-code", "codex").

- **`containerId`**

  `string` — ContainerID is the Docker container ID, when available.

- **`name`**

  `string` — Name is the user-chosen project display name.

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier.

**Example:**

```json
{
  "agentType": "",
  "containerId": "",
  "name": "",
  "projectId": ""
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
