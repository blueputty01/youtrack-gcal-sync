package sync

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB represents the database connection.
type DB struct {
	*sql.DB
}

// NewDB creates a new database connection and initializes the schema.
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

	CREATE TABLE IF NOT EXISTS last_sync (
		id INTEGER PRIMARY KEY,
		gcal_sync_token TEXT,
		yt_last_sync TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	return err
}

// SyncItem represents a synchronized item between Google Calendar and YouTrack.
type SyncItem struct {
	ID              int
	GCalID          sql.NullString
	YTID            sql.NullString
	GCalUpdatedAt   sql.NullTime
	YTUpdatedAt     sql.NullTime
}

// GetSyncItemByGCalID retrieves a SyncItem by the Google Calendar event ID.
func (db *DB) GetSyncItemByGCalID(gcalID string) (*SyncItem, error) {
	query := "SELECT id, gcal_id, yt_id, gcal_updated_at, yt_updated_at FROM sync_items WHERE gcal_id = ?"
	row := db.QueryRow(query, gcalID)
	return scanSyncItem(row)
}

// GetSyncItemByYTID retrieves a SyncItem by the YouTrack issue ID.
func (db *DB) GetSyncItemByYTID(ytID string) (*SyncItem, error) {
	query := "SELECT id, gcal_id, yt_id, gcal_updated_at, yt_updated_at FROM sync_items WHERE yt_id = ?"
	row := db.QueryRow(query, ytID)
	return scanSyncItem(row)
}

// GetAllSyncItems retrieves all sync items from the database.
func (db *DB) GetAllSyncItems() ([]*SyncItem, error) {
	query := "SELECT id, gcal_id, yt_id, gcal_updated_at, yt_updated_at FROM sync_items"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*SyncItem
	for rows.Next() {
		item, err := scanSyncItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func scanSyncItem(row interface{ Scan(dest ...interface{}) error }) (*SyncItem, error) {
	var item SyncItem
	err := row.Scan(&item.ID, &item.GCalID, &item.YTID, &item.GCalUpdatedAt, &item.YTUpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// CreateSyncItem creates a new sync item in the database.
func (db *DB) CreateSyncItem(item *SyncItem) (int64, error) {
	query := "INSERT INTO sync_items (gcal_id, yt_id, gcal_updated_at, yt_updated_at) VALUES (?, ?, ?, ?)"
	result, err := db.Exec(query, item.GCalID, item.YTID, item.GCalUpdatedAt, item.YTUpdatedAt)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateSyncItem updates an existing sync item in the database.
func (db *DB) UpdateSyncItem(item *SyncItem) error {
	query := "UPDATE sync_items SET gcal_id = ?, yt_id = ?, gcal_updated_at = ?, yt_updated_at = ? WHERE id = ?"
	_, err := db.Exec(query, item.GCalID, item.YTID, item.GCalUpdatedAt, item.YTUpdatedAt, item.ID)
	return err
}

// DeleteSyncItem deletes a sync item from the database.
func (db *DB) DeleteSyncItem(id int) error {
	query := "DELETE FROM sync_items WHERE id = ?"
	_, err := db.Exec(query, id)
	return err
}

// GetGCalSyncToken retrieves the Google Calendar sync token.
func (db *DB) GetGCalSyncToken() (string, error) {
	var token string
	query := "SELECT gcal_sync_token FROM last_sync WHERE id = 1"
	err := db.QueryRow(query).Scan(&token)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return token, nil
}

// SetGCalSyncToken sets the Google Calendar sync token.
func (db *DB) SetGCalSyncToken(token string) error {
	query := "INSERT OR REPLACE INTO last_sync (id, gcal_sync_token) VALUES (1, ?)"
	_, err := db.Exec(query, token)
	return err
}

// GetYTLastSync retrieves the last YouTrack sync time.
func (db *DB) GetYTLastSync() (time.Time, error) {
	var lastSync time.Time
	query := "SELECT yt_last_sync FROM last_sync WHERE id = 1"
	err := db.QueryRow(query).Scan(&lastSync)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return lastSync, nil
}

// SetYTLastSync sets the last YouTrack sync time.
func (db *DB) SetYTLastSync(t time.Time) error {
	query := "UPDATE last_sync SET yt_last_sync = ? WHERE id = 1"
	res, err := db.Exec(query, t)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		query = "INSERT INTO last_sync (id, yt_last_sync) VALUES (1, ?)"
		_, err = db.Exec(query, t)
	}
	return err
}