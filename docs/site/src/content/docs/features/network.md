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

Warden pre-populates the domain list based on the selected agent type: Claude Code gets `*.anthropic.com`, Codex gets `*.openai.com`, and both include shared infrastructure domains (GitHub, npm, PyPI, Go modules). You can customize this list at creation time or edit it later.

**Live domain updates:**

Allowed domains can be changed on a running container without restarting it. When you update domains in the edit dialog, Warden hot-reloads the network policy: the dnsmasq config and ipset are updated and dnsmasq is signaled to reload. Active connections to previously-allowed domains remain alive until they close naturally, while new connections to removed domains are blocked immediately.

### None

All outbound traffic is blocked. Only loopback (localhost) and established connections (responses to already-open connections) are allowed.

Use this for air-gapped operation — when Claude should work entirely with local files and tools, with no internet access.

## How It Works

Restricted mode uses [dnsmasq](https://dnsmasq.org/) as a local DNS forwarder combined with an ipset-based iptables firewall:

1. **DNS interception** — `resolv.conf` is rewritten to point to a local dnsmasq instance (`127.0.0.53`). All DNS queries from the container go through dnsmasq.
2. **Dynamic IP tracking** — When a DNS query matches an allowed domain, dnsmasq adds the resolved IPs to a kernel ipset (`warden_allowed`) with a 300-second TTL. This handles wildcard domains correctly — `*.github.com` covers `ssh.github.com` even when it resolves to a different IP.
3. **Firewall** — iptables OUTPUT rules allow traffic to any IP in the ipset and reject everything else. The `ESTABLISHED,RELATED` rule keeps existing connections alive.
4. **Hot-reload** — When domains change at runtime, the script detects the running dnsmasq and takes a fast path: regenerate config, flush and re-seed the ipset, then signal dnsmasq with SIGHUP. No iptables rules are modified, so active connections are unaffected.

## Limitations

- **Domain IPs are resolved dynamically**, but if a domain's IP changes and DNS caching hasn't refreshed, there may be a brief interruption. Editing the allowed domains list triggers a full re-resolution; otherwise restart the container.
- **Network mode changes** (e.g. `full` → `restricted`) still require container recreation since they involve different iptables rule sets and capabilities.

## For Developers

Network mode is part of the container configuration:

### HTTP API

Set `networkMode` and `allowedDomains` in the container create/update request:

```json
{
  "networkMode": "restricted",
  "allowedDomains": ["github.com", "npmjs.org", "registry.npmjs.org"]
}
```

Valid values for `networkMode`: `"full"`, `"restricted"`, `"none"`.

### Go Client

```go
c := client.New("http://localhost:8090")

result, _ := c.CreateContainer(ctx, projectID, api.CreateContainerRequest{
    NetworkMode:    "restricted",
    AllowedDomains: []string{"github.com", "npmjs.org"},
})
```

### Go Library

```go
w, _ := warden.New(warden.Options{})

result, _ := w.Service.CreateContainer(ctx, api.CreateContainerRequest{
    NetworkMode:    "restricted",
    AllowedDomains: []string{"github.com", "npmjs.org"},
})
```

See the [Go Packages](/warden/reference/go/) reference for full API documentation.
