package projects

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/blueputty01/task-sync/internal/sync"
)

type Client struct {
	BaseURL    string
	APIToken   string
	httpClient *http.Client
}

const ServiceName = "youtrack"

func (c *Client) GetUpdatedItems(since time.Time) ([]sync.ItemsToUpdate, error) {
	return nil, nil
}

func (c *Client) GetServiceName() string {
	return ServiceName
}

func NewClient(baseURL, apiToken string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &Client{
		BaseURL:    strings.Trim(baseURL, "/"),
		APIToken:   apiToken,
		httpClient: httpClient,
	}
}

func (c *Client) getUpdatedItems(projectID string, since time.Time) ([]sync.ItemsToUpdate, error) {
	issues, err := c.getUpdatedIssues(projectID, since)
	if err != nil {
		return nil, err
	}

	var items []sync.ItemsToUpdate
	for _, issue := range issues {
		updatedTime := time.Unix(issue.Updated/1000, 0)
		item := sync.ItemsToUpdate{
			ID:      issue.ID,
			Updated: updatedTime,
			Summary: issue.Summary,
		}
		items = append(items, item)
	}

	return items, nil
}

func (c *Client) getUpdatedIssues(projectID string, since time.Time) ([]Issue, error) {
	query := fmt.Sprintf("project:%s updated: %s .. {now}", projectID, since.Format("2006-01-02T15:04:05"))
	res, err := c.doQuery(query)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated issues: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}(res.Body)

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", res.StatusCode, string(bodyBytes))
	}

	var issues []Issue
	if err := json.NewDecoder(res.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return issues, nil
}

func (c *Client) doQuery(query string) (*http.Response, error) {
	requestUrl := fmt.Sprintf("%s/api/issues?query=%s&fields=id,idReadable,summary,description,updated,project(id,name,shortName),customFields(id,name,value($type,name,value))", c.BaseURL, url.QueryEscape(query))
	fmt.Printf("Fetching updated issues with query: %s\n", requestUrl)
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// consider combining with updateitem

func (c *Client) CreateItem(item *sync.ItemsToUpdate) (string, error) {
	if item == nil {
		return "", fmt.Errorf("item is nil")
	}

	endpoint := fmt.Sprintf("%s/api/issues?fields=id", c.BaseURL)
	res, err := c.doJSONRequest(http.MethodPost, endpoint, map[string]any{
		"summary": item.Summary,
	}, http.StatusOK, http.StatusCreated)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("failed to decode issue creation response: %w", err)
	}
	if created.ID == "" {
		return "", fmt.Errorf("issue creation response missing id")
	}

	return created.ID, nil
}

// UpdateItem impelements https://www.jetbrains.com/help/youtrack/devportal/operations-api-issues.html#update-Issue-method
func (c *Client) UpdateItem(item *sync.ItemsToUpdate) error {
	if item == nil {
		return fmt.Errorf("item is nil")
	}
	if item.ID == "" {
		return fmt.Errorf("item id is required for update")
	}

	endpoint := fmt.Sprintf("%s/api/issues/%s?fields=id", c.BaseURL, url.PathEscape(item.ID))
	res, err := c.doJSONRequest(http.MethodPost, endpoint, map[string]any{
		"summary": item.Summary,
	}, http.StatusOK)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return nil
}

func (c *Client) doJSONRequest(method, endpoint string, payload any, expectedStatus ...int) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	for _, code := range expectedStatus {
		if res.StatusCode == code {
			return res, nil
		}
	}

	bodyBytes, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return nil, fmt.Errorf("unexpected status code %d: %s", res.StatusCode, string(bodyBytes))
}
