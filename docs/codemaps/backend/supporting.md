# Supporting Packages

Smaller backend packages that don't warrant individual files.

## Entry Points

- `warden.go` — top-level `App` type: `New(Options)` wires SQLite database, runtime detection, engine client, event bus pipeline, and service layer into a ready-to-use handle; `Close()` shuts down all subsystems (idempotent via `sync.Once`). Convenience methods (all take project ID and call `resolveProject(id)` to fetch `*db.ProjectRow` before delegating to service): `CreateProject(ctx, name, hostPath, opts)`, `DeleteProject(ctx, projectID)`, `StopProject(ctx, projectID)`, `RestartProject(ctx, projectID)`, `StopAll(ctx)` (pre-loads all project rows to avoid N+1), `RestartWorktree(ctx, projectID, worktreeID)`, `GetProjectStatus(ctx, projectName)`. Returns typed result structs (`ProjectResult`, `WorktreeResult`, `ContainerResult`). Session watcher lifecycle: `StartSessionWatcher(projectID, containerName, agentType, workspaceDir)` creates and starts a JSONL watcher (called by `CreateProject` after container creation), `StopSessionWatcher(projectID)` stops and removes the watcher (called by `DeleteProject`). Field: `sessionWatchers map[string]*agent.SessionWatcher`. This is the primary entry point for Go library consumers.

| Binary | File | Purpose |
| --- | --- | --- |
| `warden` | `cmd/warden/main.go` | Headless engine server: creates `warden.App`, starts HTTP API server, blocks until SIGTERM/SIGINT |
| `warden-desktop` | `cmd/warden-desktop/main.go` | Web dashboard: creates `warden.App`, wires terminal proxy and HTTP server with embedded SPA, opens browser |
| `warden-desktop` | `cmd/warden-desktop/run.go` | Server lifecycle: start, wait for ready, open browser, signal handling, graceful shutdown |
| `warden-tui` | `cmd/warden-tui/main.go` | TUI binary: creates `warden.App`, wraps in `ServiceAdapter`, runs Bubble Tea program |

## access/

Public Go package for general-purpose access item management. Pure library — no dependencies on service/db/engine.

| File | Purpose |
| --- | --- |
| `types.go` | `Item` (user-created access item: ID, Label, Description, Method, Credentials JSON), `Credential`, `Source`, `Transform`, `Injection`, `Method` (enum), `DetectionResult` (Available, HostPathResolved), `AccessItemResponse` (Item + DetectionResult) |
| `resolve.go` | `Resolve(item)` (single-item resolution), `Detect(item)` (single item detection), `trySource` (internal, tests one source) |
| `builtin.go` | `BuiltInGit`, `BuiltInSSH`, `BuiltInItems` (slice), `BuiltInItemByID(id)` (lookup), `IsBuiltInID(id)` (predicate) |

## agent/

Multi-agent abstraction for status extraction, session parsing, and event translation.

