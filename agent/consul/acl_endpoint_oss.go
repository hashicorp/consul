//go:build !consulent
// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
)

func (a *ACL) tokenUpsertValidateEnterprise(token *structs.ACLToken, existing *structs.ACLToken) error {
	state := a.srv.fsm.State()
	return state.ACLTokenUpsertValidateEnterprise(token, existing)
}

func (a *ACL) policyUpsertValidateEnterprise(policy *structs.ACLPolicy, existing *structs.ACLPolicy) error {
	state := a.srv.fsm.State()
	return state.ACLPolicyUpsertValidateEnterprise(policy, existing)
}

func (a *ACL) roleUpsertValidateEnterprise(role *structs.ACLRole, existing *structs.ACLRole) error {
	state := a.srv.fsm.State()
	return state.ACLRoleUpsertValidateEnterprise(role, existing)
}

func (a *ACL) enterpriseAuthMethodTypeValidation(authMethodType string) error {
	return nil
}

func enterpriseAuthMethodValidation(method *structs.ACLAuthMethod, validator authmethod.Validator) error {
	return nil
}

func computeTargetEnterpriseMeta(
	method *structs.ACLAuthMethod,
	verifiedIdentity *authmethod.Identity,
) (*structs.EnterpriseMeta, error) {
	return &structs.EnterpriseMeta{}, nil
}
