# warden (headless engine server)

The core Warden binary. Starts the container engine and exposes the HTTP API without any frontend UI.

## Who is this for?

Developers integrating Warden into their own applications. You run this binary as a server and interact with it via the REST API, SSE events, and WebSocket terminal connections.

## Usage

```bash
# Start with defaults (127.0.0.1:8090)
./warden

# Custom address
ADDR=0.0.0.0:9000 ./warden
```

## API

All endpoints are under `/api/v1/`. See the [API server codemap](../../docs/codemaps/backend/api-server.md) for the full endpoint reference.

Key endpoints:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/projects` | List projects |
| POST | `/api/v1/containers` | Create a container |
| GET | `/api/v1/events` | SSE event stream |
| GET | `/api/v1/projects/{id}/ws/{wid}` | Terminal WebSocket |

## Building a frontend against this API

If you're building a web application, look at [`web/src/lib/api.ts`](../../web/src/lib/api.ts) — it's the HTTP client used by the web dashboard. Every call pattern you need is demonstrated there.

If you're building a Go application, see the [`client/`](../../client/) package (planned) for a ready-made Go HTTP client, or import `github.com/thesimonho/warden` directly to embed the engine in-process via `warden.New()`.
