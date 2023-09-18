//go:build !consulent
// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

func (t *DiscoveryTarget) GetEnterpriseMetadata() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}
