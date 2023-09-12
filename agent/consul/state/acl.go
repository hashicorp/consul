package state

import (
	"fmt"
	"time"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	pbacl "github.com/hashicorp/consul/proto/pbacl"
)

// ACLTokens is used when saving a snapshot
func (s *Snapshot) ACLTokens() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableACLTokens, indexID)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// ACLToken is used when restoring from a snapshot. For general inserts, use ACL.
func (s *Restore) ACLToken(token *structs.ACLToken) error {
	return aclTokenInsert(s.tx, token)
}

// ACLPolicies is used when saving a snapshot
func (s *Snapshot) ACLPolicies() (memdb.ResultIterator, error) {
	return s.tx.Get(tableACLPolicies, indexID)
}

func (s *Restore) ACLPolicy(policy *structs.ACLPolicy) error {
	return aclPolicyInsert(s.tx, policy)
}

// ACLRoles is used when saving a snapshot
func (s *Snapshot) ACLRoles() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableACLRoles, indexID)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func (s *Restore) ACLRole(role *structs.ACLRole) error {
	return aclRoleInsert(s.tx, role)
}

// ACLBindingRules is used when saving a snapshot
func (s *Snapshot) ACLBindingRules() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableACLBindingRules, indexID)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func (s *Restore) ACLBindingRule(rule *structs.ACLBindingRule) error {
	return aclBindingRuleInsert(s.tx, rule)
}

// ACLAuthMethods is used when saving a snapshot
func (s *Snapshot) ACLAuthMethods() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableACLAuthMethods, indexID)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func (s *Restore) ACLAuthMethod(method *structs.ACLAuthMethod) error {
	return aclAuthMethodInsert(s.tx, method)
}

// ACLBootstrap is used to perform a one-time ACL bootstrap operation on a
// cluster to get the first management token.
func (s *Store) ACLBootstrap(idx, resetIndex uint64, token *structs.ACLToken) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// We must have initialized before this will ever be possible.
	existing, err := tx.First(tableIndex, indexID, "acl-token-bootstrap")
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

	if err := aclTokenSetTxn(tx, idx, token, ACLTokenSetOptions{}); err != nil {
		return fmt.Errorf("failed inserting bootstrap token: %v", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{"acl-token-bootstrap", idx}); err != nil {
		return fmt.Errorf("failed to mark ACL bootstrapping as complete: %v", err)
	}
	return tx.Commit()
}

