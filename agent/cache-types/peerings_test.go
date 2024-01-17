package cachetype

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/cache"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func TestPeerings(t *testing.T) {
	client := NewMockPeeringLister(t)
	typ := &Peerings{Client: client}

	resp := &pbpeering.PeeringListResponse{
		Peerings: []*pbpeering.Peering{
			{
				Name:                "peer1",
				ID:                  "8ac403cf-6834-412f-9dfe-0ac6e69bd89f",
				PeerServerAddresses: []string{"1.2.3.4"},
				State:               pbpeering.PeeringState_ACTIVE,
			},
		},
	}

	// Expect the proper call.
	// This also set the gRPC metadata returned by pointer.
	client.On("PeeringList", mock.Anything, mock.Anything, mock.Anything).
		Return(resp, nil).
		Run(func(args mock.Arguments) {
			// Validate Query Options
			ctx := args.Get(0).(context.Context)
			out, ok := metadata.FromOutgoingContext(ctx)
			require.True(t, ok)
			ctx = metadata.NewIncomingContext(ctx, out)

			options, err := external.QueryOptionsFromContext(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(28), options.MinQueryIndex)
			require.Equal(t, time.Duration(1100), options.MaxQueryTime)
			require.True(t, options.AllowStale)

			// Send back Query Meta on pointer of header
			header := args.Get(2).(grpc.HeaderCallOption)
			qm := structs.QueryMeta{
				Index: 48,
			}

			md, err := external.GRPCMetadataFromQueryMeta(qm)
			require.NoError(t, err)
			*header.HeaderAddr = md
		})

	// Fetch and assert against the result.
	result, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 28,
		Timeout:  time.Duration(1100),
	}, &PeeringListRequest{
		Request: &pbpeering.PeeringListRequest{},
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestPeerings_PeeringDisabled(t *testing.T) {
	client := NewMockPeeringLister(t)
	typ := &Peerings{Client: client}

	var resp *pbpeering.PeeringListResponse

	// Expect the proper call, but return the peering disabled error
	client.On("PeeringList", mock.Anything, mock.Anything, mock.Anything).
		Return(resp, grpcstatus.Error(codes.FailedPrecondition, "peering must be enabled to use this endpoint"))

	// Fetch and assert against the result.
	result, err := typ.Fetch(cache.FetchOptions{}, &PeeringListRequest{
		Request: &pbpeering.PeeringListRequest{},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.EqualValues(t, 1, result.Index)
	require.NotNil(t, result.Value)
}

func TestPeerings_badReqType(t *testing.T) {
	client := pbpeering.NewPeeringServiceClient(nil)
	typ := &Peerings{Client: client}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
}
