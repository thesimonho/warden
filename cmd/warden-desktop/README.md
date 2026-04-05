# warden-desktop (web dashboard)

The Warden web dashboard. Embeds the engine, starts an HTTP server, and opens a browser-based UI for managing projects, worktrees, and agent sessions.

## Who is this for?

Users who want a visual interface to manage their Claude Code agents. Download, run, done.

## Usage

```bash
# Start and open browser
./warden-desktop

# Start without opening browser
WARDEN_NO_OPEN=1 ./warden-desktop

# Custom address
ADDR=0.0.0.0:9000 ./warden-desktop
```

## How it works

This binary embeds the full Warden engine (via `warden.New()`) and serves the React SPA from embedded filesystem assets. The SPA communicates with the engine through the same `/api/v1/` endpoints available to any external client — see [`web/src/lib/api.ts`](../../web/src/lib/api.ts).

## Platform packaging

- **macOS** — universal binary (Intel + Apple Silicon) packaged as a `.dmg` installer via `create-dmg`
- **Linux** — `.deb`, `.rpm`, Arch (`.pkg.tar.zst`), and AppImage with zsync auto-update. Packaging config in `packaging/linux/`
- **Windows** — Inno Setup installer (`Warden-Setup-amd64.exe`) with optional PATH integration. Built with `-H windowsgui` and icon embedding via `goversioninfo`

## Architecture

```
warden-desktop binary
├── warden.New()           → engine (config, runtime, event bus, service)
├── internal/server        → HTTP server + embedded SPA
└── internal/terminal      → WebSocket-to-PTY proxy
```

The frontend source is in [`web/`](../../web/). The HTTP server and API routes are in [`internal/server/`](../../internal/server/).
