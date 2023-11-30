// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package consul

import (
	"fmt"
	"strings"
)

func (b *PeeringBackend) enterpriseCheckPartitions(partition string) error {
	if partition == "" || strings.EqualFold(partition, "default") {
		return nil
	}
	return fmt.Errorf("Partitions are a Consul Enterprise feature")
}

func (b *PeeringBackend) enterpriseCheckNamespaces(namespace string) error {
	if namespace == "" || strings.EqualFold(namespace, "default") {
		return nil
	}
	return fmt.Errorf("Namespaces are a Consul Enterprise feature")
}
