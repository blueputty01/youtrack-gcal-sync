package googlecalendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GetConfig returns an OAuth2 config for Google Calendar API.
func GetConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/calendar.events"},
		Endpoint:     google.Endpoint,
	}
}

// GetTokenFromWeb uses a web flow to retrieve a token.
func GetTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser: \n%v\n", authURL)

	var authCode string
	fmt.Print("Enter authorization code: ")
	_, err := fmt.Scan(&authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return token, nil
}

// SaveToken saves a token to a file.
func SaveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// LoadToken loads a token from a file.
func LoadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// GetClient returns an HTTP client with the given token.
func GetClient(config *oauth2.Config, token *oauth2.Token) *http.Client {
	return config.Client(context.Background(), token)
}
