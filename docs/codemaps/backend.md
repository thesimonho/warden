# Backend Codemap

## Entry Point

- `warden.go` — top-level `App` type: `New(Options)` wires SQLite database, runtime detection, engine client, event bus pipeline, and service layer into a ready-to-use handle; `Close()` shuts down all subsystems (idempotent via `sync.Once`). Convenience methods (all take project ID and call `resolveProject(id)` to fetch `*db.ProjectRow` before delegating to service): `CreateProject(ctx, name, hostPath, opts)`, `DeleteProject(ctx, projectID)`, `StopProject(ctx, projectID)`, `RestartProject(ctx, projectID)`, `StopAll(ctx)` (pre-loads all project rows to avoid N+1), `RestartWorktree(ctx, projectID, worktreeID)`, `GetProjectStatus(ctx, projectName)`. Returns typed result structs (`ProjectResult`, `WorktreeResult`, `ContainerResult`). This is the primary entry point for Go library consumers.
- `cmd/warden/main.go` — headless engine server: creates `warden.App`, starts HTTP API server, blocks until SIGTERM/SIGINT
- `cmd/warden-desktop/main.go` — web dashboard: creates `warden.App`, wires terminal proxy and HTTP server with embedded SPA, opens browser, delegates to `run()`
- `cmd/warden-desktop/run.go` — server lifecycle: start, wait for ready, open browser, signal handling, graceful shutdown
- `cmd/warden-tui/main.go` — TUI binary: creates `warden.App`, wraps in `ServiceAdapter`, runs Bubble Tea program

## client/

Go HTTP client for the Warden API. The Go equivalent of `web/src/lib/api.ts`.

| File             | Purpose                                                                                |
| ---------------- | -------------------------------------------------------------------------------------- |
| `client.go`      | `Client` struct, `New(baseURL)`, all API methods (projects, worktrees, containers, settings, host utilities, audit log, SSE events, WebSocket terminal), `APIError` type with `StatusCode`, `Code`, and `Message` fields, `TerminalConnection` interface, HTTP/SSE/WebSocket helpers, `deleteWithBody` helper for DELETE with request body |
| `client_test.go` | Tests with httptest servers for each endpoint pattern, SSE stream parser tests |

## api/

API contract types shared by the service layer, HTTP client, and TUI. Consumers import this package for types without depending on the service implementation.

| File       | Purpose                                                                                                        |
| ---------- | -------------------------------------------------------------------------------------------------------------- |
| `types.go` | `ProjectResult` (ProjectID, Name, ContainerID, ContainerName), `WorktreeResult`, `ContainerResult` (with ProjectID), `ValidateContainerResult`, `AuditLogMode` (off/standard/detailed), `SettingsResponse`, `UpdateSettingsRequest`, `UpdateSettingsResult`, `PostAuditEventRequest`, `DefaultMount`, `DefaultsResponse`, `DirEntry`, `DiffFileSummary`, `DiffResponse` (diff stats + raw unified diff), `AuditCategory` (session/agent/prompt/config/budget/system/debug), `AuditFilters` (keyed by `ProjectID` instead of `Container`), `AuditSummary`, `ToolCount`, `TimeRange` |

## service/

Business logic layer — orchestrates engine, database, event store, and event logger without HTTP concerns. Both the HTTP server and direct Go library consumers call the same service methods. API contract types are defined in `api/` and re-exported via type aliases in `api_aliases.go`.

