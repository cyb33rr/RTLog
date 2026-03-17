package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cyb33rr/rtlog/internal/logfile"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

const schemaSQL = `
CREATE TABLE IF NOT EXISTS entries (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    ts       TEXT    NOT NULL,
    epoch    INTEGER NOT NULL,
    user     TEXT    NOT NULL,
    host     TEXT    NOT NULL,
    tty      TEXT    NOT NULL DEFAULT '',
    cwd      TEXT    NOT NULL,
    tool     TEXT    NOT NULL,
    cmd      TEXT    NOT NULL,
    exit     INTEGER NOT NULL DEFAULT 0,
    dur      REAL    NOT NULL DEFAULT 0.0,
    tag      TEXT    NOT NULL DEFAULT '',
    note     TEXT    NOT NULL DEFAULT '',
    out      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_entries_epoch ON entries(epoch);
CREATE INDEX IF NOT EXISTS idx_entries_tool  ON entries(tool);
CREATE INDEX IF NOT EXISTS idx_entries_tag   ON entries(tag);
`

// DB wraps a SQLite database for storing log entries.
type DB struct {
	db   *sql.DB
	path string
}

// Open opens or creates a SQLite database at dir/engagement.db.
// It enables WAL mode and, for new databases, creates the schema and stamps
// PRAGMA user_version.
func Open(dir, engagement string) (*DB, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dbPath := filepath.Join(dir, engagement+".db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for concurrent reads.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Avoid "database is locked" errors under concurrent access.
	if _, err := conn.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	// Check existing schema version.
	var version int
	if err := conn.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read user_version: %w", err)
	}

	if version > schemaVersion {
		fmt.Fprintf(os.Stderr, "warning: database schema version %d > expected %d; some features may not work correctly\n", version, schemaVersion)
	}

	// Only create schema and stamp version on a fresh database.
	if version == 0 {
		tx, err := conn.Begin()
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("begin schema tx: %w", err)
		}
		if _, err := tx.Exec(schemaSQL); err != nil {
			tx.Rollback()
			conn.Close()
			return nil, fmt.Errorf("create schema: %w", err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			tx.Rollback()
			conn.Close()
			return nil, fmt.Errorf("set user_version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("commit schema tx: %w", err)
		}
	}

	return &DB{db: conn, path: dbPath}, nil
}

// Insert inserts a single log entry into the database.
func (d *DB) Insert(e logfile.LogEntry) error {
	_, err := d.db.Exec(`
		INSERT INTO entries (ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Ts, e.Epoch, e.User, e.Host, e.TTY, e.Cwd, e.Tool, e.Cmd, e.Exit, e.Dur, e.Tag, e.Note, e.Out,
	)
	if err != nil {
		return fmt.Errorf("insert entry: %w", err)
	}
	return nil
}

// LoadAll returns all entries ordered by id ascending.
func (d *DB) LoadAll() ([]logfile.LogEntry, error) {
	return d.queryEntries("SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out FROM entries ORDER BY id ASC")
}

// LoadByDate returns entries whose ts starts with the given date string (YYYY-MM-DD).
func (d *DB) LoadByDate(dateStr string) ([]logfile.LogEntry, error) {
	return d.queryEntries(
		"SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out FROM entries WHERE ts LIKE ? ORDER BY id ASC",
		dateStr+"%",
	)
}

// Search returns entries matching the keyword across cmd, tool, cwd, tag, note, user, host fields.
func (d *DB) Search(keyword string) ([]logfile.LogEntry, error) {
	pattern := "%" + keyword + "%"
	return d.queryEntries(
		`SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out
		 FROM entries
		 WHERE cmd  LIKE ? OR tool LIKE ? OR cwd  LIKE ?
		    OR tag  LIKE ? OR note LIKE ? OR user LIKE ? OR host LIKE ?
		 ORDER BY id ASC`,
		pattern, pattern, pattern, pattern, pattern, pattern, pattern,
	)
}

// Tail returns the last n entries in chronological (ascending) order.
func (d *DB) Tail(n int) ([]logfile.LogEntry, error) {
	return d.queryEntries(
		`SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out
		 FROM (SELECT * FROM entries ORDER BY id DESC LIMIT ?)
		 ORDER BY id ASC`, n,
	)
}

// TailAfter returns entries with id > afterID in ascending order.
func (d *DB) TailAfter(afterID int64) ([]logfile.LogEntry, error) {
	return d.queryEntries(
		"SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out FROM entries WHERE id > ? ORDER BY id ASC",
		afterID,
	)
}

// Count returns the total number of entries.
func (d *DB) Count() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count entries: %w", err)
	}
	return count, nil
}

// Clear deletes all entries from the database and resets the AUTOINCREMENT counter.
func (d *DB) Clear() error {
	_, err := d.db.Exec("DELETE FROM entries")
	if err != nil {
		return fmt.Errorf("clear entries: %w", err)
	}
	_, err = d.db.Exec("DELETE FROM sqlite_sequence WHERE name = 'entries'")
	if err != nil {
		return fmt.Errorf("reset sequence: %w", err)
	}
	return nil
}

// GetByID returns a single entry by ID, or nil if not found.
func (d *DB) GetByID(id int64) (*logfile.LogEntry, error) {
	row := d.db.QueryRow(
		"SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out FROM entries WHERE id = ?", id)
	var e logfile.LogEntry
	err := row.Scan(&e.ID, &e.Ts, &e.Epoch, &e.User, &e.Host, &e.TTY,
		&e.Cwd, &e.Tool, &e.Cmd, &e.Exit, &e.Dur, &e.Tag, &e.Note, &e.Out)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entry by id: %w", err)
	}
	return &e, nil
}

// Delete removes a single entry by ID.
func (d *DB) Delete(id int64) error {
	_, err := d.db.Exec("DELETE FROM entries WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}
	return nil
}

// allowedUpdateColumns is the whitelist of columns that can be modified via Update.
var allowedUpdateColumns = map[string]bool{
	"tag":  true,
	"note": true,
}

// Update modifies allowed fields on a single entry by ID.
// Only "tag" and "note" columns are permitted; other column names return an error.
func (d *DB) Update(id int64, fields map[string]string) error {
	if len(fields) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Validate all columns first.
	cols := make([]string, 0, len(fields))
	for col := range fields {
		if !allowedUpdateColumns[col] {
			return fmt.Errorf("column %q is not editable", col)
		}
		cols = append(cols, col)
	}
	sort.Strings(cols)

	var setClauses []string
	var args []interface{}
	for _, col := range cols {
		setClauses = append(setClauses, col+" = ?")
		args = append(args, fields[col])
	}
	args = append(args, id)

	query := "UPDATE entries SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	_, err := d.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// Path returns the file path of the database.
func (d *DB) Path() string {
	return d.path
}

// queryEntries is a helper that executes a query and scans the results into LogEntry slices.
func (d *DB) queryEntries(query string, args ...interface{}) ([]logfile.LogEntry, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	var entries []logfile.LogEntry
	for rows.Next() {
		var e logfile.LogEntry
		if err := rows.Scan(
			&e.ID, &e.Ts, &e.Epoch, &e.User, &e.Host, &e.TTY,
			&e.Cwd, &e.Tool, &e.Cmd, &e.Exit, &e.Dur,
			&e.Tag, &e.Note, &e.Out,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Return empty slice instead of nil for consistency.
	if entries == nil {
		entries = []logfile.LogEntry{}
	}
	return entries, nil
}

