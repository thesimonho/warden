---
title: Go Client
description: A typed Go wrapper around the Warden HTTP API for convenience.
---

The [`client`](https://github.com/thesimonho/warden/tree/main/client) package is a convenience wrapper around the [HTTP API](../http-api/). It provides typed Go functions for every endpoint so you don't have to manage HTTP requests, response parsing, or error handling yourself.

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

## Example: Create a worktree

```go
resp, err := c.CreateWorktree(ctx, projectID, "feature-branch")
if err != nil {
    return err
}
fmt.Printf("Created worktree: %s\n", resp.WorktreeID)
```

## Example: Manage a project

The client exposes the same four management actions as the web and TUI dashboards:

```go
// Reset all cost tracking data for a project.
err := c.ResetProjectCosts(ctx, projectID)

// Purge all audit events for a project.
err := c.PurgeProjectAudit(ctx, projectID)

// Delete a project's container (stop + remove).
_, err := c.DeleteContainer(ctx, projectID)

// Remove a project from Warden (untrack).
_, err := c.RemoveProject(ctx, projectID)
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

See the [HTTP API error codes](../http-api/#error-codes) for the full list.

## When to use the client vs. the library

| Approach                          | Setup                           | Use when                              |
| --------------------------------- | ------------------------------- | ------------------------------------- |
| `client.New()` (HTTP wrapper)     | Run `warden` binary separately  | Multi-process, remote server, or when the binary is already running |
| `warden.New()` (Layer 1 import)   | No binary needed                | Single-process deployment, embedded applications, full control |

If you don't want to run a separate server process, you can import the library directly — see the [Go Library](../go-library/) guide.

## Reference implementation

This is the same approach the TUI binary uses internally. The TUI wraps the service via a `Client` interface and delegates to it. See [`internal/tui/`](https://github.com/thesimonho/warden/tree/main/internal/tui) for the reference implementation — the `Client` interface shows the abstraction boundary, and `ServiceAdapter` shows how to adapt the Layer 1 service to the interface for embedded use.
