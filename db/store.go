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

// Close closes the underlying database connection.
func (l *Store) Close() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.db.Close()
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

	args := []any{
		entry.Timestamp.Format(time.RFC3339Nano),
		string(entry.Source),
		string(entry.Level),
		entry.Event,
		entry.ProjectID,
		entry.AgentType,
		entry.ContainerName,
		entry.Worktree,
		entry.Message,
		dataStr,
		attrsStr,
	}

	// JSONL-sourced events use INSERT OR IGNORE for dedup via the
	// (project_id, agent_type, source_id) unique index. Hook/backend events
	// use plain INSERT so real constraint errors aren't silently swallowed.
	var query string
	if entry.SourceID != "" {
		query = `INSERT OR IGNORE INTO events (ts, source, level, event, project_id, agent_type, container_name, worktree, msg, data, attrs, source_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		args = append(args, entry.SourceID)
	} else {
		query = `INSERT INTO events (ts, source, level, event, project_id, agent_type, container_name, worktree, msg, data, attrs)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	}

	_, err := l.db.Exec(query, args...)
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

	query := "SELECT id, ts, source, level, event, project_id, agent_type, container_name, worktree, msg, data, attrs FROM events"
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

// scanEntries reads all rows from a query result into Entry slices.
func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry

	for rows.Next() {
		var (
			id            int64
			tsStr         string
			source        string
			level         string
			event         string
			projectID     string
			agentType     string
			containerName string
			worktree      string
			msg           string
			dataStr       sql.NullString
			attrsStr      sql.NullString
		)
		if err := rows.Scan(&id, &tsStr, &source, &level, &event, &projectID, &agentType, &containerName, &worktree, &msg, &dataStr, &attrsStr); err != nil {
			return nil, fmt.Errorf("scanning audit log row: %w", err)
		}

		ts, err := time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			continue // Skip rows with unparseable timestamps.
		}

		entry := Entry{
			ID:            id,
			Timestamp:     ts,
			Source:        Source(source),
			Level:         Level(level),
			Event:         event,
			ProjectID:     projectID,
			AgentType:     agentType,
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

// --- Audit query helpers ---

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

// ToolCountRow pairs a tool name with its invocation count.
type ToolCountRow struct {
	Name  string
	Count int
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
