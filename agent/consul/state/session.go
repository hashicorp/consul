package state

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	tableSessions      = "sessions"
	tableSessionChecks = "session_checks"

	indexNodeCheck = "node_check"
)

func indexFromSession(e *structs.Session) ([]byte, error) {
	v := strings.ToLower(e.ID)
	if v == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(v)
	return b.Bytes(), nil
}

// sessionsTableSchema returns a new table schema used for storing session
// information.
func sessionsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableSessions,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer:      sessionIndexer(),
			},
			indexNode: {
				Name:         indexNode,
				AllowMissing: false,
				Unique:       false,
				Indexer:      nodeSessionsIndexer(),
			},
		},
	}
}

// sessionChecksTableSchema returns a new table schema used for storing session
// checks.
func sessionChecksTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableSessionChecks,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer:      idCheckIndexer(),
			},
			indexNodeCheck: {
				Name:         indexNodeCheck,
				AllowMissing: false,
				Unique:       false,
				Indexer:      nodeChecksIndexer(),
			},
			indexSession: {
				Name:         indexSession,
				AllowMissing: false,
				Unique:       false,
				Indexer:      sessionCheckIndexer(),
			},
		},
	}
}

// indexNodeFromSession creates an index key from *structs.Session
func indexNodeFromSession(e *structs.Session) ([]byte, error) {
	v := strings.ToLower(e.Node)
	if v == "" {
		return nil, errMissingValueForIndex
	}
	var b indexBuilder

	b.String(v)
	return b.Bytes(), nil
}

// indexFromNodeCheckIDSession creates an index key from  sessionCheck
func indexFromNodeCheckIDSession(e *sessionCheck) ([]byte, error) {
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

	v = strings.ToLower(e.Session)
	if v == "" {
		return nil, errMissingValueForIndex
	}
	b.String(v)

	return b.Bytes(), nil
}

// indexSessionCheckFromSession creates an index key from  sessionCheck
func indexSessionCheckFromSession(e *sessionCheck) ([]byte, error) {
	var b indexBuilder
	v := strings.ToLower(e.Session)
	if v == "" {
		return nil, errMissingValueForIndex
	}
	b.String(v)

	return b.Bytes(), nil
}

type CheckIDIndex struct {
}

func (index *CheckIDIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName("CheckID")
	isPtr := fv.Kind() == reflect.Ptr
	fv = reflect.Indirect(fv)
	if !isPtr && !fv.IsValid() || !fv.CanInterface() {
		return false, nil,
			fmt.Errorf("field 'EnterpriseMeta' for %#v is invalid %v ", obj, isPtr)
	}

	checkID, ok := fv.Interface().(structs.CheckID)
	if !ok {
		return false, nil, fmt.Errorf("Field 'EnterpriseMeta' is not of type structs.EnterpriseMeta")
	}

	// Enforce lowercase and add null character as terminator
	id := strings.ToLower(string(checkID.ID)) + "\x00"

	return true, []byte(id), nil
}

func (index *CheckIDIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}

	arg = strings.ToLower(arg)

	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func (index *CheckIDIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := index.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// Sessions is used to pull the full list of sessions for use during snapshots.
func (s *Snapshot) Sessions() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableSessions, indexID)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Session is used when restoring from a snapshot. For general inserts, use
// SessionCreate.
func (s *Restore) Session(sess *structs.Session) error {
	if err := insertSessionTxn(s.tx, sess, sess.ModifyIndex, true, true); err != nil {
		return fmt.Errorf("failed inserting session: %s", err)
	}

	return nil
}

// SessionCreate is used to register a new session in the state store.
func (s *Store) SessionCreate(idx uint64, sess *structs.Session) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// This code is technically able to (incorrectly) update an existing
	// session but we never do that in practice. The upstream endpoint code
	// always adds a unique ID when doing a create operation so we never hit
	// an existing session again. It isn't worth the overhead to verify
	// that here, but it's worth noting that we should never do this in the
	// future.

	// Call the session creation
	if err := sessionCreateTxn(tx, idx, sess); err != nil {
		return err
	}

	return tx.Commit()
}

// sessionCreateTxn is the inner method used for creating session entries in
// an open transaction. Any health checks registered with the session will be
// checked for failing status. Returns any error encountered.
func sessionCreateTxn(tx WriteTxn, idx uint64, sess *structs.Session) error {
	// Check that we have a session ID
	if sess.ID == "" {
		return ErrMissingSessionID
	}

	// Verify the session behavior is valid
	switch sess.Behavior {
	case "":
		// Release by default to preserve backwards compatibility
		sess.Behavior = structs.SessionKeysRelease
	case structs.SessionKeysRelease:
	case structs.SessionKeysDelete:
	default:
		return fmt.Errorf("Invalid session behavior: %s", sess.Behavior)
	}

	// Assign the indexes. ModifyIndex likely will not be used but
	// we set it here anyways for sanity.
	sess.CreateIndex = idx
	sess.ModifyIndex = idx

	// Check that the node exists
	node, err := tx.First(tableNodes, indexID, Query{Value: sess.Node, EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(sess.PartitionOrDefault())})
	if err != nil {
		return fmt.Errorf("failed node lookup: %s", err)
	}
	if node == nil {
		return ErrMissingNode
	}

	// Verify that all session checks exist
	if err := validateSessionChecksTxn(tx, sess); err != nil {
		return err
	}

	// Insert the session
	if err := insertSessionTxn(tx, sess, idx, false, false); err != nil {
		return fmt.Errorf("failed inserting session: %s", err)
	}

	return nil
}

