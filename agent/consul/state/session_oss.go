// +build !consulent

package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-memdb"
)

func sessionIndexer() *memdb.UUIDFieldIndex {
	return &memdb.UUIDFieldIndex{
		Field: "ID",
	}
}

func nodeSessionsIndexer() *memdb.StringFieldIndex {
	return &memdb.StringFieldIndex{
		Field:     "Node",
		Lowercase: true,
	}
}

func nodeChecksIndexer() *memdb.CompoundIndex {
	return &memdb.CompoundIndex{
		Indexes: []memdb.Indexer{
			&memdb.StringFieldIndex{
				Field:     "Node",
				Lowercase: true,
			},
			&CheckIDIndex{},
		},
	}
}

func (s *Store) sessionDeleteWithSession(tx *memdb.Txn, session *structs.Session, idx uint64) error {
	if err := tx.Delete("sessions", session); err != nil {
		return fmt.Errorf("failed deleting session: %s", err)
	}

	// Update the indexes
	err := tx.Insert("index", &IndexEntry{"sessions", idx})
	if err != nil {
		return fmt.Errorf("failed updating sessions index: %v", err)
	}
	return nil
}

func (s *Store) insertSessionTxn(tx *memdb.Txn, session *structs.Session, idx uint64, updateMax bool) error {
	if err := tx.Insert("sessions", session); err != nil {
		return err
	}

	// Insert the check mappings
	for _, checkID := range session.CheckIDs() {
		mapping := &sessionCheck{
			Node:    session.Node,
			CheckID: structs.CheckID{ID: checkID},
			Session: session.ID,
		}
		if err := tx.Insert("session_checks", mapping); err != nil {
			return fmt.Errorf("failed inserting session check mapping: %s", err)
		}
	}

	// Update the index
	if updateMax {
		if err := indexUpdateMaxTxn(tx, idx, "sessions"); err != nil {
			return fmt.Errorf("failed updating sessions index: %v", err)
		}
	} else {
		err := tx.Insert("index", &IndexEntry{"sessions", idx})
		if err != nil {
			return fmt.Errorf("failed updating sessions index: %v", err)
		}
	}

	return nil
}

func (s *Store) allNodeSessionsTxn(tx *memdb.Txn, node string) (structs.Sessions, error) {
	return s.nodeSessionsTxn(tx, nil, node, nil)
}

func (s *Store) nodeSessionsTxn(tx *memdb.Txn,
	ws memdb.WatchSet, node string, entMeta *structs.EnterpriseMeta) (structs.Sessions, error) {

	sessions, err := tx.Get("sessions", "node", node)
	if err != nil {
		return nil, fmt.Errorf("failed session lookup: %s", err)
	}
	ws.Add(sessions.WatchCh())

	var result structs.Sessions
	for session := sessions.Next(); session != nil; session = sessions.Next() {
		result = append(result, session.(*structs.Session))
	}
	return result, nil
}

func (s *Store) sessionMaxIndex(tx *memdb.Txn, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "sessions")
}

func (s *Store) validateSessionChecksTxn(tx *memdb.Txn, session *structs.Session) error {
	// Go over the session checks and ensure they exist.
	for _, checkID := range session.CheckIDs() {
		check, err := tx.First("checks", "id", session.Node, string(checkID))
		if err != nil {
			return fmt.Errorf("failed check lookup: %s", err)
		}
		if check == nil {
			return fmt.Errorf("Missing check '%s' registration", checkID)
		}

		// Verify that the check is not in critical state
		status := check.(*structs.HealthCheck).Status
		if status == api.HealthCritical {
			return fmt.Errorf("Check '%s' is in %s state", checkID, status)
		}
	}
	return nil
}
