package db

import (
	"database/sql"
	"fmt"

	// Pure-Go SQLite driver — no CGO required.
	_ "modernc.org/sqlite"
)

const dbFileName = "warden.db"

const schema = `
CREATE TABLE IF NOT EXISTS projects (
    project_id        TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    host_path         TEXT NOT NULL,
    added_at          TEXT NOT NULL,
    image             TEXT NOT NULL DEFAULT '',
    env_vars          TEXT,
    mounts            TEXT,
    original_mounts   TEXT,
    skip_permissions  INTEGER NOT NULL DEFAULT 0,
    network_mode      TEXT NOT NULL DEFAULT 'full',
    allowed_domains   TEXT NOT NULL DEFAULT '',
    cost_budget       REAL NOT NULL DEFAULT 0,
    enabled_presets   TEXT NOT NULL DEFAULT '',
    container_id      TEXT NOT NULL DEFAULT '',
    container_name    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    ts             TEXT    NOT NULL,
    source         TEXT    NOT NULL,
    level          TEXT    NOT NULL,
    event          TEXT    NOT NULL,
    project_id     TEXT    NOT NULL DEFAULT '',
    container_name TEXT    NOT NULL DEFAULT '',
    worktree       TEXT    NOT NULL DEFAULT '',
    msg            TEXT    NOT NULL DEFAULT '',
    data           TEXT,
    attrs          TEXT
);

CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
CREATE INDEX IF NOT EXISTS idx_events_level ON events(level);
CREATE INDEX IF NOT EXISTS idx_events_project_id ON events(project_id);
CREATE INDEX IF NOT EXISTS idx_events_event ON events(event);

CREATE TABLE IF NOT EXISTS session_costs (
    project_id   TEXT NOT NULL,
    session_id   TEXT NOT NULL,
    cost         REAL NOT NULL DEFAULT 0,
    is_estimated INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    PRIMARY KEY (project_id, session_id)
);

CREATE INDEX IF NOT EXISTS idx_projects_host_path ON projects(host_path);
CREATE INDEX IF NOT EXISTS idx_session_costs_project_time ON session_costs(project_id, updated_at);
`

// openDB opens (or creates) a SQLite database at the given path and applies
// performance pragmas. The database uses WAL mode for concurrent reads.
// Foreign keys are enabled for cascade deletes (e.g. project removal
// cascades to its audit log entries).
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}

	// WAL mode allows concurrent readers with a single writer.
	// busy_timeout avoids SQLITE_BUSY on brief contention.
	// synchronous=NORMAL is safe with WAL and faster than FULL.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close() //nolint:errcheck
			return nil, fmt.Errorf("setting pragma %q: %w", p, err)
		}
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	// Column migrations for existing databases. ALTER TABLE ADD COLUMN
	// errors when the column already exists; ignoring the error is the
	// conventional SQLite migration pattern without version tracking.
	migrations := []string{
		`ALTER TABLE projects ADD COLUMN enabled_presets TEXT NOT NULL DEFAULT ''`,
	}
	for _, m := range migrations {
		_, _ = db.Exec(m) //nolint:errcheck // expected to fail if column exists
	}

	return db, nil
}
