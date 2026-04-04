package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

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
// Handles the string->json.RawMessage conversion for the credentials column.
func scanAccessItem(s scanner) (*AccessItemRow, error) {
	var item AccessItemRow
	var creds string
	if err := s.Scan(&item.ID, &item.Label, &item.Description, &item.Method, &creds); err != nil {
		return nil, err
	}
	item.Credentials = json.RawMessage(creds)
	return &item, nil
}
