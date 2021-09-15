// +build !consulent

package state

import (
	"fmt"
	"strings"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

func aclPolicyInsert(tx WriteTxn, policy *structs.ACLPolicy) error {
	if err := tx.Insert(tableACLPolicies, policy); err != nil {
		return fmt.Errorf("failed inserting acl policy: %v", err)
	}

	if err := indexUpdateMaxTxn(tx, policy.ModifyIndex, tableACLPolicies); err != nil {
		return fmt.Errorf("failed updating acl policies index: %v", err)
	}

	return nil
}

func aclPolicyGetByID(tx ReadTxn, id string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(tableACLPolicies, indexID, id)
}

func aclPolicyDeleteWithPolicy(tx WriteTxn, policy *structs.ACLPolicy, idx uint64) error {
	// remove the policy
	if err := tx.Delete(tableACLPolicies, policy); err != nil {
		return fmt.Errorf("failed deleting acl policy: %v", err)
	}

	// update the overall acl-policies index
	if err := indexUpdateMaxTxn(tx, idx, tableACLPolicies); err != nil {
		return fmt.Errorf("failed updating acl policies index: %v", err)
	}
	return nil
}

func aclPolicyMaxIndex(tx ReadTxn, _ *structs.ACLPolicy, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableACLPolicies)
}

func aclPolicyUpsertValidateEnterprise(ReadTxn, *structs.ACLPolicy, *structs.ACLPolicy) error {
	return nil
}

func (s *Store) ACLPolicyUpsertValidateEnterprise(*structs.ACLPolicy, *structs.ACLPolicy) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
/////                        ACL Token Functions                          /////
///////////////////////////////////////////////////////////////////////////////

func aclTokenInsert(tx WriteTxn, token *structs.ACLToken) error {
	// insert the token into memdb
	if err := tx.Insert(tableACLTokens, token); err != nil {
		return fmt.Errorf("failed inserting acl token: %v", err)
	}

	// update the overall acl-tokens index
	if err := indexUpdateMaxTxn(tx, token.ModifyIndex, tableACLTokens); err != nil {
		return fmt.Errorf("failed updating acl tokens index: %v", err)
	}

	return nil
}

func aclTokenGetFromIndex(tx ReadTxn, id string, index string, entMeta *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(tableACLTokens, index, id)
}

func aclTokenListAll(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableACLTokens, "id")
}

func aclTokenListByPolicy(tx ReadTxn, policy string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableACLTokens, indexPolicies, Query{Value: policy})
}

func aclTokenListByRole(tx ReadTxn, role string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableACLTokens, indexRoles, Query{Value: role})
}

func aclTokenListByAuthMethod(tx ReadTxn, authMethod string, _, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableACLTokens, indexAuthMethod, AuthMethodQuery{Value: authMethod})
}

func aclTokenDeleteWithToken(tx WriteTxn, token *structs.ACLToken, idx uint64) error {
	// remove the token
	if err := tx.Delete(tableACLTokens, token); err != nil {
		return fmt.Errorf("failed deleting acl token: %v", err)
	}

	// update the overall acl-tokens index
	if err := indexUpdateMaxTxn(tx, idx, tableACLTokens); err != nil {
		return fmt.Errorf("failed updating acl tokens index: %v", err)
	}
	return nil
}

func aclTokenMaxIndex(tx ReadTxn, _ *structs.ACLToken, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableACLTokens)
}

func aclTokenUpsertValidateEnterprise(tx WriteTxn, token *structs.ACLToken, existing *structs.ACLToken) error {
	return nil
}

func (s *Store) ACLTokenUpsertValidateEnterprise(token *structs.ACLToken, existing *structs.ACLToken) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
/////                         ACL Role Functions                          /////
///////////////////////////////////////////////////////////////////////////////

func aclRoleInsert(tx WriteTxn, role *structs.ACLRole) error {
	// insert the role into memdb
	if err := tx.Insert(tableACLRoles, role); err != nil {
		return fmt.Errorf("failed inserting acl role: %v", err)
	}

	// update the overall acl-roles index
	if err := indexUpdateMaxTxn(tx, role.ModifyIndex, tableACLRoles); err != nil {
		return fmt.Errorf("failed updating acl roles index: %v", err)
	}
	return nil
}

