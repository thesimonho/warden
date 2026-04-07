# Codemaps

Quick-reference maps of Warden's codebase. Each file covers one focused area — read only what you need.

## How to Use

Find the topic you need below and read that specific file. Avoid reading files outside your area of interest.

## Backend (`backend/`)

Go packages that make up the server, engine, and supporting infrastructure.

| File                                   | Covers                                                                                                          | Key packages                                                                                    |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| [supporting.md](backend/supporting.md) | Entry points, binaries, version check, access items, agent provider, watcher, Docker detection, terminal proxy | `warden`, `cmd/`, `version/`, `access/`, `watcher/`, `agent/`, `docker/`, `internal/terminal/` |
| [api-types.md](backend/api-types.md)   | API contract types, Go HTTP client                                                                              | `api/`, `client/`                                                                               |
| [api-server.md](backend/api-server.md) | HTTP handlers, routes, middleware, full endpoint table                                                          | `internal/server/`                                                                              |
| [service.md](backend/service.md)       | Business logic: projects, worktrees, containers, budget, audit, settings                                        | `service/`                                                                                      |
| [engine.md](backend/engine.md)         | Container engine API, worktree management, container creation                                                   | `engine/`                                                                                       |
| [events.md](backend/events.md)         | Event log storage, event bus watcher, SSE broker, liveness                                                      | `eventlog/`, `eventbus/`                                                                        |
| [database.md](backend/database.md)     | SQLite schema, store operations, audit writer                                                                   | `db/`                                                                                           |

## Frontend (`frontend/`)

React SPA at `web/src/`.

| File                                          | Covers                                                                 |
| --------------------------------------------- | ---------------------------------------------------------------------- |
| [app-structure.md](frontend/app-structure.md) | Entry files, pages, routes, themes, text sizing                        |
| [components.md](frontend/components.md)       | All components: shared, home, project, UI primitives                   |
| [libraries.md](frontend/libraries.md)         | `lib/` modules (API client, types, canvas, utils), external deps       |
| [hooks.md](frontend/hooks.md)                 | All React hooks (projects, worktrees, terminal, notifications, canvas) |
| [testing.md](frontend/testing.md)             | Vitest unit tests, Playwright E2E tests, test infrastructure           |

## TUI (`tui/`)

Bubble Tea v2 terminal UI at `internal/tui/`.

| File                                   | Covers                                                                  |
| -------------------------------------- | ----------------------------------------------------------------------- |
| [architecture.md](tui/architecture.md) | Design principles, key files, client interface, keybindings             |
| [views.md](tui/views.md)               | All views: projects, project detail, container form, settings, audit    |
| [components.md](tui/components.md)     | Shared components: status dot, cost, tab bar, colors, directory browser |

## Container (`container/`)

Container image, scripts, and security at `container/`.

| File                                       | Covers                                                                             |
| ------------------------------------------ | ---------------------------------------------------------------------------------- |
| [image.md](container/image.md)             | Dockerfile, install-tools.sh, devcontainer feature                                 |
| [security.md](container/security.md)       | Capability dropping, seccomp profile, external network isolation, sudo support     |
| [scripts.md](container/scripts.md)         | Terminal lifecycle scripts, event scripts, attention tracking, audit logging modes |
| [environment.md](container/environment.md) | Process architecture, env var forwarding, storage layout, event bus communication  |
