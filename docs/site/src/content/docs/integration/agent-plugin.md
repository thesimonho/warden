---
title: Agent Plugin & Skills
description: Install Warden integration skills for Claude Code and other AI coding agents.
---

Warden provides an AI agent plugin with integration skills and reference material. When installed, your coding agent gets access to Warden's architecture docs, API reference, and feature guides — so it can help you integrate Warden into your project without needing to dig through documentation.

## What's included

The plugin includes:

- **Warden Surveyor agent** — a read-only agent that explores your codebase and identifies where Warden can integrate with your existing features
- **Warden skill** — a routing skill that directs any agent to the correct reference files based on what you're integrating (projects, containers, worktrees, events, etc.)
- **Feature references** — integration guides for each Warden feature covering HTTP API, Go client, and Go library paths
- **Full API reference** — per-resource endpoint docs generated from the OpenAPI spec, optimized for agent consumption

## Installation

If you use Claude Code, the marketplace is the easiest way to install the plugin. Otherwise, using `npx skills` is recommended.

### Claude Code Marketplace

Add the marketplace in Claude Code and select the plugin in the UI:

```
/plugin marketplace add thesimonho/artificial-jellybeans
```

Or install the plugin directly:

```
/plugin install warden@artificial-jellybeans
```

This installs the skill and agent definitions into your Claude Code environment. Use it in any project where you're integrating with Warden.

### npx skills

If you use [vercel-labs/skills](https://github.com/vercel-labs/skills):

```bash
npx skills add thesimonho/warden
```

### Manual Install

If you use an agent that doesn't support plugin systems, you can download the skills directly:

1. Clone or download the `docs/plugin/` directory from the [Warden repo](https://github.com/thesimonho/warden/tree/main/docs/plugin)
2. Copy the plugin files into your agent's skills directory

## Using the Surveyor agent

The Surveyor is a read-only agent that scans your codebase and produces an integration report:

```
Run the warden-surveyor agent
```

Or target specific features:

```
I want to integrate Warden's audit system.
Call the warden-surveyor agent to explore my codebase.
```

It identifies:

- Existing container management, process orchestration, or agent lifecycle code that Warden could replace
- Where your project already has integration points (API clients, WebSocket handling, event systems)
- A recommended integration order and which integration path (HTTP API, Go client, or Go library) fits best

The Surveyor doesn't modify code — it only explores and reports.

## Using the skill

You can use the Warden skill directly to help integrate features into your application:

- _"Set up Warden project management using the HTTP API"_
- _"Add worktree support using the Go client"_
- _"How does Warden's cost tracking work?"_

The skill routes your agent to the relevant reference file so it has the full API surface, types, and examples.
