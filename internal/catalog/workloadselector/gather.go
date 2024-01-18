// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"fmt"
	"sort"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// GetWorkloadsWithSelector will retrieve all workloads for the given resources selector
// and unmarhshal them, returning a slice of objects hold both the resource and
// unmarshaled forms. Unmarshalling errors, or other cache errors
// will be returned to the caller.
func GetWorkloadsWithSelector[T WorkloadSelecting](c cache.ReadOnlyCache, res *resource.DecodedResource[T]) ([]*resource.DecodedResource[*pbcatalog.Workload], error) {
	if res == nil {
		return nil, nil
	}

	sel := res.Data.GetWorkloads()

	if sel == nil || (len(sel.GetNames()) < 1 && len(sel.GetPrefixes()) < 1) {
		return nil, nil
	}

	// this map will track all workloads by name which is needed to deduplicate workloads if they
	// are specified multiple times throughout the list of selection criteria
	workloadNames := make(map[string]struct{})

	var workloads []*resource.DecodedResource[*pbcatalog.Workload]

	// First gather all the prefix matched workloads. We could do this second but by doing
	// it first its possible we can avoid some operations to get individual
	// workloads selected by name if they are also matched by a prefix.
	for _, prefix := range sel.GetPrefixes() {
		iter, err := cache.ListIteratorDecoded[*pbcatalog.Workload](
			c,
			pbcatalog.WorkloadType,
			"id",
			&pbresource.ID{
				Type:    pbcatalog.WorkloadType,
				Tenancy: res.Id.Tenancy,
				Name:    prefix,
			},
			index.IndexQueryOptions{Prefix: true})
		if err != nil {
			return nil, err
		}

		// append all workloads in the list response to our list of all selected workloads
		for workload, err := iter.Next(); workload != nil || err != nil; workload, err = iter.Next() {
			if err != nil {
				return nil, err
			}

			// ignore duplicate workloads
			if _, found := workloadNames[workload.Id.Name]; !found {
				workloads = append(workloads, workload)
				workloadNames[workload.Id.Name] = struct{}{}
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
			Tenancy: res.Id.Tenancy,
			Name:    name,
		}

		res, err := cache.GetDecoded[*pbcatalog.Workload](c, pbcatalog.WorkloadType, "id", workloadID)
		if err != nil {
			return nil, err
		}

		// ignore workloads that don't exist as it is fine for a Service to select them. If they exist in the
		// future then the ServiceEndpoints will be regenerated to include them.
		if res == nil {
			continue
		}

		workloads = append(workloads, res)
		workloadNames[res.Id.Name] = struct{}{}
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
		return workloads[i].Id.Name < workloads[j].Id.Name
	})

	return workloads, nil
}
