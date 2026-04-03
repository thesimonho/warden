# Database

Persistent storage layer using SQLite. Located at `db/`.

## Files

| File | Purpose |
| --- | --- |
| `store.go` | `Store` struct wrapping SQLite connection, `New(dbPath)` constructor, project operations (`ListProjects`, `HasProject`, `AddProject`, `RemoveProject`), settings operations (`GetSetting`, `SetSetting` for runtime/auditLogMode/disconnectKey/defaultProjectBudget/budgetAction*), session cost operations (`UpsertSessionCost(projectID, agentType, ...)` — keyed by `(project_id, agent_type, session_id)`, monotonically non-decreasing so upsert always safe, `GetProjectTotalCost(projectID, agentType)` — single-project-agent query for budget checks, `GetAllProjectTotalCosts` → `map[string]ProjectCostRow` summing across sessions, `GetCostInTimeRange(projectID, agentType, since, until)` — time-filtered cost via session overlap (created_at..updated_at), `DeleteProjectCosts` — cleanup when audit logging is off), audit operations (`QueryAuditSummary`, `QueryTopTools`), `ProjectCostRow` type (TotalCost, IsEstimated) |
| `audit_writer.go` | `AuditWriter` — enforces mode-based filtering before writing events to the `events` table. Methods: `Write(entry *Entry, mode AuditLogMode)` applies mode filtering via a `standardEvents` allowlist (only allows session/budget/system categories in Standard mode), then calls `store.Write()`. The writer is the only permitted write path for audit events. |
| `db.go` | SQLite schema: `projects` table (project_id, agent_type, name, added_at, image, project_path, env_vars, mounts, original_mounts, skip_permissions, network_mode, allowed_domains, enabled_access_items, cost_budget; PK `(project_id, agent_type)`), `access_items` table (id, label, description, method, credentials JSON), `settings` table (key, value), `events` table (project_id, agent_type, category, source, level, source_id TEXT for JSONL dedup, and other audit data; PK `(project_id, agent_type, id)`), `session_costs` table (project_id, agent_type, session_id, cost, is_estimated, created_at, updated_at; PK `(project_id, agent_type, session_id)`). Unique index `idx_events_dedup` on (project_id, agent_type, source_id) with WHERE source_id IS NOT NULL enables INSERT OR IGNORE dedup behavior. Additive migrations via ALTER TABLE (idempotent). |
| `access.go` | `InsertAccessItem(item)`, `GetAccessItem(id)`, `GetAccessItemsByIDs(ids)`, `ListAccessItems()`, `UpdateAccessItem(item)`, `DeleteAccessItem(id)` |

## Database Location

- Linux: `$XDG_CONFIG_HOME/warden/warden.db`
- macOS: `~/Library/Application Support/warden/warden.db`
- Windows: `%AppData%/warden/warden.db`
