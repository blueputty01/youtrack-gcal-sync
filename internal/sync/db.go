package sync

import (
	"database/sql"
	"fmt"
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
		gcal_id TEXT UNIQUE,
		yt_id TEXT UNIQUE,
		gcal_updated_at TIMESTAMP,
		yt_updated_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_sync_items_gcal_id ON sync_items (gcal_id);
	CREATE INDEX IF NOT EXISTS idx_sync_items_yt_id ON sync_items (yt_id);
	
	CREATE INDEX IF NOT EXISTS idx_sync_items_gcal_updated_at ON sync_items (gcal_updated_at);
	CREATE INDEX IF NOT EXISTS idx_sync_items_yt_updated_at ON sync_items (yt_updated_at);
	`
	_, err := db.Exec(query)
	return err
}