| File               | Purpose                                                                                                |
| ------------------ | ------------------------------------------------------------------------------------------------------ |
| `service.go`       | `Service` struct, `New(engine.Client, *db.Store, *eventbus.Store, *db.AuditWriter)` constructor. Minimal — no resolve helpers. Exposes `GetProject(projectID) (*db.ProjectRow, error)` for lookups by ID. |
| `api_aliases.go`   | Type aliases re-exporting `api.*` types as `service.*` for backward compatibility                      |
| `errors.go`        | Service-layer error sentinels (`ErrNotFound`, `ErrInvalidInput`)                                        |
| `projects.go`      | `ListProjects(ctx) ([]engine.Project, error)`, `AddProject(name, hostPath) (*ProjectResult, error)`, `RemoveProject(projectID) (*ProjectResult, error)` (emits `project_removed` audit event; cleans up costs/audit when audit logging is off), `ResetProjectCosts(projectID) error` (emits `cost_reset` audit event), `PurgeProjectAudit(projectID) (int64, error)` (emits `audit_purged` audit event), `GetProject(projectID) (*db.ProjectRow, error)`, `StopProject(ctx, *db.ProjectRow) (*ProjectResult, error)`, `RestartProject(ctx, *db.ProjectRow) (*ProjectResult, error)`, `HandleContainerStale(containerName)` (writes audit entry with resolved project context — called by event bus stale callback). Helpers: `effectiveContainerName(row)` derives container name from row, `resolveProjectName(projectID)` looks up container name by project ID. Cost overlay (DB → event store → agent config fallback) and DB metadata overlay applied at `ListProjects` time. |
| `budget.go`        | `PersistSessionCost(projectID, containerName, sessionID string, cost float64, isEstimated bool)` — single gateway for all cost writes (persists to DB keyed by `projectID`, triggers budget enforcement). `enforceBudget(projectID)` (unexported, called exclusively by `PersistSessionCost` — single `GetProject` call derives budget, container name, and container ID; executes configured actions: warn, stop worktrees, stop container). `GetEffectiveBudget(projectKey) float64` (per-project or global default). `effectiveBudgetFromRow(row)` (internal, computes budget from an already-fetched row). `GetDefaultProjectBudget() float64`. `IsOverBudget(projectID) bool` (checks cost + preventStart action for blocking restarts). `getBudgetActions()` reads and parses budget action settings. |
| `containers.go`    | `CreateContainer(ctx, request) (*ContainerResult, error)`, `DeleteContainer(ctx, *db.ProjectRow) (*ContainerResult, error)` (emits `container_deleted` audit event), `InspectContainer(ctx, *db.ProjectRow) (*ContainerConfig, error)`, `UpdateContainer(ctx, *db.ProjectRow, request) (*ContainerResult, error)`, `ValidateContainer(ctx, name, image) (*ValidateContainerResult, error)`. All container methods (except `ValidateContainer`) now accept `*db.ProjectRow` instead of container ID. |
| `worktrees.go`     | `ListWorktrees(ctx, *db.ProjectRow) ([]engine.Worktree, error)`, `CreateWorktree(ctx, *db.ProjectRow, name) (*WorktreeResult, error)`, `ConnectTerminal(ctx, *db.ProjectRow, worktreeID) (*WorktreeResult, error)`, `DisconnectTerminal(ctx, *db.ProjectRow, worktreeID) (*WorktreeResult, error)`, `KillWorktreeProcess(ctx, *db.ProjectRow, worktreeID) (*WorktreeResult, error)`, `RemoveWorktree(ctx, *db.ProjectRow, worktreeID) (*WorktreeResult, error)`, `CleanupWorktrees(ctx, *db.ProjectRow) (*WorktreeResult, error)`, `GetWorktreeDiff(ctx, *db.ProjectRow, worktreeID) (*DiffResponse, error)`, `NotifyTerminalDisconnected(containerName, worktreeID)`. All methods accept `*db.ProjectRow` instead of container ID. |
| `settings.go`      | `GetSettings`, `UpdateSettings` (runtime, auditLogMode, disconnectKey, defaultProjectBudget, budgetAction{Warn,StopWorktrees,StopContainer,PreventStart}), `GetDefaultProjectBudget`, `getBudgetActions`, `parseFloat`/`formatFloat` helpers for DB serialization |
| `audit.go`         | `GetAuditLog` (filtered audit events by category/projectID/worktree/source/level/time), `GetAuditSummary` (aggregate stats: sessions, tools, prompts, cost, top tools), `PostAuditEvent` (add custom audit entry), `DeleteAuditEvents` (scoped deletion with query filters), `GetAuditProjects` (distinct project names), `WriteAuditCSV` (CSV export for compliance) |
| `host.go`          | `GetDefaults`, `ListDirectories`, `RevealInFileManager`, `ListRuntimes`                                 |

## internal/server/

HTTP server and API layer. Handlers are thin adapters: decode request → validate input → call `service.Service` → encode response.

| File             | Purpose                                                                                                                                                     |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `server.go`      | `Server` struct, `New(addr, svc, broker, termProxy)` constructor, `go:embed` for static frontend, `handleHealth`                                           |
| `doc.go`         | Package doc with swaggo general API info annotations (`@title`, `@version`, `@servers`, `@license`, `@externalDocs`)                                        |
| `errors.go`      | Error code constants (validation: `ErrCodeInvalidBody`, `ErrCodeNameTaken`, etc.; resource: `ErrCodeNotFound`, `ErrCodeNameTaken`; infrastructure: `ErrCodeNotConfigured`; server: `ErrCodeInternal`), `apiError` struct, `writeError` helper. All error responses include `{"error": "message", "code": "CODE"}` for machine-readable error handling. |
| `routes.go`      | `registerAPIRoutes()`, all API handlers (thin adapters over `service.Service`), input validation, JSON response helpers, SSE streaming, WebSocket terminal proxy with viewer counting. All handlers annotated with swaggo comments for OpenAPI spec generation. |
| `openapi_types.go` | Named request/response types for handlers that use anonymous structs — referenced by swag annotations only                                                |
| `routes_test.go` | API handler tests                                                                                                                                           |
| `debug_routes_test.go` | Tests for settings and event log endpoints                                                                                                             |
| `middleware.go`  | `corsMiddleware` (localhost-only CORS), `loggingMiddleware` (method/path/status/duration), `statusWriter`                                                   |

### API Endpoints

