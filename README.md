<div align="center">
  <table border="0" cellspacing="0" cellpadding="0">
    <tr>
      <td>
        <img src="logo.svg" width="465" alt="Warden">
        <p align="right"><em>secure autonomous agents by default.</em></p>
      </td>
    </tr>
  </table>
</div>

<p align="center">
<img alt="GitHub Repo stars" src="https://img.shields.io/github/stars/thesimonho/warden?style=for-the-badge&labelColor=1F1F28&color=c4b28a">

<img alt="GitHub Release" src="https://img.shields.io/github/v/release/thesimonho/warden?style=for-the-badge&labelColor=1F1F28&color=887389">

<img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/thesimonho/warden?style=for-the-badge&labelColor=1F1F28&color=5d7a88">

<img alt="GitHub branch check runs" src="https://img.shields.io/github/check-runs/thesimonho/warden/main?style=for-the-badge&label=build&labelColor=1F1F28&color=699469">
</p>

A modular security boundary for AI coding agents. Bring your own orchestrator, or use the included web dashboard and TUI to run agents directly.

Every project gets its own container — isolated filesystem, credentials, and network. A rogue agent can trash its container but can never escape to the host, other containers, or your production systems.

## 💡 Motivation

You want to let your agents run wild without needing to approve permissions constantly, but you're also scared of it breaking things on your system. What do?

Here are the steps:

- Turn on sandboxing and configure the permissions for which commands are allowed.
- But sandboxes only prevent unauthorized access. They don't prevent authorized stupidity. So you have to isolate them in containers to avoid dependency conflicts and other system-wide issues.
- But now you need to lock down the container. So you need to setup network policies, filesystem permissions, iptables, firewalls, etc.
- But now you need interact with those agents, so you need a way to connect to them and keep their sessions alive while they work. So, SSH? multiplexing?
- But you're running so many agents, you need a way to know when they need your attention. So you have to figure out some method for them to forward events to you.
- But agents do stupid things, so you'll want to audit their activity, how much money they're spending. OK, so let's plan some logging, metrics, and a database for storage.
- ...

Or you could just use **Warden**.

Warden is a modular, self-contained infrastructure layer that makes autonomous agents safe by default. It handles all of the above for you, while remaining configurable for your specific project needs.

You can easily use it from Day 1 as its own agent orchestrator, running as a webapp or terminal UI. But it's real power comes from being a self-contained security boundary, that developers can integrate into their existing applications, gaining containerized agent infrastructure for free.

## ✅ What you get

<div align="center">
  <img width="400" alt="light" src="docs_site/public/hero-light.webp" />
  <img width="400" alt="dark" src="docs_site/public/hero-dark.webp" />
</div>

### Security model

- **Full container isolation** — each project gets its own filesystem, env vars, and credentials. No credential bleed, no cross-project file access.
- **Process hardening** — containers run with dropped capabilities, a custom seccomp profile blocking dangerous syscalls, and `no-new-privileges` to prevent escalation. Applied automatically to every container.
- **Safe autonomous mode** — run `--dangerously-skip-permissions` without risking your host. The blast radius is one disposable container.
- **Network access controls** — per-container policy: full access, restricted (domain allowlist), or air-gapped.
- **Credential passthrough** — share Git, SSH, and custom credentials with containers automatically without storing them.

### Agent operations

- **Real-time agent status** — idle, working, needs permission, needs input, needs answer — across every agent at a glance.
- **Worktree orchestration** — isolated git worktrees allows for parallel development.
- **Session persistence** — terminals survive disconnects via abduco. Close the tab, agent keeps working. Reconnect later.
- **Attention notifications** — know exactly which agent needs you without checking each terminal.

### Developer experience

- **Go library** — embed the engine directly with `warden.New()`. No HTTP overhead, no server process.
- **HTTP API** — REST + SSE + WebSocket. Works from any language.
- **Go HTTP client** — typed client for Go apps talking to a running Warden server.
- **Reference implementations** — the web dashboard and TUI use the same public interfaces you would. Read their source as integration examples.
- **Single binary** — Go backend with embedded frontend. No runtime dependencies beyond a container engine.

### User experience

- **Full terminal scrollback** — be able to scroll back through session history.
- **Cost tracking and budget enforcement** — per-project cost tracking with configurable budget actions (warn, stop worktrees, stop container, prevent restart).
- **Diff view** — see the changes made by each agent in real time.
- **Audit system** — unified event logging with activity timeline visualization, summary dashboard, category filtering (sessions, tools, prompts, config, system), and compliance-ready export (CSV/JSON). Configurable logging modes (off/standard/detailed) to balance detail with volume.

## 🚀 Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/docs/installation)
- [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview) — currently the only supported agent (more coming soon)

### Download

There are 2 ways to use Warden: as a user or as a developer

Grab the binary for your use case from the [releases page](https://github.com/thesimonho/warden/releases):

| I want to...              | Download         |
| ------------------------- | ---------------- |
| Use the web dashboard     | `warden-desktop` |
| Use the terminal UI       | `warden-tui`     |
| Integrate into my own app | `warden`         |

### As a user — run agents from a web dashboard or terminal

Download the binary and go. No Docker knowledge, no terminal wrangling, no infrastructure setup.

**Web dashboard** (`warden-desktop`): A single binary that opens a browser UI. Create projects, spin up worktrees, monitor every agent's status and cost in one view. Close the tab — agents keep working in the background. Reconnect anytime.

**Terminal UI** (`warden-tui`): Same capabilities, native in the terminal.

```bash
# Web dashboard — opens in your browser at 127.0.0.1:8090
./warden-desktop

# Or TUI — opens in your terminal
./warden-tui
```

You can find more details in the [documentation](https://thesimonho.github.io/warden/guide/getting-started/).

### As a developer — add agent isolation to your app

Warden's engine is a Go library and HTTP API. You get container lifecycle, worktree orchestration, agent status detection, network access controls, and an event bus — all behind clean interfaces. Build your own UI, CLI, or orchestration layer on top.

```go
// Embed the engine directly
app, err := warden.New(warden.Options{})
defer app.Close()
projects, _ := app.Service.ListProjects(ctx)
```

```bash
# Or run as a headless server and hit the REST API
./warden
curl http://localhost:8090/api/v1/projects
```

Both the web dashboard and TUI also act as reference implementations — they use the exact same public interfaces you would. You can reference their source code, or look at the documentation for the [HTTP API](https://thesimonho.github.io/warden/integration/http-api/) and [Go client](https://thesimonho.github.io/warden/integration/go-client/).

See the full [Integration Paths](https://thesimonho.github.io/warden/integration/paths/) page for all options: HTTP API, Go client, Go library.

## 🤝 Contributing to Warden

See the full [Contributing Guide](https://thesimonho.github.io/warden/contributing/) for architecture details, coding guidelines, and PR checklist.
