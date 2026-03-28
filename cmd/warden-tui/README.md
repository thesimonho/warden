# warden-tui (terminal UI)

The Warden terminal user interface. Embeds the engine and provides a terminal-based interface for managing projects, worktrees, and agent sessions.

## Who is this for?

Users who prefer a terminal interface over a web browser. Download, run, done.

## Architecture

The TUI code lives in `internal/tui/` and is written against a `Client` interface. This makes it a **reference implementation** for Go developers building their own Warden frontends.

```
warden-tui binary
├── warden.New()       → engine (config, runtime, event bus, service)
└── internal/tui       → Bubble Tea application (Client interface)
```

### For Go developers building frontends

The TUI uses a `Client` interface that abstracts the Warden operations. In the TUI binary, this interface is satisfied by the embedded `service.Service`. In your own application, you can satisfy it with the `client/` HTTP package pointing at a running `warden` server:

```go
// What the TUI does (embedded engine):
tui.Run(tui.Options{Client: app.Service})

// What you would do (remote server):
tui.Run(tui.Options{Client: client.New("http://localhost:8090")})
```

See [`internal/tui/`](../../internal/tui/) for the TUI source code.