| Method | Path                                             | Handler                    | Description                                        |
| ------ | ------------------------------------------------ | -------------------------- | -------------------------------------------------- |
| GET    | `/api/v1/health`                                    | `handleHealth`             | Health check                                       |
| GET    | `/api/v1/projects`                                  | `handleListProjects`       | List projects from database with container state   |
| POST   | `/api/v1/projects`                                  | `handleAddProject`         | Add project to database (creates container)        |
| DELETE | `/api/v1/projects/{projectID}`                      | `handleRemoveProject`      | Remove project from database                       |
| DELETE | `/api/v1/projects/{projectID}/costs`                | `handleResetProjectCosts`  | Reset all cost history for a project               |
| DELETE | `/api/v1/projects/{projectID}/audit`                | `handlePurgeProjectAudit`  | Purge all audit events for a project               |
| POST   | `/api/v1/projects/{projectID}/stop`                 | `handleStopProject`        | Stop container                                     |
| POST   | `/api/v1/projects/{projectID}/restart`              | `handleRestartProject`     | Restart container                                  |
| GET    | `/api/v1/projects/{projectID}/worktrees`            | `handleListWorktrees`      | List worktrees with terminal state; skips batch exec when event store has terminal data |
| POST   | `/api/v1/projects/{projectID}/worktrees`            | `handleCreateWorktree`     | Create git worktree + connect terminal             |
| POST   | `/api/v1/projects/{projectID}/worktrees/{wid}/connect`     | `handleConnectTerminal`    | Start terminal for a worktree                      |
| POST   | `/api/v1/projects/{projectID}/worktrees/{wid}/disconnect`  | `handleDisconnectTerminal` | Close terminal WebSocket for a worktree            |
| DELETE | `/api/v1/projects/{projectID}/worktrees/{wid}`      | `handleRemoveWorktree`     | Fully remove worktree (kill abduco, git worktree remove, cleanup)                |
| GET    | `/api/v1/projects/{projectID}/ws/{wid}`             | `handleTerminalWebSocket`  | WebSocket terminal proxy — connects to abduco via docker exec |
| GET    | `/api/v1/projects/{projectID}/worktrees/{wid}/diff` | `handleGetWorktreeDiff`    | Uncommitted changes (tracked + untracked) as unified diff with per-file stats |
| POST   | `/api/v1/projects/{projectID}/worktrees/cleanup`    | `handleCleanupWorktrees`   | Remove orphaned worktree directories not tracked by git |
| GET    | `/api/v1/runtimes`                                  | `handleListRuntimes`       | Detect available container runtimes (Docker/Podman)|
| GET    | `/api/v1/settings`                                  | `handleGetSettings`        | Return server-side settings (runtime, auditLogMode, budget)|
| PUT    | `/api/v1/settings`                                  | `handleUpdateSettings`     | Update settings (runtime, auditLogMode, budget actions). Runtime requires restart; auditLogMode broadcasts to all containers |
| GET    | `/api/v1/audit`                                     | `handleGetAuditLog`        | Audit-relevant events with category/project/worktree/source/level/time filters |
| GET    | `/api/v1/audit/summary`                             | `handleGetAuditSummary`    | Aggregate audit stats (sessions, tools, prompts, cost, top tools) |
| GET    | `/api/v1/audit/projects`                            | `handleGetAuditProjects`   | Return distinct project names from audit log       |
| POST   | `/api/v1/audit`                                     | `handlePostAuditEvent`     | Add custom audit entry (for frontend or external logging)  |
| DELETE | `/api/v1/audit`                                     | `handleDeleteAuditEvents`  | Delete audit events (scoped with category/project/worktree/source/level filters) |
| GET    | `/api/v1/audit/export`                              | `handleExportAuditLog`     | Download audit events as CSV or JSON for compliance review |
| GET    | `/api/v1/filesystem/directories`                    | `handleListDirectories`    | List subdirectories at a path (filesystem browser) |
| POST   | `/api/v1/filesystem/reveal`                         | `handleRevealInFileManager`| Open a host directory in the system file manager (xdg-open/open/explorer) |
| GET    | `/api/v1/defaults`                                  | `handleDefaults`           | Server-resolved defaults for create container form |
| GET    | `/api/v1/events`                                    | `handleSSE`                | Server-Sent Events stream: worktree_state, project_state, budget_exceeded, budget_container_stopped, heartbeat |

## eventlog/

Centralized host-side event log for container and system events.

| File               | Purpose                                                                                                |
| ------------------ | ------------------------------------------------------------------------------------------------------ |
| `entry.go`         | `Entry` struct with timestamp, source (`SourceAgent`, `SourceBackend`, `SourceFrontend`, `SourceContainer`), `ProjectID` (deterministic hash of host path), `ContainerName` (snapshot at event time for display), worktree ID, action, details. `QueryFilters` for indexed queries (keyed by `ProjectID`). `ProjectRow` struct with `ProjectID`, `Name`, `HostPath`, `ContainerID`, `ContainerName`, and container config fields. `DisplayProject()` method for log display. Source constants used throughout logging. |
| `db.go`            | SQLite schema (projects keyed by `project_id`, events indexed by `project_id` + `container_name`, `session_costs` keyed by `project_id` + `session_id`), connection setup (WAL mode, pragmas), `openDB()` helper. Legacy `worktree_costs` table dropped. |
| `store.go`         | `Store` — SQLite-backed storage with concurrent safety, nil-receiver no-op. Methods: `Write()`, `Read()`, `Query(filters)`, `DistinctProjectIDs()` (replaces `DistinctContainers()`), `Clear()`, `Close()`, `InsertProject()`, `GetProject(projectID)`, `GetProjectByPath(hostPath)`, `ListAllProjects()`, `ListProjectIDs()` (replaces `ListProjectNames()`), `HasProject(projectID)`, `DeleteProject(projectID)`, `UpdateProjectContainer(projectID, containerID, containerName)`, `UpsertSessionCost(projectID, sessionID, ...)`, `GetProjectTotalCost(projectID)`, `GetAllProjectTotalCosts()`, `GetCostInTimeRange(projectID, ...)`, `DeleteProjectCosts(projectID)` |
| `slog_handler.go`  | Custom `slog.Handler` that tees backend slog records to the event log, enabling centralized debugging |
| `logger_test.go`   | Full test coverage for logger, query filters, distinct project IDs, nil safety, concurrent writes       |

## eventbus/

Push-based event system for container-to-host communication via file-based watcher.

