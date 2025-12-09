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

func (c *Client) GetUpdatedItems(since time.Time) ([]sync.ItemsToUpdate, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Client) CreateItem(item *sync.ItemsToUpdate) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Client) UpdateItem(item *sync.ItemsToUpdate) error {
	//TODO implement me
	panic("implement me")
}

func (c *Client) GetServiceName() string {
	return ServiceName
}
