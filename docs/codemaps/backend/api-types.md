# API Types & Client

## api/

API contract types shared by the service layer, HTTP client, and TUI. Consumers import this package for types without depending on the service implementation.

| File | Purpose |
| --- | --- |
| `types.go` | `ProjectResult` (ProjectID, Name, ContainerID, ContainerName, AgentType), `WorktreeResult`, `ContainerResult` (with ProjectID, AgentType), `ValidateContainerResult`, `AuditLogMode` (off/standard/detailed), `SettingsResponse`, `UpdateSettingsRequest`, `UpdateSettingsResult`, `PostAuditEventRequest`, `DefaultMount` (host/container path pair for bind mounts, with `Required` flag for mandatory agent config mounts), `DefaultEnvVar` (host-side env var names available for passthrough), `DefaultsResponse` (user Mounts [] + EnvVars []), `DirEntry`, `DiffFileSummary`, `DiffResponse` (diff stats + raw unified diff), `AuditCategory` (session/agent/prompt/config/budget/system/debug), `AuditFilters` (keyed by `ProjectID` instead of `Container`), `AuditSummary`, `ToolCount`, `TimeRange`, `ClipboardUploadResponse` (Path of staged file), `AgentType` (type alias or enum for agent type) |
| `access.go` | `Item` (user-created access item with method, label, description, credentials), `Credential`, `Source`, `Transform`, `Injection`, `Method` (enum: git, ssh, etc.), `DetectionResult`, `AccessItemResponse` (Item + detection status), `AccessItemListResponse`, `CreateAccessItemRequest`, `UpdateAccessItemRequest`, `ResolveAccessItemsRequest` (Items []access.Item), `ResolveAccessItemsResponse` |

## client/

Go HTTP client for the Warden API. The Go equivalent of `web/src/lib/api.ts`.

| File | Purpose |
| --- | --- |
| `client.go` | `Client` struct, `New(baseURL)`, all API methods (projects, worktrees, containers, settings, access items, host utilities, audit log, SSE events, WebSocket terminal). Container operations accept/return `AgentType` fields. `APIError` type with `StatusCode`, `Code`, and `Message` fields, `TerminalConnection` interface, HTTP/SSE/WebSocket helpers, `deleteWithBody` helper for DELETE with request body |
| `client_test.go` | Tests with httptest servers for each endpoint pattern, SSE stream parser tests. Tests include `AgentType` in create/update operations. |
