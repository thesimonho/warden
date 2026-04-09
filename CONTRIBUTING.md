# Contributing

Thanks for your interest in contributing to Warden! This guide covers everything you need to get started.

## Finding something to work on

- **Good first issues** are labeled [`good first issue`](https://github.com/thesimonho/warden/labels/good%20first%20issue) on GitHub.
- **Feature requests and bugs** are tracked in [GitHub Issues](https://github.com/thesimonho/warden/issues). Comment on an issue to claim it before starting work.
- If you have an idea that isn't tracked yet, open an issue first to discuss the approach.

## Prerequisites

- [Go 1.26+](https://go.dev/dl/)
- [Node.js 24+](https://nodejs.org/)
- [Docker](https://docs.docker.com/get-docker/) (for running containers locally)
- [Dev Container CLI](https://github.com/devcontainers/cli) (for E2E tests)
- [just](https://just.systems/) (optional task runner — see `justfile` for available recipes)

## Setup

```bash
git clone https://github.com/thesimonho/warden.git
cd warden
go mod download
npm --prefix web install
```

Start the dev servers (Go + Vite):

```bash
just dev
```

Open `http://localhost:5173` (Vite proxies `/api/*` to Go on `:8090`).

### Database isolation

The dev server uses a separate database (`~/.cache/warden-dev/warden.db`) so development and testing don't interfere with your production Warden data at `~/.config/warden/`. E2E tests use their own database at `~/.cache/warden-e2e-db/`.

| Environment                   | DB location                | Port            |
| ----------------------------- | -------------------------- | --------------- |
| Production (`warden-desktop`) | `~/.config/warden/`        | `:8090`         |
| Development (`just dev`)      | `~/.cache/warden-dev/`     | `:8090`         |
| E2E tests (standalone)        | `~/.cache/warden-e2e-db/`  | `:8090`         |
| E2E tests (piggybacking)      | Same as dev server         | Dev server port |

Note: dev and production both use `:8090`, so they cannot run simultaneously. If you need to use Warden as a user, stop the dev server first.

E2E tests will piggyback on a running dev server if one is available, otherwise they start their own. Test projects are cleaned up automatically after each run.

## Local container image

The container image is built on CI when changes to `container/` are pushed to `main`. For faster iteration during development, build the image locally:

```bash
docker build -t ghcr.io/thesimonho/warden:latest ./container
```

New containers created from the dashboard will use the locally built image. Existing containers need to be recreated to pick up the changes.

## Testing

```bash
go test ./...                    # Go unit tests
npm --prefix web run test        # Frontend unit tests (Vitest)
npm --prefix web run typecheck   # TypeScript type checking
npm --prefix web run test:e2e    # E2E tests (Playwright)
```

Run a single test:

```bash
go test ./engine/ -run TestParseGitWorktreeList   # Single Go test
npm --prefix web run test -- --run lib/cost.test.ts   # Single frontend test
npm --prefix web run test:e2e -- --grep "should connect terminal"   # Single E2E test
```

## Code quality checks

Run these before submitting a PR:

```bash
# Go
golangci-lint run ./...          # Linting

# Frontend
npm --prefix web run format      # Prettier formatting
npm --prefix web run lint        # ESLint
npm --prefix web run typecheck   # TypeScript type checking
```

Go formatting is handled automatically by `gofmt` via gopls.

## Architecture

For architecture diagrams, project structure, and how the engine, API, and clients fit together, see the [Architecture](https://thesimonho.github.io/warden/integration/architecture/) page.

Key directories for contributors:

| Directory            | What lives here                                               |
| -------------------- | ------------------------------------------------------------- |
| `engine/`            | Container engine API wrapper (Docker)                         |
| `service/`           | Business logic layer                                          |
| `api/`               | API contract types (request/response structs)                 |
| `db/`                | SQLite database store                                         |
| `eventbus/`          | File-based event system (watcher, SSE broker)                 |
| `agent/`             | Multi-agent abstraction (registry, parsers, status providers) |
| `internal/server/`   | HTTP server, API routes, middleware                           |
| `internal/terminal/` | WebSocket-to-PTY proxy                                        |
| `web/`               | React + Vite frontend                                         |
| `container/`         | Project container image and devcontainer feature              |

For detailed code maps, see [`docs/developer/codemaps/README.md`](docs/developer/codemaps/README.md) for an index of all maps.

## Key architectural rules

These rules are important to follow when contributing:

1. **The web SPA must only use HTTP calls to `/api/v1/*`** — it serves as a reference implementation for developers building their own frontends.
2. **The TUI must be written against a `Client` interface** — satisfiable by both the embedded service and the HTTP client.
3. **API routes include agentType as a path segment** — all project-scoped routes follow the pattern `/api/v1/projects/{projectId}/{agentType}/...` to enforce the compound primary key (projectID + agentType).
4. **New API types go in `api/`** — shared by `service/`, `client/`, and the TUI.
5. **`internal/` packages stay internal** — `server/` and `terminal/` are HTTP plumbing, not public API.
6. **All audit writes go through `db.AuditWriter`** — never call `db.Store.Write()` directly for audit events.
7. **PRs touching `agent/` should include tests for both parsers** — Claude Code and Codex each have their own JSONL parser in `agent/claudecode/` and `agent/codex/`. Changes to shared parsing logic must be validated against both.
8. **Agent CLIs are installed at container startup, not in the image** — `install-agent.sh` installs the correct CLI using pinned versions from `agent/versions.go`. Agent-specific event scripts live in `container/scripts/claude/` and `container/scripts/codex/`.

## Submitting a pull request

### Branch strategy

Trunk-based. Branch off `main` for features/fixes, PR back in when ready. Squash merged to `main`.

Merges to `main` trigger release-please PR and changelog generation. Merging the release-please PR cuts a release and triggers builds/deployments/tags.

### PR checklist

Before opening a PR:

- [ ] Code compiles and passes all tests (`go test ./...`, `npm --prefix web run test`)
- [ ] Frontend type checks pass (`npm --prefix web run typecheck`)
- [ ] Linting passes (`golangci-lint run ./...`, `npm --prefix web run lint`)
- [ ] Frontend code is formatted (`npm --prefix web run format`)
- [ ] New API endpoints have OpenAPI annotations (see `internal/server/routes.go`)
- [ ] Documentation is updated if behavior changed

### Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add cost export to CSV
fix: resolve WebSocket reconnection race
refactor: extract symlink resolver from engine
docs: update integration guide for Go client
test: add E2E tests for worktree lifecycle
```

## CI/CD

Your PR will be checked by these workflows:

| Workflow                  | Trigger                                  | What it does                                                                              |
| ------------------------- | ---------------------------------------- | ----------------------------------------------------------------------------------------- |
| `ci.yml`                  | PRs targeting `main`                     | Go tests, TS typecheck, TS tests                                                          |
| `release-please.yml`      | Push to `main`                           | Automated release creation and version tagging                                            |
| `release-build.yml`       | Release published, or manual dispatch    | Cross-platform builds, installers (DMG/deb/rpm/Arch/AppImage/Inno Setup), checksums, SBOM |
| `container.yml`           | Push to `main` (`container/**`), release | Build container image + devcontainer feature (`:latest` on push, semver on release)       |
| `container-scheduled.yml` | Daily schedule (5 AM UTC)                | Validates container image, builds CLIs, validates JSONL parsers, pushes only on success   |

## Stack

| Layer     | Technology                                     |
| --------- | ---------------------------------------------- |
| Backend   | Go (`net/http`), Docker Engine API             |
| Frontend  | React 19, Vite 7, TypeScript                   |
| UI        | shadcn/ui, Tailwind CSS v4                     |
| Terminal  | xterm.js via WebSocket to Go proxy             |
| Container | Ubuntu 24.04, tmux, Claude Code CLI, Codex CLI |
| Dev tools | just (task runner)                             |

## More resources

- [Architecture](https://thesimonho.github.io/warden/integration/architecture/) — system diagrams, communication pathways, and data flow funnels
- [Integration Paths](https://thesimonho.github.io/warden/integration/paths/) — how the engine, API, and clients fit together
- [HTTP API Reference](https://thesimonho.github.io/warden/reference/api/) — all API endpoints
- [Go Package Reference](https://thesimonho.github.io/warden/reference/go/) — Go package documentation
