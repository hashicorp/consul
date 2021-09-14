// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func testIdentityForTokenEnterprise(string) (bool, structs.ACLIdentity, error) {
	return true, nil, acl.ErrNotFound
}

func testPolicyForIDEnterprise(string) (bool, *structs.ACLPolicy, error) {
	return true, nil, acl.ErrNotFound
}

func testRoleForIDEnterprise(string) (bool, *structs.ACLRole, error) {
	return true, nil, acl.ErrNotFound
}

// EnterpriseACLResolverTestDelegate stub
type EnterpriseACLResolverTestDelegate struct{}

// RPC stub for the EnterpriseACLResolverTestDelegate
func (d *EnterpriseACLResolverTestDelegate) RPC(string, interface{}, interface{}) (bool, error) {
	return false, nil
}

func (d *EnterpriseACLResolverTestDelegate) UseTestLocalData(_ []interface{}) {}
func (d *EnterpriseACLResolverTestDelegate) UseDefaultData()                  {}
