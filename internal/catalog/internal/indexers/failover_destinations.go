package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func FailoverDestinationsIndexer() *cache.Index {
	return cache.NewIndex(failoverDestinationsIndexer{})
}

type failoverDestinationsIndexer struct{}

func (failoverDestinationsIndexer) FromArgs(args ...any) ([]byte, error) {
	return cache.ReferenceOrIDFromArgs(args...)
}

func (failoverDestinationsIndexer) FromResource(r *pbresource.Resource) (bool, [][]byte, error) {
	f, err := resource.Decode[*pbcatalog.FailoverPolicy](r)
	if err != nil {
		return false, nil, err
	}

	destRefs := f.Data.GetUnderlyingDestinationRefs()
	indexes := make([][]byte, len(destRefs)+1)

	// add all the destination reference indexes
	for idx, r := range destRefs {
		indexes[idx] = cache.IndexFromRefOrID(r)
	}

	// add the index to the name aligned service as well
	indexes[len(indexes)-1] = cache.IndexFromRefOrID(&pbresource.Reference{
		Type:    pbcatalog.ServiceType,
		Tenancy: f.Resource.Id.Tenancy,
		Name:    f.Resource.Id.Name,
	})

	return true, indexes, nil
}
