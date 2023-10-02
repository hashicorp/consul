package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func WorkloadNodeIndexer() *cache.Index {
	return cache.NewIndex(workloadNodeIndexer{})
}

type workloadNodeIndexer struct{}

func (workloadNodeIndexer) FromArgs(args ...any) ([]byte, error) {
	return cache.ReferenceOrIDFromArgs(args...)
}

func (workloadNodeIndexer) FromResource(r *pbresource.Resource) (bool, []byte, error) {
	wk, err := resource.Decode[*pbcatalog.Workload](r)
	if err != nil {
		return false, nil, err
	}

	if wk.Data.NodeName == "" {
		return false, nil, nil
	}

	ref := &pbresource.Reference{
		Type:    pbcatalog.NodeType,
		Tenancy: r.GetId().GetTenancy(),
		Name:    wk.Data.NodeName,
	}

	idx, err := cache.ReferenceOrIDFromArgs(ref)
	return true, idx, err
}
