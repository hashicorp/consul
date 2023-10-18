package indexers

import (
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TrafficPermissionsIndex() *cache.Index {
	return cache.NewIndex(trafficPermissionsIndexer{})
}

type trafficPermissionsIndexer struct{}

func (trafficPermissionsIndexer) FromArgs(args ...any) ([]byte, error) {
	return cache.ReferenceOrIDFromArgs(args...)
}

func (trafficPermissionsIndexer) FromResource(r *pbresource.Resource) (bool, []byte, error) {
	tp, err := resource.Decode[*pbauth.TrafficPermissions](r)
	if err != nil {
		return false, nil, err
	}

	if tp.Data.Destination.IdentityName == "" {
		return false, nil, types.ErrWildcardNotSupported
	}

	return true, cache.IndexFromRefOrID(&pbresource.Reference{
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp.Resource.Id.Tenancy,
		Name:    tp.Data.Destination.IdentityName,
	}), nil
}
