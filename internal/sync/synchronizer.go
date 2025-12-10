package sync

import (
	"time"
)

type Synchronizer struct {
	db            *DBClient
	Clients       []Client
	mappedClients map[string]Client
}

type ItemsToUpdate struct {
	ID      string
	Updated time.Time

	Summary   string
	StartDate time.Time
}

type Client interface {
	GetUpdatedItems(since time.Time) ([]ItemsToUpdate, error)
	GetServiceName() string
	// CreateItem creates a new item in the external service and returns its ID.
	CreateItem(item *ItemsToUpdate) (string, error)
	UpdateItem(item *ItemsToUpdate) error
}

func NewSynchronizer(dataSourceName string, clients []Client) (*Synchronizer, error) {
	services := make([]string, len(clients))
	for i, client := range clients {
		services[i] = client.GetServiceName()
	}

	db, err := NewDB(dataSourceName, services)
	if err != nil {
		return nil, err
	}

	synchronizer := &Synchronizer{db: db, Clients: clients}
	synchronizer.mappedClients = make(map[string]Client)
	for _, client := range clients {
		synchronizer.mappedClients[client.GetServiceName()] = client
	}

	return synchronizer, nil
}

func (s *Synchronizer) Sync() error {
	totalChangedItems := 0
	// map from service name to list of updated items
	updates := make(map[string][]ItemsToUpdate)
	for _, client := range s.Clients {
		timestamp, err := s.db.GetLastSyncTimestamp(client.GetServiceName())
		if err != nil {
			return err
		}

		updatedItems, err := client.GetUpdatedItems(timestamp)

		if err != nil {
			return err
		}

		updates[client.GetServiceName()] = updatedItems
		totalChangedItems += len(updatedItems)
	}

	mappedDBItems, _, err := s.getMappedDBItems(updates)
	if err != nil {
		return err
	}

	mappedUpdates := make(map[string]map[string]*ItemsToUpdate)
	for serviceName, items := range updates {
		if _, found := mappedUpdates[serviceName]; !found {
			mappedUpdates[serviceName] = make(map[string]*ItemsToUpdate)
		}
		for i := range items {
			mappedUpdates[serviceName][items[i].ID] = &items[i]
		}
	}

	for serviceName, items := range updates {
		for _, item := range items {
			originalDBItem, inDB := mappedDBItems[serviceName][item.ID]
			if !inDB {
				for otherServiceName, client := range s.mappedClients {
					if serviceName == otherServiceName {
						continue
					}

					newId, err := client.CreateItem(&item)
					if err != nil {
						return err
					}
					originalDBItem.ids[otherServiceName] = newId
				}
				originalDBItem.Summary = item.Summary
				originalDBItem.StartDate = item.StartDate
				originalDBItem.ids[serviceName] = item.ID
				err = s.db.InsertItem(originalDBItem)
				if err != nil {
					return err
				}
			} else {
				// check if other services have also queued updates for this item
				// delete the other updates to avoid redundant updates
				// thus also guaranteeing that the current item has not been processed

				// store unique summary values and the service where those values originated
				summary := make(map[string][]string)
				summary[originalDBItem.Summary] = make([]string, len(originalDBItem.ids))
				date := make(map[time.Time][]string)
				date[originalDBItem.StartDate] = make([]string, len(originalDBItem.ids))

				for serviceName, serviceId := range originalDBItem.ids {
					queuedUpdate, exists := mappedUpdates[serviceName][serviceId]
					if exists {
						summary[queuedUpdate.Summary] = append(summary[queuedUpdate.Summary], serviceName)
						date[queuedUpdate.StartDate] = append(date[queuedUpdate.StartDate], serviceName)
						delete(mappedUpdates[serviceName], serviceId)
					}
				}

			}
		}
	}

	return nil
}

// getMappedDBItems retrieves the database rows that correspond to the provided updates
// and maps them by service name and item ID.
// Thus, a single database item may be referenced by multiple services.
// Therefore, the return value is a map of service name -> map of item ID -> *DBItem along with
// the actual DBItem's
func (s *Synchronizer) getMappedDBItems(updates map[string][]ItemsToUpdate) (map[string]map[string]*DBItem, []DBItem, error) {
	dbItems, err := s.db.GetItems(updates)
	if err != nil {
		return nil, nil, err
	}

	// map of service name -> map of item ID -> *DBItem
	mappedDBItems := make(map[string]map[string]*DBItem)
	for _, item := range dbItems {
		for serviceName := range item.ids {
			if _, found := mappedDBItems[serviceName]; !found {
				mappedDBItems[serviceName] = make(map[string]*DBItem)
			}
			mappedDBItems[serviceName][item.ids[serviceName]] = &item
		}
	}

	return mappedDBItems, dbItems, nil
}
