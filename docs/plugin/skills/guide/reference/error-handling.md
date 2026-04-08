# Error Handling

All Warden API errors return a consistent JSON structure with a human-readable message and a machine-readable code. Always match on the `code` field for programmatic error handling -- the `error` message may change between versions.

## Error response format

Every error response has the same shape:

```json
{
  "error": "human-readable description of what went wrong",
  "code": "MACHINE_CODE"
}
```

- **`error`** -- a descriptive message intended for logs or developer debugging. Do not match on this string.
- **`code`** -- a stable, uppercase identifier for the error class. Match on this for branching logic.

## Error codes

| Code | HTTP Status | Meaning | When you encounter it |
|------|-------------|---------|----------------------|
| `INVALID_BODY` | 400 | Malformed JSON in request body | Request body is not valid JSON, or exceeds the size limit |
| `INVALID_CONTAINER_ID` | 400 | Container ID format is invalid | Path parameter is not a valid Docker container ID |
| `INVALID_CONTAINER_NAME` | 400 | Container name format is invalid | Name contains characters not allowed by Docker |
| `INVALID_WORKTREE_ID` | 400 | Worktree ID format is invalid | Path parameter is not a valid worktree identifier |
| `INVALID_WORKTREE_NAME` | 400 | Worktree name violates git branch naming | Name contains characters not allowed in git branch names |
| `REQUIRED_FIELD` | 400 | A required field is missing | A required field was omitted or empty (e.g. `event` when posting audit) |
| `INVALID_PATH` | 400 | Path is invalid or unsafe | Host path does not exist, is not absolute, or fails safety checks |
| `INVALID_NETWORK_CONFIG` | 400 | Network mode or domain config is invalid | Invalid network mode value or domain format in allowed domains |
| `NOT_A_DIRECTORY` | 400 | Expected a directory, got a file | Host path points to a file instead of a directory |
| `NOT_FOUND` | 404 | Resource does not exist | Project, container, or worktree not found for the given ID |
| `BUDGET_EXCEEDED` | 403 | Cost budget exceeded, action blocked | Container restart blocked because `budgetActionPreventStart` is enabled and the project is over budget |
| `NAME_TAKEN` | 409 | Name collision | Project or container name already in use by another resource |
| `STALE_MOUNTS` | 409 | Mount sources have moved or been deleted | One or more bind mount source paths no longer exist on the host |
| `NOT_CONFIGURED` | 503 | Container runtime not available | Docker is not running or not found on the system |
| `INTERNAL` | 500 | Unexpected server error | Catch-all for unclassified errors. Check server logs for details. |

## Error handling patterns

### curl

```bash
# Check HTTP status and parse error body
response=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8090/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"projectPath": "/nonexistent"}')

http_code=$(echo "$response" | tail -1)
body=$(echo "$response" | sed '$d')

if [ "$http_code" -ge 400 ]; then
  error_code=$(echo "$body" | jq -r '.code')
  error_msg=$(echo "$body" | jq -r '.error')
  echo "Error ($error_code): $error_msg"

  case "$error_code" in
    INVALID_PATH)
      echo "Check that the path exists and is absolute"
      ;;
    NOT_CONFIGURED)
      echo "Docker is not running -- start Docker and try again"
      ;;
    *)
      echo "Unexpected error"
      ;;
  esac
fi
```

### TypeScript

```typescript
interface WardenError {
  error: string;
  code: string;
}

async function wardenRequest(url: string, options?: RequestInit) {
  const response = await fetch(url, options);

  if (!response.ok) {
    const body: WardenError = await response.json();

    switch (body.code) {
      case "NOT_FOUND":
        throw new Error(`Resource not found: ${body.error}`);
      case "BUDGET_EXCEEDED":
        throw new Error(`Budget exceeded: ${body.error}`);
      case "NOT_CONFIGURED":
        throw new Error("Docker is not running");
      case "STALE_MOUNTS":
        throw new Error(`Mount paths have changed: ${body.error}`);
      default:
        throw new Error(`Warden error (${body.code}): ${body.error}`);
    }
  }

  return response.json();
}
```

### Python

```python
import requests

def warden_request(method: str, url: str, **kwargs) -> dict:
    response = requests.request(method, url, **kwargs)

    if not response.ok:
        body = response.json()
        code = body.get("code", "UNKNOWN")
        message = body.get("error", "Unknown error")

        if code == "NOT_FOUND":
            raise LookupError(f"Resource not found: {message}")
        elif code == "BUDGET_EXCEEDED":
            raise PermissionError(f"Budget exceeded: {message}")
        elif code == "NOT_CONFIGURED":
            raise ConnectionError("Docker is not running")
        elif code == "STALE_MOUNTS":
            raise FileNotFoundError(f"Mount paths changed: {message}")
        else:
            raise RuntimeError(f"Warden error ({code}): {message}")

    if response.status_code == 204:
        return {}
    return response.json()
```

## Idempotency notes

Some operations are safe to retry without side effects:

| Operation | Idempotent? | Notes |
|-----------|-------------|-------|
| **Add project** (POST) | Yes | Adding the same host path again returns the existing project instead of creating a duplicate |
| **Connect terminal** | Yes | Connecting to an already-connected worktree attaches to the existing tmux session |
| **Health check** (GET) | Yes | Pure read, no side effects |
| **Get/list** operations | Yes | All GET endpoints are safe to retry |
| **Create container** | No | Returns an error if a container already exists for the project |
| **Delete operations** | Partially | Deleting a non-existent resource returns `NOT_FOUND` (404) |
| **Stop/kill operations** | Partially | Stopping an already-stopped container is a no-op, but the operation is still logged |
| **Post audit event** | No | Each call creates a new event |

When building retry logic, safe patterns include:

- Retry any failed GET request
- Retry POST to `/api/v1/projects` (add project) -- it returns the existing project on duplicate
- Retry terminal connect -- it attaches to the existing session
- Do not retry create container, delete, or audit post without checking the error code first
