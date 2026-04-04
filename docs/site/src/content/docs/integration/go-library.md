---
title: Go Library
description: Import Warden directly into your Go application — no binary, no HTTP overhead.
---

Import Warden as a Go library for single-process deployment. No `warden` binary needed — your application initializes and controls the engine directly.

## Quick start

```go
import (
    "github.com/thesimonho/warden"
    "github.com/thesimonho/warden/api"
)

func main() {
    w, err := warden.New(warden.Options{})
    if err != nil {
        panic(err)
    }
    defer w.Close()

    ctx := context.Background()
    result, err := w.Service.CreateContainer(ctx, api.CreateContainerRequest{
        ProjectPath: "/path/to/repo",
        ProjectName: "my-project",
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Created project: %s (container: %s)\n", result.ProjectName, result.ContainerID)
}
```

## Warden initialization

```go
w, err := warden.New(warden.Options{
    DBDir:   "/path/to/db",  // Optional: override default database location
    Runtime: "docker",       // Optional: explicit runtime
})
if err != nil {
    return err
}
defer w.Close() // Idempotent; shuts down all subsystems (including session watchers)
```

`warden.New()` initializes the engine, database, event bus, agent registry, starts session watchers for active containers, and pre-warms the CLI cache in the background (downloading pinned Claude Code and Codex versions to a shared Docker volume). `w.Close()` tears down all subsystems including any running session watchers.

## Project management

### Create a project

```go
result, err := w.Service.CreateContainer(ctx, api.CreateContainerRequest{
    ProjectPath: "/workspace/path",
    ProjectName: "project-name",
    AgentType:   "claude-code",  // "claude-code" (default) or "codex"
    Image:       "ubuntu:24.04",
    EnvVars:     map[string]string{"USER": "alice", "OPENAI_API_KEY": os.Getenv("OPENAI_API_KEY")},
    Mounts:      []api.Mount{{HostPath: "/data", ContainerPath: "/data"}},
    NetworkMode: api.NetworkModeFull,
    AllowedDomains: []string{"api.example.com"},
    EnabledRuntimes: []string{"node", "python", "go"},
})
```

### List projects

```go
projects, err := w.Service.ListProjects(ctx)
for _, proj := range projects {
    fmt.Printf("%s: %s\n", proj.Name, proj.State)
}
```

### Stop/Restart

```go
err := w.Service.StopProject(ctx, "project-id", "claude-code")
err := w.Service.RestartProject(ctx, "project-id", "claude-code")
```

### Delete a project

```go
err := w.Service.DeleteContainer(ctx, "project-id", "claude-code")
err := w.Service.RemoveProject("project-id", "claude-code")
```

### Reset project cost history

```go
err := w.Service.ResetProjectCosts("project-id", "claude-code")
```

### Purge project audit history

```go
deleted, err := w.Service.PurgeProjectAudit("project-id", "claude-code")
```

### Read a project template

```go
// Read a .warden.json from an arbitrary path (for import).
tmpl, err := w.Service.ReadProjectTemplate("/path/to/.warden.json")
if err != nil {
    return err
}
fmt.Printf("Image: %s, Network: %s\n", tmpl.Image, tmpl.NetworkMode)
```

Templates are also automatically read during `GetDefaults(projectPath)` and written back to `.warden.json` on `CreateContainer` and `UpdateContainer`.

### Get project and worktree status

```go
projects, err := w.Service.ListProjects(ctx)
projectID := projects[0].ProjectID
agentType := projects[0].AgentType

worktrees, err := w.Service.ListWorktrees(ctx, projectID, agentType)
for _, wt := range worktrees {
    fmt.Printf("  - %s (state: %s)\n", wt.ID, wt.State)
}
```

## Worktree management

### Create a worktree

```go
result, err := w.Service.CreateWorktree(ctx, "project-id", "claude-code", "feature-branch")
```

The `agentType` parameter specifies which agent manages the worktree (`"claude-code"` or `"codex"`). This must match the agent type used when creating the project.

### Connect terminal (start agent)

```go
result, err := w.Service.ConnectTerminal(ctx, "project-id", "claude-code", "worktree-id")
```

### Disconnect terminal

```go
result, err := w.Service.DisconnectTerminal(ctx, "project-id", "claude-code", "worktree-id")
```

### Kill worktree process

```go
result, err := w.Service.KillWorktreeProcess(ctx, "project-id", "claude-code", "worktree-id")
```

