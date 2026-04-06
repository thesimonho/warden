# Integration Paths

Warden is designed to be integrated into your own applications. Start with the [Architecture](./concepts.md) page to understand how the layers fit together, then choose an integration path below.

## Binaries

Warden ships as four binaries that provide different integration levels:

| Binary           | What it is                                              | Code location         | CGo |
| ---------------- | ------------------------------------------------------- | --------------------- | --- |
| `warden`         | Headless engine + API server (for developers)           | `cmd/warden/`         | No  |
| `warden-desktop` | Engine + web UI + browser launch (for users)            | `cmd/warden-desktop/` | No  |
| `warden-tui`     | Engine + TUI (for terminal users)                       | `cmd/warden-tui/`     | No  |
| `warden-tray`    | System tray companion for `warden-desktop` (HTTP only)  | `cmd/warden-tray/`    | Yes |

The first three binaries share the same engine and are CGo-free. `warden-tray` is a separate Go module that talks to `warden-desktop` over HTTP, isolating its native tray dependency. The `cmd/` entry points are thin — they call `warden.New()`, set up their UI layer, and run.

## Key packages

All packages are importable via `go get github.com/thesimonho/warden`:

| Package            | Purpose                                                                                 |
| ------------------ | --------------------------------------------------------------------------------------- |
| `warden` (root)    | Engine entry point — `warden.New()` returns `*Warden` with `.Service`                   |
| `access`           | Credential passthrough model (items, credentials, resolution)                           |
| `api`              | API contract types (request/response/result)                                            |
| `client`           | Typed HTTP client for the Warden API                                                    |
| `service`          | Business logic layer                                                                    |
| `engine`           | Container engine client + domain types                                                  |
| `eventbus`         | Event system (broker, store, listener)                                                  |
| `db`               | SQLite database store (projects, settings, events)                                      |
| `runtime`          | Container runtime detection                                                             |
| `agent`            | Agent abstraction, registry, and session watcher                                        |
| `agent/claudecode` | Claude Code JSONL parser and status provider                                            |
| `agent/codex`      | Codex JSONL parser and status provider                                                  |
| `runtimes`         | Language runtime registry with auto-detection, network domains, and env var definitions |
| `watcher`          | Generic file-tailing utilities (used by agent session watcher)                          |

See the [Go Packages](https://thesimonho.github.io/warden/reference/go/) reference for full API documentation.

## Integration paths

If you're building in Go, you have two additional options: a **client package** that wraps the API for convenience, or importing the **library directly** to skip the binary entirely.

### Decision tree

```
Are you writing Go?
├─ Yes → Want single-process deployment with no separate binary?
│         ├─ Yes → Go library (import warden, call warden.New())
│         └─ No  → Running warden as a server?
│                 ├─ Yes → Go client (typed HTTP wrapper)
│                 └─ No  → Run it first, then use Go client
│
└─ No  → Using a language other than Go?
         └─ Run the warden binary as a server, then:
            ├─ HTTP API (raw REST/SSE/WebSocket)
            └─ Bindings/SDKs if available for your language
```

## HTTP API (any language)

Run the `warden` binary as a headless server and make HTTP requests to `/api/v1/*`. This works from any language and is the recommended integration path.

Ship the `warden` binary with your application and start it as a subprocess, or run it as a standalone service.

See the [HTTP API guide](./examples/api.md) for setup, examples, and the full error code reference. See the [API Reference](https://thesimonho.github.io/warden/reference/api/) for endpoint documentation.

## Go client (convenience wrapper)

If you're building a Go application that talks to a running `warden` server, the [`client`](https://github.com/thesimonho/warden/tree/main/client) package provides a typed wrapper around the HTTP API. It's the same API — just easier to call from Go.

```go
import "github.com/thesimonho/warden/client"

c := client.New("http://localhost:8090")
projects, err := c.ListProjects(ctx)
```

See the [Go client guide](./examples/client.md) for details and error handling.

## Go library (direct import)

If you want to skip the binary entirely and embed Warden's engine in your Go process, import the library directly. No HTTP server, no subprocess — just call the API in-process.

```go
import "github.com/thesimonho/warden"

w, err := warden.New(warden.Options{})
defer w.Close()
projects, _ := w.Service.ListProjects(ctx)
```

See the [Go library guide](./examples/library.md) for the full API.

## When to use which

| Approach                                | When to use                                        | What you ship                  |
| --------------------------------------- | -------------------------------------------------- | ------------------------------ |
| [HTTP API](./examples/api.md)           | Any language, simplest integration                 | `warden` binary + your app     |
| [Go client](./examples/client.md)       | Go app, want typed API calls without managing HTTP | `warden` binary + your Go app  |
| [Go library](./examples/library.md)     | Go app, want single-process deployment, no binary  | Your Go app (imports `warden`) |

## Reference implementations

Both the web dashboard and TUI use the exact same public interfaces you would:

- **TypeScript/Web**: [`web/src/lib/api.ts`](https://github.com/thesimonho/warden/blob/main/web/src/lib/api.ts) — every endpoint call pattern, `ApiError` class with `code` field
- **TypeScript types**: [`web/src/lib/types.ts`](https://github.com/thesimonho/warden/blob/main/web/src/lib/types.ts) — request/response shapes
- **SSE handling**: [`web/src/hooks/use-event-source.ts`](https://github.com/thesimonho/warden/blob/main/web/src/hooks/use-event-source.ts) — reconnection strategy
- **Terminal WebSocket**: [`web/src/hooks/use-terminal.ts`](https://github.com/thesimonho/warden/blob/main/web/src/hooks/use-terminal.ts) — binary I/O, resize
- **Go HTTP client**: [`client/`](https://github.com/thesimonho/warden/tree/main/client) — typed Go wrapper around the HTTP API
