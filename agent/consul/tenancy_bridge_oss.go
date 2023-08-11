// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package consul

func (b *V1TenancyBridge) PartitionExists(partition string) (bool, error) {
	if partition == "default" {
		return true, nil
	}
	return false, nil
}

func (b *V1TenancyBridge) IsPartitionMarkedForDeletion(partition string) (bool, error) {
	return false, nil
}

func (b *V1TenancyBridge) NamespaceExists(partition, namespace string) (bool, error) {
	if partition == "default" && namespace == "default" {
		return true, nil
	}
	return false, nil
}

func (b *V1TenancyBridge) IsNamespaceMarkedForDeletion(partition, namespace string) (bool, error) {
	return false, nil
}
