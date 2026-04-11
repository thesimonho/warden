<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Focus API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## Get focus state

- **Method:** `GET`
- **Path:** `/api/v1/focus`
- **Tags:** focus

Returns the aggregated viewer focus state across all connected clients.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`activeViewers`**

  `integer` — ActiveViewers is the total number of focused client instances.

- **`focusedWorktrees`**

  `object` — FocusedWorktrees maps "projectId:agentType" keys to deduplicated worktree ID lists.

**Example:**

```json
{
  "activeViewers": 1,
  "focusedWorktrees": {
    "additionalProperty": [
      ""
    ]
  }
}
```


---

## Report focus state

- **Method:** `POST`
- **Path:** `/api/v1/focus`
- **Tags:** focus

Reports which project and worktrees a client is actively viewing. Used by the system tray to suppress desktop notifications for focused projects.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`agentType`**

  `string` — AgentType scopes the focus to a specific agent type.

- **`clientId`**

  `string` — ClientID is a unique identifier for this client instance (UUID per page load or TUI session).

- **`focused`**

  `boolean` — Focused is true when the client has visibility focus on Warden.

- **`projectId`**

  `string` — ProjectID is the project being viewed (empty when unfocused or on a non-project page).

- **`worktreeIds`**

  `array` — WorktreeIDs lists all worktrees currently open in the client's view.

  **Items:**

  `string`

**Example:**

```json
{}
```

#### Responses

##### Status: 204 No Content

##### Status: 400 Bad Request
