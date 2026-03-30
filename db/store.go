package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store writes structured audit log entries to a SQLite database.
//
// All methods are safe for concurrent use. A nil Store is a valid no-op:
// all methods return immediately without error.
type Store struct {
	mu sync.RWMutex
	db *sql.DB
}

// New creates a Store backed by a SQLite database at dir/warden.db.
// The directory is created if it does not exist. Pass the config directory
// root (e.g. ~/.config/warden/) — the database file is created inside it.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating db dir: %w", err)
	}

	path := filepath.Join(dir, dbFileName)
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// --- Event persistence ---

// Write inserts an entry into the database.
func (l *Store) Write(entry Entry) error {
	if l == nil {
		return nil
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	entry.Timestamp = entry.Timestamp.UTC()

	var dataStr, attrsStr sql.NullString
	if len(entry.Data) > 0 {
		dataStr = sql.NullString{String: string(entry.Data), Valid: true}
	}
	if len(entry.Attrs) > 0 {
		b, err := json.Marshal(entry.Attrs)
		if err != nil {
			return fmt.Errorf("marshaling attrs: %w", err)
		}
		attrsStr = sql.NullString{String: string(b), Valid: true}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		`INSERT INTO events (ts, source, level, event, project_id, container_name, worktree, msg, data, attrs)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Timestamp.Format(time.RFC3339Nano),
		string(entry.Source),
		string(entry.Level),
		entry.Event,
		entry.ProjectID,
		entry.ContainerName,
		entry.Worktree,
		entry.Message,
		dataStr,
		attrsStr,
	)
	if err != nil {
		return fmt.Errorf("inserting audit log entry: %w", err)
	}

	return nil
}

// Read returns all entries sorted by timestamp (newest first).
// This is a convenience wrapper around [Query] with empty filters.
func (l *Store) Read() ([]Entry, error) {
	return l.Query(QueryFilters{})
}

// buildWhereClause constructs the WHERE clause and args from query filters.
// Shared by Query, Delete, and aggregate queries.
func buildWhereClause(filters QueryFilters) ([]string, []any) {
	var where []string
	var args []any

	if filters.Source != "" {
		where = append(where, "source = ?")
		args = append(args, string(filters.Source))
	}
	if filters.Level != "" {
		where = append(where, "level = ?")
		args = append(args, string(filters.Level))
	}
	if filters.ProjectID != "" {
		where = append(where, "project_id = ?")
		args = append(args, filters.ProjectID)
	}
	if filters.Worktree != "" {
		where = append(where, "worktree = ?")
		args = append(args, filters.Worktree)
	}
	if filters.Event != "" {
		where = append(where, "event = ?")
		args = append(args, filters.Event)
	}
	if len(filters.Events) > 0 {
		placeholders := make([]string, len(filters.Events))
		for i, e := range filters.Events {
			placeholders[i] = "?"
			args = append(args, e)
		}
		where = append(where, "event IN ("+strings.Join(placeholders, ",")+")")
	} else if len(filters.ExcludeEvents) > 0 {
		placeholders := make([]string, len(filters.ExcludeEvents))
		for i, e := range filters.ExcludeEvents {
			placeholders[i] = "?"
			args = append(args, e)
		}
		where = append(where, "event NOT IN ("+strings.Join(placeholders, ",")+")")
	}
	if !filters.Since.IsZero() {
		where = append(where, "ts >= ?")
		args = append(args, filters.Since.Format(time.RFC3339Nano))
	}
	if !filters.Until.IsZero() {
		where = append(where, "ts < ?")
		args = append(args, filters.Until.Format(time.RFC3339Nano))
	}

	return where, args
}

// Query returns entries matching the given filters, sorted by timestamp
// (newest first). Zero-value filter fields are ignored.
func (l *Store) Query(filters QueryFilters) ([]Entry, error) {
	if l == nil {
		return nil, nil
	}

	where, args := buildWhereClause(filters)

	query := "SELECT ts, source, level, event, project_id, container_name, worktree, msg, data, attrs FROM events"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY ts DESC, id DESC"

	limit := filters.Limit
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	query += fmt.Sprintf(" LIMIT %d", limit)
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filters.Offset)
	}

	l.mu.RLock()
	rows, err := l.db.Query(query, args...)
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("querying audit log: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close() errors on read-only queries are non-actionable

	return scanEntries(rows)
}

// Delete removes events matching the given filters. With no filters set,
// this is equivalent to [Clear]. Supports scoping by project, worktree,
// and time range (since/until).
func (l *Store) Delete(filters QueryFilters) (int64, error) {
	if l == nil {
		return 0, nil
	}

	where, args := buildWhereClause(filters)

	query := "DELETE FROM events"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	result, err := l.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("deleting events: %w", err)
	}

	return result.RowsAffected()
}

// DistinctProjectIDs returns sorted, unique project IDs that have at least
// one event logged. Empty project IDs are excluded.
func (l *Store) DistinctProjectIDs() ([]string, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	rows, err := l.db.Query(
		"SELECT DISTINCT project_id FROM events WHERE project_id != '' ORDER BY project_id",
	)
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("querying distinct project IDs: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close() errors on read-only queries are non-actionable

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning project ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Clear removes all audit log entries.
func (l *Store) Clear() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.db.Exec("DELETE FROM events"); err != nil {
		return fmt.Errorf("clearing audit log: %w", err)
	}
	return nil
}

// AuditSummaryRow holds aggregate audit data from a summary query.
type AuditSummaryRow struct {
	TotalSessions   int
	TotalToolUses   int
	TotalPrompts    int
	UniqueProjects  int
	UniqueWorktrees int
	Earliest        string
	Latest          string
}

// QueryAuditSummary returns aggregate audit statistics matching the given filters.
// Only ProjectID, Worktree, Since, and Until fields are used for filtering.
func (l *Store) QueryAuditSummary(filters QueryFilters) (*AuditSummaryRow, error) {
	if l == nil {
		return &AuditSummaryRow{}, nil
	}

	where, args := buildWhereClause(filters)

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	query := `SELECT
		COALESCE(SUM(CASE WHEN event = 'session_start' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN event = 'tool_use' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN event = 'user_prompt' THEN 1 ELSE 0 END), 0),
		COUNT(DISTINCT CASE WHEN project_id != '' THEN project_id END),
		COUNT(DISTINCT CASE WHEN worktree != '' THEN worktree END),
		COALESCE(MIN(ts), ''),
		COALESCE(MAX(ts), '')
	FROM events` + whereClause

	l.mu.RLock()
	row := l.db.QueryRow(query, args...)
	l.mu.RUnlock()

	var summary AuditSummaryRow
	if err := row.Scan(
		&summary.TotalSessions,
		&summary.TotalToolUses,
		&summary.TotalPrompts,
		&summary.UniqueProjects,
		&summary.UniqueWorktrees,
		&summary.Earliest,
		&summary.Latest,
	); err != nil {
		return nil, fmt.Errorf("querying audit summary: %w", err)
	}
	return &summary, nil
}

// QueryTopTools returns the most frequently used tools from tool_use events.
// Only ProjectID, Worktree, Since, and Until fields are used for filtering.
func (l *Store) QueryTopTools(filters QueryFilters, limit int) ([]ToolCountRow, error) {
	if l == nil {
		return nil, nil
	}

	where, args := buildWhereClause(filters)
	where = append(where, "event = 'tool_use'")
	where = append(where, "msg != ''")

	if limit <= 0 {
		limit = 10
	}

	query := fmt.Sprintf(
		`SELECT msg, COUNT(*) as cnt FROM events WHERE %s GROUP BY msg ORDER BY cnt DESC LIMIT %d`,
		strings.Join(where, " AND "), limit,
	)

	l.mu.RLock()
	rows, err := l.db.Query(query, args...)
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("querying top tools: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close() errors on read-only queries are non-actionable

	var tools []ToolCountRow
	for rows.Next() {
		var t ToolCountRow
		if err := rows.Scan(&t.Name, &t.Count); err != nil {
			return nil, fmt.Errorf("scanning tool count: %w", err)
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

// ToolCountRow pairs a tool name with its invocation count.
type ToolCountRow struct {
	Name  string
	Count int
}

// Close closes the underlying database connection.
func (l *Store) Close() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.db.Close()
}

// --- Project persistence ---

// projectColumns is the SELECT column list shared by project queries.
const projectColumns = `project_id, name, host_path, added_at, image, env_vars, mounts, original_mounts,
	skip_permissions, network_mode, allowed_domains, cost_budget, enabled_access_items, agent_type, container_id, container_name`

// InsertProject adds a project to the database. If a project with the same
// project ID already exists, it is replaced (upsert).
func (l *Store) InsertProject(p ProjectRow) error {
	if l == nil {
		return nil
	}

	if p.AddedAt.IsZero() {
		p.AddedAt = time.Now().UTC()
	}

	var envVars, mounts, origMounts sql.NullString
	if len(p.EnvVars) > 0 {
		envVars = sql.NullString{String: string(p.EnvVars), Valid: true}
	}
	if len(p.Mounts) > 0 {
		mounts = sql.NullString{String: string(p.Mounts), Valid: true}
	}
	if len(p.OriginalMounts) > 0 {
		origMounts = sql.NullString{String: string(p.OriginalMounts), Valid: true}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	agentType := p.AgentType
	if agentType == "" {
		agentType = defaultAgentType
	}

	_, err := l.db.Exec(
		`INSERT OR REPLACE INTO projects
		 (project_id, name, host_path, added_at, image, env_vars, mounts, original_mounts,
		  skip_permissions, network_mode, allowed_domains, cost_budget, enabled_access_items,
		  agent_type, container_id, container_name)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProjectID,
		p.Name,
		p.HostPath,
		p.AddedAt.Format(time.RFC3339Nano),
		p.Image,
		envVars,
		mounts,
		origMounts,
		p.SkipPermissions,
		p.NetworkMode,
		p.AllowedDomains,
		p.CostBudget,
		p.EnabledAccessItems,
		agentType,
		p.ContainerID,
		p.ContainerName,
	)
	if err != nil {
		return fmt.Errorf("inserting project %q: %w", p.ProjectID, err)
	}
	return nil
}

// DeleteProject removes a project from the projects table. Audit events and
// cost data are intentionally retained so the audit page can show historical data.
func (l *Store) DeleteProject(projectID string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.db.Exec("DELETE FROM projects WHERE project_id = ?", projectID); err != nil {
		return fmt.Errorf("deleting project %q: %w", projectID, err)
	}
	return nil
}

// ListProjectIDs returns all project IDs in insertion order.
func (l *Store) ListProjectIDs() ([]string, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	rows, err := l.db.Query("SELECT project_id FROM projects ORDER BY added_at ASC")
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("listing project IDs: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close() errors on read-only queries are non-actionable

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning project ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetProject returns a project by project ID, or nil if not found.
func (l *Store) GetProject(projectID string) (*ProjectRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	row := l.db.QueryRow(
		`SELECT `+projectColumns+` FROM projects WHERE project_id = ?`, projectID,
	)
	l.mu.RUnlock()

	p, err := scanProjectRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting project %q: %w", projectID, err)
	}
	return p, nil
}

// GetProjectByPath returns a project by its host path, or nil if not found.
func (l *Store) GetProjectByPath(hostPath string) (*ProjectRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	row := l.db.QueryRow(
		`SELECT `+projectColumns+` FROM projects WHERE host_path = ?`, hostPath,
	)
	l.mu.RUnlock()

	p, err := scanProjectRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting project by path %q: %w", hostPath, err)
	}
	return p, nil
}

// GetProjectByContainerName returns a project by its Docker container name, or nil if not found.
func (l *Store) GetProjectByContainerName(containerName string) (*ProjectRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	row := l.db.QueryRow(
		`SELECT `+projectColumns+` FROM projects WHERE container_name = ? OR name = ?`,
		containerName, containerName,
	)
	l.mu.RUnlock()

	p, err := scanProjectRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting project by container name %q: %w", containerName, err)
	}
	return p, nil
}

