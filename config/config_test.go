package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary .env file for testing
	envContent := `
YOUTRACK_BASE_URL=https://youtrack.example.com
YOUTRACK_PERMANENT_TOKEN=test-token
YOUTRACK_PROJECT_ID=test-project
GOOGLE_CLIENT_ID=test-client-id
GOOGLE_CLIENT_SECRET=test-client-secret
GOOGLE_REDIRECT_URL=https://localhost:8080
`
	tmpfile, err := os.CreateTemp("", ".env")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write([]byte(envContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Temporarily replace the .env file path
	originalEnvFile := "./.env"
	if err := os.Rename(tmpfile.Name(), originalEnvFile); err != nil {
		t.Fatalf("Failed to rename temp file: %v", err)
	}
	defer os.Rename(originalEnvFile, tmpfile.Name()) // move it back

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.YouTrackBaseURL != "https://youtrack.example.com" {
		t.Errorf("expected youtrack base url to be 'https://youtrack.example.com', got %s", cfg.YouTrackBaseURL)
	}
	if cfg.YouTrackPermanentToken != "test-token" {
		t.Errorf("expected youtrack permanent token to be 'test-token', got %s", cfg.YouTrackPermanentToken)
	}
	if cfg.YouTrackProjectID != "test-project" {
		t.Errorf("expected youtrack project id to be 'test-project', got %s", cfg.YouTrackProjectID)
	}
	if cfg.GoogleClientID != "test-client-id" {
		t.Errorf("expected google client id to be 'test-client-id', got %s", cfg.GoogleClientID)
	}
	if cfg.GoogleClientSecret != "test-client-secret" {
		t.Errorf("expected google client secret to be 'test-client-secret', got %s", cfg.GoogleClientSecret)
	}
	if cfg.GoogleRedirectURL != "https://localhost:8080" {
		t.Errorf("expected google redirect url to be 'https://localhost:8080', got %s", cfg.GoogleRedirectURL)
	}
}