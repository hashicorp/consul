// +build !consulent

package structs

import (
	"hash"

	"github.com/hashicorp/consul/acl"
)

var emptyEnterpriseMeta = EnterpriseMeta{}

// EnterpriseMeta stub
type EnterpriseMeta struct{}

func (m *EnterpriseMeta) estimateSize() int {
	return 0
}

func (m *EnterpriseMeta) addToHash(_ hash.Hash, _ bool) {
	// do nothing
}

// ReplicationEnterpriseMeta stub
func ReplicationEnterpriseMeta() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// DefaultEnterpriseMeta stub
func DefaultEnterpriseMeta() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// WildcardEnterpriseMeta stub
func WildcardEnterpriseMeta() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// FillAuthzContext stub
func (_ *EnterpriseMeta) FillAuthzContext(_ *acl.EnterpriseAuthorizerContext) {}

// FillAuthzContext stub
func (d *DirEntry) FillAuthzContext(*acl.EnterpriseAuthorizerContext) {}
