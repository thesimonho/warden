# Access

The Access system is available through all three integration paths:

## HTTP API

All access operations are available via REST endpoints:

| Method   | Endpoint                    | Description                          |
| -------- | --------------------------- | ------------------------------------ |
| `GET`    | `/api/v1/access`            | List all items with detection status |
| `POST`   | `/api/v1/access`            | Create a custom item                 |
| `GET`    | `/api/v1/access/{id}`       | Get a single item                    |
| `PUT`    | `/api/v1/access/{id}`       | Update an item                       |
| `DELETE` | `/api/v1/access/{id}`       | Delete a custom item                 |
| `POST`   | `/api/v1/access/{id}/reset` | Reset a built-in to default          |
| `POST`   | `/api/v1/access/resolve`    | Test resolution (preview injections) |

## Go Client

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

## Go Library

The `access` package (`github.com/thesimonho/warden/access`) is public and importable with no dependencies on other Warden packages:

- `access.Resolve(item, env)` — resolve an item's credentials and return injections
- `access.Detect(item, env)` — check credential availability without reading values
- `access.NewShellEnvResolver()` — create a resolver that captures the user's shell environment
- `access.BuiltInItems()` — get the default Git and SSH items

The `env` parameter accepts an `access.EnvResolver` interface. Pass `nil` to use the default process environment, or use `access.NewShellEnvResolver()` to also capture env vars from the user's shell config (`.bashrc`, `.zshrc`, `.profile`). This is important when Warden is launched from a desktop entry rather than a terminal.

See the [Go Packages](https://thesimonho.github.io/warden/reference/go/) reference for full API documentation.
