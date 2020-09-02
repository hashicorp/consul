package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
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

type UsageEntry struct {
	ID    string
	Index uint64
	Count int
}

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
			usageDeltas[change.Table] += delta
		}

		addEnterpriseUsage(usageDeltas, change)
	}

	idx := changes.Index
	// This will happen when restoring from a snapshot, just take the max index
	// of the tables we are tracking.
	if idx == 0 {
		idx = maxIndexTxn(tx, "nodes", "services")
	}

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

// ServiceUsage contains all of the usage data related to services
type ServiceUsage struct {
	Services         int
	ServiceInstances int
	EnterpriseServiceUsage
}

// NodeCount returns the latest seen Raft index, a count of the number of nodes
// registered, and any errors.
func (s *Store) NodeCount() (uint64, int, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	usage, err := tx.First("usage", "id", "nodes")
	if err != nil {
		return 0, 0, fmt.Errorf("failed nodes lookup: %s", err)
	}

	// If no nodes have been registered, the usage entry will not exist.
	if usage == nil {
		return 0, 0, nil
	}

	nodeUsage, ok := usage.(*UsageEntry)
	if !ok {
		return 0, 0, fmt.Errorf("failed nodes lookup: type %T is not *UsageEntry", usage)
	}

	return nodeUsage.Index, nodeUsage.Count, nil
}

// ServiceUsage returns the latest seen Raft index, a compiled set of service
// usage data, and any errors.
func (s *Store) ServiceUsage() (uint64, ServiceUsage, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	usage, err := firstUsageEntry(tx, "services")
	if err != nil {
		return 0, ServiceUsage{}, fmt.Errorf("failed services lookup: %s", err)
	}

	results, err := compileServiceUsage(tx, usage.Count)
	if err != nil {
		return 0, ServiceUsage{}, fmt.Errorf("failed services lookup: %s", err)
	}

	return usage.Index, results, nil
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
