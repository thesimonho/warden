# Warden – task runner
# https://just.systems

set dotenv-load := true

default:
    @just --list


# ── Setup ────────────────────────────────────────────────────────────────────
# Install Go and Node dependencies
install:
    go mod download
    npm --prefix web install
    npm --prefix docs/site install

# ── Development ──────────────────────────────────────────────────────────────
# Clean up node_modules and dist directories
clean:
  rm -rf web/node_modules
  rm -rf web/dist
  rm -rf docs/site/node_modules
  rm -rf docs/site/dist

[private]
dev-api:
    WARDEN_DB_DIR="${HOME}/.cache/warden-dev" WARDEN_NO_OPEN=1 go run ./cmd/warden-desktop

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
    ./docs/openapi/generate-spec.sh

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
    cd cmd/warden-tray && CGO_ENABLED=1 go build -o ../../bin/warden-tray .

# Build project container image
build-container:
    #!/usr/bin/env bash
    # Agent CLIs are no longer baked into the image — they are installed
    # at container startup by install-agent.sh using pinned versions from
    # agent/versions.go (passed as env vars by the engine).
    docker build --platform "linux/$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')" -t ghcr.io/thesimonho/warden:latest ./container

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

# Run E2E tests
test-e2e:
    just clean-e2e
    npm --prefix web run test:e2e

# Clean up E2E test containers
clean-e2e:
    #!/usr/bin/env bash
    if command -v docker >/dev/null 2>&1; then
        docker ps -a --filter "name=warden-e2e-" --format '{{{{.Names}}' | \
            xargs -r docker rm -f 2>/dev/null
    fi
    rm -rf /tmp/warden-e2e-workspace /tmp/warden-e2e-db
    echo "E2E cleanup complete"

# ── Packaging ────────────────────────────────────────────────────────────────

# ── Documentation ─────────────────────────────────────────────────────────────

[private]
docs-generate: generate-api-docs
    ./docs/site/generate-docs.sh

# Regenerate agent-format API reference from OpenAPI spec
[private]
generate-api-docs:
    node docs/site/generate-api-docs.mjs

# Start docs dev server (generates all docs first)
docs-dev: docs-generate
    npm --prefix docs/site run dev

# Build docs site (generates all docs + builds Starlight)
docs-build: docs-generate
    npm --prefix docs/site run build

# Build and preview docs site locally
docs-preview: docs-build
    npm --prefix docs/site run preview

