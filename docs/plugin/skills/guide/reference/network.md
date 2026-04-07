# Network Isolation

Warden enforces outbound network policy inside containers using iptables, dnsmasq, and ipset. Every container runs in one of three network modes, configured at creation time and enforced transparently to the agent.

## Network modes

### `full` (default)

Unrestricted outbound access. No iptables rules are applied. The container can reach any host on the internet.

Use when the agent needs general internet access -- installing packages, cloning repos, calling APIs.

### `restricted`

Outbound traffic is limited to a configurable domain allowlist. All other traffic is dropped by iptables. DNS resolution is handled by a local dnsmasq instance that only resolves allowed domains.

Use when you want the agent to reach specific services (your GitHub org, npm registry, internal APIs) but nothing else.

### `none`

All outbound traffic is blocked. Only loopback (localhost) and responses to already-established connections are allowed.

Use for air-gapped operation where the agent works entirely with local files.

## Domain allowlist (restricted mode)

When using `restricted` mode, you provide a list of allowed domains. Each entry automatically includes all subdomains:

| Entry             | Matches                                   |
| ----------------- | ----------------------------------------- |
| `github.com`      | `github.com` and `*.github.com`           |
| `npmjs.org`       | `npmjs.org` and `*.npmjs.org`             |
| `api.example.com` | `api.example.com` and `*.api.example.com` |

Warden pre-populates the domain list based on the agent type and enabled runtimes:

- **Claude Code** containers get `*.anthropic.com` plus shared infrastructure domains (GitHub, Ubuntu apt repos).
- **Codex** containers get `*.openai.com` plus the same shared infrastructure.
- **Runtime-specific domains** are added automatically based on enabled runtimes. For example, enabling Python adds `pypi.org` and `files.pythonhosted.org`; enabling Go adds `proxy.golang.org` and `sum.golang.org`.

You can customize this list at creation time or update it later on a running container.

## Hot-reload (live domain updates)

Allowed domains can be updated on a running container without restarting it. When you update domains via the API, Warden hot-reloads the network policy inside the container:

1. The dnsmasq configuration is regenerated with the new domain list.
2. The ipset rules are updated.
3. dnsmasq is signaled to reload its configuration.

**Behavior during hot-reload:**
- Active connections to previously-allowed domains remain alive until they close naturally.
- New connections to removed domains are blocked immediately.
- New connections to added domains work immediately after the reload completes.

## Mode changes require recreation

Changing the network mode itself (e.g. `full` to `restricted`, or `restricted` to `none`) requires recreating the container. This is because different modes use fundamentally different iptables rule sets and container capabilities that are set at container start time.

Updating the domain allowlist within `restricted` mode does **not** require recreation -- that uses the hot-reload path described above.

## How it works internally

The container entrypoint starts as root to perform privileged network setup:

1. iptables rules are installed based on the network mode.
2. For `restricted` mode: dnsmasq is configured as a local DNS resolver, ipset is populated with resolved IPs for allowed domains, and iptables rules restrict outbound traffic to those IPs plus DNS.
3. The entrypoint permanently drops to the `warden` user via `exec gosu`. No root process remains after startup.

## Runtime domains

Enabling a language runtime in a project's configuration automatically adds the minimum network surface for that runtime's package registry to the domain allowlist. This only matters in `restricted` mode -- `full` mode already allows everything, and `none` mode blocks everything.

Node.js is always enabled (required for MCP servers), so npm registry domains are always included in `restricted` mode. Other runtimes are auto-detected from project marker files (e.g. `go.mod` for Go, `pyproject.toml` for Python) and can also be explicitly set.

## API examples

### Create a container with restricted networking

```bash
curl -X POST http://localhost:8090/api/v1/projects/{projectId}/{agentType}/container \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-project",
    "projectPath": "/home/user/my-project",
    "networkMode": "restricted",
    "allowedDomains": [
      "github.com",
      "npmjs.org",
      "api.anthropic.com"
    ]
  }'
```

### Create an air-gapped container

```bash
curl -X POST http://localhost:8090/api/v1/projects/{projectId}/{agentType}/container \
  -H "Content-Type: application/json" \
  -d '{
    "name": "secure-project",
    "projectPath": "/home/user/secure-project",
    "networkMode": "none"
  }'
```

### Update allowed domains on a running container

This hot-reloads the domain list without restarting the container. Only applicable when the container is already in `restricted` mode.

```bash
curl -X PUT http://localhost:8090/api/v1/projects/{projectId}/{agentType}/container \
  -H "Content-Type: application/json" \
  -d '{
    "allowedDomains": [
      "github.com",
      "npmjs.org",
      "api.anthropic.com",
      "registry.terraform.io"
    ]
  }'
```

### Inspect current network configuration

The container config endpoint returns the current network mode and allowed domains:

```bash
curl http://localhost:8090/api/v1/projects/{projectId}/{agentType}/container/config
```

The response includes `networkMode` and `allowedDomains` fields in the container configuration.

## Port forwarding

Warden includes a reverse proxy that lets you access HTTP and WebSocket services running inside a container from the host. Declare which ports to forward in the container configuration, then access them via the proxy URL.

### Declaring forwarded ports

Include `forwardedPorts` in the create or update container request:

```bash
curl -X POST http://localhost:8090/api/v1/projects/{projectId}/{agentType}/container \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "projectPath": "/home/user/my-app",
    "forwardedPorts": [5173, 3000]
  }'
```

Forwarded ports can also be set in `.warden.json`:

```json
{
  "forwardedPorts": [5173, 3000]
}
```

### Accessing forwarded ports

Each declared port is accessible via the proxy endpoint:

```
GET /api/v1/projects/{projectId}/{agentType}/proxy/{port}/{path...}
```

The proxy handles all HTTP methods and WebSocket upgrade (needed for Vite HMR). Undeclared ports return 404. If the container is not running, the proxy returns 502.

### Hot-reload

Forwarded ports can be added or removed on a running container without recreation. Update the container configuration with the new port list -- the proxy validates against the current list on each request.

### WebSocket support

The proxy supports WebSocket upgrade transparently. When the container service responds with `101 Switching Protocols`, the proxy establishes a bidirectional tunnel. This is required for dev server features like hot module replacement (HMR).

## Edge cases

- **DNS caching:** Domain IPs are resolved dynamically, but if a domain's IP changes and DNS caching has not refreshed, there may be a brief interruption. Updating the allowed domains list triggers a full re-resolution. Otherwise, restart the container.
- **Wildcard scope:** There is no way to allow a subdomain without also allowing its parent. Each entry in the allowlist grants access to the exact domain plus all subdomains.
- **Partial updates:** The `PUT` endpoint for updating a container accepts a full configuration. To update only domains, include the existing `networkMode` value and the new `allowedDomains` list. Changing `networkMode` in the same request triggers a container recreation.
