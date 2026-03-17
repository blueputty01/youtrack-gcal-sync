package main

import (
	"log/slog"
	"os"

	"github.com/blueputty01/task-sync/internal/calendar"
	"github.com/blueputty01/task-sync/internal/projects"
	"github.com/blueputty01/task-sync/internal/sync"
)

func main() {
	clients := []sync.Client{
		constructYoutrackClient(),
		constructGoogleCalendarClient(),
	}
	synchronizer, err := sync.NewSynchronizer(clients)
	if err != nil {
		slog.Error("Failed to create synchronizer:", "DBClient error", err)
	}

	slog.Info("Starting synchronization process")
	err = synchronizer.Sync()
	if err != nil {
		slog.Error("Synchronization failed:", "Sync error", err)
	}
}

func constructYoutrackClient() *projects.Client {
	token, found := os.LookupEnv("YOUTRACK_TOKEN")
	if !found {
		slog.Error("YOUTRACK_TOKEN environment variable not set")
	}

	url, found := os.LookupEnv("YOUTRACK_URL")
	if !found {
		slog.Error("YOUTRACK_URL environment variable not set")
	}
	return projects.NewClient(url, token, []string{"General", "School"}, nil)
}

func constructGoogleCalendarClient() *calendar.Client {
	client, err := calendar.NewClient("primary", "secondary")
	if err != nil {
		slog.Error("Failed to create Google Calendar client:", "error", err)
	}
	return client
}
