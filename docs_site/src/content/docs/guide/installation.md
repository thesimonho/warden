---
title: Installation
description: Platform-specific installation instructions for all three Warden binaries.
---

## Download

Grab the binary for your platform from the [releases page](https://github.com/thesimonho/warden/releases). Each release includes builds for:

- **Linux** — `amd64` and `arm64`
- **macOS** — `amd64` (Intel) and `arm64` (Apple Silicon)
- **Windows** — `amd64` and `arm64`

## Linux

```bash
# Download (replace with your architecture)
curl -L -o warden-desktop https://github.com/thesimonho/warden/releases/latest/download/warden-desktop-linux-amd64

# Make executable
chmod +x warden-desktop

# Run
./warden-desktop
```

## macOS

```bash
# Download (Apple Silicon)
curl -L -o warden-desktop https://github.com/thesimonho/warden/releases/latest/download/warden-desktop-darwin-arm64

# Make executable
chmod +x warden-desktop

# Remove quarantine (macOS Gatekeeper)
xattr -d com.apple.quarantine warden-desktop

# Run
./warden-desktop
```

## Windows

Download the `.exe` from the releases page and run it directly. No installation required.

## Container image

The container image is pulled automatically on first use. To pull it manually:

```bash
docker pull ghcr.io/thesimonho/warden:latest
```

## Building from source

Requires [Git](https://git-scm.com/downloads), [Go 1.26+](https://go.dev/dl/), and [Node.js 24+](https://nodejs.org/).

```bash
git clone https://github.com/thesimonho/warden.git
cd warden
go mod download
npm --prefix web install
npm --prefix web run build
rm -rf internal/server/ui && cp -r web/dist internal/server/ui
go build -o bin/warden-desktop ./cmd/warden-desktop
```

See [Contributing](../../contributing/) for the full development setup.