### Remove worktree

```go
result, err := w.Service.RemoveWorktree(ctx, "project-id", "claude-code", "worktree-id")
```

### Restart a worktree

```go
// Kill and reconnect
err := w.Service.KillWorktreeProcess(ctx, "project-id", "claude-code", "worktree-id")
result, err := w.Service.ConnectTerminal(ctx, "project-id", "claude-code", "worktree-id")
```

### List worktrees

```go
worktrees, err := w.Service.ListWorktrees(ctx, "project-id", "claude-code")
for _, wt := range worktrees {
    fmt.Printf("%s: state=%s, branch=%s\n", wt.ID, wt.State, wt.Branch)
}
```

## Audit and event logging

Enable event logging via settings and query audit events through the service:

```go
// Enable detailed logging (off/standard/detailed)
mode := api.AuditLogDetailed
w.Service.UpdateSettings(ctx, api.UpdateSettingsRequest{
    AuditLogMode: &mode,
})
```

Query audit events:

```go
entries, err := w.Service.GetAuditLog(api.AuditFilters{
    Category:  api.AuditCategoryAgent,
    Container: "my-project",
    Limit:     100,
})

summary, err := w.Service.GetAuditSummary(ctx, api.AuditFilters{})
fmt.Printf("Sessions: %d, Tools: %d\n",
    summary.TotalSessions, summary.TotalToolUses)
```

Export as CSV:

```go
var buf bytes.Buffer
err := w.Service.WriteAuditCSV(&buf, api.AuditFilters{
    Container: "my-project",
})
```

## Event subscription (real-time)

```go
events, unsubscribe := w.Broker.Subscribe()
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

For advanced operations, access the service layer or engine directly:

```go
// Service methods
err := w.Service.ValidateContainer(ctx, "container-id")
containers, err := w.Service.ListContainers(ctx)
settings := w.Service.GetSettings()

// Engine client (advanced use only)
config, err := w.Engine.InspectContainer(ctx, "container-id")
```

## Error handling

Service methods return `error`. Check for sentinel errors:

```go
import "github.com/thesimonho/warden/service"

_, err := w.Service.RemoveProject("project-id", "claude-code")
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
    "github.com/thesimonho/warden/api"
)

func main() {
    ctx := context.Background()

    // 1. Initialize Warden
    w, err := warden.New(warden.Options{})
    if err != nil {
        log.Fatal(err)
    }
    defer w.Close()

    // 2. Create container for a project
    containerResult, err := w.Service.CreateContainer(ctx, api.CreateContainerRequest{
        ProjectPath: "/home/user/projects/my-app",
        ProjectName: "my-app",
        Image:       "ubuntu:24.04",
        Mounts:      []api.Mount{{HostPath: "/data", ContainerPath: "/data"}},
        NetworkMode: api.NetworkModeFull,
    })
    if err != nil {
        log.Fatal(err)
    }
    projectID := containerResult.ProjectID
    fmt.Printf("Created: %s (ID: %s)\n", containerResult.ProjectName, containerResult.ContainerID)

    // 3. Create worktree
    wtResult, err := w.Service.CreateWorktree(ctx, projectID, "claude-code", "feature-branch")
    if err != nil {
        log.Fatal(err)
    }

    // 4. Connect terminal (start agent)
    _, err = w.Service.ConnectTerminal(ctx, projectID, "claude-code", wtResult.WorktreeID)
    if err != nil {
        log.Fatal(err)
    }

    // 5. Get worktree status
    worktrees, err := w.Service.ListWorktrees(ctx, projectID, "claude-code")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Project status: %d worktrees\n", len(worktrees))

    // 6. Clean up
    _, _ = w.Service.DisconnectTerminal(ctx, projectID, "claude-code", wtResult.WorktreeID)
    _ = w.Service.DeleteContainer(ctx, projectID, "claude-code")
    _ = w.Service.RemoveProject(projectID, "claude-code")
}
```

## Reference

- **Entry point**: `warden.New(Options) (*Warden, error)`
- **Primary interface**: `w.Service` — all operations (containers, worktrees, settings, audit, access items)
- **Event bus**: `w.Broker` — subscribe to real-time events
- **Advanced access**: `w.Engine` — container runtime client, `w.DB` — SQLite store, `w.Watcher` — file-based event watcher
- **Go Packages**: See the [Go Packages reference](../../reference/go/) for API documentation
