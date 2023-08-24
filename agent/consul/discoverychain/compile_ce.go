// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package discoverychain

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func (c *compiler) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return structs.DefaultEnterpriseMetaInDefaultPartition()
}
