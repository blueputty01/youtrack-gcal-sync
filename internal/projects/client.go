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
	Projects   []string
	httpClient *http.Client
}

const ServiceName = "youtrack"

func (c *Client) GetUpdatedItems(rawTime *string) ([]sync.TimestampedItem, error) {
	since, err := time.Parse(time.RFC3339, *rawTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time: %w", err)
	}
	items := make([]sync.TimestampedItem, 10)
	for _, projectID := range c.Projects {
		items, err := c.getUpdatedItems("0-0", since)
		if err != nil {
			return nil, fmt.Errorf("failed to get updated items: for project %s %w", projectID, err)
		}
		items = append(items, items...)
	}
	return items, nil
}

func (c *Client) GetServiceName() string {
	return ServiceName
}

func NewClient(baseURL, apiToken string, projects []string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &Client{
		BaseURL:    strings.Trim(baseURL, "/"),
		APIToken:   apiToken,
		Projects:   projects,
		httpClient: httpClient,
	}
}

func (c *Client) getUpdatedItems(projectID string, since time.Time) ([]sync.TimestampedItem, error) {
	issues, err := c.getUpdatedIssues(projectID, since)
	if err != nil {
		return nil, err
	}

	var items []sync.TimestampedItem
	for _, issue := range issues {
		updatedTime := time.Unix(issue.Updated/1000, 0)
		item := sync.TimestampedItem{
			ExistingItem: sync.ExistingItem{
				ID: issue.ID,
				BasicItem: sync.BasicItem{
					StartDate: updatedTime,
					Summary:   issue.Summary,
				},
			},
			Timestamp: updatedTime,
		}
		items = append(items, item)
	}

	return items, nil
}

// getUpdatedIssues fetches issues updated since the specified time for a given project.
// works with internal data structures
func (c *Client) getUpdatedIssues(projectID string, since time.Time) ([]Issue, error) {
	query := fmt.Sprintf("project:%s updated: {%s} .. {now}", projectID, since.Format("2006-01-02T15:04:05"))
	requestUrl := fmt.Sprintf("%s/api/issues?query=%s&fields=id,idReadable,summary,description,updated,project(id,name,shortName),customFields(id,name,value($type,name,value))", c.BaseURL, url.QueryEscape(query))
	res, err := c.doQuery("GET", requestUrl, nil)

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

func (c *Client) doQuery(method string, requestUrl string, body io.Reader) (*http.Response, error) {
	fmt.Printf("Fetching updated issues with query: %s\n", requestUrl)
	req, err := http.NewRequest(method, requestUrl, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

func (c *Client) CreateItem(item sync.BasicItem) (string, error) {
	return c.updateItem(
		&sync.ExistingItem{
			BasicItem: item,
			ID:        "",
		},
	)
}

// UpdateItem implements https://www.jetbrains.com/help/youtrack/devportal/operations-api-issues.html#update-Issue-method
func (c *Client) UpdateItem(item sync.ExistingItem) error {
	if item.ID == "" {
		return fmt.Errorf("item id is required for update")
	}
	_, err := c.updateItem(&item)
	return err
}

// updateItem updates an existing item in YouTrack or creates a new one if it doesn't exist.
func (c *Client) updateItem(item *sync.ExistingItem) (string, error) {
	if item == nil {
		return "", fmt.Errorf("item is nil")
	}

	endpoint := (func() string {
		base := c.BaseURL + "/api/issues"
		if item.ID == "" {
			return fmt.Sprintf("%s?fields=id", base)
		}
		return fmt.Sprintf("%s/%s?fields=id", base, url.PathEscape(item.ID))
	})()

	var body io.Reader
	payload := map[string]any{
		"summary": item.Summary,
	}
	// TODO this doesn't even handle dates!

	buf, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}
	body = bytes.NewReader(buf)

	res, err := c.doQuery("POST", endpoint, body)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}

	bodyBytes, _ := io.ReadAll(res.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(res.Body)

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status code %d: %s", res.StatusCode, string(bodyBytes))
	}

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

func (c *Client) CompleteItem(itemID string) error {
	//TODO implement me
	panic("implement me")
}

func (c *Client) DeleteItem(itemID string) error {
	//TODO implement me
	panic("implement me")
}
