//go:build !consulent
// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

func (us *Upstream) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func (us *Upstream) DestinationID() ServiceID {
	return ServiceID{
		ID: us.DestinationName,
	}
}

func (us *Upstream) enterpriseStringPrefix() string {
	return ""
}
