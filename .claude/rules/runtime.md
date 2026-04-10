---
paths:
  - "engine/**/*"
  - "docker/**/*"
  - "container/**/*"
  - "cmd/**/*"
  - "packaging/**/*"
---

# Runtime, Platform, and Distribution

## Runtime (Docker)

Warden uses Docker as its container runtime. The engine client talks to the Docker daemon via the Docker-compatible API.

How it's structured:

- **`docker/`** — detects the Docker daemon by probing socket paths, identifies Docker Desktop vs native Docker, and resolves the bridge gateway IP for socket bridge proxies. Platform-specific socket candidates are in build-tagged files (`sockets_linux.go`, `sockets_darwin.go`, `sockets_windows.go`).
- **`engine/`** — the container engine client. Handles Docker and Windows named pipes. All containers get a bind mount for the event directory at `/var/warden/events` so container scripts can write events to the shared mount.
- **Container scripts** (`container/scripts/`) — the entrypoint uses the gosu pattern (root for privileged setup, then `exec gosu warden` to drop privileges permanently). Use `WARDEN_EVENT_DIR` env var (set by Warden) to write events to the bind-mounted directory. The user-phase entrypoint reads `WARDEN_BRIDGE_*` env vars and starts `socat` processes that create Unix sockets in the container and forward connections to the host via `host.docker.internal` (TCP bridge for SSH/GPG agent forwarding).

## Platform support

Warden supports Linux, macOS, and Windows. Build-tagged signal handlers (`syscall.SIGTERM` on Unix, `os.Interrupt` on Windows) enable graceful shutdown on each platform. Platform-specific container socket detection uses build-tagged files in `docker/sockets_*.go`.

## Desktop distribution

The desktop package ships two binaries: `warden-desktop` (CGo-free) and `warden-tray` (requires CGo for native tray APIs). On launch, `warden-desktop` starts the server, waits for it to be ready, spawns `warden-tray` as a child process, then opens the system default browser. The `run(srv, url)` function in `cmd/warden-desktop/run.go` owns this flow. If `warden-tray` is not found (e.g. built from source without CGo), the desktop server runs normally without it.

Platform packaging files are in `packaging/`:

- `macos/` — `Info.plist` + `bundle.sh` to create a universal (amd64+arm64) `.app` bundle via `lipo`, packaged as a `.dmg` installer via `create-dmg` (CI only, macOS runner)
- `linux/` — `.desktop` file + `nfpm.yaml` for building `.deb`, `.rpm`, and Arch (`.pkg.tar.zst`) packages via nfpm. AppImages are built for both amd64 and arm64 via `appimagetool`.
- `windows/` — `versioninfo.json` for icon/version embedding via goversioninfo (applied in CI with `-H windowsgui`), plus `warden.iss` Inno Setup script that produces a `warden-desktop-setup-windows-amd64.exe` installer with optional PATH integration

Release CI also generates `checksums.txt` (SHA-256) and `sbom.spdx.json` (SPDX SBOM via syft) covering all release assets. All Go builds use `-trimpath` for reproducibility.
