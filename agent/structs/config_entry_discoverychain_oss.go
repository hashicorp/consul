//go:build !consulent
// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceRouteDestination
func (dest *ServiceRouteDestination) GetEnterpriseMeta(_ *acl.EnterpriseMeta) *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceSplit
func (split *ServiceSplit) GetEnterpriseMeta(_ *acl.EnterpriseMeta) *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceResolverRedirect
func (redir *ServiceResolverRedirect) GetEnterpriseMeta(_ *acl.EnterpriseMeta) *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceResolverFailover
func (failover *ServiceResolverFailover) GetEnterpriseMeta(_ *acl.EnterpriseMeta) *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the DiscoveryChainRequest
func (req *DiscoveryChainRequest) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

// WithEnterpriseMeta will populate the corresponding fields in the
// DiscoveryChainRequest from the EnterpriseMeta struct
func (req *DiscoveryChainRequest) WithEnterpriseMeta(_ *acl.EnterpriseMeta) {
	// do nothing
}
