package watcher

// OffsetStore persists byte offsets for tailed files so the tailer can
// resume from where it left off after a restart. Implementations must be
// safe for concurrent use.
type OffsetStore interface {
	// LoadOffset returns the stored byte offset for a file, or 0 if no
	// offset has been stored.
	LoadOffset(projectID, agentType, filePath string) (int64, error)

	// SaveOffset persists the byte offset for a file (upsert).
	SaveOffset(projectID, agentType, filePath string, offset int64) error

	// DeleteOffset removes the stored offset for a single file.
	DeleteOffset(projectID, agentType, filePath string) error

	// DeleteOffsets removes all stored offsets for a project+agent pair.
	DeleteOffsets(projectID, agentType string) error
}
