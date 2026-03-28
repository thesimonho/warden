# Container Codemap

The `container/` directory defines the container image used by project containers and a devcontainer feature for injecting Warden's terminal infrastructure into user-provided devcontainer images.

## Directory Structure

```
container/
├── Dockerfile                          # Multi-stage build: builder compiles abduco, runtime has no build deps
├── scripts/
│   ├── install-tools.sh                # Shared install logic (abduco, gosu, Claude Code, dev user, managed hooks, network isolation tools)
│   ├── entrypoint.sh                   # Root-phase entrypoint: UID remapping, iptables, exec gosu to drop privileges
│   ├── user-entrypoint.sh              # User-phase entrypoint (PID 1 as dev): env forwarding, git config, heartbeat, stay alive
│   ├── setup-network-isolation.sh      # Network isolation setup: iptables rules based on WARDEN_NETWORK_MODE and WARDEN_ALLOWED_DOMAINS
│   ├── create-terminal.sh              # Terminal lifecycle: start abduco session, launch Claude Code; pushes session_exit events
│   ├── disconnect-terminal.sh          # Terminal teardown: kill processes, remove tracking dir; pushes terminal_disconnected event
│   ├── kill-worktree.sh                # Process cleanup: kills all processes for a worktree; pushes process_killed event
│   ├── warden-heartbeat.sh             # Periodic heartbeat loop: writes heartbeat event to bind-mounted event directory every 10s
│   ├── warden-write-event.sh           # Shared atomic file write function for event delivery (used by all event-posting scripts)
│   ├── warden-cost-lib.sh              # Shared cost functions: read_cost_data + send_cost_event (sourced by event.sh and capture-cost.sh)
│   ├── warden-capture-cost.sh          # Post-exit cost capture: sources warden-cost-lib.sh and fires stop event via warden-write-event.sh
│   └── warden-event.sh                 # Event bus dispatcher: writes hook events to bind-mounted event directory via warden-write-event.sh
└── devcontainer-feature/
    ├── devcontainer-feature.json        # Feature metadata (id, version, options)
    ├── install.sh                       # Feature entry point, delegates to install-tools.sh
    └── README.md                        # Feature usage documentation
```

## Shared Install Logic

`scripts/install-tools.sh` is the single source of truth for installing Warden's terminal infrastructure. It is used by both the Dockerfile and the devcontainer feature. The script:

1. Installs runtime system deps (git, curl, jq, procps, iproute2, psmisc, iptables)
2. Installs gosu if not already present (downloaded as a static binary). When used from the multi-stage Dockerfile, gosu is pre-built in the builder stage and this step is skipped entirely
3. Compiles abduco from source if not already present (installs build deps, compiles, then purges build deps). When used from the multi-stage Dockerfile, abduco is pre-built in the builder stage and this step is skipped entirely
4. Creates `dev` non-root user
5. Installs Claude Code CLI via official installer
6. Sets up `/home/dev` and `/home/dev/.claude` directories
7. Adds env var forwarding to `/home/dev/.bashrc`
8. Copies terminal scripts to `/usr/local/bin/` (including `user-entrypoint.sh`, `setup-network-isolation.sh`, `warden-heartbeat.sh`, `warden-write-event.sh`, and other lifecycle scripts)
9. Creates Claude Code managed settings at `/etc/claude-code/managed-settings.json` with hooks for attention tracking (`Notification`, `UserPromptSubmit`, `PreToolUse`) and event logging (all major hooks, controlled by `eventLogMode` setting)

All steps are idempotent — the script can be run multiple times safely.

Environment variables: `ABDUCO_VERSION` (default: `0.6`), `GOSU_VERSION` (default: `1.17`).

## Devcontainer Feature

The devcontainer feature (`container/devcontainer-feature/`) packages Warden's terminal infrastructure as an OCI artifact at `ghcr.io/thesimonho/warden/session-tools`. Users who use devcontainers can add this feature to their `.devcontainer/devcontainer.json` to bake Warden infrastructure into their image, then pass the built image to Warden like any custom image.

