package sync

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"youtrack-calendar-sync/googlecalendar"
	"youtrack-calendar-sync/youtrack"

	"google.golang.org/api/calendar/v3"
)

// GCalClient defines the interface for Google Calendar client operations.
type GCalClient interface {
	FetchEvents(calendarID, syncToken string) ([]*googlecalendar.Event, string, error)
	CreateEvent(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error)
	UpdateEvent(calendarID, eventID, summary, description string, start, end time.Time) (*calendar.Event, error)
	DeleteEvent(calendarID, eventID string) error
}

// YTClient defines the interface for YouTrack client operations.
type YTClient interface {
	GetUpdatedIssues(projectID string, since time.Time) ([]youtrack.Issue, error)
	CreateIssue(projectID, summary, description string, dueDate *time.Time) (*youtrack.Issue, error)
	UpdateIssue(issueID, summary, description string, dueDate *time.Time) error
	GetDeletedIssueIDs(projectID string, since time.Time) ([]string, error)
	GetBaseURL() string
}

// Synchronizer handles the synchronization between Google Calendar and YouTrack.
type Synchronizer struct {
	GoogleCalendarClient GCalClient
	YouTrackClient       YTClient
	DB                   *DB
	YouTrackProjectID    string
	YouTrackQueryProjectID string
	CalendarID           string
}

// NewSynchronizer creates a new Synchronizer instance.
func NewSynchronizer(
	googleClient GCalClient,
	youtrackClient YTClient,
	db *DB,
	youtrackProjectID, youtrackQueryProjectID, calendarID string,
) *Synchronizer {
	return &Synchronizer{
		GoogleCalendarClient: googleClient,
		YouTrackClient:       youtrackClient,
		DB:                   db,
		YouTrackProjectID:    youtrackProjectID,
		YouTrackQueryProjectID: youtrackQueryProjectID,
		CalendarID:           calendarID,
	}
}

// Sync performs a one-time synchronization.
func (s *Synchronizer) Sync() error {
	log.Println("Starting synchronization...")

	gcalSyncToken, err := s.DB.GetGCalSyncToken()
	if err != nil {
		return fmt.Errorf("failed to get Google Calendar sync token: %w", err)
	}
	ytLastSync, err := s.DB.GetYTLastSync()
	if err != nil {
		return fmt.Errorf("failed to get YouTrack last sync time: %w", err)
	}
	if ytLastSync.IsZero() {
		ytLastSync = time.Now().Add(-30 * 24 * time.Hour)
	}

	gcalEvents, newGCalSyncToken, err := s.GoogleCalendarClient.FetchEvents(s.CalendarID, gcalSyncToken)
	if err != nil {
		return fmt.Errorf("failed to fetch Google Calendar events: %w", err)
	}
	ytIssues, err := s.YouTrackClient.GetUpdatedIssues(s.YouTrackQueryProjectID, ytLastSync)
	if err != nil {
		return fmt.Errorf("failed to fetch YouTrack issues: %w", err)
	}
	ytDeletedIssueIDs, err := s.YouTrackClient.GetDeletedIssueIDs(s.YouTrackQueryProjectID, ytLastSync)
	if err != nil {
		return fmt.Errorf("failed to fetch deleted YouTrack issue IDs: %w", err)
	}

	if err := s.processGCalEvents(gcalEvents); err != nil {
		return err
	}
	if err := s.processYTissues(ytIssues); err != nil {
		return err
	}
	if err := s.handleDeletions(gcalEvents); err != nil {
		return err
	}
	if err := s.processYTDeletions(ytDeletedIssueIDs); err != nil {
		return err
	}

	if newGCalSyncToken != "" && newGCalSyncToken != gcalSyncToken {
		if err := s.DB.SetGCalSyncToken(newGCalSyncToken); err != nil {
			log.Printf("Error setting Google Calendar sync token: %v\n", err)
		}
	}
	if err := s.DB.SetYTLastSync(time.Now()); err != nil {
		log.Printf("Error setting YouTrack last sync time: %v\n", err)
	}

	log.Println("Synchronization finished.")
	return nil
}

func (s *Synchronizer) processGCalEvents(events []*googlecalendar.Event) error {
	for _, event := range events {
		if event.Status == "cancelled" {
			continue
		}

		syncItem, err := s.DB.GetSyncItemByGCalID(event.ID)
		if err != nil {
			log.Printf("Error getting sync item for GCal event %s: %v\n", event.ID, err)
			continue
		}

		if syncItem == nil {
			log.Printf("Creating YouTrack task for new Google Calendar event: %s (%s)\n", event.Summary, event.ID)
			issue, err := s.YouTrackClient.CreateIssue(s.YouTrackProjectID, event.Summary, event.HTMLLink, &event.Start)
			if err != nil {
				log.Printf("Error creating YouTrack task: %v\n", err)
				continue
			}
			_, err = s.DB.CreateSyncItem(&SyncItem{
				GCalID:        sql.NullString{String: event.ID, Valid: true},
				YTID:          sql.NullString{String: issue.ID, Valid: true},
				GCalUpdatedAt: sql.NullTime{Time: event.Updated, Valid: true},
				YTUpdatedAt:   sql.NullTime{Time: time.UnixMilli(issue.Updated), Valid: true},
			})
			if err != nil {
				log.Printf("Error creating sync item: %v\n", err)
			}
		} else {
			// Existing item, check for updates and conflicts
			if event.Updated.After(syncItem.GCalUpdatedAt.Time) {
				log.Printf("Google Calendar event '%s' was updated. Updating YouTrack.", event.Summary)
				err := s.YouTrackClient.UpdateIssue(syncItem.YTID.String, event.Summary, event.HTMLLink, &event.Start)
				if err != nil {
					log.Printf("Error updating YouTrack task %s: %v\n", syncItem.YTID.String, err)
				}
				syncItem.GCalUpdatedAt = sql.NullTime{Time: event.Updated, Valid: true}
				if err := s.DB.UpdateSyncItem(syncItem); err != nil {
					log.Printf("Error updating sync item: %v\n", err)
				}
			}
		}
	}
	return nil
}

