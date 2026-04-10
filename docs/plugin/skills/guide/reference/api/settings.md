<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Settings API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## Get settings

- **Method:** `GET`
- **Path:** `/api/v1/settings`
- **Tags:** settings

Returns the current server-side settings including runtime, audit log state, and disconnect key.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`auditLogMode`**

  `string`, possible values: `"off", "standard", "detailed"`

- **`budgetActionPreventStart`**

  `boolean`

- **`budgetActionStopContainer`**

  `boolean`

- **`budgetActionStopWorktrees`**

  `boolean`

- **`budgetActionWarn`**

  `boolean` — Budget enforcement actions — what happens when a project exceeds its budget.

- **`claudeCodeVersion`**

  `string` — Pinned CLI versions installed in containers.

- **`codexVersion`**

  `string`

- **`defaultProjectBudget`**

  `number`

- **`disconnectKey`**

  `string`

- **`notificationsEnabled`**

  `boolean` — NotificationsEnabled controls whether the system tray sends desktop notifications when agents need attention.

- **`runtime`**

  `string`

- **`version`**

  `string` — Version is the server build version (e.g. "v0.5.2", "dev").

- **`workingDirectory`**

  `string` — WorkingDirectory is the server process's working directory. Used by development tooling to auto-create projects without manual path entry.

**Example:**

```json
{
  "auditLogMode": "off",
  "budgetActionPreventStart": true,
  "budgetActionStopContainer": true,
  "budgetActionStopWorktrees": true,
  "budgetActionWarn": true,
  "claudeCodeVersion": "",
  "codexVersion": "",
  "defaultProjectBudget": 1,
  "disconnectKey": "",
  "notificationsEnabled": true,
  "runtime": "",
  "version": "",
  "workingDirectory": ""
}
```


---

## Update settings

- **Method:** `PUT`
- **Path:** `/api/v1/settings`
- **Tags:** settings

Updates server-side settings. Only provided fields are changed. Changing the runtime requires a server restart. Changing auditLogMode syncs the flag to all running containers.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`auditLogMode`**

  `string`, possible values: `"off", "standard", "detailed"`

- **`budgetActionPreventStart`**

  `boolean`

- **`budgetActionStopContainer`**

  `boolean`

- **`budgetActionStopWorktrees`**

  `boolean`

- **`budgetActionWarn`**

  `boolean`

- **`defaultProjectBudget`**

  `number`

- **`disconnectKey`**

  `string`

- **`notificationsEnabled`**

  `boolean`

**Example:**

```json
{}
```

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`restartRequired`**

  `boolean` — RestartRequired is true when a setting change requires a server restart.

**Example:**

```json
{
  "restartRequired": true
}
```

##### Status: 400 Bad Request
##### Status: 500 Internal Server Error
