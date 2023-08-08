// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package resource

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func v2TenancyToV1EntMeta(tenancy *pbresource.Tenancy) *acl.EnterpriseMeta {
	return acl.DefaultEnterpriseMeta()
}

func v1EntMetaToV2Tenancy(reg *resource.Registration, entMeta *acl.EnterpriseMeta, tenancy *pbresource.Tenancy) {
	if (reg.Scope == resource.ScopeNamespace || reg.Scope == resource.ScopePartition) && tenancy.Partition == "" {
		tenancy.Partition = entMeta.PartitionOrDefault()
	}

	if reg.Scope == resource.ScopeNamespace && tenancy.Namespace == "" {
		tenancy.Namespace = entMeta.NamespaceOrDefault()
	}
}
