package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache"
	cacheindexers "github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func ParentRefsIndex[T types.XRouteData]() *cache.Index {
	return cacheindexers.RefIndex[T](func(res *resource.DecodedResource[T]) []*pbresource.Reference {
		prefs := res.Data.GetParentRefs()
		refs := make([]*pbresource.Reference, 0, len(prefs))
		for _, pr := range prefs {
			if pr.Ref == nil {
				continue
			}
			refs = append(refs, pr.Ref)
		}
		return refs
	})
}
