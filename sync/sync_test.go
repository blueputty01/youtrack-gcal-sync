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

func setupTest(t *testing.T) (*DB, *mockGCalClient, *mockYTClient, *Synchronizer, func()) {
	db, cleanupDB := setupTestDB(t)

	gcalClient := &mockGCalClient{}
	ytClient := &mockYTClient{}

	s := NewSynchronizer(gcalClient, ytClient, db, "yt-project", "yt-query-project", "gcal-calendar")

	cleanup := func() {
		cleanupDB()
	}

	return db, gcalClient, ytClient, s, cleanup
}

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
	fetchEventsFunc func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error)
	createEventFunc func(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error)
	updateEventFunc func(calendarID, eventID, summary, description string, start, end time.Time) (*calendar.Event, error)
	deleteEventFunc func(calendarID, eventID string) error
}

func (m *mockGCalClient) FetchEvents(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
	return m.fetchEventsFunc(calendarID, syncToken)
}
func (m *mockGCalClient) CreateEvent(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error) {
	return m.createEventFunc(calendarID, summary, description, start, end)
}
func (m *mockGCalClient) UpdateEvent(calendarID, eventID, summary, description string, start, end time.Time) (*calendar.Event, error) {
	return m.updateEventFunc(calendarID, eventID, summary, description, start, end)
}
func (m *mockGCalClient) DeleteEvent(calendarID, eventID string) error {
	return m.deleteEventFunc(calendarID, eventID)
}

type mockYTClient struct {
	getUpdatedIssuesFunc   func(projectID string, since time.Time) ([]youtrack.Issue, error)
	createIssueFunc        func(projectID, summary, description string, dueDate *time.Time) (*youtrack.Issue, error)
	updateIssueFunc        func(issueID, summary, description string, dueDate *time.Time) error
	getDeletedIssueIDsFunc func(projectID string, since time.Time) ([]string, error)
	getBaseURLFunc         func() string
}

func (m *mockYTClient) GetUpdatedIssues(projectID string, since time.Time) ([]youtrack.Issue, error) {
	return m.getUpdatedIssuesFunc(projectID, since)
}
func (m *mockYTClient) CreateIssue(projectID, summary, description string, dueDate *time.Time) (*youtrack.Issue, error) {
	return m.createIssueFunc(projectID, summary, description, dueDate)
}
func (m *mockYTClient) UpdateIssue(issueID, summary, description string, dueDate *time.Time) error {
	return m.updateIssueFunc(issueID, summary, description, dueDate)
}
func (m *mockYTClient) GetDeletedIssueIDs(projectID string, since time.Time) ([]string, error) {
	return m.getDeletedIssueIDsFunc(projectID, since)
}
func (m *mockYTClient) GetBaseURL() string {
	return m.getBaseURLFunc()
}

