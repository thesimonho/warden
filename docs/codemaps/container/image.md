# Container Image

The `container/` directory defines the container image used by project containers and a devcontainer feature for injecting Warden's terminal infrastructure into user-provided devcontainer images.

## Directory Structure

```
container/
├── Dockerfile                          # Multi-stage build: builder compiles abduco, runtime uses layered installs
├── scripts/
│   ├── install-tools.sh                # Wrapper for devcontainer feature (calls sub-scripts in order)
│   ├── install-system-deps.sh          # System packages, GitHub CLI, Node.js, abduco/gosu (devcontainer)
│   ├── install-user.sh                 # warden user, workspace dirs, .profile env forwarding
│   ├── install-claude.sh               # Claude Code CLI + managed-settings.json hooks
│   ├── install-codex.sh                # Codex CLI (npm install -g @openai/codex)
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
builder stage:  compile abduco + fetch gosu
runtime stage:
  Layer 1: install-system-deps.sh  (system packages — rarely changes)
  Layer 2: install-user.sh         (warden user — rarely changes)
  Layer 3: install-claude.sh       (Claude CLI — changes on upstream releases)
  Layer 4: install-codex.sh        (Codex CLI — changes on upstream releases)
  Layer 5: install-warden.sh       (Warden scripts — changes every release)
```

Most Warden releases only invalidate Layer 5. CLI updates only invalidate from that CLI's layer onward.

### Sub-Script Responsibilities

- **install-system-deps.sh** — apt packages (git, curl, jq, iptables, etc.), GitHub CLI, Node.js LTS. Compiles abduco and fetches gosu when pre-built binaries aren't available (devcontainer path). Cleans up apt lists.
- **install-user.sh** — creates `warden` user (prefers UID 1000), sets up `~/.local/bin`, creates workspace and agent config directories (`~/.claude`, `~/.codex`), configures `.profile` env forwarding.
- **install-claude.sh** — installs Claude Code CLI via official installer, writes managed-settings.json with attention state hooks (Notification, PreToolUse, UserPromptSubmit).
- **install-codex.sh** — installs Codex CLI via `npm install -g @openai/codex`, creates `~/.codex` config directory.
- **install-warden.sh** — copies runtime scripts from `shared/`, `claude/`, `codex/` to `/usr/local/bin/`. Detects directory layout (subdirectories for Dockerfile, flat for devcontainer). Creates `/project` workspace.

All steps are idempotent. Environment variables: `ABDUCO_VERSION` (default: `0.6`), `GOSU_VERSION` (default: `1.17`).

## Devcontainer Feature

The devcontainer feature (`container/devcontainer-feature/`) packages Warden's terminal infrastructure as an OCI artifact at `ghcr.io/thesimonho/warden/session-tools`. Users who use devcontainers can add this feature to their `.devcontainer/devcontainer.json` to bake Warden infrastructure into their image.

At CI publish time (`.github/workflows/container.yml`), scripts from `container/scripts/` (including subdirectories) are copied flat alongside `install.sh` before packaging. The `install-warden.sh` script detects the flat layout and adjusts accordingly.

## Both CLIs Bundled

The image includes both Claude Code and Codex CLIs. `WARDEN_AGENT_TYPE` env var (set by the Go engine at container creation) controls which agent launches in `create-terminal.sh`. The JSONL schema is a contract between CLI and parser — both must be present for testing.
