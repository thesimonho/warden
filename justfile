# Warden – task runner
# https://just.systems

set dotenv-load := true

default:
    @just --list

# ── Development ──────────────────────────────────────────────────────────────
# Clean up node_modules and dist directories
clean:
  rm -rf web/node_modules
  rm -rf web/dist
  rm -rf docs/site/node_modules
  rm -rf docs/site/dist

[private]
dev-api:
    WARDEN_NO_OPEN=1 go run ./cmd/warden-desktop

[private]
dev-web:
    npm --prefix web run dev

# Build and run the TUI
dev-tui:
    go run ./cmd/warden-tui

# Start Go + Vite dev servers
dev:
    #!/usr/bin/env bash
    # Fail fast if either port is already occupied.
    for port in 8090 5173; do
        if lsof -ti :"$port" >/dev/null 2>&1; then
            echo "ERROR: port $port is already in use. Run 'just kill' first." >&2
            exit 1
        fi
    done
    trap 'kill 0' EXIT
    just dev-api &
    just dev-web &
    wait

# Kill dev servers (warden-desktop, vite) on ports 8090 and 5173
kill:
    #!/usr/bin/env bash
    killed=0
    for port in 8090 5173; do
        pids=$(lsof -ti :"$port" 2>/dev/null || true)
        if [ -n "$pids" ]; then
            echo "$pids" | xargs kill 2>/dev/null && echo "Killed process(es) on :$port" && killed=1
        fi
    done
    [ "$killed" -eq 0 ] && echo "No dev servers running"
    exit 0

# Regenerate OpenAPI 3.1 spec from swag annotations
openapi:
    swag init --v3.1 --parseInternal --parseDependency --generalInfo internal/server/doc.go --output docs/openapi --outputTypes yaml

# ── Build ────────────────────────────────────────────────────────────────────

[private]
build-web:
    npm --prefix web run build
    rm -rf internal/server/ui
    cp -r web/dist internal/server/ui
    touch internal/server/ui/.gitkeep

# Build all binaries → bin/
build: build-web
    go build -o bin/warden ./cmd/warden
    go build -o bin/warden-desktop ./cmd/warden-desktop
    go build -o bin/warden-tui ./cmd/warden-tui

# Build project container image for available runtimes
build-container:
    #!/usr/bin/env bash
    # Query latest CLI versions so Docker only rebuilds those layers when
    # an upstream release changes. Falls back to "unknown" (permanent cache)
    # if the version check fails (offline builds).
    CLAUDE_V=$(curl -sfL "https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases/latest" 2>/dev/null || echo "unknown")
    CODEX_V=$(npm view @openai/codex version 2>/dev/null || echo "unknown")
    echo "CLI versions: claude=${CLAUDE_V} codex=${CODEX_V}"
    BUILD_ARGS="--build-arg CLAUDE_VERSION=${CLAUDE_V} --build-arg CODEX_VERSION=${CODEX_V}"
    if command -v docker >/dev/null 2>&1; then
        echo "Building with Docker..."
        docker build --platform "linux/$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')" ${BUILD_ARGS} -t ghcr.io/thesimonho/warden:latest ./container
    fi
    if command -v podman >/dev/null 2>&1; then
        echo "Building with Podman..."
        podman build ${BUILD_ARGS} -t ghcr.io/thesimonho/warden:latest ./container
    fi

# ── Quality ──────────────────────────────────────────────────────────────────

# Run checkers and formatters
check: lint-go lint-web typecheck

[private]
lint-go:
    golangci-lint run ./...

[private]
lint-web:
    npm --prefix web run lint

[private]
typecheck:
    npm --prefix web run typecheck

# Format code
format: format-go format-web

[private]
format-web:
    npm --prefix web run format

[private]
format-go:
    go fmt ./...

# Run all unit tests
test: test-go test-web

[private]
test-go:
    go test ./...

[private]
test-web:
    npm --prefix web run test

[private]
test-e2e:
    just clean-e2e
    npm --prefix web run test:e2e

# Per-runtime timeout for E2E matrix (seconds). A healthy run takes ~3 minutes;
# exceeding this indicates a hang that needs manual inspection.
e2e_timeout := "240"

