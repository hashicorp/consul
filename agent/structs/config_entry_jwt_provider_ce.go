// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

func (e *JWTProviderConfigEntry) validatePartitionAndNamespace() error {
	if !acl.IsDefaultPartition(e.PartitionOrDefault()) {
		return fmt.Errorf("Partitions are an enterprise only feature")
	}

	if acl.DefaultNamespaceName != e.NamespaceOrDefault() {
		return fmt.Errorf("Namespaces are an enterprise only feature")
	}

	return nil
}
