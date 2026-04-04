package db

import "database/sql"

// LoadTailerOffset returns the stored byte offset for a tailed file.
// Returns 0 if no offset is stored.
func (l *Store) LoadTailerOffset(projectID, agentType, filePath string) (int64, error) {
	if l == nil {
		return 0, nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	var offset int64
	err := l.db.QueryRow(
		"SELECT byte_offset FROM tailer_offsets WHERE project_id = ? AND agent_type = ? AND file_path = ?",
		projectID, agentType, filePath,
	).Scan(&offset)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return offset, nil
}

// SaveTailerOffset persists the byte offset for a tailed file (upsert).
func (l *Store) SaveTailerOffset(projectID, agentType, filePath string, offset int64) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		"INSERT INTO tailer_offsets (project_id, agent_type, file_path, byte_offset) VALUES (?, ?, ?, ?) "+
			"ON CONFLICT (project_id, agent_type, file_path) DO UPDATE SET byte_offset = excluded.byte_offset",
		projectID, agentType, filePath, offset,
	)
	return err
}

// DeleteTailerOffset removes the stored offset for a single file.
func (l *Store) DeleteTailerOffset(projectID, agentType, filePath string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		"DELETE FROM tailer_offsets WHERE project_id = ? AND agent_type = ? AND file_path = ?",
		projectID, agentType, filePath,
	)
	return err
}

// DeleteTailerOffsets removes all stored offsets for a project+agent pair.
func (l *Store) DeleteTailerOffsets(projectID, agentType string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(
		"DELETE FROM tailer_offsets WHERE project_id = ? AND agent_type = ?",
		projectID, agentType,
	)
	return err
}

// OffsetStoreAdapter wraps a Store to implement watcher.OffsetStore.
// Lives in the db package to avoid the watcher package importing db.
type OffsetStoreAdapter struct {
	Store *Store
}

// LoadOffset implements watcher.OffsetStore.
func (a *OffsetStoreAdapter) LoadOffset(projectID, agentType, filePath string) (int64, error) {
	return a.Store.LoadTailerOffset(projectID, agentType, filePath)
}

// SaveOffset implements watcher.OffsetStore.
func (a *OffsetStoreAdapter) SaveOffset(projectID, agentType, filePath string, offset int64) error {
	return a.Store.SaveTailerOffset(projectID, agentType, filePath, offset)
}

// DeleteOffset implements watcher.OffsetStore.
func (a *OffsetStoreAdapter) DeleteOffset(projectID, agentType, filePath string) error {
	return a.Store.DeleteTailerOffset(projectID, agentType, filePath)
}

// DeleteOffsets implements watcher.OffsetStore.
func (a *OffsetStoreAdapter) DeleteOffsets(projectID, agentType string) error {
	return a.Store.DeleteTailerOffsets(projectID, agentType)
}
