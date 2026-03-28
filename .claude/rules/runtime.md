---
paths:
  - "engine/**/*"
  - "runtime/**/*"
  - "container/**/*"
  - "cmd/**/*"
  - "packaging/**/*"
---

# Runtime, Platform, and Distribution

## Runtime abstraction (Docker / Podman)

Both runtimes expose the same Docker-compatible API, so most code is runtime-agnostic. But rootless Podman has subtle differences (UID mapping, socket paths, capability handling) that can silently break features that work fine under Docker. **Always test changes against both runtimes.**

How it's structured:

- **`runtime/`** — detects available runtimes by probing socket paths and binaries. Platform-specific socket candidates are in build-tagged files (`sockets_linux.go`, `sockets_darwin.go`, `sockets_windows.go`). The selected runtime name (`"docker"` or `"podman"`) is stored in config and passed to `engine.NewClient()`.
- **`engine/`** — the container engine client. Handles both Docker and Podman runtimes, plus Windows named pipes. Runtime-specific logic is gated on `dc.runtimeName` and kept minimal (currently just `--userns=keep-id` for Podman in `containers.go`). All containers get a bind mount for the event directory at `/var/warden/events` so container scripts can write events to the shared mount.
- **Container scripts** (`container/scripts/`) — detect root vs non-root at runtime via `$(id -u)` and branch accordingly (e.g., skip `su - dev` and `chown` under rootless Podman). Use `WARDEN_EVENT_DIR` env var (set by Warden) to write events to the bind-mounted directory.

When adding runtime-specific behavior:

- **Go side**: gate on `dc.runtimeName` in `engine/`, don't scatter conditionals across packages.
- **Script side**: gate on `$(id -u)` (root vs non-root), not on which runtime is running. This keeps scripts runtime-agnostic.
- Keep Podman workarounds next to the Docker equivalent, not in separate files.

## Platform support

Warden supports Linux, macOS, and Windows. Build-tagged signal handlers (`syscall.SIGTERM` on Unix, `os.Interrupt` on Windows) enable graceful shutdown on each platform. Platform-specific container socket detection uses build-tagged files in `runtime/sockets_*.go`.

## Desktop distribution

There is a single binary with no build tags or CGo. On launch it starts the server, waits for it to be ready, then opens the system default browser (`open` / `xdg-open` / `start`). The `run(srv, url)` function in `cmd/dashboard/run.go` owns this flow.

Platform packaging files are in `packaging/`:

- `macos/` — `Info.plist` + `bundle.sh` to create a `.app` bundle (CI only, macOS runner)
- `linux/` — `.desktop` file for desktop integration
- `windows/` — `versioninfo.json` for icon/version embedding via goversioninfo (applied in CI with `-H windowsgui`)