| File               | Purpose                                                                                                |
| ------------------ | ------------------------------------------------------------------------------------------------------ |
| `types.go`         | Event types: `ContainerEvent` (from container hooks + lifecycle, carries `ProjectID`, `ContainerName`, and `SessionID`), `SSEEvent` (to frontend), `AttentionData`, `CostData`, `ToolUseData`, `ToolUseFailureData`, `StopFailureData`, `PermissionRequestData`, `SubagentData`, `ConfigChangeData`, `InstructionsLoadedData`, `TaskCompletedData`, `ElicitationData`, `TerminalConnectedData`, `SessionExitData`; all `Event*` constants for each hook type; `SSEBudgetExceeded` and `SSEBudgetContainerStopped` event types; `BudgetEventPayload` (shared struct: projectID, containerName, totalCost, budget) and `BudgetContainerStoppedPayload` (extends with containerId). Payloads include `projectId` for frontend routing. |
| `watcher.go`       | File-based event watcher — watches bind-mounted event directories for JSON event files using fsnotify (fast path) + polling every 2s (reliable fallback). Each project/container has a subdirectory at `<baseDir>/<containerName>/events/`. Reads files matching `<epoch_ns>-<pid>.json`, parses events (expecting `projectId` in JSON), dispatches to handler, deletes after processing. Cleans up orphaned `.tmp` files older than 30s. `NewWatcher(baseDir, handler, pollInterval)` constructor; `Start(ctx)` processes existing files (crash recovery), then runs fsnotify + polling loops; `WatchContainerDir`/`UnwatchContainerDir` for fsnotify registration; `CleanupContainerDir` drains remaining events and removes directory. |
| `watcher_test.go`  | Watcher tests: atomic file write detection, `.tmp` ignoring, invalid JSON cleanup, oversized file rejection, crash recovery (existing files), concurrent container directories, cleanup drains unprocessed events, shutdown without deadlock, Watch/Unwatch lifecycle, non-existent dir handling, fallback timestamp, missing required fields (including `projectId`) |
| `store.go`         | Thread-safe in-memory state store — per-project/container/worktree attention + cost + terminal lifecycle. `ProjectCost` type (TotalCost, MessageCount, IsEstimated, UpdatedAt). `StopCallbackFunc` signature: `(projectID, containerName, sessionID string, cost float64, isEstimated bool)` — single unified callback invoked on every stop event, responsible for both DB persistence and budget enforcement. `StaleCallbackFunc` signature: `(containerName string)` — called when a container stops sending heartbeats; the service layer resolves project context from the DB. Methods: `GetTerminalState`, `HasTerminalData`, `ActiveContainers`, `MarkContainerStale` (clears state and invokes `onStale` callback), `EvictWorktree` (removes cached state for a single worktree — called after removal/cleanup), `SetStopCallback(fn)`, `SetStaleCallback(fn)`, `broadcastBudgetEvent` (private helper shared by budget SSE broadcasts), `BroadcastBudgetExceeded` sends SSE event with projectID when budget exceeded, `BroadcastBudgetContainerStopped` sends SSE event with projectID + containerId after budget enforcement stops a container, `HandleEvent` (parses events and invokes callback with parsed `CostData` + projectID). `writeToAuditLog()` maps event types to agent/container source. |
| `store_test.go`    | Store tests: all event types, state isolation, concurrent access, unknown lookups, projectID routing                       |
| `liveness.go`      | Periodic liveness checker (runs every 15s) that checks container heartbeat staleness, marks containers stale after 30s of missing heartbeats. Audit logging is handled by the `onStale` callback (service layer), not by the liveness checker directly. |
| `broker.go`        | SSE broker — manages client subscriptions, fan-out broadcast, 15s heartbeat, slow-client drop, graceful shutdown |
| `broker_test.go`   | Broker tests: subscribe/unsubscribe, multi-client broadcast, slow client, heartbeat, shutdown           |

## runtime/

Container runtime detection (Docker/Podman/Windows named pipes).

| File              | Purpose                                                                                                        |
| ----------------- | -------------------------------------------------------------------------------------------------------------- |
| `detect.go`       | `Runtime` type (`docker`/`podman`), `DetectAvailable` (probe all runtimes), `SocketForRuntime` (first reachable socket), `probeSocket` (ping API supporting `unix://`, `tcp://`, `npipe://` schemes) |
| `sockets_linux.go` | Linux socket candidates: `/var/run/docker.sock`, `$XDG_RUNTIME_DIR/podman/podman.sock` (build-tagged `linux`) |
| `sockets_darwin.go` | macOS socket candidates: `~/.docker/run/docker.sock`, `~/.colima/default/docker.sock`, `~/.orbstack/run/docker.sock`, Podman machine (build-tagged `darwin`) |
| `sockets_windows.go` | Windows named pipe candidates: `//./pipe/docker_engine`, `//./pipe/podman-machine-default` (build-tagged `windows`) |
| `detect_test.go`  | Runtime detection tests                                                                                         |

## agent/

Agent status provider abstraction for extracting metrics from CLI agents running inside containers.

| File                          | Purpose                                                                                              |
| ----------------------------- | ---------------------------------------------------------------------------------------------------- |
| `types.go`                    | `Status`, `ModelInfo`, `TokenUsage` — agent-agnostic metric types                                   |
| `provider.go`                 | `StatusProvider` interface: `Name()`, `ConfigFilePath()`, `ExtractStatus([]byte) map[string]*Status` |
| `claudecode/provider.go`      | Claude Code implementation — reads `.claude.json`, parses per-project metrics keyed by workdir       |
| `claudecode/provider_test.go` | Tests for Claude Code provider: parsing, model mapping, multi-project, interface compliance          |

### Adding a new agent provider

Implement the `StatusProvider` interface (3 methods) and pass it to `engine.NewClient()`.

## engine/seccomp/

Embedded seccomp profile for container process isolation.

| File           | Purpose                                                                                                    |
| -------------- | ---------------------------------------------------------------------------------------------------------- |
| `profile.json` | Denylist-based seccomp profile (SCMP_ACT_ALLOW default). Blocks dangerous syscalls: kernel manipulation (kexec, reboot, modules), filesystem mounting (mount, pivot_root, new mount API), and security-sensitive operations (BPF, perf_event_open, userfaultfd, keyring). Supports x86_64 and aarch64. |
| `seccomp.go`   | `//go:embed profile.json`, exports `ProfileJSON() string` and `WriteProfileFile(dir) (string, error)` which writes the profile to disk for Docker/Podman `SecurityOpt` (file path reference). `Validate()` for build-time profile verification. |
| `seccomp_test.go` | Profile structure validation, architecture coverage, dangerous syscall denylist verification. |

## engine/

Container engine API wrapper (Docker/Podman/Windows).

