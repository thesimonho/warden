# Worktrees & Terminals

## For Developers

### HTTP API

| Method   | Endpoint                                                              | Description                |
| -------- | --------------------------------------------------------------------- | -------------------------- |
| `GET`    | `/api/v1/projects/{projectId}/{agentType}/worktrees`                  | List worktrees with state  |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees`                  | Create worktree            |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/connect`    | Connect terminal           |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/disconnect` | Disconnect terminal        |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/kill`       | Kill worktree process      |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/reset`      | Reset worktree             |
| `DELETE` | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}`            | Remove worktree            |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees/cleanup`          | Cleanup orphaned worktrees |
| `GET`    | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/diff`       | Get uncommitted diff       |
| `GET`    | `/api/v1/projects/{projectId}/{agentType}/ws/{wid}`                   | Terminal WebSocket         |

### Go Client

```go
c := client.New("http://localhost:8090")

// List worktrees with real-time state
worktrees, _ := c.ListWorktrees(ctx, projectID, agentType)

// Create a worktree (also connects terminal)
result, _ := c.CreateWorktree(ctx, projectID, agentType, "fix-auth-bug")

// Terminal lifecycle
c.ConnectTerminal(ctx, projectID, agentType, worktreeID)
c.DisconnectTerminal(ctx, projectID, agentType, worktreeID)
c.KillWorktreeProcess(ctx, projectID, agentType, worktreeID)

// Review changes
diff, _ := c.GetWorktreeDiff(ctx, projectID, agentType, worktreeID)

// Attach directly to the terminal WebSocket
conn, _ := c.AttachTerminal(ctx, projectID, agentType, worktreeID)
defer conn.Close()
```

### Go Library

```go
w, _ := warden.New(warden.Options{})

// Create worktree and start agent
result, _ := w.Service.CreateWorktree(ctx, projectID, agentType, "fix-auth-bug")

// Terminal lifecycle
w.Service.ConnectTerminal(ctx, projectID, agentType, worktreeID)
w.Service.DisconnectTerminal(ctx, projectID, agentType, worktreeID)
w.Service.KillWorktreeProcess(ctx, projectID, agentType, worktreeID)
```

See the [Go Packages](https://thesimonho.github.io/warden/reference/go/) reference for full API documentation.
