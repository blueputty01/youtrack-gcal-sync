package main

import (
	"context"
	"log"
	"os"
	"time"

	"golang.org/x/oauth2"

	"youtrack-calendar-sync/config"
	"youtrack-calendar-sync/googlecalendar"
	"youtrack-calendar-sync/sync"
	"youtrack-calendar-sync/youtrack"
)

const (
	tokenFile    = "data/token.json"
	dbFile       = "data/sync.db"
	syncInterval = 24 * time.Hour // Synchronize every 24 hours
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Google Calendar Setup
	gcalConfig := googlecalendar.GetConfig(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)

	var token *oauth2.Token
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		token, err = googlecalendar.GetTokenFromWeb(gcalConfig)
		if err != nil {
			log.Fatalf("Error getting Google Calendar token from web: %v", err)
		}
		if err := googlecalendar.SaveToken(tokenFile, token); err != nil {
			log.Fatalf("Error saving Google Calendar token: %v", err)
		}
	} else {
		token, err = googlecalendar.LoadToken(tokenFile)
		if err != nil {
			log.Fatalf("Error loading Google Calendar token: %v", err)
		}
	}

	ctx := context.Background()
	gcalClient, err := googlecalendar.NewClient(ctx, token, gcalConfig)
	if err != nil {
		log.Fatalf("Error creating Google Calendar client: %v", err)
	}

	// YouTrack Setup
	ytClient := youtrack.NewClient(cfg.YouTrackBaseURL, cfg.YouTrackPermanentToken)

	// Database Setup
	db, err := sync.NewDB(dbFile)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

	// Synchronizer Setup and Start
	synchronizer := sync.NewSynchronizer(gcalClient, ytClient, db, cfg.YouTrackProjectID, cfg.YouTrackQueryProjectID, cfg.GoogleCalendarId) // "primary" for user's primary calendar

	// Perform an initial sync
	if err := synchronizer.Sync(); err != nil {
		log.Printf("Initial synchronization failed: %v", err)
	}

	// Start periodic sync
	log.Printf("Starting periodic synchronization every %s...", syncInterval)
	synchronizer.StartSyncLoop(syncInterval)
}
