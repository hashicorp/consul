package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

const (
	serviceNamesUsageTable = "service-names"
)

// usageTableSchema returns a new table schema used for tracking various indexes
// for the Raft log.
func usageTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "usage",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ID",
					Lowercase: true,
				},
			},
		},
	}
}

func init() {
	registerSchema(usageTableSchema)
}

// UsageEntry represents a count of some arbitrary identifier within the
// state store, along with the last seen index.
type UsageEntry struct {
	ID    string
	Index uint64
	Count int
}

// ServiceUsage contains all of the usage data related to services
type ServiceUsage struct {
	Services         int
	ServiceInstances int
	EnterpriseServiceUsage
}

type uniqueServiceState int

const (
	NoChange uniqueServiceState = 0
	Deleted  uniqueServiceState = 1
	Created  uniqueServiceState = 2
)

// updateUsage takes a set of memdb changes and computes a delta for specific
// usage metrics that we track.
func updateUsage(tx WriteTxn, changes Changes) error {
	usageDeltas := make(map[string]int)
	for _, change := range changes.Changes {
		var delta int
		if change.Created() {
			delta = 1
		} else if change.Deleted() {
			delta = -1
		}

		switch change.Table {
		case "nodes":
			usageDeltas[change.Table] += delta
		case "services":
			svc := changeObject(change).(*structs.ServiceNode)
			usageDeltas[change.Table] += delta
			serviceIter, err := getWithTxn(tx, servicesTableName, "service", svc.ServiceName, &svc.EnterpriseMeta)
			if err != nil {
				return err
			}

			var serviceState uniqueServiceState
			if serviceIter.Next() == nil {
				// If no services exist, we know we deleted the last service
				// instance.
				serviceState = Deleted
				usageDeltas[serviceNamesUsageTable] -= 1
			} else if serviceIter.Next() == nil {
				// If a second call to Next() returns nil, we know only a single
				// instance exists. If, in addition, a new service name has been
				// registered, either via creating a new service instance or via
				// renaming an existing service, than we update our service count.
				//
				// We only care about two cases here:
				// 1. A new service instance has been created with a unique name
				// 2. An existing service instance has been updated with a new unique name
				//
				// These are the only ways a new unique service can be created. The
				// other valid cases here: an update that does not change the service
				// name, and a deletion, both do not impact the count of unique service
				// names in the system.

				if change.Created() {
					// Given a single existing service instance of the service: If a
					// service has just been created, then we know this is a new unique
					// service.
					serviceState = Created
					usageDeltas[serviceNamesUsageTable] += 1
				} else if serviceNameChanged(change) {
					// Given a single existing service instance of the service: If a
					// service has been updated with a new service name, then we know
					// this is a new unique service.
					serviceState = Created
					usageDeltas[serviceNamesUsageTable] += 1

					// Check whether the previous name was deleted in this rename, this
					// is a special case of renaming a service which does not result in
					// changing the count of unique service names.
					before := change.Before.(*structs.ServiceNode)
					beforeSvc, err := firstWithTxn(tx, servicesTableName, "service", before.ServiceName, &before.EnterpriseMeta)
					if err != nil {
						return err
					}
					if beforeSvc == nil {
						usageDeltas[serviceNamesUsageTable] -= 1
						// set serviceState to NoChange since we have both gained and lost a
						// service, cancelling each other out
						serviceState = NoChange
					}
				}
			}
			addEnterpriseServiceUsage(usageDeltas, change, serviceState)
		}
	}

	idx := changes.Index
	// This will happen when restoring from a snapshot, just take the max index
	// of the tables we are tracking.
	if idx == 0 {
		idx = maxIndexTxn(tx, "nodes", servicesTableName)
	}

	return writeUsageDeltas(tx, idx, usageDeltas)
}

// serviceNameChanged returns a boolean that indicates whether the
// provided change resulted in an update to the service's service name.
func serviceNameChanged(change memdb.Change) bool {
	if change.Updated() {
		before := change.Before.(*structs.ServiceNode)
		after := change.After.(*structs.ServiceNode)
		return before.ServiceName != after.ServiceName
	}

	return false
}

// writeUsageDeltas will take in a map of IDs to deltas and update each
// entry accordingly, checking for integer underflow. The index that is
// passed in will be recorded on the entry as well.
func writeUsageDeltas(tx WriteTxn, idx uint64, usageDeltas map[string]int) error {
	for id, delta := range usageDeltas {
		u, err := tx.First("usage", "id", id)
		if err != nil {
			return fmt.Errorf("failed to retrieve existing usage entry: %s", err)
		}

		if u == nil {
			if delta < 0 {
				return fmt.Errorf("failed to insert usage entry for %q: delta will cause a negative count", id)
			}
			err := tx.Insert("usage", &UsageEntry{
				ID:    id,
				Count: delta,
				Index: idx,
			})
			if err != nil {
				return fmt.Errorf("failed to update usage entry: %s", err)
			}
		} else if cur, ok := u.(*UsageEntry); ok {
			if cur.Count+delta < 0 {
				return fmt.Errorf("failed to insert usage entry for %q: delta will cause a negative count", id)
			}
			err := tx.Insert("usage", &UsageEntry{
				ID:    id,
				Count: cur.Count + delta,
				Index: idx,
			})
			if err != nil {
				return fmt.Errorf("failed to update usage entry: %s", err)
			}
		}
	}
	return nil
}

// NodeCount returns the latest seen Raft index, a count of the number of nodes
// registered, and any errors.
func (s *Store) NodeCount() (uint64, int, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	nodeUsage, err := firstUsageEntry(tx, "nodes")
	if err != nil {
		return 0, 0, fmt.Errorf("failed nodes lookup: %s", err)
	}
	return nodeUsage.Index, nodeUsage.Count, nil
}

// ServiceUsage returns the latest seen Raft index, a compiled set of service
// usage data, and any errors.
func (s *Store) ServiceUsage() (uint64, ServiceUsage, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	serviceInstances, err := firstUsageEntry(tx, servicesTableName)
	if err != nil {
		return 0, ServiceUsage{}, fmt.Errorf("failed services lookup: %s", err)
	}

	services, err := firstUsageEntry(tx, serviceNamesUsageTable)
	if err != nil {
		return 0, ServiceUsage{}, fmt.Errorf("failed services lookup: %s", err)
	}

	usage := ServiceUsage{
		ServiceInstances: serviceInstances.Count,
		Services:         services.Count,
	}
	results, err := compileEnterpriseUsage(tx, usage)
	if err != nil {
		return 0, ServiceUsage{}, fmt.Errorf("failed services lookup: %s", err)
	}

	return serviceInstances.Index, results, nil
}

func firstUsageEntry(tx ReadTxn, id string) (*UsageEntry, error) {
	usage, err := tx.First("usage", "id", id)
	if err != nil {
		return nil, err
	}

	// If no elements have been inserted, the usage entry will not exist. We
	// return a valid value so that can be certain the return value is not nil
	// when no error has occurred.
	if usage == nil {
		return &UsageEntry{ID: id, Count: 0}, nil
	}

	realUsage, ok := usage.(*UsageEntry)
	if !ok {
		return nil, fmt.Errorf("failed usage lookup: type %T is not *UsageEntry", usage)
	}

	return realUsage, nil
}
