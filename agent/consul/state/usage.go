package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	serviceNamesUsageTable = "service-names"

	tableUsage = "usage"
)

// usageTableSchema returns a new table schema used for tracking various indexes
// for the Raft log.
func usageTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableUsage,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
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

// NodeUsage contains all of the usage data related to nodes
type NodeUsage struct {
	Nodes int
	EnterpriseNodeUsage
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
	serviceNameChanges := make(map[structs.ServiceName]int)
	for _, change := range changes.Changes {
		var delta int
		if change.Created() {
			delta = 1
		} else if change.Deleted() {
			delta = -1
		}

		switch change.Table {
		case tableNodes:
			usageDeltas[change.Table] += delta
			addEnterpriseNodeUsage(usageDeltas, change)

		case tableServices:
			svc := changeObject(change).(*structs.ServiceNode)
			usageDeltas[change.Table] += delta
			addEnterpriseServiceInstanceUsage(usageDeltas, change)

			// Construct a mapping of all of the various service names that were
			// changed, in order to compare it with the finished memdb state.
			// Make sure to account for the fact that services can change their names.
			if serviceNameChanged(change) {
				serviceNameChanges[svc.CompoundServiceName()] += 1
				before := change.Before.(*structs.ServiceNode)
				serviceNameChanges[before.CompoundServiceName()] -= 1
			} else {
				serviceNameChanges[svc.CompoundServiceName()] += delta
			}
		}
	}

	serviceStates, err := updateServiceNameUsage(tx, usageDeltas, serviceNameChanges)
	if err != nil {
		return err
	}
	addEnterpriseServiceUsage(usageDeltas, serviceStates)

	idx := changes.Index
	// This will happen when restoring from a snapshot, just take the max index
	// of the tables we are tracking.
	if idx == 0 {
		// TODO(partitions? namespaces?)
		idx = maxIndexTxn(tx, tableNodes, tableServices)
	}

	return writeUsageDeltas(tx, idx, usageDeltas)
}

func updateServiceNameUsage(tx WriteTxn, usageDeltas map[string]int, serviceNameChanges map[structs.ServiceName]int) (map[structs.ServiceName]uniqueServiceState, error) {
	serviceStates := make(map[structs.ServiceName]uniqueServiceState, len(serviceNameChanges))
	for svc, delta := range serviceNameChanges {
		q := Query{
			Value:          svc.Name,
			EnterpriseMeta: svc.EnterpriseMeta,
		}
		serviceIter, err := tx.Get(tableServices, indexService, q)
		if err != nil {
			return nil, err
		}

		// Count the number of service instances associated with the given service
		// name at the end of this transaction, and compare that with how many were
		// added/removed during the transaction. This allows us to handle a single
		// transaction committing multiple changes related to a single service
		// name.
		var svcCount int
		for service := serviceIter.Next(); service != nil; service = serviceIter.Next() {
			svcCount += 1
		}

		var serviceState uniqueServiceState
		switch {
		case svcCount == 0:
			// If no services exist, we know we deleted the last service
			// instance.
			serviceState = Deleted
			usageDeltas[serviceNamesUsageTable] -= 1
		case svcCount == delta:
			// If the current number of service instances equals the number added,
			// than we know we created a new service name.
			serviceState = Created
			usageDeltas[serviceNamesUsageTable] += 1
		default:
			serviceState = NoChange
		}

		serviceStates[svc] = serviceState
	}

	return serviceStates, nil
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
		u, err := tx.First(tableUsage, indexID, id)
		if err != nil {
			return fmt.Errorf("failed to retrieve existing usage entry: %s", err)
		}

		if u == nil {
			if delta < 0 {
				// Don't return an error here, since we don't want to block updates
				// from happening to the state store. But, set the delta to 0 so that
				// we do not accidentally underflow the uint64 and begin reporting
				// large numbers.
				delta = 0
			}
			err := tx.Insert(tableUsage, &UsageEntry{
				ID:    id,
				Count: delta,
				Index: idx,
			})
			if err != nil {
				return fmt.Errorf("failed to update usage entry: %s", err)
			}
		} else if cur, ok := u.(*UsageEntry); ok {
			updated := cur.Count + delta
			if updated < 0 {
				// Don't return an error here, since we don't want to block updates
				// from happening to the state store. But, set the delta to 0 so that
				// we do not accidentally underflow the uint64 and begin reporting
				// large numbers.
				updated = 0
			}
			err := tx.Insert(tableUsage, &UsageEntry{
				ID:    id,
				Count: updated,
				Index: idx,
			})
			if err != nil {
				return fmt.Errorf("failed to update usage entry: %s", err)
			}
		}
	}
	return nil
}

// NodeUsage returns the latest seen Raft index, a compiled set of node usage
// data, and any errors.
func (s *Store) NodeUsage() (uint64, NodeUsage, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	nodes, err := firstUsageEntry(tx, tableNodes)
	if err != nil {
		return 0, NodeUsage{}, fmt.Errorf("failed nodes lookup: %s", err)
	}

	usage := NodeUsage{
		Nodes: nodes.Count,
	}
	results, err := compileEnterpriseNodeUsage(tx, usage)
	if err != nil {
		return 0, NodeUsage{}, fmt.Errorf("failed nodes lookup: %s", err)
	}

	return nodes.Index, results, nil
}

// ServiceUsage returns the latest seen Raft index, a compiled set of service
// usage data, and any errors.
func (s *Store) ServiceUsage() (uint64, ServiceUsage, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	serviceInstances, err := firstUsageEntry(tx, tableServices)
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
	results, err := compileEnterpriseServiceUsage(tx, usage)
	if err != nil {
		return 0, ServiceUsage{}, fmt.Errorf("failed services lookup: %s", err)
	}

	return serviceInstances.Index, results, nil
}

func firstUsageEntry(tx ReadTxn, id string) (*UsageEntry, error) {
	usage, err := tx.First(tableUsage, indexID, id)
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
