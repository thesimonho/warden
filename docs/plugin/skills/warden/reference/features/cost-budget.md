# Cost & Budget

## For Developers

### HTTP API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `DELETE` | `/api/v1/projects/{projectId}/costs` | Reset project costs |
| `GET` | `/api/v1/settings` | Get settings (includes budget config) |
| `PUT` | `/api/v1/settings` | Update settings (budget, enforcement actions) |

Budget configuration is part of the settings object:

```json
{
  "defaultCostBudget": 10.00,
  "budgetActionWarn": true,
  "budgetActionStopWorktrees": false,
  "budgetActionStopContainer": false,
  "budgetActionPreventStart": false
}
```

Per-project budgets are set via the container configuration when creating or updating a container (`costBudget` field in the request body).

### Go Client

```go
c := client.New("http://localhost:8090")

// Reset a project's accumulated costs
c.ResetProjectCosts(ctx, projectID)

// Configure budget enforcement
c.UpdateSettings(ctx, api.UpdateSettingsRequest{
    DefaultCostBudget:        floatPtr(10.00),
    BudgetActionWarn:         boolPtr(true),
    BudgetActionStopWorktrees: boolPtr(false),
    BudgetActionStopContainer: boolPtr(true),
    BudgetActionPreventStart:  boolPtr(false),
})
```

### Go Library

```go
w, _ := warden.New(warden.Options{})

// Reset costs
w.Service.ResetProjectCosts(projectID)
```

Budget enforcement happens automatically when Warden captures cost data — no manual triggering needed.

See the [Go Packages](https://thesimonho.github.io/warden/reference/go/) reference for full API documentation.
