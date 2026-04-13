# Dev Environment

When working in this repository, you are in the **dev environment**. The user may simultaneously be running Warden in production. Dev, production, and E2E environments are fully isolated — never cross the boundary.

## Port assignments

| Environment            | UI              | API (direct)  | DB location               | Container suffix |
| ---------------------- | --------------- | ------------- | ------------------------- | ---------------- |
| **Production**         | `:8090`         | `:8090`       | `~/.config/warden/`       | (none)           |
| **Development**        | `:5173` (Vite)  | `:8091` (Go)  | `~/.cache/warden-dev/`    | `-dev`           |
| **E2E tests**          | —               | `:8092`       | `~/.cache/warden-e2e-db/` | `-dev`           |

## Rules

1. **Use `http://localhost:5173` for all UI/browser work.** This is the Vite dev server with HMR — frontend changes appear instantly. The Go backend on `:8091` serves a stale build and should not be used for browsing the UI.
2. **Use `:8091` for direct API calls.** When curling the API, use `http://localhost:8091/api/v1/...`. Vite proxies `/api/*` to `:8091`, so `curl http://localhost:5173/api/v1/...` also works.
3. **Never touch port `:8090`.** That is the user's production instance with real data. Do not curl it, probe it, or start a server on it.
4. **Never read or write `~/.config/warden/`.** That is the production database. The dev database is at `~/.cache/warden-dev/warden.db`.
5. **Dev containers have a `-dev` suffix.** When looking up containers in Docker, dev containers are named `{name}-dev`. Production containers have no suffix.
6. **Do not start dev servers.** The user runs `just dev` themselves. Ask them if you need the server running.

## Examples

```bash
# Correct — dev API
curl http://localhost:8091/api/v1/projects
curl http://localhost:8091/api/v1/settings

# Also correct — through Vite proxy
curl http://localhost:5173/api/v1/projects

# Wrong — production server
curl http://localhost:8090/api/v1/projects
```

When using the browser (agent-browser skill, Playwright, etc.), navigate to `http://localhost:5173`.
