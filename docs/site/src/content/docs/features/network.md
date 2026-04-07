---
title: Network Isolation
description: Control outbound network access from containers.
---

Warden enforces network policies inside containers using iptables, giving you control over what Claude Code can reach on the internet. Choose between unrestricted access, a domain allowlist, or full air-gapping.

Network isolation is configured per-project when creating or updating a container.

## Network Modes

### Full (Default)

Unrestricted outbound access. The container can reach any host on the internet. No iptables rules are applied.

Use this when Claude needs general internet access — installing packages, cloning repos, calling APIs.

### Restricted

Outbound traffic is limited to a configurable list of allowed domains. All other traffic is dropped.

Use this when you want Claude to have internet access but only to specific services — your GitHub org, npm registry, internal APIs, etc.

**Configuring allowed domains:**

When selecting Restricted mode, you specify a list of domains. Both exact domains and wildcards are supported:

| Entry             | What it matches                           |
| ----------------- | ----------------------------------------- |
| `github.com`      | `github.com` and `*.github.com`           |
| `npmjs.org`       | `npmjs.org` and `*.npmjs.org`             |
| `api.example.com` | `api.example.com` and `*.api.example.com` |

Each domain entry automatically includes all subdomains.

Warden pre-populates the domain list based on the selected agent type: Claude Code gets `*.anthropic.com`, Codex gets `*.openai.com`, and both include shared infrastructure domains (GitHub, Ubuntu apt repos). Runtime-specific domains (npm, PyPI, Go modules, etc.) are added automatically based on the runtimes enabled for the project. You can customize this list at creation time or edit it later.

**Live domain updates:**

Allowed domains can be changed on a running container without restarting it. When you update domains in the edit dialog, Warden hot-reloads the network policy: the dnsmasq config and ipset are updated and dnsmasq is signaled to reload. Active connections to previously-allowed domains remain alive until they close naturally, while new connections to removed domains are blocked immediately.

### None

All outbound traffic is blocked. Only loopback (localhost) and established connections (responses to already-open connections) are allowed.

Use this for air-gapped operation — when Claude should work entirely with local files and tools, with no internet access.

## Accessing Host Services

Containers can reach services running on the host machine (e.g. a local dev server, database, or API) using the special hostname `host.docker.internal`.

For example, if you're running a dev server on port 3000 on the host:

```bash
# Inside the container
curl http://host.docker.internal:3000
```

This works in all network modes — `host.docker.internal` resolves to the host's IP via Docker's `host-gateway` mapping. It does not count as outbound internet traffic, so it is not affected by domain allowlists in Restricted mode or blocked in None mode.

:::tip
If you're building a project that has both a backend and a frontend (e.g. a Vite + Express app), you can run the backend on the host and have the agent inside the container test against it using `host.docker.internal`.
:::

## Port Forwarding

When an agent starts a web server inside the container (e.g. Vite on port 5173), you can access it from the host via Warden's built-in reverse proxy.

**Declaring ports:**

Add the ports you want to forward in your project's container settings. Ports can also be declared in `.warden.json`:

```json
{
  "forwardedPorts": [5173, 3000]
}
```

Each declared port is accessible at:

```
http://localhost:8090/api/v1/projects/{projectId}/{agentType}/proxy/{port}/
```

For example, if your project ID is `a1b2c3d4e5f6` and you're running Claude Code with Vite on port 5173:

```
http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/proxy/5173/
```

**How it works:**

Warden's Go backend reverse-proxies HTTP and WebSocket traffic to the container's internal IP. This means:

- HMR (hot module replacement) works — WebSocket upgrade is supported
- Multiple containers can each use the same port internally without conflicts
- No Docker port bindings are needed, so no container recreation is required

**Live updates:**

Forwarded ports can be added or removed on a running container without restarting it. The proxy validates each request against the current declared port list — undeclared ports return a 404.

:::note
Port forwarding only handles HTTP and WebSocket traffic. For non-HTTP protocols (gRPC, raw TCP), the container's services are not accessible from the host.
:::

## Limitations

- **Domain IPs are resolved dynamically**, but if a domain's IP changes and DNS caching hasn't refreshed, there may be a brief interruption. Editing the allowed domains list triggers a full re-resolution; otherwise restart the container.
- **Network mode changes** (e.g. `full` → `restricted`) still require container recreation since they involve different iptables rule sets and capabilities.
