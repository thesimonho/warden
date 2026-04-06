---
name: warden
description: Provides feature-level reference material and helps integrate Warden features into the project via HTTP API, Go client, or Go library. Use proactively when the user has questions or needs help with Warden, feature development/integration, or needs Warden API documentation.
argument-hint: "[feature or question]"
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

## Reference Files

The users question or feature: $ARGUMENTS

Each topic has a self-contained reference file. Read only what you need for the task at hand.

**Start here:**

| Topic             | Reference                     | What it covers                                                 |
| ----------------- | ----------------------------- | -------------------------------------------------------------- |
| Architecture      | `reference/concepts.md`       | Layered system, project identity, infrastructure layout        |
| Integration paths | `reference/paths.md`          | Binaries, key packages, decision tree for choosing an approach |
| Error handling    | `reference/error-handling.md` | Error response format, error codes, handling patterns          |

**Feature integration guides (HTTP API focused):**

| Topic         | Reference                  | What it covers                                                 |
| ------------- | -------------------------- | -------------------------------------------------------------- |
| Projects      | `reference/projects.md`    | Project lifecycle, identity, templates, stop/restart           |
| Containers    | `reference/containers.md`  | Create, configure, update vs recreate, runtimes, mounts        |
| Worktrees     | `reference/worktrees.md`   | Worktree states, create/remove/reset, cleanup, diff            |
| Terminals     | `reference/terminals.md`   | Connect/disconnect, WebSocket protocol, clipboard, auto-resume |
| Events        | `reference/events.md`      | SSE event types, payloads, reconnection strategy               |
| Network       | `reference/network.md`     | Network modes, domain allowlists, hot-reload                   |
| Access        | `reference/access.md`      | Credential passthrough, built-in/custom items, detection       |
| Audit         | `reference/audit.md`       | Logging modes, query/filter/export, custom events              |
| Cost & Budget | `reference/cost-budget.md` | Cost tracking, budget enforcement, SSE events                  |
| Settings      | `reference/settings.md`    | Global config, health check, shutdown                          |

**Additional references:**

| Topic                 | Reference                            | What it covers                                    |
| --------------------- | ------------------------------------ | ------------------------------------------------- |
| Environment variables | `reference/environment-variables.md` | Server and container env vars                     |
| Go Client examples    | `reference/examples/client.md`       | Typed Go client usage                             |
| Go Library examples   | `reference/examples/library.md`      | Embedded engine usage                             |
| API field reference   | `reference/api/` (auto-generated)    | Per-resource endpoint reference from OpenAPI spec |

## How to Use This Skill

1. **New to Warden?** Start with `reference/concepts.md` to understand the architecture and identity model.
2. **Know what feature you need?** Jump directly to that feature's reference file.
3. **Not sure where to start?** Run the `warden-surveyor` agent to scan your codebase and identify integration points.

## Documentation

Full documentation, including auto-generated API reference and Go package docs: <https://thesimonho.github.io/warden/>
