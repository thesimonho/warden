---
title: FAQ
description: Frequently asked questions about Warden.
---

## What agents does Warden support?

Currently, Warden supports [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview). More agent integrations are coming soon.

Warden's architecture is agent-agnostic — the container engine, security model, and event system are designed to work with any coding agent. Agent-specific behavior (status detection, cost tracking, event parsing) is isolated in the [`agent/`](/warden/reference/go/agent/) package, making it straightforward to add new agents.

## How is Warden different from other tools?

There are many tools for running AI coding agents and managing multi-agent workflows.

Warden is first and foremost a security-focused infrastructure layer, not an orchestrator. The container engine, security model, and REST API are the core and allows Warden to be easily integrated into other applications. The web dashboard and TUI are just reference implementations built on top.

This means you can use Warden as a standalone tool _or_ integrate it into your own tooling via the [Go library](/warden/integration/go-library/), [Go client](/warden/integration/go-client/), or [HTTP API](/warden/integration/http-api/).

For a detailed feature-by-feature breakdown, see the [Comparison](/warden/comparison/) page.

## How does Warden differ from running Docker manually?

Warden handles the infrastructure that's tedious to wire up yourself: worktree orchestration, session persistence (via abduco), terminal multiplexing, network isolation policies, real-time agent status detection, cost tracking, and an event bus for monitoring.

## Can I use my own Docker image?

Yes. You can extend the base image with a `FROM ghcr.io/thesimonho/warden` Dockerfile, use the devcontainer feature to bake Warden infrastructure into any image, or build a fully custom image. See [Custom Images](../guide/devcontainers/) for all approaches.

## What container runtimes are supported?

Warden supports both [Docker](https://docs.docker.com/get-docker/) and [Podman](https://podman.io/docs/installation). It detects the available runtime automatically. If both are available, Docker is preferred by default.

## Does network isolation work with rootless Podman?

Restricted and none network modes use iptables inside the container, which requires `NET_ADMIN` capability. This may not work with rootless Podman depending on your configuration. Full network mode works without any special capabilities.

## Why do I need to install my project dependencies again in each worktree?

Git worktrees are independent working directories — they share the `.git` history but each gets its own copy of the source tree. Dependency directories like `node_modules/`, Python virtualenvs, or Go build caches are not shared across worktrees, so each one starts without them.

This is a git worktree behavior, not specific to Warden. The same thing happens if you run `git worktree add` on your host machine.

**To install dependencies in a new worktree**, tell Claude to do it or run the install command yourself in the terminal (e.g. `npm install`, `pip install -r requirements.txt`, `go mod download`). You can also add a reminder to your project's `CLAUDE.md` so Claude knows to check for missing dependencies when starting in a new worktree.

## Why do I have so many worktrees after using Claude Code?

When Claude Code creates a worktree (via `git worktree add`), it's responsible for cleaning it up when it's done. However, if you interrupt Claude with **Ctrl-C**, it gets killed before it can run `git worktree remove`, leaving the worktree behind on disk. Warden correctly shows these because they still exist in git — it's not a Warden bug.

**To clean up stale worktrees:**

- **From the UI:** Click the gear icon next to any worktree in the sidebar and select "Remove".
- **From inside the container:** Run `git worktree remove <path>` and `git branch -D <branch>` manually.

**Why doesn't Warden clean them up automatically?** Warden delegates worktree lifecycle to Claude Code because it can't know whether you have uncommitted work in a worktree. Automatically deleting a worktree with unsaved changes would be destructive. If you want a clean slate, use the gear icon to remove worktrees you no longer need.

## Why does a worktree still show "connected" after I closed the tab?

Closing your browser tab disconnects the WebSocket viewer but does **not** stop the agent. The abduco process manager keeps Claude running in the background — that's by design so agents can work autonomously. The worktree will show "active in background". To fully stop a worktree's process, use the right-click context menu to Disconnect and Remove the worktree.
