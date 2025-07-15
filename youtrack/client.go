package youtrack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var ErrNotFound = errors.New("not found")

const (
	apiPath = "/api"
)

// Client wraps the YouTrack HTTP client.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new YouTrack API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) GetBaseURL() string {
	return c.BaseURL
}

// CreateIssue creates a new YouTrack issue.
func (c *Client) CreateIssue(projectID, summary, description string, dueDate *time.Time) (*Issue, error) {
	issue := IssueWrapper{
		YouTrackType: YouTrackType{Type: "Issue"},
		Summary:      summary,
		Description:  description,
		Project:      &Project{YouTrackType: YouTrackType{Type: "Project"}, ID: projectID},
	}

	if dueDate != nil {
		issue.CustomFields = append(issue.CustomFields, CustomField{
			YouTrackType: YouTrackType{Type: "DateIssueCustomField"},
			Name:         "Due Date", // Assuming "Due Date" is the name of your custom field
			Value:        dueDate.UnixMilli(),
		})
	}

	body, err := json.Marshal(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal issue: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s/issues?", c.BaseURL, apiPath), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create issue, status: %s, body: %s", resp.Status, respBody)
	}

	var createdIssue Issue
	if err := json.NewDecoder(resp.Body).Decode(&createdIssue); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &createdIssue, nil
}

// UpdateIssue updates an existing YouTrack issue.
func (c *Client) UpdateIssue(issueID, summary, description string, dueDate *time.Time) error {
	updates := map[string]interface{}{
		"summary":     summary,
		"description": description,
	}

	if dueDate != nil {
		updates["customFields"] = []CustomFieldWrapper{
			{
				YouTrackType: YouTrackType{Type: "DateIssueCustomField"},
				Value:        dueDate.UnixMilli(),
			},
		}
	}

	body, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to marshal updates: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s/issues/%s", c.BaseURL, apiPath, issueID), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	} else if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update issue, status: %s, body: %s", resp.Status, respBody)
	}
	return nil
}

// GetIssueBySummary searches for a YouTrack issue by its summary.
func (c *Client) GetIssueBySummary(projectID, summary string) (*Issue, error) {
	query := url.QueryEscape(fmt.Sprintf("project:%s summary:\"%s\" State: -Resolved", projectID, summary))
	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s/issues?query=%s&fields=id,idReadable,summary,description,project(id,name,shortName),customFields(id,name,value($type,name,value))", c.BaseURL, apiPath, query), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get issue by summary, status: %s, body: %s", resp.Status, respBody)
	}

	var issues []Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(issues) > 0 {
		return &issues[0], nil
	}
	return nil, nil // No issue found
}

// GetUpdatedIssues fetches issues updated since a given time.
func (c *Client) GetUpdatedIssues(projectID string, since time.Time) ([]Issue, error) {
	query := url.QueryEscape(fmt.Sprintf("project:%s updated: %s .. {now}", projectID, since.Format("2006-01-02T15:04:05")))
	url := fmt.Sprintf("%s%s/issues?query=%s&fields=id,idReadable,summary,description,updated,project(id,name,shortName),customFields(id,name,value($type,name,value))", c.BaseURL, apiPath, query)
	fmt.Printf("Fetching updated issues with query: %s\n", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get updated issues, status: %s, body: %s", resp.Status, respBody)
	}

	var issues []Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return issues, nil
}
