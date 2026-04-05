---
title: Installation
description: Platform-specific installation instructions for all three Warden binaries.
---

## Download

Grab the installer for your platform from the [releases page](https://github.com/thesimonho/warden/releases).

## Linux

Choose the format that suits your system:

| Format                  | File                                  | Install method                        |
| ----------------------- | ------------------------------------- | ------------------------------------- |
| **deb** (Debian/Ubuntu) | `warden-desktop_*_amd64.deb`          | `sudo dpkg -i *.deb`                  |
| **rpm** (Fedora/RHEL)   | `warden-desktop-*.x86_64.rpm`         | `sudo rpm -i *.rpm`                   |
| **Arch**                | `warden-desktop-*-x86_64.pkg.tar.zst` | `sudo pacman -U *.pkg.tar.zst`        |
| **AppImage** (portable) | `warden-desktop-linux-amd64.AppImage` | `chmod +x *.AppImage && ./*.AppImage` |

ARM64 builds are also available for all formats. AppImages support delta updates via [AppImageUpdate](https://github.com/AppImage/AppImageUpdate).

### Headless server and TUI

The headless server (`warden`) and TUI (`warden-tui`) are distributed as standalone binaries:

```bash
# Download (replace binary name and architecture as needed)
curl -L -o warden https://github.com/thesimonho/warden/releases/latest/download/warden-linux-amd64
chmod +x warden
```

## macOS

Download the DMG from the releases page:

1. Open `warden-desktop-macos-universal.dmg`
2. Drag **Warden.app** to Applications
3. On first launch, right-click → Open to bypass Gatekeeper (the app is not yet code-signed)

The DMG contains a universal binary that runs natively on both Intel and Apple Silicon Macs.

## Windows

Download and run `Warden-Setup-amd64.exe`. The installer optionally adds Warden to your PATH and creates a desktop shortcut.

## Prerequisites

Warden supports two agent types: **Claude Code** (Anthropic) and **Codex** (OpenAI). Each project is locked to one agent at creation time.

- **Claude Code** requires an Anthropic API key (`ANTHROPIC_API_KEY`) or a Claude Pro/Max subscription.
- **Codex** requires an OpenAI API key (`OPENAI_API_KEY`) or a ChatGPT Pro/Plus subscription.

Both CLIs are bundled in the container image — no additional installation needed.

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