// CanBootstrapACLToken checks if bootstrapping is possible and returns the reset index
func (s *Store) CanBootstrapACLToken() (bool, uint64, error) {
	tx := s.db.Txn(false)

	// Lookup the bootstrap sentinel
	out, err := tx.First(tableIndex, indexID, "acl-token-bootstrap")
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
func resolveACLLinks(tx ReadTxn, links []*pbacl.ACLLink, getName func(ReadTxn, string) (string, error)) (int, error) {
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
func fixupACLLinks(tx ReadTxn, original []*pbacl.ACLLink, getName func(ReadTxn, string) (string, error)) ([]*pbacl.ACLLink, bool, error) {
	owned := false
	links := original

	cloneLinks := func(l []*pbacl.ACLLink, copyNumLinks int) []*pbacl.ACLLink {
		clone := make([]*pbacl.ACLLink, copyNumLinks)
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
			links = append(links, &pbacl.ACLLink{ID: link.ID, Name: name})
		} else if owned {
			links = append(links, link)
		}
	}

	return links, owned, nil
}

func resolveTokenPolicyLinks(tx ReadTxn, token *structs.ACLToken, allowMissing bool) (int, error) {
	var numValid int
	for linkIndex, link := range token.Policies {
		if link.ID != "" {
			policy, err := getPolicyWithTxn(tx, nil, link.ID, aclPolicyGetByID, &token.EnterpriseMeta)

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
func fixupTokenPolicyLinks(tx ReadTxn, original *structs.ACLToken) (*structs.ACLToken, error) {
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

		policy, err := getPolicyWithTxn(tx, nil, link.ID, aclPolicyGetByID, &token.EnterpriseMeta)

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

func resolveTokenRoleLinks(tx ReadTxn, token *structs.ACLToken, allowMissing bool) (int, error) {
	var numValid int
	for linkIndex, link := range token.Roles {
		if link.ID != "" {
			role, err := getRoleWithTxn(tx, nil, link.ID, aclRoleGetByID, &token.EnterpriseMeta)

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
func fixupTokenRoleLinks(tx ReadTxn, original *structs.ACLToken) (*structs.ACLToken, error) {
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

		role, err := getRoleWithTxn(tx, nil, link.ID, aclRoleGetByID, &original.EnterpriseMeta)

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

func resolveRolePolicyLinks(tx ReadTxn, role *structs.ACLRole, allowMissing bool) error {
	for linkIndex, link := range role.Policies {
		if link.ID == "" {
			return fmt.Errorf("Encountered a Role with policies linked by Name in the state store")
		}

		policy, err := getPolicyWithTxn(tx, nil, link.ID, aclPolicyGetByID, &role.EnterpriseMeta)
		if err != nil {
			return err
		}

		if policy != nil {
			// the name doesn't matter here
			role.Policies[linkIndex].Name = policy.Name
		} else if !allowMissing {
			return fmt.Errorf("No such policy with ID: %s", link.ID)
		}
	}
	return nil
}

// fixupRolePolicyLinks is to be used when retrieving roles from memdb. The policy links could have gotten
// stale when a linked policy was deleted or renamed. This will correct them and generate a newly allocated
// role only when fixes are needed. If the policy links are still accurate then we just return the original
// role.
func fixupRolePolicyLinks(tx ReadTxn, original *structs.ACLRole) (*structs.ACLRole, error) {
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

		policy, err := getPolicyWithTxn(tx, nil, link.ID, aclPolicyGetByID, &original.EnterpriseMeta)

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

// ACLTokenSet is used in many tests to set a single ACL token. It is now a shim
// for calling ACLTokenBatchSet with default options.
func (s *Store) ACLTokenSet(idx uint64, token *structs.ACLToken) error {
	return s.ACLTokenBatchSet(idx, structs.ACLTokens{token}, ACLTokenSetOptions{})
}

type ACLTokenSetOptions struct {
	CAS                          bool
	AllowMissingPolicyAndRoleIDs bool
	ProhibitUnprivileged         bool
	Legacy                       bool // TODO(ACL-Legacy-Compat): remove
	FromReplication              bool
}

func (s *Store) ACLTokenBatchSet(idx uint64, tokens structs.ACLTokens, opts ACLTokenSetOptions) error {
	if opts.Legacy {
		return fmt.Errorf("failed inserting acl token: cannot use this endpoint to persist legacy tokens")
	}

	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, token := range tokens {
		if err := aclTokenSetTxn(tx, idx, token, opts); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// aclTokenSetTxn is the inner method used to insert an ACL token with the
// proper indexes into the state store.
func aclTokenSetTxn(tx WriteTxn, idx uint64, token *structs.ACLToken, opts ACLTokenSetOptions) error {
	// Check that the ID is set
	if token.SecretID == "" {
		return ErrMissingACLTokenSecret
	}

	if !opts.Legacy && token.AccessorID == "" {
		return ErrMissingACLTokenAccessor
	}

	if opts.FromReplication && token.Local {
		return fmt.Errorf("Cannot replicate local tokens")
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
	_, existing, err := aclTokenGetFromIndex(tx, token.SecretID, "id", nil)
	if err != nil {
		return fmt.Errorf("failed token lookup: %s", err)
	}

	var original *structs.ACLToken

	if existing != nil {
		original = existing.(*structs.ACLToken)
	}

	if opts.CAS {
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

	if opts.Legacy && original != nil {
		return fmt.Errorf("legacy tokens can not be modified")
	}

	if err := aclTokenUpsertValidateEnterprise(tx, token, original); err != nil {
		return err
	}

	var numValidPolicies int
	if numValidPolicies, err = resolveTokenPolicyLinks(tx, token, opts.AllowMissingPolicyAndRoleIDs); err != nil {
		return err
	}

	var numValidRoles int
	if numValidRoles, err = resolveTokenRoleLinks(tx, token, opts.AllowMissingPolicyAndRoleIDs); err != nil {
		return err
	}

	if token.AuthMethod != "" && !opts.FromReplication {
		methodMeta := token.ACLAuthMethodEnterpriseMeta.ToEnterpriseMeta()
		methodMeta.Merge(&token.EnterpriseMeta)
		method, err := getAuthMethodWithTxn(tx, nil, token.AuthMethod, methodMeta)
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

	for _, nodeid := range token.NodeIdentities {
		if nodeid.NodeName == "" {
			return fmt.Errorf("Encountered a Token with an empty node identity name in the state store")
		}
		if nodeid.Datacenter == "" {
			return fmt.Errorf("Encountered a Token with an empty node identity datacenter in the state store")
		}
	}

	if opts.ProhibitUnprivileged {
		if numValidRoles == 0 && numValidPolicies == 0 && len(token.ServiceIdentities) == 0 && len(token.NodeIdentities) == 0 {
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

	// ensure that a hash is set
	token.SetHash(false)

	return aclTokenInsert(tx, token)
}

// ACLTokenGetBySecret is used to look up an existing ACL token by its SecretID.
func (s *Store) ACLTokenGetBySecret(ws memdb.WatchSet, secret string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLToken, error) {
	return s.aclTokenGet(ws, secret, "id", entMeta)
}

// ACLTokenGetByAccessor is used to look up an existing ACL token by its AccessorID.
func (s *Store) ACLTokenGetByAccessor(ws memdb.WatchSet, accessor string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLToken, error) {
	return s.aclTokenGet(ws, accessor, indexAccessor, entMeta)
}

// aclTokenGet looks up a token using one of the indexes provided
func (s *Store) aclTokenGet(ws memdb.WatchSet, value, index string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLToken, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	token, err := aclTokenGetTxn(tx, ws, value, index, entMeta)
	if err != nil {
		return 0, nil, err
	}

	idx := aclTokenMaxIndex(tx, token, entMeta)
	return idx, token, nil
}

func (s *Store) ACLTokenBatchGet(ws memdb.WatchSet, accessors []string) (uint64, structs.ACLTokens, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	tokens := make(structs.ACLTokens, 0)
	for _, accessor := range accessors {
		token, err := aclTokenGetTxn(tx, ws, accessor, indexAccessor, nil)
		if err != nil {
			return 0, nil, fmt.Errorf("failed acl token lookup: %v", err)
		}

		// token == nil is valid and will indic
		if token != nil {
			tokens = append(tokens, token)
		}
	}

	idx := maxIndexTxn(tx, tableACLTokens)

	return idx, tokens, nil
}

func aclTokenGetTxn(tx ReadTxn, ws memdb.WatchSet, value, index string, entMeta *acl.EnterpriseMeta) (*structs.ACLToken, error) {
	watchCh, rawToken, err := aclTokenGetFromIndex(tx, value, index, entMeta)
	if err != nil {
		return nil, fmt.Errorf("failed acl token lookup: %v", err)
	}
	ws.Add(watchCh)

	if rawToken != nil {
		token := rawToken.(*structs.ACLToken)
		token, err := fixupTokenPolicyLinks(tx, token)
		if err != nil {
			return nil, err
		}
		token, err = fixupTokenRoleLinks(tx, token)
		if err != nil {
			return nil, err
		}
		return token, nil
	}

	return nil, nil
}

type ACLTokenListParameters struct {
	Local          bool
	Global         bool
	Policy         string
	Role           string
	ServiceName    string
	MethodName     string
	MethodMeta     *acl.EnterpriseMeta
	EnterpriseMeta *acl.EnterpriseMeta
}

// ACLTokenList return a list of ACL Tokens that match the policy, role, and method.
// This function should be treated as deprecated, and ACLTokenListWithParameters should be preferred.
//
// Deprecated: use ACLTokenListWithParameters
func (s *Store) ACLTokenList(ws memdb.WatchSet, local, global bool, policy, role, methodName string, methodMeta, entMeta *acl.EnterpriseMeta) (uint64, structs.ACLTokens, error) {
	return s.ACLTokenListWithParameters(ws, ACLTokenListParameters{
		Local:          local,
		Global:         global,
		Policy:         policy,
		Role:           role,
		MethodName:     methodName,
		MethodMeta:     methodMeta,
		EnterpriseMeta: entMeta,
	})
}

// ACLTokenListWithParameters returns a list of ACL Tokens that match the provided parameters.
func (s *Store) ACLTokenListWithParameters(ws memdb.WatchSet, params ACLTokenListParameters) (uint64, structs.ACLTokens, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var iter memdb.ResultIterator
	var err error

	// Note global == local works when both are true or false. It is not valid to set both
	// to false but for defaulted structs (zero values for both) we want it to list out
	// all tokens so our checks just ensure that global == local

	needLocalityFilter := false

	if params.Policy == "" && params.Role == "" && params.MethodName == "" && params.ServiceName == "" {
		if params.Global == params.Local {
			iter, err = aclTokenListAll(tx, params.EnterpriseMeta)
		} else {
			iter, err = aclTokenList(tx, params.EnterpriseMeta, params.Local)
		}

	} else if params.Policy != "" && params.Role == "" && params.MethodName == "" && params.ServiceName == "" {
		// Find by policy
		iter, err = aclTokenListByPolicy(tx, params.Policy, params.EnterpriseMeta)
		needLocalityFilter = true

	} else if params.Policy == "" && params.Role != "" && params.MethodName == "" && params.ServiceName == "" {
		// Find by role
		iter, err = aclTokenListByRole(tx, params.Role, params.EnterpriseMeta)
		needLocalityFilter = true

	} else if params.Policy == "" && params.Role == "" && params.MethodName != "" && params.ServiceName == "" {
		// Find by methodName
		iter, err = aclTokenListByAuthMethod(tx, params.MethodName, params.MethodMeta, params.EnterpriseMeta)
		needLocalityFilter = true

	} else if params.Policy == "" && params.Role == "" && params.MethodName == "" && params.ServiceName != "" {
		// Find by the service identity's serviceName
		iter, err = aclTokenListByServiceName(tx, params.ServiceName, params.EnterpriseMeta)
		needLocalityFilter = true

	} else {
		return 0, nil, fmt.Errorf("can only filter by one of policy, role, serviceName, or methodName at a time")
	}

	if err != nil {
		return 0, nil, fmt.Errorf("failed acl token lookup: %v", err)
	}

	if needLocalityFilter && params.Global != params.Local {
		iter = memdb.NewFilterIterator(iter, func(raw interface{}) bool {
			token, ok := raw.(*structs.ACLToken)
			if !ok {
				return true
			}

			if params.Global && !token.Local {
				return false
			} else if params.Local && token.Local {
				return false
			}

			return true
		})
	}

	ws.Add(iter.WatchCh())

	var result structs.ACLTokens
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		token := raw.(*structs.ACLToken)
		token, err := fixupTokenPolicyLinks(tx, token)
		if err != nil {
			return 0, nil, err
		}
		token, err = fixupTokenRoleLinks(tx, token)
		if err != nil {
			return 0, nil, err
		}
		result = append(result, token)
	}

	// Get the table index.
	idx := aclTokenMaxIndex(tx, nil, params.EnterpriseMeta)
	return idx, result, nil
}

// TODO(ACL-Legacy-Compat): remove in phase 2
func (s *Store) ACLTokenListUpgradeable(max int) (structs.ACLTokens, <-chan struct{}, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := tx.Get(tableACLTokens, "needs-upgrade", true)
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

	item, err := tx.First(tableACLTokens, s.expiresIndexName(local))
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

	iter, err := tx.Get(tableACLTokens, s.expiresIndexName(local))
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
		return indexExpiresLocal
	}
	return indexExpiresGlobal
}

// ACLTokenDeleteByAccessor is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *Store) ACLTokenDeleteByAccessor(idx uint64, accessor string, entMeta *acl.EnterpriseMeta) error {
	return s.aclTokenDelete(idx, accessor, indexAccessor, entMeta)
}

func (s *Store) ACLTokenBatchDelete(idx uint64, tokenIDs []string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, tokenID := range tokenIDs {
		if err := aclTokenDeleteTxn(tx, idx, tokenID, indexAccessor, nil); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) aclTokenDelete(idx uint64, value, index string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclTokenDeleteTxn(tx, idx, value, index, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

func aclTokenDeleteTxn(tx WriteTxn, idx uint64, value, index string, entMeta *acl.EnterpriseMeta) error {
	// Look up the existing token
	_, token, err := aclTokenGetFromIndex(tx, value, index, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl token lookup: %v", err)
	}

	if token == nil {
		return nil
	}

	if token.(*structs.ACLToken).AccessorID == structs.ACLTokenAnonymousID {
		return fmt.Errorf("Deletion of the builtin anonymous token is not permitted")
	}

	return aclTokenDeleteWithToken(tx, token.(*structs.ACLToken), idx)
}

func aclTokenDeleteAllForAuthMethodTxn(tx WriteTxn, idx uint64, methodName string, methodGlobalLocality bool, methodMeta *acl.EnterpriseMeta) error {
	// collect all the tokens linked with the given auth method.
	iter, err := aclTokenListByAuthMethod(tx, methodName, methodMeta, methodMeta.WithWildcardNamespace())
	if err != nil {
		return fmt.Errorf("failed acl token lookup: %v", err)
	}

	var tokens structs.ACLTokens
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		token := raw.(*structs.ACLToken)
		tokenIsGlobal := !token.Local

		// Need to ensure that if we have an auth method named "blah" in the
		// primary and secondary datacenters, and the primary instance has
		// TokenLocality==global that when we delete the secondary instance we
		// don't also blow away replicated tokens from the primary.
		if methodGlobalLocality == tokenIsGlobal {
			tokens = append(tokens, token)
		}
	}

	if len(tokens) > 0 {
		// delete them all
		for _, token := range tokens {
			if err := aclTokenDeleteWithToken(tx, token, idx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Store) ACLPolicyBatchSet(idx uint64, policies structs.ACLPolicies) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, policy := range policies {
		if err := aclPolicySetTxn(tx, idx, policy); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ACLPolicySet(idx uint64, policy *structs.ACLPolicy) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclPolicySetTxn(tx, idx, policy); err != nil {
		return err
	}

	return tx.Commit()
}

func aclPolicySetTxn(tx WriteTxn, idx uint64, policy *structs.ACLPolicy) error {
	// Check that the ID is set
	if policy.ID == "" {
		return ErrMissingACLPolicyID
	}

	if policy.Name == "" {
		return ErrMissingACLPolicyName
	}

	var existing *structs.ACLPolicy
	_, existingRaw, err := aclPolicyGetByID(tx, policy.ID, nil)
	if err != nil {
		return err
	}

	if existingRaw != nil {
		existing = existingRaw.(*structs.ACLPolicy)
	}

	if existing != nil {
		if builtinPolicy, ok := structs.ACLBuiltinPolicies[policy.ID]; ok {
			// Only the name and description are modifiable
			// Here we specifically check that the rules on the builtin policy
			// are identical to the correct policy rules within the binary. This is opposed
			// to checking against the current rules to allow us to update the rules during
			// upgrades.
			if policy.Rules != builtinPolicy.Rules {
				return fmt.Errorf("Changing the Rules for the builtin %s policy is not permitted", builtinPolicy.Name)
			}

			if policy.Datacenters != nil && len(policy.Datacenters) != 0 {
				return fmt.Errorf("Changing the Datacenters of the builtin %s policy is not permitted", builtinPolicy.Name)
			}
		}
	}

	// ensure the name is unique (cannot conflict with another policy with a different ID)
	q := Query{Value: policy.Name, EnterpriseMeta: policy.EnterpriseMeta}
	nameMatch, err := tx.First(tableACLPolicies, indexName, q)
	if err != nil {
		return err
	}
	if nameMatch != nil && policy.ID != nameMatch.(*structs.ACLPolicy).ID {
		return fmt.Errorf("A policy with name %q already exists", policy.Name)
	}

	if err := aclPolicyUpsertValidateEnterprise(tx, policy, existing); err != nil {
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
	return aclPolicyInsert(tx, policy)
}

func (s *Store) ACLPolicyGetByID(ws memdb.WatchSet, id string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLPolicy, error) {
	return s.aclPolicyGet(ws, id, aclPolicyGetByID, entMeta)
}

func (s *Store) ACLPolicyGetByName(ws memdb.WatchSet, name string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLPolicy, error) {
	return s.aclPolicyGet(ws, name, aclPolicyGetByName, entMeta)
}

func aclPolicyGetByName(tx ReadTxn, name string, entMeta *acl.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	// todo: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	q := Query{Value: name, EnterpriseMeta: *entMeta}
	return tx.FirstWatch(tableACLPolicies, indexName, q)
}

func (s *Store) ACLPolicyBatchGet(ws memdb.WatchSet, ids []string) (uint64, structs.ACLPolicies, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	policies := make(structs.ACLPolicies, 0)
	for _, pid := range ids {
		policy, err := getPolicyWithTxn(tx, ws, pid, aclPolicyGetByID, nil)
		if err != nil {
			return 0, nil, err
		}

		if policy != nil {
			policies = append(policies, policy)
		}
	}

	// We are specifically not wanting to call aclPolicyMaxIndex here as we always want the
	// index entry for the tableACLPolicies table.
	idx := maxIndexTxn(tx, tableACLPolicies)

	return idx, policies, nil
}

type aclPolicyGetFn func(ReadTxn, string, *acl.EnterpriseMeta) (<-chan struct{}, interface{}, error)

func getPolicyWithTxn(tx ReadTxn, ws memdb.WatchSet, value string, fn aclPolicyGetFn, entMeta *acl.EnterpriseMeta) (*structs.ACLPolicy, error) {
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

func (s *Store) aclPolicyGet(ws memdb.WatchSet, value string, fn aclPolicyGetFn, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLPolicy, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	policy, err := getPolicyWithTxn(tx, ws, value, fn, entMeta)
	if err != nil {
		return 0, nil, err
	}

	idx := aclPolicyMaxIndex(tx, policy, entMeta)

	return idx, policy, nil
}

func (s *Store) ACLPolicyList(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, structs.ACLPolicies, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := tx.Get(tableACLPolicies, indexName+"_prefix", entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl policy lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLPolicies
	for policy := iter.Next(); policy != nil; policy = iter.Next() {
		result = append(result, policy.(*structs.ACLPolicy))
	}

	// Get the table index.
	idx := aclPolicyMaxIndex(tx, nil, entMeta)

	return idx, result, nil
}

func (s *Store) ACLPolicyDeleteByID(idx uint64, id string, entMeta *acl.EnterpriseMeta) error {
	return s.aclPolicyDelete(idx, id, aclPolicyGetByID, entMeta)
}

func (s *Store) ACLPolicyDeleteByName(idx uint64, name string, entMeta *acl.EnterpriseMeta) error {
	return s.aclPolicyDelete(idx, name, aclPolicyGetByName, entMeta)
}

func (s *Store) ACLPolicyBatchDelete(idx uint64, policyIDs []string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, policyID := range policyIDs {
		if err := aclPolicyDeleteTxn(tx, idx, policyID, aclPolicyGetByID, nil); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) aclPolicyDelete(idx uint64, value string, fn aclPolicyGetFn, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclPolicyDeleteTxn(tx, idx, value, fn, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

func aclPolicyDeleteTxn(tx WriteTxn, idx uint64, value string, fn aclPolicyGetFn, entMeta *acl.EnterpriseMeta) error {
	// Look up the existing token
	_, rawPolicy, err := fn(tx, value, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl policy lookup: %v", err)
	}

	if rawPolicy == nil {
		return nil
	}

	policy := rawPolicy.(*structs.ACLPolicy)

	if builtinPolicy, ok := structs.ACLBuiltinPolicies[policy.ID]; ok {
		return fmt.Errorf("Deletion of the builtin %s policy is not permitted", builtinPolicy.Name)
	}

	return aclPolicyDeleteWithPolicy(tx, policy, idx)
}

func (s *Store) ACLRoleBatchSet(idx uint64, roles structs.ACLRoles, allowMissingPolicyIDs bool) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, role := range roles {
		if err := aclRoleSetTxn(tx, idx, role, allowMissingPolicyIDs); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ACLRoleSet(idx uint64, role *structs.ACLRole) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclRoleSetTxn(tx, idx, role, false); err != nil {
		return err
	}

	return tx.Commit()
}

func aclRoleSetTxn(tx WriteTxn, idx uint64, role *structs.ACLRole, allowMissing bool) error {
	// Check that the ID is set
	if role.ID == "" {
		return ErrMissingACLRoleID
	}

	if role.Name == "" {
		return ErrMissingACLRoleName
	}

	_, existingRaw, err := aclRoleGetByID(tx, role.ID, nil)
	if err != nil {
		return fmt.Errorf("failed acl role lookup: %v", err)
	}

	var existing *structs.ACLRole
	if existingRaw != nil {
		existing = existingRaw.(*structs.ACLRole)
	}

	// ensure the name is unique (cannot conflict with another role with a different ID)
	q := Query{EnterpriseMeta: role.EnterpriseMeta, Value: role.Name}
	nameMatch, err := tx.First(tableACLRoles, indexName, q)
	if err != nil {
		return fmt.Errorf("failed acl role lookup: %v", err)
	}
	if nameMatch != nil && role.ID != nameMatch.(*structs.ACLRole).ID {
		return fmt.Errorf("A role with name %q already exists", role.Name)
	}

	if err := resolveRolePolicyLinks(tx, role, allowMissing); err != nil {
		return err
	}

	for _, svcid := range role.ServiceIdentities {
		if svcid.ServiceName == "" {
			return fmt.Errorf("Encountered a Role with an empty service identity name in the state store")
		}
	}

	for _, nodeid := range role.NodeIdentities {
		if nodeid.NodeName == "" {
			return fmt.Errorf("Encountered a Role with an empty node identity name in the state store")
		}
		if nodeid.Datacenter == "" {
			return fmt.Errorf("Encountered a Role with an empty node identity datacenter in the state store")
		}
	}

	if err := aclRoleUpsertValidateEnterprise(tx, role, existing); err != nil {
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

	return aclRoleInsert(tx, role)
}

type aclRoleGetFn func(ReadTxn, string, *acl.EnterpriseMeta) (<-chan struct{}, interface{}, error)

func (s *Store) ACLRoleGetByID(ws memdb.WatchSet, id string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLRole, error) {
	return s.aclRoleGet(ws, id, aclRoleGetByID, entMeta)
}

func (s *Store) ACLRoleGetByName(ws memdb.WatchSet, name string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLRole, error) {
	return s.aclRoleGet(ws, name, aclRoleGetByName, entMeta)
}

func aclRoleGetByName(tx ReadTxn, name string, entMeta *acl.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	q := Query{EnterpriseMeta: *entMeta, Value: name}
	return tx.FirstWatch(tableACLRoles, indexName, q)
}

func (s *Store) ACLRoleBatchGet(ws memdb.WatchSet, ids []string) (uint64, structs.ACLRoles, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	roles := make(structs.ACLRoles, 0, len(ids))
	for _, rid := range ids {
		role, err := getRoleWithTxn(tx, ws, rid, aclRoleGetByID, nil)
		if err != nil {
			return 0, nil, err
		}

		if role != nil {
			roles = append(roles, role)
		}
	}

	idx := maxIndexTxn(tx, tableACLRoles)

	return idx, roles, nil
}

func getRoleWithTxn(tx ReadTxn, ws memdb.WatchSet, value string, fn aclRoleGetFn, entMeta *acl.EnterpriseMeta) (*structs.ACLRole, error) {
	watchCh, rawRole, err := fn(tx, value, entMeta)
	if err != nil {
		return nil, fmt.Errorf("failed acl role lookup: %v", err)
	}
	ws.Add(watchCh)

	if rawRole != nil {
		role := rawRole.(*structs.ACLRole)
		role, err := fixupRolePolicyLinks(tx, role)
		if err != nil {
			return nil, err
		}
		return role, nil
	}

	return nil, nil
}

func (s *Store) aclRoleGet(ws memdb.WatchSet, value string, fn aclRoleGetFn, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLRole, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	role, err := getRoleWithTxn(tx, ws, value, fn, entMeta)
	if err != nil {
		return 0, nil, err
	}

	idx := aclRoleMaxIndex(tx, role, entMeta)

	return idx, role, nil
}

func (s *Store) ACLRoleList(ws memdb.WatchSet, policy string, entMeta *acl.EnterpriseMeta) (uint64, structs.ACLRoles, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var iter memdb.ResultIterator
	var err error

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	if policy != "" {
		q := Query{Value: policy, EnterpriseMeta: *entMeta}
		iter, err = tx.Get(tableACLRoles, indexPolicies, q)
	} else {
		iter, err = tx.Get(tableACLRoles, indexName+"_prefix", entMeta)
	}

	if err != nil {
		return 0, nil, fmt.Errorf("failed acl role lookup: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result structs.ACLRoles
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		role := raw.(*structs.ACLRole)
		role, err := fixupRolePolicyLinks(tx, role)
		if err != nil {
			return 0, nil, err
		}
		result = append(result, role)
	}

	// Get the table index.
	idx := aclRoleMaxIndex(tx, nil, entMeta)

	return idx, result, nil
}

func (s *Store) ACLRoleDeleteByID(idx uint64, id string, entMeta *acl.EnterpriseMeta) error {
	return s.aclRoleDelete(idx, id, aclRoleGetByID, entMeta)
}

func (s *Store) ACLRoleDeleteByName(idx uint64, name string, entMeta *acl.EnterpriseMeta) error {
	return s.aclRoleDelete(idx, name, aclRoleGetByName, entMeta)
}

func (s *Store) ACLRoleBatchDelete(idx uint64, roleIDs []string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, roleID := range roleIDs {
		if err := aclRoleDeleteTxn(tx, idx, roleID, aclRoleGetByID, nil); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) aclRoleDelete(idx uint64, value string, fn aclRoleGetFn, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclRoleDeleteTxn(tx, idx, value, fn, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

func aclRoleDeleteTxn(tx WriteTxn, idx uint64, value string, fn aclRoleGetFn, entMeta *acl.EnterpriseMeta) error {
	// Look up the existing role
	_, rawRole, err := fn(tx, value, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl role lookup: %v", err)
	}

	if rawRole == nil {
		return nil
	}

	role := rawRole.(*structs.ACLRole)

	return aclRoleDeleteWithRole(tx, role, idx)
}

func (s *Store) ACLBindingRuleBatchSet(idx uint64, rules structs.ACLBindingRules) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, rule := range rules {
		if err := aclBindingRuleSetTxn(tx, idx, rule); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ACLBindingRuleSet(idx uint64, rule *structs.ACLBindingRule) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclBindingRuleSetTxn(tx, idx, rule); err != nil {
		return err
	}
	return tx.Commit()
}

func aclBindingRuleSetTxn(tx WriteTxn, idx uint64, rule *structs.ACLBindingRule) error {
	// Check that the ID and AuthMethod are set
	if rule.ID == "" {
		return ErrMissingACLBindingRuleID
	} else if rule.AuthMethod == "" {
		return ErrMissingACLBindingRuleAuthMethod
	}

	var existing *structs.ACLBindingRule
	_, existingRaw, err := aclBindingRuleGetByID(tx, rule.ID, nil)
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

	if err := aclBindingRuleUpsertValidateEnterprise(tx, rule, existing); err != nil {
		return err
	}

	if _, method, err := aclAuthMethodGetByName(tx, rule.AuthMethod, &rule.EnterpriseMeta); err != nil {
		return fmt.Errorf("failed acl auth method lookup: %v", err)
	} else if method == nil {
		return fmt.Errorf("failed inserting acl binding rule: auth method not found")
	}

	return aclBindingRuleInsert(tx, rule)
}

func (s *Store) ACLBindingRuleGetByID(ws memdb.WatchSet, id string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLBindingRule, error) {
	return s.aclBindingRuleGet(ws, id, entMeta)
}

func (s *Store) aclBindingRuleGet(ws memdb.WatchSet, value string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLBindingRule, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	watchCh, rawRule, err := aclBindingRuleGetByID(tx, value, entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl binding rule lookup: %v", err)
	}
	ws.Add(watchCh)

	var rule *structs.ACLBindingRule
	if rawRule != nil {
		rule = rawRule.(*structs.ACLBindingRule)
	}

	idx := aclBindingRuleMaxIndex(tx, rule, entMeta)

	return idx, rule, nil
}

func (s *Store) ACLBindingRuleList(ws memdb.WatchSet, methodName string, entMeta *acl.EnterpriseMeta) (uint64, structs.ACLBindingRules, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var (
		iter memdb.ResultIterator
		err  error
	)
	if methodName != "" {
		iter, err = aclBindingRuleListByAuthMethod(tx, methodName, entMeta)
	} else {
		iter, err = aclBindingRuleList(tx, entMeta)
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
	idx := aclBindingRuleMaxIndex(tx, nil, entMeta)

	return idx, result, nil
}

func (s *Store) ACLBindingRuleDeleteByID(idx uint64, id string, entMeta *acl.EnterpriseMeta) error {
	return s.aclBindingRuleDelete(idx, id, entMeta)
}

func (s *Store) ACLBindingRuleBatchDelete(idx uint64, bindingRuleIDs []string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, bindingRuleID := range bindingRuleIDs {
		aclBindingRuleDeleteTxn(tx, idx, bindingRuleID, nil)
	}
	return tx.Commit()
}

func (s *Store) aclBindingRuleDelete(idx uint64, id string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclBindingRuleDeleteTxn(tx, idx, id, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

func aclBindingRuleDeleteTxn(tx WriteTxn, idx uint64, id string, entMeta *acl.EnterpriseMeta) error {
	// Look up the existing binding rule
	_, rawRule, err := aclBindingRuleGetByID(tx, id, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl binding rule lookup: %v", err)
	}

	if rawRule == nil {
		return nil
	}

	rule := rawRule.(*structs.ACLBindingRule)

	if err := aclBindingRuleDeleteWithRule(tx, rule, idx); err != nil {
		return fmt.Errorf("failed deleting acl binding rule: %v", err)
	}
	return nil
}

func aclBindingRuleDeleteAllForAuthMethodTxn(tx WriteTxn, idx uint64, methodName string, entMeta *acl.EnterpriseMeta) error {
	// collect them all
	iter, err := aclBindingRuleListByAuthMethod(tx, methodName, entMeta)
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
			if err := aclBindingRuleDeleteWithRule(tx, rule, idx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Store) ACLAuthMethodBatchSet(idx uint64, methods structs.ACLAuthMethods) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, method := range methods {
		// this is only used when doing batch insertions for upgrades and replication. Therefore
		// we take whatever those said.
		if err := aclAuthMethodSetTxn(tx, idx, method); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ACLAuthMethodSet(idx uint64, method *structs.ACLAuthMethod) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclAuthMethodSetTxn(tx, idx, method); err != nil {
		return err
	}

	return tx.Commit()
}

func aclAuthMethodSetTxn(tx WriteTxn, idx uint64, method *structs.ACLAuthMethod) error {
	// Check that the Name and Type are set
	if method.Name == "" {
		return ErrMissingACLAuthMethodName
	} else if method.Type == "" {
		return ErrMissingACLAuthMethodType
	}

	var existing *structs.ACLAuthMethod
	_, existingRaw, err := aclAuthMethodGetByName(tx, method.Name, &method.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("failed acl auth method lookup: %v", err)
	}

	if err := aclAuthMethodUpsertValidateEnterprise(tx, method, existing); err != nil {
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

	return aclAuthMethodInsert(tx, method)
}

func (s *Store) ACLAuthMethodGetByName(ws memdb.WatchSet, name string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLAuthMethod, error) {
	return s.aclAuthMethodGet(ws, name, entMeta)
}

func (s *Store) aclAuthMethodGet(ws memdb.WatchSet, name string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLAuthMethod, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	method, err := getAuthMethodWithTxn(tx, ws, name, entMeta)
	if err != nil {
		return 0, nil, err
	}

	idx := aclAuthMethodMaxIndex(tx, method, entMeta)

	return idx, method, nil
}

func getAuthMethodWithTxn(tx ReadTxn, ws memdb.WatchSet, name string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, error) {
	watchCh, rawMethod, err := aclAuthMethodGetByName(tx, name, entMeta)
	if err != nil {
		return nil, fmt.Errorf("failed acl auth method lookup: %v", err)
	}
	ws.Add(watchCh)

	if rawMethod != nil {
		return rawMethod.(*structs.ACLAuthMethod), nil
	}

	return nil, nil
}

func (s *Store) ACLAuthMethodList(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, structs.ACLAuthMethods, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := aclAuthMethodList(tx, entMeta)
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
	idx := aclAuthMethodMaxIndex(tx, nil, entMeta)

	return idx, result, nil
}

func (s *Store) ACLAuthMethodDeleteByName(idx uint64, name string, entMeta *acl.EnterpriseMeta) error {
	return s.aclAuthMethodDelete(idx, name, entMeta)
}

func (s *Store) ACLAuthMethodBatchDelete(idx uint64, names []string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, name := range names {
		// NOTE: it may seem odd that one EnterpriseMeta is being used for all of the auth methods being
		// deleted. However we never actually batch these deletions as auth methods are not replicated
		// Therefore this is fine but if we ever change that precondition then this will be wrong (unless
		// we ensure all deletions in a batch should have the same enterprise meta)
		aclAuthMethodDeleteTxn(tx, idx, name, entMeta)
	}

	return tx.Commit()
}

func (s *Store) aclAuthMethodDelete(idx uint64, name string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := aclAuthMethodDeleteTxn(tx, idx, name, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

func aclAuthMethodDeleteTxn(tx WriteTxn, idx uint64, name string, entMeta *acl.EnterpriseMeta) error {
	// Look up the existing method
	_, rawMethod, err := aclAuthMethodGetByName(tx, name, entMeta)
	if err != nil {
		return fmt.Errorf("failed acl auth method lookup: %v", err)
	}

	if rawMethod == nil {
		return nil
	}

	method := rawMethod.(*structs.ACLAuthMethod)

	if err := aclBindingRuleDeleteAllForAuthMethodTxn(tx, idx, method.Name, entMeta); err != nil {
		return err
	}

	if err := aclTokenDeleteAllForAuthMethodTxn(tx, idx, method.Name, method.TokenLocality == "global", entMeta); err != nil {
		return err
	}

	return aclAuthMethodDeleteWithMethod(tx, method, idx)
}

func aclTokenList(tx ReadTxn, entMeta *acl.EnterpriseMeta, locality bool) (memdb.ResultIterator, error) {
	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	// if the namespace is the wildcard that will also be handled as the local index uses
	// the NamespaceMultiIndex instead of the NamespaceIndex
	q := BoolQuery{
		Value:          locality,
		EnterpriseMeta: *entMeta,
	}
	return tx.Get(tableACLTokens, indexLocality, q)
}

// intFromBool returns 1 if cond is true, 0 otherwise.
func intFromBool(cond bool) byte {
	if cond {
		return 1
	}
	return 0
}

func aclPolicyInsert(tx WriteTxn, policy *structs.ACLPolicy) error {
	if err := tx.Insert(tableACLPolicies, policy); err != nil {
		return fmt.Errorf("failed inserting acl policy: %v", err)
	}
	return updateTableIndexEntries(tx, tableACLPolicies, policy.ModifyIndex, &policy.EnterpriseMeta)
}

func aclRoleInsert(tx WriteTxn, role *structs.ACLRole) error {
	// insert the role into memdb
	if err := tx.Insert(tableACLRoles, role); err != nil {
		return fmt.Errorf("failed inserting acl role: %v", err)
	}

	// update acl-roles index
	return updateTableIndexEntries(tx, tableACLRoles, role.ModifyIndex, &role.EnterpriseMeta)
}

func aclTokenInsert(tx WriteTxn, token *structs.ACLToken) error {
	// insert the token into memdb
	if err := tx.Insert(tableACLTokens, token); err != nil {
		return fmt.Errorf("failed inserting acl token: %v", err)
	}
	// update the overall acl-tokens index
	return updateTableIndexEntries(tx, tableACLTokens, token.ModifyIndex, token.EnterpriseMetadata())
}

func aclAuthMethodInsert(tx WriteTxn, method *structs.ACLAuthMethod) error {
	// insert the auth method into memdb
	if err := tx.Insert(tableACLAuthMethods, method); err != nil {
		return fmt.Errorf("failed inserting acl role: %v", err)
	}

	// update acl-auth-methods index
	return updateTableIndexEntries(tx, tableACLAuthMethods, method.ModifyIndex, &method.EnterpriseMeta)
}

func aclBindingRuleInsert(tx WriteTxn, rule *structs.ACLBindingRule) error {
	rule.EnterpriseMeta.Normalize()

	// insert the role into memdb
	if err := tx.Insert(tableACLBindingRules, rule); err != nil {
		return fmt.Errorf("failed inserting acl role: %v", err)
	}

	// update acl-binding-rules index
	return updateTableIndexEntries(tx, tableACLBindingRules, rule.ModifyIndex, &rule.EnterpriseMeta)
}