// ListAllProjects returns all projects indexed by project ID.
func (l *Store) ListAllProjects() (map[string]*ProjectRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	rows, err := l.db.Query(`SELECT ` + projectColumns + ` FROM projects ORDER BY added_at ASC`)
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("listing all projects: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close() errors on read-only queries are non-actionable

	result := make(map[string]*ProjectRow)
	for rows.Next() {
		p, scanErr := scanProjectRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scanning project row: %w", scanErr)
		}
		result[p.ProjectID] = p
	}
	return result, rows.Err()
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanProjectRow scans a single project row from any scanner.
func scanProjectRow(s scanner) (*ProjectRow, error) {
	var (
		p          ProjectRow
		addedAtStr string
		envVars    sql.NullString
		mounts     sql.NullString
		origMounts sql.NullString
		skipPerms  int
	)
	err := s.Scan(
		&p.ProjectID, &p.Name, &p.HostPath, &addedAtStr, &p.Image,
		&envVars, &mounts, &origMounts,
		&skipPerms, &p.NetworkMode, &p.AllowedDomains, &p.CostBudget,
		&p.EnabledAccessItems, &p.AgentType,
		&p.ContainerID, &p.ContainerName,
	)
	if err != nil {
		return nil, err
	}

	if ts, parseErr := time.Parse(time.RFC3339Nano, addedAtStr); parseErr == nil {
		p.AddedAt = ts
	}
	if envVars.Valid {
		p.EnvVars = json.RawMessage(envVars.String)
	}
	if mounts.Valid {
		p.Mounts = json.RawMessage(mounts.String)
	}
	if origMounts.Valid {
		p.OriginalMounts = json.RawMessage(origMounts.String)
	}
	p.SkipPermissions = skipPerms != 0

	return &p, nil
}

// HasProject reports whether a project with the given project ID exists.
func (l *Store) HasProject(projectID string) (bool, error) {
	if l == nil {
		return false, nil
	}

	l.mu.RLock()
	row := l.db.QueryRow("SELECT 1 FROM projects WHERE project_id = ?", projectID)
	l.mu.RUnlock()

	var exists int
	err := row.Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking project %q: %w", projectID, err)
	}
	return true, nil
}

