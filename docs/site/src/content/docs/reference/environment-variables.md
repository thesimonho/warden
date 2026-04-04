---
title: Environment Variables
description: All environment variables recognized by Warden binaries.
---

Environment variables configure Warden's server binaries at startup. None are required — all have sensible defaults.

## Server configuration

| Variable           | Default             | Description                                        |
| ------------------ | ------------------- | -------------------------------------------------- |
| `ADDR`             | `127.0.0.1:8090`    | Listen address for the HTTP API server.            |
| `WARDEN_DB_DIR`    | `~/.config/warden/` | Directory for the SQLite database and event files. |
| `WARDEN_LOG_LEVEL` | `info`              | Log level (`debug`, `info`, `warn`, `error`).      |

## Behavior toggles

| Variable                 | Default      | Description                                                                       |
| ------------------------ | ------------ | --------------------------------------------------------------------------------- |
| `WARDEN_NO_UPDATE_CHECK` | _(disabled)_ | Set to `1` to skip the startup version check against GitHub releases.             |
| `WARDEN_NO_OPEN`         | _(disabled)_ | Set to any value to prevent `warden-desktop` from opening the browser on startup. |

## Runtime detection

| Variable      | Default           | Description                              |
| ------------- | ----------------- | ---------------------------------------- |
| `DOCKER_HOST` | _(auto-detected)_ | Overrides the Docker daemon socket path. |

## Container-internal variables

These are set automatically by the engine when creating containers. They are not user-configurable — listed here for reference when writing container scripts or debugging.

| Variable                  | Description                                                                         |
| ------------------------- | ----------------------------------------------------------------------------------- |
| `WARDEN_CONTAINER_NAME`   | Container name, used by hook scripts in event payloads.                             |
| `WARDEN_PROJECT_ID`       | Deterministic 12-char hex project identifier (SHA-256 of host path).                |
| `WARDEN_AGENT_TYPE`       | Agent type running in this container (`claude-code` or `codex`).                    |
| `WARDEN_WORKSPACE_DIR`    | Workspace directory inside the container (e.g. `/home/warden/my-project`).          |
| `WARDEN_EVENT_DIR`        | Bind-mounted directory where hook scripts write event files (`/var/warden/events`). |
| `WARDEN_HOST_UID`         | Host user ID — the entrypoint remaps the container user to match file ownership.    |
| `WARDEN_HOST_GID`         | Host group ID — paired with `WARDEN_HOST_UID`.                                      |
| `WARDEN_NETWORK_MODE`     | Network isolation mode (`full`, `restricted`, or `none`).                           |
| `WARDEN_ALLOWED_DOMAINS`  | Comma-separated domain allowlist for `restricted` network mode.                     |
| `WARDEN_ENABLED_RUNTIMES` | Comma-separated runtime IDs to install at startup (e.g. `node,python,go`).          |
| `WARDEN_CLAUDE_VERSION`   | Pinned Claude Code CLI version to install at startup (from `agent/versions.go`).    |
| `WARDEN_CODEX_VERSION`    | Pinned Codex CLI version to install at startup (from `agent/versions.go`).          |
