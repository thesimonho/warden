# Projects

## For Developers

### HTTP API

| Method   | Endpoint                                                      | Description                            |
| -------- | ------------------------------------------------------------- | -------------------------------------- |
| `GET`    | `/api/v1/projects`                                            | List all projects with status and cost |
| `POST`   | `/api/v1/projects`                                            | Add a project                          |
| `DELETE` | `/api/v1/projects/{projectId}/{agentType}`                    | Remove a project                       |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/stop`               | Stop container                         |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/restart`            | Restart container                      |
| `POST`   | `/api/v1/projects/{projectId}/{agentType}/container`          | Create container with config           |
| `PUT`    | `/api/v1/projects/{projectId}/{agentType}/container`          | Update container config                |
| `DELETE` | `/api/v1/projects/{projectId}/{agentType}/container`          | Delete container                       |
| `GET`    | `/api/v1/projects/{projectId}/{agentType}/container/config`   | Inspect current config                 |
| `GET`    | `/api/v1/projects/{projectId}/{agentType}/container/validate` | Validate infrastructure                |

### Go Client

```go
c := client.New("http://localhost:8090")

// List all projects
projects, _ := c.ListProjects(ctx)

// Add a project
result, _ := c.AddProject(ctx, "my-project", "/home/user/code/my-project", "claude-code")

// Create container with configuration
result, _ := c.CreateContainer(ctx, projectID, "claude-code", api.CreateContainerRequest{
    Image:    "ghcr.io/thesimonho/warden:latest",
    EnvVars:  map[string]string{"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY")},
    Mounts:   []api.Mount{{HostPath: "/home/user/.claude", ContainerPath: "/home/warden/.claude"}},
    NetworkMode: "restricted",
    AllowedDomains: []string{"github.com", "npmjs.org"},
})

// Stop, restart, delete
c.StopProject(ctx, projectID, "claude-code")
c.RestartProject(ctx, projectID, "claude-code")
c.DeleteContainer(ctx, projectID, "claude-code")
```

### Go Library

When using Warden as a Go library, project operations are available on the `service.Service` type:

```go
w, _ := warden.New(warden.Options{})

// Add and configure a project
result, _ := w.Service.AddProject("my-project", "/home/user/code/my-project", "claude-code")

// Create container (same CreateContainerRequest as the client)
containerResult, _ := w.Service.CreateContainer(ctx, api.CreateContainerRequest{...})
```

See the [Go Packages](https://thesimonho.github.io/warden/reference/go/) reference for full API documentation.
