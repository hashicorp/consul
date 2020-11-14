// +build !consulent

package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

///////////////////////////////////////////////////////////////////////////////
/////                          ACL Table Schemas                          /////
///////////////////////////////////////////////////////////////////////////////

func tokensTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl-tokens",
		Indexes: map[string]*memdb.IndexSchema{
			"accessor": {
				Name: "accessor",
				// DEPRECATED (ACL-Legacy-Compat) - we should not AllowMissing here once legacy compat is removed
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "AccessorID",
				},
			},
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "SecretID",
					Lowercase: false,
				},
			},
			"policies": {
				Name: "policies",
				// Need to allow missing for the anonymous token
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenPoliciesIndex{},
			},
			"roles": {
				Name:         "roles",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenRolesIndex{},
			},
			"authmethod": {
				Name:         "authmethod",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "AuthMethod",
					Lowercase: false,
				},
			},
			"local": {
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
			"expires-global": {
				Name:         "expires-global",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenExpirationIndex{LocalFilter: false},
			},
			"expires-local": {
				Name:         "expires-local",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenExpirationIndex{LocalFilter: true},
			},

			//DEPRECATED (ACL-Legacy-Compat) - This index is only needed while we support upgrading v1 to v2 acls
			// This table indexes all the ACL tokens that do not have an AccessorID
			"needs-upgrade": {
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
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"name": {
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

func rolesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl-roles",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"name": {
				Name:         "name",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Name",
					Lowercase: true,
				},
			},
			"policies": {
				Name: "policies",
				// Need to allow missing for the anonymous token
				AllowMissing: true,
				Unique:       false,
				Indexer:      &RolePoliciesIndex{},
			},
		},
	}
}

func bindingRulesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl-binding-rules",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"authmethod": {
				Name:         "authmethod",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "AuthMethod",
					Lowercase: true,
				},
			},
		},
	}
}

func authMethodsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl-auth-methods",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Name",
					Lowercase: true,
				},
			},
		},
	}
}

///////////////////////////////////////////////////////////////////////////////
/////                        ACL Policy Functions                         /////
///////////////////////////////////////////////////////////////////////////////

func aclPolicyInsert(tx *txn, policy *structs.ACLPolicy) error {
	if err := tx.Insert("acl-policies", policy); err != nil {
		return fmt.Errorf("failed inserting acl policy: %v", err)
	}

	if err := indexUpdateMaxTxn(tx, policy.ModifyIndex, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating acl policies index: %v", err)
	}

	return nil
}

func aclPolicyGetByID(tx ReadTxn, id string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch("acl-policies", "id", id)
}

func aclPolicyGetByName(tx ReadTxn, name string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch("acl-policies", "name", name)
}

func aclPolicyList(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-policies", "id")
}

func aclPolicyDeleteWithPolicy(tx *txn, policy *structs.ACLPolicy, idx uint64) error {
	// remove the policy
	if err := tx.Delete("acl-policies", policy); err != nil {
		return fmt.Errorf("failed deleting acl policy: %v", err)
	}

	// update the overall acl-policies index
	if err := indexUpdateMaxTxn(tx, idx, "acl-policies"); err != nil {
		return fmt.Errorf("failed updating acl policies index: %v", err)
	}
	return nil
}

func aclPolicyMaxIndex(tx ReadTxn, _ *structs.ACLPolicy, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "acl-policies")
}

func aclPolicyUpsertValidateEnterprise(*txn, *structs.ACLPolicy, *structs.ACLPolicy) error {
	return nil
}

func (s *Store) ACLPolicyUpsertValidateEnterprise(*structs.ACLPolicy, *structs.ACLPolicy) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
/////                        ACL Token Functions                          /////
///////////////////////////////////////////////////////////////////////////////

func aclTokenInsert(tx *txn, token *structs.ACLToken) error {
	// insert the token into memdb
	if err := tx.Insert("acl-tokens", token); err != nil {
		return fmt.Errorf("failed inserting acl token: %v", err)
	}

	// update the overall acl-tokens index
	if err := indexUpdateMaxTxn(tx, token.ModifyIndex, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating acl tokens index: %v", err)
	}

	return nil
}

func aclTokenGetFromIndex(tx ReadTxn, id string, index string, entMeta *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch("acl-tokens", index, id)
}

func aclTokenListAll(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-tokens", "id")
}

func aclTokenListLocal(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-tokens", "local", true)
}

func aclTokenListGlobal(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-tokens", "local", false)
}

func aclTokenListByPolicy(tx ReadTxn, policy string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-tokens", "policies", policy)
}

func aclTokenListByRole(tx ReadTxn, role string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-tokens", "roles", role)
}

func aclTokenListByAuthMethod(tx ReadTxn, authMethod string, _, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-tokens", "authmethod", authMethod)
}

func aclTokenDeleteWithToken(tx *txn, token *structs.ACLToken, idx uint64) error {
	// remove the token
	if err := tx.Delete("acl-tokens", token); err != nil {
		return fmt.Errorf("failed deleting acl token: %v", err)
	}

	// update the overall acl-tokens index
	if err := indexUpdateMaxTxn(tx, idx, "acl-tokens"); err != nil {
		return fmt.Errorf("failed updating acl tokens index: %v", err)
	}
	return nil
}

func aclTokenMaxIndex(tx ReadTxn, _ *structs.ACLToken, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "acl-tokens")
}

func aclTokenUpsertValidateEnterprise(tx *txn, token *structs.ACLToken, existing *structs.ACLToken) error {
	return nil
}

