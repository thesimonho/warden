---
name: warden
description: Integration guide for Warden — the container engine for AI agents. Provides feature-level reference material and helps integrate Warden into your project via HTTP API, Go client, or Go library.
---

# Warden Integration

Warden is a container engine and control plane for running AI coding agents (Claude Code, Codex) in isolated Docker containers. It ships as a headless API server, a desktop app, and a TUI — or you can import it as a Go library.

## Integration Paths

Warden offers three ways to integrate:

| Path           | When to use                                                  | Package                               |
| -------------- | ------------------------------------------------------------ | ------------------------------------- |
| **HTTP API**   | Any language. Run the `warden` binary, talk to it over HTTP. | Endpoints at `/api/v1/*`              |
| **Go Client**  | Go projects talking to a running Warden server.              | `github.com/thesimonho/warden/client` |
| **Go Library** | Go projects embedding the engine directly in-process.        | `github.com/thesimonho/warden`        |

All three paths expose the same features. Choose based on your language and deployment model.

## Features

Each feature has a dedicated reference file. Read only what you need for the integration task at hand.

| Feature               | Reference                                   | What it covers                                                     |
| --------------------- | ------------------------------------------- | ------------------------------------------------------------------ |
| Core concepts         | `reference/concepts.md`                     | Architecture, project identity, container lifecycle, terminology   |
| Integration paths     | `reference/paths.md`                        | Binaries, integration approaches, which to choose                  |
| Projects              | `reference/features/projects.md`            | Project CRUD, templates, configuration, agent types                |
| Containers            | `reference/features/containers.md`          | Create, update, lifecycle, runtimes, image management              |
| Worktrees & Terminals | `reference/features/worktrees-terminals.md` | Worktree states, terminal actions, WebSocket protocol, auto-resume |
| Events                | `reference/features/events.md`              | SSE event types, real-time subscriptions, state transitions        |
| Cost & Budget         | `reference/features/cost-budget.md`         | Cost tracking, budget enforcement, models per agent type           |
| Audit                 | `reference/features/audit.md`               | Logging modes, categories, queries, export                         |
| Access                | `reference/features/access.md`              | Credentials, mounts, source/injection types                        |
| Network               | `reference/features/network.md`             | Network modes, domain allowlisting                                 |
| Environment variables | `reference/environment-variables.md`        | Env vars for configuration                                         |
| HTTP API examples     | `reference/examples/api.md`                 | HTTP API usage examples (curl, TypeScript, Python, Go)             |
| Go Client examples    | `reference/examples/client.md`              | Go client examples                                                 |
| Go Library examples   | `reference/examples/library.md`             | Go library (embedded engine) examples                              |
| API Reference         | `reference/api/` (auto-generated)           | Per-resource endpoint reference from OpenAPI spec                  |

## How to Use This Skill

1. **New to Warden?** Start with `reference/concepts.md` to understand the architecture and identity model.
2. **Know what feature you need?** Jump directly to that feature's reference file.
3. **Not sure where to start?** Run the `warden-surveyor` agent to scan your codebase and identify integration points.

## Documentation

Full documentation, including auto-generated API reference and Go package docs: <https://thesimonho.github.io/warden/>
