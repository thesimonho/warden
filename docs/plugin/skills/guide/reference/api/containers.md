<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Containers API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## Delete container

- **Method:** `DELETE`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/container`
- **Tags:** containers

Stops and removes the container for the given project. The container is permanently deleted.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`agentType`**

  `string` — AgentType is the agent type for this container.

- **`containerId`**

  `string` — ContainerID is the Docker container ID.

- **`name`**

  `string` — Name is the container name.

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier.

- **`recreated`**

  `boolean` — Recreated is true when the container was fully recreated (not just settings updated).

**Example:**

```json
{
  "agentType": "",
  "containerId": "",
  "name": "",
  "projectId": "",
  "recreated": true
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Create container

- **Method:** `POST`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/container`
- **Tags:** containers

Creates a new container for the given project with the provided configuration. Supports network isolation modes and custom bind mounts.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`agentType`**

  `string`, possible values: `"claude-code", "codex", "claude-code"` — AgentType identifies the CLI agent running in this project (e.g. "claude-code", "codex").

- **`allowedDomains`**

  `array` — AllowedDomains lists domains accessible when NetworkMode is "restricted".

  **Items:**

  `string`

- **`costBudget`**

  `number` — CostBudget is the per-project cost limit in USD (0 = use global default).

- **`enabledAccessItems`**

  `array` — EnabledAccessItems lists active access item IDs (e.g. \["git","ssh"]).

  **Items:**

  `string`

- **`enabledRuntimes`**

  `array` — EnabledRuntimes lists active runtime IDs (e.g. \["node","python","go"]).

  **Items:**

  `string`

- **`envVars`**

  `object`

- **`image`**

  `string`

- **`mounts`**

  `array` — Mounts is a list of additional bind mounts from host into the container.

  **Items:**

  - **`containerPath`**

    `string` — ContainerPath is the absolute path inside the container.

  - **`hostPath`**

    `string` — HostPath is the absolute path on the host.

  - **`readOnly`**

    `boolean` — ReadOnly mounts the path as read-only inside the container.

- **`name`**

  `string`

- **`networkMode`**

  `string`, possible values: `"full", "restricted", "none"` — NetworkMode controls the container's network isolation level.

- **`projectPath`**

  `string`

- **`skipPermissions`**

  `boolean` — SkipPermissions controls whether terminals skip permission prompts. Stored as a Docker label on the container.

**Example:**

```json
{}
```

#### Responses

##### Status: 201 Created

###### Content-Type: application/json

- **`agentType`**

  `string` — AgentType is the agent type for this container.

- **`containerId`**

  `string` — ContainerID is the Docker container ID.

- **`name`**

  `string` — Name is the container name.

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier.

- **`recreated`**

  `boolean` — Recreated is true when the container was fully recreated (not just settings updated).

**Example:**

```json
{
  "agentType": "",
  "containerId": "",
  "name": "",
  "projectId": "",
  "recreated": true
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 409 Container name already in use
##### Status: 500 Internal Server Error
---

## Update container

- **Method:** `PUT`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/container`
- **Tags:** containers

Updates the project's container configuration. Lightweight changes (budget, skip permissions, allowed domains) are applied in-place. Other changes (image, mounts, env vars, network mode, agent type) trigger a full container recreation.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`agentType`**

  `string`, possible values: `"claude-code", "codex", "claude-code"` — AgentType identifies the CLI agent running in this project (e.g. "claude-code", "codex").

- **`allowedDomains`**

  `array` — AllowedDomains lists domains accessible when NetworkMode is "restricted".

  **Items:**

  `string`

- **`costBudget`**

  `number` — CostBudget is the per-project cost limit in USD (0 = use global default).

- **`enabledAccessItems`**

  `array` — EnabledAccessItems lists active access item IDs (e.g. \["git","ssh"]).

  **Items:**

  `string`

- **`enabledRuntimes`**

  `array` — EnabledRuntimes lists active runtime IDs (e.g. \["node","python","go"]).

  **Items:**

  `string`

- **`envVars`**

  `object`

- **`image`**

  `string`

- **`mounts`**

  `array` — Mounts is a list of additional bind mounts from host into the container.

  **Items:**

  - **`containerPath`**

    `string` — ContainerPath is the absolute path inside the container.

  - **`hostPath`**

    `string` — HostPath is the absolute path on the host.

  - **`readOnly`**

    `boolean` — ReadOnly mounts the path as read-only inside the container.

- **`name`**

  `string`

- **`networkMode`**

  `string`, possible values: `"full", "restricted", "none"` — NetworkMode controls the container's network isolation level.

- **`projectPath`**

  `string`

- **`skipPermissions`**

  `boolean` — SkipPermissions controls whether terminals skip permission prompts. Stored as a Docker label on the container.

**Example:**

```json
{}
```

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`agentType`**

  `string` — AgentType is the agent type for this container.

- **`containerId`**

  `string` — ContainerID is the Docker container ID.

- **`name`**

  `string` — Name is the container name.

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier.

- **`recreated`**

  `boolean` — Recreated is true when the container was fully recreated (not just settings updated).

**Example:**

```json
{
  "agentType": "",
  "containerId": "",
  "name": "",
  "projectId": "",
  "recreated": true
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Get container config

- **Method:** `GET`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/container/config`
- **Tags:** containers

Returns the editable configuration of the project's container, including name, image, project path, bind mounts, environment variables, and network settings.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`agentType`**

  `string`, possible values: `"claude-code", "codex", "claude-code"` — AgentType identifies the CLI agent running in this project (e.g. "claude-code", "codex").

- **`allowedDomains`**

  `array` — AllowedDomains lists domains accessible when NetworkMode is "restricted".

  **Items:**

  `string`

- **`costBudget`**

  `number` — CostBudget is the per-project cost limit in USD (0 = use global default).

- **`enabledAccessItems`**

  `array` — EnabledAccessItems lists active access item IDs (e.g. \["git","ssh"]).

  **Items:**

  `string`

- **`enabledRuntimes`**

  `array` — EnabledRuntimes lists active runtime IDs (e.g. \["node","python","go"]).

  **Items:**

  `string`

- **`envVars`**

  `object`

- **`image`**

  `string`

- **`mounts`**

  `array`

  **Items:**

  - **`containerPath`**

    `string` — ContainerPath is the absolute path inside the container.

  - **`hostPath`**

    `string` — HostPath is the absolute path on the host.

  - **`readOnly`**

    `boolean` — ReadOnly mounts the path as read-only inside the container.

- **`name`**

  `string`

- **`networkMode`**

  `string`, possible values: `"full", "restricted", "none"` — NetworkMode controls the container's network isolation level.

- **`projectPath`**

  `string`

- **`skipPermissions`**

  `boolean`

**Example:**

```json
{
  "agentType": "claude-code",
  "allowedDomains": [
    ""
  ],
  "costBudget": 1,
  "enabledAccessItems": [
    ""
  ],
  "enabledRuntimes": [
    ""
  ],
  "envVars": {
    "additionalProperty": ""
  },
  "image": "",
  "mounts": [
    {
      "containerPath": "",
      "hostPath": "",
      "readOnly": true
    }
  ],
  "name": "",
  "networkMode": "full",
  "projectPath": "",
  "skipPermissions": true
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
---

## Validate container infrastructure

- **Method:** `GET`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/container/validate`
- **Tags:** containers

Checks whether the project's running container has the required Warden terminal infrastructure installed (tmux, create-terminal.sh).

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`missing`**

  `array` — Missing lists the infrastructure components that are not installed.

  **Items:**

  `string`

- **`valid`**

  `boolean` — Valid is true when all required infrastructure is present.

**Example:**

```json
{
  "missing": [
    ""
  ],
  "valid": true
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 500 Internal Server Error
