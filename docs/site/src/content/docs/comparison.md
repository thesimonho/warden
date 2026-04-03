---
title: Comparison
description: How Warden compares to other tools for running AI coding agents.
---

There are several tools for running AI coding agents. This page compares Warden with other popular options to help you choose the right one.

## Feature comparison

| Feature                    |         Warden          |      OpenShell      | Claude Squad  |       VS Code        |
| -------------------------- | :---------------------: | :-----------------: | :-----------: | :------------------: |
| Container isolation        |           Yes           |         Yes         |      No       |         Yes          |
| Safe autonomous mode       |           Yes           |         Yes         |      No       |         Yes          |
| Network access controls    |           Yes           |   Yes (OPA/Rego)    |      No       |          No          |
| Agent status detection     |           Yes           |         No          |      Yes      |          No          |
| Attention notifications    |           Yes           |         No          |      No       |          No          |
| Cost tracking + budgets    |       Per-project       |         No          |      No       |          No          |
| Background agents          |           Yes           |         Yes         |  tmux detach  |          No          |
| Embeddable engine / API    |           Yes           |         No          |      No       |          No          |
| Web UI                     |           Yes           |         No          |      No       |      Yes (IDE)       |
| Multi-agent support        |   Claude Code + Codex   |   Multiple agents   |  Claude Code  |          No          |
| Git worktree orchestration |           Yes           |         No          |      No       |          No          |
| GPU passthrough            |           No            | Yes (experimental)  |      No       |          No          |
| Custom environment         | Dockerfile/devcontainer | Dockerfile/catalog  |      No       |     Devcontainer     |
| Setup                      |      Single binary      |  CLI + k3s cluster  | Single binary | Full IDE + extension |
| Infrastructure required    |         Docker          | Docker + Kubernetes |     None      |     Docker + IDE     |

## When to choose what

**Choose Warden** if you want:

- **A security boundary for autonomous agents** — container isolation, network controls, and cost budgets let you run agents unsupervised with confidence.
- **An embeddable engine** — Warden ships as an importable Go library and a REST API, so you can integrate agent orchestration into your own tools. See the [Integration Paths](/warden/integration/paths/) for details.
- **Visibility into what agents are doing** — real-time status detection, attention notifications, and an [audit log](/warden/integration/http-api/#audit-api) give you a full picture of agent activity.
- **Git worktree orchestration** — run multiple agents on the same repo in parallel, each in its own isolated worktree.

**Choose something else** if you live in VS Code, want GPU passthrough, or don't need isolation/security.
