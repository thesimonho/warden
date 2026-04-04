package db

import "fmt"

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
