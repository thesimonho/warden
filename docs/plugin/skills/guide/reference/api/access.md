<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Access API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## List access items

- **Method:** `GET`
- **Path:** `/api/v1/access`
- **Tags:** access

Returns all access items (built-in + user-created) enriched with per-credential host detection status. Built-in items that have been customized via the DB are returned with the customized configuration.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`items`**

  `array`

  **Items:**

  - **`builtIn`**

    `boolean` — BuiltIn is true for items that ship with Warden.

  - **`credentials`**

    `array` — Credentials are the individual credential entries in this group.

    **Items:**

    - **`injections`**

      `array` — Injections are the container-side delivery targets.

      **Items:**

      - **`key`**

        `string` — Key is the env var name or container path for the injection target.

      - **`readOnly`**

        `boolean` — ReadOnly applies to mount injections — when true the mount is read-only.

      - **`type`**

        `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

      - **`value`**

        `string` — Value is a static override for the resolved value. When set, this is used instead of the source-resolved value. Useful when the injection needs a fixed container-side path (e.g. SSH\_AUTH\_SOCK env var pointing to the container socket path).

    - **`label`**

      `string` — Label is a human-readable name for this credential (e.g. "SSH Agent Socket").

    - **`sources`**

      `array` — Sources are tried in order; the first detected value is used.

      **Items:**

      - **`type`**

        `string`, possible values: `"env", "file", "socket", "command"` — Type is the kind of host source.

      - **`value`**

        `string` — Value is the env var name, file path, socket path, or command string.

    - **`transform`**

      `object` — Transform is an optional processing step applied to the resolved value.

      - **`params`**

        `object` — Params holds type-specific configuration (e.g. "pattern" for strip\_lines).

      - **`type`**

        `string`, possible values: `"strip_lines", "git_include"` — Type identifies the transformation.

  - **`description`**

    `string` — Description explains what this access item provides.

  - **`detection`**

    `object` — Detection holds per-credential availability on the current host.

    - **`available`**

      `boolean` — Available is true when at least one credential was detected.

    - **`credentials`**

      `array` — Credentials contains per-credential detection results.

      **Items:**

      - **`available`**

        `boolean` — Available is true when at least one source was detected.

      - **`label`**

        `string` — Label is the credential's human-readable name.

      - **`sourceMatched`**

        `string` — SourceMatched describes which source was detected (empty when unavailable).

    - **`id`**

      `string` — ID is the access item identifier.

    - **`label`**

      `string` — Label is the access item display name.

  - **`id`**

    `string` — ID is a stable identifier. Built-in items use well-known IDs (e.g. "git", "ssh"); user items get generated UUIDs.

  - **`label`**

    `string` — Label is the human-readable display name (e.g. "Git Config").

  - **`method`**

    `string`, possible values: `"transport"` — Method is the delivery strategy (only "transport" for now).

**Example:**

```json
{
  "items": [
    {
      "builtIn": true,
      "credentials": [
        {
          "injections": [
            {
              "key": "",
              "readOnly": true,
              "type": "env",
              "value": ""
            }
          ],
          "label": "",
          "sources": [
            {
              "type": "env",
              "value": ""
            }
          ],
          "transform": {
            "params": {
              "additionalProperty": ""
            },
            "type": "strip_lines"
          }
        }
      ],
      "description": "",
      "detection": {
        "available": true,
        "credentials": [
          {
            "available": true,
            "label": "",
            "sourceMatched": ""
          }
        ],
        "id": "",
        "label": ""
      },
      "id": "",
      "label": "",
      "method": "transport"
    }
  ]
}
```

##### Status: 500 Internal Server Error
---

## Create access item

- **Method:** `POST`
- **Path:** `/api/v1/access`
- **Tags:** access

Creates a new user-defined access item with the given label, description, and credential configuration. Returns the created item with a generated ID.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`credentials`**

  `array`

  **Items:**

  - **`injections`**

    `array` — Injections are the container-side delivery targets.

    **Items:**

    - **`key`**

      `string` — Key is the env var name or container path for the injection target.

    - **`readOnly`**

      `boolean` — ReadOnly applies to mount injections — when true the mount is read-only.

    - **`type`**

      `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

    - **`value`**

      `string` — Value is a static override for the resolved value. When set, this is used instead of the source-resolved value. Useful when the injection needs a fixed container-side path (e.g. SSH\_AUTH\_SOCK env var pointing to the container socket path).

  - **`label`**

    `string` — Label is a human-readable name for this credential (e.g. "SSH Agent Socket").

  - **`sources`**

    `array` — Sources are tried in order; the first detected value is used.

    **Items:**

    - **`type`**

      `string`, possible values: `"env", "file", "socket", "command"` — Type is the kind of host source.

    - **`value`**

      `string` — Value is the env var name, file path, socket path, or command string.

  - **`transform`**

    `object` — Transform is an optional processing step applied to the resolved value.

    - **`params`**

      `object` — Params holds type-specific configuration (e.g. "pattern" for strip\_lines).

    - **`type`**

      `string`, possible values: `"strip_lines", "git_include"` — Type identifies the transformation.

