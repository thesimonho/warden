---
title: "Go Client"
description: "Use the typed Go client to talk to a running Warden server."
editUrl: false
---
<!-- Generated from docs/plugin/skills/guide/reference/examples/client.md — do not edit directly -->

The [`client`](https://github.com/thesimonho/warden/tree/main/client) package is a convenience wrapper around the [HTTP API](./api.md). It provides typed Go functions for every endpoint so you don't have to manage HTTP requests, response parsing, or error handling yourself.

You still need to run the `warden` binary as a server — the client talks to it over HTTP.

## Setup

```go
import "github.com/thesimonho/warden/client"

c := client.New("http://localhost:8090")
```

## Example: List projects

```go
projects, err := c.ListProjects(ctx)
if err != nil {
    return err
}
for _, proj := range projects {
    fmt.Printf("%s (%s): %s\n", proj.Name, proj.AgentType, proj.State)
}
```

Each project includes an `AgentType` field (`"claude-code"` or `"codex"`) that identifies which CLI agent it runs.

## Example: Create a container

```go
resp, err := c.CreateContainer(ctx, projectID, "claude-code", api.CreateContainerRequest{
    ProjectPath:     "/home/user/projects/my-app",
    ProjectName:     "my-app",
    NetworkMode:     api.NetworkModeFull,
    EnabledRuntimes: []string{"node", "python", "go"},
})
```

The `EnabledRuntimes` field specifies which language runtimes to install in the container. Available runtimes are auto-detected from the project and provide network domains and environment variables.

## Example: Create a worktree

```go
resp, err := c.CreateWorktree(ctx, projectID, "claude-code", "feature-branch")
if err != nil {
    return err
}
fmt.Printf("Created worktree: %s\n", resp.WorktreeID)
```

The `agentType` parameter identifies which agent the project runs (`"claude-code"` or `"codex"`). When creating a worktree for an existing project, use the same agent type as the project.

## Example: Manage a project

The client exposes the same four management actions as the web and TUI dashboards:

```go
// Reset all cost tracking data for a project.
err := c.ResetProjectCosts(ctx, projectID, agentType)

// Purge all audit events for a project.
err := c.PurgeProjectAudit(ctx, projectID, agentType)

// Delete a project's container (stop + remove).
_, err := c.DeleteContainer(ctx, projectID, agentType)

// Remove a project from Warden (untrack).
_, err := c.RemoveProject(ctx, projectID, agentType)
```

All management operations require both the `projectID` and `agentType` to uniquely identify the project.

### Read a project template

```go
// Read a .warden.json template from an arbitrary path (for import).
tmpl, err := c.ReadProjectTemplate(ctx, "/path/to/.warden.json")
if err != nil {
    return err
}
fmt.Printf("Image: %s, Network: %s\n", tmpl.Image, tmpl.NetworkMode)
```

## Example: Server lifecycle

```go
// Check if the server is running.
resp, err := http.Get("http://localhost:8090/api/v1/health")
// resp.Header.Get("X-Warden") == "1" confirms it's a Warden server.

// Gracefully shut down the server.
err := c.Shutdown(ctx)
```

## Error handling

HTTP errors are wrapped in `client.APIError` with machine-readable codes:

```go
var apiErr *client.APIError
if errors.As(err, &apiErr) {
    fmt.Printf("API error %d [%s]: %s\n", apiErr.StatusCode, apiErr.Code, apiErr.Message)

    switch apiErr.Code {
    case "NAME_TAKEN":
        // Handle name collision
    case "NOT_FOUND":
        // Handle missing resource
    }
}
```

See the [HTTP API error codes](./api.md#error-codes) for the full list.

## When to use the client vs. the library

| Approach                        | Setup                          | Use when                                                            |
| ------------------------------- | ------------------------------ | ------------------------------------------------------------------- |
| `client.New()` (HTTP wrapper)   | Run `warden` binary separately | Multi-process, remote server, or when the binary is already running |
| `warden.New()` (Layer 1 import) | No binary needed               | Single-process deployment, embedded applications, full control      |

If you don't want to run a separate server process, you can import the library directly — see the [Go Library](./library.md) guide.

## Reference implementation

This is the same approach the TUI binary uses internally. The TUI wraps the service via a `Client` interface and delegates to it. See [`internal/tui/`](https://github.com/thesimonho/warden/tree/main/internal/tui) for the reference implementation — the `Client` interface shows the abstraction boundary, and `ServiceAdapter` shows how to adapt the Layer 1 service to the interface for embedded use.
