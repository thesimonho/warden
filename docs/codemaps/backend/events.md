# Event System

## agent/session_watcher

JSONL session file parser and watcher for multi-agent event collection.

| File | Purpose |
| --- | --- |
| `agent/session_watcher.go` | `SessionWatcher` — discovers and tails multiple concurrent JSONL session files via agent-specific parsers. Discovers files using `SessionParser.FindSessionFiles()` (Claude Code: per-project directory scan; Codex: shell snapshot + glob discovery). Polls every 2 seconds for new files and new lines. Handles multiple concurrent session files via `tailedFiles` map. Feeds parsed `ParsedEvent`s to callback for bridge → eventbus → SSE/audit. Lifecycle: `Start(ctx)`, `Stop()`. One watcher per project, created when container starts, stopped when container stops. |
| `service/session_bridge.go` | `SessionEventToContainerEvent(agent.ParsedEvent, ...) *eventbus.ContainerEvent` — translates parsed JSONL events to container events for pipeline (store → broker → SSE → frontend, audit log). Maps agent-agnostic event types to container event types; attaches event payloads (tool name, prompt text, cost data). |

Data flow: Container writes JSONL session line → host-side watcher detects file change → parses line via `SessionParser` → generates `ParsedEvent` → bridge converts to `ContainerEvent` → eventbus pipeline broadcasts SSE + writes audit.

## eventlog/

Centralized host-side event log for container and system events.

| File | Purpose |
| --- | --- |
| `entry.go` | `Entry` struct with timestamp, source (`SourceAgent`, `SourceBackend`, `SourceFrontend`, `SourceContainer`), `ProjectID` (deterministic hash of host path), `ContainerName` (snapshot at event time for display), worktree ID, action, details. `QueryFilters` for indexed queries (keyed by `ProjectID`). `ProjectRow` struct with `ProjectID`, `Name`, `HostPath`, `ContainerID`, `ContainerName`, and container config fields. `DisplayProject()` method for log display. Source constants used throughout logging. |
| `db.go` | SQLite schema (projects keyed by `project_id`, events indexed by `project_id` + `container_name`, `session_costs` keyed by `project_id` + `session_id`), connection setup (WAL mode, pragmas), `openDB()` helper. Legacy `worktree_costs` table dropped. |
| `store.go` | `Store` — SQLite-backed storage with concurrent safety, nil-receiver no-op. Methods: `Write()`, `Read()`, `Query(filters)`, `DistinctProjectIDs()`, `Clear()`, `Close()`, `InsertProject()`, `GetProject(projectID)`, `GetProjectByPath(hostPath)`, `ListAllProjects()`, `ListProjectIDs()`, `HasProject(projectID)`, `DeleteProject(projectID)`, `UpdateProjectContainer(projectID, containerID, containerName)`, `UpsertSessionCost(projectID, sessionID, ...)`, `GetProjectTotalCost(projectID)`, `GetAllProjectTotalCosts()`, `GetCostInTimeRange(projectID, ...)`, `DeleteProjectCosts(projectID)` |
| `slog_handler.go` | Custom `slog.Handler` that tees backend slog records to the event log, enabling centralized debugging |
| `logger_test.go` | Full test coverage for logger, query filters, distinct project IDs, nil safety, concurrent writes |

## eventbus/

Push-based event system for container-to-host communication via file-based watcher.

### Types

| File | Purpose |
| --- | --- |
| `types.go` | Event types: `ContainerEvent` (from container hooks + lifecycle, carries `ProjectID`, `ContainerName`, and `SessionID`), `SSEEvent` (to frontend), `AttentionData`, `CostData`, `ToolUseData`, `ToolUseFailureData`, `StopFailureData`, `PermissionRequestData`, `SubagentData`, `ConfigChangeData`, `InstructionsLoadedData`, `TaskCompletedData`, `ElicitationData`, `TerminalConnectedData`, `SessionExitData`; all `Event*` constants for each hook type; `SSEBudgetExceeded` and `SSEBudgetContainerStopped` event types; `BudgetEventPayload` and `BudgetContainerStoppedPayload`. Payloads include `projectId` for frontend routing. |

### Watcher

| File | Purpose |
| --- | --- |
| `watcher.go` | File-based event watcher — watches bind-mounted event directories for JSON event files using fsnotify (fast path) + polling every 2s (reliable fallback). Each project/container has a subdirectory at `<baseDir>/<containerName>/events/`. Reads files matching `<epoch_ns>-<pid>.json`, parses events (expecting `projectId` in JSON), dispatches to handler, deletes after processing. Cleans up orphaned `.tmp` files older than 30s. `NewWatcher(baseDir, handler, pollInterval)` constructor; `Start(ctx)` processes existing files (crash recovery), then runs fsnotify + polling loops; `WatchContainerDir`/`UnwatchContainerDir` for fsnotify registration; `CleanupContainerDir` drains remaining events and removes directory. |
| `watcher_test.go` | Watcher tests: atomic file write detection, `.tmp` ignoring, invalid JSON cleanup, oversized file rejection, crash recovery, concurrent container directories, shutdown without deadlock, Watch/Unwatch lifecycle, missing required fields (including `projectId`) |

### Store

| File | Purpose |
| --- | --- |
| `store.go` | Thread-safe in-memory state store — per-project/container/worktree attention + cost + terminal lifecycle. `ProjectCost` type (TotalCost, MessageCount, IsEstimated, UpdatedAt). `ProjectStatePayload` carries cost + aggregated attention for `project_state` SSE events. `StopCallbackFunc` signature: `(projectID, containerName, sessionID string, cost float64, isEstimated bool)`. `StaleCallbackFunc` signature: `(containerName string)`. Methods: `GetWorktreeState`, `GetContainerWorktreeStates`, `AggregateContainerAttention`, `GetTerminalState`, `HasTerminalData`, `ActiveContainers`, `MarkContainerStale`, `EvictWorktree`, `SetStopCallback(fn)`, `SetStaleCallback(fn)`, `BroadcastBudgetExceeded`, `BroadcastBudgetContainerStopped`, `HandleEvent`. Every attention state change broadcasts both `worktree_state` (per-worktree) and `project_state` (aggregated) SSE events. `writeToAuditLog()` maps event types to agent/container source; skips `attention_clear`. |
| `store_test.go` | Store tests: all event types, state isolation, concurrent access, unknown lookups, projectID routing |

### Infrastructure

| File | Purpose |
| --- | --- |
| `liveness.go` | Periodic liveness checker (runs every 15s) that checks container heartbeat staleness, marks containers stale after 30s of missing heartbeats |
| `broker.go` | SSE broker — manages client subscriptions, fan-out broadcast, 15s heartbeat, slow-client drop, graceful shutdown |
| `broker_test.go` | Broker tests: subscribe/unsubscribe, multi-client broadcast, slow client, heartbeat, shutdown |
