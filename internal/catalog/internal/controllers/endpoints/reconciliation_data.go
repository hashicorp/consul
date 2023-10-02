// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type serviceData = resource.DecodedResource[*pbcatalog.Service]

type endpointsData = resource.DecodedResource[*pbcatalog.ServiceEndpoints]

type workloadData = resource.DecodedResource[*pbcatalog.Workload]

// getWorkloadData will retrieve all workloads for the given services selector
// and unmarhshal them, returning a slic of objects hold both the resource and
// unmarshaled forms. Unmarshalling errors, or other resource service errors
// will be returned to the caller.
func getWorkloadData(ctx context.Context, rt controller.Runtime, svc *serviceData) ([]*workloadData, error) {
	var workloads []*workloadData

	sel := svc.Data.GetWorkloads()

	// this map will track all the gathered workloads by name, this is mainly to deduplicate workloads if they
	// are specified multiple times throughout the list of selection criteria
	workloadNames := make(map[string]struct{})

	// First gather all the prefix matched workloads. We could do this second but by doing
	// it first its possible we can avoid some resource service calls to read individual
	// workloads selected by name if they are also matched by a prefix.
	for _, prefix := range sel.GetPrefixes() {
		resources, err := resource.ListDecodedResource[*pbcatalog.Workload](ctx, rt.Client, &pbresource.ListRequest{
			Type:       pbcatalog.WorkloadType,
			Tenancy:    svc.Resource.Id.Tenancy,
			NamePrefix: prefix,
		})
		if err != nil {
			return nil, err
		}

		// append all workloads in the list response to our list of all selected workloads
		for _, workload := range resources {
			// ignore duplicate workloads
			if _, found := workloadNames[workload.Resource.Id.Name]; !found {
				workloads = append(workloads, workload)
				workloadNames[workload.Resource.Id.Name] = struct{}{}
			}
		}
	}

	// Now gather the exact match selections
	for _, name := range sel.GetNames() {
		// ignore names we have already fetched
		if _, found := workloadNames[name]; found {
			continue
		}

		workloadID := &pbresource.ID{
			Type:    pbcatalog.WorkloadType,
			Tenancy: svc.Resource.Id.Tenancy,
			Name:    name,
		}

		workload, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, rt.Client, workloadID)
		if err != nil {
			return nil, err
		}

		if workload == nil {
			continue
		}

		workloads = append(workloads, workload)
		workloadNames[workload.Resource.Id.Name] = struct{}{}
	}

	if sel.GetFilter() != "" && len(workloads) > 0 {
		var err error
		workloads, err = resource.FilterResourcesByMetadata(workloads, sel.GetFilter())
		if err != nil {
			return nil, fmt.Errorf("error filtering results by metadata: %w", err)
		}
	}

	// Sorting ensures deterministic output. This will help for testing but
	// the real reason to do this is so we will be able to diff the set of
	// workloads endpoints to determine if we need to update them.
	sort.Slice(workloads, func(i, j int) bool {
		return workloads[i].Resource.Id.Name < workloads[j].Resource.Id.Name
	})

	return workloads, nil
}
