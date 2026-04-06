# Network

Network mode is part of the container configuration:

## HTTP API

Set `networkMode` and `allowedDomains` in the container create/update request:

```json
{
  "networkMode": "restricted",
  "allowedDomains": ["github.com", "npmjs.org", "registry.npmjs.org"]
}
```

Valid values for `networkMode`: `"full"`, `"restricted"`, `"none"`.

## Go Client

```go
c := client.New("http://localhost:8090")

result, _ := c.CreateContainer(ctx, projectID, "claude-code", api.CreateContainerRequest{
    NetworkMode:    "restricted",
    AllowedDomains: []string{"github.com", "npmjs.org"},
})
```

## Go Library

```go
w, _ := warden.New(warden.Options{})

result, _ := w.Service.CreateContainer(ctx, api.CreateContainerRequest{
    NetworkMode:    "restricted",
    AllowedDomains: []string{"github.com", "npmjs.org"},
})
```

See the [Go Packages](https://thesimonho.github.io/warden/reference/go/) reference for full API documentation.
