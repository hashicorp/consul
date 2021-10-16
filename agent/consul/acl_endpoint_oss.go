// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
)

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
