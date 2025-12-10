package main

import (
	"github.com/blueputty01/task-sync/internal/calendar"
	"github.com/blueputty01/task-sync/internal/projects"
	"github.com/blueputty01/task-sync/internal/sync"
	"log/slog"
	"os"
)

func main() {
	clients := []sync.Client{
		constructYoutrackClient(),
		constructGoogleCalendarClient(),
	}
	synchronizer, err := sync.NewSynchronizer("sync.db", clients)
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
	return calendar.NewClient(nil)
}
