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

| Entry | What it matches |
|-------|----------------|
| `github.com` | `github.com` and `*.github.com` |
| `npmjs.org` | `npmjs.org` and `*.npmjs.org` |
| `api.example.com` | `api.example.com` and `*.api.example.com` |

Each domain entry automatically includes all subdomains.

### None

All outbound traffic is blocked. Only loopback (localhost) and established connections (responses to already-open connections) are allowed.

Use this for air-gapped operation — when Claude should work entirely with local files and tools, with no internet access.

## Limitations

- **Restricted/None modes may not work with rootless Podman** depending on your configuration.
- **Domain IPs are resolved dynamically**, but if a domain's IP changes and DNS caching hasn't refreshed, there may be a brief interruption. Restart the container to force re-resolution.

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

result, _ := c.CreateContainer(ctx, projectID, engine.CreateContainerRequest{
    NetworkMode:    "restricted",
    AllowedDomains: []string{"github.com", "npmjs.org"},
})
```

### Go Library

```go
w, _ := warden.New(warden.Options{})

result, _ := w.Service.CreateContainer(ctx, engine.CreateContainerRequest{
    NetworkMode:    "restricted",
    AllowedDomains: []string{"github.com", "npmjs.org"},
})
```

See the [Go Packages](/warden/reference/go/) reference for full API documentation.
