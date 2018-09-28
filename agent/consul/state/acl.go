package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

type TokenPoliciesIndex struct {
}

func (s *TokenPoliciesIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	token, ok := obj.(*structs.ACLToken)
	if !ok {
		return false, nil, fmt.Errorf("object is not an ACLTokenPolicyLink")
	}

	links := token.Policies

	numLinks := len(links)
	if numLinks == 0 {
		return false, nil, nil
	}

	vals := make([][]byte, 0, numLinks)
	for _, link := range links {
		vals = append(vals, []byte(link.ID+"\x00"))
	}

	return true, vals, nil
}

func (s *TokenPoliciesIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func (s *TokenPoliciesIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
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

func tokensTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl-tokens",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name: "id",
				// DEPRECATED (ACL-Legacy-Compat) - do not allow missing when v1 acls are removed
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "AccessorID",
				},
			},
			"secret": &memdb.IndexSchema{
				Name:         "secret",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "SecretID",
					Lowercase: false,
				},
			},
			"policies": &memdb.IndexSchema{
				Name: "policies",
				// DEPRECATED (ACL-Legacy-Compat) - do not allow missing when v1 acls are removed
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenPoliciesIndex{},
			},
			// DEPRECATED (ACL-Legacy-Compat) - remove this index when v1 acl support is removed
			// This index will be used for the listing the old APIs
			// Not entirely sure being able to look at just legacy tokens would be useful.
			// "legacy": &memdb.IndexSchema{
			// 	Name:          "legacy",
			// 	AllowMissing:  false,
			// 	Unique:        false,
			// 	Indexer: &memdb.ConditionalIndex{
			// 		Conditional: func(obj interface{}) (bool, error) {
			// 			if tok, ok := obj.(*structs.Token); ok {
			// 				return tok.Type != structs.TokenTypeNone, nil
			// 			}
			// 			return false, nil
			// 		},
			// 	},
			// },

			//DEPRECATED (ACL-Legacy-Compat) - This index is only needed while we support upgrading v1 to v2 acls
			// This table indexes all the ACL tokens that do not have an AccessorID to m
			// "needs-upgrade": &memdb.IndexSchema{
			// 	Name: "needs-upgrade",
			// 	AllowMissing: false,
			// 	Unique: false,
			// 	Indexer: &memdb.ConditionalIndex{
			// 		Conditional: func(obj interface{}) (bool, error) {
			// 			if token, ok := obj.(*structs.ACLToken); ok {
			// 				return token.AccessorID == "", nil
			// 			}
			// 			return false, nil
			// 		},
			// 	},
			// },
		},
	}
}

func policiesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl-policies",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"name": &memdb.IndexSchema{
				Name:         "name",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Name",
					// TODO (ACL-V2) - should we coerce to lowercase?
					Lowercase: true,
				},
			},
		},
	}
}

func init() {
	registerSchema(tokensTableSchema)
	registerSchema(policiesTableSchema)
}

