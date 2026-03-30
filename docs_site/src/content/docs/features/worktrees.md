---
title: Worktrees & Terminals
description: Isolated workspaces and persistent terminal connections for Claude Code.
---

A **Worktree** is an isolated working directory within a project, backed by `git worktree`. Each worktree gets its own branch and directory, letting multiple Claude Code agents work on different tasks within the same project simultaneously.

A **Terminal** is the interface into a worktree. Terminals connect to a persistent process inside the container — closing the terminal doesn't kill Claude. You can reconnect later and pick up where you left off.

## Creating Worktrees

Create a worktree by providing a name. Warden creates an isolated directory with its own git branch, starts the agent, and connects your terminal. For non-git projects, the worktree is simply the project root directory.

### Worktree Storage

Worktrees are stored at agent-specific paths within the project:

| Agent       | Path                                  | Notes                                                   |
|-------------|---------------------------------------|-----------------------------------------------------------|
| **Claude**  | `.claude/worktrees/{worktree-id}/`    | Hardcoded by Claude Code. Cannot be configured.         |
| **Codex**   | `.warden/worktrees/{worktree-id}/`    | Warden-managed path for other agents (future support).  |
| **Others**  | `.warden/worktrees/{worktree-id}/`    | Same location for other supported agents.               |

Each agent has its own isolated worktree directory so multiple agents can work on different branches within the same project simultaneously without interference.

## Terminal Actions

| Action | What happens | Destructive? |
|--------|-------------|--------------|
| **Connect** | Start Claude Code. Terminal connects to the worktree process. | No |
| **Disconnect** | Close the terminal. Claude keeps running in the background. | No |
| **Reconnect** | Reattach to an existing background worktree. | No |
| **Kill** | Terminate all processes in the worktree. | Yes |
| **Remove** | Kill processes, then delete the worktree from disk. | Yes |

## Worktree States

Every worktree is in one of four states:

| State | What's happening | What you see |
|-------|-----------------|-------------|
| **Connected** | Claude is running, terminal attached | Live terminal |
| **Shell** | Claude exited, terminal attached | Bash prompt (can `claude --resume`) |
| **Background** | Claude is running, terminal closed | Reconnectable |
| **Disconnected** | Nothing running | Start fresh |

State transitions happen automatically:
- Close the terminal → **Connected** becomes **Background**
- Claude finishes and exits → **Connected** becomes **Shell**
- Kill the worktree process → any state becomes **Disconnected**
- Reconnect to a background worktree → **Background** becomes **Connected**

## Claude Activity

When a worktree is in the **Connected** state, Warden tracks what Claude is doing via hook events emitted from Claude Code. These sub-states tell you at a glance whether Claude needs your attention:

| Activity | Meaning | Indicator |
|----------|---------|-----------|
| **Working** | Claude is actively generating or executing tools | Amber pulsing dot |
| **Idle** | Claude is running but not actively working | Muted gray dot |
| **Need Permission** | Claude needs tool approval | Orange pulsing dot |
| **Need Answer** | Claude is asking a question | Red pulsing dot |
| **Need Input** | Claude is done, waiting for next prompt | Blue pulsing dot |

These activity states are broadcast as real-time events via SSE, so frontends can show attention indicators across all projects without opening each terminal.

## Worktree Diff

Each worktree exposes a git diff view showing uncommitted changes via the API. This lets you review what Claude has done before committing or providing feedback.

## Cleanup

Over time, worktrees can become orphaned — the git worktree directory exists on disk but isn't tracked properly. Use **Cleanup** to scan for and remove these orphaned worktrees. This is a manual operation, not automatic — invoke it when you suspect stale worktrees exist.

## For Developers

### HTTP API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/projects/{projectId}/worktrees` | List worktrees with state |
| `POST` | `/api/v1/projects/{projectId}/worktrees` | Create worktree |
| `POST` | `/api/v1/projects/{projectId}/worktrees/{wid}/connect` | Connect terminal |
| `POST` | `/api/v1/projects/{projectId}/worktrees/{wid}/disconnect` | Disconnect terminal |
| `POST` | `/api/v1/projects/{projectId}/worktrees/{wid}/kill` | Kill worktree process |
| `DELETE` | `/api/v1/projects/{projectId}/worktrees/{wid}` | Remove worktree |
| `POST` | `/api/v1/projects/{projectId}/worktrees/cleanup` | Cleanup orphaned worktrees |
| `GET` | `/api/v1/projects/{projectId}/worktrees/{wid}/diff` | Get uncommitted diff |
| `GET` | `/api/v1/projects/{projectId}/ws/{wid}` | Terminal WebSocket |

### Go Client

```go
c := client.New("http://localhost:8090")

// List worktrees with real-time state
worktrees, _ := c.ListWorktrees(ctx, projectID)

// Create a worktree (also connects terminal)
result, _ := c.CreateWorktree(ctx, projectID, "fix-auth-bug")

// Terminal lifecycle
c.ConnectTerminal(ctx, projectID, worktreeID)
c.DisconnectTerminal(ctx, projectID, worktreeID)
c.KillWorktreeProcess(ctx, projectID, worktreeID)

// Review changes
diff, _ := c.GetWorktreeDiff(ctx, projectID, worktreeID)

// Attach directly to the terminal WebSocket
conn, _ := c.AttachTerminal(ctx, projectID, worktreeID)
defer conn.Close()
```

### Go Library

```go
app, _ := warden.New(warden.Options{})

// Create worktree and start Claude
result, _ := app.Service.CreateWorktree(ctx, project, "fix-auth-bug")

// Terminal lifecycle
app.Service.ConnectTerminal(ctx, project, worktreeID)
app.Service.DisconnectTerminal(ctx, project, worktreeID)
app.Service.KillWorktreeProcess(ctx, project, worktreeID)
```

See the [Go Packages](/warden/reference/go/) reference for full API documentation.
