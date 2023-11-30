// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package connect

import (
	"strings"

	"github.com/hashicorp/consul/acl"
)

// GetEnterpriseMeta will synthesize an EnterpriseMeta struct from the SpiffeIDService.
// in CE this just returns an empty (but never nil) struct pointer
func (id SpiffeIDService) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return &acl.EnterpriseMeta{}
}

// PartitionOrDefault breaks from CE's pattern of returning empty strings.
// Although CE has no support for partitions, it still needs to be able to
// handle exportedPartition from peered Consul Enterprise clusters in order
// to generate the correct SpiffeID.
func (id SpiffeIDService) PartitionOrDefault() string {
	if id.Partition == "" {
		return "default"
	}

	return strings.ToLower(id.Partition)
}