- **`description`**

  `string`

- **`label`**

  `string`

**Example:**

```json
{}
```

#### Responses

##### Status: 201 Created

###### Content-Type: application/json

- **`builtIn`**

  `boolean` — BuiltIn is true for items that ship with Warden.

- **`credentials`**

  `array` — Credentials are the individual credential entries in this group.

  **Items:**

  - **`injections`**

    `array` — Injections are the container-side delivery targets.

    **Items:**

    - **`key`**

      `string` — Key is the env var name or container path for the injection target.

    - **`readOnly`**

      `boolean` — ReadOnly applies to mount injections — when true the mount is read-only.

    - **`type`**

      `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

    - **`value`**

      `string` — Value is a static override for the resolved value. When set, this is used instead of the source-resolved value. Useful when the injection needs a fixed container-side path (e.g. SSH\_AUTH\_SOCK env var pointing to the container socket path).

  - **`label`**

    `string` — Label is a human-readable name for this credential (e.g. "SSH Agent Socket").

  - **`sources`**

    `array` — Sources are tried in order; the first detected value is used.

    **Items:**

    - **`type`**

      `string`, possible values: `"env", "file", "socket", "command"` — Type is the kind of host source.

    - **`value`**

      `string` — Value is the env var name, file path, socket path, or command string.

  - **`transform`**

    `object` — Transform is an optional processing step applied to the resolved value.

    - **`params`**

      `object` — Params holds type-specific configuration (e.g. "pattern" for strip\_lines).

    - **`type`**

      `string`, possible values: `"strip_lines", "git_include"` — Type identifies the transformation.

- **`description`**

  `string` — Description explains what this access item provides.

- **`id`**

  `string` — ID is a stable identifier. Built-in items use well-known IDs (e.g. "git", "ssh"); user items get generated UUIDs.

- **`label`**

  `string` — Label is the human-readable display name (e.g. "Git Config").

- **`method`**

  `string`, possible values: `"transport"` — Method is the delivery strategy (only "transport" for now).

**Example:**

```json
{
  "builtIn": true,
  "credentials": [
    {
      "injections": [
        {
          "key": "",
          "readOnly": true,
          "type": "env",
          "value": ""
        }
      ],
      "label": "",
      "sources": [
        {
          "type": "env",
          "value": ""
        }
      ],
      "transform": {
        "params": {
          "additionalProperty": ""
        },
        "type": "strip_lines"
      }
    }
  ],
  "description": "",
  "id": "",
  "label": "",
  "method": "transport"
}
```

##### Status: 400 Invalid input (missing label or credentials)
##### Status: 500 Internal Server Error
---

## Delete access item

- **Method:** `DELETE`
- **Path:** `/api/v1/access/{id}`
- **Tags:** access

Deletes a user-defined access item. Built-in items cannot be deleted — use the reset endpoint instead.

#### Responses

##### Status: 204 No content

##### Status: 400 Cannot delete built-in item
##### Status: 500 Internal Server Error
---

## Get access item

- **Method:** `GET`
- **Path:** `/api/v1/access/{id}`
- **Tags:** access

Returns a single access item with detection status. For built-in items, returns the DB override if one exists, otherwise the default configuration.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`builtIn`**

  `boolean` — BuiltIn is true for items that ship with Warden.

- **`credentials`**

  `array` — Credentials are the individual credential entries in this group.

  **Items:**

  - **`injections`**

    `array` — Injections are the container-side delivery targets.

    **Items:**

    - **`key`**

      `string` — Key is the env var name or container path for the injection target.

    - **`readOnly`**

      `boolean` — ReadOnly applies to mount injections — when true the mount is read-only.

    - **`type`**

      `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

    - **`value`**

      `string` — Value is a static override for the resolved value. When set, this is used instead of the source-resolved value. Useful when the injection needs a fixed container-side path (e.g. SSH\_AUTH\_SOCK env var pointing to the container socket path).

  - **`label`**

    `string` — Label is a human-readable name for this credential (e.g. "SSH Agent Socket").

  - **`sources`**

    `array` — Sources are tried in order; the first detected value is used.

    **Items:**

    - **`type`**

      `string`, possible values: `"env", "file", "socket", "command"` — Type is the kind of host source.

    - **`value`**

      `string` — Value is the env var name, file path, socket path, or command string.

  - **`transform`**

    `object` — Transform is an optional processing step applied to the resolved value.

    - **`params`**

      `object` — Params holds type-specific configuration (e.g. "pattern" for strip\_lines).

    - **`type`**

      `string`, possible values: `"strip_lines", "git_include"` — Type identifies the transformation.

