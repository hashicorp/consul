// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"fmt"
	"strings"
)

func (b *ConfigEntryBackend) enterpriseCheckPartitions(partition string) error {
	if partition == "" || strings.EqualFold(partition, "default") {
		return nil
	}
	return fmt.Errorf("Partitions are a Consul Enterprise feature")
}
