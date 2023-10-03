// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MapSelector returns requests of the provided type given a workload
// selector and tenancy. The type has to be name-aligned with the workload.
func MapSelector(ctx context.Context,
	client pbresource.ResourceServiceClient,
	typ *pbresource.Type,
	selector *pbcatalog.WorkloadSelector,
	tenancy *pbresource.Tenancy) ([]controller.Request, error) {
	if selector == nil {
		return nil, nil
	}

	var result []controller.Request

	for _, prefix := range selector.GetPrefixes() {
		resp, err := client.List(ctx, &pbresource.ListRequest{
			Type:       pbcatalog.WorkloadType,
			Tenancy:    tenancy,
			NamePrefix: prefix,
		})
		if err != nil {
			return nil, err
		}
		if len(resp.Resources) == 0 {
			return nil, fmt.Errorf("no workloads found")
		}
		for _, r := range resp.Resources {
			id := resource.ReplaceType(typ, r.Id)
			result = append(result, controller.Request{
				ID: id,
			})
		}
	}

	// We don't do lookups for names as this should be done in the controller's reconcile.
	for _, name := range selector.GetNames() {
		id := &pbresource.ID{
			Name:    name,
			Tenancy: tenancy,
			Type:    typ,
		}
		result = append(result, controller.Request{
			ID: id,
		})
	}

	return result, nil
}
