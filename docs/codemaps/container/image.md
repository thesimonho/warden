# Container Image

The `container/` directory defines the container image used by project containers and a devcontainer feature for injecting Warden's terminal infrastructure into user-provided devcontainer images.

## Directory Structure

```
container/
├── Dockerfile                          # Multi-stage build: builder compiles abduco, runtime has no build deps
├── scripts/
│   ├── install-tools.sh                # Shared install logic (abduco, gosu, Claude Code, dev user, managed hooks, network isolation tools)
│   ├── entrypoint.sh                   # Root-phase entrypoint: UID remapping, iptables, exec gosu to drop privileges
│   ├── user-entrypoint.sh              # User-phase entrypoint (PID 1 as dev): env forwarding, git config, heartbeat, stay alive
│   └── ...                             # See scripts.md for all scripts
└── devcontainer-feature/
    ├── devcontainer-feature.json        # Feature metadata (id, version, options)
    ├── install.sh                       # Feature entry point, delegates to install-tools.sh
    └── README.md                        # Feature usage documentation
```

## Shared Install Logic

`scripts/install-tools.sh` is the single source of truth for installing Warden's terminal infrastructure. It is used by both the Dockerfile and the devcontainer feature. The script:

1. Installs runtime system deps (git, curl, jq, procps, iproute2, psmisc, iptables)
2. Installs gosu if not already present (downloaded as a static binary). When used from the multi-stage Dockerfile, gosu is pre-built in the builder stage and this step is skipped
3. Compiles abduco from source if not already present. When used from the multi-stage Dockerfile, abduco is pre-built in the builder stage and this step is skipped
4. Creates `dev` non-root user
5. Installs Claude Code CLI via official installer
6. Sets up `/home/dev` and `/home/dev/.claude` directories
7. Adds env var forwarding to `/home/dev/.bashrc`
8. Copies terminal scripts to `/usr/local/bin/`
9. Creates Claude Code managed settings at `/etc/claude-code/managed-settings.json` with hooks for attention tracking and event logging

All steps are idempotent. Environment variables: `ABDUCO_VERSION` (default: `0.6`), `GOSU_VERSION` (default: `1.17`).

## Devcontainer Feature

The devcontainer feature (`container/devcontainer-feature/`) packages Warden's terminal infrastructure as an OCI artifact at `ghcr.io/thesimonho/warden/session-tools`. Users who use devcontainers can add this feature to their `.devcontainer/devcontainer.json` to bake Warden infrastructure into their image, then pass the built image to Warden like any custom image.

At CI publish time (`.github/workflows/devcontainer-feature.yml`), scripts from `container/scripts/` are copied into the feature directory before packaging.
