package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Mapper) MapProxyConfigurationToProxyStateTemplate(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var proxyConfig pbmesh.ProxyConfiguration
	err := res.Data.UnmarshalTo(&proxyConfig)
	if err != nil {
		return nil, err
	}

	var proxyIDs []resource.ReferenceOrID

	requests, err := mapWorkloadsBySelector(ctx, rt.Client, proxyConfig.Workloads, res.Id.Tenancy, func(id *pbresource.ID) {
		proxyIDs = append(proxyIDs, id)
	})

	m.proxyCfgCache.TrackProxyConfiguration(res.Id, proxyIDs)

	return requests, nil
}
