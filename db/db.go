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
    project_id        TEXT NOT NULL,
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
    enabled_access_items TEXT NOT NULL DEFAULT '',
    enabled_runtimes  TEXT NOT NULL DEFAULT 'node',
    agent_type        TEXT NOT NULL,
    container_id      TEXT NOT NULL DEFAULT '',
    container_name    TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (project_id, agent_type)
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
    agent_type     TEXT    NOT NULL DEFAULT '',
    container_name TEXT    NOT NULL DEFAULT '',
    worktree       TEXT    NOT NULL DEFAULT '',
    msg            TEXT    NOT NULL DEFAULT '',
    data           TEXT,
    attrs          TEXT,
    source_id      TEXT
);

CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
CREATE INDEX IF NOT EXISTS idx_events_level ON events(level);
CREATE INDEX IF NOT EXISTS idx_events_project_id ON events(project_id);
CREATE INDEX IF NOT EXISTS idx_events_event ON events(event);

CREATE UNIQUE INDEX IF NOT EXISTS idx_events_dedup ON events(project_id, agent_type, source_id) WHERE source_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS session_costs (
    project_id   TEXT NOT NULL,
    agent_type   TEXT NOT NULL,
    session_id   TEXT NOT NULL,
    cost         REAL NOT NULL DEFAULT 0,
    is_estimated INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    PRIMARY KEY (project_id, agent_type, session_id)
);

CREATE TABLE IF NOT EXISTS access_items (
    id          TEXT PRIMARY KEY,
    label       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    method      TEXT NOT NULL DEFAULT 'transport',
    credentials TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tailer_offsets (
    project_id  TEXT NOT NULL,
    agent_type  TEXT NOT NULL,
    file_path   TEXT NOT NULL,
    byte_offset INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, agent_type, file_path)
);

CREATE INDEX IF NOT EXISTS idx_projects_host_path ON projects(host_path);
CREATE INDEX IF NOT EXISTS idx_session_costs_project_time ON session_costs(project_id, agent_type, updated_at);
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

	// Detect old single-column PK and drop tables for compound PK migration.
	// This is a one-time destructive migration — all existing project, cost,
	// and audit data is lost.
	var pkCount int
	row := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('projects') WHERE pk > 0`)
	if err := row.Scan(&pkCount); err == nil && pkCount == 1 {
		db.Exec("DROP TABLE IF EXISTS projects")      //nolint:errcheck
		db.Exec("DROP TABLE IF EXISTS session_costs") //nolint:errcheck
		db.Exec("DROP TABLE IF EXISTS events")        //nolint:errcheck
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	// Additive column migrations — ALTER TABLE ADD COLUMN is idempotent
	// when the column already exists (the error is harmlessly ignored).
	db.Exec("ALTER TABLE projects ADD COLUMN enabled_runtimes TEXT NOT NULL DEFAULT 'node'")  //nolint:errcheck
	db.Exec("ALTER TABLE projects ADD COLUMN forwarded_ports TEXT NOT NULL DEFAULT ''") //nolint:errcheck

	return db, nil
}