| File                | Purpose                                                                                                                                                     |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `project_id.go`     | `ProjectID(hostPath) string` — deterministic 12-char hex hash of resolved absolute host path (SHA-256). `ValidProjectID(id) bool` validates format. |
| `project_id_test.go`| Tests for determinism, symlink normalization, trailing slashes, relative path rejection, validation |
| `types.go`          | `Client` interface, `Project` (incl. `ProjectID`, `HostPath`, `HasContainer` flag, `MountedDir`, `WorkspaceDir` for host↔container path mapping), `Worktree`, `ContainerConfig`, `CreateContainerRequest` types, `NetworkMode` enum (`full`/`restricted`/`none`), `ClaudeStatus`, `WorktreeState` (`connected`/`shell`/`background`/`disconnected`), `NotificationType` enums. Removed: `ContainerInfo`, `NotFound`. `ValidateInfrastructure(ctx, containerName) error` — checks for abduco, create-terminal.sh in container; `ListWorktrees` interface now accepts `skipEnrich ...bool`; `CreateWorktree(ctx, containerName, name, skipPermissions)` and `ConnectTerminal` methods return `(string, error)` (worktree ID) instead of struct |
| `client.go`         | `DockerClient` implementation: connects via socket path (Docker, Podman, Windows named pipes), `NewClient`, `ListProjects`, `StopProject`, `RestartProject`, `enrichProjectStatus` (worktree counts + claude status; cost overlaid at service layer from DB/event store), `checkClaudeStatus`, `checkIsGitRepo`, `execAndCapture`, `workspaceDir` (per-container workspace path resolver with `sync.Map` cache), `envValue` (extract env var from container config), `ContainerWorkspaceDir(name)` (computes `/home/dev/<name>`), `projectMountPaths(name, mounts)` (returns host source + container destination of workspace bind mount). Removed: `ListAllContainers`. `SetEventDir(dir)` configures the bind-mounted event directory path passed to containers via `WARDEN_EVENT_DIR`. |
| `agent_status.go`   | `ReadAgentStatus` (reads agent config via docker exec), `ReadAgentCostAndBillingType` (single-call cost + billing type extraction with per-session breakdown via `SessionCost` type), `IsEstimatedCost` (checks `oauthAccount.billingType` for subscription vs API key), `ProjectCostFromContainerStatuses` (sums cost filtered by workspace prefix), `sessionCostsFromStatuses` (extracts per-session costs keyed by agent session ID), `AgentCostResult` type. Cost is persisted to `session_costs` DB table keyed by `(projectID, sessionID)` (monotonically non-decreasing, upsert-safe); agent config is also read opportunistically when DB has no data and persisted on read. |
| `worktrees.go`      | `CreateWorktree`, `ListWorktrees` (accepts `skipEnrich` flag for event-driven mode), `ConnectTerminal` (validates worktree exists in git, auto-detects background state and reconnects), `DisconnectTerminal`, `RemoveWorktree` (kills abduco, prunes git metadata, tolerates already-removed worktrees, cleans up .warden-terminals/<id>/), `CleanupOrphanedWorktrees` (3-step: `pruneGitWorktrees` removes orphaned git entries, `cleanupOrphanedWorktreeDirs` removes .claude/worktrees/ dirs, `cleanupStaleTerminals` removes .warden-terminals/<id>/ with dead abduco OR missing worktree directory — kills orphaned abduco sessions first), `isAbducoSessionAlive`, `isGitWorktreeKnown`; worktree discovery: git porcelain (main + linked worktrees, prunable entries skipped, supports both `.worktrees/` and `.claude/worktrees/` paths) → `mergeTerminalWorktrees` (pre-git-create race) → `enrichWorktreeState` (terminal state from `.warden-terminals/`) |
| `worktrees_test.go` | Worktree parsing tests (`parseGitWorktreeList` incl. claude path + prunable skip, `parseTerminalBatch` incl. abduco alive/dead/backwards-compat, `isValidWorktreeID`, `worktreeIDFromPath`)    |
| `containers.go`     | `CreateContainer`, `DeleteContainer`, `InspectContainer`, `RecreateContainer`, `stopAndRemove` helper, `ensureImage`, `checkNameAvailable`, network mode env var injection (Docker labels removed). `buildSecurityConfig(networkMode, seccompPath)` returns CapDrop/CapAdd/SecurityOpt: drops ALL capabilities then re-adds `baseCapabilities` (CHOWN, DAC_OVERRIDE, FOWNER, FSETID, KILL, SETUID, SETGID, AUDIT_WRITE, NET_BIND_SERVICE, NET_RAW, SYS_CHROOT) plus conditional NET_ADMIN for restricted/none modes; applies `no-new-privileges` and seccomp profile (referenced by file path written to config dir at startup). All containers get a bind mount for the event directory at `/var/warden/events` (passed via `WARDEN_EVENT_DIR` env var). `CreateContainer` sets `WARDEN_PROJECT_ID=<projectID>` (deterministic hash of host path) and `WARDEN_WORKSPACE_DIR=/home/dev/<name>` and mounts the project at that path (unique per container). Calls `resolveSymlinksForMounts` before building bind mounts. |
| `containers_security_test.go` | Unit tests for `buildSecurityConfig` across all three network modes: verifies CapDrop ALL, base capabilities, conditional NET_ADMIN, dropped capabilities (SETPCAP, MKNOD, SETFCAP), no-new-privileges, and seccomp profile presence. |
| `mount_strategy.go` | `buildBindMounts(projectPath, containerWorkspaceDir, mounts)` — constructs bind mount strings. Workspace mount target is parameterized (no longer hardcoded to `/project`). |
| `symlink_resolver.go` | `resolveSymlinksForMounts` — walks each mount's host path and finds symlinks whose targets are outside the mounted directory tree, adding extra bind mounts for the resolved targets. Handles file symlinks (in-place host path replacement), directory symlinks (extra directory mount), nested chains, broken/circular symlinks (skipped gracefully). Used by `CreateContainer` to ensure symlink-managed dotfiles (e.g. `~/.claude` via Nix Home Manager) resolve correctly inside containers. |
| `symlink_resolver_test.go` | Tests for symlink resolution: file symlinks, directory symlinks, nested chains, circular links, broken links, non-symlink passthrough |

