package calendar

import (
	"net/http"
	"time"

	"github.com/blueputty01/task-sync/internal/sync"
)

const ServiceName = "calendar"

type Client struct {
	httpClient *http.Client
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &Client{
		httpClient: httpClient,
	}
}

func (c Client) GetUpdatedItems(since time.Time) ([]sync.UpdatedItem, error) {
	//TODO implement me
	panic("implement me")
}

func (c Client) GetServiceName() string {
	return ServiceName
}

func (c Client) CreateItem(item sync.BasicItem) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (c Client) UpdateItem(item sync.ExistingItem) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) CompleteItem(itemID string) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) DeleteItem(itemID string) error {
	//TODO implement me
	panic("implement me")
}
