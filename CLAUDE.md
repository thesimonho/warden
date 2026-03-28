# CLAUDE.md

## What is Warden

A container engine and control plane for running multiple Claude Code agents in isolated containers. Shipped as three binaries: `warden` (headless API server), `warden-desktop` (web UI), and `warden-tui` (terminal UI). The engine is also importable as a Go library. Supports Docker and Podman.

## Terminology (MUST follow)

Read @docs/terminology.md before writing any code. It defines the required terms (Project, Worktree, Terminal), banned terms (Session, Conversation), worktree states, terminal actions, Claude activity sub-states, and the ownership boundary between Warden and Claude Code.

## Commands

### Development

```bash
go run ./cmd/warden-desktop   # Go API server + embedded SPA
npm --prefix web run dev      # Vite dev server (optional, for frontend HMR)
```

Dev UI at `http://localhost:5173`. Always use `:5173` to access the app and API in development — the Go server on `:8090` does not serve the SPA when the Vite dev server is running.

### Debugging

Proactively use the agent-browser skill to check and debug frontend code.

Use warden's audit api to track down container and server bugs.

### Testing & Quality

```bash
go test ./...                          # Go unit tests
npm --prefix web run test              # Frontend unit tests (Vitest)
npm --prefix web run typecheck         # TypeScript type checking
golangci-lint run ./...                # Go linter
npm --prefix web run format            # Format frontend code (Prettier)
npm --prefix web run lint              # Frontend linter (ESLint)
```

Run a single Go test:

```bash
go test ./engine/ -run TestParseGitWorktreeList
```

Run a single frontend test:

```bash
npm --prefix web run test -- --run lib/cost.test.ts
```

E2E tests (auto-builds frontend and starts Go backend if no server is running):

```bash
npm --prefix web run test:e2e              # Current runtime
just test-e2e-matrix                       # Both Docker and Podman (4min timeout per runtime)
WARDEN_RUNTIME=podman npm --prefix web run test:e2e   # Specific runtime
```

If the Vite dev server (`:5173`) or Go backend (`:8090`) is already running, E2E tests reuse it. Otherwise Playwright builds the frontend and starts `warden-desktop` automatically. No manual server setup required.

Run a single E2E test:

```bash
npm --prefix web run test:e2e -- --grep "should connect terminal"
```

### Build

```bash
npm --prefix web run build                        # Frontend → web/dist/
go build -o bin/warden ./cmd/warden               # Headless server
go build -o bin/warden-desktop ./cmd/warden-desktop   # Desktop binary
```

### OpenAPI Spec

```bash
swag init --v3.1 --parseInternal --parseDependency --generalInfo internal/server/doc.go --output docs/openapi --outputTypes yaml
```

Regenerate after changing any `@Router` or `@Param`/`@Success`/`@Failure` annotations in `internal/server/routes.go`. See `.claude/rules/openapi.md` for details.

### Documentation Site

```bash
just docs-dev              # Starlight dev server (generates Go docs first)
just docs-build            # Full production build (gomarkdoc + Starlight)
just docs-preview          # Build and preview locally
```

The docs site lives in `docs_site/`. Go package reference pages are auto-generated at build time by `gomarkdoc` and are gitignored. OpenAPI reference pages are auto-generated from `docs/openapi/swagger.yaml` by the `starlight-openapi` plugin.
