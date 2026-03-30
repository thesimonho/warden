# Database

Persistent storage layer using SQLite. Located at `db/`.

## Files

| File | Purpose |
| --- | --- |
| `store.go` | `Store` struct wrapping SQLite connection, `New(dbPath)` constructor, project operations (`ListProjects`, `HasProject`, `AddProject`, `RemoveProject`), settings operations (`GetSetting`, `SetSetting` for runtime/auditLogMode/disconnectKey/defaultProjectBudget/budgetAction*), session cost operations (`UpsertSessionCost` — keyed by session ID, monotonically non-decreasing so upsert always safe, `GetProjectTotalCost(container)` — single-container query for budget checks, `GetAllProjectTotalCosts` → `map[string]ProjectCostRow` summing across sessions with legacy `worktree_costs` fallback, `GetCostInTimeRange(container, since, until)` — time-filtered cost via session overlap (created_at..updated_at), `DeleteProjectCosts` — cleanup when audit logging is off), audit operations (`QueryAuditSummary`, `QueryTopTools`), `ProjectCostRow` type (TotalCost, IsEstimated) |
| `audit_writer.go` | `AuditWriter` — enforces mode-based filtering before writing events to the `events` table. Methods: `Write(entry *Entry, mode AuditLogMode)` applies mode filtering via a `standardEvents` allowlist (only allows session/budget/system categories in Standard mode), then calls `store.Write()`. The writer is the only permitted write path for audit events. |
| `db.go` | SQLite schema: `projects` table (project_id, name, added_at, image, project_path, env_vars, mounts, original_mounts, skip_permissions, network_mode, allowed_domains, enabled_access_items, cost_budget, agent_type TEXT NOT NULL DEFAULT 'claude-code'), `access_items` table (id, label, description, method, credentials JSON), `settings` table (key, value), `events` table (audit events with category, source, level), `session_costs` table (project_id, session_id, cost, is_estimated, created_at, updated_at). Additive migrations via ALTER TABLE (idempotent). |
| `access.go` | `InsertAccessItem(item)`, `GetAccessItem(id)`, `GetAccessItemsByIDs(ids)`, `ListAccessItems()`, `UpdateAccessItem(item)`, `DeleteAccessItem(id)` |

## Database Location

- Linux: `$XDG_CONFIG_HOME/warden/warden.db`
- macOS: `~/Library/Application Support/warden/warden.db`
- Windows: `%AppData%/warden/warden.db`
