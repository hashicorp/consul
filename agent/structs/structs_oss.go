// +build !consulent

package structs

import (
	"hash"

	"github.com/hashicorp/consul/acl"
)

// EnterpriseMeta stub
type EnterpriseMeta struct{}

func (m *EnterpriseMeta) estimateSize() int {
	return 0
}

func (m *EnterpriseMeta) addToHash(hasher hash.Hash) {
	// do nothing
}

// ReplicationEnterpriseMeta stub
func ReplicationEnterpriseMeta() *EnterpriseMeta {
	return nil
}

// DefaultEnterpriseMeta stub
func DefaultEnterpriseMeta() *EnterpriseMeta {
	return nil
}

// InitDefault stub
func (m *EnterpriseMeta) InitDefault() {}

// FillAuthzContext stub
func (m *EnterpriseMeta) FillAuthzContext(*acl.EnterpriseAuthorizerContext) {}

// FillAuthzContext stub
func (d *DirEntry) FillAuthzContext(*acl.EnterpriseAuthorizerContext) {}
