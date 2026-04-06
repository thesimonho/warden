# Architectural Rules

Warden is an **engine-first product**. The container engine and security model are the core value. The web dashboard and TUI are **reference implementations** — they exist both as usable products AND as documented examples for developers building their own frontends.

## Four binaries

| Binary           | What it is                                              | Code location         | CGo required |
| ---------------- | ------------------------------------------------------- | --------------------- | ------------ |
| `warden`         | Headless engine + API server (for developers)           | `cmd/warden/`         | No           |
| `warden-desktop` | Engine + web UI + browser launch (for users)            | `cmd/warden-desktop/` | No           |
| `warden-tui`     | Engine + TUI (for terminal users)                       | `cmd/warden-tui/`     | No           |
| `warden-tray`    | System tray companion for `warden-desktop` (HTTP only)  | `cmd/warden-tray/`    | Yes          |

`warden-tray` is a separate Go module with its own `go.mod`. It talks to `warden-desktop` over HTTP and does not import any packages from the main module. This isolates the CGo dependency (needed for native tray APIs) so the three core binaries remain pure Go with zero C toolchain requirements.

## Three layers

How do you want to engage with the engine?

    Are you writing Go and want to embed the engine?
      Yes → Service (import warden, call warden.New(), use Service directly)
      No  → Run the warden binary, then:
             Writing Go? → Client (typed HTTP wrapper)
             Other lang? → API (raw HTTP)

| Layer       | Package            | What it is                                               | Consumer                                     |
| ----------- | ------------------ | -------------------------------------------------------- | -------------------------------------------- |
| **Service** | `service/`         | The engine itself — all lifecycle, business logic, state | Go devs embedding the engine in-process      |
| **API**     | `internal/server/` | Thin HTTP skin over Service                              | Any language, via running `warden` binary    |
| **Client**  | `client/`          | Typed Go wrapper around HTTP API                         | Go devs talking to a running `warden` binary |

Key properties:

- Service is the single orchestration layer. ALL lifecycle (session watchers, event dir watchers), ALL business logic, ALL resolve-by-ID convenience lives here.
- API is a thin HTTP skin. Handlers are trivially simple: parse request, call Service, write response.
- Client mirrors the API surface 1:1. Client → network boundary → API → Service.
- Client and API have identical surface area. Service has the same operations PLUS lower-level access for power users.
- Each reference implementation uses exactly ONE layer:
  - Web SPA → API (HTTP calls to `/api/v1/projects/{projectID}/{agentType}/*` and other routes)
  - TUI → Client interface (via `ServiceAdapter` for embedded mode, swappable with `client.Client` for remote mode)
- `warden.New()` wires deps and returns `*Warden` which exposes Service. It's the constructor, not a layer.

## Rules (MUST follow)

1. **The web SPA (`web/src/lib/api.ts`) MUST only use HTTP calls to `/api/v1/*`.** It must never import Go packages or use any mechanism other than standard HTTP/SSE/WebSocket. Developers building web frontends copy these patterns directly — if the SPA uses a shortcut, the reference breaks.

2. **The TUI (`internal/tui/`) MUST be written against a `Client` interface, not against `service.Service` directly.** The same interface must be satisfiable by both the embedded service (for the TUI binary via `ServiceAdapter`) and the `client/` HTTP package (for developers). If you add a new operation to the TUI, add it to the interface first. The TUI imports `api/` for request/response types — it must never import `service/` or `db/`. The `ServiceAdapter` is trivially thin — every method is a one-liner delegation to `w.Service.*`. This makes the TUI the primary reference for the Client interface.

3. **The `client/` package is the Go equivalent of `api.ts`.** It speaks HTTP to a running `warden` server. Keep them in sync — if you add an endpoint, add it to both. The client imports `api/` for types, not `service/`.

3a. **The `api/` package holds the API contract types** (request/response/result structs). These are shared by `service/`, `client/`, and the TUI. When adding a new service method, define its request/response types in `api/`. The `service/` package re-exports them via type aliases for backward compatibility.

4. **The `access/` package is public and importable.** It provides the general-purpose access item library (types, resolution, and built-in items) with no dependencies on service/db/engine. Consumers can use it to resolve credentials and mounts without needing the full Warden engine.

5. **`internal/server/` and `internal/terminal/` stay in `internal/`.** They are HTTP-specific plumbing, not part of the public API. Everything else is importable by external Go consumers.

6. **`warden.New()` in `warden.go` is the library entry point.** It returns `*Warden` which exposes `Service`, `Broker`, `Engine`, `DB`, and `Watcher`. All engine initialization wiring lives here. The `cmd/` binaries should be thin — they call `warden.New()`, set up their UI layer, and run. Do not put engine initialization logic in `cmd/` files.

7. **Service methods accept project IDs (strings), not `*db.ProjectRow`.** The `*db.ProjectRow` type is an internal implementation detail. Service resolves project IDs internally. HTTP handlers and the TUI adapter should never need to import `db/` for project resolution.

8. **Do not optimize the frontend code in ways that break its value as a reference.** Efficiency is good, but clarity for developers reading the code is more important. If a change makes the implementation faster but harder to understand as a pattern to copy, don't make it. Add a comment explaining the trade-off if needed.

9. **All audit writes go through `db.AuditWriter`.** Never call `db.Store.Write()` directly for audit events. The writer handles mode filtering via a `standardEvents` allowlist before persisting to the audit DB.

10. **Types that cross the HTTP boundary MUST live in `api/`; domain types live in `engine/`.** The test: "Does this type appear in a JSON request or response body of an HTTP handler?" If yes, it belongs in `api/`. Examples: `CreateContainerRequest`, `ContainerConfig`, `Mount`, `NetworkMode` are in `api/` because they are HTTP contract types. `Project`, `Worktree`, `AgentStatus`, `WorktreeState` are in `engine/` because they are domain types. `ListProjects` returns `[]api.ProjectResponse` (not `engine.Project`) to decouple the HTTP contract from the domain model. The conversion happens in the service layer via `service/convert.go`.

11. **Generic file-watching primitives live in `watcher/`; agent-specific parsers live in `agent/<name>/`.** `watcher/` must have zero internal module dependencies (only stdlib + external libs like fsnotify). The `watcher.FileTailer` is the shared primitive for tailing append-only files; `agent.SessionWatcher` is a thin wrapper that wires parser callbacks to it.

See the [Integration Paths](https://thesimonho.github.io/warden/integration/paths/) page for the full integration guide that developers follow.
