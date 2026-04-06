# Events

Warden broadcasts real-time state changes to connected clients via **Server-Sent Events (SSE)**. A single persistent connection to `GET /api/v1/events` delivers every event type -- there are no polling endpoints. Connect once, receive all updates for every project and worktree managed by the server.

## SSE vs polling

SSE is the primary mechanism for receiving real-time updates. The server sends a `heartbeat` event every 15 seconds as a keepalive so clients and proxies know the connection is alive.

However, SSE does not eliminate polling entirely. Some state is best obtained by periodic polling of `GET /api/v1/projects`:

- **Container state changes** (stop, crash, external restart) may not always produce SSE events immediately
- **Cost data captures** are asynchronous — cost totals on project objects may update between SSE events
- **Codex attention state** is unavailable via SSE (Codex does not support hooks — see below)
- **Container recreation** causes a brief gap in SSE events while the session watcher restarts

A reasonable pattern: connect SSE for real-time attention/worktree updates, poll projects every 10-30 seconds for container state and cost totals.

On reconnect, there is no event replay — SSE connections start fresh. After reconnecting, poll `GET /api/v1/projects` once to get the current state of all projects, then let the SSE stream handle subsequent updates.

## Connecting

Open a standard SSE connection to the events endpoint:

```
GET /api/v1/events
Accept: text/event-stream
```

Each SSE message has an `event` field (the event type) and a `data` field (JSON payload). The server responds with `503 Service Unavailable` if the event system is not configured.

## Event types

### `worktree_state`

Sent when a worktree's terminal or attention state changes. This is the most frequent event -- it fires on every state transition (agent starts, agent needs input, terminal connects/disconnects, agent exits).

**Payload fields:**

| Field              | Type    | Description                                                                                                     |
| ------------------ | ------- | --------------------------------------------------------------------------------------------------------------- |
| `projectId`        | string  | The 12-char hex project identifier.                                                                             |
| `agentType`        | string  | `"claude-code"` or `"codex"`.                                                                                   |
| `containerName`    | string  | Docker container name.                                                                                          |
| `worktreeId`       | string  | The worktree identifier (e.g. `"main"`, `"feature-x"`).                                                        |
| `state`            | string  | Worktree state: `"connected"`, `"shell"`, `"background"`, or `"stopped"`.                                       |
| `needsInput`       | boolean | `true` when the agent is waiting for user attention.                                                            |
| `notificationType` | string  | Why attention is needed: `"permission_prompt"`, `"idle_prompt"`, `"elicitation_dialog"`, or `"auth_success"`.    |
| `sessionActive`    | boolean | `true` when an agent process is running in this worktree.                                                       |
| `exitCode`         | number  | The agent's exit code (only meaningful when `state` is `"shell"`).                                              |

### `project_state`

Sent when project-level aggregated state changes. Carries cost and the highest-priority attention state across all worktrees. Use this for dashboard cards or project list views where you need a single status per project.

**Payload fields:**

| Field              | Type    | Description                                                                                                  |
| ------------------ | ------- | ------------------------------------------------------------------------------------------------------------ |
| `projectId`        | string  | The 12-char hex project identifier.                                                                          |
| `agentType`        | string  | `"claude-code"` or `"codex"`.                                                                                |
| `containerName`    | string  | Docker container name.                                                                                       |
| `totalCost`        | number  | Cumulative cost in USD across all worktrees.                                                                 |
| `messageCount`     | number  | Total messages across all worktrees.                                                                         |
| `needsInput`       | boolean | `true` if any worktree in this project needs attention.                                                      |
| `notificationType` | string  | Highest-priority notification type across all worktrees (same values as `worktree_state`).                   |

### `worktree_list_changed`

Sent when a worktree is created, removed, or cleaned up. Clients should refresh their worktree list for the affected project.

**Payload fields:**

| Field           | Type   | Description                        |
| --------------- | ------ | ---------------------------------- |
| `projectId`     | string | The 12-char hex project identifier.|
| `agentType`     | string | `"claude-code"` or `"codex"`.      |
| `containerName` | string | Docker container name.             |

### `budget_exceeded`

Sent when a project's cumulative cost crosses its configured budget threshold.

**Payload fields:**

| Field           | Type   | Description                            |
| --------------- | ------ | -------------------------------------- |
| `projectId`     | string | The 12-char hex project identifier.    |
| `agentType`     | string | `"claude-code"` or `"codex"`.          |
| `containerName` | string | Docker container name.                 |
| `totalCost`     | number | Current cumulative cost in USD.        |
| `budget`        | number | The configured budget limit in USD.    |

### `budget_container_stopped`

Sent after a container is stopped due to budget enforcement. Frontends should redirect users away from the now-stopped project.

**Payload fields:**

Same as `budget_exceeded`, plus:

| Field         | Type   | Description                                        |
| ------------- | ------ | -------------------------------------------------- |
| `containerId` | string | Docker container ID of the stopped container.      |

