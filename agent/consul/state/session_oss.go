//go:build !consulent
// +build !consulent

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func sessionIndexer() indexerSingleWithPrefix {
	return indexerSingleWithPrefix{
		readIndex:   readIndex(indexFromQuery),
		writeIndex:  writeIndex(indexFromSession),
		prefixIndex: prefixIndex(prefixIndexFromQuery),
	}
}

func nodeSessionsIndexer() indexerSingle {
	return indexerSingle{
		readIndex:  readIndex(indexFromIDValueLowerCase),
		writeIndex: writeIndex(indexNodeFromSession),
	}
}

func idCheckIndexer() indexerSingle {
	return indexerSingle{
		readIndex:  indexFromNodeCheckIDSession,
		writeIndex: indexFromNodeCheckIDSession,
	}
}

func sessionCheckIndexer() indexerSingle {
	return indexerSingle{
		readIndex:  indexFromQuery,
		writeIndex: indexSessionCheckFromSession,
	}
}

func nodeChecksIndexer() indexerSingle {
	return indexerSingle{
		readIndex:  indexFromMultiValueID,
		writeIndex: indexFromNodeCheckID,
	}
}

// indexFromNodeCheckID creates an index key from a sessionCheck structure
func indexFromNodeCheckID(raw interface{}) ([]byte, error) {
	e, ok := raw.(*sessionCheck)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T, does not implement *structs.Session", raw)
	}
	var b indexBuilder
	v := strings.ToLower(e.Node)
	if v == "" {
		return nil, errMissingValueForIndex
	}
	b.String(v)

	v = strings.ToLower(string(e.CheckID.ID))
	if v == "" {
		return nil, errMissingValueForIndex
	}
	b.String(v)

	return b.Bytes(), nil
}

func sessionDeleteWithSession(tx WriteTxn, session *structs.Session, idx uint64) error {
	if err := tx.Delete(tableSessions, session); err != nil {
		return fmt.Errorf("failed deleting session: %s", err)
	}

	// Update the indexes
	err := tx.Insert(tableIndex, &IndexEntry{"sessions", idx})
	if err != nil {
		return fmt.Errorf("failed updating sessions index: %v", err)
	}
	return nil
}

func insertSessionTxn(tx WriteTxn, session *structs.Session, idx uint64, updateMax bool, _ bool) error {
	if err := tx.Insert(tableSessions, session); err != nil {
		return err
	}

	// Insert the check mappings
	for _, checkID := range session.CheckIDs() {
		mapping := &sessionCheck{
			Node:    session.Node,
			CheckID: structs.CheckID{ID: checkID},
			Session: session.ID,
		}
		if err := tx.Insert(tableSessionChecks, mapping); err != nil {
			return fmt.Errorf("failed inserting session check mapping: %s", err)
		}
	}

	// Update the index
	if updateMax {
		if err := indexUpdateMaxTxn(tx, idx, "sessions"); err != nil {
			return fmt.Errorf("failed updating sessions index: %v", err)
		}
	} else {
		err := tx.Insert(tableIndex, &IndexEntry{"sessions", idx})
		if err != nil {
			return fmt.Errorf("failed updating sessions index: %v", err)
		}
	}

	return nil
}

func allNodeSessionsTxn(tx ReadTxn, node string) (structs.Sessions, error) {
	return nodeSessionsTxn(tx, nil, node, nil)
}

func nodeSessionsTxn(tx ReadTxn,
	ws memdb.WatchSet, node string, entMeta *structs.EnterpriseMeta) (structs.Sessions, error) {

	sessions, err := tx.Get(tableSessions, indexNode, Query{Value: node})
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

func sessionMaxIndex(tx ReadTxn, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "sessions")
}

func validateSessionChecksTxn(tx ReadTxn, session *structs.Session) error {
	// Go over the session checks and ensure they exist.
	for _, checkID := range session.CheckIDs() {
		check, err := tx.First(tableChecks, indexID, NodeCheckQuery{Node: session.Node, CheckID: string(checkID)})
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

// SessionList returns a slice containing all of the active sessions.
func (s *Store) SessionList(ws memdb.WatchSet, entMeta *structs.EnterpriseMeta) (uint64, structs.Sessions, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := sessionMaxIndex(tx, entMeta)

	var result structs.Sessions

	// Query all of the active sessions.
	sessions, err := tx.Get(tableSessions, indexID+"_prefix", Query{})
	if err != nil {
		return 0, nil, fmt.Errorf("failed session lookup: %s", err)
	}
	ws.Add(sessions.WatchCh())
	// Go over the sessions and create a slice of them.
	for session := sessions.Next(); session != nil; session = sessions.Next() {
		result = append(result, session.(*structs.Session))
	}

	return idx, result, nil
}
