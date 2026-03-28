---
title: Go Library
description: Import Warden directly into your Go application — no binary, no HTTP overhead.
---

Import Warden as a Go library for single-process deployment. No `warden` binary needed — your application initializes and controls the engine directly.

## Quick start

```go
import "github.com/thesimonho/warden"

func main() {
    app, err := warden.New(warden.Options{})
    if err != nil {
        panic(err)
    }
    defer app.Close()

    ctx := context.Background()
    result, err := app.CreateProject(ctx, "my-project", "/path/to/repo", nil)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Created project: %s (container: %s)\n", result.Name, result.ContainerID)
}
```

## App initialization

```go
app, err := warden.New(warden.Options{
    DBDir:   "/path/to/db",  // Optional: override default database location
    Runtime: "docker",       // Optional: explicit runtime
})
if err != nil {
    return err
}
defer app.Close() // Idempotent; shuts down all subsystems
```

## Project management

### Create a project

```go
// Minimal creation
result, err := app.CreateProject(ctx, "project-name", "/workspace/path", nil)

// With options
result, err := app.CreateProject(ctx, "project-name", "/workspace/path", &warden.CreateProjectOptions{
    Image:           "ubuntu:24.04",
    EnvVars:         map[string]string{"USER": "alice", "DEBUG": "1"},
    Mounts:          []string{"/data:/data"},
    SkipPermissions: false,
    NetworkMode:     engine.NetworkModeFull,
    AllowedDomains:  []string{"api.example.com"},
})
```

### List projects

```go
projects, err := app.Service.ListProjects(ctx)
for _, proj := range projects {
    fmt.Printf("%s: %s\n", proj.Name, proj.State)
}
```

### Stop/Restart

```go
result, err := app.StopProject(ctx, "project-id")
result, err := app.RestartProject(ctx, "project-id")
results, err := app.StopAll(ctx)
```

### Delete a project

```go
result, err := app.DeleteProject(ctx, "project-id")
```

### Reset project cost history

```go
err := app.Service.ResetProjectCosts("project-id")
```

### Purge project audit history

```go
deleted, err := app.Service.PurgeProjectAudit("project-id")
```

### Get project status

```go
status, err := app.GetProjectStatus(ctx, "project-name")
fmt.Printf("Project: %s\n", status.Project.Name)
for _, wt := range status.Worktrees {
    fmt.Printf("  - %s (state: %s)\n", wt.ID, wt.State)
}
```

## Worktree management

### Create a worktree

```go
// Resolve project row from project ID
project, err := app.Service.GetProject("project-id")
if err != nil {
    return err
}
result, err := app.Service.CreateWorktree(ctx, project, "feature-branch")
```

### Connect terminal (start Claude)

```go
// Resolve project row from project ID
project, err := app.Service.GetProject("project-id")
if err != nil {
    return err
}
result, err := app.Service.ConnectTerminal(ctx, project, "worktree-id")
```

### Disconnect terminal

```go
// Resolve project row from project ID
project, err := app.Service.GetProject("project-id")
if err != nil {
    return err
}
result, err := app.Service.DisconnectTerminal(ctx, project, "worktree-id")
```

### Kill worktree process

```go
// Resolve project row from project ID
project, err := app.Service.GetProject("project-id")
if err != nil {
    return err
}
result, err := app.Service.KillWorktreeProcess(ctx, project, "worktree-id")
```

### Remove worktree

```go
// Resolve project row from project ID
project, err := app.Service.GetProject("project-id")
if err != nil {
    return err
}
result, err := app.Service.RemoveWorktree(ctx, project, "worktree-id")
```

### Restart a worktree

```go
result, err := app.RestartWorktree(ctx, "project-id", "worktree-id")
```

### List worktrees

```go
// Resolve project row from project ID
project, err := app.Service.GetProject("project-id")
if err != nil {
    return err
}
worktrees, err := app.Service.ListWorktrees(ctx, project)
for _, wt := range worktrees {
    fmt.Printf("%s: state=%s, branch=%s\n", wt.ID, wt.State, wt.Branch)
}
```

## Audit and event logging

Enable event logging via settings and query audit events through the service:

