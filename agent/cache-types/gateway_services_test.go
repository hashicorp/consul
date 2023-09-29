package cachetype

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

func TestGatewayServices(t *testing.T) {
	rpc := TestRPC(t)
	typ := &GatewayServices{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedGatewayServices
	rpc.On("RPC", mock.Anything, "Catalog.GatewayServices", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*structs.ServiceSpecificRequest)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)
			require.Equal(t, "foo", req.ServiceName)

			services := structs.GatewayServices{
				{
					Service:     structs.NewServiceName("api", nil),
					Gateway:     structs.NewServiceName("gateway", nil),
					GatewayKind: structs.ServiceKindIngressGateway,
					Port:        1234,
					CAFile:      "api/ca.crt",
					CertFile:    "api/client.crt",
					KeyFile:     "api/client.key",
					SNI:         "my-domain",
				},
			}
			reply := args.Get(3).(*structs.IndexedGatewayServices)
			reply.Services = services
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "foo",
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)

	rpc.AssertExpectations(t)
}
