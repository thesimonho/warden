# Supporting Packages

Smaller backend packages that don't warrant individual files.

## Entry Points

- `warden.go` — top-level `Warden` type: `New(Options)` wires SQLite database, runtime detection, engine client, event bus pipeline, and service layer into a ready-to-use handle; `Close()` shuts down all subsystems (idempotent via `sync.Once`). Fields: `Service` (primary interface for all operations), `Broker` (SSE event bus), `Engine` (container runtime client, advanced use), `DB` (SQLite store, advanced use), `Watcher` (file-based event watcher, advanced use). All container/worktree operations are accessed through `w.Service`. Session watcher lifecycle is now managed by the service layer: `service.StartSessionWatcher()` and `service.StopSessionWatcher()` handle JSONL session file monitoring. This is the primary entry point for Go library consumers.

| Binary | File | Purpose |
| --- | --- | --- |
| `warden` | `cmd/warden/main.go` | Headless engine server: creates `*Warden`, starts HTTP API server, blocks until SIGTERM/SIGINT |
| `warden-desktop` | `cmd/warden-desktop/main.go` | Web dashboard: creates `*Warden`, wires terminal proxy and HTTP server with embedded SPA, opens browser |
| `warden-desktop` | `cmd/warden-desktop/run.go` | Server lifecycle: start, wait for ready, open browser, signal handling, graceful shutdown |
| `warden-tui` | `cmd/warden-tui/main.go` | TUI binary: creates `*Warden`, wraps in `ServiceAdapter`, runs Bubble Tea program |

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
| `registry.go` | `Registry` (agent type → StatusProvider resolver), constants: `ClaudeCode`, `Codex`, `DefaultAgentType`; `AllTypes` (slice of agent type IDs in display order), `DisplayLabels` (map of agent type ID → human-readable label), `ShortLabel()` (compact label for UI); methods: `Register(name, provider)`, `Get(name)`, `Default()`, `Resolve(agentType)` (fallback to default) |
| `types.go` | `Status`, `ModelInfo`, `TokenUsage` — agent-agnostic metric types; `ParsedEventType` constants (`EventSessionStart/End`, `EventToolUse`, `EventToolUseFailure`, `EventStopFailure`, `EventUserPrompt`, `EventTurnComplete/Duration`, `EventTokenUpdate`, `EventPermissionRequest`, `EventElicitation`, `EventSubagentStop`, `EventApiMetrics`, `EventPermissionGrant`, `EventContextCompact`, `EventSystemInfo`); `ParsedEvent` (Type, SessionID, Timestamp, Model, ToolName, ToolInput, Prompt, DurationMs, ErrorContent, ServerName, Tokens, EstimatedCostUSD, GitBranch, WorktreeID, Subtype, Content, Commands, TTFTMs, OutputTokensPerSec, CompactTrigger, PreCompactTokens); `ProjectInfo` (ProjectID, WorkspaceDir, ProjectName) |
| `parser.go` | `SessionParser` interface: `ParseLine([]byte) []ParsedEvent` (parse JSONL line), `SessionDir(homeDir, ProjectInfo) string` (host-side session file path), `FindSessionFiles(homeDir, ProjectInfo) []string` (discover active session files for a project) |
| `session_watcher.go` | `SessionWatcher` (monitors JSONL session directory via polling every 2s, tails new lines from multiple concurrent session files, handles file rotation via `tailedFiles` map). Reads files from the start (no seek-to-EOF) for crash recovery — duplicate events are deduped by the audit DB's `source_id` unique index. Stamps `SourceLine` (raw JSONL bytes) and `SourceIndex` (event ordinal) on each parsed `ParsedEvent` before invoking the callback. `FindSessionFiles()` globs worktree session dirs (per-agent discovery logic). Lifecycle: `Start(ctx)`, `Stop()`. |
| `strings.go` | `MaxToolInputLength` (1000), `MaxPromptLength` (500), `TruncateString(s, maxLen)` — text truncation with ASCII fast path, `WorktreeIDFromCWD(cwd)` — extracts worktree ID from container-side paths |
| `validate.go` | `ParseAllEvents(parser, reader) []ParsedEvent` — parses all JSONL lines and returns events (shared test helper); `ValidateJSONL(parser SessionParser, reader io.Reader) *ValidationResult` — validates JSONL lines from a reader and returns event counts; `ValidationResult` type with `Require(eventType, minCount)` and `Check()` assertion methods. Used by parser tests and CI validation workflows. |
| `provider.go` | `StatusProvider` interface: `Name()`, `ProcessName()` (CLI binary name for pgrep), `ConfigFilePath()`, `ExtractStatus([]byte) map[string]*Status`, `NewSessionParser() SessionParser` |
| `claudecode/provider.go` | Claude Code implementation — reads `.claude.json`, parses per-workspace metrics; implements `Name()`, `ProcessName()`, `ConfigFilePath()`, `ExtractStatus()`, `NewSessionParser()` |
| `claudecode/parser.go` | Claude Code `Parser` — implements `SessionParser`, stateful token/cost accumulation per session, parses all 14 system subtypes (turn_duration, api_error, agents_killed, api_metrics, permission_retry, compact_boundary, microcompact_boundary, stop_hook_summary, away_summary, memory_saved, bridge_status, local_command, informational, scheduled_task_fire), `FindSessionFiles()` scans per-project session directory for `.jsonl` files. See `docs/events_claude.md` (gitignored) for source-verified catalog. |
| `claudecode/jsonl_types.go` | `SessionEntry` (with `TTFTMs`, `OutputTokensPerSec`, `Commands`, `CompactMetadata` fields for system subtypes), `MessageBody`, `ContentField`, `ContentBlock` (polymorphic content), `UsageInfo`, `CompactMetadata` — JSONL line types |
| `claudecode/jsonl_unmarshal.go` | Polymorphic unmarshaling for content field (assistant, user, thinking blocks) |
| `claudecode/pricing.go` | `EstimateCost(tokens TokenUsage) float64` — per-model token pricing lookup |
| `claudecode/provider_test.go` | Tests for Claude Code provider: parsing, model mapping, multi-project, interface compliance, optional `TestValidateLive` for CI validation (env-gated via `VALIDATE_JSONL`) |
| `codex/provider.go` | Codex implementation — no config file, cost from JSONL only; implements `Name()`, `ProcessName()`, `ExtractStatus()`, `NewSessionParser()` |
| `codex/parser.go` | Codex `Parser` — implements `SessionParser`, parses session_meta, response_item (function_call, function_call_output, local_shell_call, web_search_call, custom_tool_call, image_generation_call, tool_search_call), event_msg (user_message, token_count, task_complete/turn_complete, turn_aborted, context_compacted, thread_rolled_back, error, exec_command_end, mcp_tool_call_end, patch_apply_end), and top-level compacted entries. Clears toolNames map on turn_complete. Codex persistence policy filters events — many event_msg types are never written in limited mode (CLI default); extended mode requires `codex app-server`. See `docs/events_codex.md` (gitignored) for full persistence policy. `FindSessionFiles()` discovers sessions via shell snapshots at `~/.codex/shell_snapshots/` (reads WARDEN_PROJECT_ID env var to filter), globs for matching `.jsonl` files across date directories. |
| `codex/jsonl_types.go` | `RolloutItem`, `SessionMeta`, `TurnContext`, `ResponseItem` (with `Action *ActionPayload`, `Input` for custom tools), `ActionPayload` (shell command + web search query), `CompactedItem`, `EventMsg` (with `NumTurns` for thread_rolled_back), `TokenCountInfo`, `RateLimits` — JSONL line types matching Codex format |
| `codex/pricing.go` | `EstimateCost(model string, usage TokenUsage) float64` — OpenAI model pricing (gpt-5.4, gpt-5.4-mini, gpt-5.3-codex, claude-3-5-sonnet via OpenAI API) |
| `codex/parser_test.go` | Tests for Codex parser: fixture parsing, session start/response items, token accumulation, model mapping, interface compliance, uses `ValidateJSONL` for event count verification, optional `TestValidateLive` for CI validation (env-gated via `VALIDATE_JSONL`) |
| `codex/pricing_test.go` | Tests for Codex pricing: model lookup, token cost calculation |
| `types.go` — Agent display | Constants updated: "Claude exited" → "Agent exited" in worktree state comments |

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
