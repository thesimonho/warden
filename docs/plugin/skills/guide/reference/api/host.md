<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Host API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## Get defaults

- **Method:** `GET`
- **Path:** `/api/v1/defaults`
- **Tags:** host

Returns server-resolved default values for the create container form, including the host home directory and auto-detected bind mounts.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`containerHomeDir`**

  `string`

- **`envVars`**

  `array`

  **Items:**

  - **`key`**

    `string`

  - **`value`**

    `string`

- **`homeDir`**

  `string`

- **`mounts`**

  `array`

  **Items:**

  - **`agentType`**

    `string` — AgentType restricts this mount to a specific agent type. Empty means the mount applies to all agent types.

  - **`containerPath`**

    `string`

  - **`hostPath`**

    `string`

  - **`readOnly`**

    `boolean`

  - **`required`**

    `boolean` — Required marks this mount as mandatory for the agent to function. Clients must not allow users to remove or change the container path of required mounts.

- **`restrictedDomains`**

  `object`

- **`runtimes`**

  `array` — Runtimes lists available language runtimes with detection results.

  **Items:**

  - **`alwaysEnabled`**

    `boolean` — AlwaysEnabled means this runtime cannot be deselected.

  - **`description`**

    `string` — Description briefly explains what gets installed.

  - **`detected`**

    `boolean` — Detected is true when marker files were found in the project directory.

  - **`domains`**

    `array` — Domains lists network domains required for this runtime's package registry.

    **Items:**

    `string`

  - **`envVars`**

    `object` — EnvVars maps environment variable names to values set when enabled.

  - **`id`**

    `string` — ID is the unique identifier (e.g. "node", "python", "go").

  - **`label`**

    `string` — Label is the human-readable name (e.g. "Node.js", "Python").

- **`template`**

  `object` — Template holds project template values loaded from .warden.json, if present.

  - **`agents`**

    `object`

  - **`costBudget`**

    `number`

  - **`forwardedPorts`**

    `array`

    **Items:**

    `integer`

  - **`image`**

    `string`

  - **`networkMode`**

    `string`, possible values: `"full", "restricted", "none"` — NetworkMode controls the container's network isolation level.

  - **`runtimes`**

    `array`

    **Items:**

    `string`

  - **`skipPermissions`**

    `boolean`

**Example:**

```json
{
  "containerHomeDir": "",
  "envVars": [
    {
      "key": "",
      "value": ""
    }
  ],
  "homeDir": "",
  "mounts": [
    {
      "agentType": "",
      "containerPath": "",
      "hostPath": "",
      "readOnly": true,
      "required": true
    }
  ],
  "restrictedDomains": {
    "additionalProperty": [
      ""
    ]
  },
  "runtimes": [
    {
      "alwaysEnabled": true,
      "description": "",
      "detected": true,
      "domains": [
        ""
      ],
      "envVars": {
        "additionalProperty": ""
      },
      "id": "",
      "label": ""
    }
  ],
  "template": {
    "agents": {
      "additionalProperty": {
        "allowedDomains": [
          ""
        ]
      }
    },
    "costBudget": 1,
    "forwardedPorts": [
      1
    ],
    "image": "",
    "networkMode": "full",
    "runtimes": [
      ""
    ],
    "skipPermissions": true
  }
}
```


---

## List filesystem entries

- **Method:** `GET`
- **Path:** `/api/v1/filesystem/directories`
- **Tags:** host

Returns filesystem entries at the given path. By default only directories are returned. Pass mode=file to include files alongside directories.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

**Array of:**

- **`isDir`**

  `boolean`

- **`name`**

  `string`

- **`path`**

  `string`

**Example:**

```json
[
  {
    "isDir": true,
    "name": "",
    "path": ""
  }
]
```

##### Status: 400 Bad Request
##### Status: 500 Internal Server Error
---

## Open in editor

- **Method:** `POST`
- **Path:** `/api/v1/filesystem/editor`
- **Tags:** host

Opens the given host directory in the user's preferred code editor ($VISUAL, $EDITOR, or a well-known editor CLI).

#### Request Body

##### Content-Type: application/json

**One of:**

- **`path`**

  `string` — Path is the absolute host directory path to act on.

**Example:**

```json
{}
```

#### Responses

##### Status: 204 Editor launched

##### Status: 400 Bad Request
##### Status: 404 Path not found
##### Status: 422 No editor found
##### Status: 500 Internal Server Error
---

