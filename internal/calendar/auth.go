package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/blueputty01/task-sync/internal/sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(serviceName string, config *oauth2.Config) (*http.Client, error) {
	db, err := sync.NewDBConstClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create DBConstClient: %w", err)
	}

	rawTok, err := db.GetConst(serviceName + "_token")
	tok := &oauth2.Token{}
	err = json.Unmarshal([]byte(rawTok), tok)
	if err != nil {
		tok = getTokenFromWeb(config)
		rawTokBytes, err := json.Marshal(tok)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal token: %w", err)
		}
		db.SetConst(serviceName+"_token", string(rawTokBytes))
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func GetGoogleCalendarService() (*calendar.Service, error) {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client secret file to config: %v", err)
	}
	client, err := getClient("google_calendar", config)
	if err != nil {
		return nil, fmt.Errorf("Unable to get Google Calendar client: %v", err)
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Calendar client: %v", err)
	}

	return srv, nil
}