- **`description`**

  `string` — Description explains what this access item provides.

- **`detection`**

  `object` — Detection holds per-credential availability on the current host.

  - **`available`**

    `boolean` — Available is true when at least one credential was detected.

  - **`credentials`**

    `array` — Credentials contains per-credential detection results.

    **Items:**

    - **`available`**

      `boolean` — Available is true when at least one source was detected.

    - **`label`**

      `string` — Label is the credential's human-readable name.

    - **`sourceMatched`**

      `string` — SourceMatched describes which source was detected (empty when unavailable).

  - **`id`**

    `string` — ID is the access item identifier.

  - **`label`**

    `string` — Label is the access item display name.

- **`id`**

  `string` — ID is a stable identifier. Built-in items use well-known IDs (e.g. "git", "ssh"); user items get generated UUIDs.

- **`label`**

  `string` — Label is the human-readable display name (e.g. "Git Config").

- **`method`**

  `string`, possible values: `"transport"` — Method is the delivery strategy (only "transport" for now).

**Example:**

```json
{
  "builtIn": true,
  "credentials": [
    {
      "injections": [
        {
          "key": "",
          "readOnly": true,
          "type": "env",
          "value": ""
        }
      ],
      "label": "",
      "sources": [
        {
          "type": "env",
          "value": ""
        }
      ],
      "transform": {
        "params": {
          "additionalProperty": ""
        },
        "type": "strip_lines"
      }
    }
  ],
  "description": "",
  "detection": {
    "available": true,
    "credentials": [
      {
        "available": true,
        "label": "",
        "sourceMatched": ""
      }
    ],
    "id": "",
    "label": ""
  },
  "id": "",
  "label": "",
  "method": "transport"
}
```

##### Status: 404 Item not found
##### Status: 500 Internal Server Error
---

## Update access item

- **Method:** `PUT`
- **Path:** `/api/v1/access/{id}`
- **Tags:** access

Updates an access item. For built-in items, saves a customized copy to the DB (overriding the default). For user items, updates the existing DB row. Only provided fields are changed.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`credentials`**

  `array`

  **Items:**

  - **`injections`**

    `array` — Injections are the container-side delivery targets.

    **Items:**

    - **`key`**

      `string` — Key is the env var name or container path for the injection target.

    - **`readOnly`**

      `boolean` — ReadOnly applies to mount injections — when true the mount is read-only.

    - **`type`**

      `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

    - **`value`**

      `string` — Value is a static override for the resolved value. When set, this is used instead of the source-resolved value. Useful when the injection needs a fixed container-side path (e.g. SSH\_AUTH\_SOCK env var pointing to the container socket path).

  - **`label`**

    `string` — Label is a human-readable name for this credential (e.g. "SSH Agent Socket").

  - **`sources`**

    `array` — Sources are tried in order; the first detected value is used.

    **Items:**

    - **`type`**

      `string`, possible values: `"env", "file", "socket", "command"` — Type is the kind of host source.

    - **`value`**

      `string` — Value is the env var name, file path, socket path, or command string.

  - **`transform`**

    `object` — Transform is an optional processing step applied to the resolved value.

    - **`params`**

      `object` — Params holds type-specific configuration (e.g. "pattern" for strip\_lines).

    - **`type`**

      `string`, possible values: `"strip_lines", "git_include"` — Type identifies the transformation.

- **`description`**

  `string`

- **`label`**

  `string`

**Example:**

```json
{}
```

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`builtIn`**

  `boolean` — BuiltIn is true for items that ship with Warden.

- **`credentials`**

  `array` — Credentials are the individual credential entries in this group.

  **Items:**

  - **`injections`**

    `array` — Injections are the container-side delivery targets.

    **Items:**

    - **`key`**

      `string` — Key is the env var name or container path for the injection target.

    - **`readOnly`**

      `boolean` — ReadOnly applies to mount injections — when true the mount is read-only.

    - **`type`**

      `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

    - **`value`**

      `string` — Value is a static override for the resolved value. When set, this is used instead of the source-resolved value. Useful when the injection needs a fixed container-side path (e.g. SSH\_AUTH\_SOCK env var pointing to the container socket path).

  - **`label`**

    `string` — Label is a human-readable name for this credential (e.g. "SSH Agent Socket").

  - **`sources`**

    `array` — Sources are tried in order; the first detected value is used.

    **Items:**

    - **`type`**

      `string`, possible values: `"env", "file", "socket", "command"` — Type is the kind of host source.

    - **`value`**

      `string` — Value is the env var name, file path, socket path, or command string.

  - **`transform`**

    `object` — Transform is an optional processing step applied to the resolved value.

    - **`params`**

      `object` — Params holds type-specific configuration (e.g. "pattern" for strip\_lines).

    - **`type`**

      `string`, possible values: `"strip_lines", "git_include"` — Type identifies the transformation.

