package cachetype

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGatewayServices(t *testing.T) {
	rpc := TestRPC(t)
	typ := &GatewayServices{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedGatewayServices
	rpc.On("RPC", "Catalog.GatewayServices", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.ServiceSpecificRequest)
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
			reply := args.Get(2).(*structs.IndexedGatewayServices)
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

func TestGatewayServices_IntegrationWithCache_NotModifiedResponse(t *testing.T) {
	rpc := &MockRPC{}
	typ := &GatewayServices{RPC: rpc}

	services := structs.GatewayServices{
		&structs.GatewayService{Gateway: structs.NewServiceName("gateway", nil)},
	}
	rpc.On("RPC", "Catalog.GatewayServices", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.ServiceSpecificRequest)
			require.True(t, req.AllowStale)
			require.True(t, req.AllowNotModifiedResponse)

			reply := args.Get(2).(*structs.IndexedGatewayServices)
			reply.QueryMeta.Index = 44
			reply.NotModified = true
		})

	c := cache.New(cache.Options{})
	c.RegisterType(GatewayServicesName, typ)
	last := cache.FetchResult{
		Value: &structs.IndexedGatewayServices{
			Services:  services,
			QueryMeta: structs.QueryMeta{Index: 42},
		},
		Index: 42,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{
			Token:         "token",
			MinQueryIndex: 44,
			MaxQueryTime:  time.Second,
		},
	}

	err := c.Prepopulate(GatewayServicesName, last, "dc1", "token", req.CacheInfo().Key)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	actual, _, err := c.Get(ctx, GatewayServicesName, req)
	require.NoError(t, err)

	expected := &structs.IndexedGatewayServices{
		Services:  services,
		QueryMeta: structs.QueryMeta{Index: 42},
	}
	require.Equal(t, expected, actual)

	rpc.AssertExpectations(t)
}