### Key Constants and Environment Variables

**Container Environment Variables (set by `CreateContainer`):**
- `WARDEN_WORKSPACE_DIR` — container-side workspace path, defaults to `/home/dev/<name>`. All paths (terminals dir, worktrees) are derived from this. Shell scripts use `${WARDEN_WORKSPACE_DIR:-/project}` for backward compat with legacy containers.
- `WARDEN_PROJECT_ID` — deterministic project identifier (12-char hex SHA-256 of resolved absolute host path). Set on every container at creation. Used by event-posting scripts to tag events with project identity for proper routing across container rebuilds.
- `WARDEN_EVENT_DIR` — bind-mounted event directory path (`/var/warden/events`), for hook scripts to write events
- `WARDEN_NETWORK_MODE` — network isolation mode (`full`/`restricted`/`none`)
- `WARDEN_ALLOWED_DOMAINS` — comma-separated domain list for `restricted` mode

**Script Paths:**
- `terminalsDirSuffix = "/.warden-terminals"` — appended to workspace dir for terminal tracking
- `createTerminalScript = "/usr/local/bin/create-terminal.sh"` — in-container script to initialize abduco session for a worktree
- `disconnectTerminalScript = "/usr/local/bin/disconnect-terminal.sh"` — in-container script to kill abduco session

## internal/tui/

Terminal user interface using Bubble Tea v2. Written against the `Client` interface — serves as both a product and a reference implementation for Go developers. Features tabbed navigation (Projects, Settings, Audit), project management with worktree detail view, container create/edit forms, and real-time event monitoring.

### Architecture

