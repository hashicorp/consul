// +build !consulent

package structs

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceRouteDestination
func (dest *ServiceRouteDestination) GetEnterpriseMeta(_ *EnterpriseMeta) *EnterpriseMeta {
	return DefaultEnterpriseMeta()
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceSplit
func (split *ServiceSplit) GetEnterpriseMeta(_ *EnterpriseMeta) *EnterpriseMeta {
	return DefaultEnterpriseMeta()
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceResolverRedirect
func (redir *ServiceResolverRedirect) GetEnterpriseMeta(_ *EnterpriseMeta) *EnterpriseMeta {
	return DefaultEnterpriseMeta()
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceResolverFailover
func (failover *ServiceResolverFailover) GetEnterpriseMeta(_ *EnterpriseMeta) *EnterpriseMeta {
	return DefaultEnterpriseMeta()
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the DiscoveryChainRequest
func (req *DiscoveryChainRequest) GetEnterpriseMeta() *EnterpriseMeta {
	return DefaultEnterpriseMeta()
}

// WithEnterpriseMeta will populate the corresponding fields in the
// DiscoveryChainRequest from the EnterpriseMeta struct
func (req *DiscoveryChainRequest) WithEnterpriseMeta(_ *EnterpriseMeta) {
	// do nothing
}
