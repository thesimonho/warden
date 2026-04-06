# Worktrees

## Core Principle

A worktree is a unit of independent work. The user creates worktrees to have the agent work on separate things in parallel, each on its own branch. Within a worktree, the agent manages conversations internally — Warden does not track or manage conversations.

## Worktree States

| State          | Meaning                                                    | What happens on click                                   |
| -------------- | ---------------------------------------------------------- | ------------------------------------------------------- |
| **connected**  | Terminal running, agent active                             | Show the existing terminal                              |
| **shell**      | Agent exited, bash shell still alive in tmux               | Show the terminal (with "Agent exited" indicator)       |
| **background** | tmux session alive, WebSocket closed (viewer disconnected) | Reconnect: reconnect WebSocket to existing tmux session |
| **stopped**    | No processes running                                       | Connect: start new terminal, launch agent               |

## Discovery

Worktrees are discovered by listing the filesystem:

- **Git repos:** `git worktree list` inside the container gives all worktrees with their paths and branches.
- **Non-git repos:** Exactly one implicit worktree — the workspace root itself.

This is always consistent because git is the source of truth for worktrees, just as the agent is the source of truth for conversations.

## Lifecycle Across Container Events

| Container Event | Impact on Terminals                                     | Impact on Worktrees                                                                                  |
| --------------- | ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Stop            | All tmux sessions killed. WebSocket connections closed. | Git worktrees preserved on bind mount. All become "stopped". Auto-resume on reconnect.               |
| Start / Restart | Entrypoint writes exit_code for orphaned terminals.     | Worktrees rediscovered from filesystem. All start as "stopped". Auto-resume on reconnect.            |
| Recreate (edit) | Old container removed.                                  | Worktrees preserved on bind mount. Reconnectable in new container.                                   |
| Delete          | Container removed.                                      | Worktrees preserved on bind mount. Available when a new container mounts the same project directory. |

## Worktree Attention/Notifications

Worktrees can require user attention. Attention state is pushed via the event bus:

1. For Claude Code: hook events (Notification, PreToolUse, UserPromptSubmit) are written by `warden-event-claude.sh` to the bind-mounted event directory. The backend's file watcher detects them, tracks attention state per worktree, and broadcasts changes via SSE. Codex does not yet support hooks — attention tracking for Codex projects is a known upstream gap.

| Attention Type       | Priority    | Indicator                                           |
| -------------------- | ----------- | --------------------------------------------------- |
| `permission_prompt`  | 3 (highest) | Orange dot — Claude needs tool approval             |
| `elicitation_dialog` | 2           | Red dot — Claude is asking a question               |
| `idle_prompt`        | 1           | Blue dot — Claude finished, waiting for next prompt |

Project cards on the home page show the highest-priority attention type across all worktrees.
