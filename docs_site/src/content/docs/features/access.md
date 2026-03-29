---
title: Access
description: Credential passthrough for sharing host credentials with containers.
---

The Access system shares host credentials with containers without storing or copying them. Warden detects what's available on your host (Git config, SSH agent, GitHub tokens, AWS credentials, etc.) and injects them into containers at creation time.

Warden never stores credentials — only the recipes for how to obtain and inject them. Credentials are resolved from sources at container start and injected immediately. Nothing is persisted.

## Core Concepts

### Access Items

An **Access Item** is a named group of related credentials. Each item has a label (e.g., "Git", "SSH", "AWS CLI") and contains one or more credentials that work together.

Examples:
- **Git** — mounts `.gitconfig` so git commands use your identity
- **SSH** — forwards config, known_hosts, and agent socket
- **GitHub CLI** — injects OAuth token as `GH_TOKEN`
- **AWS CLI** — injects access keys and config file

### Credentials

A **Credential** is the atomic unit within an Access Item. Each credential has two components:

1. **Source** — where to get the value on the host (env var, file, socket, or command)
2. **Injection** — where to deliver it in the container (env var, file mount, or socket mount)

Sources are tried in order — the first one detected wins. If none are detected, the credential is silently skipped (partial resolution).

### Source Types

| Source | Example | Use case |
|--------|---------|----------|
| **Env var** | `GITHUB_TOKEN` | Tokens, API keys already in your shell |
| **File** | `~/.gitconfig` | Config files, certificates |
| **Socket** | `$SSH_AUTH_SOCK` | SSH agent socket |
| **Command** | `gh auth token` | Tokens in keychains, dynamic values |

### Injection Types

| Injection | Example | Use case |
|-----------|---------|----------|
| **Env var** | `GH_TOKEN=ghp_xxx` | Tools that read env vars |
| **File mount** | Mount `~/.aws/config` → `/home/dev/.aws/config` | Config files |
| **Socket mount** | Mount SSH agent socket | SSH agent forwarding |

### Detection vs Resolution

**Detection** checks if a credential's source exists on the host *without reading its value*. This is fast and safe — used by the UI to show availability status.

**Resolution** actually reads the values and prepares injections. This happens at container creation time, right before the container starts.

## Built-in Items

Warden ships with two pre-configured Access Items. You can edit them to customize their behavior, and reset to defaults if needed.

### Git

Mounts your host `.gitconfig` (read-only) so git commands inside the container use your identity and settings (user.name, user.email, aliases, etc.).

**What it does:**
- Looks for `~/.gitconfig` or `~/.config/git/config` (first found wins)
- Mounts it read-only at `/home/dev/.gitconfig.host`
- The container entrypoint includes it via `git config --global include.path`

**When to enable:** Always, unless you want the container to use a different git identity.

:::note
The Git item only passes through git *configuration* (identity, aliases, settings). It has nothing to do with SSH keys or authentication — that's what the SSH item is for.
:::

### SSH

Forwards SSH config, known_hosts, and the SSH agent socket so git-over-SSH and SSH commands work without copying private keys into the container.

**What it does:**
- Mounts `~/.ssh/config` read-only (filtered to remove `IdentitiesOnly` directives that would block the forwarded agent)
- Mounts `~/.ssh/known_hosts` (read-write, so new hosts can be added)
- Forwards the SSH agent socket from `$SSH_AUTH_SOCK` and sets the env var inside the container

**When to enable:** Whenever you need git-over-SSH (`git clone git@github.com:...`) or direct SSH access to other machines.

:::tip
SSH agent forwarding is the secure way to use SSH keys in containers. The private key never enters the container — signing requests are forwarded to the host agent via the socket.
:::

## Creating Custom Access Items

Navigate to the **Access** page (key icon in the top nav) and click **Create**.

### Example 1: GitHub CLI

The GitHub CLI (`gh`) stores its OAuth token in the OS keychain. On the host, `gh auth token` extracts it. Inside the container, `gh` checks the `GH_TOKEN` env var.

