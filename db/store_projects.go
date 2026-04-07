package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// projectColumns is the SELECT column list shared by project queries.
const projectColumns = `project_id, name, host_path, added_at, image, env_vars, mounts, original_mounts,
	skip_permissions, network_mode, allowed_domains, cost_budget, enabled_access_items, enabled_runtimes, forwarded_ports, agent_type, container_id, container_name`

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
		  enabled_runtimes, forwarded_ports, agent_type, container_id, container_name)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		p.EnabledRuntimes,
		p.ForwardedPorts,
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
func (l *Store) DeleteProject(projectID, agentType string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.db.Exec("DELETE FROM projects WHERE project_id = ? AND agent_type = ?", projectID, agentType); err != nil {
		return fmt.Errorf("deleting project %q/%s: %w", projectID, agentType, err)
	}
	return nil
}

// ListProjectKeys returns all project+agent pairs in insertion order.
func (l *Store) ListProjectKeys() ([]ProjectAgentKey, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	rows, err := l.db.Query("SELECT project_id, agent_type FROM projects ORDER BY added_at ASC")
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("listing project keys: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close() errors on read-only queries are non-actionable

	var keys []ProjectAgentKey
	for rows.Next() {
		var k ProjectAgentKey
		if err := rows.Scan(&k.ProjectID, &k.AgentType); err != nil {
			return nil, fmt.Errorf("scanning project key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// GetProject returns a project by its compound key, or nil if not found.
func (l *Store) GetProject(projectID, agentType string) (*ProjectRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	row := l.db.QueryRow(
		`SELECT `+projectColumns+` FROM projects WHERE project_id = ? AND agent_type = ?`,
		projectID, agentType,
	)
	l.mu.RUnlock()

	p, err := scanProjectRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting project %q/%s: %w", projectID, agentType, err)
	}
	return p, nil
}

// GetProjectsByPath returns all projects at a host path (one per agent type).
func (l *Store) GetProjectsByPath(hostPath string) ([]*ProjectRow, error) {
	if l == nil {
		return nil, nil
	}

	l.mu.RLock()
	rows, err := l.db.Query(
		`SELECT `+projectColumns+` FROM projects WHERE host_path = ? ORDER BY agent_type ASC`, hostPath,
	)
	l.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("getting projects by path %q: %w", hostPath, err)
	}
	defer rows.Close() //nolint:errcheck

	var result []*ProjectRow
	for rows.Next() {
		p, scanErr := scanProjectRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scanning project row: %w", scanErr)
		}
		result = append(result, p)
	}
	return result, rows.Err()
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

// ListAllProjects returns all projects as a flat slice ordered by insertion time.
func (l *Store) ListAllProjects() ([]*ProjectRow, error) {
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

	var result []*ProjectRow
	for rows.Next() {
		p, scanErr := scanProjectRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scanning project row: %w", scanErr)
		}
		result = append(result, p)
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
		&p.EnabledAccessItems, &p.EnabledRuntimes, &p.ForwardedPorts, &p.AgentType,
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

// HasProject reports whether a project with the given compound key exists.
func (l *Store) HasProject(projectID, agentType string) (bool, error) {
	if l == nil {
		return false, nil
	}

	l.mu.RLock()
	row := l.db.QueryRow("SELECT 1 FROM projects WHERE project_id = ? AND agent_type = ?", projectID, agentType)
	l.mu.RUnlock()

	var exists int
	err := row.Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking project %q/%s: %w", projectID, agentType, err)
	}
	return true, nil
}

// UpdateProjectContainer updates the container ID and name for a project.
// Used after container creation, rebuild, or deletion.
func (l *Store) UpdateProjectContainer(projectID, agentType, containerID, containerName string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		"UPDATE projects SET container_id = ?, container_name = ? WHERE project_id = ? AND agent_type = ?",
		containerID, containerName, projectID, agentType,
	)
	if err != nil {
		return fmt.Errorf("updating container for project %q/%s: %w", projectID, agentType, err)
	}
	return nil
}

// UpdateProjectSettings updates lightweight project settings that do not
// require container recreation (name, skip_permissions, cost_budget,
// container_name, allowed_domains, forwarded_ports). All other fields
// remain unchanged.
func (l *Store) UpdateProjectSettings(projectID, agentType, name, containerName string, skipPermissions bool, costBudget float64, allowedDomains, forwardedPorts string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		`UPDATE projects
		 SET name = ?, container_name = ?, skip_permissions = ?, cost_budget = ?, allowed_domains = ?, forwarded_ports = ?
		 WHERE project_id = ? AND agent_type = ?`,
		name, containerName, skipPermissions, costBudget, allowedDomains, forwardedPorts, projectID, agentType,
	)
	if err != nil {
		return fmt.Errorf("updating settings for project %q/%s: %w", projectID, agentType, err)
	}
	return nil
}
