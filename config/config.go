package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

type Config struct {
	YouTrackBaseURL        string
	YouTrackPermanentToken string
	YouTrackProjectID      string
	YouTrackQueryProjectID string
	GoogleClientID         string
	GoogleClientSecret     string
	GoogleRedirectURL      string
	GoogleCalendarId       string
}

func SetENV() {
	// Open the .env file
	envFile, err := os.Open("./.env")
	// check for errors
	if err != nil {
		log.Fatalln(err)
	}
	//	defer closing the file until the function exits
	defer envFile.Close()

	// create a new scanner to read each row
	scanner := bufio.NewScanner(envFile)

	// loop through each row of the .env file
	for scanner.Scan() {
		// split the text on the equal sign to get the key and value
		line := scanner.Text()
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		envVar := strings.SplitN(line, "=", 2)
		if len(envVar) < 2 {
			continue
		}
		envVar[0] = strings.TrimSpace(envVar[0])
		envVar[1] = strings.TrimSpace(envVar[1])
		envVar[1] = strings.Trim(envVar[1], "\"")
		//	os.Setenv takes in a key and a value which are both strings

		os.Setenv(envVar[0], envVar[1])
		// fmt.Fprint(os.Stdout, "Setting environment variable: ", envVar[0], "=", envVar[1], "\n")
	}
	// check for errors with scanner.Scan
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func LoadConfig() (*Config, error) {
	SetENV()

	cfg := &Config{
		YouTrackBaseURL:        os.Getenv("YOUTRACK_BASE_URL"),
		YouTrackPermanentToken: os.Getenv("YOUTRACK_PERMANENT_TOKEN"),
		YouTrackProjectID:      os.Getenv("YOUTRACK_PROJECT_ID"),
		YouTrackQueryProjectID: os.Getenv("YOUTRACK_QUERY_PROJECT_ID"),
		GoogleClientID:         os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:     os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:      os.Getenv("GOOGLE_REDIRECT_URL"),
		GoogleCalendarId:       os.Getenv("GOOGLE_CALENDAR_ID"),
	}

	if cfg.YouTrackBaseURL == "" {
		return nil, fmt.Errorf("YOUTRACK_BASE_URL not set")
	}
	if cfg.YouTrackPermanentToken == "" {
		return nil, fmt.Errorf("YOUTRACK_PERMANENT_TOKEN not set")
	}
	if cfg.YouTrackProjectID == "" {
		return nil, fmt.Errorf("YOUTRACK_PROJECT_ID not set")
	}
	if cfg.YouTrackQueryProjectID == "" {
		cfg.YouTrackQueryProjectID = cfg.YouTrackProjectID
	}
	if cfg.GoogleClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID not set")
	}
	if cfg.GoogleClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET not set")
	}
	if cfg.GoogleRedirectURL == "" {
		return nil, fmt.Errorf("GOOGLE_REDIRECT_URL not set")
	}

	return cfg, nil
}
