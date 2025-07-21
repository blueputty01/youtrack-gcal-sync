package googlecalendar

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// Client wraps the Google Calendar service.
type Client struct {
	srv *calendar.Service
}

// NewClient creates a new Google Calendar client.
func NewClient(ctx context.Context, token *oauth2.Token, config *oauth2.Config) (*Client, error) {
	httpClient := config.Client(ctx, token)
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Calendar client: %v", err)
	}
	return &Client{srv: srv}, nil
}

// Event represents a simplified Google Calendar event.
type Event struct {
	ID               string
	Summary          string
	HTMLLink         string
	Start            time.Time
	End              time.Time
	Status           string
	Organizer        string
	Recurrence       []string
	RecurringEventID string
	Updated          time.Time
}

// FetchEvents fetches events from the specified calendar ID.
// If a syncToken is provided, it will fetch only the events that have changed since the last sync.
// Otherwise, it will perform a full sync.
func (c *Client) FetchEvents(calendarID, syncToken string) ([]*Event, string, error) {
	var simplifiedEvents []*Event
	pageToken := ""

	for {
		eventsCall := c.srv.Events.List(calendarID).
			ShowDeleted(true).
			SingleEvents(false).
			PageToken(pageToken)

		if syncToken != "" {
			eventsCall.SyncToken(syncToken)
		} else {
			// Initial sync, fetch all events
			eventsCall.TimeMin(time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)) // Fetch last 30 days for initial sync
		}

		events, err := eventsCall.Do()
		if err != nil {
			// If sync token is invalid, perform a full sync
			if googleErr, ok := err.(*googleapi.Error); ok && googleErr.Code == 410 {
				return c.FetchEvents(calendarID, "")
			}
			return nil, "", fmt.Errorf("unable to retrieve events from calendar: %v", err)
		}

		for _, item := range events.Items {
			var organizer string
			if item.Organizer != nil {
				organizer = item.Organizer.Email
			}
			start := parseDateTime(item.Start)
			end := parseDateTime(item.End)
			updated, _ := time.Parse(time.RFC3339, item.Updated)

			simplifiedEvents = append(simplifiedEvents, &Event{
				ID:               item.Id,
				Summary:          item.Summary,
				HTMLLink:         item.HtmlLink,
				Start:            start,
				End:              end,
				Status:           item.Status,
				Organizer:        organizer,
				Recurrence:       item.Recurrence,
				RecurringEventID: item.RecurringEventId,
				Updated:          updated,
			})
		}

		if events.NextPageToken == "" {
			return simplifiedEvents, events.NextSyncToken, nil
		}
		pageToken = events.NextPageToken
	}
}

func parseDateTime(dateTime *calendar.EventDateTime) time.Time {
	if dateTime == nil {
		return time.Time{}
	}
	if dateTime.DateTime != "" {
		t, _ := time.Parse(time.RFC3339, dateTime.DateTime)
		return t
	}
	if dateTime.Date != "" {
		t, _ := time.Parse("2006-01-02", dateTime.Date)
		return t
	}
	return time.Time{}
}

// CreateEvent creates a new Google Calendar event.
func (c *Client) CreateEvent(calendarID, summary, description string, start, end time.Time) (*calendar.Event, error) {
	event := &calendar.Event{
		Summary:     summary,
		Description: description,
		Start:       &calendar.EventDateTime{Date: start.Format("2006-01-02")},
		End:         &calendar.EventDateTime{Date: end.AddDate(0, 0, 1).Format("2006-01-02")},
	}
	return c.srv.Events.Insert(calendarID, event).Do()
}

// UpdateEvent updates an existing Google Calendar event.
func (c *Client) UpdateEvent(calendarID, eventID, summary, description string, start, end time.Time) (*calendar.Event, error) {
	event := &calendar.Event{
		Summary:     summary,
		Description: description,
		Start:       &calendar.EventDateTime{Date: start.Format("2006-01-02")},
		End:         &calendar.EventDateTime{Date: end.AddDate(0, 0, 1).Format("2006-01-02")},
	}
	return c.srv.Events.Update(calendarID, eventID, event).Do()
}

// DeleteEvent deletes a Google Calendar event.
func (c *Client) DeleteEvent(calendarID, eventID string) error {
	return c.srv.Events.Delete(calendarID, eventID).Do()
}