- **`description`**

  `string` — Description explains what this access item provides.

- **`id`**

  `string` — ID is a stable identifier. Built-in items use well-known IDs (e.g. "git", "ssh"); user items get generated UUIDs.

- **`label`**

  `string` — Label is the human-readable display name (e.g. "Git Config").

- **`method`**

  `string`, possible values: `"transport"` — Method is the delivery strategy (only "transport" for now).

**Example:**

```json
{
  "builtIn": true,
  "credentials": [
    {
      "injections": [
        {
          "key": "",
          "readOnly": true,
          "type": "env",
          "value": ""
        }
      ],
      "label": "",
      "sources": [
        {
          "type": "env",
          "value": ""
        }
      ],
      "transform": {
        "params": {
          "additionalProperty": ""
        },
        "type": "strip_lines"
      }
    }
  ],
  "description": "",
  "id": "",
  "label": "",
  "method": "transport"
}
```

##### Status: 400 Invalid input
##### Status: 404 Item not found
##### Status: 500 Internal Server Error
---

## Reset access item

- **Method:** `POST`
- **Path:** `/api/v1/access/{id}/reset`
- **Tags:** access

Restores a built-in access item to its default configuration by removing any DB override. Only works for built-in items (git, ssh).

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`builtIn`**

  `boolean` — BuiltIn is true for items that ship with Warden.

