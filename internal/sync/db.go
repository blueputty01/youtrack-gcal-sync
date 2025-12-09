package sync

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/blueputty01/task-sync/internal/utils"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
	"strings"
	"time"
)

var TableName = "sync_items"
var ColumnSuffix = "_id"

type DBClient struct {
	*sql.DB
	services    []string
	columnNames []string
}

func NewDB(dataSourceName string, services []string) (*DBClient, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := createSchema(db, services); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &DBClient{db, services, servicesWithSuffix(services)}, nil
}

func servicesWithSuffix(services []string) []string {
	suffixedServices := make([]string, len(services))
	for i, service := range suffixedServices {
		suffixedServices[i] = service + ColumnSuffix
	}
	return suffixedServices
}

// TODO consider making this table dynamic based on the services used
func createSchema(db *sql.DB, services []string) error {
	suffixedServices := servicesWithSuffix(services)
	serviceColumns := make([]string, len(services))
	for i, suffixedService := range suffixedServices {
		serviceColumns[i] = fmt.Sprintf("%s TEXT UNIQUE", suffixedService)
	}

	mainTable := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		summary TEXT,
		%s
	);
	`, TableName, strings.Join(serviceColumns, ", "))

	quotedServices := make([]string, len(services))
	for i, service := range services {
		quotedServices[i] = fmt.Sprintf("'%s'", service)
	}
	updateTable := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS last_sync (
		service TEXT PRIMARY KEY,
		timestamp INTEGER,
	    CHECK ( timestamp >= 0 ),
		CHECK ( service IN (%s))
	);
	`, strings.Join(quotedServices, ", "))

	indexStrings := make([]string, len(suffixedServices))
	for i, service := range suffixedServices {
		indexStrings[i] = fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_sync_items_%s_id ON sync_items (%s)", service)
	}
	indices := strings.Join(indexStrings, "; ")
	_, err := db.Exec(mainTable + updateTable + indices)
	return err
}

func (c *DBClient) GetLastSyncTimestamp(service string) (time.Time, error) {
	var rawTime int64
	err := c.QueryRow("SELECT timestamp FROM last_sync WHERE service = ?", service).Scan(&rawTime)

	if errors.Is(err, sql.ErrNoRows) {
		slog.Warn("No last sync timestamp found, returning zero time", slog.String("service", service))
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last sync rawTime: %w", err)
	}
	return time.Unix(rawTime, 0), nil
}

func (c *DBClient) UpdateLastSyncTimestamp(service string, timestamp int64) error {
	_, err := c.Exec("INSERT INTO last_sync (service, timestamp) VALUES (?, ?) ON CONFLICT(service) DO UPDATE SET timestamp = excluded.timestamp", service, timestamp)
	if err != nil {
		return fmt.Errorf("failed to update last sync timestamp: %w", err)
	}
	return nil
}

func (c *DBClient) isValidColumn(col string) bool {
	return utils.ArrayContains(c.services, col[:len(col)-len(ColumnSuffix)])
}

func (c *DBClient) UpdateIssue(service, id, summary string) error {
	if !c.isValidColumn(service) {
		return fmt.Errorf("invalid service column: %s", service)
	}

	_, err := c.Exec("INSERT INTO sync_items (?, summary) VALUES (?, ?) ON CONFLICT(?) DO UPDATE SET summary = excluded.summary", service, id, summary, service)
	if err != nil {
		return fmt.Errorf("failed to update calendar event: %w", err)
	}
	return nil
}

type DBItem struct {
	ids     map[string]string
	Summary string
}

// GetItems retrieves items that exist in potentially multiple services based on the provided serviceItems map.
func (c *DBClient) GetItems(serviceItems map[string][]ItemsToUpdate) ([]DBItem, error) {
	query := fmt.Sprintf("SELECT %s, summary FROM sync_items WHERE", strings.Join(c.columnNames, ", "))

	var queryBuilder strings.Builder

	idx := 0
	for serviceName, serviceItem := range serviceItems {
		if idx > 0 {
			queryBuilder.WriteString(" OR ")
		}
		queryBuilder.WriteString(fmt.Sprintf("%s IN (", serviceName))
		for idx, item := range serviceItem {
			if idx > 0 {
				queryBuilder.WriteString(",")
			}
			queryBuilder.WriteString(item.ID)
		}
		queryBuilder.WriteString(")")
		idx += 1
	}

	rows, err := c.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query overlap serviceItems: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			slog.Error("Failed to close rows:", err)
		}
	}(rows)

	var overlapItems []DBItem

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Create a slice of interface{} pointers for scanning
	// Allocate one for each column
	scanValues := make([]interface{}, len(columns))
	for i := range scanValues {
		scanValues[i] = new(sql.NullString)
	}

	for rows.Next() {
		var item DBItem

		item.ids = make(map[string]string)
		ids := make([]string, len(c.services))

		for idx, col := range c.columnNames {
			item.ids[col] = ids[idx]
		}

		if err := rows.Scan(ids, &item.Summary); err != nil {
			return nil, fmt.Errorf("failed to scan overlap item: %w", err)
		}
		overlapItems = append(overlapItems, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over overlap serviceItems: %w", err)
	}

	return overlapItems, nil
}
