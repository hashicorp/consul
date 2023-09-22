// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	scope := reg.GetScope()
	if (scope == pbresource.Scope_SCOPE_NAMESPACE || scope == pbresource.Scope_SCOPE_PARTITION) && tenancy.Partition == "" {
		tenancy.Partition = entMeta.PartitionOrDefault()
	}

	if scope == pbresource.Scope_SCOPE_NAMESPACE && tenancy.Namespace == "" {
		tenancy.Namespace = entMeta.NamespaceOrDefault()
	}
}
