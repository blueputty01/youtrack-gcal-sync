package sync

import (
	"strings"
	"time"
)

type Synchronizer struct {
	db            *DBClient
	Clients       []Client
	mappedClients map[string]Client
}

type BasicItem struct {
	Summary   string
	StartDate time.Time
}

// ExistingItem is an item currently existing in a single service
type ExistingItem struct {
	BasicItem
	ID string
}

// UpdatedItem is an item that has been updated since the last sync
type UpdatedItem struct {
	ExistingItem
	Updated time.Time
}

type Client interface {
	GetUpdatedItems(since time.Time) ([]UpdatedItem, error)
	GetServiceName() string
	// CreateItem creates a new item in the external service and returns its ID.
	CreateItem(item BasicItem) (string, error)
	UpdateItem(item BasicItem) error
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
	syncStartTime := time.Now()
	totalChangedItems := 0
	// map from service name to list of updated items
	updates := make(map[string][]UpdatedItem)
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

	mappedUpdates := make(map[string]map[string]*UpdatedItem)
	for serviceName, items := range updates {
		if _, found := mappedUpdates[serviceName]; !found {
			mappedUpdates[serviceName] = make(map[string]*UpdatedItem)
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

					newId, err := client.CreateItem(BasicItem{
						Summary:   item.Summary,
						StartDate: item.StartDate,
					})
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

				// store services to update and their current (mismatched) states
				toUpdate := make(map[string]*UpdatedItem)
				for serviceName, serviceId := range originalDBItem.ids {
					queuedUpdate, exists := mappedUpdates[serviceName][serviceId]
					if exists {
						toUpdate[serviceName] = queuedUpdate
						delete(mappedUpdates[serviceName], serviceId)
					}
				}

				finalItem := resolveConflict(originalDBItem, toUpdate)
				// extend toUpdate to also include the state of other unqueued services
				for serviceName, serviceId := range originalDBItem.ids {
					if _, exists := toUpdate[serviceName]; !exists {
						toUpdate[serviceName] = &UpdatedItem{
							ExistingItem: ExistingItem{
								BasicItem: BasicItem{
									Summary:   originalDBItem.Summary,
									StartDate: originalDBItem.StartDate,
								},
								ID: serviceId,
							},
							Updated: syncStartTime,
						}
					}
				}

				for serviceName, itemState := range toUpdate {
					needsUpdate := false
					needsUpdate = itemState.Summary != finalItem.Summary
					needsUpdate = itemState.StartDate != finalItem.StartDate || needsUpdate
					if needsUpdate {
						client := s.mappedClients[serviceName]
						err = client.UpdateItem(finalItem)
						if err != nil {
							return err
						}
					}
				}

				originalDBItem.Summary = finalItem.Summary
				originalDBItem.StartDate = finalItem.StartDate
				err = s.db.UpdateItem(serviceName, item.ID, originalDBItem)
				if err != nil {
					return err
				}
			}
		}
	}

	for _, client := range s.Clients {
		err = s.db.UpdateLastSyncTimestamp(client.GetServiceName(), syncStartTime)
	}
	return nil
}

func resolveConflict(originalDBItem *DBItem, toUpdate map[string]*UpdatedItem) BasicItem {
	// now perform the updates
	finalItem := BasicItem{
		Summary:   originalDBItem.Summary,
		StartDate: originalDBItem.StartDate,
	}
	// calculate new date, first the new date will win, then last
	// write will win.
	latestUpdate := 0
	for _, queuedUpdate := range toUpdate {
		if queuedUpdate.StartDate != originalDBItem.StartDate {
			if queuedUpdate.Updated.Unix() > int64(latestUpdate) {
				finalItem.StartDate = queuedUpdate.StartDate
				latestUpdate = int(queuedUpdate.Updated.Unix())
			}
		}
	}

	// concatenation of the new summaries
	newSummaries := make([]string, len(toUpdate))
	for _, queuedUpdate := range toUpdate {
		if queuedUpdate.Summary != originalDBItem.Summary {
			newSummaries = append(newSummaries, queuedUpdate.Summary)
		}
	}

	if len(newSummaries) > 0 {
		finalItem.Summary = strings.Join(newSummaries, " | ")
	}

	return finalItem
}

// getMappedDBItems retrieves the database rows that correspond to the provided updates
// and maps them by service name and item ID.
// Thus, a single database item may be referenced by multiple services.
// Therefore, the return value is a map of service name -> map of item ID -> *DBItem along with
// the actual DBItem's
func (s *Synchronizer) getMappedDBItems(updates map[string][]UpdatedItem) (map[string]map[string]*DBItem, []DBItem, error) {
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