func (s *Synchronizer) processYTissues(issues []youtrack.Issue) error {
	for _, issue := range issues {
		syncItem, err := s.DB.GetSyncItemByYTID(issue.ID)
		if err != nil {
			log.Printf("Error getting sync item for YouTrack issue %s: %v\n", issue.ID, err)
			continue
		}

		var dueDate time.Time
		for _, cf := range issue.CustomFields {
			if cf.Name == "Due Date" {
				if val, ok := cf.Value.(float64); ok {
					dueDate = time.UnixMilli(int64(val))
				}
			}
		}

		if syncItem == nil {
			if !dueDate.IsZero() {
				log.Printf("Creating Google Calendar event for new YouTrack task: %s (%s)\n", issue.Summary, issue.ID)
				description := fmt.Sprintf("YouTrack Issue: %s/issue/%s", s.YouTrackClient.GetBaseURL(), issue.ID)
				event, err := s.GoogleCalendarClient.CreateEvent(s.CalendarID, issue.Summary, description, dueDate, dueDate.Add(time.Hour))
				if err != nil {
					log.Printf("Error creating Google Calendar event: %v\n", err)
					continue
				}
				updatedTime, _ := time.Parse(time.RFC3339, event.Updated)
				_, err = s.DB.CreateSyncItem(&SyncItem{
					GCalID:        sql.NullString{String: event.Id, Valid: true},
					YTID:          sql.NullString{String: issue.ID, Valid: true},
					GCalUpdatedAt: sql.NullTime{Time: updatedTime, Valid: true},
					YTUpdatedAt:   sql.NullTime{Time: time.UnixMilli(issue.Updated), Valid: true},
				})
				if err != nil {
					log.Printf("Error creating sync item: %v\n", err)
				}
			}
		} else {
			issueUpdatedTime := time.UnixMilli(issue.Updated)
			if issueUpdatedTime.After(syncItem.YTUpdatedAt.Time) {
				log.Printf("YouTrack task '%s' was updated. Updating Google Calendar.", issue.Summary)
				description := fmt.Sprintf("YouTrack Issue: %s/issue/%s", s.YouTrackClient.GetBaseURL(), issue.ID)
				_, err := s.GoogleCalendarClient.UpdateEvent(s.CalendarID, syncItem.GCalID.String, issue.Summary, description, dueDate, dueDate.Add(time.Hour))
				if err != nil {
					log.Printf("Error updating Google Calendar event %s: %v\n", syncItem.GCalID.String, err)
				}
				syncItem.YTUpdatedAt = sql.NullTime{Time: issueUpdatedTime, Valid: true}
				if err := s.DB.UpdateSyncItem(syncItem); err != nil {
					log.Printf("Error updating sync item: %v\n", err)
				}
			}
		}
	}
	return nil
}

func (s *Synchronizer) handleDeletions(gcalEvents []*googlecalendar.Event) error {
	allDbItems, err := s.DB.GetAllSyncItems()
	if err != nil {
		return fmt.Errorf("failed to get all sync items: %w", err)
	}

	gcalEventMap := make(map[string]*googlecalendar.Event)
	for _, event := range gcalEvents {
		gcalEventMap[event.ID] = event
	}

	for _, item := range allDbItems {
		if item.GCalID.Valid {
			event, exists := gcalEventMap[item.GCalID.String]
			if exists && event.Status == "cancelled" {
				log.Printf("Google Calendar event %s was cancelled. Deleting sync item and updating YouTrack.", item.GCalID.String)
				err := s.YouTrackClient.UpdateIssue(item.YTID.String, "", "", nil) // Remove due date
				if err != nil {
					log.Printf("Error updating YouTrack issue %s: %v\n", item.YTID.String, err)
				}
				if err := s.DB.DeleteSyncItem(item.ID); err != nil {
					log.Printf("Error deleting sync item %d: %v\n", item.ID, err)
				}
			}
		}
	}
	return nil
}

func (s *Synchronizer) processYTDeletions(deletedYTIDs []string) error {
	for _, ytID := range deletedYTIDs {
		syncItem, err := s.DB.GetSyncItemByYTID(ytID)
		if err != nil {
			log.Printf("Error getting sync item for YouTrack issue %s: %v\n", ytID, err)
			continue
		}

		if syncItem != nil && syncItem.GCalID.Valid {
			log.Printf("YouTrack issue %s was deleted. Deleting Google Calendar event %s.", ytID, syncItem.GCalID.String)
			err := s.GoogleCalendarClient.DeleteEvent(s.CalendarID, syncItem.GCalID.String)
			if err != nil {
				log.Printf("Error deleting Google Calendar event %s: %v\n", syncItem.GCalID.String, err)
			}
			if err := s.DB.DeleteSyncItem(syncItem.ID); err != nil {
				log.Printf("Error deleting sync item %d: %v\n", syncItem.ID, err)
			}
		}
	}
	return nil
}

// StartSyncLoop starts a periodic synchronization loop.
func (s *Synchronizer) StartSyncLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.Sync(); err != nil {
			log.Printf("Error during synchronization loop: %v\n", err)
		}
	}
}
