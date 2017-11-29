package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// aclsTableSchema returns a new table schema used for storing ACL tokens.
func aclsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acls",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ID",
					Lowercase: false,
				},
			},
		},
	}
}

// aclsBootstrapTableSchema returns a new schema used for tracking the ACL
// bootstrap status for a cluster. This is designed to have only a single
// row, so it has a somewhat unusual no-op indexer.
func aclsBootstrapTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acls-bootstrap",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.ConditionalIndex{
					Conditional: func(obj interface{}) (bool, error) { return true, nil },
				},
			},
		},
	}
}

func init() {
	registerSchema(aclsTableSchema)
	registerSchema(aclsBootstrapTableSchema)
}

// ACLs is used to pull all the ACLs from the snapshot.
func (s *Snapshot) ACLs() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("acls", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// ACL is used when restoring from a snapshot. For general inserts, use ACLSet.
func (s *Restore) ACL(acl *structs.ACL) error {
	if err := s.tx.Insert("acls", acl); err != nil {
		return fmt.Errorf("failed restoring acl: %s", err)
	}

	if err := indexUpdateMaxTxn(s.tx, acl.ModifyIndex, "acls"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// ACLBootstrap is used to pull the ACL bootstrap info from the snapshot. This
// might return nil, in which case nothing should be saved to the snapshot.
func (s *Snapshot) ACLBootstrap() (*structs.ACLBootstrap, error) {
	existing, err := s.tx.First("acls-bootstrap", "id")
	if err != nil {
		return nil, fmt.Errorf("failed acl bootstrap lookup: %s", err)
	}
	if existing != nil {
		return existing.(*structs.ACLBootstrap), nil
	}
	return nil, nil
}

// ACLBootstrap is used to restore the ACL bootstrap info from the snapshot.
func (s *Restore) ACLBootstrap(bs *structs.ACLBootstrap) error {
	if err := s.tx.Insert("acls-bootstrap", bs); err != nil {
		return fmt.Errorf("failed updating acl bootstrap: %v", err)
	}
	return nil
}

// ACLBootstrapInit is used to perform a scan for existing tokens which will
// decide whether bootstrapping is allowed for a cluster. This is initiated by
// the leader when it steps up, if necessary. This is because the state store
// snapshots would become incompatible with older agents if we added this on
// the fly, so we rely on the leader to determine a safe time to add this so
// we can start tracking whether bootstrap is enabled. This will return an
// error if bootstrap is already initialized.
//
// This returns a boolean indicating if ACL boostrapping is enabled.
func (s *Store) ACLBootstrapInit(idx uint64) (bool, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Don't allow this to happen more than once.
	existing, err := tx.First("acls-bootstrap", "id")
	if err != nil {
		return false, fmt.Errorf("failed acl bootstrap lookup: %s", err)
	}
	if existing != nil {
		return false, fmt.Errorf("acl bootstrap init already done")
	}

	// See if there are any management tokens, which means we shouldn't
	// allow bootstrapping.
	foundMgmt, err := s.aclHasManagementTokensTxn(tx)
	if err != nil {
		return false, fmt.Errorf("failed checking for management tokens: %v", err)
	}
	allowBootstrap := !foundMgmt

	// Create a new bootstrap record.
	bs := structs.ACLBootstrap{
		AllowBootstrap: allowBootstrap,
		RaftIndex: structs.RaftIndex{
			CreateIndex: idx,
			ModifyIndex: idx,
		},
	}
	if err := tx.Insert("acls-bootstrap", &bs); err != nil {
		return false, fmt.Errorf("failed creating acl bootstrap: %v", err)
	}

	tx.Commit()
	return allowBootstrap, nil
}

// ACLBootstrap is used to perform a one-time ACL bootstrap operation on a
// cluster to get the first management token.
func (s *Store) ACLBootstrap(idx uint64, acl *structs.ACL) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// We must have initialized before this will ever be possible.
	existing, err := tx.First("acls-bootstrap", "id")
	if err != nil {
		return fmt.Errorf("failed acl bootstrap lookup: %s", err)
	}
	if existing == nil {
		return structs.ACLBootstrapNotInitializedErr
	}

	// See if this cluster has already been bootstrapped.
	bs := *existing.(*structs.ACLBootstrap)
	if !bs.AllowBootstrap {
		return structs.ACLBootstrapNotAllowedErr
	}

	// This should not be required since we keep the boolean above in sync
	// with any new management tokens that are added, but since this is such
	// a critical thing for correct operation we perform a sanity check.
	foundMgmt, err := s.aclHasManagementTokensTxn(tx)
	if err != nil {
		return fmt.Errorf("failed checking for management tokens: %v", err)
	}
	if foundMgmt {
		return fmt.Errorf("internal error: acl bootstrap enabled but existing management tokens were found")
	}

	// Bootstrap and then make sure we disable bootstrapping forever. The
	// set will also disable this as a side effect but we want to be super
	// explicit here.
	if err := s.aclSetTxn(tx, idx, acl); err != nil {
		return fmt.Errorf("failed inserting bootstrap token: %v", err)
	}
	if disabled, err := s.aclDisableBootstrapTxn(tx, idx); err != nil || !disabled {
		return fmt.Errorf("failed to disable acl bootstrap (disabled=%v): %v", disabled, err)
	}

	tx.Commit()
	return nil
}

// aclDisableBootstrapTxn will disable ACL bootstrapping if the bootstrap init
// has been completed and bootstrap is currently enabled. This will return true
// if bootstrap is disabled.
func (s *Store) aclDisableBootstrapTxn(tx *memdb.Txn, idx uint64) (bool, error) {
	// If the init hasn't been done then we aren't tracking this yet, so we
	// can bail out. When the init is done for the first time it will scan
	// for management tokens to set the initial state correctly.
	existing, err := tx.First("acls-bootstrap", "id")
	if err != nil {
		return false, fmt.Errorf("failed acl bootstrap lookup: %s", err)
	}
	if existing == nil {
		// Not yet init-ed, nothing to do.
		return false, nil
	}

	// See if bootstrap is already disabled, which is the common case, so we
	// can avoid a spurious write. We do a copy here in case we need to write
	// down below, though.
	bs := *existing.(*structs.ACLBootstrap)
	if !bs.AllowBootstrap {
		return true, nil
	}

	// Need to disable bootstrap!
	bs.AllowBootstrap = false
	bs.ModifyIndex = idx
	if err := tx.Insert("acls-bootstrap", &bs); err != nil {
		return false, fmt.Errorf("failed updating acl bootstrap: %v", err)
	}
	return true, nil
}

// aclHasManagementTokensTxn returns true if any management tokens are present
// in the state store.
func (s *Store) aclHasManagementTokensTxn(tx *memdb.Txn) (bool, error) {
	iter, err := tx.Get("acls", "id")
	if err != nil {
		return false, fmt.Errorf("failed acl lookup: %s", err)
	}
	for acl := iter.Next(); acl != nil; acl = iter.Next() {
		if acl.(*structs.ACL).Type == structs.ACLTypeManagement {
			return true, nil
		}
	}
	return false, nil
}

// ACLGetBootstrap returns the ACL bootstrap status for the cluster, which might
// be nil if it hasn't yet been initialized.
func (s *Store) ACLGetBootstrap() (*structs.ACLBootstrap, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	existing, err := tx.First("acls-bootstrap", "id")
	if err != nil {
		return nil, fmt.Errorf("failed acl bootstrap lookup: %s", err)
	}
	if existing != nil {
		return existing.(*structs.ACLBootstrap), nil
	}
	return nil, nil
}

// ACLSet is used to insert an ACL rule into the state store.
func (s *Store) ACLSet(idx uint64, acl *structs.ACL) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call set on the ACL
	if err := s.aclSetTxn(tx, idx, acl); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// aclSetTxn is the inner method used to insert an ACL rule with the
// proper indexes into the state store.
func (s *Store) aclSetTxn(tx *memdb.Txn, idx uint64, acl *structs.ACL) error {
	// Check that the ID is set
	if acl.ID == "" {
		return ErrMissingACLID
	}

	// Check for an existing ACL
	existing, err := tx.First("acls", "id", acl.ID)
	if err != nil {
		return fmt.Errorf("failed acl lookup: %s", err)
	}

	// Set the indexes
	if existing != nil {
		acl.CreateIndex = existing.(*structs.ACL).CreateIndex
		acl.ModifyIndex = idx
	} else {
		acl.CreateIndex = idx
		acl.ModifyIndex = idx
	}

	// Insert the ACL
	if err := tx.Insert("acls", acl); err != nil {
		return fmt.Errorf("failed inserting acl: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"acls", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// If this is a management token, make sure bootstrapping gets disabled.
	if acl.Type == structs.ACLTypeManagement {
		if _, err := s.aclDisableBootstrapTxn(tx, idx); err != nil {
			return fmt.Errorf("failed disabling acl bootstrapping: %v", err)
		}
	}

	return nil
}

// ACLGet is used to look up an existing ACL by ID.
func (s *Store) ACLGet(ws memdb.WatchSet, aclID string) (uint64, *structs.ACL, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "acls")

	// Query for the existing ACL
	watchCh, acl, err := tx.FirstWatch("acls", "id", aclID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl lookup: %s", err)
	}
	ws.Add(watchCh)

	if acl != nil {
		return idx, acl.(*structs.ACL), nil
	}
	return idx, nil, nil
}

// ACLList is used to list out all of the ACLs in the state store.
func (s *Store) ACLList(ws memdb.WatchSet) (uint64, structs.ACLs, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "acls")

	// Return the ACLs.
	acls, err := s.aclListTxn(tx, ws)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl lookup: %s", err)
	}
	return idx, acls, nil
}

// aclListTxn is used to list out all of the ACLs in the state store. This is a
// function vs. a method so it can be called from the snapshotter.
func (s *Store) aclListTxn(tx *memdb.Txn, ws memdb.WatchSet) (structs.ACLs, error) {
	// Query all of the ACLs in the state store
	iter, err := tx.Get("acls", "id")
	if err != nil {
		return nil, fmt.Errorf("failed acl lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	// Go over all of the ACLs and build the response
	var result structs.ACLs
	for acl := iter.Next(); acl != nil; acl = iter.Next() {
		a := acl.(*structs.ACL)
		result = append(result, a)
	}
	return result, nil
}

// ACLDelete is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *Store) ACLDelete(idx uint64, aclID string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the ACL delete
	if err := s.aclDeleteTxn(tx, idx, aclID); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// aclDeleteTxn is used to delete an ACL from the state store within
// an existing transaction.
func (s *Store) aclDeleteTxn(tx *memdb.Txn, idx uint64, aclID string) error {
	// Look up the existing ACL
	acl, err := tx.First("acls", "id", aclID)
	if err != nil {
		return fmt.Errorf("failed acl lookup: %s", err)
	}
	if acl == nil {
		return nil
	}

	// Delete the ACL from the state store and update indexes
	if err := tx.Delete("acls", acl); err != nil {
		return fmt.Errorf("failed deleting acl: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"acls", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}
