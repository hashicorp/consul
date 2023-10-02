package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func ServiceWorkloadIndexer() *cache.Index {
	return cache.NewIndex(serviceWorkloadIndexer{})
}

type serviceWorkloadIndexer struct{}

func (serviceWorkloadIndexer) FromArgs(args ...any) ([]byte, error) {
	return cache.ReferenceOrIDFromArgs(args...)
}

func (serviceWorkloadIndexer) FromResource(r *pbresource.Resource) (bool, [][]byte, error) {
	svc, err := resource.Decode[*pbcatalog.Service](r)
	if err != nil {
		return false, nil, err
	}

	if svc.Data.Workloads == nil || (len(svc.Data.Workloads.Prefixes) == 0 && len(svc.Data.Workloads.Names) == 0) {
		return false, nil, nil
	}

	var indexes [][]byte

	for _, name := range svc.Data.Workloads.Names {
		ref := &pbresource.Reference{
			Type:    pbcatalog.WorkloadType,
			Tenancy: svc.Resource.Id.Tenancy,
			Name:    name,
		}

		indexes = append(indexes, cache.IndexFromRefOrID(ref))
	}

	for _, name := range svc.Data.Workloads.Prefixes {
		ref := &pbresource.Reference{
			Type:    pbcatalog.WorkloadType,
			Tenancy: svc.Resource.Id.Tenancy,
			Name:    name,
		}

		b := cache.IndexFromRefOrID(ref)

		// need to remove the path separator to be compatible with prefix matching
		indexes = append(indexes, b[:len(b)-1])
	}

	return true, indexes, err
}
