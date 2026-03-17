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

// TimestampedItem is an item that has been updated since the last sync
type TimestampedItem struct {
	ExistingItem
	Timestamp time.Time
}

type Client interface {
	GetUpdatedItems(syncInfo *string) ([]TimestampedItem, error)
	GetServiceName() string
	// CreateItem creates a new item in the external service and returns its ID.
	CreateItem(item BasicItem) (string, error)
	UpdateItem(item ExistingItem) error

	CompleteItem(itemID string) error
	DeleteItem(itemID string) error
}

func NewSynchronizer(clients []Client) (*Synchronizer, error) {
	services := make([]string, len(clients))
	for i, client := range clients {
		services[i] = client.GetServiceName()
	}

	db, err := NewDB(services)
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

// Sync syncs all clients and on success updates the last sync timestamp for all clients
func (s *Synchronizer) Sync(successStates map[string]string) error {
	updates, err := s.gatherUpdatesFromClients()
	if err != nil {
		return err
	}
	mappedUpdates := s.buildUpdatesByService(updates)

	mappedDBItems, err := s.getMappedDBItems(updates)
	if err != nil {
		return err
	}

	if err := s.processUpdates(updates, mappedDBItems, mappedUpdates); err != nil {
		return err
	}

	return s.updateSyncTimestamps(successStates)
}

// Fetches items updated in all clients since the last sync
func (s *Synchronizer) gatherUpdatesFromClients() (map[string][]TimestampedItem, error) {
	updates := make(map[string][]TimestampedItem)
	for _, client := range s.Clients {
		syncInfo, err := s.db.GetSyncInfo(client.GetServiceName())
		if err != nil {
			return nil, err
		}

		updatedItems, err := client.GetUpdatedItems(&syncInfo)
		if err != nil {
			return nil, err
		}

		updates[client.GetServiceName()] = updatedItems
	}
	return updates, nil
}

// Creates a nested map for quick lookup of updates by service and item ID.
func (s *Synchronizer) buildUpdatesByService(updates map[string][]TimestampedItem) map[string]map[string]*TimestampedItem {
	mappedUpdates := make(map[string]map[string]*TimestampedItem)
	for serviceName, items := range updates {
		if _, found := mappedUpdates[serviceName]; !found {
			mappedUpdates[serviceName] = make(map[string]*TimestampedItem)
		}
		for i := range items {
			mappedUpdates[serviceName][items[i].ID] = &items[i]
		}
	}
	return mappedUpdates
}

// ConsolidatedUpdate represents a consolidated update ready for propagation
type ConsolidatedUpdate struct {
	dbID          int64
	itemFields    BasicItem
	serviceStates map[string]*TimestampedItem
}

// processUpdates handles all updates in two stages:
// Stage 1: Consolidate all queued updates and resolve conflicts
// Stage 2: Propagate consolidated changes to all services
func (s *Synchronizer) processUpdates(
	updates map[string][]TimestampedItem,
	mappedDBItems map[string]map[string]*DBItem,
	mappedUpdates map[string]map[string]*TimestampedItem,
) error {
	consolidatedUpdates := s.consolidateUpdates(updates, mappedDBItems, mappedUpdates)
	s.resolveMergeConflicts(consolidatedUpdates)

	if err := s.propagateUpdates(consolidatedUpdates); err != nil {
		return err
	}

	return nil
}

// consolidateUpdates performs stage 1: build grouped queued changes.
// It creates one entry per logical item, including new-to-DB items.
func (s *Synchronizer) consolidateUpdates(
	updates map[string][]TimestampedItem,
	mappedDBItems map[string]map[string]*DBItem,
	mappedUpdates map[string]map[string]*TimestampedItem,
) []ConsolidatedUpdate {
	var consolidated []ConsolidatedUpdate
	existingItemsProcessed := make(map[*DBItem]bool)

	for serviceName, items := range updates {
		for _, item := range items {
			originalDBItem, inDB := mappedDBItems[serviceName][item.ID]
			if !inDB {
				itemCopy := item
				consolidated = append(consolidated, ConsolidatedUpdate{
					itemFields: item.BasicItem,
					serviceStates: map[string]*TimestampedItem{
						serviceName: &itemCopy,
					},
					dbID: 0,
				})
				continue
			}
			if existingItemsProcessed[originalDBItem] {
				continue
			}

			queuedStates := s.consolidateQueuedUpdates(originalDBItem, mappedUpdates)
			s.addUnqueuedServiceStates(originalDBItem, queuedStates)
			existingItemsProcessed[originalDBItem] = true

			consolidated = append(consolidated, ConsolidatedUpdate{
				dbID:          originalDBItem.ID,
				itemFields:    BasicItem{Summary: originalDBItem.Summary, StartDate: originalDBItem.StartDate},
				serviceStates: queuedStates,
			})
		}
	}

	return consolidated
}

// resolveMergeConflicts performs stage 2: conflict resolution only.
func (s *Synchronizer) resolveMergeConflicts(consolidated []ConsolidatedUpdate) {
	for i := range consolidated {
		consolidated[i].itemFields = mergeChanges(consolidated[i].itemFields, consolidated[i].serviceStates)
	}
}

// propagateUpdates performs stage 3: executes all consolidated changes
// by creating new items or updating existing ones across all services.
func (s *Synchronizer) propagateUpdates(consolidated []ConsolidatedUpdate) error {
	for _, update := range consolidated {
		dbItem, err := s.propagateChangesToServices(&update)
		if err != nil {
			return err
		}

		if err := s.db.UpdateOrCreateItem(&dbItem); err != nil {
			return err
		}
	}
	return nil
}

// consolidateQueuedUpdates collects all queued updates for an item and removes them from the map.
func (s *Synchronizer) consolidateQueuedUpdates(
	originalDBItem *DBItem,
	mappedUpdates map[string]map[string]*TimestampedItem,
) map[string]*TimestampedItem {
	toUpdate := make(map[string]*TimestampedItem)
	for serviceName, serviceId := range originalDBItem.ids {
		queuedUpdate, exists := mappedUpdates[serviceName][serviceId]
		if exists {
			toUpdate[serviceName] = queuedUpdate
			delete(mappedUpdates[serviceName], serviceId)
		}
	}
	return toUpdate
}

// addUnqueuedServiceStates extends the toUpdate map with the current state of services without queued updates.
func (s *Synchronizer) addUnqueuedServiceStates(
	originalDBItem *DBItem,
	toUpdate map[string]*TimestampedItem,
) {
	for serviceName, idInService := range originalDBItem.ids {
		if _, exists := toUpdate[serviceName]; !exists {
			toUpdate[serviceName] = &TimestampedItem{
				ExistingItem: ExistingItem{
					BasicItem: BasicItem{
						Summary:   originalDBItem.Summary,
						StartDate: originalDBItem.StartDate,
					},
					ID: idInService,
				},
				Timestamp: time.Unix(0, 0), // Set to epoch to indicate no update
			}
		}
	}
}

// propagateChangesToServices updates stale services and creates missing items when needed.
func (s *Synchronizer) propagateChangesToServices(item *ConsolidatedUpdate) (DBItem, error) {
	dbItem := DBItem{
		Summary:   item.itemFields.Summary,
		StartDate: item.itemFields.StartDate,
		ids:       make(map[string]string),
		ID:        item.dbID,
	}
	for serviceName, client := range s.mappedClients {
		existingServiceDefinition, exists := item.serviceStates[serviceName]
		if !exists {
			newID, err := client.CreateItem(item.itemFields)
			if err != nil {
				return dbItem, err
			}
			dbItem.ids[serviceName] = newID
			item.serviceStates[serviceName] = &TimestampedItem{
				ExistingItem: ExistingItem{
					BasicItem: item.itemFields,
					ID:        newID,
				},
				Timestamp: time.Unix(0, 0),
			}
			continue
		}

		if s.itemNeedsUpdate(existingServiceDefinition, item.itemFields) {
			if err := client.UpdateItem(ExistingItem{BasicItem: item.itemFields, ID: existingServiceDefinition.ID}); err != nil {
				return dbItem, err
			}
		}
	}
	return dbItem, nil
}

// Checks if an item's state differs from the final item.
func (s *Synchronizer) itemNeedsUpdate(itemState *TimestampedItem, finalItem BasicItem) bool {
	return itemState.Summary != finalItem.Summary || itemState.StartDate != finalItem.StartDate
}

// updateSyncTimestamps updates the last sync timestamp for all clients.
func (s *Synchronizer) updateSyncTimestamps(updates map[string]string) error {
	for _, client := range s.Clients {
		if err := s.db.UpdateLastSyncInfo(client.GetServiceName(), updates[client.GetServiceName()]); err != nil {
			return err
		}
	}
	return nil
}

// mergeChanges takes the original item fields and a map of service states (including queued updates and unqueued current states) and merges them into a final item state.
func mergeChanges(itemFields BasicItem, toUpdate map[string]*TimestampedItem) BasicItem {
	// now perform the updates
	finalItem := BasicItem{
		Summary:   itemFields.Summary,
		StartDate: itemFields.StartDate,
	}
	// calculate new date, first the new date will win, then last
	// write will win.
	latestUpdate := 0
	for _, queuedUpdate := range toUpdate {
		if queuedUpdate.StartDate != itemFields.StartDate {
			if queuedUpdate.Timestamp.Unix() > int64(latestUpdate) {
				finalItem.StartDate = queuedUpdate.StartDate
				latestUpdate = int(queuedUpdate.Timestamp.Unix())
			}
		}
	}

	// concatenation of the new summaries
	newSummaries := make([]string, len(toUpdate))
	for _, queuedUpdate := range toUpdate {
		if queuedUpdate.Summary != itemFields.Summary {
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
func (s *Synchronizer) getMappedDBItems(updates map[string][]TimestampedItem) (map[string]map[string]*DBItem, error) {
	dbItems, err := s.db.GetItems(updates)
	if err != nil {
		return nil, err
	}

	// map of service name -> map of item ID -> *DBItem
	mappedDBItems := make(map[string]map[string]*DBItem)
	for i := range dbItems {
		item := &dbItems[i]
		for serviceName := range item.ids {
			if _, found := mappedDBItems[serviceName]; !found {
				mappedDBItems[serviceName] = make(map[string]*DBItem)
			}
			mappedDBItems[serviceName][item.ids[serviceName]] = item
		}
	}

	return mappedDBItems, nil
}
