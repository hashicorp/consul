package xroute

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func parentRefSliceToRefSlice(parentRefs []*pbmesh.ParentReference) []resource.ReferenceOrID {
	if parentRefs == nil {
		return nil
	}
	parents := make([]resource.ReferenceOrID, 0, len(parentRefs))
	for _, parentRef := range parentRefs {
		if parentRef.Ref != nil {
			parents = append(parents, parentRef.Ref)
		}
	}
	return parents
}

func MapParentRefs[T types.XRouteData]() controller.DependencyMapper {
	return func(_ context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		dec, err := resource.Decode[T](res)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling xRoute: %w", err)
		}

		refs := parentRefSliceToRefSlice(dec.Data.GetParentRefs())

		return controller.MakeRequests(pbmesh.ComputedRoutesType, refs), nil
	}
}
