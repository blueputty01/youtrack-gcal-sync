package sync

import "time"

type Synchronizer struct {
	db *DB
}

type ItemsToUpdate struct {
	ID      string
	Updated time.Time
	Summary string
}

type Client interface {
	GetUpdatedItems(since time.Time) ([]ItemsToUpdate, error)
	GetServiceName() string
}

func NewSynchronizer(dataSourceName string) (*Synchronizer, error) {
	db, err := NewDB(dataSourceName)
	if err != nil {
		return nil, err
	}

	return &Synchronizer{db: db}, nil
}

func (s *Synchronizer) Sync(clients []Client) error {
	updates := make(map[string][]ItemsToUpdate)
	for _, client := range clients {
		timestamp, err := s.db.GetLastSyncTimestamp(client.GetServiceName())
		if err != nil {
			return err
		}

		updatedItems, err := client.GetUpdatedItems(timestamp)
		if err != nil {
			return err
		}

		updates[client.GetServiceName()] = updatedItems
	}

	items, err := s.db.GetOverlapItems(updates)
	if err != nil {
		return err
	}

	resolvedItems := make(map[string]ItemsToUpdate)
	for _, serviceItems := range updates {

	}

	return nil
}
