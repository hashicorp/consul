// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

func (e *ExportedServicesConfigEntry) validateServicesEnterprise() error {
	for i, svc := range e.Services {
		for j, consumer := range svc.Consumers {
			if !acl.IsDefaultPartition(consumer.Partition) {
				return fmt.Errorf("Services[%d].Consumers[%d]: partitions are an enterprise-only feature", i, j)
			}
			if consumer.SamenessGroup != "" {
				return fmt.Errorf("Services[%d].Consumers[%d]: sameness-groups are an enterprise-only feature", i, j)
			}
		}
	}
	return nil
}
