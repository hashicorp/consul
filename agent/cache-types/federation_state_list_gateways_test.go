package cachetype

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFederationStateListMeshGateways(t *testing.T) {
	rpc := TestRPC(t)
	typ := &FederationStateListMeshGateways{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.DatacenterIndexedCheckServiceNodes
	rpc.On("RPC", "FederationState.ListMeshGateways", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DCSpecificRequest)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)

			reply := args.Get(2).(*structs.DatacenterIndexedCheckServiceNodes)
			reply.DatacenterNodes = map[string]structs.CheckServiceNodes{
				"dc9": []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							ID:         "664bac9f-4de7-4f1b-ad35-0e5365e8f329",
							Node:       "gateway1",
							Datacenter: "dc9",
							Address:    "1.2.3.4",
						},
						Service: &structs.NodeService{
							ID:      "mesh-gateway",
							Service: "mesh-gateway",
							Kind:    structs.ServiceKindMeshGateway,
							Port:    1111,
							Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
						},
						Checks: []*structs.HealthCheck{
							{
								Name:      "web connectivity",
								Status:    api.HealthPassing,
								ServiceID: "mesh-gateway",
							},
						},
					},
					{
						Node: &structs.Node{
							ID:         "3fb9a696-8209-4eee-a1f7-48600deb9716",
							Node:       "gateway2",
							Datacenter: "dc9",
							Address:    "9.8.7.6",
						},
						Service: &structs.NodeService{
							ID:      "mesh-gateway",
							Service: "mesh-gateway",
							Kind:    structs.ServiceKindMeshGateway,
							Port:    2222,
							Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
						},
						Checks: []*structs.HealthCheck{
							{
								Name:      "web connectivity",
								Status:    api.HealthPassing,
								ServiceID: "mesh-gateway",
							},
						},
					},
				},
			}
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.DCSpecificRequest{
		Datacenter: "dc1",
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)

	rpc.AssertExpectations(t)
}

func TestFederationStateListMeshGateways_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	typ := &FederationStateListMeshGateways{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
	rpc.AssertExpectations(t)
}
