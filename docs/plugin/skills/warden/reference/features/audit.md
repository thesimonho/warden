# Audit

## For Developers

### HTTP API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/audit` | Query events with filters |
| `GET` | `/api/v1/audit/summary` | Aggregate statistics |
| `GET` | `/api/v1/audit/export?format=csv\|json` | Export filtered events |
| `GET` | `/api/v1/audit/projects` | List distinct project IDs with events |
| `POST` | `/api/v1/audit` | Write a custom event (for integrations) |
| `DELETE` | `/api/v1/audit` | Delete events matching filters |
| `DELETE` | `/api/v1/projects/{projectId}/audit` | Purge all audit data for a project |

Query parameters for `GET /api/v1/audit`:

| Parameter | Type | Description |
|-----------|------|-------------|
| `category` | string | Filter by category (comma-separated) |
| `source` | string | Filter by source |
| `level` | string | Filter by level |
| `projectId` | string | Filter by project |
| `worktree` | string | Filter by worktree |
| `since` | string | ISO 8601 start time |
| `until` | string | ISO 8601 end time |
| `limit` | int | Max events to return (default: 10,000) |
| `offset` | int | Pagination offset |

### Go Client

```go
c := client.New("http://localhost:8090")

// Query audit events
events, _ := c.GetAuditLog(ctx, api.AuditFilters{
    Category:  "agent",
    ProjectID: projectID,
    Since:     time.Now().Add(-24 * time.Hour),
})

// Get summary statistics
summary, _ := c.GetAuditSummary(ctx, api.AuditFilters{})

// Post a custom event (for integrations)
c.PostAuditEvent(ctx, api.PostAuditEventRequest{
    Event:   "deployment_started",
    Message: "Deploying v2.1.0 to staging",
    Level:   "info",
})

// Delete old events
c.DeleteAuditEvents(ctx, api.AuditFilters{
    Until: time.Now().Add(-30 * 24 * time.Hour),
})
```

### Go Library

```go
w, _ := warden.New(warden.Options{})

// Query events
events, _ := w.Service.GetAuditLog(api.AuditFilters{
    Category: "session",
    Limit:    100,
})

// Export as CSV
var buf bytes.Buffer
w.Service.WriteAuditCSV(&buf, api.AuditFilters{})
```

See the [Go Packages](https://thesimonho.github.io/warden/reference/go/) reference for full API documentation.
