// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package resource

func (b *V2TenancyBridge) PartitionExists(partition string) (bool, error) {
	if partition == "default" {
		return true, nil
	}
	return false, nil
}

func (b *V2TenancyBridge) IsPartitionMarkedForDeletion(partition string) (bool, error) {
	return false, nil
}

func (b *V2TenancyBridge) NamespaceExists(partition, namespace string) (bool, error) {
	if partition == "default" && namespace == "default" {
		return true, nil
	}
	return false, nil
}

func (b *V2TenancyBridge) IsNamespaceMarkedForDeletion(partition, namespace string) (bool, error) {
	return false, nil
}
