// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package bridge

func (b *V2TenancyBridge) PartitionExists(partition string) (bool, error) {
	if partition == "default" {
		return true, nil
	}
	return false, nil
}

func (b *V2TenancyBridge) IsPartitionMarkedForDeletion(partition string) (bool, error) {
	return false, nil
}
