package servicetoroutes

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/servicenamealigned"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func Map(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	iter, err := rt.Cache.ListIterator(pbcatalog.FailoverPolicyType, "destinations", res.Id)
	if err != nil {
		return nil, err
	}

	var effectiveServiceIDs []*pbresource.ID
	for failover := iter.Next(); failover != nil; failover = iter.Next() {
		effectiveServiceIDs = append(effectiveServiceIDs, resource.ReplaceType(pbcatalog.ServiceType, failover.GetId()))
	}

	// (case 1) Do the direct mapping also.
	effectiveServiceIDs = append(effectiveServiceIDs, res.Id)

	var reqs []controller.Request
	for _, svcID := range effectiveServiceIDs {
		got, err := servicenamealigned.Map(ctx, rt, &pbresource.Resource{Id: svcID})
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, got...)
	}

	return reqs, nil
}