At CI publish time (`.github/workflows/devcontainer-feature.yml`), scripts from `container/scripts/` are copied into the feature directory before packaging.

## Process Isolation

Every Warden container is hardened with three layers of process isolation, applied unconditionally at creation time:

### 1. Capability Dropping

All default Linux capabilities are dropped (`CapDrop: ALL`), then only the minimum required set is re-added:

| Capability       | Why needed                                   | When            |
| ---------------- | -------------------------------------------- | --------------- |
| CHOWN            | Entrypoint chown of bind mounts              | Always          |
| DAC_OVERRIDE     | Root reading/writing files owned by dev user | Always          |
| FOWNER           | Entrypoint file ownership operations         | Always          |
| FSETID           | Preserve setuid/setgid bits during chown     | Always          |
| KILL             | Shutdown handler: kill -TERM -1              | Always          |
| SETUID           | gosu privilege drop (setuid syscall)         | Always          |
| SETGID           | gosu privilege drop (setgid syscall)         | Always          |
| NET_BIND_SERVICE | Dev servers binding to ports < 1024          | Always          |
| NET_RAW          | Ping and network diagnostics                 | Always          |
| SYS_CHROOT       | Some tools (e.g. npm) use chroot             | Always          |
| NET_ADMIN        | iptables for network isolation               | restricted/none |

Dropped from Docker defaults: SETPCAP (modify capability sets), MKNOD (create device nodes), SETFCAP (set file capabilities), AUDIT_WRITE (PAM audit logging — not needed since gosu bypasses PAM).

### 2. Seccomp Profile

A denylist-based seccomp profile (`SCMP_ACT_ALLOW` default) blocks dangerous syscalls while allowing all standard dev tooling. Embedded in the Go binary via `engine/seccomp/profile.json`. Blocked syscall categories:

- **Kernel manipulation**: kexec_load, reboot, init_module, delete_module
- **Filesystem mounting**: mount, umount2, pivot_root, fsopen, fsmount, move_mount, open_tree
- **Security-sensitive**: bpf, perf_event_open, userfaultfd, open_by_handle_at, keyring operations
- **System administration**: acct, swapon, swapoff, syslog, settimeofday

### 3. No New Privileges

The `no-new-privileges` flag prevents privilege escalation via setuid/setgid binaries inside the container. The entrypoint starts as root for privileged setup (UID remapping, iptables), then permanently drops to the `dev` user via `exec gosu`. PID 1 runs as `dev` — no root process remains in the container after the privilege drop.

### Podman Compatibility

All three security layers work with both Docker and Podman. When Podman uses `--userns=keep-id` (rootless mode), capabilities are limited by the user namespace — the CapDrop/CapAdd settings are accepted but the effective capability set is further constrained by rootless restrictions. This is additive security, not a conflict.

## Network Isolation

Container network modes are passed as environment variables to enforce isolation at container start:

| Mode         | Env Var                          | Behavior                                    |
| ------------ | -------------------------------- | ------------------------------------------- |
| `full`       | `WARDEN_NETWORK_MODE=full`       | Unrestricted internet access (default)      |
| `restricted` | `WARDEN_NETWORK_MODE=restricted` | Outbound traffic limited to allowed domains |
| `none`       | `WARDEN_NETWORK_MODE=none`       | All outbound traffic blocked (air-gapped)   |

For `restricted` mode, allowed domains are passed as `WARDEN_ALLOWED_DOMAINS=domain1.com,domain2.com`. The `setup-network-isolation.sh` script runs in the entrypoint (before user code executes) and configures iptables OUTPUT rules based on the network mode:

- **full**: No rules applied
- **restricted**: DNS server IP (from `/etc/resolv.conf`) and resolved domain IPs are whitelisted; all other outbound traffic on port 53 (DNS) and application ports is blocked
- **none**: All outbound traffic blocked except loopback

Wildcard domains (e.g. `*.github.com`) are supported — the base domain is resolved and its IPs are whitelisted. This works for CDNs that share IPs with the base domain but may not cover all subdomains for services like AWS/GCP.

