# Access Items

Access Items are Warden's credential passthrough system. They let you share host credentials with containers without storing or copying secrets. Each Access Item is a named group of credentials with recipes for how to obtain values from the host and inject them into the container at creation time.

Warden never stores credential values -- only the configuration that describes where to find them (sources) and where to deliver them (injections). Values are resolved at container start, injected into the container's runtime environment, and gone when the container stops.

## Key concepts

### Items, credentials, sources, and injections

The hierarchy is:

```
Access Item (e.g. "AWS CLI")
  Credential (e.g. "AWS Access Key ID")
    Sources: [env var AWS_ACCESS_KEY_ID]    -- where to get it on the host
    Injections: [env var AWS_ACCESS_KEY_ID] -- where to put it in the container
  Credential (e.g. "AWS Config")
    Sources: [file ~/.aws/config]
    Injections: [mount /home/warden/.aws/config, readOnly]
```

Each credential has one or more **sources** (tried in order; first detected wins) and one or more **injections** (all applied when the source resolves).

### Source types

| Type      | Value field contains      | Use case                            |
| --------- | ------------------------- | ----------------------------------- |
| `env`     | Environment variable name | Tokens and API keys in your shell   |
| `file`    | Absolute file path        | Config files, certificates          |
| `socket`  | Socket path (or env var)  | SSH agent, Docker socket            |
| `command` | Shell command string      | Tokens in keychains, dynamic values |

Sources are tried in declaration order. The first one where the value is detected on the host is used. Remaining sources are skipped.

### Injection types

| Type           | `key` field contains      | Use case                            |
| -------------- | ------------------------- | ----------------------------------- |
| `env`          | Environment variable name | Tools that read env vars            |
| `mount_file`   | Container file path       | Config files mounted read-only      |
| `mount_socket` | Container socket path     | SSH agent forwarding, Docker socket |

The `readOnly` field controls whether file mounts are read-only (recommended for sensitive files). The `value` field on an injection can provide a static override -- useful when the injection needs a fixed container-side path regardless of the resolved source value (e.g. `SSH_AUTH_SOCK` env var pointing to a well-known socket path).

### Detection vs resolution

**Detection** checks whether a credential's source exists on the host without reading its value. This is fast and non-destructive -- used to show availability status in the UI and API responses. The `GET /api/v1/access` response includes per-credential detection results.

**Resolution** reads actual values from sources and prepares injections. This happens at container creation time, immediately before the container starts. The `POST /api/v1/access/resolve` endpoint lets you test resolution without creating a container.

**Partial resolution** is normal. If an Access Item has three credentials and only two are detected, the two that resolve are injected and the third is silently skipped. The container still starts.

## Built-in items

Warden ships with three pre-configured Access Items that cover the most common needs. You can edit them to customize behavior and reset them to defaults if needed.

### Git (`id: "git"`)

Mounts your host `.gitconfig` (read-only) so git commands inside the container use your identity and settings.

- Looks for `~/.gitconfig` or `~/.config/git/config` (first found wins)
- Mounts read-only at `/home/warden/.gitconfig.host`
- The container entrypoint includes it via `git config --global include.path`

The Git item only handles git configuration (identity, aliases, settings). For SSH-based git authentication, use the SSH item.

### SSH (`id: "ssh"`)

Forwards SSH config, known_hosts, and the SSH agent socket so git-over-SSH and SSH commands work without copying private keys.

- Mounts `~/.ssh/config` read-only (filtered to remove `IdentitiesOnly` directives that would block the forwarded agent)
- Mounts `~/.ssh/known_hosts` read-write (so new hosts can be added)
- Forwards `$SSH_AUTH_SOCK` as a socket mount

Private keys never enter the container. Signing requests are forwarded to the host's SSH agent via the socket.

### GPG (`id: "gpg"`)

Forwards the host's gpg-agent socket so git commit signing (`-S`) and GPG operations work inside the container without copying private keys.

- Finds the gpg-agent socket via platform-specific discovery (Linux: `gpgconf --list-dir agent-socket`, macOS: `$GNUPGHOME` or `~/.gnupg/S.gpg-agent`)
- Mounts the socket at `/home/warden/.gnupg/S.gpg-agent` (the default gpg socket location, so gpg finds it automatically)

Private keys never enter the container. Signing requests are forwarded to the host's gpg-agent via the socket.

## Custom items

Create custom Access Items for any credential that needs to reach the container. Common examples: GitHub CLI tokens, AWS credentials, Docker registry auth.

Each custom item gets a generated UUID as its `id`. Custom items can be freely created, updated, and deleted via the API. Built-in items cannot be deleted, only edited or reset.

## The `access` package (Go developers)

The `access/` package is a standalone Go library with zero Warden dependencies. It provides the types, resolution logic, and built-in item definitions. You can import it directly to resolve credentials in your own Go programs without running the Warden server:

```go
import "github.com/thesimonho/warden/access"
```

## API operations

### List access items (with detection status)

Returns all items (built-in + user-created) with per-credential host detection status.

```bash
curl http://localhost:8090/api/v1/access
```

