package sync

import "time"

type Synchronizer struct {
	db *DB
}

type SyncItem struct {
	ID      string
	Updated time.Time
	Summary string
}

type SyncClient interface {
	GetUpdatedItems(since time.Time) ([]SyncItem, error)
	GetServiceName() string
}

func NewSynchronizer(dataSourceName string) (*Synchronizer, error) {
	db, err := NewDB(dataSourceName)
	if err != nil {
		return nil, err
	}

	return &Synchronizer{db: db}, nil
}

func (s *Synchronizer) Sync(clients []SyncClient) error {
	for _, client := range clients {
		timestamp, err := s.db.GetLastSyncTimestamp(client.GetServiceName())
		if err != nil {
			return err
		}

		_, err = client.GetUpdatedItems(timestamp)
		if err != nil {
			return err
		}
	}

	return nil
}
