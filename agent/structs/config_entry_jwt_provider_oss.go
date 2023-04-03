// Copyright (c) HashiCorp, Inc.

//go:build !consulent
// +build !consulent

package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

func (e *JWTProviderConfigEntry) validatePartition() error {
	if !acl.IsDefaultPartition(e.PartitionOrDefault()) {
		return fmt.Errorf("Partitions are an enterprise only feature")
	}
	return nil
}