// ACLTokens is used when saving a snapshot
func (s *Snapshot) ACLTokens() (memdb.ResultIterator, error) {
	// DEPRECATED (ACL-Legacy-Compat) - This could use the "accessor" index when we remove v1 compat
	iter, err := s.tx.Get("acl-tokens", "secret")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// ACLToken is used when restoring from a snapshot. For general inserts, use ACL.
func (s *Restore) ACLToken(token *structs.ACLToken) error {
	if err := s.tx.Insert("acl-tokens", token); err != nil {
		return fmt.Errorf("failed restoring acl token: %s", err)
	}

	if err := indexUpdateMaxTxn(s.tx, token.ModifyIndex, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// ACLPolicies is used when saving a snapshot
func (s *Snapshot) ACLPolicies() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("acl-policies", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func (s *Restore) ACLPolicy(policy *structs.ACLPolicy) error {
	if err := s.tx.Insert("acl-policies", policy); err != nil {
		return fmt.Errorf("failed restoring acl policy: %s", err)
	}

	if err := indexUpdateMaxTxn(s.tx, policy.ModifyIndex, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// ACLBootstrap is used to perform a one-time ACL bootstrap operation on a
// cluster to get the first management token.
func (s *Store) ACLBootstrap(idx, resetIndex uint64, token *structs.ACLToken) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// We must have initialized before this will ever be possible.
	existing, err := tx.First("index", "id", "acl-token-bootstrap")
	if err != nil {
		fmt.Errorf("bootstrap check failed: %v", err)
	}
	if existing != nil {
		if resetIndex == 0 {
			return fmt.Errorf("ACL bootstrap was already done")
		} else if resetIndex != existing.(*IndexEntry).Value {
			return fmt.Errorf("invalid reset index for ACL bootstrap")
		}
	}

	if err := s.aclTokenSetTxn(tx, idx, token); err != nil {
		return fmt.Errorf("failed inserting bootstrap token: %v", err)
	}
	if err := tx.Insert("index", &IndexEntry{"acl-token-bootstrap", idx}); err != nil {
		return fmt.Errorf("failed to mark ACL bootstrapping as complete: %v", err)
	}
	tx.Commit()
	return nil
}

// CanBootstrapACLToken checks if bootstrapping is possible and returns the reset index
func (s *Store) CanBootstrapACLToken() (bool, uint64, error) {
	txn := s.db.Txn(false)

	// Lookup the bootstrap sentinel
	out, err := txn.First("index", "id", "acl-token-bootstrap")
	if err != nil {
		return false, 0, err
	}

	// No entry, we haven't bootstrapped yet
	if out == nil {
		return true, 0, nil
	}

	// Return the reset index if we've already bootstrapped
	return false, out.(*IndexEntry).Value, nil
}

// ACLGetBootstrap returns the ACL bootstrap status for the cluster, which might
// be nil if it hasn't yet been initialized.
func (s *Store) ACLGetBootstrap() (bool, uint64, error) {
	_, index, err := s.CanBootstrapACLToken()
	if err != nil {
		return false, 0, fmt.Errorf("failed acl bootstrap lookup: %v", err)
	}

	return index != 0, index, nil
}

func (s *Store) resolveTokenPolicyLinks(tx *memdb.Txn, token *structs.ACLToken, allowMissing bool) error {
	for linkIndex, link := range token.Policies {
		if link.ID != "" {
			policy, err := s.getPolicyWithTxn(tx, nil, link.ID, "id")

			if err != nil {
				return err
			}

			if policy != nil {
				// the name doesn't matter here
				token.Policies[linkIndex].Name = policy.Name
			} else if !allowMissing {
				return fmt.Errorf("No such policy with ID: %s", link.ID)
			}
		} else {
			return fmt.Errorf("Encountered a Token with policies linked by Name in the state store")
		}
	}
	return nil
}

// ACLTokenSet is used to insert an ACL rule into the state store.
func (s *Store) ACLTokenSet(idx uint64, token *structs.ACLToken) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call set on the ACL
	if err := s.aclTokenSetTxn(tx, idx, token); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// aclTokenSetTxn is the inner method used to insert an ACL token with the
// proper indexes into the state store.
func (s *Store) aclTokenSetTxn(tx *memdb.Txn, idx uint64, token *structs.ACLToken) error {
	// Check that the ID is set
	if token.SecretID == "" {
		return ErrMissingACLTokenSecret
	}

	if token.AccessorID == "" {
		return ErrMissingACLTokenAccessor
	}

	// Check for an existing ACL
	// DEPRECATED (ACL-Legacy-Compat) - transition to using accessor index instead of secret once v1 compat is removed
	existing, err := tx.First("acl-tokens", "secret", token.SecretID)
	if err != nil {
		return fmt.Errorf("failed token lookup: %s", err)
	}

	if err := s.resolveTokenPolicyLinks(tx, token, false); err != nil {
		return err
	}

	// Set the indexes
	if existing != nil {
		token.CreateIndex = existing.(*structs.ACLToken).CreateIndex
		token.ModifyIndex = idx
	} else {
		token.CreateIndex = idx
		token.ModifyIndex = idx
	}

	// Insert the ACL
	if err := tx.Insert("acl-tokens", token); err != nil {
		return fmt.Errorf("failed inserting acl token: %v", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// ACLTokenGetBySecret is used to look up an existing ACL token by its SecretID.
func (s *Store) ACLTokenGetBySecret(ws memdb.WatchSet, secret string) (uint64, *structs.ACLToken, error) {
	return s.aclTokenGet(ws, secret, "secret")
}

// ACLTokenGetByAccessor is used to look up an existing ACL token by its AccessorID.
func (s *Store) ACLTokenGetByAccessor(ws memdb.WatchSet, accessor string) (uint64, *structs.ACLToken, error) {
	return s.aclTokenGet(ws, accessor, "id")
}

// aclTokenGet looks up a token using one of the indexes provided
func (s *Store) aclTokenGet(ws memdb.WatchSet, value, index string) (uint64, *structs.ACLToken, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	watchCh, rawToken, err := tx.FirstWatch("acl-tokens", index, value)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl token lookup: %v", err)
	}
	ws.Add(watchCh)

	idx := maxIndexTxn(tx, "acl-tokens")

	if rawToken != nil {
		token := rawToken.(*structs.ACLToken)
		if err := s.resolveTokenPolicyLinks(tx, token, true); err != nil {
			return 0, nil, err
		}
		return idx, token, nil
	}

	return idx, nil, nil
}

// ACLTokenList is used to list out all of the ACLs in the state store.
func (s *Store) ACLTokenList(ws memdb.WatchSet) (uint64, structs.ACLTokens, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := tx.Get("acl-tokens", "secret")
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl token lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLTokens
	for token := iter.Next(); token != nil; token = iter.Next() {
		result = append(result, token.(*structs.ACLToken))
	}

	// Get the table index.
	idx := maxIndexTxn(tx, "acl-tokens")

	return idx, result, nil
}

func (s *Store) ACLGet(ws memdb.WatchSet, aclID string) (uint64, *structs.ACL, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "acl-tokens")

	// Query for the existing ACL
	watchCh, token, err := tx.FirstWatch("acl-tokens", "secret", aclID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl lookup: %s", err)
	}
	ws.Add(watchCh)

	if token != nil {
		acl, _ := token.(*structs.ACLToken).Convert()
		if acl != nil {
			return idx, acl, nil
		}
	}
	return idx, nil, nil
}

// DEPRECATED (ACL-Legacy-Compat) - Only needed for V1 compat
func (s *Store) ACLList(ws memdb.WatchSet) (uint64, structs.ACLs, error) {
	var result structs.ACLs
	idx, tokens, err := s.ACLTokenList(ws)
	if err != nil {
		return 0, nil, err
	}

	for _, token := range tokens {
		if compat, err := token.Convert(); err == nil && compat != nil {
			result = append(result, compat)
		}
	}

	return idx, result, nil
}

// ACLTokenDeleteSecret is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *Store) ACLTokenDeleteSecret(idx uint64, secret string) error {
	return s.aclTokenDelete(idx, secret, "secret")
}

// ACLTokenDeleteAccessor is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *Store) ACLTokenDeleteAccessor(idx uint64, accessor string) error {
	return s.aclTokenDelete(idx, accessor, "id")
}

func (s *Store) aclTokenDelete(idx uint64, value, index string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Look up the existing token
	token, err := tx.First("acl-tokens", index, value)
	if err != nil {
		return fmt.Errorf("failed acl token lookup: %v", err)
	}

	if token == nil {
		return nil
	}

	if err := tx.Delete("acl-tokens", token); err != nil {
		return fmt.Errorf("failed deleting acl token: %v", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	tx.Commit()
	return nil
}

func (s *Store) ACLPolicySet(idx uint64, policy *structs.ACLPolicy) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Check that the ID is set
	if policy.ID == "" {
		return ErrMissingACLPolicyID
	}

	if policy.Name == "" {
		return ErrMissingACLPolicyName
	}

	var policyMatch *structs.ACLPolicy
	existing, err := tx.First("acl-policies", "id", policy.ID)
	if err != nil {
		return fmt.Errorf("failed acl policy lookup: %v", err)
	}

	if existing != nil {
		policyMatch = existing.(*structs.ACLPolicy)

		if policy.ID == structs.ACLPolicyGlobalManagementID {
			// Only the name and description are modifiable
			if policy.Rules != policyMatch.Rules {
				return fmt.Errorf("Changing the Rules for the builtin global-management policy is not permitted")
			}

			if policy.Datacenters != nil && len(policy.Datacenters) != 0 {
				return fmt.Errorf("Changing the Datacenters of the builtin global-management policy is not permitted")
			}
		}
	}

	// ensure the name is unique (cannot conflict with another policy with a different ID)
	nameMatch, err := tx.First("acl-policies", "name", policy.Name)
	if err != nil {
		return fmt.Errorf("failed acl policy lookup: %v", err)
	}
	if nameMatch != nil && policy.ID != nameMatch.(*structs.ACLPolicy).ID {
		return fmt.Errorf("A policy with name %q already exists", policy.Name)
	}

	// Set the indexes
	if existing != nil {
		policy.CreateIndex = existing.(*structs.ACLPolicy).CreateIndex
		policy.ModifyIndex = idx
	} else {
		policy.CreateIndex = idx
		policy.ModifyIndex = idx
	}

	// Insert the ACL
	if err := tx.Insert("acl-policies", policy); err != nil {
		return fmt.Errorf("failed inserting acl policy: %v", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

func (s *Store) ACLPolicyGetByID(ws memdb.WatchSet, id string) (uint64, *structs.ACLPolicy, error) {
	return s.aclPolicyGet(ws, id, "id")
}

func (s *Store) ACLPolicyGetByName(ws memdb.WatchSet, name string) (uint64, *structs.ACLPolicy, error) {
	return s.aclPolicyGet(ws, name, "name")
}

func (s *Store) getPolicyWithTxn(tx *memdb.Txn, ws memdb.WatchSet, value, index string) (*structs.ACLPolicy, error) {
	watchCh, policy, err := tx.FirstWatch("acl-policies", index, value)
	if err != nil {
		return nil, fmt.Errorf("failed acl policy lookup: %v", err)
	}
	ws.Add(watchCh)

	if err != nil || policy == nil {
		return nil, err
	}

	return policy.(*structs.ACLPolicy), nil
}

func (s *Store) aclPolicyGet(ws memdb.WatchSet, value, index string) (uint64, *structs.ACLPolicy, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	policy, err := s.getPolicyWithTxn(tx, ws, value, index)
	if err != nil {
		return 0, nil, err
	}

	idx := maxIndexTxn(tx, "acl-policies")

	return idx, policy, nil
}

func (s *Store) ACLPolicyList(ws memdb.WatchSet) (uint64, structs.ACLPolicies, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := tx.Get("acl-policies", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl policy lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLPolicies
	for policy := iter.Next(); policy != nil; policy = iter.Next() {
		result = append(result, policy.(*structs.ACLPolicy))
	}

	// Get the table index.
	idx := maxIndexTxn(tx, "acl-policies")

	return idx, result, nil
}

func (s *Store) ACLPolicyDeleteByID(idx uint64, id string) error {
	return s.aclPolicyDelete(idx, id, "id")
}

func (s *Store) ACLPolicyDeleteByName(idx uint64, name string) error {
	return s.aclPolicyDelete(idx, name, "name")
}

func (s *Store) aclPolicyDelete(idx uint64, value, index string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Look up the existing token
	rawPolicy, err := tx.First("acl-policies", index, value)
	if err != nil {
		return fmt.Errorf("failed acl policy lookup: %v", err)
	}

	if rawPolicy == nil {
		return nil
	}

	policy := rawPolicy.(*structs.ACLPolicy)

	if policy.ID == structs.ACLPolicyGlobalManagementID {
		return fmt.Errorf("Deletion of the builtin global-management policy is not permitted")
	}

	if err := tx.Delete("acl-policies", policy); err != nil {
		return fmt.Errorf("failed deleting acl policy: %v", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	tx.Commit()
	return nil
}