Response includes an `items` array. Each item has a `detection` object showing which credentials are available on the current host:

```json
{
  "items": [
    {
      "id": "git",
      "label": "Git Config",
      "builtIn": true,
      "description": "Mounts host .gitconfig for git identity",
      "method": "transport",
      "credentials": [...],
      "detection": {
        "id": "git",
        "label": "Git Config",
        "available": true,
        "credentials": [
          {
            "label": "Git Config File",
            "available": true,
            "sourceMatched": "file: /home/user/.gitconfig"
          }
        ]
      }
    }
  ]
}
```

### Get a single access item

```bash
curl http://localhost:8090/api/v1/access/{id}
```

Returns the item with detection status. For built-in items, returns the DB override if one exists, otherwise the default configuration.

### Create a custom item

```bash
curl -X POST http://localhost:8090/api/v1/access \
  -H "Content-Type: application/json" \
  -d '{
    "label": "GitHub CLI",
    "description": "Injects GitHub OAuth token for gh commands",
    "credentials": [
      {
        "label": "GitHub Token",
        "sources": [
          {"type": "command", "value": "gh auth token"},
          {"type": "env", "value": "GH_TOKEN"}
        ],
        "injections": [
          {"type": "env", "key": "GH_TOKEN"}
        ]
      }
    ]
  }'
```

This item tries two sources in order: first runs `gh auth token` on the host, falling back to the `GH_TOKEN` environment variable. The resolved value is injected as the `GH_TOKEN` env var inside the container.

Returns `201 Created` with the full item including its generated `id`.

### Update an access item

```bash
curl -X PUT http://localhost:8090/api/v1/access/{id} \
  -H "Content-Type: application/json" \
  -d '{
    "label": "GitHub CLI",
    "description": "Updated description",
    "credentials": [...]
  }'
```

For built-in items, this saves a customized copy to the database that overrides the default. For user items, it updates the existing record.

### Delete a custom item

```bash
curl -X DELETE http://localhost:8090/api/v1/access/{id}
```

Returns `204 No Content`. Built-in items cannot be deleted -- returns `400`.

### Reset a built-in item to defaults

```bash
curl -X POST http://localhost:8090/api/v1/access/{id}/reset
```

Removes any DB override and restores the built-in item's default configuration. Only works for built-in items (`git`, `ssh`, `gpg`).

### Test resolution (preview)

Resolve items without creating a container. Useful for verifying that sources are detected and injections are correct before attaching items to a project.

```bash
curl -X POST http://localhost:8090/api/v1/access/resolve \
  -H "Content-Type: application/json" \
  -d '{
    "items": [
      {
        "id": "ssh",
        "label": "SSH",
        "builtIn": true,
        "method": "transport",
        "credentials": [
          {
            "label": "SSH Config",
            "sources": [{"type": "file", "value": "~/.ssh/config"}],
            "injections": [{"type": "mount_file", "key": "/home/warden/.ssh/config", "readOnly": true}]
          },
          {
            "label": "SSH Agent Socket",
            "sources": [{"type": "socket", "value": "$SSH_AUTH_SOCK"}],
            "injections": [
              {"type": "mount_socket", "key": "/home/warden/.ssh/agent.sock"},
              {"type": "env", "key": "SSH_AUTH_SOCK", "value": "/home/warden/.ssh/agent.sock"}
            ]
          }
        ]
      }
    ]
  }'
```

Response shows per-credential resolution results:

```json
{
  "items": [
    {
      "id": "ssh",
      "label": "SSH",
      "credentials": [
        {
          "label": "SSH Config",
          "resolved": true,
          "sourceMatched": "file: /home/user/.ssh/config",
          "injections": [
            {
              "type": "mount_file",
              "key": "/home/warden/.ssh/config",
              "value": "/home/user/.ssh/config",
              "readOnly": true
            }
          ]
        },
        {
          "label": "SSH Agent Socket",
          "resolved": true,
          "sourceMatched": "socket: /tmp/ssh-agent.sock",
          "injections": [
            {
              "type": "mount_socket",
              "key": "/home/warden/.ssh/agent.sock",
              "value": "/tmp/ssh-agent.sock"
            },
            {
              "type": "env",
              "key": "SSH_AUTH_SOCK",
              "value": "/home/warden/.ssh/agent.sock"
            }
          ]
        }
      ]
    }
  ]
}
```

A credential with `"resolved": false` and an empty `sourceMatched` means no source was detected. A credential with an `"error"` field means resolution was attempted but failed.

## Enabling access items for a project

Access Items are selected per-project via the `enabledAccessItems` field in the container configuration. Pass an array of item IDs when creating or updating a container:

```bash
curl -X POST http://localhost:8090/api/v1/projects/{projectId}/{agentType}/container \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-project",
    "projectPath": "/home/user/my-project",
    "enabledAccessItems": ["git", "ssh", "gpg", "a1b2c3d4-..."]
  }'
```

At container start, Warden resolves all enabled items and injects the results (env vars, file mounts, socket mounts) into the container. Undetected credentials within enabled items are silently skipped.
