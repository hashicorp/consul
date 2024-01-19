// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	IndexName = "selected-workloads"
)

func Index[T WorkloadSelecting](name string) *index.Index {
	return indexers.DecodedMultiIndexer[T](
		name,
		index.SingleValueFromOneOrTwoArgs[resource.ReferenceOrID, index.IndexQueryOptions](fromArgs),
		fromResource[T],
	)
}

func fromArgs(r resource.ReferenceOrID, opts index.IndexQueryOptions) ([]byte, error) {
	workloadRef := &pbresource.Reference{
		Type:    pbcatalog.WorkloadType,
		Tenancy: r.GetTenancy(),
		Name:    r.GetName(),
	}

	if opts.Prefix {
		return index.PrefixIndexFromRefOrID(workloadRef), nil
	} else {
		return index.IndexFromRefOrID(workloadRef), nil
	}
}

func fromResource[T WorkloadSelecting](res *resource.DecodedResource[T]) (bool, [][]byte, error) {
	sel := res.Data.GetWorkloads()
	if sel == nil || (len(sel.Prefixes) == 0 && len(sel.Names) == 0) {
		return false, nil, nil
	}

	var indexes [][]byte

	for _, name := range sel.Names {
		ref := &pbresource.Reference{
			Type:    pbcatalog.WorkloadType,
			Tenancy: res.Id.Tenancy,
			Name:    name,
		}

		indexes = append(indexes, index.IndexFromRefOrID(ref))
	}

	for _, name := range sel.Prefixes {
		ref := &pbresource.Reference{
			Type:    pbcatalog.WorkloadType,
			Tenancy: res.Id.Tenancy,
			Name:    name,
		}

		b := index.IndexFromRefOrID(ref)

		// need to remove the path separator to be compatible with prefix matching
		indexes = append(indexes, b[:len(b)-1])
	}

	return true, indexes, nil
}