// UpdateProjectContainer updates the container ID and name for a project.
// Used after container creation, rebuild, or deletion.
func (l *Store) UpdateProjectContainer(projectID, containerID, containerName string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		"UPDATE projects SET container_id = ?, container_name = ? WHERE project_id = ?",
		containerID, containerName, projectID,
	)
	if err != nil {
		return fmt.Errorf("updating container for project %q: %w", projectID, err)
	}
	return nil
}

// --- Session cost persistence ---

// UpsertSessionCost persists cost for a specific agent session. Cost for a
// given session ID is monotonically non-decreasing, so upserting is always
// safe — we simply take the max of old and new values. Project total cost
// is computed as SUM(cost) across all sessions for a project.
func (l *Store) UpsertSessionCost(projectID, sessionID string, cost float64, isEstimated bool) error {
	if l == nil || sessionID == "" {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	// created_at is intentionally excluded from the UPDATE clause — it records
	// when a session was first seen, giving each session a time span for
	// range-filtered cost queries (created_at..updated_at).
	_, err := l.db.Exec(
		`INSERT INTO session_costs (project_id, session_id, cost, is_estimated, updated_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, session_id) DO UPDATE SET
		   cost = MAX(cost, excluded.cost),
		   is_estimated = excluded.is_estimated,
		   updated_at = excluded.updated_at`,
		projectID, sessionID, cost, isEstimated, now, now,
	)
	if err != nil {
		return fmt.Errorf("upserting session cost: %w", err)
	}
	return nil
}

// ProjectCostRow holds aggregated cost data for a project from the DB.
type ProjectCostRow struct {
	TotalCost   float64
	IsEstimated bool
}

// GetProjectTotalCost returns the cumulative cost for a single project.
func (l *Store) GetProjectTotalCost(projectID string) (ProjectCostRow, error) {
	if l == nil {
		return ProjectCostRow{}, nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	var row ProjectCostRow
	err := l.db.QueryRow(
		"SELECT COALESCE(SUM(cost), 0), COALESCE(MAX(is_estimated), 0) FROM session_costs WHERE project_id = ?",
		projectID,
	).Scan(&row.TotalCost, &row.IsEstimated)
	if err != nil {
		return ProjectCostRow{}, fmt.Errorf("querying project cost: %w", err)
	}
	return row, nil
}

// GetAllProjectTotalCosts returns cumulative costs for all projects by
// summing across all sessions.
func (l *Store) GetAllProjectTotalCosts() (map[string]ProjectCostRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	costs := make(map[string]ProjectCostRow)

	rows, err := l.db.Query("SELECT project_id, SUM(cost), MAX(is_estimated) FROM session_costs GROUP BY project_id")
	if err != nil {
		return nil, fmt.Errorf("querying session costs: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close() errors on read-only queries are non-actionable

	for rows.Next() {
		var row ProjectCostRow
		var id string
		if err := rows.Scan(&id, &row.TotalCost, &row.IsEstimated); err != nil {
			return nil, fmt.Errorf("scanning session cost row: %w", err)
		}
		costs[id] = row
	}
	return costs, rows.Err()
}

// GetCostInTimeRange returns the total cost for sessions that overlap
// with the given time range. A session overlaps if it was created before
// the range ends and last updated after the range starts.
// Pass zero-value times to leave either bound open.
func (l *Store) GetCostInTimeRange(projectID string, since, until time.Time) (ProjectCostRow, error) {
	if l == nil {
		return ProjectCostRow{}, nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	query := "SELECT COALESCE(SUM(cost), 0), COALESCE(MAX(is_estimated), 0) FROM session_costs WHERE 1=1"
	var args []any

	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	if !since.IsZero() {
		query += " AND updated_at >= ?"
		args = append(args, since.Format(time.RFC3339Nano))
	}
	if !until.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, until.Format(time.RFC3339Nano))
	}

	var row ProjectCostRow
	if err := l.db.QueryRow(query, args...).Scan(&row.TotalCost, &row.IsEstimated); err != nil {
		return ProjectCostRow{}, fmt.Errorf("querying cost in time range: %w", err)
	}
	return row, nil
}

// DeleteProjectCosts removes all cost entries for a project.
func (l *Store) DeleteProjectCosts(projectID string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.db.Exec("DELETE FROM session_costs WHERE project_id = ?", projectID); err != nil {
		return fmt.Errorf("deleting session costs: %w", err)
	}
	return nil
}

// --- Access item persistence ---

// AccessItemRow represents a user-created access item stored in the database.
// Built-in items (Git, SSH) are not stored — they come from the access package.
type AccessItemRow struct {
	// ID is the unique identifier (UUID for user items).
	ID string
	// Label is the human-readable display name.
	Label string
	// Description explains what this access item provides.
	Description string
	// Method is the delivery strategy (only "transport" for now).
	Method string
	// Credentials is JSON-encoded []access.Credential.
	Credentials json.RawMessage
}

// InsertAccessItem adds a user-created access item to the database.
func (l *Store) InsertAccessItem(item AccessItemRow) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		`INSERT INTO access_items (id, label, description, method, credentials)
		 VALUES (?, ?, ?, ?, ?)`,
		item.ID, item.Label, item.Description, item.Method, string(item.Credentials),
	)
	if err != nil {
		return fmt.Errorf("inserting access item %q: %w", item.ID, err)
	}
	return nil
}

// GetAccessItem returns a user-created access item by ID, or nil if not found.
func (l *Store) GetAccessItem(id string) (*AccessItemRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	row := l.db.QueryRow(
		"SELECT id, label, description, method, credentials FROM access_items WHERE id = ?", id,
	)
	l.mu.RUnlock()

	item, err := scanAccessItem(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting access item %q: %w", id, err)
	}
	return item, nil
}

// GetAccessItemsByIDs returns user-created access items matching the given IDs.
// IDs not found are silently skipped.
func (l *Store) GetAccessItemsByIDs(ids []string) ([]AccessItemRow, error) {
	if l == nil || len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT id, label, description, method, credentials FROM access_items WHERE id IN (" +
		strings.Join(placeholders, ",") + ")"

	l.mu.RLock()
	rows, err := l.db.Query(query, args...)
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("getting access items by IDs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var items []AccessItemRow
	for rows.Next() {
		item, scanErr := scanAccessItem(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scanning access item: %w", scanErr)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// ListAccessItems returns all user-created access items.
func (l *Store) ListAccessItems() ([]AccessItemRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	rows, err := l.db.Query("SELECT id, label, description, method, credentials FROM access_items ORDER BY label ASC")
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("listing access items: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var items []AccessItemRow
	for rows.Next() {
		item, scanErr := scanAccessItem(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scanning access item: %w", scanErr)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// UpdateAccessItem updates a user-created access item.
func (l *Store) UpdateAccessItem(item AccessItemRow) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		`UPDATE access_items SET label = ?, description = ?, method = ?, credentials = ?
		 WHERE id = ?`,
		item.Label, item.Description, item.Method, string(item.Credentials), item.ID,
	)
	if err != nil {
		return fmt.Errorf("updating access item %q: %w", item.ID, err)
	}
	return nil
}

// DeleteAccessItem removes a user-created access item by ID.
func (l *Store) DeleteAccessItem(id string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.db.Exec("DELETE FROM access_items WHERE id = ?", id); err != nil {
		return fmt.Errorf("deleting access item %q: %w", id, err)
	}
	return nil
}

// scanAccessItem scans a single access item row from any scanner.
// Handles the string→json.RawMessage conversion for the credentials column.
func scanAccessItem(s scanner) (*AccessItemRow, error) {
	var item AccessItemRow
	var creds string
	if err := s.Scan(&item.ID, &item.Label, &item.Description, &item.Method, &creds); err != nil {
		return nil, err
	}
	item.Credentials = json.RawMessage(creds)
	return &item, nil
}

// --- Settings persistence ---

// GetSetting returns the value for a settings key, or the provided default
// if the key does not exist.
func (l *Store) GetSetting(key, defaultValue string) string {
	if l == nil {
		return defaultValue
	}

	l.mu.RLock()
	row := l.db.QueryRow("SELECT value FROM settings WHERE key = ?", key)
	l.mu.RUnlock()

	var value string
	if err := row.Scan(&value); err != nil {
		return defaultValue
	}
	return value
}

// SetSetting writes a key-value pair to the settings table (upsert).
func (l *Store) SetSetting(key, value string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		"INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("setting %q: %w", key, err)
	}
	return nil
}

// scanEntries reads all rows from a query result into Entry slices.
func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry

	for rows.Next() {
		var (
			tsStr         string
			source        string
			level         string
			event         string
			projectID     string
			containerName string
			worktree      string
			msg           string
			dataStr       sql.NullString
			attrsStr      sql.NullString
		)
		if err := rows.Scan(&tsStr, &source, &level, &event, &projectID, &containerName, &worktree, &msg, &dataStr, &attrsStr); err != nil {
			return nil, fmt.Errorf("scanning audit log row: %w", err)
		}

		ts, err := time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			continue // Skip rows with unparseable timestamps.
		}

		entry := Entry{
			Timestamp:     ts,
			Source:        Source(source),
			Level:         Level(level),
			Event:         event,
			ProjectID:     projectID,
			ContainerName: containerName,
			Worktree:      worktree,
			Message:       msg,
		}

		if dataStr.Valid && dataStr.String != "" {
			entry.Data = json.RawMessage(dataStr.String)
		}
		if attrsStr.Valid && attrsStr.String != "" {
			if err := json.Unmarshal([]byte(attrsStr.String), &entry.Attrs); err != nil {
				// Store raw string as a single-key map rather than losing the data.
				entry.Attrs = map[string]any{"_raw": attrsStr.String}
			}
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
