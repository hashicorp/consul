// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package catalog

import (
	"github.com/hashicorp/consul/acl"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
)

func GetEnterpriseMetaFromResourceID(id *pbresource.ID) *acl.EnterpriseMeta {
	return acl.DefaultEnterpriseMeta()
}
