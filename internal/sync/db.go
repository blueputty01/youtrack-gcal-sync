package sync

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/blueputty01/task-sync/internal/utils"
	_ "github.com/mattn/go-sqlite3"
)

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

type DBItem struct {
	ids       map[string]string
	Summary   string
	StartDate time.Time
}

func servicesWithSuffix(services []string) []string {
	suffixedServices := make([]string, len(services))
	for i, service := range services {
		suffixedServices[i] = service + ColumnSuffix
	}
	return suffixedServices
}

func createSchema(db *sql.DB, services []string) error {
	suffixedServices := servicesWithSuffix(services)
	serviceColumns := make([]string, len(services))
	for i, suffixedService := range suffixedServices {
		serviceColumns[i] = fmt.Sprintf("%s TEXT UNIQUE", suffixedService)
	}

	mainTable := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS sync_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		summary TEXT,
		startDate INTEGER,
		%s
	);
	`, strings.Join(serviceColumns, ", "))

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
		indexStrings[i] = fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_sync_items_%s_id ON sync_items (%s)", service, service)
	}
	indices := strings.Join(indexStrings, "; ")

	slog.Info("Creating database schema", "Main", mainTable, "Update", updateTable, "Indices", indices)
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

func (c *DBClient) UpdateLastSyncTimestamp(service string, timestamp time.Time) error {
	_, err := c.Exec("INSERT INTO last_sync (service, timestamp) VALUES (?, ?) ON CONFLICT(service) DO UPDATE SET timestamp = excluded.timestamp", service, timestamp.Unix())
	if err != nil {
		return fmt.Errorf("failed to update last sync timestamp: %w", err)
	}
	return nil
}

func (c *DBClient) isValidColumn(col string) bool {
	return utils.ArrayContains(c.services, col)
}

func (c *DBClient) createUpdateIdQuery(item *DBItem, queryFormat string) ([]string, []interface{}, error) {
	updateIdQuery := make([]string, 0, len(item.ids))
	updateIdValues := make([]interface{}, 0, len(item.ids))
	for itemService, id := range item.ids {
		if !c.isValidColumn(itemService) {
			return nil, nil, fmt.Errorf("invalid service column: %s", itemService)
		}
		columnName := itemService + ColumnSuffix
		updateIdQuery = append(updateIdQuery, fmt.Sprintf(queryFormat, columnName))
		updateIdValues = append(updateIdValues, id)
	}
	return updateIdQuery, updateIdValues, nil
}

func (c *DBClient) UpdateItem(service string, serviceId string, item *DBItem) error {
	if !c.isValidColumn(service) {
		return fmt.Errorf("invalid service column: %s", service)
	}

	updateIdQuery, updateIdValues, err := c.createUpdateIdQuery(item, "%s=(?)")
	if err != nil {
		return err
	}

	setClauses := []string{"summary = (?)", "startDate = (?)"}
	if len(updateIdQuery) > 0 {
		setClauses = append(setClauses, updateIdQuery...)
	}

	args := append([]interface{}{item.Summary, item.StartDate.Unix()}, updateIdValues...)
	args = append(args, serviceId)

	_, err = c.Exec(fmt.Sprintf(
		"UPDATE sync_items SET %s WHERE %s=(?)",
		strings.Join(setClauses, ", "),
		service+ColumnSuffix), args...)
	if err != nil {
		return fmt.Errorf("failed to update calendar event: %w", err)
	}
	return nil
}

func (c *DBClient) InsertItem(item *DBItem) error {
	updateIdQuery, updateIdValues, err := c.createUpdateIdQuery(item, "%s")
	if err != nil {
		return err
	}

	columns := []string{"summary", "startDate"}
	placeholders := []string{"?", "?"}
	args := []interface{}{item.Summary, item.StartDate.Unix()}

	if len(updateIdQuery) > 0 {
		columns = append(columns, updateIdQuery...)
		for range updateIdQuery {
			placeholders = append(placeholders, "?")
		}
		args = append(args, updateIdValues...)
	}

	_, err = c.Exec(
		fmt.Sprintf("INSERT INTO sync_items (%s) VALUES (%s)", strings.Join(columns, ", "), strings.Join(placeholders, ", ")),
		args...)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}
	return nil
}

// GetItems retrieves items that exist in potentially multiple services based on the provided serviceItems map.
func (c *DBClient) GetItems(serviceItems map[string][]UpdatedItem) ([]DBItem, error) {
	columnsToSelect := append(append([]string{}, c.columnNames...), "summary", "startDate")
	query := fmt.Sprintf("SELECT %s FROM sync_items WHERE", strings.Join(columnsToSelect, ", "))

	var (
		queryBuilder strings.Builder
		args         []interface{}
	)

	idx := 0
	for serviceName, serviceItem := range serviceItems {
		if !c.isValidColumn(serviceName) {
			return nil, fmt.Errorf("invalid service column: %s", serviceName)
		}

		if idx > 0 {
			queryBuilder.WriteString(" OR ")
		}
		queryBuilder.WriteString(fmt.Sprintf("%s IN (", serviceName+ColumnSuffix))
		for itemIdx, item := range serviceItem {
			if itemIdx > 0 {
				queryBuilder.WriteString(", ")
			}
			queryBuilder.WriteString("?")
			args = append(args, item.ID)
		}
		queryBuilder.WriteString(")")
		idx += 1
	}

	rows, err := c.Query(query+" "+queryBuilder.String(), args...)
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

	for rows.Next() {
		var item DBItem
		item.ids = make(map[string]string)

		idValues := make([]sql.NullString, len(c.columnNames))
		var summary sql.NullString
		var date sql.NullInt64

		scanTargets := make([]interface{}, 0, len(c.columnNames)+2)
		for i := range idValues {
			scanTargets = append(scanTargets, &idValues[i])
		}
		scanTargets = append(scanTargets, &summary, &date)

		if err := rows.Scan(scanTargets...); err != nil {
			return nil, fmt.Errorf("failed to scan overlap item: %w", err)
		}

		for idx, col := range c.columnNames {
			if idValues[idx].Valid {
				serviceName := strings.TrimSuffix(col, ColumnSuffix)
				item.ids[serviceName] = idValues[idx].String
			}
		}
		item.Summary = summary.String
		if date.Valid {
			item.StartDate = time.Unix(date.Int64, 0)
		}
		overlapItems = append(overlapItems, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over overlap serviceItems: %w", err)
	}

	return overlapItems, nil
}
