// +build !consulent

package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

func sessionIndexer() *memdb.UUIDFieldIndex {
	return &memdb.UUIDFieldIndex{
		Field: "ID",
	}
}

func (s *Store) collectNodeSessions(sessions memdb.ResultIterator, entMeta *structs.EnterpriseMeta) structs.Sessions {
	// Go over all of the sessions and return them as a slice
	var result structs.Sessions
	for s := sessions.Next(); s != nil; s = sessions.Next() {
		result = append(result, s.(*structs.Session))
	}
	return result
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
	for _, checkID := range session.Checks {
		mapping := &sessionCheck{
			Node:    session.Node,
			CheckID: checkID,
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

func (s *Store) sessionMaxIndex(tx *memdb.Txn, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "sessions")
}