## Reveal in file manager

- **Method:** `POST`
- **Path:** `/api/v1/filesystem/reveal`
- **Tags:** host

Opens the given host directory in the system file manager (Finder, Nautilus, Explorer).

#### Request Body

##### Content-Type: application/json

**One of:**

- **`path`**

  `string` — Path is the absolute host directory path to act on.

**Example:**

```json
{}
```

#### Responses

##### Status: 204 Directory opened

##### Status: 400 Bad Request
##### Status: 404 Path not found
##### Status: 500 Internal Server Error
---

## List runtimes

- **Method:** `GET`
- **Path:** `/api/v1/runtimes`
- **Tags:** host

Detects and returns available container runtimes with their socket paths and API versions.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`available`**

  `boolean` — Available indicates whether the Docker API socket is reachable.

- **`bridgeIP`**

  `string` — BridgeIP is the host IP address reachable from containers via host.docker.internal. On Docker Desktop this is 127.0.0.1 (the VM's NAT forwards to host loopback). On native Docker it's the bridge network gateway (e.g. 172.17.0.1). Used as the listen address for socket bridge TCP proxies.

- **`isDesktop`**

  `boolean` — IsDesktop indicates whether the Docker runtime is Docker Desktop (as opposed to native Docker Engine, Colima, OrbStack, etc.). Detected via the OperatingSystem field from the Docker API info endpoint, which returns "Docker Desktop" for all Docker Desktop installations regardless of host OS.

- **`name`**

  `string` — Name is the runtime identifier ("docker").

- **`socketPath`**

  `string` — SocketPath is the filesystem path to the Docker API socket.

- **`version`**

  `string` — Version is Docker's reported API version, if available.

**Example:**

```json
{
  "available": true,
  "bridgeIP": "",
  "isDesktop": true,
  "name": "",
  "socketPath": "",
  "version": ""
}
```


---

## Read project template

- **Method:** `GET`
- **Path:** `/api/v1/template`
- **Tags:** host

Reads and parses a .warden.json file from the given path. Used to import templates from outside the project directory.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`agents`**

  `object`

- **`costBudget`**

  `number`

- **`forwardedPorts`**

  `array`

  **Items:**

  `integer`

- **`image`**

  `string`

- **`networkMode`**

  `string`, possible values: `"full", "restricted", "none"` — NetworkMode controls the container's network isolation level.

- **`runtimes`**

  `array`

  **Items:**

  `string`

- **`skipPermissions`**

  `boolean`

**Example:**

```json
{
  "agents": {
    "additionalProperty": {
      "allowedDomains": [
        ""
      ]
    }
  },
  "costBudget": 1,
  "forwardedPorts": [
    1
  ],
  "image": "",
  "networkMode": "full",
  "runtimes": [
    ""
  ],
  "skipPermissions": true
}
```

##### Status: 400 Bad Request
##### Status: 404 Not Found
---

## Validate project template

- **Method:** `POST`
- **Path:** `/api/v1/template`
- **Tags:** host

Accepts a raw .warden.json body, validates it against the ProjectTemplate schema, applies security sanitization, and returns the cleaned template. Used by the frontend import-from-file flow.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`agents`**

  `object`

- **`costBudget`**

  `number`

- **`forwardedPorts`**

  `array`

  **Items:**

  `integer`

- **`image`**

  `string`

- **`networkMode`**

  `string`, possible values: `"full", "restricted", "none"` — NetworkMode controls the container's network isolation level.

- **`runtimes`**

  `array`

  **Items:**

  `string`

- **`skipPermissions`**

  `boolean`

**Example:**

```json
{}
```

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`agents`**

  `object`

- **`costBudget`**

  `number`

- **`forwardedPorts`**

  `array`

  **Items:**

  `integer`

- **`image`**

  `string`

- **`networkMode`**

  `string`, possible values: `"full", "restricted", "none"` — NetworkMode controls the container's network isolation level.

- **`runtimes`**

  `array`

  **Items:**

  `string`

- **`skipPermissions`**

  `boolean`

**Example:**

```json
{
  "agents": {
    "additionalProperty": {
      "allowedDomains": [
        ""
      ]
    }
  },
  "costBudget": 1,
  "forwardedPorts": [
    1
  ],
  "image": "",
  "networkMode": "full",
  "runtimes": [
    ""
  ],
  "skipPermissions": true
}
```

##### Status: 400 Bad Request
## Schemas


