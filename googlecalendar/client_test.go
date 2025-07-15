package googlecalendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func TestGetConfig(t *testing.T) {
	config := GetConfig("test-id", "test-secret", "test-url")
	if config.ClientID != "test-id" {
		t.Errorf("expected client id to be 'test-id', got %s", config.ClientID)
	}
	if config.ClientSecret != "test-secret" {
		t.Errorf("expected client secret to be 'test-secret', got %s", config.ClientSecret)
	}
	if config.RedirectURL != "test-url" {
		t.Errorf("expected redirect url to be 'test-url', got %s", config.RedirectURL)
	}
}

func TestSaveLoadToken(t *testing.T) {
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now(),
	}

	tmpfile, err := os.CreateTemp("", "token.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if err := SaveToken(tmpfile.Name(), token); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}

	loadedToken, err := LoadToken(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}

	if loadedToken.AccessToken != token.AccessToken {
		t.Errorf("expected access token to be '%s', got '%s'", token.AccessToken, loadedToken.AccessToken)
	}
}

func TestNewClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "{}")
	}))
	defer ts.Close()

	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  ts.URL,
			TokenURL: ts.URL,
		},
	}
	token := &oauth2.Token{AccessToken: "test"}

	_, err := NewClient(context.Background(), token, config)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
}

func TestFetchEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/calendars/primary/events" {
			t.Errorf("Expected to request '/calendars/primary/events', got: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&calendar.Events{
			Items: []*calendar.Event{
				{Id: "1", Summary: "Event 1"},
			},
			NextSyncToken: "new-sync-token",
		})
	}))
	defer server.Close()

	ctx := context.Background()
	srv, err := calendar.NewService(ctx, option.WithEndpoint(server.URL), option.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("Unable to create calendar service: %v", err)
	}

	c := &Client{srv: srv}
	events, syncToken, err := c.FetchEvents("primary", "")
	if err != nil {
		t.Fatalf("FetchEvents() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Summary != "Event 1" {
		t.Errorf("expected event summary to be 'Event 1', got '%s'", events[0].Summary)
	}
	if syncToken != "new-sync-token" {
		t.Errorf("expected sync token to be 'new-sync-token', got '%s'", syncToken)
	}
}

func TestCreateEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected 'POST' request, got '%s'", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&calendar.Event{
			Id:      "new-event",
			Summary: "New Event",
		})
	}))
	defer server.Close()

	ctx := context.Background()
	srv, err := calendar.NewService(ctx, option.WithEndpoint(server.URL), option.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("Unable to create calendar service: %v", err)
	}

	c := &Client{srv: srv}
	event, err := c.CreateEvent("primary", "New Event", "Description", time.Now(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateEvent() error = %v", err)
	}

	if event.Summary != "New Event" {
		t.Errorf("expected event summary to be 'New Event', got '%s'", event.Summary)
	}
}

func TestUpdateEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected 'PUT' request, got '%s'", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&calendar.Event{
			Id:      "updated-event",
			Summary: "Updated Event",
		})
	}))
	defer server.Close()

	ctx := context.Background()
	srv, err := calendar.NewService(ctx, option.WithEndpoint(server.URL), option.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("Unable to create calendar service: %v", err)
	}

	c := &Client{srv: srv}
	event, err := c.UpdateEvent("primary", "event-id", "Updated Event", "Description", time.Now(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("UpdateEvent() error = %v", err)
	}

	if event.Summary != "Updated Event" {
		t.Errorf("expected event summary to be 'Updated Event', got '%s'", event.Summary)
	}
}

func TestParseDateTime(t *testing.T) {
	testCases := []struct {
		name     string
		input    *calendar.EventDateTime
		expected time.Time
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: time.Time{},
		},
		{
			name: "DateTime",
			input: &calendar.EventDateTime{
				DateTime: "2024-01-01T10:00:00Z",
			},
			expected: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		},
		{
			name: "Date",
			input: &calendar.EventDateTime{
				Date: "2024-01-01",
			},
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseDateTime(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}