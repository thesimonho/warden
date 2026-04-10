# Container Image

The `container/` directory defines the container image used by project containers and a devcontainer feature for injecting Warden's terminal infrastructure into user-provided devcontainer images.

## Directory Structure

```
container/
├── Dockerfile                          # Multi-stage build: builder fetches gosu, runtime uses layered installs
├── scripts/
│   ├── install-tools.sh                # Wrapper for devcontainer feature (calls sub-scripts in order)
│   ├── install-system-deps.sh          # System packages, GitHub CLI, Node.js, tmux/gosu (devcontainer)
│   ├── install-user.sh                 # warden user, workspace dirs, .profile env forwarding
│   ├── install-warden.sh               # Copy scripts to /usr/local/bin/, create /project
│   ├── shared/                         # Agent-agnostic runtime scripts
│   ├── claude/                         # Claude-specific event handler
│   └── codex/                          # Codex-specific event handler (placeholder)
└── devcontainer-feature/
    ├── devcontainer-feature.json        # Feature metadata (id, version, options)
    ├── install.sh                       # Feature entry point, delegates to install-tools.sh
    └── README.md                        # Feature usage documentation
```

## Install Pipeline

The install pipeline is split into composable sub-scripts. The Dockerfile calls each as a separate `RUN` instruction for Docker layer caching. The devcontainer feature calls `install-tools.sh` which orchestrates all sub-scripts.

### Dockerfile Layer Order

```
builder stage:  fetch gosu
runtime stage:
  Layer 1: install-system-deps.sh  (system packages — rarely changes)
  Layer 2: install-user.sh         (warden user — rarely changes)
  Layer 3: install-warden.sh       (Warden scripts — changes every release)
```

Agent CLIs (Claude Code, Codex) are **not baked into the image**. They are installed at container startup by `install-agent.sh` using pinned versions from `agent/versions.go`, passed as env vars (`WARDEN_CLAUDE_VERSION`, `WARDEN_CODEX_VERSION`). The `warden-cache` Docker volume caches downloads across container creates. For remote projects, a Docker volume named `warden-workspace-{containerName}` persists the cloned workspace across container recreates (when `temporary=false`); temporary remote workspaces (when `temporary=true`) store code in the container's writable layer and are lost on recreation.

### Sub-Script Responsibilities

- **install-system-deps.sh** — apt packages (git, curl, jq, iptables, socat, etc.), GitHub CLI, Node.js LTS. Fetches gosu when pre-built binaries aren't available (devcontainer path). Cleans up apt lists.
- **install-user.sh** — creates `warden` user (prefers UID 1000), sets up `~/.local/bin`, creates workspace and agent config directories (`~/.claude`, `~/.codex`), configures `.profile` env forwarding.
- **install-warden.sh** — copies runtime scripts from `shared/`, `claude/`, `codex/` to `/usr/local/bin/`, calls `install-clipboard-shim.sh` to set up the xclip wrapper. Detects directory layout (subdirectories for Dockerfile, flat for devcontainer). Creates `/project` workspace.

All steps are idempotent. Environment variables: `GOSU_VERSION` (default: `1.17`).

### Startup-Time Installation

At container startup, the entrypoint runs `install-agent.sh` and `install-runtimes.sh` before dropping privileges:

- **install-agent.sh** — installs the correct agent CLI based on `WARDEN_AGENT_TYPE`. Claude Code is downloaded as a standalone binary from GCS; Codex is installed via npm. Both cache to the `warden-cache` volume keyed by version.
- **install-runtimes.sh** — installs user-selected language runtimes (Python, Go, Rust, Ruby, Lua) from `WARDEN_ENABLED_RUNTIMES`.

Both scripts fire SSE events (`agent_installing`/`agent_installed`, `runtime_installing`/`runtime_installed`) for frontend progress tracking.

## Devcontainer Feature

The devcontainer feature (`container/devcontainer-feature/`) packages Warden's terminal infrastructure as an OCI artifact at `ghcr.io/thesimonho/warden/session-tools`. Users who use devcontainers can add this feature to their `.devcontainer/devcontainer.json` to bake Warden infrastructure into their image.

At CI publish time (`.github/workflows/container.yml`), scripts from `container/scripts/` (including subdirectories) are copied flat alongside `install.sh` before packaging. The `install-warden.sh` script detects the flat layout and adjusts accordingly.

## Agent CLI Version Pinning

Each agent CLI is pinned to an exact version in `agent/versions.go`. The JSONL parser is tightly coupled to CLI output format, so pinning prevents breakage from unvalidated upstream changes. CI bumps these constants after parser compatibility tests pass for the new version.

- **Claude Code**: standalone binary from GCS (`storage.googleapis.com`). Version pinning uses the deterministic URL pattern `{GCS_BUCKET}/{VERSION}/{PLATFORM}/claude`.
- **Codex**: npm package `@openai/codex@{VERSION}`. npm cache on the volume speeds up reinstalls.

Only the CLI matching the container's `WARDEN_AGENT_TYPE` is installed — a Claude container never downloads Codex, and vice versa.
