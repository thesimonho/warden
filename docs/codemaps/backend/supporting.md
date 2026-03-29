# Supporting Packages

Smaller backend packages that don't warrant individual files.

## Entry Points

- `warden.go` — top-level `App` type: `New(Options)` wires SQLite database, runtime detection, engine client, event bus pipeline, and service layer into a ready-to-use handle; `Close()` shuts down all subsystems (idempotent via `sync.Once`). Convenience methods (all take project ID and call `resolveProject(id)` to fetch `*db.ProjectRow` before delegating to service): `CreateProject(ctx, name, hostPath, opts)`, `DeleteProject(ctx, projectID)`, `StopProject(ctx, projectID)`, `RestartProject(ctx, projectID)`, `StopAll(ctx)` (pre-loads all project rows to avoid N+1), `RestartWorktree(ctx, projectID, worktreeID)`, `GetProjectStatus(ctx, projectName)`. Returns typed result structs (`ProjectResult`, `WorktreeResult`, `ContainerResult`). This is the primary entry point for Go library consumers.

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

Agent status provider abstraction for extracting metrics from CLI agents running inside containers.

| File | Purpose |
| --- | --- |
| `types.go` | `Status`, `ModelInfo`, `TokenUsage` — agent-agnostic metric types |
| `provider.go` | `StatusProvider` interface: `Name()`, `ConfigFilePath()`, `ExtractStatus([]byte) map[string]*Status` |
| `claudecode/provider.go` | Claude Code implementation — reads `.claude.json`, parses per-project metrics keyed by workdir |
| `claudecode/provider_test.go` | Tests for Claude Code provider: parsing, model mapping, multi-project, interface compliance |

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
