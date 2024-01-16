// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package bridge

import "github.com/hashicorp/consul/internal/resource"

func (b *V2TenancyBridge) PartitionExists(partition string) (bool, error) {
	if partition == resource.DefaultPartitionName {
		// In CE partition resources are never actually created. However, conceptually
		// the default partition always exists.
		return true, nil
	}
	return false, nil
}

func (b *V2TenancyBridge) IsPartitionMarkedForDeletion(partition string) (bool, error) {
	return false, nil
}