| File | Purpose |
| --- | --- |
| `registry.go` | `Registry` (agent type → StatusProvider resolver), constants: `ClaudeCode`, `Codex`, `DefaultAgentType`; methods: `Register(name, provider)`, `Get(name)`, `Default()`, `Resolve(agentType)` (fallback to default) |
| `types.go` | `Status`, `ModelInfo`, `TokenUsage` — agent-agnostic metric types; `ParsedEventType` constants (`EventSessionStart/End`, `EventToolUse`, `EventUserPrompt`, `EventTurnComplete/Duration`, `EventTokenUpdate`); `ParsedEvent` (Type, SessionID, Timestamp, Model, ToolName, ToolInput, Prompt, DurationMs, Tokens, EstimatedCostUSD, GitBranch, WorktreeID); `ProjectInfo` (WorkspaceDir, ProjectName) |
| `parser.go` | `SessionParser` interface: `ParseLine([]byte) []ParsedEvent` (parse JSONL line), `SessionDir(homeDir, ProjectInfo) string` (host-side session file path) |
| `session_watcher.go` | `SessionWatcher` (monitors JSONL session directory, tails new lines, handles file rotation via fsnotify + polling fallback, feeds parsed events to callback), lifecycle: `Start(ctx)`, `Stop()` |
| `provider.go` | `StatusProvider` interface: `Name()`, `ProcessName()` (CLI binary name for pgrep), `ConfigFilePath()`, `ExtractStatus([]byte) map[string]*Status`, `NewSessionParser() SessionParser` |
| `claudecode/provider.go` | Claude Code implementation — reads `.claude.json`, parses per-workspace metrics; implements `Name()`, `ProcessName()`, `ConfigFilePath()`, `ExtractStatus()`, `NewSessionParser()` |
| `claudecode/parser.go` | Claude Code `Parser` — implements `SessionParser`, stateful token/cost accumulation per session, parses lines via `jsonl_unmarshal.go` |
| `claudecode/jsonl_types.go` | `SessionEntry`, `MessageBody`, `ContentField`, `ContentBlock` (polymorphic content), `UsageInfo` — JSONL line types |
| `claudecode/jsonl_unmarshal.go` | Polymorphic unmarshaling for content field (assistant, user, thinking blocks) |
| `claudecode/pricing.go` | `EstimateCost(tokens TokenUsage) float64` — per-model token pricing lookup |
| `claudecode/provider_test.go` | Tests for Claude Code provider: parsing, model mapping, multi-project, interface compliance |
| `codex/provider.go` | Codex implementation — no config file, cost from JSONL only; implements `Name()`, `ProcessName()`, `ExtractStatus()`, `NewSessionParser()` |
| `codex/parser.go` | Codex `Parser` — implements `SessionParser`, parses lines via `jsonl_unmarshal.go`, maps session_meta/response_item/event_msg to `ParsedEvent` |
| `codex/jsonl_types.go` | `RolloutItem`, `SessionMeta`, `TurnContext`, `ResponseItem`, `EventMsg`, `TokenCountInfo`, `RateLimits` — JSONL line types matching Codex format |
| `codex/pricing.go` | `EstimateCost(model string, usage TokenUsage) float64` — OpenAI model pricing (gpt-5.4, gpt-5.4-mini, gpt-5.3-codex, claude-3-5-sonnet via OpenAI API) |
| `codex/parser_test.go` | Tests for Codex parser: fixture parsing, session start/response items, token accumulation, model mapping, interface compliance |
| `codex/pricing_test.go` | Tests for Codex pricing: model lookup, token cost calculation |

## runtime/

Container runtime detection (Docker/Podman/Windows named pipes).

| File | Purpose |
| --- | --- |
| `detect.go` | `Runtime` type (`docker`/`podman`), `DetectAvailable` (probe all runtimes), `SocketForRuntime` (first reachable socket), `probeSocket` (ping API supporting `unix://`, `tcp://`, `npipe://` schemes) |
| `sockets_linux.go` | Linux socket candidates: `/var/run/docker.sock`, `$XDG_RUNTIME_DIR/podman/podman.sock` (build-tagged `linux`) |
| `sockets_darwin.go` | macOS socket candidates: `~/.docker/run/docker.sock`, `~/.colima/default/docker.sock`, `~/.orbstack/run/docker.sock`, Podman machine (build-tagged `darwin`) |
| `sockets_windows.go` | Windows named pipe candidates: `//./pipe/docker_engine`, `//./pipe/podman-machine-default` (build-tagged `windows`) |
| `detect_test.go` | Runtime detection tests |

## internal/terminal/

WebSocket-to-PTY proxy for terminal connections.

| File | Purpose |
| --- | --- |
| `proxy.go` | `Proxy` type with `ServeWS()` — upgrades HTTP to WebSocket, creates docker exec with TTY attached to abduco, bridges bytes bidirectionally. Handles resize via text frames, ping/pong heartbeat (30s), graceful close. PTY output is filtered through `AltScreenFilter` to enable scrollback. The HTTP handler (`handleTerminalWS`) lives in `internal/server/routes.go`. |
| `altscreen.go` | `AltScreenFilter` — an `io.Reader` wrapper that strips alternate screen escape sequences (DECSET/DECRST 47, 1047, 1049) from the PTY output stream. Forces applications like Claude Code (via Ink) to render in the normal buffer where xterm.js scrollback works. Handles combined DECSET params and sequences split across read boundaries. |
| `proxy_test.go` | Tests for bidirectional data, resize, browser disconnect, PTY exit, malformed messages, exec errors, alt-screen stripping |
| `altscreen_test.go` | Comprehensive tests for alt-screen filter: passthrough, simple/combined stripping, split boundaries, large throughput |
