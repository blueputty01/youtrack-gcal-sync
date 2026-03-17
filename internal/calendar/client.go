package calendar

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/blueputty01/task-sync/internal/sync"
	"google.golang.org/api/calendar/v3"
)

const ServiceName = "calendar"

type Client struct {
	srv              *calendar.Service
	ActiveCalendar   string
	FinishedCalendar string
}

func NewClient(activeCalendar, finishedCalendar string) (*Client, error) {
	srv, err := GetGoogleCalendarService()
	if err != nil {
		slog.Error("Failed to create Google Calendar service:", "error", err)
		return nil, err
	}

	return &Client{
		srv:              srv,
		ActiveCalendar:   activeCalendar,
		FinishedCalendar: finishedCalendar,
	}, nil
}

func (c Client) GetUpdatedItems(syncToken *string) ([]sync.TimestampedItem, error) {
	request := c.srv.Events.List(c.ActiveCalendar)
	if syncToken != nil {
		request = request.SyncToken(*syncToken)
	}
	events, err := request.Do()
	if err != nil {
		return nil, err
	}

	var items []sync.TimestampedItem
	for _, event := range events.Items {
		time, err := time.Parse(time.RFC3339, event.Updated)
		if err != nil {
			slog.Warn("Failed to parse event updated time:", "error", err, "eventID", event.Id)
			continue
		}

		items = append(items, sync.TimestampedItem{
			ExistingItem: sync.ExistingItem{
				ID: event.Id,
				BasicItem: sync.BasicItem{
					Summary:   event.Summary,
					StartDate: time,
				},
			},
			Timestamp: time,
		})
	}
	return items, nil
}

func (c Client) GetServiceName() string {
	return ServiceName
}

func buildAllDayEvent(item sync.BasicItem) *calendar.Event {
	return &calendar.Event{
		Summary: item.Summary,
		Start: &calendar.EventDateTime{
			Date: item.StartDate.Format("2006-01-02"),
		},
		End: &calendar.EventDateTime{
			Date: item.StartDate.AddDate(0, 0, 1).Format("2006-01-02"),
		},
	}
}

func (c Client) CreateItem(item sync.BasicItem) (string, error) {
	event, err := c.srv.Events.Insert(c.ActiveCalendar, buildAllDayEvent(item)).Do()
	if err != nil {
		return "", fmt.Errorf("failed to create calendar event: %w", err)
	}
	if event.Id == "" {
		return "", fmt.Errorf("created calendar event missing id")
	}

	return event.Id, nil
}

func (c Client) UpdateItem(item sync.ExistingItem) error {
	if item.ID == "" {
		return fmt.Errorf("item id is required for update")
	}

	event := buildAllDayEvent(item.BasicItem)
	event.Id = item.ID
	if _, err := c.srv.Events.Update(c.ActiveCalendar, item.ID, event).Do(); err != nil {
		return fmt.Errorf("failed to update calendar event %s: %w", item.ID, err)
	}

	return nil
}

func (c Client) CompleteItem(itemID string) error {
	if itemID == "" {
		return fmt.Errorf("item id is required for completion")
	}

	if _, err := c.srv.Events.Move(c.ActiveCalendar, itemID, c.FinishedCalendar).Do(); err != nil {
		return fmt.Errorf("failed to move calendar event %s to finished calendar: %w", itemID, err)
	}

	return nil
}

func (c Client) DeleteItem(itemID string) error {
	if itemID == "" {
		return fmt.Errorf("item id is required for deletion")
	}

	if err := c.srv.Events.Delete(c.ActiveCalendar, itemID).Do(); err != nil {
		return fmt.Errorf("failed to delete calendar event %s: %w", itemID, err)
	}

	return nil
}