// SessionGet is used to retrieve an active session from the state store.
func (s *Store) SessionGet(ws memdb.WatchSet,
	sessionID string, entMeta *acl.EnterpriseMeta) (uint64, *structs.Session, error) {

	tx := s.db.Txn(false)
	defer tx.Abort()

	idx := maxIndexTxnSessions(tx, entMeta)

	// Look up the session by its ID
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	watchCh, session, err := tx.FirstWatch(tableSessions, indexID, Query{Value: sessionID, EnterpriseMeta: *entMeta})

	if err != nil {
		return 0, nil, fmt.Errorf("failed session lookup: %s", err)
	}
	ws.Add(watchCh)

	if session != nil {
		return idx, session.(*structs.Session), nil
	}
	return idx, nil, nil
}

// NodeSessions returns a set of active sessions associated
// with the given node ID. The returned index is the highest
// index seen from the result set.
func (s *Store) NodeSessions(ws memdb.WatchSet, nodeID string, entMeta *acl.EnterpriseMeta) (uint64, structs.Sessions, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxnSessions(tx, entMeta)

	// Get all of the sessions which belong to the node
	result, err := nodeSessionsTxn(tx, ws, nodeID, entMeta)
	if err != nil {
		return 0, nil, err
	}
	return idx, result, nil
}

// SessionDestroy is used to remove an active session. This will
// implicitly invalidate the session and invoke the specified
// session destroy behavior.
func (s *Store) SessionDestroy(idx uint64, sessionID string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Call the session deletion.
	if err := s.deleteSessionTxn(tx, idx, sessionID, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

// deleteSessionTxn is the inner method, which is used to do the actual
// session deletion and handle session invalidation, etc.
func (s *Store) deleteSessionTxn(tx WriteTxn, idx uint64, sessionID string, entMeta *acl.EnterpriseMeta) error {
	// Look up the session.
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	sess, err := tx.First(tableSessions, indexID, Query{Value: sessionID, EnterpriseMeta: *entMeta})
	if err != nil {
		return fmt.Errorf("failed session lookup: %s", err)
	}
	if sess == nil {
		return nil
	}

	// Delete the session and write the new index.
	session := sess.(*structs.Session)
	if err := sessionDeleteWithSession(tx, session, idx); err != nil {
		return fmt.Errorf("failed deleting session: %v", err)
	}

	// Enforce the max lock delay.
	delay := session.LockDelay
	if delay > structs.MaxLockDelay {
		delay = structs.MaxLockDelay
	}

	// Snag the current now time so that all the expirations get calculated
	// the same way.
	now := time.Now()

	// Get an iterator over all of the keys with the given session.
	entries, err := tx.Get(tableKVs, indexSession, sessionID)
	if err != nil {
		return fmt.Errorf("failed kvs lookup: %s", err)
	}
	var kvs []interface{}
	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		kvs = append(kvs, entry)
	}

	// Invalidate any held locks.
	switch session.Behavior {
	case structs.SessionKeysRelease:
		for _, obj := range kvs {
			// Note that we clone here since we are modifying the
			// returned object and want to make sure our set op
			// respects the transaction we are in.
			e := obj.(*structs.DirEntry).Clone()
			e.Session = ""
			if err := kvsSetTxn(tx, idx, e, true); err != nil {
				return fmt.Errorf("failed kvs update: %s", err)
			}

			// Apply the lock delay if present.
			if delay > 0 {
				s.lockDelay.SetExpiration(e.Key, now, delay, entMeta)
			}
		}
	case structs.SessionKeysDelete:
		for _, obj := range kvs {
			e := obj.(*structs.DirEntry)
			if err := s.kvsDeleteTxn(tx, idx, e.Key, entMeta); err != nil {
				return fmt.Errorf("failed kvs delete: %s", err)
			}

			// Apply the lock delay if present.
			if delay > 0 {
				s.lockDelay.SetExpiration(e.Key, now, delay, entMeta)
			}
		}
	default:
		return fmt.Errorf("unknown session behavior %#v", session.Behavior)
	}

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	// Delete any check mappings.
	mappings, err := tx.Get(tableSessionChecks, indexSession, Query{Value: sessionID, EnterpriseMeta: *entMeta})
	if err != nil {
		return fmt.Errorf("failed session checks lookup: %s", err)
	}
	{
		var objs []interface{}
		for mapping := mappings.Next(); mapping != nil; mapping = mappings.Next() {
			objs = append(objs, mapping)
		}

		// Do the delete in a separate loop so we don't trash the iterator.
		for _, obj := range objs {
			if err := tx.Delete(tableSessionChecks, obj); err != nil {
				return fmt.Errorf("failed deleting session check: %s", err)
			}
		}
	}

	// Delete any prepared queries.
	queries, err := tx.Get("prepared-queries", "session", sessionID)
	if err != nil {
		return fmt.Errorf("failed prepared query lookup: %s", err)
	}
	{
		var ids []string
		for wrapped := queries.Next(); wrapped != nil; wrapped = queries.Next() {
			ids = append(ids, toPreparedQuery(wrapped).ID)
		}

		// Do the delete in a separate loop so we don't trash the iterator.
		for _, id := range ids {
			if err := preparedQueryDeleteTxn(tx, idx, id); err != nil {
				return fmt.Errorf("failed prepared query delete: %s", err)
			}
		}
	}

	return nil
}
