# client/ — Go HTTP client for the Warden API

A ready-made Go client for interacting with a running `warden` server over HTTP.

## Who is this for?

Go developers who want to control Warden from their own application without embedding the engine. You run the `warden` binary as a headless server and use this package to talk to it.

If you want to embed the engine directly (no separate server process), use `warden.New()` from the root package instead.

## Usage

```go
import "github.com/thesimonho/warden/client"

c := client.New("http://localhost:8090")

// List all projects
projects, err := c.ListProjects(ctx)

// Create a worktree
resp, err := c.CreateWorktree(ctx, projectID, "feature-branch")

// Stop a project
err = c.StopProject(ctx, projectID)
```

## Error handling

Non-2xx responses are returned as `*client.APIError`, which includes the HTTP status code and the error message from the server:

```go
_, err := c.CreateContainer(ctx, req)
if err != nil {
    var apiErr *client.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API error %d: %s\n", apiErr.StatusCode, apiErr.Message)
    }
}
```

## Relationship to web/src/lib/api.ts

This package is the Go equivalent of the TypeScript API client used by the web dashboard. Both make the same HTTP calls to the same endpoints. If you're building a web frontend, look at `api.ts` instead. If you're building a Go frontend or CLI, use this package.