func aclRoleGetByID(tx ReadTxn, id string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(tableACLRoles, indexID, id)
}

func aclRoleDeleteWithRole(tx WriteTxn, role *structs.ACLRole, idx uint64) error {
	// remove the role
	if err := tx.Delete(tableACLRoles, role); err != nil {
		return fmt.Errorf("failed deleting acl role: %v", err)
	}

	// update the overall acl-roles index
	if err := indexUpdateMaxTxn(tx, idx, tableACLRoles); err != nil {
		return fmt.Errorf("failed updating acl policies index: %v", err)
	}
	return nil
}

func aclRoleMaxIndex(tx ReadTxn, _ *structs.ACLRole, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableACLRoles)
}

func aclRoleUpsertValidateEnterprise(tx WriteTxn, role *structs.ACLRole, existing *structs.ACLRole) error {
	return nil
}

func (s *Store) ACLRoleUpsertValidateEnterprise(role *structs.ACLRole, existing *structs.ACLRole) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
/////                     ACL Binding Rule Functions                      /////
///////////////////////////////////////////////////////////////////////////////

func aclBindingRuleInsert(tx WriteTxn, rule *structs.ACLBindingRule) error {
	// insert the role into memdb
	if err := tx.Insert(tableACLBindingRules, rule); err != nil {
		return fmt.Errorf("failed inserting acl role: %v", err)
	}

	// update the overall acl-binding-rules index
	if err := indexUpdateMaxTxn(tx, rule.ModifyIndex, tableACLBindingRules); err != nil {
		return fmt.Errorf("failed updating acl binding-rules index: %v", err)
	}

	return nil
}

func aclBindingRuleGetByID(tx ReadTxn, id string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(tableACLBindingRules, "id", id)
}

func aclBindingRuleList(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableACLBindingRules, "id")
}

func aclBindingRuleListByAuthMethod(tx ReadTxn, method string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableACLBindingRules, indexAuthMethod, Query{Value: method})
}

func aclBindingRuleDeleteWithRule(tx WriteTxn, rule *structs.ACLBindingRule, idx uint64) error {
	// remove the acl-binding-rule
	if err := tx.Delete(tableACLBindingRules, rule); err != nil {
		return fmt.Errorf("failed deleting acl binding rule: %v", err)
	}

	// update the overall acl-binding-rules index
	if err := indexUpdateMaxTxn(tx, idx, tableACLBindingRules); err != nil {
		return fmt.Errorf("failed updating acl binding rules index: %v", err)
	}
	return nil
}

func aclBindingRuleMaxIndex(tx ReadTxn, _ *structs.ACLBindingRule, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableACLBindingRules)
}

func aclBindingRuleUpsertValidateEnterprise(tx ReadTxn, rule *structs.ACLBindingRule, existing *structs.ACLBindingRule) error {
	return nil
}

func (s *Store) ACLBindingRuleUpsertValidateEnterprise(rule *structs.ACLBindingRule, existing *structs.ACLBindingRule) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
/////                     ACL Auth Method Functions                       /////
///////////////////////////////////////////////////////////////////////////////

func aclAuthMethodInsert(tx WriteTxn, method *structs.ACLAuthMethod) error {
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

func aclAuthMethodDeleteWithMethod(tx WriteTxn, method *structs.ACLAuthMethod, idx uint64) error {
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

func aclAuthMethodUpsertValidateEnterprise(_ ReadTxn, method *structs.ACLAuthMethod, existing *structs.ACLAuthMethod) error {
	return nil
}

func (s *Store) ACLAuthMethodUpsertValidateEnterprise(method *structs.ACLAuthMethod, existing *structs.ACLAuthMethod) error {
	return nil
}

func indexAuthMethodFromACLToken(raw interface{}) ([]byte, error) {
	p, ok := raw.(*structs.ACLToken)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLToken index", raw)
	}

	if p.AuthMethod == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(p.AuthMethod))

	return b.Bytes(), nil
}
