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

func NewClient( /* parameters for initialization */ ) *Client {
	return &Client{
		// Initialize fields
	}
}

func (c *Client) GetUpdatedItems(since time.Time) ([]sync.UpdatedItem, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Client) CreateItem(item sync.BasicItem) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Client) UpdateItem(item sync.BasicItem) error {
	//TODO implement me
	panic("implement me")
}

func (c *Client) GetServiceName() string {
	return ServiceName
}
