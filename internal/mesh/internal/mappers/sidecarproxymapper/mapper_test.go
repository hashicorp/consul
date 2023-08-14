package sidecarproxymapper

import (
	"context"
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestMapWorkloadsBySelector(t *testing.T) {
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
	w3 := resourcetest.Resource(catalog.WorkloadType, "prefix-w3").
		WithData(t, workloadData).
		Write(t, client).Id
	w4 := resourcetest.Resource(catalog.WorkloadType, "prefix-w4").
		WithData(t, workloadData).
		Write(t, client).Id

	selector := &pbcatalog.WorkloadSelector{
		Names:    []string{"w1", "w2"},
		Prefixes: []string{"prefix"},
	}
	expReqs := []controller.Request{
		{ID: resource.ReplaceType(types.ProxyStateTemplateType, w1)},
		{ID: resource.ReplaceType(types.ProxyStateTemplateType, w2)},
		{ID: resource.ReplaceType(types.ProxyStateTemplateType, w3)},
		{ID: resource.ReplaceType(types.ProxyStateTemplateType, w4)},
	}

	var cachedReqs []controller.Request

	reqs, err := mapWorkloadsBySelector(context.Background(), client, selector, defaultTenancy(), func(id *pbresource.ID) {
		// save IDs to check that the cache func is called
		cachedReqs = append(cachedReqs, controller.Request{ID: id})
	})
	require.NoError(t, err)
	require.Len(t, reqs, len(expReqs))
	prototest.AssertElementsMatch(t, expReqs, reqs)
	prototest.AssertElementsMatch(t, expReqs, cachedReqs)
}

func defaultTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Namespace: "default",
		Partition: "default",
		PeerName:  "local",
	}
}
