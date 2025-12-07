package sync

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
	"time"
)

type DB struct {
	*sql.DB
}

func NewDB(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := createSchema(db); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &DB{db}, nil
}

func createSchema(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS sync_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		summary TEXT,
		cal_id TEXT UNIQUE,
		yt_id TEXT UNIQUE
	);

	CREATE TABLE IF NOT EXISTS last_sync (
		service TEXT PRIMARY KEY,
		timestamp INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_sync_items_cal_id ON sync_items (cal_id);
	CREATE INDEX IF NOT EXISTS idx_sync_items_yt_id ON sync_items (yt_id);
	`
	_, err := db.Exec(query)
	return err
}

func (db *DB) GetLastSyncTimestamp(service string) (time.Time, error) {
	var rawTime int64
	err := db.QueryRow("SELECT timestamp FROM last_sync WHERE service = ?", service).Scan(&rawTime)

	if errors.Is(err, sql.ErrNoRows) {
		slog.Warn("No last sync timestamp found, returning zero time", slog.String("service", service))
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last sync rawTime: %w", err)
	}
	return time.Unix(rawTime, 0), nil
}

func (db *DB) UpdateLastSyncTimestamp(service string, timestamp int64) error {
	_, err := db.Exec("INSERT INTO last_sync (service, timestamp) VALUES (?, ?) ON CONFLICT(service) DO UPDATE SET timestamp = excluded.timestamp", service, timestamp)
	if err != nil {
		return fmt.Errorf("failed to update last sync timestamp: %w", err)
	}
	return nil
}

func isValidColumn(col string) bool {
	validColumns := map[string]bool{
		"cal_id":  true,
		"yt_id":   true,
		"summary": true,
	}
	return validColumns[col]
}

func (db *DB) UpdateIssue(service, id, summary string) error {
	if !isValidColumn(service) {
		return fmt.Errorf("invalid service column: %s", service)
	}

	_, err := db.Exec("INSERT INTO sync_items (?, summary) VALUES (?, ?) ON CONFLICT(?) DO UPDATE SET summary = excluded.summary", service, id, summary, service)
	if err != nil {
		return fmt.Errorf("failed to update calendar event: %w", err)
	}
	return nil
}
