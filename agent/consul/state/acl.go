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
			"accessor": &memdb.IndexSchema{
				Name: "accessor",
				// DEPRECATED (ACL-Legacy-Compat) - we should not AllowMissing here once legacy compat is removed
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "AccessorID",
				},
			},
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "SecretID",
					Lowercase: false,
				},
			},
			"policies": &memdb.IndexSchema{
				Name: "policies",
				// Need to allow missing for the anonymous token
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenPoliciesIndex{},
			},
			"local": &memdb.IndexSchema{
				Name:         "local",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: func(obj interface{}) (bool, error) {
						if token, ok := obj.(*structs.ACLToken); ok {
							return token.Local, nil
						}
						return false, nil
					},
				},
			},

			//DEPRECATED (ACL-Legacy-Compat) - This index is only needed while we support upgrading v1 to v2 acls
			// This table indexes all the ACL tokens that do not have an AccessorID
			"needs-upgrade": &memdb.IndexSchema{
				Name:         "needs-upgrade",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: func(obj interface{}) (bool, error) {
						if token, ok := obj.(*structs.ACLToken); ok {
							return token.AccessorID == "", nil
						}
						return false, nil
					},
				},
			},
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
	// DEPRECATED (ACL-Legacy-Compat) - This could use the "id" index when we remove v1 compat
	iter, err := s.tx.Get("acl-tokens", "id")
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
func (s *Store) ACLBootstrap(idx, resetIndex uint64, token *structs.ACLToken, legacy bool) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// We must have initialized before this will ever be possible.
	existing, err := tx.First("index", "id", "acl-token-bootstrap")
	if err != nil {
		return fmt.Errorf("bootstrap check failed: %v", err)
	}
	if existing != nil {
		if resetIndex == 0 {
			return structs.ACLBootstrapNotAllowedErr
		} else if resetIndex != existing.(*IndexEntry).Value {
			return structs.ACLBootstrapInvalidResetIndexErr
		}
	}

	if err := s.aclTokenSetTxn(tx, idx, token, false, false, legacy); err != nil {
		return fmt.Errorf("failed inserting bootstrap token: %v", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
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
func (s *Store) ACLTokenSet(idx uint64, token *structs.ACLToken, legacy bool) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call set on the ACL
	if err := s.aclTokenSetTxn(tx, idx, token, false, false, legacy); err != nil {
		return err
	}

	if err := indexUpdateMaxTxn(tx, idx, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

func (s *Store) ACLTokenBatchSet(idx uint64, tokens structs.ACLTokens, cas bool) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, token := range tokens {
		// this is only used when doing batch insertions for upgrades and replication. Therefore
		// we take whatever those said.
		if err := s.aclTokenSetTxn(tx, idx, token, cas, true, false); err != nil {
			return err
		}
	}

	if err := indexUpdateMaxTxn(tx, idx, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

// aclTokenSetTxn is the inner method used to insert an ACL token with the
// proper indexes into the state store.
func (s *Store) aclTokenSetTxn(tx *memdb.Txn, idx uint64, token *structs.ACLToken, cas, allowMissingPolicyIDs, legacy bool) error {
	// Check that the ID is set
	if token.SecretID == "" {
		return ErrMissingACLTokenSecret
	}

	if !legacy && token.AccessorID == "" {
		return ErrMissingACLTokenAccessor
	}

	// DEPRECATED (ACL-Legacy-Compat)
	if token.Rules != "" {
		// When we update a legacy acl token we may have to correct old HCL to
		// prevent the propagation of older syntax into the state store and
		// into in-memory representations.
		correctedRules := structs.SanitizeLegacyACLTokenRules(token.Rules)
		if correctedRules != "" {
			token.Rules = correctedRules
		}
	}

	// Check for an existing ACL
	// DEPRECATED (ACL-Legacy-Compat) - transition to using accessor index instead of secret once v1 compat is removed
	existing, err := tx.First("acl-tokens", "id", token.SecretID)
	if err != nil {
		return fmt.Errorf("failed token lookup: %s", err)
	}

	var original *structs.ACLToken

	if existing != nil {
		original = existing.(*structs.ACLToken)
	}

	if cas {
		// set-if-unset case
		if token.ModifyIndex == 0 && original != nil {
			return nil
		}
		// token already deleted
		if token.ModifyIndex != 0 && original == nil {
			return nil
		}
		// check for other modifications
		if token.ModifyIndex != 0 && token.ModifyIndex != original.ModifyIndex {
			return nil
		}
	}

	if legacy && original != nil {
		if len(original.Policies) > 0 || original.Type == "" {
			return fmt.Errorf("failed inserting acl token: cannot use legacy endpoint to modify a non-legacy token")
		}

		token.AccessorID = original.AccessorID
	}

	if err := s.resolveTokenPolicyLinks(tx, token, allowMissingPolicyIDs); err != nil {
		return err
	}

	// Set the indexes
	if original != nil {
		if original.AccessorID != "" && token.AccessorID != original.AccessorID {
			return fmt.Errorf("The ACL Token AccessorID field is immutable")
		}

		if token.SecretID != original.SecretID {
			return fmt.Errorf("The ACL Token SecretID field is immutable")
		}

		token.CreateIndex = original.CreateIndex
		token.ModifyIndex = idx
	} else {
		token.CreateIndex = idx
		token.ModifyIndex = idx
	}

	// Insert the ACL
	if err := tx.Insert("acl-tokens", token); err != nil {
		return fmt.Errorf("failed inserting acl token: %v", err)
	}

	return nil
}

// ACLTokenGetBySecret is used to look up an existing ACL token by its SecretID.
func (s *Store) ACLTokenGetBySecret(ws memdb.WatchSet, secret string) (uint64, *structs.ACLToken, error) {
	return s.aclTokenGet(ws, secret, "id")
}

// ACLTokenGetByAccessor is used to look up an existing ACL token by its AccessorID.
func (s *Store) ACLTokenGetByAccessor(ws memdb.WatchSet, accessor string) (uint64, *structs.ACLToken, error) {
	return s.aclTokenGet(ws, accessor, "accessor")
}

// aclTokenGet looks up a token using one of the indexes provided
func (s *Store) aclTokenGet(ws memdb.WatchSet, value, index string) (uint64, *structs.ACLToken, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	token, err := s.aclTokenGetTxn(tx, ws, value, index)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl token lookup: %v", err)
	}

	idx := maxIndexTxn(tx, "acl-tokens")
	return idx, token, nil
}

func (s *Store) ACLTokenBatchGet(ws memdb.WatchSet, accessors []string) (uint64, structs.ACLTokens, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	tokens := make(structs.ACLTokens, 0)
	for _, accessor := range accessors {
		token, err := s.aclTokenGetTxn(tx, ws, accessor, "accessor")
		if err != nil {
			return 0, nil, fmt.Errorf("failed acl token lookup: %v", err)
		}

		// token == nil is valid and will indic
		if token != nil {
			tokens = append(tokens, token)
		}
	}

	idx := maxIndexTxn(tx, "acl-tokens")

	return idx, tokens, nil
}

func (s *Store) aclTokenGetTxn(tx *memdb.Txn, ws memdb.WatchSet, value, index string) (*structs.ACLToken, error) {
	watchCh, rawToken, err := tx.FirstWatch("acl-tokens", index, value)
	if err != nil {
		return nil, fmt.Errorf("failed acl token lookup: %v", err)
	}
	ws.Add(watchCh)

	if rawToken != nil {
		token := rawToken.(*structs.ACLToken)
		if err := s.resolveTokenPolicyLinks(tx, token, true); err != nil {
			return nil, err
		}
		return token, nil
	}

	return nil, nil
}

// ACLTokenList is used to list out all of the ACLs in the state store.
func (s *Store) ACLTokenList(ws memdb.WatchSet, local, global bool, policy string) (uint64, structs.ACLTokens, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var iter memdb.ResultIterator
	var err error

	// Note global == local works when both are true or false. It is not valid to set both
	// to false but for defaulted structs (zero values for both) we want it to list out
	// all tokens so our checks just ensure that global == local

	if policy != "" {
		iter, err = tx.Get("acl-tokens", "policies", policy)
		if err == nil && global != local {
			iter = memdb.NewFilterIterator(iter, func(raw interface{}) bool {
				token, ok := raw.(*structs.ACLToken)
				if !ok {
					return false
				}

				if global && !token.Local {
					return true
				} else if local && token.Local {
					return true
				}

				return false
			})
		}
	} else if global == local {
		iter, err = tx.Get("acl-tokens", "id")
	} else if global {
		iter, err = tx.Get("acl-tokens", "local", false)
	} else {
		iter, err = tx.Get("acl-tokens", "local", true)
	}

	if err != nil {
		return 0, nil, fmt.Errorf("failed acl token lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLTokens
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		token := raw.(*structs.ACLToken)
		if err := s.resolveTokenPolicyLinks(tx, token, true); err != nil {
			return 0, nil, err
		}
		result = append(result, token)
	}

	// Get the table index.
	idx := maxIndexTxn(tx, "acl-tokens")

	return idx, result, nil
}

func (s *Store) ACLTokenListUpgradeable(max int) (structs.ACLTokens, <-chan struct{}, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := tx.Get("acl-tokens", "needs-upgrade", true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed acl token listing: %v", err)
	}

	var tokens structs.ACLTokens
	i := 0
	for token := iter.Next(); token != nil; token = iter.Next() {
		tokens = append(tokens, token.(*structs.ACLToken))
		i += 1
		if i >= max {
			return tokens, nil, nil
		}
	}

	return tokens, iter.WatchCh(), nil
}

// ACLTokenDeleteBySecret is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *Store) ACLTokenDeleteBySecret(idx uint64, secret string) error {
	return s.aclTokenDelete(idx, secret, "id")
}

// ACLTokenDeleteByAccessor is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *Store) ACLTokenDeleteByAccessor(idx uint64, accessor string) error {
	return s.aclTokenDelete(idx, accessor, "accessor")
}

func (s *Store) ACLTokenBatchDelete(idx uint64, tokenIDs []string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, tokenID := range tokenIDs {
		if err := s.aclTokenDeleteTxn(tx, idx, tokenID, "accessor"); err != nil {
			return err
		}
	}

	tx.Commit()
	return nil
}

func (s *Store) aclTokenDelete(idx uint64, value, index string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclTokenDeleteTxn(tx, idx, value, index); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) aclTokenDeleteTxn(tx *memdb.Txn, idx uint64, value, index string) error {
	// Look up the existing token
	token, err := tx.First("acl-tokens", index, value)
	if err != nil {
		return fmt.Errorf("failed acl token lookup: %v", err)
	}

	if token == nil {
		return nil
	}

	if token.(*structs.ACLToken).AccessorID == structs.ACLTokenAnonymousID {
		return fmt.Errorf("Deletion of the builtin anonymous token is not permitted")
	}

	if err := tx.Delete("acl-tokens", token); err != nil {
		return fmt.Errorf("failed deleting acl token: %v", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}
	return nil
}

func (s *Store) ACLPolicyBatchSet(idx uint64, policies structs.ACLPolicies) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, policy := range policies {
		if err := s.aclPolicySetTxn(tx, idx, policy); err != nil {
			return err
		}
	}

	if err := indexUpdateMaxTxn(tx, idx, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

func (s *Store) ACLPolicySet(idx uint64, policy *structs.ACLPolicy) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclPolicySetTxn(tx, idx, policy); err != nil {
		return err
	}
	if err := indexUpdateMaxTxn(tx, idx, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

func (s *Store) aclPolicySetTxn(tx *memdb.Txn, idx uint64, policy *structs.ACLPolicy) error {
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
	return nil
}

func (s *Store) ACLPolicyGetByID(ws memdb.WatchSet, id string) (uint64, *structs.ACLPolicy, error) {
	return s.aclPolicyGet(ws, id, "id")
}

func (s *Store) ACLPolicyGetByName(ws memdb.WatchSet, name string) (uint64, *structs.ACLPolicy, error) {
	return s.aclPolicyGet(ws, name, "name")
}

func (s *Store) ACLPolicyBatchGet(ws memdb.WatchSet, ids []string) (uint64, structs.ACLPolicies, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	policies := make(structs.ACLPolicies, 0)
	for _, pid := range ids {
		policy, err := s.getPolicyWithTxn(tx, ws, pid, "id")
		if err != nil {
			return 0, nil, err
		}

		if policy != nil {
			policies = append(policies, policy)
		}
	}

	idx := maxIndexTxn(tx, "acl-policies")

	return idx, policies, nil
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

func (s *Store) ACLPolicyBatchDelete(idx uint64, policyIDs []string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, policyID := range policyIDs {
		s.aclPolicyDeleteTxn(tx, idx, policyID, "id")
	}

	if err := indexUpdateMaxTxn(tx, idx, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}
	tx.Commit()
	return nil
}

func (s *Store) aclPolicyDelete(idx uint64, value, index string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclPolicyDeleteTxn(tx, idx, value, index); err != nil {
		return err
	}
	if err := indexUpdateMaxTxn(tx, idx, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	tx.Commit()
	return nil
}

func (s *Store) aclPolicyDeleteTxn(tx *memdb.Txn, idx uint64, value, index string) error {
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
	return nil
}
