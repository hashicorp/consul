package sidecarproxymapper

import (
	"context"
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestProxyConfigurationMapper(t *testing.T) {
	client := svctest.RunResourceService(t, types.Register, catalog.RegisterTypes)

	// Create some workloads.
	// For this test, we don't care about the workload data, so we will re-use
	// the same data for all workloads.
	workloadData := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{{Host: "127.0.0.1"}},
		Ports:     map[string]*pbcatalog.WorkloadPort{"p1": {Port: 8080}},
	}
	w1 := resourcetest.Resource(catalog.WorkloadType, "w1").
		WithData(t, workloadData).
		Write(t, client).Id
	w2 := resourcetest.Resource(catalog.WorkloadType, "w2").
		WithData(t, workloadData).
		Write(t, client).Id

	// Create proxy configuration.
	proxyCfgData := &pbmesh.ProxyConfiguration{
		Workloads: &pbcatalog.WorkloadSelector{
			Names: []string{"w1", "w2"},
		},
	}
	pCfg := resourcetest.Resource(types.ProxyConfigurationType, "proxy-config").
		WithData(t, proxyCfgData).
		Write(t, client)

	m := Mapper{proxyCfgCache: sidecarproxycache.NewProxyConfigurationCache()}
	reqs, err := m.MapProxyConfigurationToProxyStateTemplate(context.Background(), controller.Runtime{
		Client: client,
	}, pCfg)
	require.NoError(t, err)

	p1 := resource.ReplaceType(types.ProxyStateTemplateType, w1)
	p2 := resource.ReplaceType(types.ProxyStateTemplateType, w2)
	expReqs := []controller.Request{
		{ID: p1},
		{ID: p2},
	}
	prototest.AssertElementsMatch(t, expReqs, reqs)

	// Check that the cache is populated.

	// Clean out UID as we don't care about it in the cache.
	pCfg.Id.Uid = ""
	prototest.AssertElementsMatch(t,
		[]*pbresource.ID{pCfg.Id},
		m.proxyCfgCache.ProxyConfigurationsByProxyID(p1))

	prototest.AssertElementsMatch(t,
		[]*pbresource.ID{pCfg.Id},
		m.proxyCfgCache.ProxyConfigurationsByProxyID(p2))
}