func (s *Store) ACLTokenUpsertValidateEnterprise(token *structs.ACLToken, existing *structs.ACLToken) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
/////                         ACL Role Functions                          /////
///////////////////////////////////////////////////////////////////////////////

func aclRoleInsert(tx *txn, role *structs.ACLRole) error {
	// insert the role into memdb
	if err := tx.Insert("acl-roles", role); err != nil {
		return fmt.Errorf("failed inserting acl role: %v", err)
	}

	// update the overall acl-roles index
	if err := indexUpdateMaxTxn(tx, role.ModifyIndex, "acl-roles"); err != nil {
		return fmt.Errorf("failed updating acl roles index: %v", err)
	}
	return nil
}

func aclRoleGetByID(tx ReadTxn, id string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch("acl-roles", "id", id)
}

func aclRoleGetByName(tx ReadTxn, name string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch("acl-roles", "name", name)
}

func aclRoleList(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-roles", "id")
}

func aclRoleListByPolicy(tx ReadTxn, policy string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-roles", "policies", policy)
}

func aclRoleDeleteWithRole(tx *txn, role *structs.ACLRole, idx uint64) error {
	// remove the role
	if err := tx.Delete("acl-roles", role); err != nil {
		return fmt.Errorf("failed deleting acl role: %v", err)
	}

	// update the overall acl-roles index
	if err := indexUpdateMaxTxn(tx, idx, "acl-roles"); err != nil {
		return fmt.Errorf("failed updating acl policies index: %v", err)
	}
	return nil
}

func aclRoleMaxIndex(tx ReadTxn, _ *structs.ACLRole, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "acl-roles")
}

func aclRoleUpsertValidateEnterprise(tx *txn, role *structs.ACLRole, existing *structs.ACLRole) error {
	return nil
}

func (s *Store) ACLRoleUpsertValidateEnterprise(role *structs.ACLRole, existing *structs.ACLRole) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
/////                     ACL Binding Rule Functions                      /////
///////////////////////////////////////////////////////////////////////////////

func aclBindingRuleInsert(tx *txn, rule *structs.ACLBindingRule) error {
	// insert the role into memdb
	if err := tx.Insert("acl-binding-rules", rule); err != nil {
		return fmt.Errorf("failed inserting acl role: %v", err)
	}

	// update the overall acl-binding-rules index
	if err := indexUpdateMaxTxn(tx, rule.ModifyIndex, "acl-binding-rules"); err != nil {
		return fmt.Errorf("failed updating acl binding-rules index: %v", err)
	}

	return nil
}

func aclBindingRuleGetByID(tx ReadTxn, id string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch("acl-binding-rules", "id", id)
}

func aclBindingRuleList(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-binding-rules", "id")
}

func aclBindingRuleListByAuthMethod(tx ReadTxn, method string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-binding-rules", "authmethod", method)
}

func aclBindingRuleDeleteWithRule(tx *txn, rule *structs.ACLBindingRule, idx uint64) error {
	// remove the rule
	if err := tx.Delete("acl-binding-rules", rule); err != nil {
		return fmt.Errorf("failed deleting acl binding rule: %v", err)
	}

	// update the overall acl-binding-rules index
	if err := indexUpdateMaxTxn(tx, idx, "acl-binding-rules"); err != nil {
		return fmt.Errorf("failed updating acl binding rules index: %v", err)
	}
	return nil
}

func aclBindingRuleMaxIndex(tx ReadTxn, _ *structs.ACLBindingRule, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "acl-binding-rules")
}

func aclBindingRuleUpsertValidateEnterprise(tx *txn, rule *structs.ACLBindingRule, existing *structs.ACLBindingRule) error {
	return nil
}

func (s *Store) ACLBindingRuleUpsertValidateEnterprise(rule *structs.ACLBindingRule, existing *structs.ACLBindingRule) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
/////                     ACL Auth Method Functions                       /////
///////////////////////////////////////////////////////////////////////////////

func aclAuthMethodInsert(tx *txn, method *structs.ACLAuthMethod) error {
	// insert the role into memdb
	if err := tx.Insert("acl-auth-methods", method); err != nil {
		return fmt.Errorf("failed inserting acl role: %v", err)
	}

	// update the overall acl-auth-methods index
	if err := indexUpdateMaxTxn(tx, method.ModifyIndex, "acl-auth-methods"); err != nil {
		return fmt.Errorf("failed updating acl auth methods index: %v", err)
	}

	return nil
}

func aclAuthMethodGetByName(tx ReadTxn, method string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch("acl-auth-methods", "id", method)
}

func aclAuthMethodList(tx ReadTxn, entMeta *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("acl-auth-methods", "id")
}

func aclAuthMethodDeleteWithMethod(tx *txn, method *structs.ACLAuthMethod, idx uint64) error {
	// remove the method
	if err := tx.Delete("acl-auth-methods", method); err != nil {
		return fmt.Errorf("failed deleting acl auth method: %v", err)
	}

	// update the overall acl-auth-methods index
	if err := indexUpdateMaxTxn(tx, idx, "acl-auth-methods"); err != nil {
		return fmt.Errorf("failed updating acl auth methods index: %v", err)
	}
	return nil
}

func aclAuthMethodMaxIndex(tx ReadTxn, _ *structs.ACLAuthMethod, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "acl-auth-methods")
}

func aclAuthMethodUpsertValidateEnterprise(tx *txn, method *structs.ACLAuthMethod, existing *structs.ACLAuthMethod) error {
	return nil
}

func (s *Store) ACLAuthMethodUpsertValidateEnterprise(method *structs.ACLAuthMethod, existing *structs.ACLAuthMethod) error {
	return nil
}
