# Architectural Rules

Warden is an **engine-first product**. The container engine and security model are the core value. The web dashboard and TUI are **reference implementations** — they exist both as usable products AND as documented examples for developers building their own frontends.

## Three binaries

| Binary           | What it is                                    | Code location         |
| ---------------- | --------------------------------------------- | --------------------- |
| `warden`         | Headless engine + API server (for developers) | `cmd/warden/`         |
| `warden-desktop` | Engine + web UI + browser launch (for users)  | `cmd/warden-desktop/` |
| `warden-tui`     | Engine + TUI (for terminal users)             | `cmd/warden-tui/`     |

## Rules (MUST follow)

1. **The web SPA (`web/src/lib/api.ts`) MUST only use HTTP calls to `/api/v1/*`.** It must never import Go packages or use any mechanism other than standard HTTP/SSE/WebSocket. Developers building web frontends copy these patterns directly — if the SPA uses a shortcut, the reference breaks.

2. **The TUI (`internal/tui/`) MUST be written against a `Client` interface, not against `service.Service` directly.** The same interface must be satisfiable by both the embedded service (for the TUI binary via `ServiceAdapter`) and the `client/` HTTP package (for developers). If you add a new operation to the TUI, add it to the interface first. The TUI imports `api/` for request/response types — it must never import `service/`. The `ServiceAdapter` adapter uses the generic `withProject[T](adapter, projectID, fn)` helper to resolve project rows before delegating to service methods.

3. **The `client/` package is the Go equivalent of `api.ts`.** It speaks HTTP to a running `warden` server. Keep them in sync — if you add an endpoint, add it to both. The client imports `api/` for types, not `service/`.

3a. **The `api/` package holds the API contract types** (request/response/result structs). These are shared by `service/`, `client/`, and the TUI. When adding a new service method, define its request/response types in `api/`. The `service/` package re-exports them via type aliases for backward compatibility.

4. **`internal/server/` and `internal/terminal/` stay in `internal/`.** They are HTTP-specific plumbing, not part of the public API. Everything else is importable by external Go consumers.

5. **`warden.New()` in `warden.go` is the library entry point.** All engine initialization wiring lives here. The `cmd/` binaries should be thin — they call `warden.New()`, set up their UI layer, and run. Do not put engine initialization logic in `cmd/` files.

6. **Do not optimize the frontend code in ways that break its value as a reference.** Efficiency is good, but clarity for developers reading the code is more important. If a change makes the implementation faster but harder to understand as a pattern to copy, don't make it. Add a comment explaining the trade-off if needed.

7. **All audit writes go through `db.AuditWriter`.** Never call `db.Store.Write()` directly for audit events. The writer handles mode filtering via a `standardEvents` allowlist before persisting to the audit DB.

See the [Integration Paths](https://thesimonho.github.io/warden/integration/paths/) page for the full integration guide that developers follow.
