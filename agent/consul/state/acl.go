package state

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/agentpb"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

type TokenPoliciesIndex struct {
}

func (s *TokenPoliciesIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	token, ok := obj.(*structs.ACLToken)
	if !ok {
		return false, nil, fmt.Errorf("object is not an ACLToken")
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

type TokenRolesIndex struct {
}

func (s *TokenRolesIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	token, ok := obj.(*structs.ACLToken)
	if !ok {
		return false, nil, fmt.Errorf("object is not an ACLToken")
	}

	links := token.Roles

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

func (s *TokenRolesIndex) FromArgs(args ...interface{}) ([]byte, error) {
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

func (s *TokenRolesIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
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

type RolePoliciesIndex struct {
}

func (s *RolePoliciesIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	role, ok := obj.(*structs.ACLRole)
	if !ok {
		return false, nil, fmt.Errorf("object is not an ACLRole")
	}

	links := role.Policies

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

func (s *RolePoliciesIndex) FromArgs(args ...interface{}) ([]byte, error) {
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

func (s *RolePoliciesIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
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

type TokenExpirationIndex struct {
	LocalFilter bool
}

func (s *TokenExpirationIndex) encodeTime(t time.Time) []byte {
	val := t.Unix()
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(val))
	return buf
}

func (s *TokenExpirationIndex) FromObject(obj interface{}) (bool, []byte, error) {
	token, ok := obj.(*structs.ACLToken)
	if !ok {
		return false, nil, fmt.Errorf("object is not an ACLToken")
	}
	if s.LocalFilter != token.Local {
		return false, nil, nil
	}
	if !token.HasExpirationTime() {
		return false, nil, nil
	}
	if token.ExpirationTime.Unix() < 0 {
		return false, nil, fmt.Errorf("token expiration time cannot be before the unix epoch: %s", token.ExpirationTime)
	}

	buf := s.encodeTime(*token.ExpirationTime)

	return true, buf, nil
}

func (s *TokenExpirationIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(time.Time)
	if !ok {
		return nil, fmt.Errorf("argument must be a time.Time: %#v", args[0])
	}
	if arg.Unix() < 0 {
		return nil, fmt.Errorf("argument must be a time.Time after the unix epoch: %s", args[0])
	}

	buf := s.encodeTime(arg)

	return buf, nil
}

func init() {
	registerSchema(tokensTableSchema)
	registerSchema(policiesTableSchema)
	registerSchema(rolesTableSchema)
	registerSchema(bindingRulesTableSchema)
	registerSchema(authMethodsTableSchema)
}

// ACLTokens is used when saving a snapshot
func (s *Snapshot) ACLTokens() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("acl-tokens", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// ACLToken is used when restoring from a snapshot. For general inserts, use ACL.
func (s *Restore) ACLToken(token *structs.ACLToken) error {
	return s.store.aclTokenInsert(s.tx, token)
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
	return s.store.aclPolicyInsert(s.tx, policy)
}

// ACLRoles is used when saving a snapshot
func (s *Snapshot) ACLRoles() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("acl-roles", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func (s *Restore) ACLRole(role *structs.ACLRole) error {
	return s.store.aclRoleInsert(s.tx, role)
}

// ACLBindingRules is used when saving a snapshot
func (s *Snapshot) ACLBindingRules() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("acl-binding-rules", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func (s *Restore) ACLBindingRule(rule *structs.ACLBindingRule) error {
	return s.store.aclBindingRuleInsert(s.tx, rule)
}

// ACLAuthMethods is used when saving a snapshot
func (s *Snapshot) ACLAuthMethods() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("acl-auth-methods", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func (s *Restore) ACLAuthMethod(method *structs.ACLAuthMethod) error {
	return s.store.aclAuthMethodInsert(s.tx, method)
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

	if err := s.aclTokenSetTxn(tx, idx, token, false, false, false, legacy); err != nil {
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

// resolveACLLinks is used to populate the links Name given its ID as the object is inserted
// into the Store. This will modify the links in place and should not be performed on data
// already inserted into the store without copying first. This is an optimization so that when
// the link is read back out of the Store, we hopefully will not have to copy the object being retrieved
// to update the name. Unlike the older functions to operate specifically on role or policy links
// this function does not itself handle the case where the id cannot be found. Instead the
// getName function should handle that and return an error if necessary
func (s *Store) resolveACLLinks(tx *memdb.Txn, links []agentpb.ACLLink, getName func(*memdb.Txn, string) (string, error)) (int, error) {
	var numValid int
	for linkIndex, link := range links {
		if link.ID != "" {
			name, err := getName(tx, link.ID)

			if err != nil {
				return 0, err
			}

			// the name doesn't matter here
			if name != "" {
				links[linkIndex].Name = name
				numValid++
			}
		} else {
			return 0, fmt.Errorf("Encountered an ACL resource linked by Name in the state store")
		}
	}
	return numValid, nil
}

// fixupACLLinks is used to ensure data returned by read operations have the most up to date name
// associated with the ID of the link. Ideally this will be a no-op if the names are already correct
// however if a linked resource was renamed it might be stale. This function will treat the incoming
// links with copy-on-write semantics and its output will indicate whether any modifications were made.
func (s *Store) fixupACLLinks(tx *memdb.Txn, original []agentpb.ACLLink, getName func(*memdb.Txn, string) (string, error)) ([]agentpb.ACLLink, bool, error) {
	owned := false
	links := original

	cloneLinks := func(l []agentpb.ACLLink, copyNumLinks int) []agentpb.ACLLink {
		clone := make([]agentpb.ACLLink, copyNumLinks)
		copy(clone, l[:copyNumLinks])
		return clone
	}

	for linkIndex, link := range original {
		name, err := getName(tx, link.ID)

		if err != nil {
			return nil, false, err
		}

		if name == "" {
			if !owned {
				// clone the original as we cannot modify anything stored in memdb
				links = cloneLinks(original, linkIndex)
				owned = true
			}
			// if already owned then we just don't append it.
		} else if name != link.Name {
			if !owned {
				links = cloneLinks(original, linkIndex)
				owned = true
			}

			// append the corrected link
			links = append(links, agentpb.ACLLink{ID: link.ID, Name: name})
		} else if owned {
			links = append(links, link)
		}
	}

	return links, owned, nil
}

func (s *Store) resolveTokenPolicyLinks(tx *memdb.Txn, token *structs.ACLToken, allowMissing bool) (int, error) {
	var numValid int
	for linkIndex, link := range token.Policies {
		if link.ID != "" {
			policy, err := s.getPolicyWithTxn(tx, nil, link.ID, s.aclPolicyGetByID, &token.EnterpriseMeta)

			if err != nil {
				return 0, err
			}

			if policy != nil {
				// the name doesn't matter here
				token.Policies[linkIndex].Name = policy.Name
				numValid++
			} else if !allowMissing {
				return 0, fmt.Errorf("No such policy with ID: %s", link.ID)
			}
		} else {
			return 0, fmt.Errorf("Encountered a Token with policies linked by Name in the state store")
		}
	}
	return numValid, nil
}

// fixupTokenPolicyLinks is to be used when retrieving tokens from memdb. The policy links could have gotten
// stale when a linked policy was deleted or renamed. This will correct them and generate a newly allocated
// token only when fixes are needed. If the policy links are still accurate then we just return the original
// token.
func (s *Store) fixupTokenPolicyLinks(tx *memdb.Txn, original *structs.ACLToken) (*structs.ACLToken, error) {
	owned := false
	token := original

	cloneToken := func(t *structs.ACLToken, copyNumLinks int) *structs.ACLToken {
		clone := *t
		clone.Policies = make([]structs.ACLTokenPolicyLink, copyNumLinks)
		copy(clone.Policies, t.Policies[:copyNumLinks])
		return &clone
	}

	for linkIndex, link := range original.Policies {
		if link.ID == "" {
			return nil, fmt.Errorf("Detected corrupted token within the state store - missing policy link ID")
		}

		policy, err := s.getPolicyWithTxn(tx, nil, link.ID, s.aclPolicyGetByID, &token.EnterpriseMeta)

		if err != nil {
			return nil, err
		}

		if policy == nil {
			if !owned {
				// clone the token as we cannot touch the original
				token = cloneToken(original, linkIndex)
				owned = true
			}
			// if already owned then we just don't append it.
		} else if policy.Name != link.Name {
			if !owned {
				token = cloneToken(original, linkIndex)
				owned = true
			}

			// append the corrected policy
			token.Policies = append(token.Policies, structs.ACLTokenPolicyLink{ID: link.ID, Name: policy.Name})

		} else if owned {
			token.Policies = append(token.Policies, link)
		}
	}

	return token, nil
}

func (s *Store) resolveTokenRoleLinks(tx *memdb.Txn, token *structs.ACLToken, allowMissing bool) (int, error) {
	var numValid int
	for linkIndex, link := range token.Roles {
		if link.ID != "" {
			role, err := s.getRoleWithTxn(tx, nil, link.ID, s.aclRoleGetByID, &token.EnterpriseMeta)

			if err != nil {
				return 0, err
			}

			if role != nil {
				// the name doesn't matter here
				token.Roles[linkIndex].Name = role.Name
				numValid++
			} else if !allowMissing {
				return 0, fmt.Errorf("No such role with ID: %s", link.ID)
			}
		} else {
			return 0, fmt.Errorf("Encountered a Token with roles linked by Name in the state store")
		}
	}
	return numValid, nil
}

// fixupTokenRoleLinks is to be used when retrieving tokens from memdb. The role links could have gotten
// stale when a linked role was deleted or renamed. This will correct them and generate a newly allocated
// token only when fixes are needed. If the role links are still accurate then we just return the original
// token.
func (s *Store) fixupTokenRoleLinks(tx *memdb.Txn, original *structs.ACLToken) (*structs.ACLToken, error) {
	owned := false
	token := original

	cloneToken := func(t *structs.ACLToken, copyNumLinks int) *structs.ACLToken {
		clone := *t
		clone.Roles = make([]structs.ACLTokenRoleLink, copyNumLinks)
		copy(clone.Roles, t.Roles[:copyNumLinks])
		return &clone
	}

	for linkIndex, link := range original.Roles {
		if link.ID == "" {
			return nil, fmt.Errorf("Detected corrupted token within the state store - missing role link ID")
		}

		role, err := s.getRoleWithTxn(tx, nil, link.ID, s.aclRoleGetByID, &original.EnterpriseMeta)

		if err != nil {
			return nil, err
		}

		if role == nil {
			if !owned {
				// clone the token as we cannot touch the original
				token = cloneToken(original, linkIndex)
				owned = true
			}
			// if already owned then we just don't append it.
		} else if role.Name != link.Name {
			if !owned {
				token = cloneToken(original, linkIndex)
				owned = true
			}

			// append the corrected policy
			token.Roles = append(token.Roles, structs.ACLTokenRoleLink{ID: link.ID, Name: role.Name})

		} else if owned {
			token.Roles = append(token.Roles, link)
		}
	}

	return token, nil
}

func (s *Store) resolveRolePolicyLinks(tx *memdb.Txn, role *structs.ACLRole, allowMissing bool) error {
	for linkIndex, link := range role.Policies {
		if link.ID != "" {
			policy, err := s.getPolicyWithTxn(tx, nil, link.ID, s.aclPolicyGetByID, &role.EnterpriseMeta)

			if err != nil {
				return err
			}

			if policy != nil {
				// the name doesn't matter here
				role.Policies[linkIndex].Name = policy.Name
			} else if !allowMissing {
				return fmt.Errorf("No such policy with ID: %s", link.ID)
			}
		} else {
			return fmt.Errorf("Encountered a Role with policies linked by Name in the state store")
		}
	}
	return nil
}

// fixupRolePolicyLinks is to be used when retrieving roles from memdb. The policy links could have gotten
// stale when a linked policy was deleted or renamed. This will correct them and generate a newly allocated
// role only when fixes are needed. If the policy links are still accurate then we just return the original
// role.
func (s *Store) fixupRolePolicyLinks(tx *memdb.Txn, original *structs.ACLRole) (*structs.ACLRole, error) {
	owned := false
	role := original

	cloneRole := func(t *structs.ACLRole, copyNumLinks int) *structs.ACLRole {
		clone := *t
		clone.Policies = make([]structs.ACLRolePolicyLink, copyNumLinks)
		copy(clone.Policies, t.Policies[:copyNumLinks])
		return &clone
	}

	for linkIndex, link := range original.Policies {
		if link.ID == "" {
			return nil, fmt.Errorf("Detected corrupted role within the state store - missing policy link ID")
		}

		policy, err := s.getPolicyWithTxn(tx, nil, link.ID, s.aclPolicyGetByID, &original.EnterpriseMeta)

		if err != nil {
			return nil, err
		}

		if policy == nil {
			if !owned {
				// clone the token as we cannot touch the original
				role = cloneRole(original, linkIndex)
				owned = true
			}
			// if already owned then we just don't append it.
		} else if policy.Name != link.Name {
			if !owned {
				role = cloneRole(original, linkIndex)
				owned = true
			}

			// append the corrected policy
			role.Policies = append(role.Policies, structs.ACLRolePolicyLink{ID: link.ID, Name: policy.Name})

		} else if owned {
			role.Policies = append(role.Policies, link)
		}
	}

	return role, nil
}

// ACLTokenSet is used to insert an ACL rule into the state store.
func (s *Store) ACLTokenSet(idx uint64, token *structs.ACLToken, legacy bool) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call set on the ACL
	if err := s.aclTokenSetTxn(tx, idx, token, false, false, false, legacy); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) ACLTokenBatchSet(idx uint64, tokens structs.ACLTokens, cas, allowMissingPolicyAndRoleIDs, prohibitUnprivileged bool) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, token := range tokens {
		if err := s.aclTokenSetTxn(tx, idx, token, cas, allowMissingPolicyAndRoleIDs, prohibitUnprivileged, false); err != nil {
			return err
		}
	}

	tx.Commit()
	return nil
}

// aclTokenSetTxn is the inner method used to insert an ACL token with the
// proper indexes into the state store.
func (s *Store) aclTokenSetTxn(tx *memdb.Txn, idx uint64, token *structs.ACLToken, cas, allowMissingPolicyAndRoleIDs, prohibitUnprivileged, legacy bool) error {
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
	_, existing, err := s.aclTokenGetFromIndex(tx, token.SecretID, "id", nil)
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
		if original.UsesNonLegacyFields() {
			return fmt.Errorf("failed inserting acl token: cannot use legacy endpoint to modify a non-legacy token")
		}

		token.AccessorID = original.AccessorID
	}

	if err := s.aclTokenUpsertValidateEnterprise(tx, token, original); err != nil {
		return err
	}

	var numValidPolicies int
	if numValidPolicies, err = s.resolveTokenPolicyLinks(tx, token, allowMissingPolicyAndRoleIDs); err != nil {
		return err
	}

	var numValidRoles int
	if numValidRoles, err = s.resolveTokenRoleLinks(tx, token, allowMissingPolicyAndRoleIDs); err != nil {
		return err
	}

	if token.AuthMethod != "" {
		method, err := s.getAuthMethodWithTxn(tx, nil, token.AuthMethod, token.ACLAuthMethodEnterpriseMeta.ToEnterpriseMeta())
		if err != nil {
			return err
		} else if method == nil {
			return fmt.Errorf("No such auth method with Name: %s", token.AuthMethod)
		}
	}

	for _, svcid := range token.ServiceIdentities {
		if svcid.ServiceName == "" {
			return fmt.Errorf("Encountered a Token with an empty service identity name in the state store")
		}
	}

	if prohibitUnprivileged {
		if numValidRoles == 0 && numValidPolicies == 0 && len(token.ServiceIdentities) == 0 {
			return ErrTokenHasNoPrivileges
		}
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

	return s.aclTokenInsert(tx, token)
}

// ACLTokenGetBySecret is used to look up an existing ACL token by its SecretID.
func (s *Store) ACLTokenGetBySecret(ws memdb.WatchSet, secret string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLToken, error) {
	return s.aclTokenGet(ws, secret, "id", entMeta)
}

// ACLTokenGetByAccessor is used to look up an existing ACL token by its AccessorID.
func (s *Store) ACLTokenGetByAccessor(ws memdb.WatchSet, accessor string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLToken, error) {
	return s.aclTokenGet(ws, accessor, "accessor", entMeta)
}

// aclTokenGet looks up a token using one of the indexes provided
func (s *Store) aclTokenGet(ws memdb.WatchSet, value, index string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLToken, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	token, err := s.aclTokenGetTxn(tx, ws, value, index, entMeta)
	if err != nil {
		return 0, nil, err
	}

	idx := s.aclTokenMaxIndex(tx, token, entMeta)
	return idx, token, nil
}

func (s *Store) ACLTokenBatchGet(ws memdb.WatchSet, accessors []string) (uint64, structs.ACLTokens, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	tokens := make(structs.ACLTokens, 0)
	for _, accessor := range accessors {
		token, err := s.aclTokenGetTxn(tx, ws, accessor, "accessor", nil)
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

func (s *Store) aclTokenGetTxn(tx *memdb.Txn, ws memdb.WatchSet, value, index string, entMeta *structs.EnterpriseMeta) (*structs.ACLToken, error) {
	watchCh, rawToken, err := s.aclTokenGetFromIndex(tx, value, index, entMeta)
	if err != nil {
		return nil, fmt.Errorf("failed acl token lookup: %v", err)
	}
	ws.Add(watchCh)

	if rawToken != nil {
		token := rawToken.(*structs.ACLToken)
		token, err := s.fixupTokenPolicyLinks(tx, token)
		if err != nil {
			return nil, err
		}
		token, err = s.fixupTokenRoleLinks(tx, token)
		if err != nil {
			return nil, err
		}
		return token, nil
	}

	return nil, nil
}

// ACLTokenList is used to list out all of the ACLs in the state store.
func (s *Store) ACLTokenList(ws memdb.WatchSet, local, global bool, policy, role, methodName string, methodMeta, entMeta *structs.EnterpriseMeta) (uint64, structs.ACLTokens, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var iter memdb.ResultIterator
	var err error

	// Note global == local works when both are true or false. It is not valid to set both
	// to false but for defaulted structs (zero values for both) we want it to list out
	// all tokens so our checks just ensure that global == local

	needLocalityFilter := false
	if policy == "" && role == "" && methodName == "" {
		if global == local {
			iter, err = s.aclTokenListAll(tx, entMeta)
		} else if global {
			iter, err = s.aclTokenListGlobal(tx, entMeta)
		} else {
			iter, err = s.aclTokenListLocal(tx, entMeta)
		}

	} else if policy != "" && role == "" && methodName == "" {
		iter, err = s.aclTokenListByPolicy(tx, policy, entMeta)
		needLocalityFilter = true

	} else if policy == "" && role != "" && methodName == "" {
		iter, err = s.aclTokenListByRole(tx, role, entMeta)
		needLocalityFilter = true

	} else if policy == "" && role == "" && methodName != "" {
		iter, err = s.aclTokenListByAuthMethod(tx, methodName, methodMeta, entMeta)
		needLocalityFilter = true

	} else {
		return 0, nil, fmt.Errorf("can only filter by one of policy, role, or methodName at a time")
	}

	if err != nil {
		return 0, nil, fmt.Errorf("failed acl token lookup: %v", err)
	}

	if needLocalityFilter && global != local {
		iter = memdb.NewFilterIterator(iter, func(raw interface{}) bool {
			token, ok := raw.(*structs.ACLToken)
			if !ok {
				return true
			}

			if global && !token.Local {
				return false
			} else if local && token.Local {
				return false
			}

			return true
		})
	}

	ws.Add(iter.WatchCh())

	var result structs.ACLTokens
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		token := raw.(*structs.ACLToken)
		token, err := s.fixupTokenPolicyLinks(tx, token)
		if err != nil {
			return 0, nil, err
		}
		token, err = s.fixupTokenRoleLinks(tx, token)
		if err != nil {
			return 0, nil, err
		}
		result = append(result, token)
	}

	// Get the table index.
	idx := s.aclTokenMaxIndex(tx, nil, entMeta)

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

func (s *Store) ACLTokenMinExpirationTime(local bool) (time.Time, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	item, err := tx.First("acl-tokens", s.expiresIndexName(local))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed acl token listing: %v", err)
	}

	if item == nil {
		return time.Time{}, nil
	}

	token := item.(*structs.ACLToken)

	return *token.ExpirationTime, nil
}

// ACLTokenListExpires lists tokens that are expired as of the provided time.
// The returned set will be no larger than the max value provided.
func (s *Store) ACLTokenListExpired(local bool, asOf time.Time, max int) (structs.ACLTokens, <-chan struct{}, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := tx.Get("acl-tokens", s.expiresIndexName(local))
	if err != nil {
		return nil, nil, fmt.Errorf("failed acl token listing: %v", err)
	}

	var (
		tokens structs.ACLTokens
		i      int
	)
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		token := raw.(*structs.ACLToken)
		if token.ExpirationTime != nil && !token.ExpirationTime.Before(asOf) {
			return tokens, nil, nil
		}

		tokens = append(tokens, token)
		i += 1
		if i >= max {
			return tokens, nil, nil
		}
	}

	return tokens, iter.WatchCh(), nil
}

func (s *Store) expiresIndexName(local bool) string {
	if local {
		return "expires-local"
	}
	return "expires-global"
}

// ACLTokenDeleteBySecret is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *Store) ACLTokenDeleteBySecret(idx uint64, secret string, entMeta *structs.EnterpriseMeta) error {
	return s.aclTokenDelete(idx, secret, "id", entMeta)
}

// ACLTokenDeleteByAccessor is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *Store) ACLTokenDeleteByAccessor(idx uint64, accessor string, entMeta *structs.EnterpriseMeta) error {
	return s.aclTokenDelete(idx, accessor, "accessor", entMeta)
}

func (s *Store) ACLTokenBatchDelete(idx uint64, tokenIDs []string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, tokenID := range tokenIDs {
		if err := s.aclTokenDeleteTxn(tx, idx, tokenID, "accessor", nil); err != nil {
			return err
		}
	}

	tx.Commit()
	return nil
}

func (s *Store) aclTokenDelete(idx uint64, value, index string, entMeta *structs.EnterpriseMeta) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclTokenDeleteTxn(tx, idx, value, index, entMeta); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) aclTokenDeleteTxn(tx *memdb.Txn, idx uint64, value, index string, entMeta *structs.EnterpriseMeta) error {
	// Look up the existing token
	_, token, err := s.aclTokenGetFromIndex(tx, value, index, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl token lookup: %v", err)
	}

	if token == nil {
		return nil
	}

	if token.(*structs.ACLToken).AccessorID == structs.ACLTokenAnonymousID {
		return fmt.Errorf("Deletion of the builtin anonymous token is not permitted")
	}

	return s.aclTokenDeleteWithToken(tx, token.(*structs.ACLToken), idx)
}

func (s *Store) aclTokenDeleteAllForAuthMethodTxn(tx *memdb.Txn, idx uint64, methodName string, methodMeta *structs.EnterpriseMeta) error {
	// collect all the tokens linked with the given auth method.
	iter, err := s.aclTokenListByAuthMethod(tx, methodName, methodMeta, structs.WildcardEnterpriseMeta())
	if err != nil {
		return fmt.Errorf("failed acl token lookup: %v", err)
	}

	var tokens structs.ACLTokens
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		token := raw.(*structs.ACLToken)
		tokens = append(tokens, token)
	}

	if len(tokens) > 0 {
		// delete them all
		for _, token := range tokens {
			if err := s.aclTokenDeleteWithToken(tx, token, idx); err != nil {
				return err
			}
		}
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

	tx.Commit()
	return nil
}

func (s *Store) ACLPolicySet(idx uint64, policy *structs.ACLPolicy) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclPolicySetTxn(tx, idx, policy); err != nil {
		return err
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

	var existing *structs.ACLPolicy
	_, existingRaw, err := s.aclPolicyGetByID(tx, policy.ID, nil)
	if err != nil {
		return err
	}

	if existingRaw != nil {
		existing = existingRaw.(*structs.ACLPolicy)
	}

	if existing != nil {
		if policy.ID == structs.ACLPolicyGlobalManagementID {
			// Only the name and description are modifiable
			// Here we specifically check that the rules on the global management policy
			// are identical to the correct policy rules within the binary. This is opposed
			// to checking against the current rules to allow us to update the rules during
			// upgrades.
			if policy.Rules != structs.ACLPolicyGlobalManagement {
				return fmt.Errorf("Changing the Rules for the builtin global-management policy is not permitted")
			}

			if policy.Datacenters != nil && len(policy.Datacenters) != 0 {
				return fmt.Errorf("Changing the Datacenters of the builtin global-management policy is not permitted")
			}
		}
	}

	// ensure the name is unique (cannot conflict with another policy with a different ID)
	_, nameMatch, err := s.aclPolicyGetByName(tx, policy.Name, &policy.EnterpriseMeta)
	if err != nil {
		return err
	}
	if nameMatch != nil && policy.ID != nameMatch.(*structs.ACLPolicy).ID {
		return fmt.Errorf("A policy with name %q already exists", policy.Name)
	}

	if err := s.aclPolicyUpsertValidateEnterprise(tx, policy, existing); err != nil {
		return err
	}

	// Set the indexes
	if existing != nil {
		policy.CreateIndex = existing.CreateIndex
		policy.ModifyIndex = idx
	} else {
		policy.CreateIndex = idx
		policy.ModifyIndex = idx
	}

	// Insert the ACL
	return s.aclPolicyInsert(tx, policy)
}

func (s *Store) ACLPolicyGetByID(ws memdb.WatchSet, id string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLPolicy, error) {
	return s.aclPolicyGet(ws, id, s.aclPolicyGetByID, entMeta)
}

func (s *Store) ACLPolicyGetByName(ws memdb.WatchSet, name string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLPolicy, error) {
	return s.aclPolicyGet(ws, name, s.aclPolicyGetByName, entMeta)
}

func (s *Store) ACLPolicyBatchGet(ws memdb.WatchSet, ids []string) (uint64, structs.ACLPolicies, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	policies := make(structs.ACLPolicies, 0)
	for _, pid := range ids {
		policy, err := s.getPolicyWithTxn(tx, ws, pid, s.aclPolicyGetByID, nil)
		if err != nil {
			return 0, nil, err
		}

		if policy != nil {
			policies = append(policies, policy)
		}
	}

	// We are specifically not wanting to call aclPolicyMaxIndex here as we always want the
	// index entry for the "acl-policies" table.
	idx := maxIndexTxn(tx, "acl-policies")

	return idx, policies, nil
}

type aclPolicyGetFn func(*memdb.Txn, string, *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error)

func (s *Store) getPolicyWithTxn(tx *memdb.Txn, ws memdb.WatchSet, value string, fn aclPolicyGetFn, entMeta *structs.EnterpriseMeta) (*structs.ACLPolicy, error) {
	watchCh, policy, err := fn(tx, value, entMeta)
	if err != nil {
		return nil, fmt.Errorf("failed acl policy lookup: %v", err)
	}
	ws.Add(watchCh)

	if policy == nil {
		return nil, err
	}

	return policy.(*structs.ACLPolicy), nil
}

func (s *Store) aclPolicyGet(ws memdb.WatchSet, value string, fn aclPolicyGetFn, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLPolicy, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	policy, err := s.getPolicyWithTxn(tx, ws, value, fn, entMeta)
	if err != nil {
		return 0, nil, err
	}

	idx := s.aclPolicyMaxIndex(tx, policy, entMeta)

	return idx, policy, nil
}

func (s *Store) ACLPolicyList(ws memdb.WatchSet, entMeta *structs.EnterpriseMeta) (uint64, structs.ACLPolicies, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := s.aclPolicyList(tx, entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl policy lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLPolicies
	for policy := iter.Next(); policy != nil; policy = iter.Next() {
		result = append(result, policy.(*structs.ACLPolicy))
	}

	// Get the table index.
	idx := s.aclPolicyMaxIndex(tx, nil, entMeta)

	return idx, result, nil
}

func (s *Store) ACLPolicyDeleteByID(idx uint64, id string, entMeta *structs.EnterpriseMeta) error {
	return s.aclPolicyDelete(idx, id, s.aclPolicyGetByID, entMeta)
}

func (s *Store) ACLPolicyDeleteByName(idx uint64, name string, entMeta *structs.EnterpriseMeta) error {
	return s.aclPolicyDelete(idx, name, s.aclPolicyGetByName, entMeta)
}

func (s *Store) ACLPolicyBatchDelete(idx uint64, policyIDs []string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, policyID := range policyIDs {
		if err := s.aclPolicyDeleteTxn(tx, idx, policyID, s.aclPolicyGetByID, nil); err != nil {
			return err
		}
	}
	tx.Commit()
	return nil
}

func (s *Store) aclPolicyDelete(idx uint64, value string, fn aclPolicyGetFn, entMeta *structs.EnterpriseMeta) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclPolicyDeleteTxn(tx, idx, value, fn, entMeta); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) aclPolicyDeleteTxn(tx *memdb.Txn, idx uint64, value string, fn aclPolicyGetFn, entMeta *structs.EnterpriseMeta) error {
	// Look up the existing token
	_, rawPolicy, err := fn(tx, value, entMeta)
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

	return s.aclPolicyDeleteWithPolicy(tx, policy, idx)
}

func (s *Store) ACLRoleBatchSet(idx uint64, roles structs.ACLRoles, allowMissingPolicyIDs bool) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, role := range roles {
		if err := s.aclRoleSetTxn(tx, idx, role, allowMissingPolicyIDs); err != nil {
			return err
		}
	}

	tx.Commit()
	return nil
}

func (s *Store) ACLRoleSet(idx uint64, role *structs.ACLRole) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclRoleSetTxn(tx, idx, role, false); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) aclRoleSetTxn(tx *memdb.Txn, idx uint64, role *structs.ACLRole, allowMissing bool) error {
	// Check that the ID is set
	if role.ID == "" {
		return ErrMissingACLRoleID
	}

	if role.Name == "" {
		return ErrMissingACLRoleName
	}

	_, existingRaw, err := s.aclRoleGetByID(tx, role.ID, nil)
	if err != nil {
		return fmt.Errorf("failed acl role lookup: %v", err)
	}

	var existing *structs.ACLRole
	if existingRaw != nil {
		existing = existingRaw.(*structs.ACLRole)
	}

	// ensure the name is unique (cannot conflict with another role with a different ID)
	_, nameMatch, err := s.aclRoleGetByName(tx, role.Name, &role.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("failed acl role lookup: %v", err)
	}
	if nameMatch != nil && role.ID != nameMatch.(*structs.ACLRole).ID {
		return fmt.Errorf("A role with name %q already exists", role.Name)
	}

	if err := s.resolveRolePolicyLinks(tx, role, allowMissing); err != nil {
		return err
	}

	for _, svcid := range role.ServiceIdentities {
		if svcid.ServiceName == "" {
			return fmt.Errorf("Encountered a Role with an empty service identity name in the state store")
		}
	}

	if err := s.aclRoleUpsertValidateEnterprise(tx, role, existing); err != nil {
		return err
	}

	// Set the indexes
	if existing != nil {
		role.CreateIndex = existing.CreateIndex
		role.ModifyIndex = idx
	} else {
		role.CreateIndex = idx
		role.ModifyIndex = idx
	}

	return s.aclRoleInsert(tx, role)
}

type aclRoleGetFn func(*memdb.Txn, string, *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error)

func (s *Store) ACLRoleGetByID(ws memdb.WatchSet, id string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLRole, error) {
	return s.aclRoleGet(ws, id, s.aclRoleGetByID, entMeta)
}

func (s *Store) ACLRoleGetByName(ws memdb.WatchSet, name string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLRole, error) {
	return s.aclRoleGet(ws, name, s.aclRoleGetByName, entMeta)
}

func (s *Store) ACLRoleBatchGet(ws memdb.WatchSet, ids []string) (uint64, structs.ACLRoles, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	roles := make(structs.ACLRoles, 0, len(ids))
	for _, rid := range ids {
		role, err := s.getRoleWithTxn(tx, ws, rid, s.aclRoleGetByID, nil)
		if err != nil {
			return 0, nil, err
		}

		if role != nil {
			roles = append(roles, role)
		}
	}

	idx := maxIndexTxn(tx, "acl-roles")

	return idx, roles, nil
}

func (s *Store) getRoleWithTxn(tx *memdb.Txn, ws memdb.WatchSet, value string, fn aclRoleGetFn, entMeta *structs.EnterpriseMeta) (*structs.ACLRole, error) {
	watchCh, rawRole, err := fn(tx, value, entMeta)
	if err != nil {
		return nil, fmt.Errorf("failed acl role lookup: %v", err)
	}
	ws.Add(watchCh)

	if rawRole != nil {
		role := rawRole.(*structs.ACLRole)
		role, err := s.fixupRolePolicyLinks(tx, role)
		if err != nil {
			return nil, err
		}
		return role, nil
	}

	return nil, nil
}

func (s *Store) aclRoleGet(ws memdb.WatchSet, value string, fn aclRoleGetFn, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLRole, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	role, err := s.getRoleWithTxn(tx, ws, value, fn, entMeta)
	if err != nil {
		return 0, nil, err
	}

	idx := s.aclRoleMaxIndex(tx, role, entMeta)

	return idx, role, nil
}

func (s *Store) ACLRoleList(ws memdb.WatchSet, policy string, entMeta *structs.EnterpriseMeta) (uint64, structs.ACLRoles, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var iter memdb.ResultIterator
	var err error

	if policy != "" {
		iter, err = s.aclRoleListByPolicy(tx, policy, entMeta)
	} else {
		iter, err = s.aclRoleList(tx, entMeta)
	}

	if err != nil {
		return 0, nil, fmt.Errorf("failed acl role lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLRoles
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		role := raw.(*structs.ACLRole)
		role, err := s.fixupRolePolicyLinks(tx, role)
		if err != nil {
			return 0, nil, err
		}
		result = append(result, role)
	}

	// Get the table index.
	idx := s.aclRoleMaxIndex(tx, nil, entMeta)

	return idx, result, nil
}

func (s *Store) ACLRoleDeleteByID(idx uint64, id string, entMeta *structs.EnterpriseMeta) error {
	return s.aclRoleDelete(idx, id, s.aclRoleGetByID, entMeta)
}

func (s *Store) ACLRoleDeleteByName(idx uint64, name string, entMeta *structs.EnterpriseMeta) error {
	return s.aclRoleDelete(idx, name, s.aclRoleGetByName, entMeta)
}

func (s *Store) ACLRoleBatchDelete(idx uint64, roleIDs []string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, roleID := range roleIDs {
		if err := s.aclRoleDeleteTxn(tx, idx, roleID, s.aclRoleGetByID, nil); err != nil {
			return err
		}
	}
	tx.Commit()
	return nil
}

func (s *Store) aclRoleDelete(idx uint64, value string, fn aclRoleGetFn, entMeta *structs.EnterpriseMeta) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclRoleDeleteTxn(tx, idx, value, fn, entMeta); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) aclRoleDeleteTxn(tx *memdb.Txn, idx uint64, value string, fn aclRoleGetFn, entMeta *structs.EnterpriseMeta) error {
	// Look up the existing role
	_, rawRole, err := fn(tx, value, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl role lookup: %v", err)
	}

	if rawRole == nil {
		return nil
	}

	role := rawRole.(*structs.ACLRole)

	return s.aclRoleDeleteWithRole(tx, role, idx)
}

func (s *Store) ACLBindingRuleBatchSet(idx uint64, rules structs.ACLBindingRules) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, rule := range rules {
		if err := s.aclBindingRuleSetTxn(tx, idx, rule); err != nil {
			return err
		}
	}

	tx.Commit()
	return nil
}

func (s *Store) ACLBindingRuleSet(idx uint64, rule *structs.ACLBindingRule) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclBindingRuleSetTxn(tx, idx, rule); err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *Store) aclBindingRuleSetTxn(tx *memdb.Txn, idx uint64, rule *structs.ACLBindingRule) error {
	// Check that the ID and AuthMethod are set
	if rule.ID == "" {
		return ErrMissingACLBindingRuleID
	} else if rule.AuthMethod == "" {
		return ErrMissingACLBindingRuleAuthMethod
	}

	var existing *structs.ACLBindingRule
	_, existingRaw, err := s.aclBindingRuleGetByID(tx, rule.ID, nil)
	if err != nil {
		return fmt.Errorf("failed acl binding rule lookup: %v", err)
	}

	// Set the indexes
	if existingRaw != nil {
		existing = existingRaw.(*structs.ACLBindingRule)
		rule.CreateIndex = existing.CreateIndex
		rule.ModifyIndex = idx
	} else {
		rule.CreateIndex = idx
		rule.ModifyIndex = idx
	}

	if err := s.aclBindingRuleUpsertValidateEnterprise(tx, rule, existing); err != nil {
		return err
	}

	if _, method, err := s.aclAuthMethodGetByName(tx, rule.AuthMethod, &rule.EnterpriseMeta); err != nil {
		return fmt.Errorf("failed acl auth method lookup: %v", err)
	} else if method == nil {
		return fmt.Errorf("failed inserting acl binding rule: auth method not found")
	}

	return s.aclBindingRuleInsert(tx, rule)
}

func (s *Store) ACLBindingRuleGetByID(ws memdb.WatchSet, id string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLBindingRule, error) {
	return s.aclBindingRuleGet(ws, id, entMeta)
}

func (s *Store) aclBindingRuleGet(ws memdb.WatchSet, value string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLBindingRule, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	watchCh, rawRule, err := s.aclBindingRuleGetByID(tx, value, entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl binding rule lookup: %v", err)
	}
	ws.Add(watchCh)

	var rule *structs.ACLBindingRule
	if rawRule != nil {
		rule = rawRule.(*structs.ACLBindingRule)
	}

	idx := s.aclBindingRuleMaxIndex(tx, rule, entMeta)

	return idx, rule, nil
}

func (s *Store) ACLBindingRuleList(ws memdb.WatchSet, methodName string, entMeta *structs.EnterpriseMeta) (uint64, structs.ACLBindingRules, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var (
		iter memdb.ResultIterator
		err  error
	)
	if methodName != "" {
		iter, err = s.aclBindingRuleListByAuthMethod(tx, methodName, entMeta)
	} else {
		iter, err = s.aclBindingRuleList(tx, entMeta)
	}
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl binding rule lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLBindingRules
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		rule := raw.(*structs.ACLBindingRule)
		result = append(result, rule)
	}

	// Get the table index.
	idx := s.aclBindingRuleMaxIndex(tx, nil, entMeta)

	return idx, result, nil
}

func (s *Store) ACLBindingRuleDeleteByID(idx uint64, id string, entMeta *structs.EnterpriseMeta) error {
	return s.aclBindingRuleDelete(idx, id, entMeta)
}

func (s *Store) ACLBindingRuleBatchDelete(idx uint64, bindingRuleIDs []string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, bindingRuleID := range bindingRuleIDs {
		s.aclBindingRuleDeleteTxn(tx, idx, bindingRuleID, nil)
	}
	tx.Commit()
	return nil
}

func (s *Store) aclBindingRuleDelete(idx uint64, id string, entMeta *structs.EnterpriseMeta) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclBindingRuleDeleteTxn(tx, idx, id, entMeta); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) aclBindingRuleDeleteTxn(tx *memdb.Txn, idx uint64, id string, entMeta *structs.EnterpriseMeta) error {
	// Look up the existing binding rule
	_, rawRule, err := s.aclBindingRuleGetByID(tx, id, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl binding rule lookup: %v", err)
	}

	if rawRule == nil {
		return nil
	}

	rule := rawRule.(*structs.ACLBindingRule)

	if err := s.aclBindingRuleDeleteWithRule(tx, rule, idx); err != nil {
		return fmt.Errorf("failed deleting acl binding rule: %v", err)
	}
	return nil
}

func (s *Store) aclBindingRuleDeleteAllForAuthMethodTxn(tx *memdb.Txn, idx uint64, methodName string, entMeta *structs.EnterpriseMeta) error {
	// collect them all
	iter, err := s.aclBindingRuleListByAuthMethod(tx, methodName, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl binding rule lookup: %v", err)
	}

	var rules structs.ACLBindingRules
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		rule := raw.(*structs.ACLBindingRule)
		rules = append(rules, rule)
	}

	if len(rules) > 0 {
		// delete them all
		for _, rule := range rules {
			if err := s.aclBindingRuleDeleteWithRule(tx, rule, idx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Store) ACLAuthMethodBatchSet(idx uint64, methods structs.ACLAuthMethods) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, method := range methods {
		// this is only used when doing batch insertions for upgrades and replication. Therefore
		// we take whatever those said.
		if err := s.aclAuthMethodSetTxn(tx, idx, method); err != nil {
			return err
		}
	}
	tx.Commit()
	return nil
}

func (s *Store) ACLAuthMethodSet(idx uint64, method *structs.ACLAuthMethod) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclAuthMethodSetTxn(tx, idx, method); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) aclAuthMethodSetTxn(tx *memdb.Txn, idx uint64, method *structs.ACLAuthMethod) error {
	// Check that the Name and Type are set
	if method.Name == "" {
		return ErrMissingACLAuthMethodName
	} else if method.Type == "" {
		return ErrMissingACLAuthMethodType
	}

	var existing *structs.ACLAuthMethod
	_, existingRaw, err := s.aclAuthMethodGetByName(tx, method.Name, &method.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("failed acl auth method lookup: %v", err)
	}

	if err := s.aclAuthMethodUpsertValidateEnterprise(tx, method, existing); err != nil {
		return err
	}

	// Set the indexes
	if existingRaw != nil {
		existing = existingRaw.(*structs.ACLAuthMethod)
		method.CreateIndex = existing.CreateIndex
		method.ModifyIndex = idx
	} else {
		method.CreateIndex = idx
		method.ModifyIndex = idx
	}

	return s.aclAuthMethodInsert(tx, method)
}

func (s *Store) ACLAuthMethodGetByName(ws memdb.WatchSet, name string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLAuthMethod, error) {
	return s.aclAuthMethodGet(ws, name, entMeta)
}

func (s *Store) aclAuthMethodGet(ws memdb.WatchSet, name string, entMeta *structs.EnterpriseMeta) (uint64, *structs.ACLAuthMethod, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	method, err := s.getAuthMethodWithTxn(tx, ws, name, entMeta)
	if err != nil {
		return 0, nil, err
	}

	idx := s.aclAuthMethodMaxIndex(tx, method, entMeta)

	return idx, method, nil
}

func (s *Store) getAuthMethodWithTxn(tx *memdb.Txn, ws memdb.WatchSet, name string, entMeta *structs.EnterpriseMeta) (*structs.ACLAuthMethod, error) {
	watchCh, rawMethod, err := s.aclAuthMethodGetByName(tx, name, entMeta)
	if err != nil {
		return nil, fmt.Errorf("failed acl auth method lookup: %v", err)
	}
	ws.Add(watchCh)

	if rawMethod != nil {
		return rawMethod.(*structs.ACLAuthMethod), nil
	}

	return nil, nil
}

func (s *Store) ACLAuthMethodList(ws memdb.WatchSet, entMeta *structs.EnterpriseMeta) (uint64, structs.ACLAuthMethods, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := s.aclAuthMethodList(tx, entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl auth method lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLAuthMethods
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		method := raw.(*structs.ACLAuthMethod)
		result = append(result, method)
	}

	// Get the table index.
	idx := s.aclAuthMethodMaxIndex(tx, nil, entMeta)

	return idx, result, nil
}

func (s *Store) ACLAuthMethodDeleteByName(idx uint64, name string, entMeta *structs.EnterpriseMeta) error {
	return s.aclAuthMethodDelete(idx, name, entMeta)
}

func (s *Store) ACLAuthMethodBatchDelete(idx uint64, names []string, entMeta *structs.EnterpriseMeta) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, name := range names {
		// NOTE: it may seem odd that one EnterpriseMeta is being used for all of the auth methods being
		// deleted. However we never actually batch these deletions as auth methods are not replicated
		// Therefore this is fine but if we ever change that precondition then this will be wrong (unless
		// we ensure all deletions in a batch should have the same enterprise meta)
		s.aclAuthMethodDeleteTxn(tx, idx, name, entMeta)
	}

	tx.Commit()
	return nil
}

func (s *Store) aclAuthMethodDelete(idx uint64, name string, entMeta *structs.EnterpriseMeta) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.aclAuthMethodDeleteTxn(tx, idx, name, entMeta); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) aclAuthMethodDeleteTxn(tx *memdb.Txn, idx uint64, name string, entMeta *structs.EnterpriseMeta) error {
	// Look up the existing method
	_, rawMethod, err := s.aclAuthMethodGetByName(tx, name, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl auth method lookup: %v", err)
	}

	if rawMethod == nil {
		return nil
	}

	method := rawMethod.(*structs.ACLAuthMethod)

	if err := s.aclBindingRuleDeleteAllForAuthMethodTxn(tx, idx, method.Name, entMeta); err != nil {
		return err
	}

	if err := s.aclTokenDeleteAllForAuthMethodTxn(tx, idx, method.Name, entMeta); err != nil {
		return err
	}

	return s.aclAuthMethodDeleteWithMethod(tx, method, idx)
}
