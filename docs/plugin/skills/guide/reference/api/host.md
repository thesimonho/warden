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

## Reveal in file manager

- **Method:** `POST`
- **Path:** `/api/v1/filesystem/reveal`
- **Tags:** host

Opens the given host directory in the system file manager (Finder, Nautilus, Explorer).

#### Request Body

##### Content-Type: application/json

**One of:**

- **`path`**

  `string` — Path is the absolute host directory path to open.

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


