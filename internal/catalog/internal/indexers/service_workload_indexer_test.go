package indexers

import (
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestServiceWorkloadIndexer(t *testing.T) {
	c := cache.New()
	require.NoError(t, c.AddIndex(pbcatalog.ServiceType, "workloads", ServiceWorkloadIndexer()))

	foo := rtest.Resource(pbcatalog.ServiceType, "foo").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{
					"api-2",
				},
				Prefixes: []string{
					"api-1",
				},
			},
		}).
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
			PeerName:  "local",
		}).
		Build()

	require.NoError(t, c.Insert(foo))

	bar := rtest.Resource(pbcatalog.ServiceType, "bar").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{
					"api-3",
				},
				Prefixes: []string{
					"api-2",
				},
			},
		}).
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
			PeerName:  "local",
		}).
		Build()

	require.NoError(t, c.Insert(bar))

	api123 := rtest.Resource(pbcatalog.WorkloadType, "api-123").
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
			PeerName:  "local",
		}).
		Reference("")

	api2 := rtest.Resource(pbcatalog.WorkloadType, "api-2").
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
			PeerName:  "local",
		}).
		Reference("")

	resources, err := c.Parents(pbcatalog.ServiceType, "workloads", api123)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	prototest.AssertDeepEqual(t, foo, resources[0])

	resources, err = c.Parents(pbcatalog.ServiceType, "workloads", api2)
	require.NoError(t, err)
	require.Len(t, resources, 2)
	prototest.AssertElementsMatch(t, []*pbresource.Resource{foo, bar}, resources)
}
