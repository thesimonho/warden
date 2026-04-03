---
title: Worktrees & Terminals
description: Isolated workspaces and persistent terminal connections for AI coding agents.
---

A **Worktree** is an isolated working directory within a project, backed by `git worktree`. Each worktree gets its own branch and directory, letting multiple agents work on different tasks within the same project simultaneously.

A **Terminal** is the interface into a worktree. Terminals connect to a persistent process inside the container — closing the terminal doesn't kill the agent. You can reconnect later and pick up where you left off.

## Creating Worktrees

Create a worktree by providing a name. Warden creates an isolated directory with its own git branch, starts the agent, and connects your terminal. For non-git projects, the worktree is simply the project root directory.

### Worktree Storage

Worktrees are stored at agent-specific paths within the project:

| Agent      | Path                               | Notes                                           |
| ---------- | ---------------------------------- | ----------------------------------------------- |
| **Claude** | `.claude/worktrees/{worktree-id}/` | Hardcoded by Claude Code. Cannot be configured. |
| **Codex**  | `.warden/worktrees/{worktree-id}/` | Warden-managed via `git worktree add/remove`.   |
| **Others** | `.warden/worktrees/{worktree-id}/` | Same Warden-managed location for future agents. |

Claude Code manages its own worktrees internally (via `--worktree`), while Codex worktrees are managed by Warden (via `git worktree add/remove`). From the user's perspective, both behave the same — create a worktree, connect, and start working.

## Terminal Actions

| Action         | What happens                                                   | Destructive? |
| -------------- | -------------------------------------------------------------- | ------------ |
| **Connect**    | Start the agent. Terminal connects to the worktree process.    | No           |
| **Disconnect** | Close the terminal. The agent keeps running in the background. | No           |
| **Reconnect**  | Reattach to an existing background worktree.                   | No           |
| **Kill**       | Terminate all processes in the worktree.                       | Yes          |
| **Remove**     | Kill processes, then delete the worktree from disk.            | Yes          |

## Worktree States

Every worktree is in one of four states:

| State            | What's happening                    | What you see             |
| ---------------- | ----------------------------------- | ------------------------ |
| **Connected**    | Agent is running, terminal attached | Live terminal            |
| **Shell**        | Agent exited, terminal attached     | Bash prompt (can resume) |
| **Background**   | Agent is running, terminal closed   | Reconnectable            |
| **Disconnected** | Nothing running                     | Start fresh              |

State transitions happen automatically:

- Close the terminal → **Connected** becomes **Background**
- Agent finishes and exits → **Connected** becomes **Shell**
- Kill the worktree process → any state becomes **Disconnected**
- Reconnect to a background worktree → **Background** becomes **Connected**

## Agent Activity

When a worktree is in the **Connected** state, Warden tracks what the agent is doing. For Claude Code, attention state comes from hook events. For Codex, attention tracking is a known upstream limitation — Codex does not yet support hooks, so sub-state detection is not available. These sub-states tell you at a glance whether the agent needs your attention:

| Activity            | Meaning                                         | Indicator          |
| ------------------- | ----------------------------------------------- | ------------------ |
| **Working**         | Agent is actively generating or executing tools | Amber pulsing dot  |
| **Idle**            | Agent is running but not actively working       | Muted gray dot     |
| **Need Permission** | Agent needs tool approval                       | Orange pulsing dot |
| **Need Answer**     | Agent is asking a question                      | Red pulsing dot    |
| **Need Input**      | Agent is done, waiting for next prompt          | Blue pulsing dot   |

These activity states are broadcast as real-time events via SSE, so frontends can show attention indicators across all projects without opening each terminal.

## Worktree Diff

Each worktree exposes a git diff view showing uncommitted changes via the API. This lets you review what the agent has done before committing or providing feedback.

## Cleanup

Over time, worktrees can become orphaned — the git worktree directory exists on disk but isn't tracked properly. Use **Cleanup** to scan for and remove these orphaned worktrees. This is a manual operation, not automatic — invoke it when you suspect stale worktrees exist.

## For Developers

### HTTP API

| Method   | Endpoint                                                              | Description                |
| -------- | --------------------------------------------------------------------- | -------------------------- |
| `GET`    | `/api/v1/projects/{projectId}/{agentType}/worktrees`                  | List worktrees with state  |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees`                  | Create worktree            |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/connect`    | Connect terminal           |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/disconnect` | Disconnect terminal        |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/kill`       | Kill worktree process      |
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

See the [Go Packages](/warden/reference/go/) reference for full API documentation.
