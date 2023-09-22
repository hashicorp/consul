// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package authv2beta1

import "github.com/hashicorp/consul/proto-public/pbresource"

type SourceToSpiffe interface {
	GetIdentityName() string
	GetPartition() string
	GetNamespace() string
	GetPeer() string
	GetSamenessGroup() string
}

var _ SourceToSpiffe = (*Source)(nil)
var _ SourceToSpiffe = (*ExcludeSource)(nil)

func SourceToTenancy(s SourceToSpiffe) *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: s.GetPartition(),
		Namespace: s.GetNamespace(),
		PeerName:  s.GetPeer(),
	}
}
