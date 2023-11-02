package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func ServiceIndexer() *cache.Index {
	return cache.NewIndex(servicesIndexer{})
}

type servicesIndexer struct{}

func (servicesIndexer) FromArgs(args ...any) ([]byte, error) {
	return cache.ReferenceOrIDFromArgs(args...)
}

func (servicesIndexer) FromResource(r *pbresource.Resource) (bool, [][]byte, error) {

	var indexes [][]byte

	switch {
	case resource.EqualType(r.Id.Type, pbmulticluster.PartitionExportedServicesType):
		var builder cache.IndexBuilder
		builder.Raw(cache.IndexFromType(pbcatalog.ServiceType))
		builder.String(r.Id.Tenancy.Partition)
		indexes = append(indexes, builder.Bytes())
	case resource.EqualType(r.Id.Type, pbmulticluster.NamespaceExportedServicesType):
		var builder cache.IndexBuilder
		builder.Raw(cache.IndexFromType(pbcatalog.ServiceType))
		builder.String(r.Id.Tenancy.Partition)
		builder.String(r.Id.Tenancy.Namespace)
		indexes = append(indexes, builder.Bytes())
	case resource.EqualType(r.Id.Type, pbmulticluster.ExportedServicesType):
		var builder cache.IndexBuilder
		builder.Raw(cache.IndexFromType(pbcatalog.ServiceType))
		builder.String(r.Id.Tenancy.Partition)
		builder.String(r.Id.Tenancy.Namespace)

		base := builder.Bytes()

		dec, err := resource.Decode[*pbmulticluster.ExportedServices](r)
		if err != nil {
			return false, nil, err
		}
		
		for _, svc := range dec.Data.Services {
			var builder cache.IndexBuilder
			builder.Raw(base)
			builder.String(svc)
			indexes = append(indexes, builder.Bytes())
		}
	}

	return true, indexes, nil
}
