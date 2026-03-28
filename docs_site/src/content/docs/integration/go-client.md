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
    fmt.Printf("%s: %s\n", proj.Name, proj.State)
}
```

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

| `client.New()` (typed HTTP wrapper) | `warden.New()` (direct import) |
| ----------------------------------- | ------------------------------ |
| Requires `warden` binary running    | No binary needed               |
| Multi-process or remote             | Single-process deployment      |
| Simpler integration                 | Full control over lifecycle    |

If you don't want to run a separate server process, you can import the library directly — see the [Go Library](../go-library/) guide.

## Reference implementation

This is the same approach the TUI binary uses internally. See [`internal/tui/`](https://github.com/thesimonho/warden/tree/main/internal/tui) for the reference implementation — specifically the Client interface, data loading patterns, and terminal attachment.
