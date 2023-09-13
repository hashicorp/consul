// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

import (
	"fmt"

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

// ValidateEnterprise validates that enterprise fields are only set
// with enterprise binaries.
func (redir *ServiceResolverRedirect) ValidateEnterprise() error {
	if redir.Partition != "" {
		return fmt.Errorf("Setting Partition requires Consul Enterprise")
	}

	if redir.Namespace != "" {
		return fmt.Errorf("Setting Namespace requires Consul Enterprise")
	}

	if redir.SamenessGroup != "" {
		return fmt.Errorf("Setting SamenessGroup requires Consul Enterprise")
	}

	return nil
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceResolverFailover
func (failover *ServiceResolverFailover) GetEnterpriseMeta(_ *acl.EnterpriseMeta) *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

// ValidateEnterprise validates that enterprise fields are only set
// with enterprise binaries.
func (failover *ServiceResolverFailover) ValidateEnterprise() error {
	if failover.Namespace != "" {
		return fmt.Errorf("Setting Namespace requires Consul Enterprise")
	}

	if failover.SamenessGroup != "" {
		return fmt.Errorf("Setting SamenessGroup requires Consul Enterprise")
	}

	return nil
}

// GetEnterpriseMeta is used to synthesize the EnterpriseMeta struct from
// fields in the ServiceResolverFailoverTarget
func (target *ServiceResolverFailoverTarget) GetEnterpriseMeta(_ *acl.EnterpriseMeta) *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

// ValidateEnterprise validates that enterprise fields are only set
// with enterprise binaries.
func (redir *ServiceResolverFailoverTarget) ValidateEnterprise() error {
	if redir.Partition != "" {
		return fmt.Errorf("Setting Partition requires Consul Enterprise")
	}

	if redir.Namespace != "" {
		return fmt.Errorf("Setting Namespace requires Consul Enterprise")
	}

	return nil
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

// ValidateEnterprise validates that enterprise fields are only set
// with enterprise binaries.
func (f *ServiceResolverFailoverPolicy) ValidateEnterprise() error {
	if f == nil {
		return nil
	}

	if f.Mode != "" {
		return fmt.Errorf("Setting failover policies requires Consul Enterprise")
	}

	return nil
}

// RelatedSamenessGroups doesn't return anything in community edition.
func (e *ServiceResolverConfigEntry) RelatedSamenessGroups() []string {
	return nil
}

func (pbl *ServiceResolverPrioritizeByLocality) validate() error {
	var zero ServiceResolverPrioritizeByLocality
	if pbl == nil || *pbl == zero {
		return nil
	}
	return fmt.Errorf("Prioritize-by-locality requires Consul Enterprise ")
}
