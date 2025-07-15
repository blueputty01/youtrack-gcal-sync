package sync

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"youtrack-calendar-sync/googlecalendar"
	"youtrack-calendar-sync/youtrack"

	"google.golang.org/api/calendar/v3"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	tmpfile, err := os.CreateTemp("", "test.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	db, err := NewDB(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create new DB: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpfile.Name())
	}

	return db, cleanup
}

func TestDBCreateAndGet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	item := &SyncItem{
		GCalID:        sql.NullString{String: "gcal-id", Valid: true},
		YTID:          sql.NullString{String: "yt-id", Valid: true},
		GCalUpdatedAt: sql.NullTime{Time: time.Now(), Valid: true},
		YTUpdatedAt:   sql.NullTime{Time: time.Now(), Valid: true},
	}

	id, err := db.CreateSyncItem(item)
	if err != nil {
		t.Fatalf("CreateSyncItem() error = %v", err)
	}

	retrieved, err := db.GetSyncItemByGCalID("gcal-id")
	if err != nil {
		t.Fatalf("GetSyncItemByGCalID() error = %v", err)
	}
	if retrieved.ID != int(id) {
		t.Errorf("Expected ID %d, got %d", id, retrieved.ID)
	}

	retrieved, err = db.GetSyncItemByYTID("yt-id")
	if err != nil {
		t.Fatalf("GetSyncItemByYTID() error = %v", err)
	}
	if retrieved.ID != int(id) {
		t.Errorf("Expected ID %d, got %d", id, retrieved.ID)
	}
}

func TestDBUpdateAndDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	item := &SyncItem{GCalID: sql.NullString{String: "gcal-id", Valid: true}}
	id, _ := db.CreateSyncItem(item)
	item.ID = int(id)
	item.YTID = sql.NullString{String: "yt-id", Valid: true}

	err := db.UpdateSyncItem(item)
	if err != nil {
		t.Fatalf("UpdateSyncItem() error = %v", err)
	}

	retrieved, _ := db.GetSyncItemByGCalID("gcal-id")
	if !retrieved.YTID.Valid || retrieved.YTID.String != "yt-id" {
		t.Errorf("Expected YTID to be 'yt-id', got %s", retrieved.YTID.String)
	}

	err = db.DeleteSyncItem(int(id))
	if err != nil {
		t.Fatalf("DeleteSyncItem() error = %v", err)
	}

	retrieved, _ = db.GetSyncItemByGCalID("gcal-id")
	if retrieved != nil {
		t.Error("Expected item to be deleted")
	}
}

func TestSyncTokens(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.SetGCalSyncToken("test-token")
	if err != nil {
		t.Fatalf("SetGCalSyncToken() error = %v", err)
	}

	token, err := db.GetGCalSyncToken()
	if err != nil {
		t.Fatalf("GetGCalSyncToken() error = %v", err)
	}
	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", token)
	}

	now := time.Now().Truncate(time.Second)
	err = db.SetYTLastSync(now)
	if err != nil {
		t.Fatalf("SetYTLastSync() error = %v", err)
	}

	lastSync, err := db.GetYTLastSync()
	if err != nil {
		t.Fatalf("GetYTLastSync() error = %v", err)
	}
	if !lastSync.Equal(now) {
		t.Errorf("Expected time %v, got %v", now, lastSync)
	}
}

type mockGCalClient struct {
	fetchEvents func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error)
	createEvent func(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error)
	updateEvent func(calendarID, eventID, summary, description string, start, end time.Time) (*calendar.Event, error)
}

func (m *mockGCalClient) FetchEvents(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
	return m.fetchEvents(calendarID, syncToken)
}
func (m *mockGCalClient) CreateEvent(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error) {
	return m.createEvent(calendarID, summary, description, start, end)
}
func (m *mockGCalClient) UpdateEvent(calendarID, eventID, summary, description string, start, end time.Time) (*calendar.Event, error) {
	return m.updateEvent(calendarID, eventID, summary, description, start, end)
}

type mockYTClient struct {
	getUpdatedIssues func(projectID string, since time.Time) ([]youtrack.Issue, error)
	createIssue      func(projectID, summary, description string, dueDate *time.Time) (*youtrack.Issue, error)
	updateIssue      func(issueID, summary, description string, dueDate *time.Time) error
	getBaseURL       func() string
}

func (m *mockYTClient) GetUpdatedIssues(projectID string, since time.Time) ([]youtrack.Issue, error) {
	return m.getUpdatedIssues(projectID, since)
}
func (m *mockYTClient) CreateIssue(projectID, summary, description string, dueDate *time.Time) (*youtrack.Issue, error) {
	return m.createIssue(projectID, summary, description, dueDate)
}
func (m *mockYTClient) UpdateIssue(issueID, summary, description string, dueDate *time.Time) error {
	return m.updateIssue(issueID, summary, description, dueDate)
}

func (m *mockYTClient) GetBaseURL() string {
	return m.getBaseURL()
}

func TestSynchronizer(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gcalClient := &mockGCalClient{
		fetchEvents: func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
			return []*googlecalendar.Event{
				{ID: "gcal-1", Summary: "New GCal Event", Updated: time.Now()},
			}, "new-gcal-token", nil
		},
		createEvent: func(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error) {
			return &calendar.Event{Id: "new-gcal-event"}, nil
		},
	}
	ytClient := &mockYTClient{
		getUpdatedIssues: func(projectID string, since time.Time) ([]youtrack.Issue, error) {
			return []youtrack.Issue{
				{ID: "yt-1", Summary: "New YT Issue", Updated: time.Now().UnixMilli()},
			}, nil
		},
		createIssue: func(projectID, summary, description string, dueDate *time.Time) (*youtrack.Issue, error) {
			return &youtrack.Issue{ID: "new-yt-issue"}, nil
		},
		getBaseURL: func() string {
			return "http://youtrack.example.com"
		},
	}

	s := NewSynchronizer(gcalClient, ytClient, db, "yt-project", "yt-query-project", "gcal-calendar")
	err := s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Assert that a new YouTrack issue was created for the new Google Calendar event
	item, err := db.GetSyncItemByGCalID("gcal-1")
	if err != nil {
		t.Fatalf("GetSyncItemByGCalID() error = %v", err)
	}
	if item == nil || !item.YTID.Valid {
		t.Error("Expected a new YouTrack issue to be created and stored in DB")
	}
}