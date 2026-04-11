# HTTP API

Run the `warden` binary as a headless server and make HTTP requests to `/api/v1/*`. This works from any language.

## Setup

```bash
# Start the server (default: localhost:8090)
./warden

# Or with a custom address
ADDR=0.0.0.0:9000 ./warden
```

All endpoints are under `/api/v1/`. See the [API Reference](https://thesimonho.github.io/warden/reference/api/) for full endpoint documentation.

## Example: List projects

### curl

```bash
curl http://localhost:8090/api/v1/projects
```

### TypeScript

```typescript
const response = await fetch("http://localhost:8090/api/v1/projects");
const projects = await response.json();
```

### Python

```python
import requests

response = requests.get("http://localhost:8090/api/v1/projects")
projects = response.json()
```

## Example: Create a project and container

Creating a project is a two-step flow: register the project, then create its container.

### curl

```bash
# Step 1: Register the project
curl -X POST http://localhost:8090/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"name": "my-project", "projectPath": "/home/user/project"}'

# Step 2: Create the container (using projectId from step 1)
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-project",
    "projectPath": "/home/user/project",
    "networkMode": "restricted",
    "allowedDomains": ["github.com", "npmjs.org"],
    "envVars": {"ANTHROPIC_API_KEY": "sk-ant-..."},
    "enabledRuntimes": ["node", "python"]
  }'
```

### TypeScript

```typescript
// Step 1: Register the project
const addResponse = await fetch("http://localhost:8090/api/v1/projects", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    name: "my-project",
    projectPath: "/home/user/project",
  }),
});
const { projectId } = await addResponse.json();

// Step 2: Create the container
const createResponse = await fetch(
  `http://localhost:8090/api/v1/projects/${projectId}/claude-code/container`,
  {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      name: "my-project",
      projectPath: "/home/user/project",
      networkMode: "restricted",
      allowedDomains: ["github.com", "npmjs.org"],
      envVars: { ANTHROPIC_API_KEY: "sk-ant-..." },
      enabledRuntimes: ["node", "python"],
    }),
  },
);
const container = await createResponse.json();
```

### Python

```python
import requests

BASE = "http://localhost:8090/api/v1"

# Step 1: Register the project
result = requests.post(f"{BASE}/projects", json={
    "name": "my-project",
    "projectPath": "/home/user/project",
}).json()
project_id = result["projectId"]

# Step 2: Create the container
container = requests.post(
    f"{BASE}/projects/{project_id}/claude-code/container",
    json={
        "name": "my-project",
        "projectPath": "/home/user/project",
        "networkMode": "restricted",
        "allowedDomains": ["github.com", "npmjs.org"],
        "envVars": {"ANTHROPIC_API_KEY": "sk-ant-..."},
        "enabledRuntimes": ["node", "python"],
    },
).json()
```

## Example: Real-time events (SSE)

Subscribe to `GET /api/v1/events` for real-time state updates across all projects.

### curl

```bash
curl -N http://localhost:8090/api/v1/events
```

### TypeScript

```typescript
const eventSource = new EventSource("http://localhost:8090/api/v1/events");

eventSource.addEventListener("worktree_state", (event) => {
  const data = JSON.parse(event.data);
  console.log(`Worktree ${data.worktreeId}: ${data.state}`);
});

eventSource.addEventListener("project_state", (event) => {
  const data = JSON.parse(event.data);
  console.log(
    `Project ${data.projectId}: cost=${data.totalCost}, needsInput=${data.needsInput}`,
  );
});
```

### Python

```python
import sseclient
import requests

response = requests.get("http://localhost:8090/api/v1/events", stream=True)
client = sseclient.SSEClient(response)

for event in client.events():
    print(f"{event.event}: {event.data}")
```

## Example: Terminal lifecycle

Terminal interaction is a two-step flow: connect (start the agent process), then attach via WebSocket.

### curl

```bash
PROJECT_ID="a1b2c3d4e5f6"
AGENT="claude-code"
WORKTREE="main"

# Connect terminal (starts tmux session + agent)
curl -X POST "http://localhost:8090/api/v1/projects/$PROJECT_ID/$AGENT/worktrees/$WORKTREE/connect"

# Attach via WebSocket (use a WebSocket client — binary frames for PTY I/O)
# wscat -c "ws://localhost:8090/api/v1/projects/$PROJECT_ID/$AGENT/ws/$WORKTREE"

# Or attach the auxiliary shell terminal for ad-hoc commands alongside the agent
# wscat -c "ws://localhost:8090/api/v1/projects/$PROJECT_ID/$AGENT/ws/$WORKTREE/shell"

# Disconnect (notify server the viewer closed)
curl -X POST "http://localhost:8090/api/v1/projects/$PROJECT_ID/$AGENT/worktrees/$WORKTREE/disconnect"

# Or kill (terminate the agent process)
curl -X POST "http://localhost:8090/api/v1/projects/$PROJECT_ID/$AGENT/worktrees/$WORKTREE/kill"
```

### TypeScript

```typescript
const BASE = "http://localhost:8090/api/v1";
const projectId = "a1b2c3d4e5f6";
const agent = "claude-code";
const worktree = "main";

// Connect terminal
await fetch(
  `${BASE}/projects/${projectId}/${agent}/worktrees/${worktree}/connect`,
  { method: "POST" },
);

// Attach via WebSocket
const ws = new WebSocket(
  `ws://localhost:8090/api/v1/projects/${projectId}/${agent}/ws/${worktree}`,
);

// Or attach the auxiliary shell terminal for ad-hoc commands alongside the agent
// const shell = new WebSocket(
//   `ws://localhost:8090/api/v1/projects/${projectId}/${agent}/ws/${worktree}/shell`,
// );

// Binary frames = PTY data, text frames = control messages
ws.onmessage = (event) => {
  if (event.data instanceof Blob) {
    // Terminal output
  } else {
    // Control message (e.g. resize acknowledgment)
  }
};

// Send resize
ws.send(JSON.stringify({ type: "resize", cols: 120, rows: 40 }));

// Send terminal input as binary
ws.send(new TextEncoder().encode("ls -la\n"));
```

## Error handling

All error responses include a machine-readable `code` field:

```json
{
  "error": "Project name already in use",
  "code": "NAME_TAKEN"
}
```

### TypeScript

```typescript
const response = await fetch("http://localhost:8090/api/v1/projects", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ name: "my-project", projectPath: "/path" }),
});

if (!response.ok) {
  const error = await response.json();
  console.error(`${error.code}: ${error.error}`);
}
```

### Python

```python
response = requests.post(
    "http://localhost:8090/api/v1/projects",
    json={"name": "my-project", "projectPath": "/path"},
)

if not response.ok:
    error = response.json()
    print(f"{error['code']}: {error['error']}")
```

See `reference/error-handling.md` for the full error code table.