### `runtime_status`

Sent when a language runtime installation starts or completes inside a container.

**Payload fields:**

| Field           | Type   | Description                                            |
| --------------- | ------ | ------------------------------------------------------ |
| `projectId`     | string | The 12-char hex project identifier.                    |
| `agentType`     | string | `"claude-code"` or `"codex"`.                          |
| `containerName` | string | Docker container name.                                 |
| `phase`         | string | Installation phase (e.g. installing, installed).       |
| `runtimeId`     | string | Runtime identifier (e.g. `"python"`, `"go"`).          |
| `runtimeLabel`  | string | Human-readable runtime name (e.g. `"Python"`, `"Go"`).|

### `agent_status`

Sent when an agent CLI installation or update starts or completes inside a container.

**Payload fields:**

| Field           | Type   | Description                                        |
| --------------- | ------ | -------------------------------------------------- |
| `projectId`     | string | The 12-char hex project identifier.                |
| `agentType`     | string | `"claude-code"` or `"codex"`.                      |
| `containerName` | string | Docker container name.                             |
| `phase`         | string | Installation phase (e.g. installing, installed).   |
| `version`       | string | Agent CLI version string.                          |

### `heartbeat`

Keepalive sent every 15 seconds. No meaningful payload -- use it to detect connection liveness.

### `server_shutdown`

Sent immediately before the Warden server stops. Frontends should show a "Warden stopped" state and prepare for reconnection.

## Data sources

Warden derives events from two data sources inside containers:

1. **JSONL session files** (primary) -- Both Claude Code and Codex write append-only JSONL files. Warden tails these files and parses cost updates, tool usage, turn completions, and attention state. This is the universal path that works for all agent types.

2. **Claude Code hooks** (supplementary) -- Claude Code supports lifecycle hooks (`Notification`, `PreToolUse`, `UserPromptSubmit`, etc.) that push real-time attention state. These provide lower-latency attention tracking than JSONL parsing alone. Codex does not support hooks, so attention tracking for Codex relies entirely on JSONL parsing.

## Reconnection strategy

The SSE protocol handles reconnection natively -- most client libraries (`EventSource` in browsers, `sseclient` in Python) reconnect automatically when the connection drops.

After reconnecting:

1. The SSE stream starts fresh (no event replay).
2. Poll `GET /api/v1/projects` to get current state for all projects.
3. For each project, poll `GET /api/v1/projects/{projectId}/{agentType}/worktrees` if you need per-worktree state.
4. Let subsequent SSE events handle incremental updates.

If you receive a `server_shutdown` event, the server is going away intentionally. Implement a backoff-and-retry loop until the server comes back.

## Examples

### curl

```bash
curl -N -H "Accept: text/event-stream" http://localhost:8090/api/v1/events
```

Each event arrives as:

```
event: worktree_state
data: {"projectId":"a1b2c3d4e5f6","agentType":"claude-code","containerName":"my-project","worktreeId":"main","state":"connected","needsInput":true,"notificationType":"permission_prompt","sessionActive":true,"exitCode":0}

event: project_state
data: {"projectId":"a1b2c3d4e5f6","agentType":"claude-code","containerName":"my-project","totalCost":0.42,"messageCount":12,"needsInput":true,"notificationType":"permission_prompt"}

event: heartbeat
data: {}
```

### TypeScript (EventSource)

```typescript
const events = new EventSource("http://localhost:8090/api/v1/events");

events.addEventListener("worktree_state", (e) => {
  const data = JSON.parse(e.data);
  console.log(`Worktree ${data.worktreeId}: state=${data.state}, needsInput=${data.needsInput}`);
});

events.addEventListener("project_state", (e) => {
  const data = JSON.parse(e.data);
  console.log(`Project ${data.projectId}: cost=$${data.totalCost}, messages=${data.messageCount}`);
});

events.addEventListener("budget_exceeded", (e) => {
  const data = JSON.parse(e.data);
  console.log(`Budget exceeded for ${data.projectId}: $${data.totalCost} / $${data.budget}`);
});

events.addEventListener("server_shutdown", () => {
  console.log("Warden server shutting down, will reconnect automatically");
});
```

### Python (sseclient)

```python
import json
import requests
import sseclient

response = requests.get(
    "http://localhost:8090/api/v1/events",
    headers={"Accept": "text/event-stream"},
    stream=True,
)
client = sseclient.SSEClient(response)

for event in client.events():
    data = json.loads(event.data) if event.data else {}

    if event.event == "worktree_state":
        print(f"Worktree {data['worktreeId']}: state={data['state']}")
    elif event.event == "project_state":
        print(f"Project {data['projectId']}: cost=${data['totalCost']:.2f}")
    elif event.event == "budget_exceeded":
        print(f"Budget exceeded: ${data['totalCost']:.2f} / ${data['budget']:.2f}")
    elif event.event == "server_shutdown":
        print("Server shutting down")
        break
```
