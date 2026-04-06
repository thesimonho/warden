package db

import (
	"fmt"
	"time"
)

// ProjectCostRow holds aggregated cost data for a project from the DB.
type ProjectCostRow struct {
	TotalCost   float64
	IsEstimated bool
}

// UpsertSessionCost persists cost for a specific agent session. Cost for a
// given session ID is monotonically non-decreasing, so upserting is always
// safe — we simply take the max of old and new values. Project total cost
// is computed as SUM(cost) across all sessions for a project.
func (l *Store) UpsertSessionCost(projectID, agentType, sessionID string, cost float64, isEstimated bool) error {
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
		`INSERT INTO session_costs (project_id, agent_type, session_id, cost, is_estimated, updated_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, agent_type, session_id) DO UPDATE SET
		   cost = MAX(cost, excluded.cost),
		   is_estimated = excluded.is_estimated,
		   updated_at = excluded.updated_at`,
		projectID, agentType, sessionID, cost, isEstimated, now, now,
	)
	if err != nil {
		return fmt.Errorf("upserting session cost: %w", err)
	}
	return nil
}

// GetProjectTotalCost returns the cumulative cost for a single project+agent pair.
func (l *Store) GetProjectTotalCost(projectID, agentType string) (ProjectCostRow, error) {
	if l == nil {
		return ProjectCostRow{}, nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	var row ProjectCostRow
	err := l.db.QueryRow(
		"SELECT COALESCE(SUM(cost), 0), COALESCE(MAX(is_estimated), 0) FROM session_costs WHERE project_id = ? AND agent_type = ?",
		projectID, agentType,
	).Scan(&row.TotalCost, &row.IsEstimated)
	if err != nil {
		return ProjectCostRow{}, fmt.Errorf("querying project cost: %w", err)
	}
	return row, nil
}

// GetAllProjectTotalCosts returns cumulative costs for all project+agent pairs
// by summing across all sessions.
func (l *Store) GetAllProjectTotalCosts() (map[ProjectAgentKey]ProjectCostRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	costs := make(map[ProjectAgentKey]ProjectCostRow)

	rows, err := l.db.Query("SELECT project_id, agent_type, SUM(cost), MAX(is_estimated) FROM session_costs GROUP BY project_id, agent_type")
	if err != nil {
		return nil, fmt.Errorf("querying session costs: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close() errors on read-only queries are non-actionable

	for rows.Next() {
		var row ProjectCostRow
		var key ProjectAgentKey
		if err := rows.Scan(&key.ProjectID, &key.AgentType, &row.TotalCost, &row.IsEstimated); err != nil {
			return nil, fmt.Errorf("scanning session cost row: %w", err)
		}
		costs[key] = row
	}
	return costs, rows.Err()
}

// SessionCostRow holds cost data for a single session.
type SessionCostRow struct {
	SessionID   string
	Cost        float64
	IsEstimated bool
	CreatedAt   string
	UpdatedAt   string
}

// ListSessionCosts returns per-session cost data for a project+agent pair.
func (l *Store) ListSessionCosts(projectID, agentType string) ([]SessionCostRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	rows, err := l.db.Query(
		`SELECT session_id, cost, is_estimated, created_at, updated_at
		 FROM session_costs
		 WHERE project_id = ? AND agent_type = ?
		 ORDER BY updated_at DESC`,
		projectID, agentType,
	)
	if err != nil {
		return nil, fmt.Errorf("listing session costs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var result []SessionCostRow
	for rows.Next() {
		var r SessionCostRow
		if err := rows.Scan(&r.SessionID, &r.Cost, &r.IsEstimated, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning session cost: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
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

// DeleteProjectCosts removes all cost entries for a project+agent pair.
func (l *Store) DeleteProjectCosts(projectID, agentType string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.db.Exec("DELETE FROM session_costs WHERE project_id = ? AND agent_type = ?", projectID, agentType); err != nil {
		return fmt.Errorf("deleting session costs: %w", err)
	}
	return nil
}

// DeleteSessionCosts removes session cost entries matching the given filters.
// Supports scoping by project ID and time range. With no filters, clears all
// session costs.
func (l *Store) DeleteSessionCosts(projectID string, since, until time.Time) error {
	if l == nil {
		return nil
	}

	query := "DELETE FROM session_costs WHERE 1=1"
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

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.db.Exec(query, args...); err != nil {
		return fmt.Errorf("deleting session costs: %w", err)
	}
	return nil
}