func TestSync_NewGCalEventCreatesYTIssue(t *testing.T) {
	db, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return []*googlecalendar.Event{
			{ID: "gcal-1", Summary: "New GCal Event", Updated: time.Now()},
		}, "new-gcal-token", nil
	}
	ytClient.createIssueFunc = func(projectID, summary, description string, dueDate *time.Time) (*youtrack.Issue, error) {
		return &youtrack.Issue{ID: "new-yt-issue"}, nil
	}
	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return nil, nil
	}
	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return nil, nil
	}

	err := s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	item, err := db.GetSyncItemByGCalID("gcal-1")
	if err != nil {
		t.Fatalf("GetSyncItemByGCalID() error = %v", err)
	}
	if item == nil || !item.YTID.Valid || item.YTID.String != "new-yt-issue" {
		t.Error("Expected a new YouTrack issue to be created and stored in DB")
	}
}
func TestSync_NewYTIssueCreatesGCalEvent(t *testing.T) {
	db, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return []youtrack.Issue{
			{ID: "yt-1", Summary: "New YT Issue", Updated: time.Now().UnixMilli(), CustomFields: []youtrack.CustomField{
				{Name: "Due Date", Value: float64(time.Now().UnixMilli())},
			}},
		}, nil
	}
	gcalClient.createEventFunc = func(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error) {
		return &calendar.Event{Id: "new-gcal-event"}, nil
	}
	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return nil, "new-gcal-token", nil
	}
	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return nil, nil
	}
	ytClient.getBaseURLFunc = func() string {
		return "http://youtrack.example.com"
	}

	err := s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	item, err := db.GetSyncItemByYTID("yt-1")
	if err != nil {
		t.Fatalf("GetSyncItemByYTID() error = %v", err)
	}
	if item == nil || !item.GCalID.Valid || item.GCalID.String != "new-gcal-event" {
		t.Error("Expected a new GCal event to be created and stored in DB")
	}
}
func TestSync_NewYTIssueWithoutDueDateDoesNotCreateGCalEvent(t *testing.T) {
	db, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return []youtrack.Issue{
			{ID: "yt-1", Summary: "New YT Issue", Updated: time.Now().UnixMilli()},
		}, nil
	}
	gcalClient.createEventFunc = func(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error) {
		t.Error("CreateEvent should not be called")
		return nil, nil
	}
	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return nil, "new-gcal-token", nil
	}
	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return nil, nil
	}

	err := s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	item, err := db.GetSyncItemByYTID("yt-1")
	if err != nil {
		t.Fatalf("GetSyncItemByYTID() error = %v", err)
	}
	if item != nil {
		t.Error("Expected no sync item to be created")
	}
}
func TestSync_UpdateGCalEventUpdatesYTIssue(t *testing.T) {
	db, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	updatedTime := time.Now()
	_, err := db.CreateSyncItem(&SyncItem{
		GCalID:        sql.NullString{String: "gcal-1", Valid: true},
		YTID:          sql.NullString{String: "yt-1", Valid: true},
		GCalUpdatedAt: sql.NullTime{Time: updatedTime.Add(-time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateSyncItem() error = %v", err)
	}

	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return []*googlecalendar.Event{
			{ID: "gcal-1", Summary: "Updated GCal Event", Updated: updatedTime},
		}, "new-gcal-token", nil
	}
	var updatedSummary string
	ytClient.updateIssueFunc = func(issueID, summary, description string, dueDate *time.Time) error {
		updatedSummary = summary
		return nil
	}
	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return nil, nil
	}
	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return nil, nil
	}

	err = s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if updatedSummary != "Updated GCal Event" {
		t.Errorf("Expected YouTrack issue to be updated, but it was not")
	}
}
func TestSync_UpdateYTIssueUpdatesGCalEvent(t *testing.T) {
	db, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	updatedTime := time.Now()
	_, err := db.CreateSyncItem(&SyncItem{
		GCalID:      sql.NullString{String: "gcal-1", Valid: true},
		YTID:        sql.NullString{String: "yt-1", Valid: true},
		YTUpdatedAt: sql.NullTime{Time: updatedTime.Add(-time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateSyncItem() error = %v", err)
	}

	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return []youtrack.Issue{
			{ID: "yt-1", Summary: "Updated YT Issue", Updated: updatedTime.UnixMilli(), CustomFields: []youtrack.CustomField{
				{Name: "Due Date", Value: float64(time.Now().UnixMilli())},
			}},
		}, nil
	}
	var updatedSummary string
	gcalClient.updateEventFunc = func(calendarID, eventID, summary, description string, start, end time.Time) (*calendar.Event, error) {
		updatedSummary = summary
		return &calendar.Event{}, nil
	}
	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return nil, "new-gcal-token", nil
	}
	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return nil, nil
	}
	ytClient.getBaseURLFunc = func() string {
		return "http://youtrack.example.com"
	}

	err = s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if updatedSummary != "Updated YT Issue" {
		t.Errorf("Expected GCal event to be updated, but it was not")
	}
}
func TestSync_CancelledGCalEventUpdatesYTIssue(t *testing.T) {
	db, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	_, err := db.CreateSyncItem(&SyncItem{
		GCalID: sql.NullString{String: "gcal-1", Valid: true},
		YTID:   sql.NullString{String: "yt-1", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateSyncItem() error = %v", err)
	}

	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return []*googlecalendar.Event{
			{ID: "gcal-1", Status: "cancelled"},
		}, "new-gcal-token", nil
	}
	var dueDateCleared bool
	ytClient.updateIssueFunc = func(issueID, summary, description string, dueDate *time.Time) error {
		if dueDate == nil {
			dueDateCleared = true
		}
		return nil
	}
	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return nil, nil
	}
	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return nil, nil
	}

	err = s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if !dueDateCleared {
		t.Errorf("Expected YouTrack issue due date to be cleared, but it was not")
	}
	item, err := db.GetSyncItemByGCalID("gcal-1")
	if err != nil {
		t.Fatalf("GetSyncItemByGCalID() error = %v", err)
	}
	if item != nil {
		t.Error("Expected sync item to be deleted")
	}
}
func TestSync_DeletedYTIssueDeletesGCalEvent(t *testing.T) {
	db, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	_, err := db.CreateSyncItem(&SyncItem{
		GCalID: sql.NullString{String: "gcal-1", Valid: true},
		YTID:   sql.NullString{String: "yt-1", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateSyncItem() error = %v", err)
	}

	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return []string{"yt-1"}, nil
	}
	var eventDeleted bool
	gcalClient.deleteEventFunc = func(calendarID, eventID string) error {
		if eventID == "gcal-1" {
			eventDeleted = true
		}
		return nil
	}
	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return nil, "new-gcal-token", nil
	}
	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return nil, nil
	}

	err = s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if !eventDeleted {
		t.Errorf("Expected GCal event to be deleted, but it was not")
	}
	item, err := db.GetSyncItemByYTID("yt-1")
	if err != nil {
		t.Fatalf("GetSyncItemByYTID() error = %v", err)
	}
	if item != nil {
		t.Error("Expected sync item to be deleted")
	}
}
func TestSync_UpdatesTokensAndTimestamps(t *testing.T) {
	db, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return nil, "new-gcal-token", nil
	}
	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return nil, nil
	}
	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return nil, nil
	}

	err := s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	gcalToken, err := db.GetGCalSyncToken()
	if err != nil {
		t.Fatalf("GetGCalSyncToken() error = %v", err)
	}
	if gcalToken != "new-gcal-token" {
		t.Errorf("Expected GCal sync token to be updated")
	}

	ytLastSync, err := db.GetYTLastSync()
	if err != nil {
		t.Fatalf("GetYTLastSync() error = %v", err)
	}
	if ytLastSync.IsZero() {
		t.Errorf("Expected YT last sync time to be updated")
	}
}
func TestSync_NoChanges(t *testing.T) {
	_, gcalClient, ytClient, s, cleanup := setupTest(t)
	defer cleanup()

	gcalClient.fetchEventsFunc = func(calendarID, syncToken string) ([]*googlecalendar.Event, string, error) {
		return nil, "", nil
	}
	ytClient.getUpdatedIssuesFunc = func(projectID string, since time.Time) ([]youtrack.Issue, error) {
		return nil, nil
	}
	ytClient.getDeletedIssueIDsFunc = func(projectID string, since time.Time) ([]string, error) {
		return nil, nil
	}
	gcalClient.createEventFunc = func(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error) {
		t.Error("CreateEvent should not be called")
		return nil, nil
	}
	ytClient.createIssueFunc = func(projectID, summary, description string, dueDate *time.Time) (*youtrack.Issue, error) {
		t.Error("CreateIssue should not be called")
		return nil, nil
	}

	err := s.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}