# Run E2E tests against available runtimes (docker + podman)
test-e2e-matrix:
    #!/usr/bin/env bash
    set -euo pipefail

    runtimes=()
    for rt in docker podman; do
        if command -v "$rt" >/dev/null 2>&1 && "$rt" info >/dev/null 2>&1; then
            runtimes+=("$rt")
        fi
    done

    if [ ${#runtimes[@]} -eq 0 ]; then
        echo "No container runtimes available"
        exit 1
    fi

    if [ ${#runtimes[@]} -eq 1 ]; then
        echo "Only ${runtimes[0]} is available — run 'just test-e2e' instead for single-runtime tests"
        exit 1
    fi

    echo "Running E2E matrix for: ${runtimes[*]}"
    echo ""

    failed=()
    for rt in "${runtimes[@]}"; do
        echo "══════════════════════════════════════"
        echo "  E2E: $rt (timeout: {{ e2e_timeout }}s)"
        echo "══════════════════════════════════════"

        just clean-e2e >/dev/null 2>&1
        just kill >/dev/null 2>&1 || true

        if timeout {{ e2e_timeout }} env WARDEN_RUNTIME="$rt" npm --prefix web --silent run test:e2e; then
            echo "✓ $rt passed"
        elif [ $? -eq 124 ]; then
            echo "✗ $rt timed out after {{ e2e_timeout }}s — likely hung, needs inspection"
            failed+=("$rt")
        else
            echo "✗ $rt failed"
            failed+=("$rt")
        fi

        just kill >/dev/null 2>&1 || true
        echo ""
    done

    if [ ${#failed[@]} -gt 0 ]; then
        echo "FAILED runtimes: ${failed[*]}"
        exit 1
    fi

    echo "All runtimes passed: ${runtimes[*]}"

# Clean up E2E test containers
clean-e2e:
    #!/usr/bin/env bash
    for runtime in podman docker; do
        if command -v "$runtime" >/dev/null 2>&1; then
            "$runtime" ps -a --filter "name=warden-e2e-" --format '{{{{.Names}}' | \
                xargs -r "$runtime" rm -f 2>/dev/null
        fi
    done
    rm -rf /tmp/warden-e2e-workspace /tmp/warden-e2e-db
    echo "E2E cleanup complete"

# ── Packaging ────────────────────────────────────────────────────────────────

# Generate all platform icons from icon.svg (square) and copy logo.svg (wide)
generate-icons:
    #!/usr/bin/env bash
    set -euo pipefail
    ICON="icon.svg"
    LOGO="logo.svg"
    for f in "${ICON}" "${LOGO}"; do
        if [ ! -f "${f}" ]; then
            echo "Missing ${f}" >&2
            exit 1
        fi
    done
    # Rasterize square icon SVG to a high-res intermediate PNG for crisp downscaling.
    # Uses a solid white background so the icon is readable on all platforms.
    magick -density 384 -background white "${ICON}" -flatten -resize 1024x1024 /tmp/warden-icon-1024.png
    HI="/tmp/warden-icon-1024.png"
    # Linux — 512px PNG
    magick "${HI}" -resize 512x512 packaging/linux/warden.png
    # Windows — multi-size .ico
    magick "${HI}" \
        \( -clone 0 -resize 16x16 \) \
        \( -clone 0 -resize 32x32 \) \
        \( -clone 0 -resize 48x48 \) \
        \( -clone 0 -resize 64x64 \) \
        \( -clone 0 -resize 128x128 \) \
        \( -clone 0 -resize 256x256 \) \
        -delete 0 packaging/windows/warden.ico
    # macOS — iconset (converted to .icns by bundle.sh on macOS)
    mkdir -p packaging/macos/warden.iconset
    for pair in "16x16:icon_16x16" "32x32:icon_16x16@2x" "32x32:icon_32x32" \
                "64x64:icon_32x32@2x" "128x128:icon_128x128" "256x256:icon_128x128@2x" \
                "256x256:icon_256x256" "512x512:icon_256x256@2x" "512x512:icon_512x512"; do
        size="${pair%%:*}"
        name="${pair#*:}"
        magick "${HI}" -resize "${size}" "packaging/macos/warden.iconset/${name}.png"
    done
    # Web — favicon and PWA icons (served from web/public/)
    mkdir -p web/public
    magick "${HI}" -resize 32x32 web/public/favicon.ico
    magick "${HI}" -resize 180x180 web/public/apple-touch-icon.png
    magick "${HI}" -resize 192x192 web/public/favicon-192.png
    magick "${HI}" -resize 512x512 web/public/favicon-512.png
    # Copy SVGs for direct use in web UI (transparent, theme-adapted via CSS)
    cp "${ICON}" web/public/icon.svg
    cp "${LOGO}" web/public/logo.svg
    rm -f "${HI}"
    echo "Generated icons for Linux, Windows, macOS, and web"

# ── Documentation ─────────────────────────────────────────────────────────────

[private]
docs-generate:
    ./docs/site/generate-go-docs.sh
    ./docs/site/generate-changelog.sh
    ./docs/site/generate-contributing.sh

# Start docs dev server (generates Go reference docs first)
docs-dev: docs-generate
    npm --prefix docs/site run dev

# Build docs site (generates Go docs + builds Starlight)
docs-build: docs-generate
    npm --prefix docs/site run build

# Build and preview docs site locally
docs-preview: docs-build
    npm --prefix docs/site run preview

# ── Setup ────────────────────────────────────────────────────────────────────

# Install Go and Node dependencies
install:
    go mod download
    npm --prefix web install
    npm --prefix docs/site install
