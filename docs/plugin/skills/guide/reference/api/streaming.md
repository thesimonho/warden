<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Streaming API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## Subscribe to events (SSE)

- **Method:** `GET`
- **Path:** `/api/v1/events`
- **Tags:** streaming

Opens a Server-Sent Events stream for real-time updates. Optionally filter by projectId and agentType to receive events for a single project. Event types: worktree\_state, project\_state, worktree\_list\_changed, budget\_exceeded, budget\_container\_stopped, heartbeat, server\_shutdown, runtime\_status, agent\_status.

#### Responses

##### Status: 200 SSE event stream

###### Content-Type: application/json

`string`

**Example:**

```json
""
```

##### Status: 500 Internal Server Error
##### Status: 503 Event streaming not configured
---

## Terminal WebSocket

- **Method:** `GET`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/ws/{wid}`
- **Tags:** streaming

Upgrades to a WebSocket connection and bridges it to a tmux terminal session inside the container via docker exec. Binary frames carry raw PTY data; text frames carry JSON control messages (e.g. {"type":"resize","cols":80,"rows":24}).

#### Responses

##### Status: 101 WebSocket upgrade

##### Status: 400 Bad Request
##### Status: 404 Not Found
##### Status: 503 Terminal proxy not configured