Note: Domain IPs are resolved once at container start. CDN IP rotation or dynamic IP changes require container restart.

NET_ADMIN capability is added only for `restricted` and `none` modes (see Process Isolation above).

## Env Var Forwarding

The `gosu` exec creates a clean environment, stripping container env vars. The user-phase entrypoint works around this by:

1. Writing all env vars to `/home/dev/.docker_env` at startup (excluding `HOME`, `USER`, `SHELL`, etc.)
2. `.bashrc` sources this file on every new shell

This ensures all vars passed via `docker run -e` or `podman run -e` are available in terminal sessions.

**Key environment variables set by Warden:**

- `WARDEN_HOST_UID` / `WARDEN_HOST_GID` — host user's UID/GID (from `os.Stat()` of project path). Used by the root-phase entrypoint for UID remapping via `usermod`/`groupmod`
- `WARDEN_WORKSPACE_DIR` — container-side workspace path (e.g. `/home/dev/my-project`). Set by Warden at container creation to give each project a unique path in Claude Code's `.claude.json` (which keys cost data by workspace path). All scripts use `${WARDEN_WORKSPACE_DIR:-/project}` for backward compatibility with legacy containers.
- `WARDEN_PROJECT_ID` — deterministic 12-char hex identifier (SHA-256 of resolved absolute host path). Set by Warden at container creation. All event-posting scripts include this in their JSON payloads so the host-side event bus can associate events with the correct project identity, even across container rebuilds.
- `WARDEN_EVENT_DIR` — bind-mounted event directory path (`/var/warden/events`), used by event-posting scripts
- `WARDEN_NETWORK_MODE` — network isolation mode (`full`/`restricted`/`none`), controls iptables rules
- `WARDEN_ALLOWED_DOMAINS` — comma-separated domain list for `restricted` mode (optional)

## Terminal Lifecycle

### create-terminal.sh

Accepts `<worktree-id> [--skip-permissions]`:

- Validates worktree ID (alphanumeric, hyphens, underscores, dots; no path traversal)
- Starts abduco session `warden-<worktree-id>`
- Launches Claude Code with `--worktree <worktree-id>` (no `--session-id`)
- If `--skip-permissions` is passed, adds `--dangerously-skip-permissions` to the Claude invocation
- When Claude exits, captures cost from `.claude.json` via `warden-capture-cost.sh` (catches Ctrl-C case), records exit code to `.warden-terminals/<worktree-id>/exit_code`, pushes `session_exit` event, then drops to `exec bash` so the shell stays alive
- Outputs JSON `{"worktreeId":"..."}` to stdout

### disconnect-terminal.sh

- Kills abduco session via `pkill`
- Pushes `terminal_disconnected` event (via `warden-push-event.sh`)
- Removes the terminal tracking directory entry (NOT the git worktree itself)
- Outputs `{"status":"disconnected"}` to stdout

### kill-worktree.sh

- Kills abduco for a worktree
- Pushes `process_killed` event (via `warden-push-event.sh`)
- Removes all terminal tracking state

### warden-heartbeat.sh

- Runs as background process (started by entrypoint.sh)
- Writes a heartbeat event to the bind-mounted event directory every 10s
- Allows backend liveness checker to detect stale containers

### warden-write-event.sh

- Shared helper script used by all event-posting scripts (`warden-event.sh`, `warden-heartbeat.sh`, terminal lifecycle scripts)
- Atomically writes events to the bind-mounted event directory (write to `.tmp`, rename to `.json`)
- Filename format: `<epoch_ns>-<pid>.json`

## Attention Tracking

Claude Code's `Notification` hook (configured via managed settings at `/etc/claude-code/managed-settings.json`) pushes the notification type to the event bus via `warden-event.sh`. The dispatcher determines the worktree ID from Claude's cwd path (`/project/.claude/worktrees/<id>` → id, `/project` → "main"). Attention types:

- `permission_prompt` — Claude needs tool approval
- `idle_prompt` — Claude is done and waiting for the next prompt
- `elicitation_dialog` — Claude is asking the user a question
- `auth_success` — authentication completed (not treated as attention-requiring)

`UserPromptSubmit` and `PreToolUse` hooks push attention-clear events when the user responds or Claude resumes work. `PreToolUse` with `tool_name == "AskUserQuestion"` pushes a `needs_answer` event instead. All hooks merge with user/project hooks — they never override user configuration.

## Audit Logging Modes

The `auditLogMode` setting (off/standard/detailed) controls which Claude Code hook events are captured by managed settings and written to the event directory. The backend broadcasts mode changes to all running containers via SSE, and they dynamically register/unregister hooks.

**off mode** — No hooks registered. No events logged. Audit dashboard is empty. Minimal container overhead.

**standard mode** — Only attention-tracking hooks registered (`Notification`, `UserPromptSubmit`, `PreToolUse`). Terminal lifecycle and cost events are always logged. High-value, low-volume events suitable for production.

**detailed mode** — All major Claude Code hooks registered, including:

- `SessionStart` — session begins (includes `sessionId`, `model`, `source`)
- `SessionEnd` — session ends (includes `reason`)
- `Stop` — process stop/failure
- `Notification` — attention-requiring events (permission prompt, idle prompt, etc.)
- `UserPromptSubmit` — user responds to Claude (fires both attention-clear and audit prompt events)
- `PreToolUse` — tool about to execute (captures tool name, input)
- `PostToolUseFailure` — tool execution failed
- `StopFailure` — process stop failed
- `PermissionRequest` — tool permission requested
- `SubagentStart` / `SubagentStop` — subagent lifecycle
- `ConfigChange` — config modified
- `InstructionsLoaded` — instructions changed
- `TaskCompleted` — task finished
- `Elicitation` / `ElicitationResult` — clarification question/answer

Events are written via `warden-event.sh` to the bind-mounted event directory (`WARDEN_EVENT_DIR`), timestamped in the JSON payload, and processed by the watcher. The audit dashboard is only populated when `auditLogMode` is `detailed`.

**Not hooked**: `WorktreeCreate` — registering any hook replaces Claude Code's default `git worktree` creation behavior, breaking worktree setup. Worktree creation can be observed indirectly via `SessionStart`.

## Terminal Storage

```
/project/.warden-terminals/          # Ephemeral — cleared on container restart
  .gitignore                           # '*' — prevents tracking artifacts
  <worktree-id>/
    exit_code                          # Claude's exit code (present when Claude exited)
```

## Event Bus Communication

Terminal lifecycle events and hook events are pushed to the host via file-based delivery. Containers write JSON event files to a bind-mounted directory (`WARDEN_EVENT_DIR=/var/warden/events`). The host watches this directory using fsnotify (fast path) + polling every 2s (reliable fallback):

- `terminal_connected` — abduco session created, terminal ready for WebSocket connections
- `terminal_disconnected` — terminal viewer disconnected, abduco continues in background
- `process_killed` — all processes for a worktree terminated
- `session_exit` — Claude Code exited (includes exit code)
- `heartbeat` — periodic liveness signal from container (every 10s)

The backend liveness checker monitors heartbeats and marks containers stale after 30s of silence. Containers set timestamp in the JSON payload (not the watcher at read time). The watcher enforces a safety valve: 50,000 files maximum, dropping oldest files when exceeded.

## Worktree Storage

```
/project/.worktrees/                 # Persistent — survives container restarts
  <worktree-id>/                       # git worktree checkout (created by `git worktree add`)
```

## Ports

No dedicated ports for terminals. WebSocket connections are proxied through the backend on the same port as the HTTP API (default 8090 on the host).

## Usage

```bash
# Build
docker build -t claude-project-dev ./container

# Run with project mounted (Warden automatically sets WARDEN_EVENT_DIR)
docker run -d \
  -e ANTHROPIC_API_KEY=sk-xxx \
  -v ./my-project:/project \
  claude-project-dev
```
