---
paths:
  - "engine/**/*"
  - "runtime/**/*"
  - "container/**/*"
  - "cmd/**/*"
  - "packaging/**/*"
---

# Runtime, Platform, and Distribution

## Runtime (Docker)

Warden uses Docker as its container runtime. The engine client talks to the Docker daemon via the Docker-compatible API.

How it's structured:

- **`runtime/`** — detects the Docker runtime by probing socket paths. Platform-specific socket candidates are in build-tagged files (`sockets_linux.go`, `sockets_darwin.go`, `sockets_windows.go`).
- **`engine/`** — the container engine client. Handles Docker and Windows named pipes. All containers get a bind mount for the event directory at `/var/warden/events` so container scripts can write events to the shared mount.
- **Container scripts** (`container/scripts/`) — the entrypoint uses the gosu pattern (root for privileged setup, then `exec gosu warden` to drop privileges permanently). Use `WARDEN_EVENT_DIR` env var (set by Warden) to write events to the bind-mounted directory.

## Platform support

Warden supports Linux, macOS, and Windows. Build-tagged signal handlers (`syscall.SIGTERM` on Unix, `os.Interrupt` on Windows) enable graceful shutdown on each platform. Platform-specific container socket detection uses build-tagged files in `runtime/sockets_*.go`.

## Desktop distribution

There is a single binary with no build tags or CGo. On launch it starts the server, waits for it to be ready, then opens the system default browser (`open` / `xdg-open` / `start`). The `run(srv, url)` function in `cmd/dashboard/run.go` owns this flow.

Platform packaging files are in `packaging/`:

- `macos/` — `Info.plist` + `bundle.sh` to create a `.app` bundle (CI only, macOS runner)
- `linux/` — `.desktop` file for desktop integration
- `windows/` — `versioninfo.json` for icon/version embedding via goversioninfo (applied in CI with `-H windowsgui`)