- **`credentials`**

  `array` — Credentials are the individual credential entries in this group.

  **Items:**

  - **`injections`**

    `array` — Injections are the container-side delivery targets.

    **Items:**

    - **`key`**

      `string` — Key is the env var name or container path for the injection target.

    - **`readOnly`**

      `boolean` — ReadOnly applies to mount injections — when true the mount is read-only.

    - **`type`**

      `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

    - **`value`**

      `string` — Value is a static override for the resolved value. When set, this is used instead of the source-resolved value. Useful when the injection needs a fixed container-side path (e.g. SSH\_AUTH\_SOCK env var pointing to the container socket path).

  - **`label`**

    `string` — Label is a human-readable name for this credential (e.g. "SSH Agent Socket").

  - **`sources`**

    `array` — Sources are tried in order; the first detected value is used.

    **Items:**

    - **`type`**

      `string`, possible values: `"env", "file", "socket", "command"` — Type is the kind of host source.

    - **`value`**

      `string` — Value is the env var name, file path, socket path, or command string.

  - **`transform`**

    `object` — Transform is an optional processing step applied to the resolved value.

    - **`params`**

      `object` — Params holds type-specific configuration (e.g. "pattern" for strip\_lines).

    - **`type`**

      `string`, possible values: `"strip_lines", "git_include"` — Type identifies the transformation.

- **`description`**

  `string` — Description explains what this access item provides.

- **`id`**

  `string` — ID is a stable identifier. Built-in items use well-known IDs (e.g. "git", "ssh"); user items get generated UUIDs.

- **`label`**

  `string` — Label is the human-readable display name (e.g. "Git Config").

- **`method`**

  `string`, possible values: `"transport"` — Method is the delivery strategy (only "transport" for now).

**Example:**

```json
{
  "builtIn": true,
  "credentials": [
    {
      "injections": [
        {
          "key": "",
          "readOnly": true,
          "type": "env",
          "value": ""
        }
      ],
      "label": "",
      "sources": [
        {
          "type": "env",
          "value": ""
        }
      ],
      "transform": {
        "params": {
          "additionalProperty": ""
        },
        "type": "strip_lines"
      }
    }
  ],
  "description": "",
  "id": "",
  "label": "",
  "method": "transport"
}
```

##### Status: 400 Not a built-in item
##### Status: 500 Internal Server Error
---

## Resolve access items

- **Method:** `POST`
- **Path:** `/api/v1/access/resolve`
- **Tags:** access

Resolves the given access items by checking host sources and computing the injections (env vars, mounts) that would be applied to containers. Used by the UI "Test" button to preview resolution without creating a container.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`items`**

  `array`

  **Items:**

  - **`builtIn`**

    `boolean` — BuiltIn is true for items that ship with Warden.

  - **`credentials`**

    `array` — Credentials are the individual credential entries in this group.

    **Items:**

    - **`injections`**

      `array` — Injections are the container-side delivery targets.

      **Items:**

      - **`key`**

        `string` — Key is the env var name or container path for the injection target.

      - **`readOnly`**

        `boolean` — ReadOnly applies to mount injections — when true the mount is read-only.

      - **`type`**

        `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

      - **`value`**

        `string` — Value is a static override for the resolved value. When set, this is used instead of the source-resolved value. Useful when the injection needs a fixed container-side path (e.g. SSH\_AUTH\_SOCK env var pointing to the container socket path).

    - **`label`**

      `string` — Label is a human-readable name for this credential (e.g. "SSH Agent Socket").

    - **`sources`**

      `array` — Sources are tried in order; the first detected value is used.

      **Items:**

      - **`type`**

        `string`, possible values: `"env", "file", "socket", "command"` — Type is the kind of host source.

      - **`value`**

        `string` — Value is the env var name, file path, socket path, or command string.

    - **`transform`**

      `object` — Transform is an optional processing step applied to the resolved value.

      - **`params`**

        `object` — Params holds type-specific configuration (e.g. "pattern" for strip\_lines).

      - **`type`**

        `string`, possible values: `"strip_lines", "git_include"` — Type identifies the transformation.

  - **`description`**

    `string` — Description explains what this access item provides.

  - **`id`**

    `string` — ID is a stable identifier. Built-in items use well-known IDs (e.g. "git", "ssh"); user items get generated UUIDs.

  - **`label`**

    `string` — Label is the human-readable display name (e.g. "Git Config").

  - **`method`**

    `string`, possible values: `"transport"` — Method is the delivery strategy (only "transport" for now).

**Example:**

```json
{}
```

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`items`**

  `array`

  **Items:**

  - **`credentials`**

    `array` — Credentials contains per-credential resolution results.

    **Items:**

    - **`error`**

      `string` — Error is set when resolution failed (distinct from "not detected").

    - **`injections`**

      `array` — Injections are the resolved container-side deliveries.

      **Items:**

      - **`key`**

        `string` — Key is the env var name or container path.

      - **`readOnly`**

        `boolean` — ReadOnly applies to mount injections.

      - **`type`**

        `string`, possible values: `"env", "mount_file", "mount_socket"` — Type is the injection kind (env, mount\_file, mount\_socket).

      - **`value`**

        `string` — Value is the resolved content (env var value, host file path, or host socket path).

    - **`label`**

      `string` — Label is the credential's human-readable name.

    - **`resolved`**

      `boolean` — Resolved is true when a source was detected and all injections were produced.

    - **`sourceMatched`**

      `string` — SourceMatched describes which source was matched (empty when unresolved).

  - **`id`**

    `string` — ID is the access item identifier.

  - **`label`**

    `string` — Label is the access item display name.

**Example:**

```json
{
  "items": [
    {
      "credentials": [
        {
          "error": "",
          "injections": [
            {
              "key": "",
              "readOnly": true,
              "type": "env",
              "value": ""
            }
          ],
          "label": "",
          "resolved": true,
          "sourceMatched": ""
        }
      ],
      "id": "",
      "label": ""
    }
  ]
}
```

##### Status: 400 Invalid request body
##### Status: 404 Item not found
##### Status: 500 Internal Server Error
