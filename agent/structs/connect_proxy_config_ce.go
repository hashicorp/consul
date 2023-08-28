// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

func (us *Upstream) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func (us *Upstream) DestinationID() PeeredServiceName {
	return PeeredServiceName{
		Peer:        us.DestinationPeer,
		ServiceName: NewServiceName(us.DestinationName, DefaultEnterpriseMetaInDefaultPartition()),
	}
}

func (us *Upstream) enterpriseStringPrefix() string {
	return ""
}