- **Tab-based navigation** — Three top-level tabs: Projects (default), Settings, Audit. Tab switching via `[1]`, `[2]`, `[3]` keys.
- **Project view** → worktree detail flow — Click a project to see its worktrees (bubbles/list with custom delegate for colored state dots). Worktree detail modal for creating new worktrees.
- **Forms** — Container creation/edit uses native bubbles components (textinput, textarea, directory browser). Full field set: name, path, skip permissions, network mode, allowed domains, and an Advanced collapsible section with image, bind mounts, and environment variables.
- **Terminal passthrough** — Pressing `enter` on a connected worktree runs `tea.Exec()` to yield the terminal to the remote PTY. User presses configured disconnect key (default `ctrl+\`) to return to Warden.
- **ANSI basic 16 colors** — All colors use terminal palette constants (`lipgloss.BrightBlue`, `lipgloss.Red`, etc.) so the TUI inherits the user's terminal theme. Single source of truth in `components/colors.go`.
- **Real-time updates** — SSE subscription in `NewApp()` (not `Init()`) broadcasts worktree state and cost changes to all views via `SSEEventMsg`.
- **Settings view** — 3-way cycle for `auditLogMode`: off → standard → detailed. Enables audit log viewing with appropriate event filtering (Standard shows session/budget/system only; Detailed shows all audit categories).

### Views (implement `View` interface)

| View                | Purpose                                                                                   |
| ------------------- | ----------------------------------------------------------------------------------------- |
| `ProjectsView`      | Full-width table (`bubbles/table`) with columns: status dot, project name, runtime, worktree count, cost. Row selection highlight. Actions: open (detail), edit (or create container for no-container projects), toggle (start/stop container), new, manage (opens ManageProjectView), refresh. Shows "no container" status for projects without containers. |
| `ManageProjectView` | Inline overlay dialog for project management with four independent checkboxes: Remove from Warden, Delete container, Reset cost history, Purge audit history. Purge requires typing "purge" to confirm. Execution order: container → parallel data cleanup → project removal. Handles OperationResultMsg and navigates back on completion. |
| `ProjectDetailView` | Worktree list using `bubbles/list` with custom `worktreeDelegate` for 3-line items (title, branch, colored status). Actions: connect (terminal passthrough), disconnect, kill, remove, new (dialog), cleanup. |
| `ContainerFormView` | Create or edit container. Full field set matching the webapp: name, path (directory browser), skip permissions, network mode, allowed domains, and collapsible Advanced section with image, bind mounts (auto-populated from server defaults), and environment variables. Split across three files for readability. |
| `SettingsView`      | Settings menu (cursor navigation j/k, enter to activate): event log toggle, runtime selector (cycles through available runtimes), disconnect key capture (waits for `ctrl+<key>` press), default budget input, budget enforcement action toggles (warn, stop worktrees, stop container, prevent restart). |
| `EventLogView`      | Event log entries filtered by level (info/warn/error) and source (agent/backend/frontend/container). Entries displayed in reverse chronological order. Actions: level filter, source filter, refresh, clear. |
| `AuditLogView`      | Audit log viewer with full filtering: category (session/agent/prompt/config/budget/system/debug), level (info/warn/error), source, project, time range (1h/6h/24h/7d/30d). Auto-refresh (10s/30s/1m/5m). Scoped delete with type-to-confirm ("delete") overlay. Summary shows sessions, tools, prompts, cost, projects, worktrees. |

### Key Components

| File                       | Purpose                                                                                                                                                                |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `client.go`                | `Client` interface — the key architectural boundary. Each method maps 1:1 to an API endpoint with GoDoc comment. Methods: list/add/remove projects, reset costs, purge audit, list/create/connect worktrees, terminal operations, settings, event log, defaults, event subscription. |
| `adapter.go`               | `ServiceAdapter` wraps `warden.App` to satisfy `Client` for embedded mode (TUI binary). Most methods delegate to `app.Service`; `AttachTerminal` uses docker exec; `SubscribeEvents` uses `app.Broker.Subscribe()`. Includes `ResetProjectCosts` and `PurgeProjectAudit` for management dialog. Enables TUI and external Go clients to use the same interface. |
| `app.go`                   | Root `tea.Model` — manages four tabs, active view routing, SSE event subscription (started in `NewApp()`), global key handling (quit, help, tab switching). Delegates `Update()` and `Render()` to active view. Tracks `auditLogMode` setting and `disconnectKey`. |
| `common.go`                | Message types: `ProjectsLoadedMsg`, `WorktreesLoadedMsg`, `SettingsLoadedMsg`, `RuntimesLoadedMsg`, `EventLogLoadedMsg`, `AuditLogLoadedMsg`, `AuditProjectsLoadedMsg`, `DefaultsLoadedMsg`, `OperationResultMsg`, `NavigateMsg`, `NavigateBackMsg`, `SSEEventMsg`, `EventStreamClosedMsg`. `Tab` enum: `TabProjects`, `TabSettings`, `TabEventLog`, `TabAuditLog`. |
| `keymap.go`                | Key binding definitions using `bubbles/key`. Keymaps: `GlobalKeyMap` (quit, help, [1]/[2]/[3]/[4] tab switching), `ProjectKeyMap` (enter open, e edit, s start/stop toggle, x manage, n new, R refresh), `WorktreeKeyMap` (enter connect, d disconnect, X kill, x remove, n new, c cleanup, esc/backspace back), `SettingsKeyMap` (enter/space toggle), `AuditLogKeyMap` (c category filter, f project/worktree filter, R refresh), `ManageKeyMap` (space toggle, enter confirm, esc cancel, j/k navigate). |
| `render.go`                | Shared rendering helpers: `padRight`, `truncate`.                                                                                                                     |
| `terminal.go`              | `TerminalExecCmd` struct — bridges stdin/stdout to `client.TerminalConnection` for terminal passthrough mode. Handles raw terminal mode, SIGWINCH resize, and graceful close on disconnect key press. Satisfies `tea.ExecCmd` interface used by `tea.Exec()`. |
| `theme.go`                 | Lip Gloss v2 styles — imports colors from `components/colors.go` (single source of truth). Defines `Styles` struct with layout, text, and component styles. `helpStyles()` for bubbles help bar. |
| `validate.go`              | `ValidateWorktreeName` — git branch name validation rules ported from webapp (no spaces, special chars, etc.). |
| `view_projects.go`         | `ProjectsView` — project list via `bubbles/table`. Displays status dot (container state), project name, runtime, worktree count, cost. Actions: open (navigate to detail), edit (form, or create container for no-container projects), toggle (start/stop), manage (opens ManageProjectView), new, refresh. Shows "no container" for projects without containers. |
| `view_manage_project.go`  | `ManageProjectView` — inline overlay with four toggle checkboxes (remove, delete container, reset costs, purge audit), type-to-confirm for audit purge, ordered execution with concurrent data cleanup, and OperationResultMsg handling. |
| `view_project_detail.go`   | `ProjectDetailView` — worktree list using `bubbles/list` with custom `worktreeDelegate` for multi-line rendering (name, branch, colored state dot). Actions: connect (terminal passthrough via `tea.Exec`), disconnect, kill, remove, new (shows modal form with name input + validation), cleanup. Terminal passthrough yields to `TerminalExecCmd`, returns on disconnect key. |
| `view_container_form.go`   | `ContainerFormView` — state, Update, key handling. Fields: name, path, skip perms, network, domains, Advanced (image, mounts, env vars). Mount/env inline editing with enter=save, esc=cancel. |
| `view_container_form_render.go` | Render logic: `buildFieldLines()` with cursor tracking for scroll-to-cursor, `fieldView()`, `appendListSection()` (shared mount/env renderer), helper functions (`cursorPrefix`, `textInputView`, `boolSelector`, `orEmpty`, `roLabel`). |
| `view_container_form_help.go`  | Help keymaps for form modal states: `FormKeyMap`, `browsingHelpKeyMap`, `editingHelpKeyMap`, `inlineEditHelpKeyMap`, `formSelectionKeyMap`, `formWithRemoveKeyMap`. All `key.NewBinding` calls hoisted to package-level vars. |
| `view_settings.go`         | `SettingsView` — settings menu. Cursor navigation (j/k), actions (enter/space). Items: event log toggle (enabled/disabled status), runtime selector (cycles through available runtimes, shows restart warning after change), disconnect key capture (waits for `ctrl+<key>` press, `esc` cancels), default budget text input (enter to edit, enter to save, esc to cancel), budget enforcement action toggles (warn, stop worktrees, stop container, prevent restart). |
| `view_eventlog.go`         | `EventLogView` — event log viewer (tab [3]). Fetches audit entries with optional filters (level, source). Displays entries in reverse chronological order with timestamps. Actions: cycle level filter (l key), cycle source filter (s key), refresh (R key), clear (C key). Visible only when auditLogMode is Standard or Detailed. |
| `view_auditlog.go`         | `AuditLogView` — audit log viewer (tab [4]). Full filter set: category (c), level (l), source (s), project (p), time range (t), auto-refresh (a), refresh (R), scoped clear with type-to-confirm (C). Fetches via `GetAuditLog`, `GetAuditSummary`, `GetAuditProjects`. Summary shows sessions, tools, prompts, cost, projects, worktrees. Auto-refresh uses `tea.Tick`. |
| `components/status_dot.go` | Maps `WorktreeState`, `NotificationType`, and container state to styled unicode dot characters. Exports `FormatStatusDot()` function. Defines state→color mapping (connected=green, shell=amber, background=purple, disconnected=gray, working=pulsing, etc.). |
| `components/cost.go`       | `FormatCost(cents int64) string` and `FormatDuration(d time.Duration) string` — ported from `web/src/lib/cost.ts`. |
| `components/tab_bar.go`    | Renders horizontal tab bar with active tab highlighted, inactive tabs muted. Used in the main UI header. |
| `components/colors.go`    | Single source of truth for ANSI basic 16 color palette. Exported vars (`ColorAccent`, `ColorGray`, etc.) used by both components and parent `tui` package. |
| `components/directory_browser.go` | `DirectoryBrowser` — navigable filesystem tree with scrolling support. `SetHeight()` constrains visible rows, auto-adjusts scroll offset to keep cursor visible. Loads directories via `Client.ListDirectories()`. |

### Key Constants and Enums

- `Tab` enum: `TabProjects` (0), `TabSettings` (1), `TabEventLog` (2), `TabAuditLog` (3).
- `TabLabels` map: maps `Tab` to display string ("Projects", "Settings", "Event Log", "Audit Log").
- **Keybindings**: `[1]` = Projects, `[2]` = Settings, `[3]` = Event Log, `[4]` = Audit Log.
- **Standardized action keys**: `n` = new, `x` = remove, `X` = kill, `j`/`k` = navigate (handled by bubbles components).
- **Disconnect key** — configurable in settings, defaults to `config.DefaultDisconnectKey` (typically `"ctrl+\\"`). Used by `TerminalExecCmd` to detect user intent to return from terminal passthrough.

### Tests

| File                       | Purpose                                                                                 |
| -------------------------- | --------------------------------------------------------------------------------------- |
| `validate_test.go`         | Worktree name validation tests — all git branch naming rules (spaces, special chars). |
| `adapter_test.go`          | Interface compliance tests for `ServiceAdapter`.                                        |
| `client_compliance_test.go`| Compile-time check: `client.Client` satisfies `tui.Client`.                            |
| `view_container_form_test.go` | Form logic: field visibility rules, cursor navigation, mount/env sub-cursor movement, add/remove items, submit validation, sensitive key detection, field view rendering. |
| `components/status_dot_test.go` | All worktree states × notification types → correct styled output.                 |
| `components/cost_test.go`  | Cost and duration formatting tests ported from frontend.                               |
| `components/directory_browser_test.go` | Height clamping, entry loading, error handling, keyboard navigation, scroll offset tracking, scroll indicators. |
| `components/tab_bar_test.go` | Tab bar plain text output verification with ANSI stripping.                          |

## internal/terminal/

WebSocket-to-PTY proxy for terminal connections.

| File                | Purpose                                                                                                    |
| ------------------- | ------------------------------------------ |
| `proxy.go`          | `Proxy` type with `ServeWS()` — upgrades HTTP to WebSocket, creates docker exec with TTY attached to abduco, bridges bytes bidirectionally. Handles resize via text frames, ping/pong heartbeat (30s), graceful close. PTY output is filtered through `AltScreenFilter` to enable scrollback. The HTTP handler (`handleTerminalWS`) lives in `internal/server/routes.go`. |
| `altscreen.go`      | `AltScreenFilter` — an `io.Reader` wrapper that strips alternate screen escape sequences (DECSET/DECRST 47, 1047, 1049) from the PTY output stream. Forces applications like Claude Code (via Ink) to render in the normal buffer where xterm.js scrollback works. Handles combined DECSET params and sequences split across read boundaries. |
| `proxy_test.go`     | Tests for bidirectional data, resize, browser disconnect, PTY exit, malformed messages, exec errors, alt-screen stripping |
| `altscreen_test.go` | Comprehensive tests for alt-screen filter: passthrough, simple/combined stripping, split boundaries, large throughput |

## db/

Persistent storage layer using SQLite for projects, settings, and audit event logging.

| File            | Purpose                                                                                                                                                                                                                      |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `store.go`      | `Store` struct wrapping SQLite connection, `New(dbPath)` constructor, project operations (`ListProjects`, `HasProject`, `AddProject`, `RemoveProject`), settings operations (`GetSetting`, `SetSetting` for runtime/auditLogMode/disconnectKey/defaultProjectBudget/budgetAction*), session cost operations (`UpsertSessionCost` — keyed by session ID, monotonically non-decreasing so upsert always safe, `GetProjectTotalCost(container)` — single-container query for budget checks, `GetAllProjectTotalCosts` → `map[string]ProjectCostRow` summing across sessions with legacy `worktree_costs` fallback, `GetCostInTimeRange(container, since, until)` — time-filtered cost via session overlap (created_at..updated_at), `DeleteProjectCosts` — cleanup when audit logging is off), audit operations (`QueryAuditSummary`, `QueryTopTools`), `ProjectCostRow` type (TotalCost, IsEstimated) |
| `audit_writer.go` | `AuditWriter` — enforces mode-based filtering before writing events to the `events` table. Methods: `Write(entry *Entry, mode AuditLogMode)` applies mode filtering via a `standardEvents` allowlist (only allows session/budget/system categories in Standard mode), then calls `store.Write()`. The writer is the only permitted write path for audit events. |
| `db.go`         | SQLite schema: `projects` table (name, added_at, image, project_path, env_vars, mounts, original_mounts, skip_permissions, network_mode, allowed_domains, cost_budget), `settings` table (key, value), `events` table (audit events with category, source, level), `session_costs` table (container, session_id, cost, is_estimated, created_at, updated_at), legacy `worktree_costs` table. Additive migrations via ALTER TABLE (idempotent). |

### Database Location

- Linux: `$XDG_CONFIG_HOME/warden/warden.db`
- macOS: `~/Library/Application Support/warden/warden.db`
- Windows: `%AppData%/warden/warden.db`