**Setup:**
1. Click **Create** on the Access page
2. **Label:** "GitHub CLI"
3. **Description:** "Injects GitHub OAuth token for gh commands"
4. **Add a credential:**
   - **Label:** "GitHub Token"
   - **Source:** Command — `gh auth token`
   - **Injection:** Env var — `GH_TOKEN`
5. Click **Save**, then **Test** to verify

**What happens at container start:**
1. Warden runs `gh auth token` on your host
2. Captures the token from stdout
3. Injects it as `GH_TOKEN=gho_xxx` into the container
4. `gh` commands inside the container work automatically

### Example 2: AWS CLI (Multiple Credentials)

AWS CLI needs multiple pieces: access key, secret key, and optionally a config file. This shows how one Access Item can group several credentials.

**Setup:**
1. Click **Create** on the Access page
2. **Label:** "AWS CLI"
3. **Add credential 1:**
   - **Label:** "AWS Access Key ID"
   - **Source:** Env var — `AWS_ACCESS_KEY_ID`
   - **Injection:** Env var — `AWS_ACCESS_KEY_ID`
4. **Add credential 2:**
   - **Label:** "AWS Secret Access Key"
   - **Source:** Env var — `AWS_SECRET_ACCESS_KEY`
   - **Injection:** Env var — `AWS_SECRET_ACCESS_KEY`
5. **Add credential 3:**
   - **Label:** "AWS Config"
   - **Source:** File — `~/.aws/config`
   - **Injection:** Mount file — `/home/dev/.aws/config` (read-only)
6. Click **Save**

**Partial resolution:** If you have the env vars set but no `~/.aws/config` file, Warden injects the env vars and silently skips the file mount. Each credential resolves independently.

:::caution
Always mount sensitive files as read-only to prevent the container from accidentally modifying them.
:::

## How Resolution Works

When you create or restart a container with Access Items enabled:

1. **Detection** — Warden checks which credentials have available sources (file exists? env var set? command succeeds?). The UI shows green/gray dots per credential.

2. **Selection** — You choose which Access Items to enable for this container via the project configuration form.

3. **Resolution** — At container start, Warden reads each enabled credential's source and prepares injections (env vars, bind mounts, socket mounts).

4. **Injection** — The container starts with all resolved values in place.

5. **No persistence** — Credentials exist only in the container's runtime environment. When the container stops, they're gone.

## Testing Access Items

Both the create and edit dialogs include a **Test** button that resolves the current form state and shows exactly what would be injected:

- Which credentials were detected and which weren't
- The exact env vars, file mounts, and socket mounts that will be created
- Which source was matched for each credential

Use this to verify items work before saving or attaching them to a project.

## For Developers

The Access system is available through all three integration paths:

### Go Library

The `access` package (`github.com/thesimonho/warden/access`) is public and importable with no dependencies on other Warden packages:

- `access.Resolve(item)` — resolve an item's credentials and return injections
- `access.Detect(item)` — check credential availability without reading values
- `access.BuiltInItems()` — get the default Git and SSH items

### HTTP API

All access operations are available via REST endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/access` | List all items with detection status |
| `POST` | `/api/v1/access` | Create a custom item |
| `GET` | `/api/v1/access/{id}` | Get a single item |
| `PUT` | `/api/v1/access/{id}` | Update an item |
| `DELETE` | `/api/v1/access/{id}` | Delete a custom item |
| `POST` | `/api/v1/access/{id}/reset` | Reset a built-in to default |
| `POST` | `/api/v1/access/resolve` | Test resolution (preview injections) |

### Go Client

The `client` package mirrors the HTTP API with typed methods:

```go
c := client.New("http://localhost:8090")

// List items with detection status
items, _ := c.ListAccessItems(ctx)

// Create a custom item
item, _ := c.CreateAccessItem(ctx, api.CreateAccessItemRequest{
    Label: "GitHub CLI",
    Credentials: []access.Credential{...},
})

// Test resolution (accepts full items — no DB lookup needed)
resolved, _ := c.ResolveAccessItems(ctx, api.ResolveAccessItemsRequest{
    Items: []access.Item{*item},
})
```

See the [Go Packages](/warden/reference/go/) reference for full API documentation.
