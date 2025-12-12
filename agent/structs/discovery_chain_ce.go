// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

func (t *DiscoveryTarget) GetEnterpriseMetadata() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}
