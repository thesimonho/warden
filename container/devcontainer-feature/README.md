# Warden Terminal Infrastructure (devcontainer feature)

Installs the terminal infrastructure required by [Warden](https://github.com/thesimonho/warden) into any devcontainer image.

## What it installs

- **abduco** — terminal persistence across viewer disconnections
- **gosu** — lightweight privilege drop for the entrypoint
- **Claude Code CLI** — AI coding assistant
- **Codex CLI** — AI coding assistant (OpenAI)
- **Terminal scripts** — `create-terminal.sh`, `disconnect-terminal.sh`, `entrypoint.sh`, `user-entrypoint.sh`
- **`dev` user** — non-root user for running terminals

## Usage

Add the feature to your `.devcontainer/devcontainer.json`, build the image with your preferred tooling, and pass the resulting image name to Warden when creating a project:

```jsonc
{
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "features": {
    "ghcr.io/thesimonho/warden/session-tools:1": {}
  }
}
```

Build the image:

```bash
devcontainer build --workspace-folder . --image-name my-warden-image:latest
```

Then create a project in Warden using `my-warden-image:latest` as the image.

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `abducoVersion` | string | `0.6` | Version of abduco to install |
| `gosuVersion` | string | `1.17` | Version of gosu to install |

## Notes

- Requires a Debian/Ubuntu-based image (`apt-get` is used for package installation).
- Creates a `dev` user (UID auto-assigned). Warden terminal scripts run as this user.