```go
// Enable detailed logging (off/standard/detailed)
mode := api.AuditLogDetailed
app.Service.UpdateSettings(ctx, api.UpdateSettingsRequest{
    AuditLogMode: &mode,
})
```

Query audit events:

```go
entries, err := app.Service.GetAuditLog(api.AuditFilters{
    Category:  api.AuditCategoryAgent,
    Container: "my-project",
    Limit:     100,
})

summary, err := app.Service.GetAuditSummary(ctx, api.AuditFilters{})
fmt.Printf("Sessions: %d, Tools: %d\n",
    summary.TotalSessions, summary.TotalToolUses)
```

Export as CSV:

```go
var buf bytes.Buffer
err := app.Service.WriteAuditCSV(&buf, api.AuditFilters{
    Container: "my-project",
})
```

## Event subscription (real-time)

```go
events, unsubscribe := app.Broker.Subscribe()
defer unsubscribe()

for event := range events {
    switch event.Event {
    case eventbus.SSEWorktreeState:
        fmt.Printf("Worktree state: %s\n", string(event.Data))
    case eventbus.SSEProjectState:
        fmt.Printf("Project state: %s\n", string(event.Data))
    case eventbus.SSEBudgetExceeded:
        fmt.Printf("Budget exceeded: %s\n", string(event.Data))
    case eventbus.SSEBudgetContainerStopped:
        // Includes containerId for matching against project views
        fmt.Printf("Container stopped by budget: %s\n", string(event.Data))
    }
}
```

## Lower-level API access

For operations not exposed via App convenience methods, access the service layer directly:

```go
svc := app.Service

err := svc.ValidateContainer(ctx, "container-id")
containers, err := svc.ListContainers(ctx)
config, err := svc.InspectContainer(ctx, "container-id")
settings := svc.GetSettings()
```

## Error handling

Service methods return `error`. Check for sentinel errors:

```go
import "github.com/thesimonho/warden/service"

_, err := app.Service.RemoveProject("project-id")
if errors.Is(err, service.ErrNotFound) {
    fmt.Println("Project not found")
}
if errors.Is(err, service.ErrInvalidInput) {
    fmt.Println("Invalid input")
}
```

## Complete example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/thesimonho/warden"
    "github.com/thesimonho/warden/engine"
)

func main() {
    ctx := context.Background()

    // 1. Initialize App
    app, err := warden.New(warden.Options{})
    if err != nil {
        log.Fatal(err)
    }
    defer app.Close()

    // 2. Create project
    projectResult, err := app.CreateProject(ctx, "my-app", "/home/user/projects/my-app", &warden.CreateProjectOptions{
        Image:       "ubuntu:24.04",
        Mounts:      []string{"/data:/data"},
        NetworkMode: engine.NetworkModeFull,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created: %s (ID: %s)\n", projectResult.Name, projectResult.ContainerID)

    // 3. Resolve project row for subsequent operations
    project, err := app.Service.GetProject(projectResult.ProjectID)
    if err != nil {
        log.Fatal(err)
    }

    // 4. Create worktree
    wtResult, err := app.Service.CreateWorktree(ctx, project, "feature-branch")
    if err != nil {
        log.Fatal(err)
    }

    // 5. Connect terminal (start Claude)
    _, err = app.Service.ConnectTerminal(ctx, project, wtResult.WorktreeID)
    if err != nil {
        log.Fatal(err)
    }

    // 6. Get status
    status, err := app.GetProjectStatus(ctx, projectResult.Name)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Project status: %d worktrees\n", len(status.Worktrees))

    // 7. Clean up
    _, _ = app.Service.DisconnectTerminal(ctx, project, wtResult.WorktreeID)
    _, _ = app.DeleteProject(ctx, projectResult.ProjectID)
}
```

## Reference

- **Entry point**: `warden.New(Options) (*App, error)`
- **Convenience methods**: `CreateProject`, `DeleteProject`, `StopProject`, `RestartProject`, `StopAll`, `RestartWorktree`, `GetProjectStatus`
- **Service layer**: `app.Service` — full access to business logic
- **Event bus**: `app.Broker` — subscribe to real-time events
- **Go Packages**: See the [Go Packages reference](../../reference/go/) for API documentation
